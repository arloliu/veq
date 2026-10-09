package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// partitionLog is synthetic: a primary named by DEVICEID, its reply without the token,
// and a message from another connection named only by its header's session ID.
const partitionLog = `2026-01-02 08:00:00.0000 INFO - [Handler.EventHandler]: Received S6F5 /Header=[00-00-86-05-00-00-00-00-00-10] TID=16 DEVICEID=2 FORMAT=RAW
2026-01-02 08:00:00.0000 INFO - [Logger.PrintSECSMsgBody]: : 'S6F5' W
    <L [0] >
.
2026-01-02 08:00:00.0010 INFO - [Sender.Reply]: Send Msg: [S6F6] /Header=[00-00-06-06-00-00-00-00-00-10] TID=16
2026-01-02 08:00:00.0010 INFO - [Sender.Reply]: Grant: 'S6F6'
    <B [1] 0x00 >
.
2026-01-02 08:00:01.0000 INFO - [Sender.Send]: Send Msg: [S1F1] /Header=[00-07-81-01-00-00-00-00-00-20] TID=32
2026-01-02 08:00:01.0000 INFO - [Logger.PrintSECSMsgBody]: S1F1: 'S1F1' W
.
`

func TestScanToolPartitionsByDevice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool.log")
	if err := os.WriteFile(path, []byte(partitionLog), 0o600); err != nil {
		t.Fatal(err)
	}
	tcs, err := scanTool("T", path, time.UTC, newFormatTable(), newFormatTable())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]int{
		"deviceid=2": {"E->H S6F5": 1, "H->E S6F6": 1}, // the reply follows its primary
		"session=7":  {"H->E S1F1": 1},
	}
	if len(tcs) != len(want) {
		t.Fatalf("got %d catalogs, want %d", len(tcs), len(want))
	}
	for _, tc := range tcs {
		w, ok := want[tc.Device]
		if !ok {
			t.Errorf("unexpected device %q", tc.Device)
			continue
		}
		for k, n := range w {
			if tc.Messages[k] != n {
				t.Errorf("%s: %s = %d, want %d", tc.Device, k, tc.Messages[k], n)
			}
		}
	}
}
