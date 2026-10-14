// Command logcat lists the GEM identifiers and data formats observable in EAP SML logs.
//
// Usage:
//
//	logcat [-out DIR] [-tz ZONE] TOOL=LOGFILE...
//
// It writes three files to DIR:
// catalog.json (per tool and device: message counts, log quality, CEIDs, RPTIDs, ALIDs, RCMDs),
// svid_formats.csv (S1F3/S1F4 paired by TID)
// and ecid_formats.csv (S2F13/S2F14 paired by TID, plus S2F15 values).
//
// One log can hold the traffic of several equipment connections.
// A message's device is the EAP's DEVICEID when the log names it,
// else the session ID of the message's own logged header;
// a reply takes its primary's device.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"veq/internal/eapsml"
)

func main() {
	out := flag.String("out", ".", "output directory")
	tz := flag.String("tz", "Local", "time zone of the log timestamps, e.g. Asia/Taipei")
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("usage: logcat [-out DIR] [-tz ZONE] TOOL=LOGFILE...")
	}
	loc, err := time.LoadLocation(*tz)
	if err != nil {
		log.Fatal(err)
	}

	svids, ecids := newFormatTable(), newFormatTable()
	var tools []*toolCatalog
	for _, arg := range flag.Args() {
		tool, path, ok := strings.Cut(arg, "=")
		if !ok {
			log.Fatalf("argument %q is not TOOL=LOGFILE", arg)
		}
		tcs, err := scanTool(tool, path, loc, svids, ecids)
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		for _, tc := range tcs {
			fmt.Println(tc.summary())
		}
		tools = append(tools, tcs...)
	}

	if err := writeJSON(filepath.Join(*out, "catalog.json"), tools); err != nil {
		log.Fatal(err)
	}
	if err := svids.writeCSV(filepath.Join(*out, "svid_formats.csv"), false); err != nil {
		log.Fatal(err)
	}
	if err := ecids.writeCSV(filepath.Join(*out, "ecid_formats.csv"), true); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("distinct (tool,device,SVID)=%d (tool,device,ECID)=%d\n", svids.len(), ecids.len())
}

// scanTool reads one log and returns one catalog per device, ordered by device.
func scanTool(tool, path string, loc *time.Location, svids, ecids *formatTable) ([]*toolCatalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	byDevice := map[string]*toolCatalog{}
	catalog := func(dev string) *toolCatalog {
		tc := byDevice[dev]
		if tc == nil {
			tc = newToolCatalog(tool, dev, path)
			byDevice[dev] = tc
		}

		return tc
	}
	corr := eapsml.NewCorrelator()
	rd := eapsml.NewReader(f, loc)
	for {
		m, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		tx := corr.Add(m)
		dev := device(m)
		if tx != nil && tx.Primary != nil {
			// A reply travels on its primary's connection, whatever its own lines say.
			dev = device(tx.Primary)
		}
		tc := catalog(dev)
		tc.addMessage(m)
		ecids.addSet(tool, dev, m)
		if tx == nil {
			continue
		}
		if tx.Primary == nil {
			tc.OrphanReplies[m.SxFy()]++
			continue
		}
		tc.addTransaction(tx)
		svids.addTransaction(tool, dev, tx, "S1F3")
		ecids.addTransaction(tool, dev, tx, "S2F13")
	}
	for _, m := range corr.Unreplied() {
		catalog(device(m)).Unreplied[m.Dir.String()+" "+m.SxFy()]++
	}

	out := make([]*toolCatalog, 0, len(byDevice))
	for _, dev := range slices.Sorted(maps.Keys(byDevice)) {
		out = append(out, byDevice[dev])
	}

	return out, nil
}

const unknownDevice = "unknown"

// device names the equipment connection of m from what its own log lines say.
// DEVICEID and session ID are separate numberings, so the label says which one it is.
func device(m *eapsml.Message) string {
	switch {
	case m.Device >= 0:
		return "deviceid=" + strconv.Itoa(m.Device)
	case m.SessionID >= 0:
		return "session=" + strconv.Itoa(m.SessionID)
	default:
		return unknownDevice
	}
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(b, '\n'), 0o600)
}
