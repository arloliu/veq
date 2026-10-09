package main

import (
	"encoding/csv"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/arloliu/go-secs/secs2"

	"github.com/arloliu/veq/internal/eapsml"
)

// formatTable records, per tool and variable ID, the values a tool returned for it.
type formatTable struct {
	stats map[[2]string]*formatStat
}

type formatStat struct {
	formats    map[string]int
	values     map[string]int
	names      map[string]int // the EAP's names of the messages that queried the ID
	obs, empty int
	sets       int
	maxSize    int
	minV, maxV float64
	numeric    bool
	first      time.Time
	last       time.Time
}

func newFormatTable() *formatTable {
	return &formatTable{stats: map[[2]string]*formatStat{}}
}

func (t *formatTable) len() int { return len(t.stats) }

// addTransaction records a <L ID...> query (S1F3, S2F13) answered by a <L V...> reply of the same length.
func (t *formatTable) addTransaction(tool string, tx *eapsml.Transaction, query string) {
	if tx.Primary.SxFy() != query || tx.Primary.Item == nil || tx.Reply.Item == nil {
		return
	}
	ids, vals := listAt(tx.Primary.Item), listAt(tx.Reply.Item)
	if len(ids) != len(vals) {
		return
	}
	for i, id := range ids {
		t.observe(tool, scalar(id), vals[i], tx.Primary.Name, tx.Reply.Time, false)
	}
}

// addSet records the values of an S2F15 <L <L ECID ECV>...> sent to the tool.
func (t *formatTable) addSet(tool string, m *eapsml.Message) {
	if m.SxFy() != "S2F15" || m.Dir != eapsml.HostToEquip || m.Item == nil {
		return
	}
	for _, p := range listAt(m.Item) {
		ecv, err := p.Get(1)
		if err != nil {
			continue
		}
		t.observe(tool, scalarAt(p, 0), ecv, m.Name, m.Time, true)
	}
}

func (t *formatTable) observe(tool, id string, v secs2.Item, name string, ts time.Time, isSet bool) {
	k := [2]string{tool, id}
	s := t.stats[k]
	if s == nil {
		s = &formatStat{formats: map[string]int{}, values: map[string]int{}, names: map[string]int{},
			minV: math.Inf(1), maxV: math.Inf(-1), first: ts}
		t.stats[k] = s
	}
	s.last = ts
	if name != "" {
		s.names[name]++
	}
	if isSet {
		s.sets++
	} else {
		s.obs++
	}
	if v.Size() == 0 {
		s.empty++
	}
	s.formats[valueFormat(v)]++
	s.maxSize = max(s.maxSize, v.Size())
	s.values[scalar(v)]++
	if v.Type() == secs2.ListType {
		return
	}
	var nums []float64
	if f, err := v.ToFloat(); err == nil {
		nums = f
	} else if u, err := v.ToUint(); err == nil {
		for _, x := range u {
			nums = append(nums, float64(x))
		}
	} else if n, err := v.ToInt(); err == nil {
		for _, x := range n {
			nums = append(nums, float64(x))
		}
	}
	for _, x := range nums {
		s.numeric = true
		s.minV = math.Min(s.minV, x)
		s.maxV = math.Max(s.maxV, x)
	}
}

func (t *formatTable) writeCSV(path string, withSets bool) (err error) {
	keys := make([][2]string, 0, len(t.stats))
	for k := range t.stats {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}

		return idLess(keys[i][1], keys[j][1])
	})

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	w := csv.NewWriter(f)
	head := []string{"tool", "id", "format", "format_variants", "observations", "empty_replies", "max_size",
		"distinct_values", "min", "max", "sample_values", "query_names", "first_ts", "last_ts"}
	if withSets {
		head = append(head, "s2f15_sets")
	}
	if err := w.Write(head); err != nil {
		return err
	}
	for _, k := range keys {
		s := t.stats[k]
		fmts := byCount(s.formats)
		row := []string{k[0], k[1], fmts[0], strings.Join(fmts, "|"), strconv.Itoa(s.obs), strconv.Itoa(s.empty),
			strconv.Itoa(s.maxSize), strconv.Itoa(len(s.values)), "", "",
			strings.Join(head3(byCount(s.values)), " ; "), strings.Join(head3(byCount(s.names)), " ; "),
			s.first.Format(tsOut), s.last.Format(tsOut)}
		if s.numeric {
			row[8] = strconv.FormatFloat(s.minV, 'g', 7, 64)
			row[9] = strconv.FormatFloat(s.maxV, 'g', 7, 64)
		}
		if withSets {
			row = append(row, strconv.Itoa(s.sets))
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()

	return w.Error()
}

const tsOut = "2006-01-02 15:04:05.0000"

// byCount returns the keys of m, most frequent first, ties in key order.
func byCount(m map[string]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool {
		if m[ks[i]] != m[ks[j]] {
			return m[ks[i]] > m[ks[j]]
		}

		return ks[i] < ks[j]
	})

	return ks
}

func head3(s []string) []string {
	return s[:min(len(s), 3)]
}
