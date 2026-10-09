package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/arloliu/go-secs/tracepack"
)

// errUnmapped reports connections found in the logs that no -tool flag names.
var errUnmapped = errors.New("connections without a tool id")

// capture writes one capture of a run as segment packs, one per run of records in the same UTC hour.
// It owns the capture's seq counter and its start and stop boundaries,
// which it writes at the times of its first and last records.
type capture struct {
	toolID  string
	id      tracepack.UUID
	meta    tracepack.PackMeta // template; period, seq_start and pack identity are set per segment
	sink    tracepack.SegmentSink
	now     func() time.Time
	nextSeq uint64
	lastTS  int64
	seg     *segment
	stats   captureStats
}

// segment is the open segment pack of a capture.
type segment struct {
	hour int64
	file tracepack.SegmentFile
	w    *tracepack.Writer
}

// captureStats is what a capture reports when the run ends.
type captureStats struct {
	Records  uint64                            // records, boundaries included
	Segments int                               // segment packs written
	Bytes    int64                             // source bytes its records and annotations cover
	Status   map[tracepack.DecodeStatus]uint64 // records per decode_status, boundaries excluded
}

func newCapture(toolID string, id tracepack.UUID, meta tracepack.PackMeta, sink tracepack.SegmentSink, now func() time.Time) *capture {
	meta.ToolID = toolID

	return &capture{
		toolID: toolID, id: id, meta: meta, sink: sink, now: now,
		stats: captureStats{Status: map[tracepack.DecodeStatus]uint64{}},
	}
}

// add appends r, a record without seq that covers n source bytes;
// before the capture's first record it appends the start boundary at r's time.
func (c *capture) add(ctx context.Context, r *tracepack.Record, n int64) error {
	if c.nextSeq == 0 {
		start, err := boundary(tracepack.BoundaryKindStart, r.TSUTCNs)
		if err != nil {
			return err
		}
		if err := c.append(ctx, &start); err != nil {
			return err
		}
	}
	if err := c.append(ctx, r); err != nil {
		return err
	}
	c.stats.Status[r.DecodeStatus]++
	c.stats.Bytes += n

	return nil
}

// close appends the stop boundary at the time of the last record and commits the last segment.
func (c *capture) close(ctx context.Context) error {
	stop, err := boundary(tracepack.BoundaryKindStop, c.lastTS)
	if err != nil {
		return err
	}
	if err := c.append(ctx, &stop); err != nil {
		return err
	}

	return c.finish(ctx)
}

// abort discards the open segment, if any.
func (c *capture) abort() {
	if c.seg != nil {
		_ = c.seg.file.Abort()
		c.seg = nil
	}
}

func (c *capture) append(ctx context.Context, r *tracepack.Record) error {
	hour := tracepack.HourOf(time.Unix(0, r.TSUTCNs))
	if c.seg != nil && c.seg.hour != hour {
		if err := c.finish(ctx); err != nil {
			return err
		}
	}
	if c.seg == nil {
		if err := c.open(ctx, hour); err != nil {
			return err
		}
	}
	r.Seq = c.nextSeq
	// Every record of the log is of epoch 0, which the format marks correlation-incomplete.
	r.Quality |= tracepack.QualityCorrelationIncomplete
	if err := c.seg.w.Append(r); err != nil {
		return fmt.Errorf("%s: seq %d: %w", c.toolID, r.Seq, err)
	}
	c.nextSeq++
	c.lastTS = r.TSUTCNs
	c.stats.Records++

	return nil
}

// open starts the segment of hour, whose period is the whole UTC hour.
func (c *capture) open(ctx context.Context, hour int64) error {
	start := time.Unix(hour*3600, 0).UTC()
	end := start.Add(time.Hour)
	packID := deriveID(start, "pack", c.id.String(), fmt.Sprint(c.nextSeq))
	meta := c.meta
	meta.PeriodStart, meta.PeriodEnd, meta.SeqStart = start.UnixNano(), end.UnixNano(), c.nextSeq
	file, err := c.sink.Create(ctx, tracepack.SegmentInfo{
		ToolID: c.toolID, CaptureID: c.id, PackID: packID, SeqFirst: c.nextSeq,
		Hour: hour, PeriodStart: start, PeriodEnd: end,
	})
	if err != nil {
		return fmt.Errorf("%s: create segment: %w", c.toolID, err)
	}
	w, err := tracepack.NewWriter(file, tracepack.WriterOptions{
		Meta:      &meta,
		Facts:     tracepack.PackFacts{AnyClassified: true},
		Codec:     tracepack.CodecZstd,
		Sync:      file,
		PackID:    packID,
		CaptureID: c.id,
		Now:       c.now,
	})
	if err != nil {
		_ = file.Abort()
		return fmt.Errorf("%s: start segment: %w", c.toolID, err)
	}
	c.seg = &segment{hour: hour, file: file, w: w}
	c.stats.Segments++

	return nil
}

// finish closes and commits the open segment.
func (c *capture) finish(ctx context.Context) error {
	s := c.seg
	if _, err := s.w.Close(); err != nil {
		return fmt.Errorf("%s: close segment: %w", c.toolID, err)
	}
	if err := s.file.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit segment: %w", c.toolID, err)
	}
	c.seg = nil

	return nil
}

// boundary returns a capture-boundary record of kind at ts, as a SegmentWriter writes one.
func boundary(kind tracepack.BoundaryKind, ts int64) (tracepack.Record, error) {
	ev := tracepack.TransportEvent{Event: tracepack.EventCaptureBoundary, BoundaryKind: &kind}
	payload, err := ev.MarshalBinary()
	if err != nil {
		return tracepack.Record{}, err
	}

	return tracepack.Record{
		TSUTCNs: ts, Kind: tracepack.KindTransportEvent, Dir: tracepack.DirLocal,
		Fidelity: tracepack.FidelityNotApplicable, DecodeStatus: tracepack.DecodeStatusNotApplicable,
		Quality: tracepack.QualityCaptureBoundary, Payload: payload,
	}, nil
}

// deriveID returns a UUIDv7 whose time is t's millisecond and whose other bits hash the parts,
// so the same input always yields the same identifier.
func deriveID(t time.Time, parts ...string) tracepack.UUID {
	h := sha256.New()
	for _, p := range parts {
		_ = binary.Write(h, binary.BigEndian, uint64(len(p)))
		h.Write([]byte(p))
	}
	sum := h.Sum(nil)

	var u tracepack.UUID
	ms := uint64(t.UnixMilli())
	for i := range 6 {
		u[i] = byte(ms >> (40 - 8*i))
	}
	copy(u[6:], sum[:10])
	u[6] = 0x70 | u[6]&0x0f // version 7
	u[8] = 0x80 | u[8]&0x3f // RFC 9562 variant

	return u
}
