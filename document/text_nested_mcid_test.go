package document

import "testing"

func TestTextAndGlyphMCIDUseNearestEnclosingID(t *testing.T) {
	d := &Document{}
	page := Page{dict: Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}}},
		Name("Contents"):  newStream(nil, []byte("/P << /MCID 7 >> BDC /Span << /ActualText (replacement) >> BDC BT /F1 10 Tf (x) Tj ET EMC EMC")),
	}}
	for run := 0; run < 2; run++ {
		count := 0
		for text, err := range page.Texts(d) {
			if err != nil {
				t.Fatal(err)
			}
			count++
			if !text.HasMCID() || text.MCID() != 7 {
				t.Fatalf("text MCID=%d/%v, want enclosing 7", text.MCID(), text.HasMCID())
			}
			if text.MarkedTag() != "Span" || text.ActualText() != "replacement" {
				t.Fatal("immediate marked context lost")
			}
			stack := text.MarkedStackCopy()
			if len(stack) != 2 || stack[1].HasMCID() {
				t.Fatal("inner stack context acquired an ID")
			}
		}
		if count != 1 {
			t.Fatalf("texts=%d", count)
		}
		for glyph, err := range page.Glyphs(d) {
			if err != nil {
				t.Fatal(err)
			}
			if !glyph.HasMCID() || glyph.MCID() != 7 {
				t.Fatalf("glyph MCID=%d/%v", glyph.MCID(), glyph.HasMCID())
			}
		}
	}
}
