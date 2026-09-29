package document_test

import (
	"testing"

	"github.com/lin-string/go-playa/document"
)

func TestType0BaseFontUsesDescendantIdentity(t *testing.T) {
	d := &document.Document{}
	f, err := d.GetFontWithError(0, document.Dict{
		"Type": document.Name("Font"), "Subtype": document.Name("Type0"), "BaseFont": document.Name("Root-Identity-H"), "Encoding": document.Name("Identity-H"),
		"DescendantFonts": document.Array{document.Dict{"Subtype": document.Name("CIDFontType0"), "BaseFont": document.Name("Descendant"), "FontDescriptor": document.Dict{"FontName": document.Name("Descriptor")}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.BaseFont() != "Descendant" || f.Name() != "Descriptor" {
		t.Fatalf("Type0 identity = %q / %q", f.BaseFont(), f.Name())
	}
}
