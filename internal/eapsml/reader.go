// Package eapsml reads the SECS messages
// that an EAP (equipment automation program) prints to its text log as SML,
// and pairs primaries with their replies.
//
// The dialect it reads writes one header line per message
// and, separately, one body line followed by the indented SML item tree:
//
//	2026-10-02 08:48:06.4475 INFO - [...]: Send Msg: [S1F3] /Header=[00-01-81-03-00-00-3A-A0-6A-1F] TID=2555 ...
//	2026-10-02 08:48:06.4475 INFO - [...PrintSECSMsgBody]: S1F3_QueryState: 'S1F3' W
//	    <L [1]
//	        <U4 [1] 1001 >
//	     >
//	.
//
// "Received SxFy" header lines are equipment-to-host; "Send Msg: [SxFy]" lines are host-to-equipment.
// A body line is any "Name: 'SxFy'" line that follows a header of the same SxFy.
// Other such lines, such as messages the EAP builds for its own use, are not traffic and are skipped,
// except that a PrintSECSMsgBody line without a header is kept with FlagHeaderMissing.
// The logged header of a reply can carry the primary's stream and function,
// so the W-bit and the session ID are read from the header only when the header names the same message.
// Some EAP versions add a "DEVICEID=n" token to the header line:
// the EAP's number for the equipment connection, which can differ from the header's session ID.
// Bodies the EAP logs encrypted ("Encrypt S7Fy=...") are reported with FlagEncrypted and no item.
//
// An entry is a timestamped line with the untimestamped lines after it.
// Each message records the byte spans of the entries it was read from;
// WithEveryEntry also returns the entries that hold no message, so the spans cover the whole input.
package eapsml

import (
	"bufio"
	"cmp"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/arloliu/go-secs/v2/secs2"
	"github.com/arloliu/go-secs/v2/sml"
)

// Direction is the sending side of a message.
type Direction uint8

const (
	DirUnknown Direction = iota
	EquipToHost
	HostToEquip
)

func (d Direction) String() string {
	switch d {
	case EquipToHost:
		return "E->H"
	case HostToEquip:
		return "H->E"
	case DirUnknown:
		return "?"
	default:
		return fmt.Sprintf("Direction(%d)", uint8(d))
	}
}

// Flag marks what the log could not supply for a message.
type Flag uint8

const (
	// FlagEncrypted: the EAP logged the body encrypted; Item is nil.
	FlagEncrypted Flag = 1 << iota
	// FlagBodyMissing: a header line with no body line for it.
	FlagBodyMissing
	// FlagHeaderMissing: a body line with no header line for it; Direction and TID are unknown.
	FlagHeaderMissing
	// FlagParseFailed: the SML body did not parse; ParseErr says why and Item is nil.
	FlagParseFailed
	// FlagNotMessage: an entry that holds no message, returned only WithEveryEntry;
	// only Spans, Line and Time are set.
	FlagNotMessage
	// FlagNotTraffic: a message the EAP built for its own use and logged without a header, returned only WithEveryEntry;
	// it was never sent, so its body is not parsed.
	FlagNotTraffic
)

// Message is one SECS message reconstructed from the log.
type Message struct {
	Time     time.Time // log timestamp of the header line, or of the body line when the header is missing
	Line     int       // 1-based line number of the header line, or of the body line when the header is missing
	Dir      Direction
	Stream   uint8
	Function uint8
	WBit     bool
	TID      uint32 // the EAP's transaction id; equal on a primary and its reply
	Header   []byte // the 10-byte HSMS header as logged, nil when absent
	// SessionID is bytes 0-1 of the logged header, -1 when the header is absent or names another message.
	SessionID int
	// SystemBytes is bytes 6-9 of the logged header, -1 when the header is absent or names another message.
	SystemBytes int64
	// Device is the EAP's device number from the header line's DEVICEID token, -1 when not logged.
	Device   int
	Name     string // the EAP's own name for the message, may be empty
	Body     string // the SML text as logged, without the body line
	Item     secs2.Item
	Flags    Flag
	ParseErr error
	// Spans are the log entries the message was read from, in log order:
	// the header entry, then the body or encrypted-body entry, as the log holds them.
	Spans []Span
	// textW: the body line itself carries " W", so the SML text is parsed with it.
	textW bool
}

// SxFy returns the message's stream and function as "SxFy".
func (m *Message) SxFy() string { return fmt.Sprintf("S%dF%d", m.Stream, m.Function) }

const tsLayout = "2006-01-02 15:04:05.0000"

var (
	tsRe      = regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{4}`)
	headerRe  = regexp.MustCompile(`(?:Received S(\d+)F(\d+)|Send Msg: \[S(\d+)F(\d+)\]) /Header=\[([0-9A-Fa-f-]+)\] TID=(\d+)`)
	bodyRe    = regexp.MustCompile(`\]: (?:(.*?): )?'S(\d+)F(\d+)'( W)?\s*$`)
	deviceRe  = regexp.MustCompile(` DEVICEID=(\d+)`)
	encryptRe = regexp.MustCompile(`PrintSECSMsgBody\]: Encrypt S\d+Fy=S(\d+)F(\d+),TID=(\d+),`)
)

// entryOwner says what the open entry belongs to, which decides what closing it does.
type entryOwner uint8

const (
	ownerNone   entryOwner = iota // nothing keeps the entry
	ownerHeader                   // the header of a message waiting for its body
	ownerBody                     // the body of a message, finished when the entry closes
	ownerWhole                    // a complete message, returned when the entry closes
)

// Reader reads messages from an EAP log in log order of their bodies.
type Reader struct {
	sc      *bufio.Scanner
	loc     *time.Location
	every   bool
	line    int
	offset  int64               // bytes read so far
	started bool                // an entry is open
	entry   Span                // the open entry
	owner   *Message            // what the open entry belongs to, by kind
	kind    entryOwner          // how the open entry belongs to owner
	pending map[string]*Message // header seen, body not yet; keyed by SxFy
	lines   []string            // SML lines of the open body entry
	out     []*Message
	eof     bool
}

// NewReader returns a Reader that interprets log timestamps in loc.
func NewReader(r io.Reader, loc *time.Location, opts ...Option) *Reader {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	sc.Split(scanLines)
	rd := &Reader{sc: sc, loc: loc, pending: map[string]*Message{}}
	for _, o := range opts {
		o(rd)
	}

	return rd
}

// Next returns the next message, or io.EOF after the last one.
func (r *Reader) Next() (*Message, error) {
	for len(r.out) == 0 {
		if r.eof {
			return nil, io.EOF
		}
		if err := r.step(); err != nil {
			return nil, err
		}
	}
	m := r.out[0]
	r.out = r.out[1:]

	return m, nil
}

func (r *Reader) step() error {
	if !r.sc.Scan() {
		if err := r.sc.Err(); err != nil {
			return err
		}
		r.eof = true
		r.closeEntry()
		rest := slices.Collect(maps.Values(r.pending))
		slices.SortFunc(rest, func(a, b *Message) int { return cmp.Compare(a.Line, b.Line) })
		for _, m := range rest {
			m.Flags |= FlagBodyMissing
			r.out = append(r.out, m)
		}
		clear(r.pending)

		return nil
	}
	r.line++
	raw := r.sc.Bytes()
	ln := string(trimLineEnd(raw))
	ts := tsRe.FindString(ln)
	if ts == "" && r.started {
		r.entry.Len += int64(len(raw))
		r.offset += int64(len(raw))
		if r.kind == ownerBody {
			r.lines = append(r.lines, ln)
		}

		return nil
	}
	r.closeEntry()
	r.started = true
	r.entry = Span{Offset: r.offset, Len: int64(len(raw)), Line: r.line}
	r.offset += int64(len(raw))
	if ts == "" {
		// Lines before the first timestamped line.
		r.keepOther(newMessage("0", "0", time.Time{}, r.line))
		return nil
	}
	t, uncertain, err := parseLocal(ts, r.loc)
	if err != nil {
		if !r.every {
			return fmt.Errorf("line %d: %w", r.line, err)
		}
		r.keepOther(newMessage("0", "0", time.Time{}, r.line))

		return nil
	}
	r.entry.Time, r.entry.TimeUncertain = t, uncertain
	r.classify(ln, t)

	return nil
}

// classify decides what the timestamped line ln opens.
func (r *Reader) classify(ln string, t time.Time) {
	if h := headerRe.FindStringSubmatch(ln); h != nil {
		r.owner, r.kind = r.onHeader(h, deviceRe.FindStringSubmatch(ln), t), ownerHeader
		return
	}
	if b := bodyRe.FindStringSubmatch(ln); b != nil {
		key := "S" + b[2] + "F" + b[3]
		if _, ok := r.pending[key]; !ok && !strings.Contains(ln, "PrintSECSMsgBody]") {
			m := newMessage(b[2], b[3], t, r.line)
			m.Name, m.WBit, m.Flags = b[1], b[4] != "", FlagNotTraffic
			r.keepOther(m)

			return
		}
		m := r.takePending(b[2], b[3], t)
		m.Name = b[1]
		m.textW = b[4] != ""
		m.WBit = m.WBit || m.textW
		r.owner, r.kind = m, ownerBody

		return
	}
	if e := encryptRe.FindStringSubmatch(ln); e != nil {
		m := r.takePending(e[1], e[2], t)
		m.Flags |= FlagEncrypted
		r.owner, r.kind = m, ownerWhole

		return
	}
	m := newMessage("0", "0", t, r.line)
	r.keepOther(m)
}

// keepOther makes m, an entry that is no traffic, the owner of the open entry when every entry is returned.
func (r *Reader) keepOther(m *Message) {
	if !r.every {
		return
	}
	if m.Flags&FlagNotTraffic == 0 {
		m.Flags |= FlagNotMessage
	}
	r.owner, r.kind = m, ownerWhole
}

// closeEntry gives the open entry to its owner and finishes what the entry completes.
func (r *Reader) closeEntry() {
	if !r.started {
		return
	}
	m, kind := r.owner, r.kind
	r.started, r.owner, r.kind = false, nil, ownerNone
	if kind == ownerNone {
		r.lines = r.lines[:0]
		return
	}
	m.Spans = append(m.Spans, r.entry)
	// A header entry waits in pending for its body.
	if kind == ownerBody {
		r.finishBody(m)
	}
	if kind == ownerWhole {
		r.out = append(r.out, m)
	}
}

func (r *Reader) onHeader(h, dev []string, t time.Time) *Message {
	s, f, dir := h[1], h[2], EquipToHost
	if s == "" {
		s, f, dir = h[3], h[4], HostToEquip
	}
	m := newMessage(s, f, t, r.line)
	m.Dir = dir
	tid, _ := strconv.ParseUint(h[6], 10, 32)
	m.TID = uint32(tid)
	if hdr, err := hex.DecodeString(strings.ReplaceAll(h[5], "-", "")); err == nil && len(hdr) == 10 {
		m.Header = hdr
		if hdr[2]&0x7f == m.Stream && hdr[3] == m.Function {
			m.WBit = hdr[2]&0x80 != 0
			m.SessionID = int(hdr[0])<<8 | int(hdr[1])
			m.SystemBytes = int64(binary.BigEndian.Uint32(hdr[6:10]))
		}
	}
	if dev != nil {
		if d, err := strconv.ParseUint(dev[1], 10, 16); err == nil {
			m.Device = int(d)
		}
	}
	key := m.SxFy()
	if prev, ok := r.pending[key]; ok {
		prev.Flags |= FlagBodyMissing
		r.out = append(r.out, prev)
	}
	r.pending[key] = m

	return m
}

// takePending returns the message whose header precedes this body, or a header-less one.
func (r *Reader) takePending(s, f string, t time.Time) *Message {
	m := newMessage(s, f, t, r.line)
	if p, ok := r.pending[m.SxFy()]; ok {
		delete(r.pending, m.SxFy())
		return p
	}
	m.Flags |= FlagHeaderMissing

	return m
}

func (r *Reader) finishBody(m *Message) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "'%s'", m.SxFy())
	// Only a W in the source text is parsed; a W from the header alone is metadata,
	// so an invalid one surfaces when the message is built, not here.
	if m.textW {
		sb.WriteString(" W")
	}
	sb.WriteByte('\n')
	for _, l := range r.lines {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	r.lines = r.lines[:0]
	text := sb.String()
	m.Body = text[strings.IndexByte(text, '\n')+1:]
	msgs, err := sml.Parse(text)
	switch {
	case err != nil:
		m.Flags |= FlagParseFailed
		m.ParseErr = err
	case len(msgs) != 1:
		m.Flags |= FlagParseFailed
		m.ParseErr = fmt.Errorf("body parsed into %d messages", len(msgs))
	default:
		if m.Item, err = msgs[0].Item(); err != nil {
			m.Flags |= FlagParseFailed
			m.ParseErr = err
		}
	}
	r.out = append(r.out, m)
}

func newMessage(s, f string, t time.Time, line int) *Message {
	sn, _ := strconv.ParseUint(s, 10, 8)
	fn, _ := strconv.ParseUint(f, 10, 8)

	return &Message{Time: t, Line: line, Stream: uint8(sn), Function: uint8(fn), SessionID: -1, SystemBytes: -1, Device: -1}
}

// ReadAll reads every message from r.
func ReadAll(r io.Reader, loc *time.Location, opts ...Option) ([]*Message, error) {
	rd := NewReader(r, loc, opts...)
	var all []*Message
	for {
		m, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return all, nil
		}
		if err != nil {
			return all, err
		}
		all = append(all, m)
	}
}
