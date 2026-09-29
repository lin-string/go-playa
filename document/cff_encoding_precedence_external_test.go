package document_test

import (
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/pdftypes"
)

func TestExplicitCFFEncodingSurvivesLazyDecodeAndFinalize(t *testing.T) {
	d := &document.Document{}
	spec := document.Dict{"Subtype": document.Name("Type1"), "BaseFont": document.Name("CFFExplicit"), "Encoding": document.Dict{"BaseEncoding": document.Name("StandardEncoding"), "Differences": document.Array{document.Number(32), document.Name("A")}}, "FontDescriptor": document.Dict{"FontFile3": pdftypes.NewStream(document.Dict{"Subtype": document.Name("Type1C")}, cffWithSpaceWidth())}}
	font, err := d.GetFontWithError(0, spec)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []*document.Font{font, font.Finalize()} {
		for repeat := 0; repeat < 2; repeat++ {
			if got := f.Decode([]byte{32}); got != "A" {
				t.Errorf("decode=%q want A", got)
			}
			if name, ok, err := f.GlyphNameWithError(32); err != nil || !ok || name != "A" {
				t.Errorf("glyph name=%q,%v,%v", name, ok, err)
			}
		}
	}
}
