package document_test

import (
	"math"
	"testing"

	"github.com/lin-string/go-playa/document"
)

func TestSymbolUsesAdobeGlyphListCodepoints(t *testing.T) {
	font, err := (&document.Document{}).GetFontWithError(0, document.Dict{"Subtype": document.Name("Type1"), "BaseFont": document.Name("Symbol")})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []*document.Font{font, font.Finalize()} {
		for _, tc := range []struct {
			code  byte
			text  string
			width float64
		}{{109, "\u00b5", .576}, {68, "\u2206", .612}, {87, "\u2126", .768}} {
			for repeat := 0; repeat < 2; repeat++ {
				if got := f.Decode([]byte{tc.code}); got != tc.text {
					t.Errorf("Symbol code%d=%q want%q", tc.code, got, tc.text)
				}
				if got := f.HDisp(int(tc.code)); math.Abs(got-tc.width) > 1e-12 {
					t.Errorf("Symbol code%d displacement=%v want%v", tc.code, got, tc.width)
				}
			}
		}
	}
}
