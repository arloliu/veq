package eapsml

import (
	"strings"
	"testing"
	"time"
)

// sampleLog is synthetic; it mimics the layout of a real EAP log without any real data.
const sampleLog = `2026-01-02 08:00:00.0001 INFO - [Sender.Send]: Send Msg: [S1F3] /Header=[00-01-81-03-00-00-00-00-00-10] TID=16 APPID=EAP
2026-01-02 08:00:00.0001 INFO - [Logger.PrintSECSMsgBody]: S1F3_QueryState: 'S1F3' W
    <L [2]
        <U4 [1] 1001 >
        <U4 [1] 999 >
     >
.
2026-01-02 08:00:00.0002 INFO - [Other.Log]: unrelated line
2026-01-02 08:00:00.0100 INFO - [Handler.EventHandler]: Received S1F4 /Header=[00-01-01-04-00-00-00-00-00-11] TID=16
2026-01-02 08:00:00.0100 INFO - [Logger.PrintSECSMsgBody]: S1F3_QueryState: 'S1F4'
    <L [2]
        <B [1] 0x05 >
        <B [0] >
     >
.
2026-01-02 08:00:01.0000 INFO - [Handler.EventHandler]: Received S6F11 /Header=[00-00-06-0B-00-00-00-00-00-20] TID=32
2026-01-02 08:00:01.0000 INFO - [Logger.PrintSECSMsgBody]: : 'S6F11'
    <L [3]
        <U4 [1] 1 >
        <U4 [1] 300080 >
        <L [0] >
     >
.
2026-01-02 08:00:01.0001 INFO - [Collector.Custom]: S6F11_Internal: 'S6F11'
    <L [0] >
.
2026-01-02 08:00:02.0000 INFO - [Sender.Reply]: Send Msg: [S6F6] /Header=[00-01-86-05-00-00-00-00-00-30] TID=48
2026-01-02 08:00:02.0000 INFO - [Sender.Reply]: Grant: 'S6F6'
    <B [1] 0x00 >
.
2026-01-02 08:00:03.0000 INFO - [Handler.EventHandler]: Received S7F26 /Header=[00-01-07-1A-00-00-00-00-00-40] TID=64
2026-01-02 08:00:03.0000 INFO - [Logger.PrintSECSMsgBody]: Encrypt S7Fy=S7F26,TID=64,Message=ABCDEF
2026-01-02 08:00:04.0000 INFO - [Sender.Send]: Send Msg: [S1F1] /Header=[00-04-81-01-00-00-00-00-00-50] TID=80
2026-01-02 08:00:04.0000 INFO - [Logger.PrintSECSMsgBody]: S1F1: 'S1F1' W
    <A [2] 'hi' >
    <A [2] 'yo' >
.
2026-01-02 08:00:05.0000 INFO - [Sender.Send]: Send Msg: [S2F13] /Header=[00-01-82-0D-00-00-00-00-00-60] TID=96
`

func TestReadAll(t *testing.T) {
	msgs, err := ReadAll(strings.NewReader(sampleLog), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		sxfy  string
		dir   Direction
		tid   uint32
		wbit  bool
		name  string
		flags Flag
		item  bool
	}
	wants := []want{
		{"S1F3", HostToEquip, 16, true, "S1F3_QueryState", 0, true},
		{"S1F4", EquipToHost, 16, false, "S1F3_QueryState", 0, true},
		{"S6F11", EquipToHost, 32, false, "", 0, true},
		{"S6F6", HostToEquip, 48, false, "Grant", 0, true},
		{"S7F26", EquipToHost, 64, false, "", FlagEncrypted, false},
		{"S1F1", HostToEquip, 80, true, "S1F1", FlagParseFailed, false},
		{"S2F13", HostToEquip, 96, true, "", FlagBodyMissing, false},
	}
	if len(msgs) != len(wants) {
		for _, m := range msgs {
			t.Logf("got %s %s line %d flags %d", m.Dir, m.SxFy(), m.Line, m.Flags)
		}
		t.Fatalf("got %d messages, want %d", len(msgs), len(wants))
	}
	for i, w := range wants {
		m := msgs[i]
		if m.SxFy() != w.sxfy || m.Dir != w.dir || m.TID != w.tid || m.WBit != w.wbit || m.Name != w.name ||
			m.Flags != w.flags || (m.Item != nil) != w.item {
			t.Errorf("message %d = %s %s tid=%d w=%v name=%q flags=%d item=%v, want %+v",
				i, m.Dir, m.SxFy(), m.TID, m.WBit, m.Name, m.Flags, m.Item != nil, w)
		}
	}
	if got := msgs[0].Time; !got.Equal(time.Date(2026, 1, 2, 8, 0, 0, 100000, time.UTC)) {
		t.Errorf("time = %v", got)
	}
	if !strings.Contains(msgs[5].Body, "'yo'") {
		t.Errorf("raw body of the unparsable message not kept: %q", msgs[5].Body)
	}
}

func TestCorrelator(t *testing.T) {
	msgs, err := ReadAll(strings.NewReader(sampleLog), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	c := NewCorrelator()
	var txs []*Transaction
	for _, m := range msgs {
		if tx := c.Add(m); tx != nil {
			txs = append(txs, tx)
		}
	}
	// S1F3->S1F4 pairs; S6F6 and S7F26 have no logged primary; S1F1 and S2F13 stay unreplied.
	if len(txs) != 3 {
		t.Fatalf("got %d transactions, want 3", len(txs))
	}
	if txs[0].Primary == nil || txs[0].Primary.SxFy() != "S1F3" || txs[0].Reply.SxFy() != "S1F4" {
		t.Errorf("first transaction = %+v", txs[0])
	}
	if txs[1].Primary != nil || txs[1].Reply.SxFy() != "S6F6" {
		t.Errorf("S6F6 should be an orphan reply, got %+v", txs[1])
	}
	if txs[2].Primary != nil || txs[2].Reply.SxFy() != "S7F26" {
		t.Errorf("S7F26 should be an orphan reply, got %+v", txs[2])
	}
	if n := len(c.Unreplied()); n != 2 {
		t.Errorf("unreplied = %d, want 2", n)
	}
}

// deviceLog is synthetic; it mimics header lines with and without the DEVICEID token.
const deviceLog = `2026-01-02 08:00:00.0000 INFO - [Handler.EventHandler]: Received S1F2 /Header=[80-00-01-02-00-00-00-00-00-10] TID=16 DEVICEID=4 FORMAT=RAW
2026-01-02 08:00:00.0000 INFO - [Logger.PrintSECSMsgBody]: : 'S1F2'
    <L [0] >
.
2026-01-02 08:00:01.0000 INFO - [Sender.Reply]: Send Msg: [S6F6] /Header=[00-01-86-05-00-00-00-00-00-20] TID=32
2026-01-02 08:00:01.0000 INFO - [Sender.Reply]: Grant: 'S6F6'
    <B [1] 0x00 >
.
2026-01-02 08:00:02.0000 INFO - [Handler.EventHandler]: Received S6F11 /Header=[00-01-06-0B-00-00-00-00-00-30] TID=48
2026-01-02 08:00:02.0000 INFO - [Logger.PrintSECSMsgBody]: : 'S6F11'
    <L [0] >
.
2026-01-02 08:00:03.0000 INFO - [Logger.PrintSECSMsgBody]: Orphan: 'S1F1' W
.
`

func TestReadDevice(t *testing.T) {
	msgs, err := ReadAll(strings.NewReader(deviceLog), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	wants := []struct {
		sxfy      string
		sessionID int
		device    int
	}{
		{"S1F2", 0x8000, 4}, // DEVICEID token logged; the session ID is kept raw
		{"S6F6", -1, -1},    // header names the primary S6F5, so it is not this message's
		{"S6F11", 1, -1},    // own header, no token
		{"S1F1", -1, -1},    // no header line
	}
	if len(msgs) != len(wants) {
		t.Fatalf("got %d messages, want %d", len(msgs), len(wants))
	}
	for i, w := range wants {
		m := msgs[i]
		if m.SxFy() != w.sxfy || m.SessionID != w.sessionID || m.Device != w.device {
			t.Errorf("message %d = %s session=%d device=%d, want %+v", i, m.SxFy(), m.SessionID, m.Device, w)
		}
	}
}

// TestHeaderWIsNotParsed keeps an invalid W from the header out of the SML parse:
// the body parses, and the W stays on the message for the builder to reject.
func TestHeaderWIsNotParsed(t *testing.T) {
	const log = "2026-01-02 08:00:00.0000 INFO - [Sender.Reply]: Send Msg: [S6F12] /Header=[00-01-86-0C-00-00-00-00-00-50] TID=80\n" +
		"2026-01-02 08:00:00.0000 INFO - [Sender.Reply]: Ack: 'S6F12'\n" +
		"    <B [1] 0x00 >\n.\n"
	msgs, err := ReadAll(strings.NewReader(log), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Item == nil || !msgs[0].WBit || msgs[0].Flags != 0 {
		t.Fatalf("got %+v, want one parsed S6F12 with W", msgs[0])
	}
}
