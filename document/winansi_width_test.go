package document

import "testing"

func TestWinAnsiSoftHyphenToUnicodeKeepsHyphenWidth(t *testing.T) {
	d := &Document{}
	font, err := d.GetFontWithError(0, Dict{
		Name("Type"): Name("Font"), Name("Subtype"): Name("Type1"),
		Name("BaseFont"): Name("Times-Roman"), Name("Encoding"): Name("WinAnsiEncoding"),
		Name("ToUnicode"): newStream(nil, []byte("1 begincodespacerange <00> <ff> endcodespacerange 1 beginbfchar <ad> <00ad> endbfchar")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := font.ToUnicodeValue(173); !ok || got != "\u00ad" {
		t.Fatalf("ToUnicode(173) = %q, %v, want soft hyphen", got, ok)
	}
	if got := font.Decode([]byte{173}); got != "\u00ad" {
		t.Fatalf("decoded text = %q, want ToUnicode soft hyphen", got)
	}
	if got := font.HDisp(173); got != 0.333 {
		t.Fatalf("HDisp(173) = %v, want hyphen width 0.333", got)
	}
	if got := font.interpreterWidthState().hDisp(173); got != 0.333 {
		t.Fatalf("interpreter HDisp(173) = %v, want hyphen width 0.333", got)
	}
}
