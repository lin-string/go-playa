package documentdata

import (
	"encoding/json"
	"strconv"
	"strings"
)

// PageLabelSpec is the dependency-free value model for one PDF page-label
// range. Its fields are immutable outside this package.
type PageLabelSpec struct {
	style  string
	prefix string
	start  int
}

// NewPageLabelSpec constructs a page-label range. An omitted or zero start is
// interpreted as one by Format, matching PDF page-label defaults.
func NewPageLabelSpec(style, prefix string, start ...int) PageLabelSpec {
	value := 0
	if len(start) > 0 {
		value = start[0]
	}
	return PageLabelSpec{style: style, prefix: prefix, start: value}
}

// Style returns the numbering style for this page-label range.
func (s PageLabelSpec) Style() string { return s.style }

// Prefix returns the literal prefix for this page-label range.
func (s PageLabelSpec) Prefix() string { return s.prefix }

// Start returns the configured first number, or zero when omitted in the PDF.
func (s PageLabelSpec) Start() int { return s.start }

// Format renders the label for a zero-based offset within this label range.
// PDF label ranges default their first number to one when Start is omitted.
func (s PageLabelSpec) Format(offset int) string {
	if offset < 0 {
		return ""
	}
	start := s.start
	if start == 0 {
		start = 1
	}
	return s.prefix + formatPageLabel(s.style, start+offset)
}

func (s PageLabelSpec) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Style  string
		Prefix string
		Start  int
	}{s.style, s.prefix, s.start})
}

func formatPageLabel(style string, n int) string {
	switch style {
	case "D":
		return strconv.Itoa(n)
	case "R":
		return romanPageLabel(n, true)
	case "r":
		return romanPageLabel(n, false)
	case "A":
		return alphaPageLabel(n, true)
	case "a":
		return alphaPageLabel(n, false)
	default:
		return ""
	}
}

func alphaPageLabel(n int, upper bool) string {
	if n < 1 {
		return ""
	}
	out := ""
	for n > 0 {
		n--
		out = string(byte('a'+n%26)) + out
		n /= 26
	}
	if upper {
		return strings.ToUpper(out)
	}
	return out
}

func romanPageLabel(n int, upper bool) string {
	values := []struct {
		value int
		text  string
	}{{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"}, {100, "c"}, {90, "xc"}, {50, "l"}, {40, "xl"}, {10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"}}
	out := ""
	for _, item := range values {
		for n >= item.value {
			out += item.text
			n -= item.value
		}
	}
	if upper {
		return strings.ToUpper(out)
	}
	return out
}
