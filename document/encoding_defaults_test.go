package document

import "testing"

func TestSimpleFontWithoutEncodingUsesGeneratedStandardEncoding(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	font := Dict{
		Name("Subtype"):  Name("Type1"),
		Name("BaseFont"): Name("Test"),
	}
	d.objects[Ref{Object: 1}] = font
	resources := Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): resources}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil {
		t.Fatal("font was not loaded")
	}
	if _, ok := f.encoding[0x80]; ok {
		t.Fatalf("default encoding contains undefined 0x80 slot: %q", f.encoding[0x80])
	}
	if got := f.encoding[0xa1]; got != '¡' {
		t.Fatalf("default encoding[0xa1] = %q, want exclamdown", got)
	}
}

func TestExplicitWinAnsiOverridesImplicitStandardEncoding(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	font := Dict{
		Name("Subtype"):  Name("Type1"),
		Name("BaseFont"): Name("Test"),
		Name("Encoding"): Name("WinAnsiEncoding"),
	}
	d.objects[Ref{Object: 1}] = font
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F"].encoding[0x80]; got != '€' {
		t.Fatalf("explicit WinAnsi encoding[0x80] = %q, want euro", got)
	}
}

func TestSimpleFontWithUnknownEncodingUsesPlayaFallback(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):  Name("Type1"),
		Name("BaseFont"): Name("Test"),
		Name("Encoding"): Name("UnknownEncoding"),
	}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if got := f.Decode([]byte{0x41, 0x80, 0xa1, 0xff}); got != "A¡" {
		t.Fatalf("unknown Encoding decode = %q, want Playa fallback", got)
	}
	if _, ok := f.EncodingValue(0xa1); ok {
		t.Fatal("unknown Encoding fallback leaked into EncodingValue")
	}
}
