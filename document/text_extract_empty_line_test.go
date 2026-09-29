package document

import (
	"testing"

	"github.com/lin-string/go-playa/textconfig"
)

func TestExtractTextUntaggedPreservesEmptyTextLines(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{
			Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica"),
			Name("Encoding"): Dict{Name("Differences"): Array{Number(88), Name(".notdef")}},
		}}},
		Name("Contents"): newStream(nil, []byte("BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj 1 0 0 1 10 80 Tm (X) Tj 1 0 0 1 10 60 Tm (B) Tj ET")),
	}}
	for _, tc := range []struct{ name, content, want string }{
		{"line", "BT /F1 10 Tf 1 0 0 1 10 100 Tm (A) Tj 1 0 0 1 10 80 Tm (X) Tj 1 0 0 1 10 60 Tm (B) Tj ET", "A\n\nB"},
		{"glyph gap", "BT /F1 10 Tf [(X) -1000 (X)] TJ ET", " "},
		{"object gap", "BT /F1 10 Tf 1 0 0 1 10 100 Tm (A ) Tj 1 0 0 1 40 100 Tm (X) Tj 1 0 0 1 60 100 Tm (B) Tj ET", "A  B"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page.dict[Name("Contents")] = newStream(nil, []byte(tc.content))
			got, err := page.ExtractTextUntagged(d, textconfig.Options{})
			if err != nil || got != tc.want {
				t.Fatalf("extracted=%q err=%v, want %q", got, err, tc.want)
			}
		})
	}
}
