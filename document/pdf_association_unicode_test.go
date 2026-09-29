package document_test

import (
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/document"
)

func TestPDFAssociationReiwaUnicodeExtraction(t *testing.T) {
	t.Run("ToUnicode", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "unicode-reiwa-tounicode")
		defer closePDFAssociationFixture(t, doc)
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		text, err := pages[0].ExtractTextUntagged(doc, document.DefaultTextExtractionOptions())
		if err != nil {
			t.Fatal(err)
		}
		if text != "U+4EE4 U+548C 令和\nU+32FF ㋿" {
			t.Fatalf("ToUnicode text = %q", text)
		}
	})

	t.Run("ActualText", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "unicode-reiwa-actualtext")
		defer closePDFAssociationFixture(t, doc)
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions())
		if err != nil {
			t.Fatal(err)
		}
		if text != "令和㋿" {
			t.Fatalf("ActualText text = %q, want %q", text, "令和㋿")
		}
	})
}

func TestPDFAssociationUTF16LETextStringsRemainPDFDocEncoding(t *testing.T) {
	doc := openPDFAssociationFixture(t, "utf16le-text-strings")
	defer closePDFAssociationFixture(t, doc)
	outline, err := doc.CollectOutline()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(outline))
	for index := range outline {
		got[index] = outline[index].Title()
	}
	want := []string{"ÿþ0\"P2J$", "ÿþ0\"4'P2", "ÿþP2eQJ$"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UTF-16LE outline titles = %q, want PDFDocEncoding %q", got, want)
	}
}
