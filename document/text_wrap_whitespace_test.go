package document

import (
	"testing"

	"github.com/lin-string/go-playa/textconfig"
)

func TestTaggedTextWrapNormalizesWhitespace(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"a\tb\nc", "a       b c"},
		{"ab\r\tc", "ab         c"},
		{"界\tx", "界       x"},
		{"a\vb\fc", "a b c"},
		{"a\u00a0b", "a\u00a0b"},
	} {
		if got := wrapTaggedText(tc.input); got != tc.want {
			t.Errorf("wrap(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte("/Span << /ActualText (a\\tb\\nc) >> BDC BT /F1 10 Tf (X) Tj ET EMC"))}}
	if got, err := p.ExtractTextTagged(d, textconfig.Options{}); err != nil || got != "a       b c" {
		t.Fatalf("tagged extraction = %q, %v; want normalized whitespace", got, err)
	}
}
