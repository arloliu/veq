package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arloliu/go-secs/tracepack"
)

// twoToolLog is synthetic: two equipment connections (DEVICEID 1 and 2) and lines of neither.
const twoToolLog = "2026-01-02 08:59:59.0000 INFO - [Main.Start]: EAP scenario started\n" +
	// Connection 1: a primary and its reply, whose header names the primary.
	"2026-01-02 08:59:59.5000 INFO - [Sender.Send]: Send Msg: [S1F3] /Header=[00-01-81-03-00-00-00-00-00-10] TID=16 DEVICEID=1\n" +
	"2026-01-02 08:59:59.5000 INFO - [Logger.PrintSECSMsgBody]: S1F3: 'S1F3' W\n" +
	"    <L [1]\n        <U4 [1] 1001 >\n     >\n.\n" +
	"2026-01-02 09:00:00.1000 INFO - [Handler.EventHandler]: Received S1F4 /Header=[00-01-81-03-00-00-00-00-00-10] TID=16 DEVICEID=1\n" +
	"2026-01-02 09:00:00.1000 INFO - [Logger.PrintSECSMsgBody]: S1F3: 'S1F4'\n" +
	"    <L [1]\n        <U4 [1] 7 >\n     >\n.\n" +
	// Connection 2: an event, an encrypted body, a body that does not parse.
	"2026-01-02 09:00:01.0000 INFO - [Handler.EventHandler]: Received S6F11 /Header=[00-02-06-0B-00-00-00-00-00-20] TID=32 DEVICEID=2\n" +
	"2026-01-02 09:00:01.0000 INFO - [Logger.PrintSECSMsgBody]: : 'S6F11'\n" +
	"    <L [0] >\n.\n" +
	"2026-01-02 09:00:02.0000 INFO - [Handler.EventHandler]: Received S7F26 /Header=[00-02-07-1A-00-00-00-00-00-30] TID=48 DEVICEID=2\n" +
	"2026-01-02 09:00:02.0000 INFO - [Logger.PrintSECSMsgBody]: Encrypt S7Fy=S7F26,TID=48,Message=ABCDEF\n" +
	"2026-01-02 09:00:03.0000 INFO - [Sender.Send]: Send Msg: [S1F1] /Header=[00-02-81-01-00-00-00-00-00-40] TID=64 DEVICEID=2\n" +
	"2026-01-02 09:00:03.0000 INFO - [Logger.PrintSECSMsgBody]: S1F1: 'S1F1' W\n" +
	"    <A [2] 'hi' >\n    <A [2] 'yo' >\n.\n" +
	// Connection 1: a reply whose header carries W, which no reply may.
	"2026-01-02 09:00:04.0000 INFO - [Sender.Reply]: Send Msg: [S6F12] /Header=[00-01-86-0C-00-00-00-00-00-50] TID=80 DEVICEID=1\n" +
	"2026-01-02 09:00:04.0000 INFO - [Logger.PrintSECSMsgBody]: Ack: 'S6F12'\n" +
	"    <B [1] 0x00 >\n.\n" +
	// Lines of neither connection: a bus line and a message the EAP built for itself.
	"2026-01-02 09:00:05.0000 INFO - [Bus.Publish]: published to the message bus\n" +
	"2026-01-02 09:00:05.1000 INFO - [Collector.Custom]: Internal: 'S6F11'\n" +
	"    <L [0] >\n.\n"

const testInstance = "01890000-0000-7000-8000-000000000001"

func testConfig(t *testing.T, dir string) *config {
	t.Helper()
	log := filepath.Join(dir, "eap.log")
	if err := os.WriteFile(log, []byte(twoToolLog), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := tracepack.ParseUUID(testInstance)
	if err != nil {
		t.Fatal(err)
	}

	return &config{
		files: []string{log}, tools: map[string]string{"deviceid=1": "TOOL-A", "deviceid=2": "TOOL-B"},
		eapID: "EAP-1", tz: "UTC", instance: id, out: filepath.Join(dir, "out"), version: "test",
	}
}

func TestRunWritesOneCapturePerToolAndOneForTheRest(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, t.TempDir())
	res, err := run(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]captureStats{}
	var bytesCovered int64
	for _, c := range res.captures {
		got[c.toolID] = c.stats
		bytesCovered += c.stats.Bytes
	}
	if len(got) != 3 {
		t.Fatalf("captures = %v, want TOOL-A, TOOL-B and EAP-1", got)
	}
	// Every input byte is in exactly one record or annotation of exactly one capture.
	if bytesCovered != int64(len(twoToolLog)) {
		t.Errorf("captures cover %d bytes, the log is %d", bytesCovered, len(twoToolLog))
	}
	want := map[string]map[tracepack.DecodeStatus]uint64{
		"TOOL-A": {tracepack.DecodeStatusReconstructedOK: 2, tracepack.DecodeStatusBuildRejected: 2},
		"TOOL-B": {tracepack.DecodeStatusReconstructedOK: 1, tracepack.DecodeStatusParseFailed: 4},
		"EAP-1":  {tracepack.DecodeStatusNotApplicable: 2, tracepack.DecodeStatusParseFailed: 1},
	}
	for tool, w := range want {
		for st, n := range w {
			if got[tool].Status[st] != n {
				t.Errorf("%s: %s = %d, want %d (all: %v)", tool, st, got[tool].Status[st], n, got[tool].Status)
			}
		}
	}
	// TOOL-A spans 08:59 and 09:00, so it is written as two segments.
	if got["TOOL-A"].Segments != 2 {
		t.Errorf("TOOL-A segments = %d, want 2", got["TOOL-A"].Segments)
	}
}

func TestRunWritesVerifiablePacks(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, t.TempDir())
	if _, err := run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	var export strings.Builder
	packs := 0
	for _, path := range packFiles(t, cfg.out) {
		packs++
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		st, err := f.Stat()
		if err != nil {
			t.Fatal(err)
		}
		rep, err := tracepack.Verify(t.Context(), f, st.Size(), tracepack.VerifyOptions{})
		if err != nil || rep.Outcome != tracepack.OutcomeFinalizedConsistent {
			t.Errorf("%s: outcome %v, err %v", filepath.Base(path), rep.Outcome, err)
		}
		r, err := tracepack.Open(t.Context(), f, st.Size(), tracepack.ReaderOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tracepack.ExportJSONL(t.Context(), r, &export); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
	if packs != 5 {
		t.Errorf("packs = %d, want 5", packs)
	}
	for _, want := range []string{
		`"capture_method":"log"`,
		`"source_ref":["eap.log#sha256=`,
		`"kind":"data","dir":"host-to-equipment","fidelity":"reconstructed","decode_status":"reconstructed-ok"`,
		`"decode_status":"build-rejected"`,
		`"annotation_kind":"unparsed-entry"`,
		`"annotation_kind":"unrecognised-line","text":"2026-01-02 08:59:59.0000 INFO - [Main.Start]: EAP scenario started\n"`,
		`"boundary_kind":"start"`,
		`"boundary_kind":"stop"`,
	} {
		if !strings.Contains(export.String(), want) {
			t.Errorf("export lacks %s", want)
		}
	}
}

func TestRunIsReproducible(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := testConfig(t, dir)
	if _, err := run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	first := cfg.out
	cfg.out = filepath.Join(dir, "again")
	if _, err := run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	a, b := packFiles(t, first), packFiles(t, cfg.out)
	if len(a) != len(b) {
		t.Fatalf("%d packs, then %d", len(a), len(b))
	}
	for i := range a {
		ra, _ := filepath.Rel(first, a[i])
		rb, _ := filepath.Rel(cfg.out, b[i])
		ba, errA := os.ReadFile(a[i])
		bb, errB := os.ReadFile(b[i])
		if ra != rb || errA != nil || errB != nil || !bytes.Equal(ba, bb) {
			t.Errorf("pack %d differs: %s and %s", i, ra, rb)
		}
	}
}

func TestRunRefusesAnUnmappedConnection(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, t.TempDir())
	delete(cfg.tools, "deviceid=2")
	_, err := run(t.Context(), cfg)
	if !errors.Is(err, errUnmapped) || !strings.Contains(err.Error(), "deviceid=2") {
		t.Fatalf("err = %v, want one naming deviceid=2", err)
	}
}

func TestRunRefusesAToolIDEqualToTheEAPs(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t, t.TempDir())
	cfg.eapID = "TOOL-B"
	if _, err := run(t.Context(), cfg); err == nil {
		t.Fatal("run accepted an EAP id equal to a tool id")
	}
}

func TestToolFlag(t *testing.T) {
	t.Parallel()
	f := toolFlags{}
	for _, ok := range []string{"deviceid=1=A", "session=4=B=C"} {
		if err := f.Set(ok); err != nil {
			t.Errorf("Set(%q) = %v", ok, err)
		}
	}
	if f["session=4"] != "B=C" {
		t.Errorf("session=4 maps to %q", f["session=4"])
	}
	for _, bad := range []string{"deviceid=1", "port=1=A", "deviceid==A"} {
		if err := f.Set(bad); err == nil {
			t.Errorf("Set(%q) accepted", bad)
		}
	}
}

// packFiles returns the committed packs under root, in path order.
func packFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".partial" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			out = append(out, p)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return out
}
