package eapsml

import (
	"bytes"
	"time"
)

// Span is the source bytes of one log entry:
// a timestamped line and the untimestamped lines after it, line ends included.
// Lines before the first timestamped line form an entry of their own.
type Span struct {
	Offset int64 // byte offset of the entry's first line in the input
	Len    int64 // byte length of the entry
	Line   int   // 1-based line number of the entry's first line
	// Time is the entry's timestamp, zero when its first line carries none that parses.
	Time time.Time
	// TimeUncertain marks a local time that is ambiguous or does not exist in the reader's location,
	// for example in a daylight-saving change; Time is then the earlier of its readings.
	TimeUncertain bool
}

// Option configures a Reader.
type Option func(*Reader)

// WithEveryEntry makes the Reader return every entry of the log,
// so the spans of the returned messages cover the input exactly once.
// Besides messages it returns each entry that holds no message, with FlagNotMessage,
// and each message the EAP built for its own use, with FlagNotTraffic.
// A timestamp that does not parse no longer fails the read: its entry gets a zero time.
func WithEveryEntry() Option {
	return func(r *Reader) { r.every = true }
}

// scanLines is bufio.ScanLines that keeps each line's end, so line lengths add up to the input's.
func scanLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i+1], nil
	}
	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}

// trimLineEnd returns line without its "\n" or "\r\n".
func trimLineEnd(line []byte) []byte {
	line = bytes.TrimSuffix(line, []byte("\n"))

	return bytes.TrimSuffix(line, []byte("\r"))
}

// parseLocal parses a log timestamp as local time in loc.
// A local time that occurs twice or never in a daylight-saving change is reported uncertain.
// It resolves to the earlier of its readings.
func parseLocal(s string, loc *time.Location) (time.Time, bool, error) {
	naive, err := time.Parse(tsLayout, s)
	if err != nil {
		return time.Time{}, false, err
	}
	// The offsets in force a day before and a day after cover both sides of any change.
	var readings []time.Time
	for _, probe := range []time.Time{naive.Add(-24 * time.Hour), naive.Add(24 * time.Hour)} {
		_, off := probe.In(loc).Zone()
		t := naive.Add(-time.Duration(off) * time.Second).In(loc)
		if !hasWall(readings, t) {
			readings = append(readings, t)
		}
	}
	var valid []time.Time
	for _, t := range readings {
		if sameWall(t, naive) {
			valid = append(valid, t)
		}
	}
	if len(valid) == 1 {
		return valid[0], false, nil
	}
	if len(valid) == 0 {
		valid = readings
	}
	earliest := valid[0]
	for _, t := range valid[1:] {
		if t.Before(earliest) {
			earliest = t
		}
	}

	return earliest, true, nil
}

func hasWall(ts []time.Time, t time.Time) bool {
	for _, u := range ts {
		if u.Equal(t) {
			return true
		}
	}

	return false
}

// sameWall reports whether t reads as the wall clock of naive, a time in UTC.
func sameWall(t, naive time.Time) bool {
	y, mo, d := t.Date()
	h, mi, s := t.Clock()

	return time.Date(y, mo, d, h, mi, s, t.Nanosecond(), time.UTC).Equal(naive)
}
