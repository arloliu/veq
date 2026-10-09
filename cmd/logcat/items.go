package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arloliu/go-secs/v2/secs2"
)

// scalar renders a single-value item as its plain value, and anything else as compact SML.
func scalar(it secs2.Item) string {
	if s, err := it.ToASCII(); err == nil {
		return s
	}
	if b, err := it.ToBinary(); err == nil && len(b) > 0 {
		parts := make([]string, len(b))
		for i, x := range b {
			parts[i] = fmt.Sprintf("0x%02X", x)
		}

		return strings.Join(parts, " ")
	}
	if it.Size() == 1 {
		if u, err := it.ToUint(); err == nil {
			return strconv.FormatUint(u[0], 10)
		}
		if n, err := it.ToInt(); err == nil {
			return strconv.FormatInt(n[0], 10)
		}
		if f, err := it.ToFloat(); err == nil {
			return strconv.FormatFloat(f[0], 'g', 7, 64)
		}
		if b, err := it.ToBoolean(); err == nil {
			return strconv.FormatBool(b[0])
		}
	}

	return compact(it.ToSML())
}

// listAt returns the children of a list item, or nil when it is not a list.
func listAt(it secs2.Item, indices ...int) []secs2.Item {
	if it == nil {
		return nil
	}
	if len(indices) > 0 {
		var err error
		if it, err = it.Get(indices...); err != nil {
			return nil
		}
	}
	if it.Type() != secs2.ListType {
		return nil
	}
	items, _ := it.ToList()

	return items
}

// scalarAt renders the item at indices with scalar, or returns "" when there is none.
func scalarAt(it secs2.Item, indices ...int) string {
	if it == nil {
		return ""
	}
	c, err := it.Get(indices...)
	if err != nil {
		return ""
	}

	return scalar(c)
}

func compact(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 120 {
		s = s[:117] + "..."
	}

	return s
}

// leafFormat names the SML format of a non-list item, e.g. "U4", "A", "B[0]".
// A zero-length item is how the tool answers an ID it does not know (SEMI E5 S1F4, S2F14).
func leafFormat(it secs2.Item) string {
	var name string
	switch t := it.Type(); t {
	case secs2.ASCIIType:
		name = "A"
	case secs2.BinaryType:
		name = "B"
	case secs2.BooleanType:
		name = "BOOLEAN"
	case secs2.JIS8Type:
		name = "J"
	default:
		name = strings.ToUpper(t)
	}
	if it.Size() == 0 {
		name += "[0]"
	}

	return name
}

// valueFormat names the format of a variable's value;
// a list is summarized by the distinct formats of its children.
func valueFormat(it secs2.Item) string {
	if it.Type() != secs2.ListType {
		return leafFormat(it)
	}
	items, _ := it.ToList()
	if len(items) == 0 {
		return "L[0]"
	}
	kids := map[string]bool{}
	for _, c := range items {
		kids[valueFormat(c)] = true
	}

	return "L<" + strings.Join(sortedKeys(kids), "|") + ">"
}

// shape writes the full format tree of an item, e.g. "L[U4 A L[A A]]".
func shape(it secs2.Item) string {
	if it.Type() != secs2.ListType {
		return leafFormat(it)
	}
	items, _ := it.ToList()
	parts := make([]string, len(items))
	for i, c := range items {
		parts[i] = shape(c)
	}

	return "L[" + strings.Join(parts, " ") + "]"
}
