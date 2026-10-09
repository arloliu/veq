package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arloliu/go-secs/tracepack"
	"github.com/arloliu/go-secs/v2/hsms"

	"github.com/arloliu/veq/internal/eapsml"
)

// dialect names the source dialect and the parser mode (source_dialect).
const dialect = "eapsml/1 sml.Parse"

// config is what one conversion run needs.
type config struct {
	files    []string          // source files, in the order their source_ref entries are listed
	tools    map[string]string // connection key → tool_id
	eapID    string            // tool_id of the capture of lines that belong to no connection
	tz       string            // IANA name of the logs' time zone
	instance tracepack.UUID    // recorder_instance_id of this converter deployment
	out      string            // root of the segment keys
	now      time.Time         // writer start time; zero means the time of the last entry
	version  string            // converter version, in recorder and in every derived identifier
}

// source is one input file with its identity.
type source struct {
	path  string
	ref   string // source_ref: base name and content digest
	index uint64 // source_index of its annotations
}

// survey is what the first pass learns about the run's input.
type survey struct {
	conns      map[string]int // connection key → messages
	unassigned int            // messages of no known connection
	first      time.Time      // first parseable entry time
	last       time.Time      // last parseable entry time
}

// result is the outcome of a run, one entry per capture written.
type result struct {
	captures []*capture
	survey   survey
}

// run converts cfg.files into one capture per connection, plus one for lines of no connection, under cfg.out.
func run(ctx context.Context, cfg *config) (*result, error) {
	loc, err := time.LoadLocation(cfg.tz)
	if err != nil {
		return nil, err
	}
	srcs, err := identify(cfg.files)
	if err != nil {
		return nil, err
	}
	sv, err := surveyRun(srcs, loc)
	if err != nil {
		return nil, err
	}
	if err := checkTools(cfg, sv); err != nil {
		return nil, err
	}
	if sv.first.IsZero() {
		return nil, errors.New("no entry with a parseable timestamp")
	}
	if err := os.MkdirAll(cfg.out, 0o750); err != nil { //nolint:gosec // the operator names the output directory
		return nil, err
	}
	sink, err := tracepack.NewDirSink(cfg.out, "")
	if err != nil {
		return nil, err
	}
	now := cfg.now
	if now.IsZero() {
		now = sv.last
	}
	cv := &converter{
		cfg: cfg, srcs: srcs, sink: sink, now: func() time.Time { return now },
		meta: packMeta(cfg, srcs), captures: map[string]*capture{}, prevTime: sv.first,
	}
	for _, s := range srcs {
		if err := cv.convertFile(ctx, s, loc); err != nil {
			cv.abort()
			return nil, err
		}
	}
	for _, key := range cv.order {
		if err := cv.captures[key].close(ctx); err != nil {
			cv.abort()
			return nil, err
		}
	}
	res := &result{survey: sv}
	for _, key := range cv.order {
		res.captures = append(res.captures, cv.captures[key])
	}

	return res, nil
}

// identify names each file by its base name and the SHA-256 of its content,
// so the source_ref entries carry no local path.
func identify(paths []string) ([]source, error) {
	srcs := make([]source, 0, len(paths))
	for i, p := range paths {
		f, err := os.Open(p) //nolint:gosec // the operator names the files to convert
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		ref := filepath.Base(p) + "#sha256=" + hex.EncodeToString(h.Sum(nil))
		srcs = append(srcs, source{path: p, ref: ref, index: uint64(i)})
	}

	return srcs, nil
}

// surveyRun reads every file once to learn its connections and its first and last entry times.
func surveyRun(srcs []source, loc *time.Location) (survey, error) {
	sv := survey{conns: map[string]int{}}
	for _, s := range srcs {
		err := readFile(s.path, loc, func(m *eapsml.Message, conn string) error {
			for _, sp := range m.Spans {
				if sp.Time.IsZero() {
					continue
				}
				if sv.first.IsZero() {
					sv.first = sp.Time
				}
				sv.last = sp.Time
			}
			if m.Flags&(eapsml.FlagNotMessage|eapsml.FlagNotTraffic) != 0 {
				return nil
			}
			if conn == "" {
				sv.unassigned++
			} else {
				sv.conns[conn]++
			}

			return nil
		})
		if err != nil {
			return sv, err
		}
	}

	return sv, nil
}

// checkTools fails unless every connection found has a tool id
// and the EAP's id differs from every tool's, which no tracepack writer can check across captures.
func checkTools(cfg *config, sv survey) error {
	var missing []string
	for _, conn := range slices.Sorted(maps.Keys(sv.conns)) {
		if _, ok := cfg.tools[conn]; !ok {
			missing = append(missing, fmt.Sprintf("%s (%d messages)", conn, sv.conns[conn]))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %s", errUnmapped, strings.Join(missing, ", "))
	}
	seen := map[string]string{}
	for _, conn := range slices.Sorted(maps.Keys(cfg.tools)) {
		id := cfg.tools[conn]
		if id == cfg.eapID {
			return fmt.Errorf("tool id %q of %s is also the EAP's id", id, conn)
		}
		if other, ok := seen[id]; ok {
			return fmt.Errorf("tool id %q names both %s and %s", id, other, conn)
		}
		seen[id] = conn
	}

	return nil
}

// readFile reads every entry of path and calls fn with each message and its connection key, "" for none.
// A reply belongs to its primary's connection, whatever its own lines say.
func readFile(path string, loc *time.Location, fn func(*eapsml.Message, string) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	corr := eapsml.NewCorrelator()
	rd := eapsml.NewReader(f, loc, eapsml.WithEveryEntry())
	for {
		m, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		conn := ""
		if m.Flags&(eapsml.FlagNotMessage|eapsml.FlagNotTraffic) == 0 {
			conn = connection(m)
			if tx := corr.Add(m); tx != nil && tx.Primary != nil {
				conn = connection(tx.Primary)
			}
		}
		if err := fn(m, conn); err != nil {
			return err
		}
	}
}

// connection names the equipment connection of m from its own log lines, as cmd/logcat does:
// the EAP's DEVICEID when logged, else the session ID of its own header; "" when neither is known.
func connection(m *eapsml.Message) string {
	switch {
	case m.Device >= 0:
		return "deviceid=" + strconv.Itoa(m.Device)
	case m.SessionID >= 0:
		return "session=" + strconv.Itoa(m.SessionID)
	default:
		return ""
	}
}

// packMeta returns the pack metadata every segment of the run shares, tool and period aside.
func packMeta(cfg *config, srcs []source) tracepack.PackMeta {
	refs := make([]string, len(srcs))
	for i, s := range srcs {
		refs[i] = s.ref
	}
	recorder := "eapconv/" + cfg.version

	return tracepack.PackMeta{
		Transport:          tracepack.TransportHSMSSS,
		CaptureMethod:      tracepack.CaptureMethodLog,
		Vantage:            tracepack.VantageHost,
		Recorder:           recorder,
		Writer:             writerTag(),
		Classifiers:        []string{recorder + " sml.Parse"},
		TimeSource:         tracepack.TimeSourceSourceLog,
		SourceTZ:           &cfg.tz,
		SourceDialect:      new(dialect),
		SourceRefs:         refs,
		LifecycleCoverage:  tracepack.LifecycleCoverageNone,
		QualityEvaluated:   true,
		PackRole:           tracepack.PackRoleSegment,
		RecorderInstanceID: cfg.instance,
		ScopeGeneration:    new(uint64(0)),
	}
}

// converter is the second pass: it turns each message into records of its capture.
type converter struct {
	cfg      *config
	srcs     []source
	sink     tracepack.SegmentSink
	now      func() time.Time
	meta     tracepack.PackMeta
	captures map[string]*capture // by tool id
	order    []string            // tool ids in the order their captures started
	prevTime time.Time           // time of the previous entry, for an entry without one
}

func (cv *converter) convertFile(ctx context.Context, s source, loc *time.Location) error {
	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return readFile(s.path, loc, func(m *eapsml.Message, conn string) error {
		toolID := cv.cfg.eapID
		if conn != "" {
			toolID = cv.cfg.tools[conn]
		}
		c := cv.capture(toolID, m)
		if r, ok := cv.dataRecord(m); ok {
			return c.add(ctx, &r, spanBytes(m.Spans))
		}
		kind, status := annotationOf(m)
		for _, sp := range m.Spans {
			r, err := cv.annotation(f, s, sp, kind, status)
			if err != nil {
				return err
			}
			if err := c.add(ctx, &r, sp.Len); err != nil {
				return err
			}
		}

		return nil
	})
}

// capture returns the capture of toolID, starting it at m's time when it has none yet.
func (cv *converter) capture(toolID string, m *eapsml.Message) *capture {
	if c, ok := cv.captures[toolID]; ok {
		return c
	}
	t, _ := cv.timeOf(m.Spans[0])
	refs := make([]string, 0, len(cv.srcs)+2)
	for _, s := range cv.srcs {
		refs = append(refs, s.ref)
	}
	refs = append(refs, toolID, cv.cfg.version)
	c := newCapture(toolID, deriveID(t, append([]string{"capture"}, refs...)...), cv.meta, cv.sink, cv.now)
	cv.captures[toolID] = c
	cv.order = append(cv.order, toolID)

	return c
}

// dataRecord rebuilds m as a data record, or reports false when m cannot be one.
func (cv *converter) dataRecord(m *eapsml.Message) (tracepack.Record, bool) {
	// A body logged without its header is still a message, of unknown direction and identities.
	if m.Flags&^eapsml.FlagHeaderMissing != 0 || m.Item == nil {
		return tracepack.Record{}, false
	}
	var sysBytes [4]byte
	if m.SystemBytes >= 0 {
		sysBytes = [4]byte{byte(m.SystemBytes >> 24), byte(m.SystemBytes >> 16), byte(m.SystemBytes >> 8), byte(m.SystemBytes)}
	}
	sessionID := uint16(max(m.SessionID, 0))
	msg, err := hsms.NewDataMessage(m.Stream, m.Function, m.WBit, sessionID, sysBytes, m.Item)
	if err != nil {
		return tracepack.Record{}, false
	}
	ts, uncertain := cv.timeOf(m.Spans[0])
	r := tracepack.Record{
		TSUTCNs: ts.UnixNano(), Kind: tracepack.KindData, Dir: direction(m.Dir),
		Fidelity: tracepack.FidelityReconstructed, DecodeStatus: tracepack.DecodeStatusReconstructedOK,
		Payload: msg.ToBytes(),
	}
	r.SetCapturedFieldValidity()
	if m.SessionID < 0 {
		r.FieldValidity &^= tracepack.FieldValiditySessionID
	}
	if m.SystemBytes < 0 {
		r.FieldValidity &^= tracepack.FieldValiditySystemBytes
	}
	if uncertain {
		r.Quality |= tracepack.QualityOrderingUncertain
	}

	return r, true
}

// annotation returns the annotation of one entry: its source bytes, as text when they are UTF-8.
func (cv *converter) annotation(f io.ReaderAt, s source, sp eapsml.Span, kind tracepack.AnnotationKind, status tracepack.DecodeStatus) (tracepack.Record, error) {
	raw := make([]byte, sp.Len)
	if _, err := f.ReadAt(raw, sp.Offset); err != nil {
		return tracepack.Record{}, fmt.Errorf("%s: line %d: %w", s.path, sp.Line, err)
	}
	a := tracepack.Annotation{
		AnnotationKind: kind,
		SourceIndex:    new(s.index), SourceOffset: new(uint64(sp.Offset)), SourceLen: new(uint64(sp.Len)),
	}
	if utf8.Valid(raw) {
		a.Text = new(string(raw))
	} else {
		a.Raw = raw
	}
	payload, err := a.MarshalBinary()
	if err != nil {
		return tracepack.Record{}, err
	}
	ts, uncertain := cv.timeOf(sp)
	r := tracepack.Record{
		TSUTCNs: ts.UnixNano(), Kind: tracepack.KindAnnotation, Dir: tracepack.DirLocal,
		Fidelity: tracepack.FidelityNotApplicable, DecodeStatus: status, Payload: payload,
	}
	if uncertain {
		r.Quality |= tracepack.QualityOrderingUncertain
	}

	return r, nil
}

// timeOf returns the time of sp and whether its order is uncertain.
// An entry without a parseable time takes the previous entry's,
// or for the first entries of the run the first parseable time.
func (cv *converter) timeOf(sp eapsml.Span) (time.Time, bool) {
	if sp.Time.IsZero() {
		return cv.prevTime, true
	}
	cv.prevTime = sp.Time

	return sp.Time, sp.TimeUncertain
}

func (cv *converter) abort() {
	for _, c := range cv.captures {
		c.abort()
	}
}

// annotationOf returns the annotation kind and decode status of the entries of m, which is no data record.
func annotationOf(m *eapsml.Message) (tracepack.AnnotationKind, tracepack.DecodeStatus) {
	switch {
	case m.Flags&eapsml.FlagNotMessage != 0:
		return tracepack.AnnotationKindUnrecognisedLine, tracepack.DecodeStatusNotApplicable
	case m.Flags&^eapsml.FlagHeaderMissing != 0 || m.Item == nil:
		return tracepack.AnnotationKindUnparsedEntry, tracepack.DecodeStatusParseFailed
	default:
		// The entry parsed, but the message it gives is invalid, such as W on a reply.
		return tracepack.AnnotationKindUnparsedEntry, tracepack.DecodeStatusBuildRejected
	}
}

func direction(d eapsml.Direction) tracepack.Dir {
	if d == eapsml.HostToEquip {
		return tracepack.DirHostToEquipment
	}
	if d == eapsml.EquipToHost {
		return tracepack.DirEquipmentToHost
	}

	return tracepack.DirUnknown
}

func spanBytes(spans []eapsml.Span) int64 {
	var n int64
	for _, sp := range spans {
		n += sp.Len
	}

	return n
}
