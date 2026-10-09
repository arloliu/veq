package eapsml

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// spanLog is synthetic: a line before the first timestamp, CRLF line ends,
// lines of no message, an EAP-built body, a bad timestamp and no final line end.
const spanLog = "leading junk\n" +
	"2026-01-02 08:00:00.0001 INFO - [Sender.Send]: Send Msg: [S1F3] /Header=[00-01-81-03-00-00-00-00-00-10] TID=16\r\n" +
	"2026-01-02 08:00:00.0002 INFO - [Other.Log]: unrelated line\n" +
	"  continuation of the unrelated line\n" +
	"2026-01-02 08:00:00.0003 INFO - [Logger.PrintSECSMsgBody]: S1F3_QueryState: 'S1F3' W\r\n" +
	"    <U4 [1] 1001 >\r\n" +
	".\r\n" +
	"2026-01-02 08:00:01.0001 INFO - [Collector.Custom]: S6F11_Internal: 'S6F11'\n" +
	"    <L [0] >\n" +
	".\n" +
	"2026-13-02 08:00:02.0000 INFO - [Other.Log]: a month that does not exist\n" +
	"2026-01-02 08:00:03.0000 INFO - [Handler.EventHandler]: Received S7F26 /Header=[00-01-07-1A-00-00-00-00-00-40] TID=64\n" +
	"2026-01-02 08:00:03.0000 INFO - [Logger.PrintSECSMsgBody]: Encrypt S7Fy=S7F26,TID=64,Message=ABCDEF"

func readEvery(t *testing.T, log string, loc *time.Location) []*Message {
	t.Helper()
	msgs, err := ReadAll(strings.NewReader(log), loc, WithEveryEntry())
	if err != nil {
		t.Fatal(err)
	}

	return msgs
}

func TestEveryEntrySpansPartitionTheInput(t *testing.T) {
	t.Parallel()
	msgs := readEvery(t, spanLog, time.UTC)
	checkPartition(t, spanLog, msgs)
}

func TestEveryEntryKinds(t *testing.T) {
	t.Parallel()
	msgs := readEvery(t, spanLog, time.UTC)
	type want struct {
		first string // start of the first span's text
		spans int
		flags Flag
	}
	wants := []want{
		{"leading junk", 1, FlagNotMessage},
		{"2026-01-02 08:00:00.0002 INFO - [Other.Log]", 1, FlagNotMessage},
		{"2026-01-02 08:00:00.0001 INFO - [Sender.Send]", 2, 0},
		{"2026-01-02 08:00:01.0001 INFO - [Collector.Custom]", 1, FlagNotTraffic},
		{"2026-13-02", 1, FlagNotMessage},
		{"2026-01-02 08:00:03.0000 INFO - [Handler.EventHandler]", 2, FlagEncrypted},
	}
	if len(msgs) != len(wants) {
		for _, m := range msgs {
			t.Logf("got flags=%d spans=%+v", m.Flags, m.Spans)
		}
		t.Fatalf("got %d messages, want %d", len(msgs), len(wants))
	}
	for i, w := range wants {
		m := msgs[i]
		text := spanLog[m.Spans[0].Offset:]
		if !strings.HasPrefix(text, w.first) || len(m.Spans) != w.spans || m.Flags != w.flags {
			t.Errorf("message %d: first span %q, %d spans, flags %d; want %+v", i, text[:min(len(text), 40)], len(m.Spans), m.Flags, w)
		}
	}
	if !msgs[0].Spans[0].Time.IsZero() || !msgs[4].Spans[0].Time.IsZero() {
		t.Error("an entry without a parseable timestamp must have a zero time")
	}
	if got := msgs[2].Spans[1].Line; got != 5 {
		t.Errorf("body entry line = %d, want 5", got)
	}
	if msgs[2].Item == nil {
		t.Errorf("the message with CRLF line ends did not parse: %v", msgs[2].ParseErr)
	}
}

func TestBadTimestampFailsWithoutEveryEntry(t *testing.T) {
	t.Parallel()
	_, err := ReadAll(strings.NewReader(spanLog), time.UTC)
	if err == nil || !strings.Contains(err.Error(), "line 11") {
		t.Fatalf("err = %v, want an error naming line 11", err)
	}
}

func TestSystemBytesOnlyFromOwnHeader(t *testing.T) {
	t.Parallel()
	msgs, err := ReadAll(strings.NewReader(deviceLog), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	got := []int64{msgs[0].SystemBytes, msgs[1].SystemBytes, msgs[2].SystemBytes, msgs[3].SystemBytes}
	want := []int64{0x10, -1, 0x30, -1}
	if !slices.Equal(got, want) {
		t.Errorf("system bytes = %x, want %x", got, want)
	}
}

func TestLocalTimeAcrossDST(t *testing.T) {
	t.Parallel()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	cases := []struct {
		local     string
		want      time.Time
		uncertain bool
	}{
		{"2026-07-01 12:00:00.0000", time.Date(2026, 7, 1, 16, 0, 0, 0, time.UTC), false},
		// 01:30 happens twice on 2026-11-01; the earlier instant is the one in daylight time.
		{"2026-11-01 01:30:00.0000", time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC), true},
		// 02:30 does not exist on 2026-03-08; the earlier of its two readings is 02:30 in daylight time.
		{"2026-03-08 02:30:00.0000", time.Date(2026, 3, 8, 6, 30, 0, 0, time.UTC), true},
	}
	for _, c := range cases {
		got, uncertain, err := parseLocal(c.local, loc)
		if err != nil || !got.Equal(c.want) || uncertain != c.uncertain {
			t.Errorf("parseLocal(%s) = %v, %v, %v; want %v, %v", c.local, got.UTC(), uncertain, err, c.want, c.uncertain)
		}
	}
}

func FuzzEveryEntrySpans(f *testing.F) {
	f.Add(spanLog)
	f.Add(sampleLog)
	f.Add("")
	f.Add("\n\r\n")
	f.Fuzz(func(t *testing.T, log string) {
		msgs, err := ReadAll(strings.NewReader(log), time.UTC, WithEveryEntry())
		if err != nil {
			t.Skip(err) // an over-long line is the only error
		}
		checkPartition(t, log, msgs)
	})
}

// checkPartition fails t unless the spans of msgs cover [0, len(log)) exactly once.
func checkPartition(t *testing.T, log string, msgs []*Message) {
	t.Helper()
	var spans []Span
	for _, m := range msgs {
		if len(m.Spans) == 0 {
			t.Fatalf("message %s at line %d has no span", m.SxFy(), m.Line)
		}
		spans = append(spans, m.Spans...)
	}
	slices.SortFunc(spans, func(a, b Span) int { return int(a.Offset - b.Offset) })
	var next int64
	for _, s := range spans {
		if s.Offset != next || s.Len <= 0 {
			t.Fatalf("span %+v: want offset %d and a positive length", s, next)
		}
		next = s.Offset + s.Len
	}
	if next != int64(len(log)) {
		t.Fatalf("spans end at %d, input is %d bytes", next, len(log))
	}
}
