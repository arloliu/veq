package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/arloliu/veq/internal/eapsml"
)

// toolCatalog accumulates what one tool's log shows.
type toolCatalog struct {
	Tool          string
	Source        string
	First, Last   time.Time
	Messages      map[string]int // "E->H S6F11" -> count
	Flags         map[string]int
	Unreplied     map[string]int // primaries with the W-bit whose reply is not in the log
	OrphanReplies map[string]int // replies whose primary is not in the log

	ceids  map[string]*idCount
	rptids map[string]*idCount
	alids  map[string]*idCount
	rcmds  map[string]*idCount
}

// idCount is one identifier with how often it appeared and the variants seen with it.
type idCount struct {
	ID       string         `json:"id"`
	Count    int            `json:"count"`
	Related  map[string]int `json:"related,omitempty"`  // CEID: RPTIDs; ALID: ALTX; RCMD: CPNAME sets
	Variants map[string]int `json:"variants,omitempty"` // RPTID: value shapes; ALID: ALCD; RCMD: HCACK
}

func newToolCatalog(tool, source string) *toolCatalog {
	return &toolCatalog{
		Tool: tool, Source: source,
		Messages: map[string]int{}, Flags: map[string]int{},
		Unreplied: map[string]int{}, OrphanReplies: map[string]int{},
		ceids: map[string]*idCount{}, rptids: map[string]*idCount{},
		alids: map[string]*idCount{}, rcmds: map[string]*idCount{},
	}
}

var flagNames = []struct {
	f    eapsml.Flag
	name string
}{
	{eapsml.FlagEncrypted, "encrypted"},
	{eapsml.FlagBodyMissing, "body_missing"},
	{eapsml.FlagHeaderMissing, "header_missing"},
	{eapsml.FlagParseFailed, "parse_failed"},
}

func (tc *toolCatalog) addMessage(m *eapsml.Message) {
	if tc.First.IsZero() || m.Time.Before(tc.First) {
		tc.First = m.Time
	}
	if m.Time.After(tc.Last) {
		tc.Last = m.Time
	}
	tc.Messages[m.Dir.String()+" "+m.SxFy()]++
	for _, fn := range flagNames {
		if m.Flags&fn.f != 0 {
			tc.Flags[fn.name]++
		}
	}
	if m.Item == nil {
		return
	}
	switch m.SxFy() {
	case "S6F11":
		// <L DATAID CEID <L <L RPTID <L V...>>...>>
		ce := bump(tc.ceids, scalarAt(m.Item, 1))
		for _, rpt := range listAt(m.Item, 2) {
			rid := scalarAt(rpt, 0)
			ce.Related[rid]++
			r := bump(tc.rptids, rid)
			if vals, err := rpt.Get(1); err == nil {
				r.Variants[compactShape(shape(vals))]++
			}
		}
	case "S5F1":
		// <L ALCD ALID ALTX>
		a := bump(tc.alids, scalarAt(m.Item, 1))
		a.Variants[scalarAt(m.Item, 0)]++
		a.Related[scalarAt(m.Item, 2)]++
	}
}

// addTransaction records what needs both sides of a transaction: the HCACK of each RCMD.
func (tc *toolCatalog) addTransaction(tx *eapsml.Transaction) {
	p := tx.Primary
	if p.SxFy() != "S2F41" || p.Item == nil {
		return
	}
	// <L RCMD <L <L CPNAME CPVAL>...>>
	r := bump(tc.rcmds, scalarAt(p.Item, 0))
	var names []string
	for _, cp := range listAt(p.Item, 1) {
		names = append(names, scalarAt(cp, 0))
	}
	r.Related[fmt.Sprint(names)]++
	if tx.Reply.Item != nil {
		r.Variants["HCACK "+scalarAt(tx.Reply.Item, 0)]++
	}
}

func bump(m map[string]*idCount, id string) *idCount {
	c := m[id]
	if c == nil {
		c = &idCount{ID: id, Related: map[string]int{}, Variants: map[string]int{}}
		m[id] = c
	}
	c.Count++

	return c
}

func compactShape(s string) string {
	if len(s) > 300 {
		return s[:297] + "..."
	}

	return s
}

func (tc *toolCatalog) summary() string {
	return fmt.Sprintf("%s: %s..%s messages=%d flags=%v unreplied=%d orphan_replies=%d ceid=%d rptid=%d alid=%d rcmd=%d",
		tc.Tool, tc.First.Format(time.DateTime), tc.Last.Format(time.DateTime), sum(tc.Messages), tc.Flags,
		sum(tc.Unreplied), sum(tc.OrphanReplies), len(tc.ceids), len(tc.rptids), len(tc.alids), len(tc.rcmds))
}

// MarshalJSON writes the catalog with every identifier list sorted by ID.
func (tc *toolCatalog) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Tool          string         `json:"tool"`
		Source        string         `json:"source"`
		First         time.Time      `json:"first"`
		Last          time.Time      `json:"last"`
		Messages      map[string]int `json:"messages"`
		Flags         map[string]int `json:"flags"`
		Unreplied     map[string]int `json:"unreplied"`
		OrphanReplies map[string]int `json:"orphan_replies"`
		CEIDs         []*idCount     `json:"ceids"`
		RPTIDs        []*idCount     `json:"rptids"`
		ALIDs         []*idCount     `json:"alids"`
		RCMDs         []*idCount     `json:"rcmds"`
	}{
		tc.Tool, tc.Source, tc.First, tc.Last, tc.Messages, tc.Flags, tc.Unreplied, tc.OrphanReplies,
		sortedIDs(tc.ceids), sortedIDs(tc.rptids), sortedIDs(tc.alids), sortedIDs(tc.rcmds),
	})
}

func sortedIDs(m map[string]*idCount) []*idCount {
	out := make([]*idCount, 0, len(m))
	for _, c := range m {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return idLess(out[i].ID, out[j].ID) })

	return out
}

// idLess orders numeric IDs numerically and puts them before non-numeric ones.
func idLess(a, b string) bool {
	x, errA := strconv.ParseUint(a, 10, 64)
	y, errB := strconv.ParseUint(b, 10, 64)
	switch {
	case errA == nil && errB == nil:
		return x < y
	case errA == nil:
		return true
	case errB == nil:
		return false
	default:
		return a < b
	}
}

func sum(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}

	return n
}

func sortedKeys(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)

	return ks
}
