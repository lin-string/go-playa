package document

import "testing"

func TestStandardFontDifferencesWidthIgnoresToUnicode(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{
		Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Times-Bold"),
		Name("Encoding"):  Dict{Name("Differences"): Array{Number(1), Name("space")}},
		Name("ToUnicode"): newStream(nil, []byte("1 begincodespacerange <00> <ff> endcodespacerange 1 beginbfchar <01> <000a> endbfchar")),
	}}}}}
	fonts, err := d.pageFontsDirect(p)
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	count := 0
	for glyph, err := range f.DecodeGlyphsSeqWithError([]byte{1}) {
		count++
		if err != nil {
			t.Fatal(err)
		}
		if glyph.Text() != "\n" {
			t.Fatalf("decoded glyph = %q, want ToUnicode newline", glyph.Text())
		}
	}
	if count != 1 {
		t.Fatalf("decoded glyphs = %d, want 1", count)
	}
	if got, err := f.HDispWithError(1); err != nil || got != .25 {
		t.Errorf("horizontal displacement = %v, %v; want space width .25", got, err)
	}
	state := f.interpreterWidthState()
	if got := state.hDisp(1); got != .25 {
		t.Errorf("interpreter displacement = %v, want .25", got)
	}
	// Replacing the Encoding must not mutate an already-published width table.
	f.applyDifferences(Array{Number(1), Name("A")})
	if got := state.hDisp(1); got != .25 {
		t.Errorf("published displacement changed = %v", got)
	}
	if got, err := f.HDispWithError(1); err != nil || got != .722 {
		t.Errorf("updated displacement = %v, %v; want .722", got, err)
	}
}
