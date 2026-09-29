package document

import (
	"os"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestSemanticAcceptanceCorpusKeepsDomainContractsTogether(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		password         string
		text             string
		glyphs           int
		textParts        []string
		glyphsPerPart    []int
		glyphChildCounts [][]int
		vertical         bool
		wantStructure    bool
		wantForm         bool
		wantImage        bool
		wantNavigation   bool
	}{
		{name: "tagged", path: testfixture.Path(t, "acceptance_tagged_text.pdf"), text: "AB", glyphs: 2, wantStructure: true},
		{name: "cjk", path: testfixture.Path(t, "acceptance_cjk_cid.pdf"), text: "中国", glyphs: 2},
		{name: "type3", path: testfixture.Path(t, "acceptance_type3_glyph.pdf"), textParts: []string{"Quarterly financial review", "A"}, glyphsPerPart: []int{26, 1}, glyphChildCounts: [][]int{nil, {2}}},
		{name: "wide-tounicode", path: testfixture.Path(t, "acceptance_wide_tounicode.pdf"), textParts: []string{"Annual report: wide-code font compatibility", "A😀中𐀃B"}, glyphsPerPart: []int{43, 5}},
		{name: "vertical-cid", path: testfixture.Path(t, "acceptance_vertical_cid.pdf"), text: "AB", glyphs: 2, vertical: true},
		{name: "encrypted", path: testfixture.Path(t, "acceptance_encrypted_r2.pdf"), password: "secret", text: "Encrypted report", glyphs: 16},
		{name: "form", path: testfixture.Path(t, "acceptance_form_choice.pdf"), text: "Annual report selection", glyphs: 23, wantForm: true},
		{name: "image", path: testfixture.Path(t, "acceptance_rgb_image.pdf"), wantImage: true},
		{name: "navigation", path: testfixture.Path(t, "acceptance_navigation_semantics.pdf"), text: "Quarterly report", glyphs: 16, wantNavigation: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile(test.path)
			if err != nil {
				t.Fatal(err)
			}
			var document *Document
			if test.password == "" {
				document, err = OpenBytes(data)
			} else {
				document, err = OpenBytes(data, WithPassword(test.password))
			}
			if err != nil {
				t.Fatal(err)
			}

			pages, err := document.CollectPages()
			if err != nil || len(pages) != 1 {
				t.Fatalf("pages = %#v, err=%v", pages, err)
			}
			page := pages[0]
			assertPageSequenceRepeatable(t, document, page)

			texts, err := document.PageText(page)
			if err != nil {
				t.Fatal(err)
			}
			if test.text == "" && len(test.textParts) == 0 {
				if len(texts) != 0 {
					t.Fatalf("unexpected text objects = %#v", texts)
				}
			} else {
				expectedParts := test.textParts
				if len(expectedParts) == 0 {
					expectedParts = []string{test.text}
				}
				if len(texts) != len(expectedParts) {
					t.Fatalf("text objects = %#v, want %q", texts, expectedParts)
				}
				for index, expected := range expectedParts {
					if texts[index].Text() != expected {
						t.Fatalf("text object %d = %q, want %q", index, texts[index].Text(), expected)
					}
					wantGlyphs := test.glyphs
					if len(test.glyphsPerPart) > 0 {
						wantGlyphs = test.glyphsPerPart[index]
					}
					if got := texts[index].GlyphsCopy(); len(got) != wantGlyphs {
						t.Fatalf("text object %d glyph count = %d, want %d", index, len(got), wantGlyphs)
					}
					if index < len(test.glyphChildCounts) && len(test.glyphChildCounts[index]) > 0 {
						glyphs := texts[index].GlyphsCopy()
						if len(glyphs) != len(test.glyphChildCounts[index]) {
							t.Fatalf("text object %d glyph count = %d, want %d child-count probes", index, len(glyphs), len(test.glyphChildCounts[index]))
						}
						for glyphIndex, wantChildren := range test.glyphChildCounts[index] {
							gotChildren, childErr := glyphs[glyphIndex].Len(document)
							if childErr != nil || gotChildren != wantChildren {
								t.Fatalf("glyph %d child count = %d, err=%v, want %d", glyphIndex, gotChildren, childErr, wantChildren)
							}
						}
					}
					if texts[index].Vertical() != test.vertical {
						t.Fatalf("vertical = %v, want %v", texts[index].Vertical(), test.vertical)
					}
					if _, err := texts[index].GlyphsCopyWithError(); err != nil {
						t.Fatalf("strict glyph snapshot = %v", err)
					}
					if _, err := texts[index].FontCopyWithError(); err != nil {
						t.Fatalf("strict font snapshot = %v", err)
					}
				}
			}

			if test.wantStructure {
				structure, err := document.StructureTree()
				if err != nil || len(structure) != 1 {
					t.Fatalf("structure = %#v, err=%v", structure, err)
				}
				if _, err := document.StructureTreeIndex(); err != nil {
					t.Fatalf("structure index = %v", err)
				}
				if _, err := texts[0].ParentWithError(document); err != nil {
					t.Fatalf("text parent = %v", err)
				}
			}

			if test.wantForm {
				fields, err := document.CollectFormFields()
				if err != nil || len(fields) != 1 {
					t.Fatalf("form fields = %#v, err=%v", fields, err)
				}
				if _, err := fields[0].KidsCopy(); err != nil {
					t.Fatalf("form kids = %v", err)
				}
				if _, err := fields[0].FinalizeWithError(); err != nil {
					t.Fatalf("form snapshot = %v", err)
				}
			}

			if test.wantImage {
				images, err := collectPageImages(document, page)
				if err != nil || len(images) != 1 {
					t.Fatalf("images = %#v, err=%v", images, err)
				}
				if _, _, err := images[0].SamplesWithError(); err != nil {
					t.Fatalf("image samples = %v", err)
				}
				if _, err := images[0].FinalizeWithError(); err != nil {
					t.Fatalf("image snapshot = %v", err)
				}
			}

			if test.wantNavigation {
				metadata, err := document.MetadataXML()
				if err != nil || string(metadata) != "<x:xmpmeta><dc:title>Quarterly report</dc:title></x:xmpmeta>" {
					t.Fatalf("navigation metadata = %q, err=%v", metadata, err)
				}
				labels, err := document.PageLabels()
				if err != nil || len(labels) != 1 || labels[0] != "Section 3" {
					t.Fatalf("navigation labels = %#v, err=%v", labels, err)
				}
				destinations, err := document.Destinations()
				if err != nil || len(destinations) != 1 {
					t.Fatalf("navigation destinations = %#v, err=%v", destinations, err)
				}
				action, err := document.OpenActionWithError()
				if err != nil || action == nil || action.Kind() != "GoTo" {
					t.Fatalf("navigation action = %#v, err=%v", action, err)
				}
				outlines := make([]OutlineNode, 0, 1)
				for node, itemErr := range document.Outline() {
					if itemErr != nil {
						t.Fatal(itemErr)
					}
					outlines = append(outlines, node)
				}
				if len(outlines) != 1 || outlines[0].Title() != "Chapter 1" {
					t.Fatalf("navigation outlines = %#v", outlines)
				}
				annotations, err := document.CollectAnnotations(page)
				if err != nil || len(annotations) != 4 || annotations[0].ActionKind() != "URI" || annotations[0].URI() != "https://example.com/report" {
					t.Fatalf("navigation annotations = %#v, err=%v", annotations, err)
				}
			}

			document.ReleaseTransientCaches()
			textsAfterRelease, err := document.PageText(page)
			if err != nil || len(textsAfterRelease) != len(texts) {
				t.Fatalf("text after cache release = %#v, err=%v", textsAfterRelease, err)
			}
		})
	}
}

func assertPageSequenceRepeatable(t *testing.T, document *Document, page Page) {
	t.Helper()
	for pass := 0; pass < 2; pass++ {
		count := 0
		for object, err := range page.Contents(document) {
			if err != nil {
				t.Fatal(err)
			}
			if object.Operator() == "" {
				t.Fatalf("empty content object on pass %d", pass)
			}
			count++
		}
		if count == 0 {
			t.Fatalf("content sequence pass %d was empty", pass)
		}
	}
}

func collectPageImages(document *Document, page Page) ([]ImageObject, error) {
	var images []ImageObject
	for image, err := range page.Images(document) {
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, nil
}

func TestSemanticAcceptanceCorpusNamesRemainStable(t *testing.T) {
	paths := []string{
		"acceptance_cjk_cid.pdf",
		"acceptance_encrypted_r2.pdf",
		"acceptance_form_choice.pdf",
		"acceptance_rgb_image.pdf",
		"acceptance_tagged_text.pdf",
		"acceptance_vertical_cid.pdf",
		"acceptance_wide_tounicode.pdf",
		"acceptance_resource_semantics.pdf",
		"acceptance_visual_semantics.pdf",
	}
	for _, name := range paths {
		if _, err := os.Stat(testfixture.Path(t, name)); err != nil {
			t.Fatalf("acceptance fixture %q: %v", name, err)
		}
	}
}
