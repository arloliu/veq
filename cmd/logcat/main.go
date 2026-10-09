// Command logcat lists the GEM identifiers and data formats observable in EAP SML logs.
//
// Usage:
//
//	logcat [-out DIR] [-tz ZONE] TOOL=LOGFILE...
//
// It writes three files to DIR:
// catalog.json (per tool: message counts, log quality, CEIDs, RPTIDs, ALIDs, RCMDs),
// svid_formats.csv (S1F3/S1F4 paired by TID)
// and ecid_formats.csv (S2F13/S2F14 paired by TID, plus S2F15 values).
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arloliu/veq/internal/eapsml"
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
	tools := make([]*toolCatalog, 0, flag.NArg())
	for _, arg := range flag.Args() {
		tool, path, ok := strings.Cut(arg, "=")
		if !ok {
			log.Fatalf("argument %q is not TOOL=LOGFILE", arg)
		}
		tc, err := scanTool(tool, path, loc, svids, ecids)
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		tools = append(tools, tc)
		fmt.Println(tc.summary())
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
	fmt.Printf("distinct (tool,SVID)=%d (tool,ECID)=%d\n", svids.len(), ecids.len())
}

func scanTool(tool, path string, loc *time.Location, svids, ecids *formatTable) (*toolCatalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	tc := newToolCatalog(tool, path)
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
		tc.addMessage(m)
		ecids.addSet(tool, m)
		tx := corr.Add(m)
		if tx == nil {
			continue
		}
		if tx.Primary == nil {
			tc.OrphanReplies[m.SxFy()]++
			continue
		}
		tc.addTransaction(tx)
		svids.addTransaction(tool, tx, "S1F3")
		ecids.addTransaction(tool, tx, "S2F13")
	}
	for _, m := range corr.Unreplied() {
		tc.Unreplied[m.Dir.String()+" "+m.SxFy()]++
	}

	return tc, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(b, '\n'), 0o600)
}
