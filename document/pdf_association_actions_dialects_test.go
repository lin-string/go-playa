package document_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lin-string/go-playa/document"
)

func TestPDFAssociationLinkAnnotationAppearanceAndActionChain(t *testing.T) {
	doc := openPDFAssociationFixture(t, "link-annotation-appearance")
	defer closePDFAssociationFixture(t, doc)
	annotations := pdfaFirstPageAnnotations(t, doc)
	if len(annotations) != 1 {
		t.Fatalf("annotations = %d, want 1", len(annotations))
	}
	annotation := annotations[0]
	if annotation.Rect() != [4]float64{400, 400, 300, 300} {
		t.Fatalf("source annotation Rect = %v, want [400 400 300 300]", annotation.Rect())
	}
	if annotation.Subtype() != "Link" || !annotation.HasBorder() || annotation.Border() != [3]float64{0, 0, 8} {
		t.Fatalf("link annotation = subtype:%q border:%v present:%v", annotation.Subtype(), annotation.Border(), annotation.HasBorder())
	}
	if got := annotation.ColorCopy(); !reflect.DeepEqual(got, []float64{0, 1, 0}) {
		t.Fatalf("annotation color = %v, want [0 1 0]", got)
	}
	if got := annotation.DictCopy()[document.Name("H")]; got != document.Name("I") {
		t.Fatalf("annotation highlight mode = %#v, want /I", got)
	}
	if appearance := annotation.AppearanceCopy(); appearance[document.Name("N")] == nil {
		t.Fatalf("normal appearance = %#v, want /N entry", appearance)
	}
	if got := pdfaAnnotationActionURIs(t, annotation); !reflect.DeepEqual(got, []string{
		"https://pdfa.org",
		"https://www.wikipedia.org/",
		"https://google.com",
	}) {
		t.Fatalf("action URI chain = %q", got)
	}
}

func TestPDFAssociationURIActionTrees(t *testing.T) {
	doc := openPDFAssociationFixture(t, "uri-action-tree")
	defer closePDFAssociationFixture(t, doc)
	annotations := pdfaFirstPageAnnotations(t, doc)
	if len(annotations) != 2 {
		t.Fatalf("annotations = %d, want 2", len(annotations))
	}
	want := [][]string{
		{"https://pdfa.org", "https://www.wikipedia.org/", "https://google.com"},
		{"https://pdf-issues.pdfa.org", "https://www.darpa.mil/", "https://github.com/", "https://stackoverflow.com/"},
	}
	for i := range annotations {
		if got := pdfaAnnotationActionURIs(t, annotations[i]); !reflect.DeepEqual(got, want[i]) {
			t.Fatalf("annotation %d action URI tree = %q, want %q", i, got, want[i])
		}
	}
}

func TestPDFAssociationURIBaseAndRelativeActions(t *testing.T) {
	doc := openPDFAssociationFixture(t, "uri-actions-with-base")
	defer closePDFAssociationFixture(t, doc)
	catalog, err := doc.CatalogWithError()
	if err != nil {
		t.Fatal(err)
	}
	uri, ok := catalog[document.Name("URI")].(document.Dict)
	if !ok {
		t.Fatalf("catalog URI dictionary = %#v", catalog[document.Name("URI")])
	}
	base, ok := uri[document.Name("Base")].(document.String)
	if !ok || string(base) != "https://pdfa.org/community/" {
		t.Fatalf("catalog URI Base = %#v", uri[document.Name("Base")])
	}
	annotations := pdfaFirstPageAnnotations(t, doc)
	if len(annotations) != 2 {
		t.Fatalf("annotations = %d, want 2", len(annotations))
	}
	want := [][]string{
		{"https://www.darpa.mil"},
		{"pdf-technical-working-group", "https://www.google.com/"},
	}
	for i := range annotations {
		if got := pdfaAnnotationActionURIs(t, annotations[i]); !reflect.DeepEqual(got, want[i]) {
			t.Fatalf("annotation %d raw action URIs = %q, want %q", i, got, want[i])
		}
	}
}

func TestPDFAssociationURIIsMapAction(t *testing.T) {
	doc := openPDFAssociationFixture(t, "uri-ismap")
	defer closePDFAssociationFixture(t, doc)
	annotations := pdfaFirstPageAnnotations(t, doc)
	if len(annotations) != 1 {
		t.Fatalf("annotations = %d, want 1", len(annotations))
	}
	annotation := annotations[0]
	if annotation.URI() != "https://safedocs.pdfa.org/uri-ismap-test.html" {
		t.Fatalf("URI = %q", annotation.URI())
	}
	action, err := annotation.ActionValueCopyWithError()
	if err != nil {
		t.Fatal(err)
	}
	if action == nil || action.RawCopy()[document.Name("IsMap")] != document.Bool(true) {
		t.Fatalf("URI action = %#v, want /IsMap true", action)
	}
	if annotation.Border() != [3]float64{0, 0, 2} || !reflect.DeepEqual(annotation.ColorCopy(), []float64{1, 0, 0}) {
		t.Fatalf("link decoration = border:%v color:%v", annotation.Border(), annotation.ColorCopy())
	}
}

func TestPDFAssociationNestedFontDiscovery(t *testing.T) {
	tests := []struct {
		id   string
		want map[string]bool
	}{
		{id: "font-inside-nested-type3", want: map[string]bool{"FType3A": true, "FType3B": true, "Helvetica": false}},
		{id: "font-inside-pattern", want: map[string]bool{"Helvetica": false, "Times-Italic": false}},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			fonts, err := doc.Fonts()
			if err != nil {
				t.Fatal(err)
			}
			got := make(map[string]bool, len(fonts))
			for name, font := range fonts {
				got[name] = font.IsType3()
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("document fonts = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestPDFAssociationContentStreamDictionaries(t *testing.T) {
	for _, id := range []string{"content-stream-dictionaries", "content-stream-resource-names"} {
		t.Run(id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			ops, err := pages[0].ContentOps(doc)
			if err != nil {
				t.Fatal(err)
			}
			var inlineDict, markedDict, compatibilityDict bool
			for _, op := range ops {
				switch op.Operator() {
				case "BI":
					operands := op.OperandsCopy()
					if len(operands) == 1 {
						if stream, ok := operands[0].(document.Stream); ok {
							_, inlineDict = stream.DictCopy()[document.Name("ACME_Private")].(document.Dict)
						}
					}
				case "BDC":
					operands := op.OperandsCopy()
					if len(operands) == 2 {
						_, markedDict = operands[1].(document.Dict)
					}
				case "newoperator":
					operands := op.OperandsCopy()
					if len(operands) == 1 {
						_, compatibilityDict = operands[0].(document.Dict)
					}
				}
			}
			if !inlineDict || !markedDict || !compatibilityDict {
				t.Fatalf("direct dictionaries = inline:%v marked:%v compatibility:%v", inlineDict, markedDict, compatibilityDict)
			}
			text, err := pages[0].ExtractText(doc, document.DefaultTextExtractionOptions())
			if err != nil {
				t.Fatal(err)
			}
			for _, phrase := range []string{"Inline Image", "Inside marked content", "Inside BX/EX, after unknown operator"} {
				if !strings.Contains(text, phrase) {
					t.Fatalf("extracted text = %q, want phrase %q", text, phrase)
				}
			}
		})
	}
}

func TestPDFAssociationContentStreamRejectsIndirectReferences(t *testing.T) {
	doc := openPDFAssociationFixture(t, "content-stream-indirect-references")
	defer closePDFAssociationFixture(t, doc)
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pages[0].ContentOps(doc); err == nil {
		t.Fatal("content stream containing indirect references parsed without error")
	}
}

func TestPDFAssociationStreamAndDictionaryKindsAreNotInterchangeable(t *testing.T) {
	t.Run("stream is dictionary", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "dialect-stream-is-dict")
		defer closePDFAssociationFixture(t, doc)
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 1 {
			t.Fatalf("pages = %d, want 1", len(pages))
		}
		if _, err := pages[0].ContentOps(doc); err == nil {
			t.Fatal("page Contents dictionary was accepted as a stream")
		}
		if metadata, err := doc.MetadataXML(); err == nil || metadata != nil {
			t.Fatalf("MetadataXML = %q, err=%v; want nil and a type error", metadata, err)
		}
	})

	t.Run("dictionary is stream", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "dialect-dict-is-stream")
		defer closePDFAssociationFixture(t, doc)
		if pages, err := doc.CollectPages(); err == nil {
			t.Fatalf("CollectPages = %#v, want a page-node type error", pages)
		}
	})
}

func pdfaFirstPageAnnotations(t *testing.T, doc *document.Document) []document.Annotation {
	t.Helper()
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	annotations, err := doc.CollectAnnotations(pages[0])
	if err != nil {
		t.Fatal(err)
	}
	return annotations
}

func pdfaAnnotationActionURIs(t *testing.T, annotation document.Annotation) []string {
	t.Helper()
	action, err := annotation.ActionValueCopyWithError()
	if err != nil {
		t.Fatal(err)
	}
	if action == nil {
		t.Fatal("annotation has no action")
	}
	var uris []string
	var walk func(*document.Action)
	walk = func(action *document.Action) {
		uris = append(uris, action.URI())
		next, err := action.NextCopy()
		if err != nil {
			t.Fatal(err)
		}
		for _, child := range next {
			walk(child)
		}
	}
	walk(action)
	return uris
}
