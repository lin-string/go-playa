package document_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestPDFAssociationCorpusOpensAtPinnedDigests(t *testing.T) {
	manifest, err := testfixture.PDFAManifestValue()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			if fixture.ExpectedPasswordFailure {
				_, err := document.Open(testfixture.PDFAPath(t, fixture.ID), document.WithPassword(fixture.Password))
				if !errors.Is(err, document.ErrInvalidPassword) {
					t.Fatalf("open error = %v, want %v", err, document.ErrInvalidPassword)
				}
				return
			}
			doc := openPDFAssociationFixture(t, fixture.ID)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if fixture.ExpectedPageFailure {
				if err == nil {
					t.Fatalf("collect pages = %#v, want an error", pages)
				}
				return
			}
			if err != nil {
				t.Fatalf("collect pages: %v", err)
			}
			if len(pages) == 0 {
				t.Fatal("document has no pages")
			}
		})
	}
}

func TestPDFAssociationEffectiveVersions(t *testing.T) {
	tests := []struct {
		id      string
		version string
	}{
		{id: "pdf20-incremental", version: "2.0"},
		{id: "pdf20-offset-start", version: "2.0"},
		{id: "pdf-version-1", version: "1.6"},
		{id: "pdf-version-2", version: "1.6"},
		{id: "pdf-version-3", version: "1.6"},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			if got := doc.PDFVersion(); got != test.version {
				t.Fatalf("PDFVersion = %q, want %q", got, test.version)
			}
		})
	}
}

func TestPDFAssociationSimplePDF20Document(t *testing.T) {
	doc := openPDFAssociationFixture(t, "pdf20-simple")
	defer closePDFAssociationFixture(t, doc)
	if got := doc.PDFVersion(); got != "2.0" {
		t.Fatalf("PDFVersion = %q, want 2.0", got)
	}
	xmp, err := doc.MetadataXML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(xmp), "A simple PDF 2.0 example file") ||
		!strings.Contains(string(xmp), "Demonstration of a simple PDF 2.0 file.") {
		t.Fatalf("XMP metadata = %q", xmp)
	}
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	text, err := pages[0].ExtractText(doc, document.DefaultTextExtractionOptions())
	if err != nil {
		t.Fatal(err)
	}
	if text != "Hello World" {
		t.Fatalf("text = %q, want Hello World", text)
	}
	streams := 0
	for _, err := range pages[0].Streams(doc) {
		if err != nil {
			t.Fatal(err)
		}
		streams++
	}
	if streams != 2 {
		t.Fatalf("content streams = %d, want 2", streams)
	}
}

func TestPDFAssociationPDF20BlackPointCompensation(t *testing.T) {
	doc := openPDFAssociationFixture(t, "pdf20-black-point-compensation")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	states := 0
	for state, err := range pages[0].ExtGStates(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if state.GState().BlackPointComp() == "ON" && state.GState().Intent() == "Perceptual" {
			states++
		}
	}
	if states != 1 {
		t.Fatalf("black-point-compensation states = %d, want 1", states)
	}
	images := 0
	for image, err := range pages[0].Images(doc) {
		if err != nil {
			t.Fatal(err)
		}
		images++
		state := image.GState()
		if state.BlackPointComp() != "ON" || state.Intent() != "Perceptual" {
			t.Fatalf("image %s graphics state = %#v", image.Name(), state)
		}
	}
	if images != 2 {
		t.Fatalf("images = %d, want 2", images)
	}
}

func TestPDFAssociationPDF20PageLevelOutputIntent(t *testing.T) {
	doc := openPDFAssociationFixture(t, "pdf20-page-output-intent")
	defer closePDFAssociationFixture(t, doc)
	catalog := doc.Catalog()
	if got := pdfAssociationOutputIntentIdentifier(t, doc, catalog[document.Name("OutputIntents")]); got != "Adobe RGB (1998)" {
		t.Fatalf("document output intent = %q, want Adobe RGB (1998)", got)
	}
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(pages))
	}
	value, present := pages[0].Get(document.Name("OutputIntents"))
	if !present {
		t.Fatal("page 0 has no OutputIntents entry")
	}
	if got := pdfAssociationOutputIntentIdentifier(t, doc, value); got != "eciRGB" {
		t.Fatalf("page output intent = %q, want eciRGB", got)
	}
	if pages[1].Has(document.Name("OutputIntents")) {
		t.Fatal("page 1 unexpectedly has an OutputIntents entry")
	}
}

func pdfAssociationOutputIntentIdentifier(t *testing.T, doc *document.Document, value document.Object) string {
	t.Helper()
	resolved, err := doc.ResolveObject(value)
	if err != nil {
		t.Fatal(err)
	}
	intents, ok := resolved.(document.Array)
	if !ok || len(intents) != 1 {
		t.Fatalf("OutputIntents = %#v, want one-element array", resolved)
	}
	resolved, err = doc.ResolveObject(intents[0])
	if err != nil {
		t.Fatal(err)
	}
	intent, ok := resolved.(document.Dict)
	if !ok {
		t.Fatalf("output intent = %#v, want dictionary", resolved)
	}
	resolved, err = doc.ResolveObject(intent[document.Name("OutputConditionIdentifier")])
	if err != nil {
		t.Fatal(err)
	}
	identifier, ok := resolved.(document.String)
	if !ok {
		t.Fatalf("OutputConditionIdentifier = %#v, want string", resolved)
	}
	return string(identifier)
}

func TestPDFAssociationIncrementalAndBackwardXRefSelection(t *testing.T) {
	tests := []struct {
		id   string
		text string
	}{
		{id: "pdf20-incremental", text: "PDF 2.0 Words Have Spacing"},
		{id: "pdf20-offset-start", text: "This is a PDF 2.0 document"},
		{id: "dual-startxref", text: "Second startxref"},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			text, err := pages[0].ExtractText(doc, document.DefaultTextExtractionOptions())
			if err != nil {
				t.Fatal(err)
			}
			if text != test.text {
				t.Fatalf("text = %q, want %q", text, test.text)
			}
		})
	}
}

func TestPDFAssociationUTF8TextStrings(t *testing.T) {
	t.Run("outline", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "pdf20-utf8")
		defer closePDFAssociationFixture(t, doc)
		outline, err := doc.CollectOutline()
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"PDF 2.0 with UTF-8 test file", "\u202a\u202atest\u202a", "🌈️\n"}
		got := make([]string, len(outline))
		for i := range outline {
			got[i] = outline[i].Title()
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("outline titles = %q, want %q", got, want)
		}
	})
	t.Run("annotation", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "pdf20-utf8-annotation")
		defer closePDFAssociationFixture(t, doc)
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		annotations, err := doc.CollectAnnotations(pages[0])
		if err != nil {
			t.Fatal(err)
		}
		if len(annotations) != 1 || annotations[0].Contents() != "ไฮไลต์ข้อความ" {
			t.Fatalf("annotations = %#v", annotations)
		}
	})
}

func TestPDFAssociationPageLabels(t *testing.T) {
	doc := openPDFAssociationFixture(t, "page-labels")
	defer closePDFAssociationFixture(t, doc)
	got, err := doc.PageLabels()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"i", "ii", "iii", "iv",
		"1", "2", "3", "4",
		"٤", "٥", "٦", "٧",
		" Long Label - 11", " Long Label - 12", " Long Label - 13", " Long Label - 14",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PageLabels = %q, want %q", got, want)
	}
}

func TestPDFAssociationInlineImageAbbreviationsTakePrecedence(t *testing.T) {
	doc := openPDFAssociationFixture(t, "inline-image-abbreviations")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for image, err := range pages[0].Images(doc) {
		if err != nil {
			t.Fatalf("image %d: %v", count, err)
		}
		count++
		if !image.Inline() || image.Width() != 20 || image.Height() != 10 || image.BPC() != 8 || image.ColorSpace() != "DeviceRGB" {
			t.Fatalf("image %d = inline:%v %dx%d bpc:%d color:%q", count, image.Inline(), image.Width(), image.Height(), image.BPC(), image.ColorSpace())
		}
	}
	if count != 8 {
		t.Fatalf("inline image count = %d, want 8", count)
	}
}

func TestPDFAssociationType3WordSpacing(t *testing.T) {
	doc := openPDFAssociationFixture(t, "type3-word-spacing")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	wantSpaceAdvances := []float64{60, 50, 40, 30, 20, 10}
	var got []float64
	for text, err := range pages[0].Texts(doc) {
		if err != nil {
			t.Fatal(err)
		}
		for glyph, err := range text.GlyphsSeq() {
			if err != nil {
				t.Fatal(err)
			}
			if glyph.CID() == 32 {
				if glyph.Text() != " " {
					t.Fatalf("Type 3 code 32 glyph text = %q, want SPACE", glyph.Text())
				}
				got = append(got, glyph.Displacement()[0])
				break
			}
		}
	}
	if !reflect.DeepEqual(got, wantSpaceAdvances) {
		t.Fatalf("first-space advances = %v, want %v", got, wantSpaceAdvances)
	}
}

func TestPDFAssociationVerticalWritingMode(t *testing.T) {
	doc := openPDFAssociationFixture(t, "vertical-text")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	var extracted strings.Builder
	for text, err := range pages[0].Texts(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if text.Vertical() && text.Displacement()[0] == 0 && text.Displacement()[1] != 0 {
			found = true
		}
		extracted.WriteString(text.Text())
	}
	if !found {
		t.Fatal("no vertical text object with vertical displacement")
	}
	if !strings.Contains(extracted.String(), "縦書きのテスト") {
		t.Fatalf("vertical text = %q, want Japanese text", extracted.String())
	}
	fonts, err := pages[0].Fonts(doc)
	if err != nil {
		t.Fatal(err)
	}
	if fonts["English"] == nil || fonts["English"].Name() != "TimesNewRomanPSMT" || fonts["Japanese"] == nil {
		t.Fatalf("vertical fonts = %s", formatPDFAssociationFonts(fonts))
	}
}

func TestPDFAssociationUnknownFiltersAreScopedToTheirStreams(t *testing.T) {
	t.Run("optional linearization", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "unknown-filter-linearized")
		defer closePDFAssociationFixture(t, doc)
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		var text strings.Builder
		for object, err := range pages[0].Texts(doc) {
			if err != nil {
				t.Fatal(err)
			}
			text.WriteString(object.Text())
		}
		if !strings.Contains(text.String(), "Hello!") {
			t.Fatalf("page text = %q, want Hello!", text.String())
		}
	})
	t.Run("page content", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "unknown-filter-page-content")
		defer closePDFAssociationFixture(t, doc)
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		var contentErr error
		for _, err := range pages[0].Contents(doc) {
			if err != nil {
				contentErr = err
				break
			}
		}
		if contentErr == nil {
			t.Fatal("unknown page-content filter was silently treated as empty content")
		}
	})
}

func TestPDFAssociationUnknownFilterRecoveryScopes(t *testing.T) {
	assertPageTextAndImage := func(t *testing.T, id string) *document.Document {
		t.Helper()
		doc := openPDFAssociationFixture(t, id)
		t.Cleanup(func() { closePDFAssociationFixture(t, doc) })
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 1 {
			t.Fatalf("pages = %d, want 1", len(pages))
		}
		text, err := pages[0].ExtractText(doc, document.DefaultTextExtractionOptions())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(text, "Hello!") {
			t.Fatalf("page text = %q, want Hello!", text)
		}
		images := 0
		for _, err := range pages[0].Images(doc) {
			if err != nil {
				t.Fatal(err)
			}
			images++
		}
		if images != 1 {
			t.Fatalf("images = %d, want 1", images)
		}
		return doc
	}

	for _, id := range []string{
		"unknown-filter-xref-stream",
		"unknown-filter-object-stream",
		"unknown-filter-icc",
	} {
		t.Run(id, func(t *testing.T) {
			assertPageTextAndImage(t, id)
		})
	}

	t.Run("unknown-filter-outline-object-stream", func(t *testing.T) {
		doc := assertPageTextAndImage(t, "unknown-filter-outline-object-stream")
		outline, err := doc.CollectOutline()
		if err != nil {
			t.Fatal(err)
		}
		if len(outline) != 0 {
			t.Fatalf("outline = %#v, want unavailable optional outline to be skipped", outline)
		}
	})
}

func TestPDFAssociationCompactedSyntax(t *testing.T) {
	doc := openPDFAssociationFixture(t, "compacted-syntax")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, err := range pages[0].Contents(doc) {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count < 20 {
		t.Fatalf("content operation count = %d, want at least 20", count)
	}
}

func TestPDFAssociationPageWithoutContentsIsEmptyAndRepeatable(t *testing.T) {
	doc := openPDFAssociationFixture(t, "no-page-contents")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		count := 0
		for _, err := range pages[0].Contents(doc) {
			if err != nil {
				t.Fatal(err)
			}
			count++
		}
		if count != 0 {
			t.Fatalf("pass %d content operation count = %d, want 0", pass, count)
		}
	}
}

func TestPDFAssociationNestedType3Fonts(t *testing.T) {
	t.Run("valid nesting", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "type3-content-no-cycle")
		defer closePDFAssociationFixture(t, doc)
		fonts, err := doc.Fonts()
		if err != nil {
			t.Fatal(err)
		}
		if len(fonts) != 3 {
			t.Fatalf("font count = %d, want 3", len(fonts))
		}
	})
	t.Run("cycle terminates", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "type3-content-cycle")
		done := make(chan error, 1)
		go func() {
			_, err := doc.Fonts()
			done <- err
		}()
		select {
		case <-done:
			closePDFAssociationFixture(t, doc)
		case <-time.After(2 * time.Second):
			t.Fatal("font traversal did not terminate within 2 seconds")
		}
	})
}

func openPDFAssociationFixture(t *testing.T, id string) *document.Document {
	t.Helper()
	fixture, err := testfixture.PDFAFixtureByID(id)
	if err != nil {
		t.Fatalf("PDF Association fixture %q: %v", id, err)
	}
	path := testfixture.PDFAPath(t, id)
	var doc *document.Document
	if fixture.Password != "" {
		doc, err = document.Open(path, document.WithPassword(fixture.Password))
	} else {
		doc, err = document.Open(path)
	}
	if err != nil {
		t.Fatalf("open PDF Association fixture %q: %v", id, err)
	}
	return doc
}

func closePDFAssociationFixture(t *testing.T, doc *document.Document) {
	t.Helper()
	if err := doc.Close(); err != nil {
		t.Errorf("close PDF Association fixture: %v", err)
	}
}

func formatPDFAssociationFonts(fonts map[string]*document.Font) string {
	names := make([]string, 0, len(fonts))
	for name, font := range fonts {
		names = append(names, fmt.Sprintf("%s:%s", name, font.Subtype()))
	}
	return strings.Join(names, ",")
}
