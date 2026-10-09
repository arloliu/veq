// Command eapconv converts EAP SML logs into tracepack captures.
//
// Usage:
//
//	eapconv -out DIR -tz ZONE -instance UUID -eap ID -tool CONN=ID... LOGFILE...
//
// One run converts a set of log files, listed in order.
// It writes one capture per equipment connection found in them,
// and one more for the lines that belong to no connection, such as the EAP's own,
// as segment packs under DIR, laid out as the keys of a tracepack store.
// A connection is "deviceid=N" when the log names the EAP's DEVICEID, else "session=N" from its own logged header;
// a reply takes its primary's connection.
// Each -tool flag maps a connection to its tool id, and a connection without one fails the run.
//
// The mapping of log entries to records is docs/specs/eapsml/eapsml-dialect.md §5.
// The output depends only on the input and the flags, so a re-run over the same files writes the same bytes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/arloliu/go-secs/tracepack"
)

// errUsage reports a missing required flag or log file.
var errUsage = errors.New("-out, -tz, -instance, -eap and at least one log file are required")

// toolFlags collects repeated -tool CONN=ID flags.
type toolFlags map[string]string

func (f toolFlags) String() string { return fmt.Sprint(map[string]string(f)) }

// Set takes "deviceid=N=ID" or "session=N=ID".
func (f toolFlags) Set(v string) error {
	kind, rest, ok1 := strings.Cut(v, "=")
	num, id, ok2 := strings.Cut(rest, "=")
	if !ok1 || !ok2 || (kind != "deviceid" && kind != "session") || num == "" || id == "" {
		return fmt.Errorf("%q is not deviceid=N=ID or session=N=ID", v)
	}
	f[kind+"="+num] = id

	return nil
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	res, err := run(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	report(res)
}

func parseFlags(args []string) (*config, error) {
	fs := flag.NewFlagSet("eapconv", flag.ContinueOnError)
	tools := toolFlags{}
	fs.Var(tools, "tool", "connection and its tool id, deviceid=N=ID or session=N=ID; repeat for each connection")
	out := fs.String("out", "", "output directory, the root of the segment keys")
	tz := fs.String("tz", "", "IANA time zone of the log timestamps, e.g. Asia/Taipei")
	instance := fs.String("instance", "", "this converter deployment's recorder_instance_id, a UUID kept across runs")
	eap := fs.String("eap", "", "tool id of the capture of lines that belong to no connection, the EAP's identity")
	now := fs.String("now", "", "writer start time, RFC 3339; default the time of the last log entry")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if *out == "" || *tz == "" || *instance == "" || *eap == "" || fs.NArg() == 0 {
		fs.Usage()
		return nil, errUsage
	}
	id, err := tracepack.ParseUUID(*instance)
	if err != nil {
		return nil, fmt.Errorf("-instance: %w", err)
	}
	cfg := &config{
		files: fs.Args(), tools: tools, eapID: *eap, tz: *tz, instance: id, out: *out, version: version(),
	}
	if *now != "" {
		if cfg.now, err = time.Parse(time.RFC3339Nano, *now); err != nil {
			return nil, fmt.Errorf("-now: %w", err)
		}
	}

	return cfg, nil
}

// report prints each capture's counts, which the tracepack storage specification §7 asks a converter to report.
func report(res *result) {
	if res.survey.unassigned > 0 {
		fmt.Printf("messages of no known connection, kept in the EAP's capture: %d\n", res.survey.unassigned)
	}
	for _, c := range res.captures {
		s := c.stats
		parts := make([]string, 0, len(s.Status))
		for _, st := range slices.Sorted(maps.Keys(s.Status)) {
			parts = append(parts, fmt.Sprintf("%s=%d", st, s.Status[st]))
		}
		fmt.Printf("%s capture %s: %d records in %d segments, %d source bytes; %s\n",
			c.toolID, c.id, s.Records, s.Segments, s.Bytes, strings.Join(parts, " "))
	}
}

// version returns the converter's module version,
// else the VCS revision it was built from, suffixed "-dirty" for a modified tree, else "devel".
// It enters every derived identifier, so two converters that may write different bytes never share an identifier.
// A modified tree hashes as its base revision, which is enough to tell released converters apart, not work in progress.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "devel"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value[:min(len(s.Value), 12)]
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		default:
		}
	}
	if rev == "" {
		return "devel"
	}

	return rev + dirty
}

// writerTag returns the writer tag a tracepack SegmentWriter writes: tracepack/<version> of the module in this build.
func writerTag() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, d := range info.Deps {
			if d.Path == "github.com/arloliu/go-secs/tracepack" {
				return "tracepack/" + d.Version
			}
		}
	}

	return "tracepack/unknown"
}
