package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"veq/internal/eapsml"
)

// toolCatalog accumulates what one tool's log shows for one device.
type toolCatalog struct {
	Tool          string
	Device        string
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

func newToolCatalog(tool, device, source string) *toolCatalog {
	return &toolCatalog{
		Tool: tool, Device: device, Source: source,
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
	default:
		// Other messages carry no identifier this catalog lists.
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
	params := listAt(p.Item, 1)
	names := make([]string, 0, len(params))
	for _, cp := range params {
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
	return fmt.Sprintf("%s %s: %s..%s messages=%d flags=%v unreplied=%d orphan_replies=%d ceid=%d rptid=%d alid=%d rcmd=%d",
		tc.Tool, tc.Device, tc.First.Format(time.DateTime), tc.Last.Format(time.DateTime), sum(tc.Messages), tc.Flags,
		sum(tc.Unreplied), sum(tc.OrphanReplies), len(tc.ceids), len(tc.rptids), len(tc.alids), len(tc.rcmds))
}

// MarshalJSON writes the catalog with every identifier list sorted by ID.
func (tc *toolCatalog) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Tool          string         `json:"tool"`
		Device        string         `json:"device"`
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
		tc.Tool, tc.Device, tc.Source, tc.First, tc.Last, tc.Messages, tc.Flags, tc.Unreplied, tc.OrphanReplies,
		sortedIDs(tc.ceids), sortedIDs(tc.rptids), sortedIDs(tc.alids), sortedIDs(tc.rcmds),
	})
}

func sortedIDs(m map[string]*idCount) []*idCount {
	out := make([]*idCount, 0, len(m))
	for _, c := range m {
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b *idCount) int { return idCompare(a.ID, b.ID) })

	return out
}

// idCompare orders numeric IDs numerically and puts them before non-numeric ones.
func idCompare(a, b string) int {
	x, errA := strconv.ParseUint(a, 10, 64)
	y, errB := strconv.ParseUint(b, 10, 64)
	switch {
	case errA == nil && errB == nil:
		return cmp.Compare(x, y)
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	default:
		return strings.Compare(a, b)
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
	slices.Sort(ks)

	return ks
}
