package document_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/lin-string/go-playa/document"
)

func TestPDFAssociationAccessibleRoleMapping(t *testing.T) {
	tests := []struct {
		id      string
		rawRole string
		role    string
	}{
		{id: "accessibility-role-map", rawRole: "Paragraph", role: "P"},
		{id: "accessibility-role-map-missing", rawRole: "FirstParagraph", role: "FirstParagraph"},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			elements := collectPDFAssociationStructure(t, doc)
			for _, element := range elements {
				if element.RawRole() == test.rawRole {
					if element.Role() != test.role {
						t.Fatalf("role for %q = %q, want %q", test.rawRole, element.Role(), test.role)
					}
					return
				}
			}
			t.Fatalf("raw role %q not found", test.rawRole)
		})
	}
}

func TestPDFAssociationTextContainerGranularity(t *testing.T) {
	const want = "This is the paragraph."
	tests := []struct {
		id            string
		containerText []string
	}{
		{id: "accessibility-text-one-container", containerText: []string{want}},
		{id: "accessibility-text-word-containers", containerText: []string{"This ", "is ", "the ", "paragraph."}},
		{id: "accessibility-text-character-containers", containerText: strings.Split(want, "")},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)

			assert := func() {
				roots, err := doc.StructureTree()
				if err != nil {
					t.Fatal(err)
				}
				if len(roots) != 1 || roots[0].Role() != "Document" {
					t.Fatalf("structure roots = %#v, want one Document", roots)
				}
				children, err := roots[0].ChildrenCopy()
				if err != nil {
					t.Fatal(err)
				}
				if len(children) != 1 || children[0].Role() != "P" {
					t.Fatalf("Document children = %#v, want one P", children)
				}
				contents, err := children[0].ContentsCopy()
				if err != nil {
					t.Fatal(err)
				}
				if len(contents) != len(test.containerText) {
					t.Fatalf("P marked-content containers = %d, want %d", len(contents), len(test.containerText))
				}
				for index, content := range contents {
					if !content.HasMCID() || content.MCID() != index {
						t.Fatalf("container %d MCID = %d/%v, want %d/true", index, content.MCID(), content.HasMCID(), index)
					}
					if text, err := content.Text(doc); err != nil || text != test.containerText[index] {
						t.Fatalf("container %d text = %q, %v; want %q", index, text, err, test.containerText[index])
					}
				}

				if text, err := children[0].Text(doc); err != nil || text != want {
					t.Fatalf("P text = %q, %v; want %q", text, err, want)
				}
				pages, err := doc.CollectPages()
				if err != nil {
					t.Fatal(err)
				}
				if text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions()); err != nil || text != want {
					t.Fatalf("tagged text = %q, %v; want %q", text, err, want)
				}
				if text, err := pages[0].ExtractTextUntagged(doc, document.DefaultTextExtractionOptions()); err != nil || text != want {
					t.Fatalf("untagged text = %q, %v; want %q", text, err, want)
				}
			}
			assert()
			doc.ReleaseTransientCaches()
			assert()
		})
	}
}

func TestPDFAssociationVisibleTextWithoutExtractableCharacters(t *testing.T) {
	doc := openPDFAssociationFixture(t, "accessibility-visible-text-not-extractable")
	defer closePDFAssociationFixture(t, doc)

	assert := func() {
		if roles := rootChildRoles(t, doc); !reflect.DeepEqual(roles, []string{"P", "Figure"}) {
			t.Fatalf("Document child roles = %v, want [P Figure]", roles)
		}
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		if text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions()); err != nil || text != "FAIL" {
			t.Fatalf("tagged text = %q, %v; want FAIL", text, err)
		}
		untagged, err := pages[0].ExtractTextUntagged(doc, document.DefaultTextExtractionOptions())
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(strings.Fields(untagged), " "); got != "Page 1 FAIL" {
			t.Fatalf("untagged text = %q, want page label and failure marker only", untagged)
		}
		if strings.Contains(untagged, "Available") {
			t.Fatalf("image-only sentence was fabricated as extractable text: %q", untagged)
		}
		images := 0
		for image, err := range pages[0].Images(doc) {
			if err != nil {
				t.Fatal(err)
			}
			images++
			if image.MarkedTag() != "Figure" || !image.HasMCID() || image.MCID() != 1 {
				t.Fatalf("image tag = %q, MCID = %d/%v; want Figure, 1/true", image.MarkedTag(), image.MCID(), image.HasMCID())
			}
		}
		if images != 1 {
			t.Fatalf("image count = %d, want 1", images)
		}
	}
	assert()
	doc.ReleaseTransientCaches()
	assert()
}

func TestPDFAssociationAccessibleUnicodeMapping(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{id: "accessibility-unicode-correct", want: "⮊"},
		{id: "accessibility-unicode-missing", want: "\x01"},
		{id: "accessibility-unicode-incorrect", want: ">"},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(text, test.want) {
				t.Fatalf("tagged text = %q, want suffix %q", text, test.want)
			}
		})
	}
}

func TestPDFAssociationStructureActualTextReplacesOCR(t *testing.T) {
	doc := openPDFAssociationFixture(t, "accessibility-actualtext-ocr")
	defer closePDFAssociationFixture(t, doc)
	const want = "Available 24/7, you can book an appointment online."
	var span *document.StructElement
	for _, element := range collectPDFAssociationStructure(t, doc) {
		if element.Role() == "Span" {
			copy := element
			span = &copy
			break
		}
	}
	if span == nil {
		t.Fatal("Span structure element not found")
	}
	if span.ActualText() != want {
		t.Fatalf("Span ActualText = %q, want %q", span.ActualText(), want)
	}
	if text, err := span.Text(doc); err != nil || text != want {
		t.Fatalf("Span Text = %q, %v; want %q", text, err, want)
	}
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	if text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions()); err != nil || text != want {
		t.Fatalf("tagged text = %q, %v; want %q", text, err, want)
	}
}

func TestPDFAssociationGraphicsRepresentingText(t *testing.T) {
	t.Run("ActualText", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "accessibility-graphics-text-actualtext")
		defer closePDFAssociationFixture(t, doc)
		span := findPDFAssociationStructureRole(t, doc, "Span")
		if span == nil {
			t.Fatal("Span structure element not found")
		}
		if span.ActualText() != "paragraph." || span.AlternateDescription() != "" {
			t.Fatalf("Span ActualText = %q, Alt = %q", span.ActualText(), span.AlternateDescription())
		}
		if text, err := span.Text(doc); err != nil || text != "paragraph." {
			t.Fatalf("Span Text = %q, %v; want paragraph.", text, err)
		}
	})
	t.Run("Alt is not replacement text", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "accessibility-graphics-text-figure")
		defer closePDFAssociationFixture(t, doc)
		figure := findPDFAssociationStructureRole(t, doc, "Figure")
		if figure == nil {
			t.Fatal("Figure structure element not found")
		}
		if figure.AlternateDescription() != "paragraph." || figure.ActualText() != "" {
			t.Fatalf("Figure Alt = %q, ActualText = %q", figure.AlternateDescription(), figure.ActualText())
		}
		if text, err := figure.Text(doc); err != nil || text != "" {
			t.Fatalf("Figure Text = %q, %v; want empty text", text, err)
		}
	})
}

func TestPDFAssociationMarkedContentOrderWithinTag(t *testing.T) {
	tests := []struct {
		id       string
		want     [][]int
		textRuns []string
	}{
		{
			id:   "accessibility-marked-order-correct",
			want: [][]int{{0, 1, 2, 3, 4}, {5, 6, 7, 8, 9}},
			textRuns: []string{
				"This is the first paragraph.",
				"Second line of first paragraph.",
				"Third line of first paragraph.",
				"Fourth line of first paragraph.",
				"Fifth line of first paragraph.",
				"This is the second paragraph.",
				"Second line of second paragraph.",
				"Third line of second paragraph.",
				"Fourth line of second paragraph.",
				"Fifth line of second paragraph.",
			},
		},
		{
			id:   "accessibility-marked-order-incorrect",
			want: [][]int{{0}, {6, 7, 8, 9, 10}, {1, 2, 3, 4, 5}},
			textRuns: []string{
				"FAIL",
				"Fifth line of first paragraph.",
				"Fourth line of first paragraph.",
				"Third line of first paragraph.",
				"Second line of first paragraph.",
				"This is the first paragraph.",
				"Fifth line of second paragraph.",
				"Fourth line of second paragraph.",
				"Third line of second paragraph.",
				"Second line of second paragraph.",
				"This is the second paragraph.",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			roots, err := doc.StructureTree()
			if err != nil {
				t.Fatal(err)
			}
			if len(roots) != 1 {
				t.Fatalf("structure root count = %d, want 1", len(roots))
			}
			children, err := roots[0].ChildrenCopy()
			if err != nil {
				t.Fatal(err)
			}
			got := make([][]int, len(children))
			for i, child := range children {
				contents, err := child.ContentsCopy()
				if err != nil {
					t.Fatal(err)
				}
				for _, content := range contents {
					if content.HasMCID() {
						got[i] = append(got[i], content.MCID())
					}
				}
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("paragraph MCIDs = %v, want %v", got, test.want)
			}
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions())
			if err != nil {
				t.Fatal(err)
			}
			assertPDFAssociationTextOrder(t, text, test.textRuns)
		})
	}
}

func TestPDFAssociationHeadingSevenRoleMapping(t *testing.T) {
	tests := []struct {
		id   string
		role string
	}{
		{id: "accessibility-heading-h7-as-p", role: "P"},
		{id: "accessibility-heading-h7-as-h6", role: "H6"},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			var h7 *document.StructElement
			for _, element := range collectPDFAssociationStructure(t, doc) {
				if element.RawRole() == "H7" {
					copy := element
					h7 = &copy
					break
				}
			}
			if h7 == nil {
				t.Fatal("raw H7 structure element not found")
			}
			if h7.Role() != test.role {
				t.Fatalf("H7 mapped role = %q, want %q", h7.Role(), test.role)
			}
		})
	}
}

func TestPDFAssociationListsSpanPages(t *testing.T) {
	tests := []struct {
		id           string
		listItems    int
		spanningRole string
	}{
		{id: "accessibility-list-spans-pages", listItems: 25, spanningRole: "L"},
		{id: "accessibility-list-item-spans-pages", listItems: 3, spanningRole: "LBody"},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			if len(pages) != 2 {
				t.Fatalf("page count = %d, want 2", len(pages))
			}
			list := findPDFAssociationStructureRole(t, doc, "L")
			if list == nil {
				t.Fatal("L structure element not found")
			}
			children, err := list.ChildrenCopy()
			if err != nil {
				t.Fatal(err)
			}
			if len(children) != test.listItems {
				t.Fatalf("list item count = %d, want %d", len(children), test.listItems)
			}
			spanning := list
			if test.spanningRole != "L" {
				spanning = nil
				for element, err := range list.FindAllSeq(test.spanningRole) {
					if err != nil {
						t.Fatal(err)
					}
					if refs := pdfAssociationStructurePageRefs(t, element); len(refs) == len(pages) {
						copy := element.Finalize()
						spanning = &copy
						break
					}
				}
			}
			if spanning == nil {
				t.Fatalf("%s spanning both pages not found in list subtree", test.spanningRole)
			}
			pageRefs := pdfAssociationStructurePageRefs(t, *spanning)
			for _, page := range pages {
				if !pageRefs[page.Ref()] {
					t.Fatalf("%s structure does not reference page %v; refs = %v", test.spanningRole, page.Ref(), pageRefs)
				}
			}
		})
	}
}

func TestPDFAssociationRealContentTagging(t *testing.T) {
	type textExpectation struct {
		text    string
		tag     string
		mcid    int
		hasMCID bool
	}
	tests := []struct {
		id       string
		roles    []string
		texts    []textExpectation
		tagged   string
		untagged string
	}{
		{
			id:    "accessibility-real-content-tagged",
			roles: []string{"P"},
			texts: []textExpectation{{
				text:    "This paragraph is real content, therefore it is tagged with a P structure element.",
				tag:     "P",
				mcid:    0,
				hasMCID: true,
			}},
			tagged:   "This paragraph is real content, therefore it is tagged with a P structure element.",
			untagged: "This paragraph is real content, therefore it is tagged with a P structure element.",
		},
		{
			id:    "accessibility-real-content-partially-untagged",
			roles: []string{"P", "P"},
			texts: []textExpectation{
				{text: "Fail", tag: "P", mcid: 1, hasMCID: true},
				{text: "This paragraph is real content, but it is not tagged", tag: "P", mcid: 2, hasMCID: true},
				{text: "compl", tag: "Artifact", mcid: -1},
				{text: "etely", tag: "Artifact", mcid: -1},
				{text: ".", tag: "Artifact", mcid: -1},
			},
			tagged:   "Fail This paragraph is real content, but it is not tagged",
			untagged: "Fail This paragraph is real content, but it is not tagged completely.",
		},
		{
			id:    "accessibility-real-content-artifacted",
			roles: []string{"P"},
			texts: []textExpectation{
				{text: "FAIL", tag: "P", mcid: 0, hasMCID: true},
				{text: "This paragraph is real content, but it is artifacted.", tag: "Artifact", mcid: -1},
			},
			tagged:   "FAIL",
			untagged: "FAIL This paragraph is real content, but it is artifacted.",
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			assert := func() {
				if roles := rootChildRoles(t, doc); !reflect.DeepEqual(roles, test.roles) {
					t.Fatalf("Document child roles = %v, want %v", roles, test.roles)
				}
				pages, err := doc.CollectPages()
				if err != nil {
					t.Fatal(err)
				}
				got := make([]textExpectation, 0, len(test.texts))
				for text, err := range pages[0].Texts(doc) {
					if err != nil {
						t.Fatal(err)
					}
					got = append(got, textExpectation{
						text:    strings.Join(strings.Fields(text.Text()), " "),
						tag:     text.MarkedTag(),
						mcid:    text.MCID(),
						hasMCID: text.HasMCID(),
					})
				}
				if !reflect.DeepEqual(got, test.texts) {
					t.Fatalf("text objects = %#v, want %#v", got, test.texts)
				}
				if text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions()); err != nil || strings.Join(strings.Fields(text), " ") != test.tagged {
					t.Fatalf("tagged text = %q, %v; want %q", text, err, test.tagged)
				}
				if text, err := pages[0].ExtractTextUntagged(doc, document.DefaultTextExtractionOptions()); err != nil || strings.Join(strings.Fields(text), " ") != test.untagged {
					t.Fatalf("untagged text = %q, %v; want %q", text, err, test.untagged)
				}
			}
			assert()
			doc.ReleaseTransientCaches()
			assert()
		})
	}
}

func TestPDFAssociationDecorativeContentTagging(t *testing.T) {
	const paragraph = "This paragraph is real content, therefore it is tagged with a P structure element. The blue backgroundis decorative and therefore it is artifacted."
	tests := []struct {
		id          string
		roles       []string
		pathTag     string
		pathMCID    int
		pathHasMCID bool
		tagged      string
	}{
		{id: "accessibility-decorative-artifact", roles: []string{"P"}, pathTag: "Artifact", pathMCID: -1, tagged: paragraph},
		{id: "accessibility-decorative-figure", roles: []string{"P", "P", "Figure"}, pathTag: "Figure", pathMCID: 1, pathHasMCID: true, tagged: "FAIL " + paragraph},
		{id: "accessibility-decorative-unmarked", roles: []string{"P", "P"}, pathMCID: -1, tagged: "FAIL " + paragraph},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			assert := func() {
				if roles := rootChildRoles(t, doc); !reflect.DeepEqual(roles, test.roles) {
					t.Fatalf("Document child roles = %v, want %v", roles, test.roles)
				}
				pages, err := doc.CollectPages()
				if err != nil {
					t.Fatal(err)
				}
				paths := 0
				for path, err := range pages[0].Paths(doc) {
					if err != nil {
						t.Fatal(err)
					}
					paths++
					if path.MarkedTag() != test.pathTag || path.MCID() != test.pathMCID || path.HasMCID() != test.pathHasMCID {
						t.Fatalf("decorative path tag = %q, MCID = %d/%v; want %q, %d/%v", path.MarkedTag(), path.MCID(), path.HasMCID(), test.pathTag, test.pathMCID, test.pathHasMCID)
					}
				}
				if paths != 1 {
					t.Fatalf("path count = %d, want 1", paths)
				}
				if text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions()); err != nil || strings.Join(strings.Fields(text), " ") != test.tagged {
					t.Fatalf("tagged text = %q, %v; want %q", text, err, test.tagged)
				}
			}
			assert()
			doc.ReleaseTransientCaches()
			assert()
		})
	}
}

func TestPDFAssociationLogicalStructureOrder(t *testing.T) {
	tests := []struct {
		id    string
		roles []string
		runs  []string
	}{
		{
			id:    "accessibility-order-correct",
			roles: []string{"H1", "P", "P", "P"},
			runs: []string{
				"This is an H1",
				"This is the first paragraph.",
				"This is the second paragraph.",
				"This is the third paragraph.",
			},
		},
		{
			id:    "accessibility-order-incorrect",
			roles: []string{"P", "P", "H1", "P", "P"},
			runs: []string{
				"FAIL",
				"This is the second paragraph.",
				"This is an H1",
				"This is the third paragraph.",
				"This is the first paragraph.",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			if got := rootChildRoles(t, doc); !reflect.DeepEqual(got, test.roles) {
				t.Fatalf("root child roles = %v, want %v", got, test.roles)
			}
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions())
			if err != nil {
				t.Fatal(err)
			}
			assertPDFAssociationTextOrder(t, text, test.runs)
		})
	}
}

func TestPDFAssociationAnnotationTabOrderAndTraversalRepeatability(t *testing.T) {
	tests := []struct {
		id           string
		tabs         string
		hasTabs      bool
		fieldNames   []string
		alternate    []string
		structParent []int
	}{
		{
			id:           "accessibility-tab-order-structure",
			tabs:         "S",
			hasTabs:      true,
			fieldNames:   []string{"FirstName", "LastName", "SubmitButton"},
			alternate:    []string{"First Name", "Last Name", "Submit Form"},
			structParent: []int{3, 2, 5},
		},
		{
			id:           "accessibility-tab-order-row",
			tabs:         "R",
			hasTabs:      true,
			fieldNames:   []string{"Text1", "SubmitButton", "Text2"},
			alternate:    []string{"First name", "Submit Form", "Last name "},
			structParent: []int{2, 5, 3},
		},
		{
			id:           "accessibility-tab-order-missing",
			hasTabs:      false,
			fieldNames:   []string{"Text2", "SubmitButton", "Text1"},
			alternate:    []string{"Last name ", "Submit Form", "First name"},
			structParent: []int{3, 8, 7},
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			assert := func() {
				pages, err := doc.CollectPages()
				if err != nil {
					t.Fatal(err)
				}
				if len(pages) != 1 {
					t.Fatalf("pages = %d, want 1", len(pages))
				}
				tabs, hasTabs := pages[0].Get(document.Name("Tabs"))
				if hasTabs != test.hasTabs {
					t.Fatalf("Tabs present = %v, want %v", hasTabs, test.hasTabs)
				}
				if test.hasTabs && tabs != document.Name(test.tabs) {
					t.Fatalf("Tabs = %#v, want /%s", tabs, test.tabs)
				}
				annotations, err := pages[0].CollectAnnotations(doc)
				if err != nil {
					t.Fatal(err)
				}
				if len(annotations) != 3 {
					t.Fatalf("annotations = %d, want 3", len(annotations))
				}
				for index, annotation := range annotations {
					if annotation.Subtype() != "Widget" {
						t.Fatalf("annotation %d subtype = %q, want Widget", index, annotation.Subtype())
					}
					dict := annotation.DictCopy()
					if got := pdfaAnnotationDictString(t, dict, "T"); got != test.fieldNames[index] {
						t.Fatalf("annotation %d /T = %q, want %q", index, got, test.fieldNames[index])
					}
					if got := pdfaAnnotationDictString(t, dict, "TU"); got != test.alternate[index] {
						t.Fatalf("annotation %d /TU = %q, want %q", index, got, test.alternate[index])
					}
					value, ok := dict[document.Name("StructParent")].(document.Number)
					if !ok || int(value) != test.structParent[index] || value < 0 {
						t.Fatalf("annotation %d /StructParent = %#v, want nonnegative %d", index, dict[document.Name("StructParent")], test.structParent[index])
					}
				}
				structure, err := pages[0].Structure(doc)
				if err != nil {
					t.Fatal(err)
				}
				if len(structure.ElementsCopy()) == 0 {
					t.Fatal("page structure has no elements")
				}
				marked := 0
				for _, err := range pages[0].MarkedContent(doc) {
					if err != nil {
						t.Fatal(err)
					}
					marked++
				}
				if marked == 0 {
					t.Fatal("page has no marked-content sections")
				}
			}
			assert()
			doc.ReleaseTransientCaches()
			assert()
		})
	}
}

func TestPDFAssociationIncorrectMarkedSequenceOrder(t *testing.T) {
	doc := openPDFAssociationFixture(t, "accessibility-marked-sequence-order-incorrect")
	defer closePDFAssociationFixture(t, doc)
	assert := func() {
		roots, err := doc.StructureTree()
		if err != nil {
			t.Fatal(err)
		}
		if len(roots) != 1 || roots[0].Role() != "Document" {
			t.Fatalf("structure roots = %#v, want one Document", roots)
		}
		paragraphs, err := roots[0].ChildrenCopy()
		if err != nil {
			t.Fatal(err)
		}
		if len(paragraphs) != 2 {
			t.Fatalf("Document paragraph children = %d, want 2", len(paragraphs))
		}
		if roles := []string{paragraphs[0].Role(), paragraphs[1].Role()}; !reflect.DeepEqual(roles, []string{"P", "P"}) {
			t.Fatalf("paragraph roles = %v, want [P P]", roles)
		}
		contents, err := paragraphs[1].ContentsCopy()
		if err != nil {
			t.Fatal(err)
		}
		if len(contents) != 1 || !contents[0].HasMCID() || contents[0].MCID() != 1 {
			t.Fatalf("second paragraph contents = %#v, want MCID 1", contents)
		}
		pages, err := doc.CollectPages()
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 1 {
			t.Fatalf("pages = %d, want 1", len(pages))
		}
		var chunks []string
		for text, err := range pages[0].Texts(doc) {
			if err != nil {
				t.Fatal(err)
			}
			if text.HasMCID() && text.MCID() == 1 {
				chunks = append(chunks, text.Text())
			}
		}
		if want := []string{"ectly set", "t sequence incorr", "t within a marked conten", "Conten"}; !reflect.DeepEqual(chunks, want) {
			t.Fatalf("MCID 1 source chunks = %q, want %q", chunks, want)
		}
		if tagged, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions()); err != nil || tagged != "FAIL\nectly sett sequence incorrt within a marked contenConten" {
			t.Fatalf("tagged text = %q, %v", tagged, err)
		}
		if untagged, err := pages[0].ExtractTextUntagged(doc, document.DefaultTextExtractionOptions()); err != nil || untagged != "FAIL\nectly set\nt sequence incorr\nt within a marked conten\nConten" {
			t.Fatalf("untagged text = %q, %v", untagged, err)
		}
	}
	assert()
	doc.ReleaseTransientCaches()
	assert()
}

func pdfaAnnotationDictString(t *testing.T, dict document.Dict, key string) string {
	t.Helper()
	value, ok := dict[document.Name(key)].(document.String)
	if !ok {
		t.Fatalf("/%s = %#v, want string", key, dict[document.Name(key)])
	}
	return string(value)
}

func TestPDFAssociationTaggedTextUsesStructureOrderForFailureMarkers(t *testing.T) {
	tests := []struct {
		id   string
		runs []string
	}{
		{id: "accessibility-heading-as-paragraph", runs: []string{"FAIL", "This is an H1", "This is a paragraph."}},
		{id: "accessibility-graphics-text-figure", runs: []string{"FAIL", "This is the"}},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions())
			if err != nil {
				t.Fatal(err)
			}
			assertPDFAssociationTextOrder(t, text, test.runs)
		})
	}
}

func TestPDFAssociationColumnAndSidebarReadingOrder(t *testing.T) {
	tests := []struct {
		id    string
		roles []string
		runs  []string
	}{
		{
			id:    "accessibility-columns-correct",
			roles: []string{"H1", "P", "P", "P"},
			runs: []string{
				"This is an H1",
				"This is the first paragraph.",
				"Second line of first paragraph.",
				"Third line of first paragraph.",
				"Fourth line of first paragraph.",
				"Fifth line of first paragraph.",
				"This is the second paragraph.",
				"Second line of second paragraph.",
				"Third line of second paragraph.",
				"Fourth line of second paragraph.",
				"Fifth line of second paragraph.",
				"This is the third paragraph.",
				"Second line of third paragraph.",
				"Third line of third paragraph.",
				"Fourth line of third paragraph.",
				"Fifth line of third paragraph.",
			},
		},
		{
			id:    "accessibility-columns-incorrect",
			roles: []string{"P", "H1", "P", "P", "P", "P", "P", "P", "P", "P", "P"},
			runs: []string{
				"FAIL",
				"This is an H1",
				"This is the first paragraph.",
				"Fourth line of second paragraph.",
				"Second line of first paragraph.",
				"Fifth line of second paragraph.",
				"Third line of first paragraph.",
				"Fourth line of first paragraph.",
				"This is the third paragraph.",
				"Fifth line of first paragraph.",
				"Second line of third paragraph.",
				"Third line of third paragraph.",
				"This is the second paragraph.",
				"Fourth line of third paragraph.",
				"Second line of second paragraph.",
				"Fifth line of third paragraph.",
				"Third line of second paragraph.",
			},
		},
		{
			id:    "accessibility-sidebar-correct",
			roles: []string{"H1", "Div", "P", "P", "P"},
			runs: []string{
				"This is an H1",
				"This is sidebar text with the main idea of the following paragraphs.",
				"This is the first paragraph.",
				"This is the second paragraph.",
				"This is the third paragraph.",
			},
		},
		{
			id:    "accessibility-sidebar-incorrect",
			roles: []string{"P", "H1", "P", "P", "Div", "P"},
			runs: []string{
				"FAIL",
				"This is an H1",
				"This is the first paragraph.",
				"This is the second paragraph.",
				"This is sidebar text with the main idea of the first paragraph.",
				"This is the third paragraph.",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			if got := rootChildRoles(t, doc); !reflect.DeepEqual(got, test.roles) {
				t.Fatalf("root child roles = %v, want %v", got, test.roles)
			}
			pages, err := doc.CollectPages()
			if err != nil {
				t.Fatal(err)
			}
			text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions())
			if err != nil {
				t.Fatal(err)
			}
			assertPDFAssociationTextOrder(t, text, test.runs)
		})
	}
}

func TestPDFAssociationHeadingSemantics(t *testing.T) {
	tests := []struct {
		id   string
		want []string
	}{
		{id: "accessibility-heading-h1", want: []string{"H1", "P"}},
		{id: "accessibility-heading-as-paragraph", want: []string{"P", "P", "P"}},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			if got := rootChildRoles(t, doc); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("root child roles = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPDFAssociationRemainingHeadingSemantics(t *testing.T) {
	const hierarchyText = "Section 1 - Main Heading\nParagraph for first heading level 1\nFirst-level sub-heading under Section 1\nParagraph for first heading level 2\nSecond-level sub-heading under Section 1\nParagraph for first heading level 3\nSection 2 - Main Heading\nParagraph for second heading level 1\nSection 3 - Main Heading\nParagraph for third heading level 1\nFirst-level sub-heading under Section 3\nParagraph for heading level 2\nSecond-level sub-heading under Section 3\nParagraph for heading level 3"
	tests := []struct {
		id         string
		projection []string
		text       []string
	}{
		{
			id:         "accessibility-heading-multiline",
			projection: []string{"H1>H1@0:[1]", "P>P@0:[3]"},
			text:       []string{"Two-line Heading\nThis is the paragraph."},
		},
		{
			id: "accessibility-heading-level-hierarchy",
			projection: []string{
				"H1>H1@0:[0]", "P>P@0:[1]", "H2>H2@0:[2]", "P>P@0:[3]",
				"H3>H3@0:[4]", "P>P@0:[5]", "H1>H1@0:[6]", "P>P@0:[7]",
				"H1>H1@0:[8]", "P>P@0:[9]", "H2>H2@0:[10]", "P>P@0:[11]",
				"H3>H3@0:[12]", "P>P@0:[13]",
			},
			text: []string{hierarchyText},
		},
		{
			id:         "accessibility-heading-image",
			projection: []string{"H1>H1@0:[0]", "H2>H2@-1:[]", "P>P@0:[3]", "H2>H2@-1:[]", "P>P@0:[7]", "P>P@0:[10]", "P>P@0:[13]"},
			text:       []string{"Contact us\n+1 123 456789\nOur Company\nMain Street 101\nBig City"},
		},
		{
			id:         "accessibility-heading-levels-contiguous",
			projection: []string{"H1>H1@0:[0]", "H2>H2@0:[8]"},
			text:       []string{"1 This is a heading 1\n1.1.1 This is a heading 3"},
		},
		{
			id: "accessibility-heading-subtitle",
			projection: []string{
				"Title>P@0:[2]", "Subtitle>P@0:[5]", "H1>H1@1:[4]", "P>P@1:[9]",
				"H2>H2@1:[11]", "P>P@1:[17]", "H3>H3@1:[19]", "P>P@1:[25]",
				"H1>H1@1:[27]", "P>P@1:[32]", "H1>H1@1:[34]", "P>P@1:[39]",
				"H2>H2@1:[41]", "P>P@1:[47]", "H3>H3@1:[49]", "P>P@1:[55]",
			},
			text: []string{"This is the Title\nThis is the subtitle", hierarchyText},
		},
		{
			id: "accessibility-heading-title",
			projection: []string{
				"Title>P@0:[2]", "H1>H1@1:[4]", "P>P@1:[9]", "H2>H2@1:[11]",
				"P>P@1:[17]", "H3>H3@1:[19]", "P>P@1:[25]", "H1>H1@1:[27]",
				"P>P@1:[32]", "H1>H1@1:[34]", "P>P@1:[39]", "H2>H2@1:[41]",
				"P>P@1:[47]", "H3>H3@1:[49]", "P>P@1:[55]",
			},
			text: []string{"This is the Title", hierarchyText},
		},
		{
			id:         "accessibility-heading-multiline-split",
			projection: []string{"P>P@0:[0]", "H1>H1@0:[3]", "H1>H1@0:[4]", "P>P@0:[2]"},
			text:       []string{"FAIL\nTwo-line\nHeading\nThis is the paragraph."},
		},
		{
			id:         "accessibility-heading-paragraph-as-h2",
			projection: []string{"H1>H1@0:[0]", "H2>H2@0:[2]"},
			text:       []string{"This is an H1\nThis is a paragraph."},
		},
		{
			id:         "accessibility-heading-table-cells",
			projection: []string{"P>P@0:[2]", "Table>Table@-1:[]"},
			text:       []string{"Fail\n2018 2019 2020\nNumber 10 20 30\nPercentage 11% 12% 13%"},
		},
		{
			id:         "accessibility-heading-second-level-as-paragraph",
			projection: []string{"P>P@0:[0]", "H1>H1@0:[2]", "P>P@0:[4]", "P>P@0:[6]", "P>P@0:[8]"},
			text:       []string{"Fail\nThis is an H1\nThis is a paragraph.\nThis is an H2\nThis is a paragraph"},
		},
		{
			id:         "accessibility-heading-mixed-h-and-hn",
			projection: []string{"P>P@0:[0]", "H>H@0:[2]", "H1>H1@0:[5]"},
			text:       []string{"FAIL\nThis is a Heading level 1\nThis is a Heading level 2"},
		},
		{
			id: "accessibility-heading-title-as-h1",
			projection: []string{
				"P>P@0:[2]", "H1>H1@0:[4]", "H1>H1@1:[4]", "P>P@1:[9]",
				"H2>H2@1:[11]", "P>P@1:[17]", "H3>H3@1:[19]", "P>P@1:[25]",
				"H1>H1@1:[27]", "P>P@1:[32]", "H1>H1@1:[34]", "P>P@1:[39]",
				"H2>H2@1:[41]", "P>P@1:[47]", "H3>H3@1:[49]", "P>P@1:[55]",
			},
			text: []string{"Fail\nThis is the Title", hierarchyText},
		},
		{
			id:         "accessibility-heading-level-skipped",
			projection: []string{"P>P@0:[10]", "H1>H1@0:[0]", "H3>H3@0:[8]"},
			text:       []string{"FAIL\n1 This is a heading 1\n1.1.1 This is a heading 3"},
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			assert := func() {
				pages, err := doc.CollectPages()
				if err != nil {
					t.Fatal(err)
				}
				pageIndex := make(map[document.Ref]int, len(pages))
				for index, page := range pages {
					pageIndex[page.Ref()] = index
				}
				roots, err := doc.StructureTree()
				if err != nil {
					t.Fatal(err)
				}
				if len(roots) != 1 || roots[0].Role() != "Document" {
					t.Fatalf("structure roots = %#v, want one Document", roots)
				}
				children, err := roots[0].ChildrenCopy()
				if err != nil {
					t.Fatal(err)
				}
				projection := make([]string, len(children))
				for index, child := range children {
					page := -1
					if child.HasPage() {
						var ok bool
						page, ok = pageIndex[child.Page()]
						if !ok {
							t.Fatalf("child %d references unknown page %v", index, child.Page())
						}
					}
					contents, err := child.ContentsCopy()
					if err != nil {
						t.Fatal(err)
					}
					mcids := make([]int, 0, len(contents))
					for _, content := range contents {
						if content.HasMCID() {
							mcids = append(mcids, content.MCID())
						}
					}
					projection[index] = fmt.Sprintf("%s>%s@%d:%v", child.RawRole(), child.Role(), page, mcids)
				}
				if !reflect.DeepEqual(projection, test.projection) {
					t.Fatalf("root child projection = %v, want %v", projection, test.projection)
				}
				if len(pages) != len(test.text) {
					t.Fatalf("pages = %d, want %d", len(pages), len(test.text))
				}
				for index, page := range pages {
					text, err := page.ExtractTextTagged(doc, document.DefaultTextExtractionOptions())
					if err != nil || text != test.text[index] {
						t.Fatalf("page %d tagged text = %q, %v; want %q", index, text, err, test.text[index])
					}
				}
			}
			assert()
			doc.ReleaseTransientCaches()
			assert()
		})
	}
}

func TestPDFAssociationHeadingImagesNestFigures(t *testing.T) {
	doc := openPDFAssociationFixture(t, "accessibility-heading-image")
	defer closePDFAssociationFixture(t, doc)
	assert := func() {
		roots, err := doc.StructureTree()
		if err != nil {
			t.Fatal(err)
		}
		children, err := roots[0].ChildrenCopy()
		if err != nil {
			t.Fatal(err)
		}
		for index, want := range []struct {
			child int
			mcid  int
			alt   string
		}{{child: 1, mcid: 15, alt: "Phone"}, {child: 3, mcid: 16, alt: "Address"}} {
			heading := children[want.child]
			if heading.Role() != "H2" {
				t.Fatalf("image heading %d role = %q, want H2", index, heading.Role())
			}
			figures, err := heading.ChildrenCopy()
			if err != nil {
				t.Fatal(err)
			}
			if len(figures) != 1 || figures[0].Role() != "Figure" || figures[0].AlternateDescription() != want.alt {
				t.Fatalf("image heading %d children = %#v, want one Figure with alt %q", index, figures, want.alt)
			}
			contents, err := figures[0].ContentsCopy()
			if err != nil {
				t.Fatal(err)
			}
			if len(contents) != 1 || !contents[0].HasMCID() || contents[0].MCID() != want.mcid {
				t.Fatalf("image heading %d contents = %#v, want MCID %d", index, contents, want.mcid)
			}
		}
	}
	assert()
	doc.ReleaseTransientCaches()
	assert()
}

func TestPDFAssociationAppropriateSemantics(t *testing.T) {
	tests := []struct {
		id         string
		roles      []string
		mcids      [][]int
		taggedRuns []string
	}{
		{
			id:         "accessibility-semantic-heading",
			roles:      []string{"H1", "P"},
			mcids:      [][]int{{0}, {2}},
			taggedRuns: []string{"This is an H1", "This is a paragraph."},
		},
		{
			id:         "accessibility-semantic-similar-content",
			roles:      []string{"H1", "P", "P"},
			mcids:      [][]int{{0}, {2}, {13}},
			taggedRuns: []string{"This is an H1", "This is the first paragraph.", "This is the second paragraph."},
		},
		{
			id:         "accessibility-semantic-visually-separated",
			roles:      []string{"P"},
			mcids:      [][]int{{0}},
			taggedRuns: []string{"This is repeating text to fill the left column", "make it overflow to the right column"},
		},
		{
			id:         "accessibility-semantic-multiline-heading",
			roles:      []string{"H1", "P"},
			mcids:      [][]int{{1}, {3}},
			taggedRuns: []string{"Two-line Heading", "This is the paragraph."},
		},
		{
			id:         "accessibility-semantic-heading-inappropriate",
			roles:      []string{"P", "P", "P"},
			mcids:      [][]int{{3}, {0}, {2}},
			taggedRuns: []string{"FAIL", "This is an H1", "This is a paragraph."},
		},
		{
			id:         "accessibility-semantic-similar-content-inappropriate",
			roles:      []string{"P", "H1", "P"},
			mcids:      [][]int{{0}, {2}, {4, 16}},
			taggedRuns: []string{"Fail", "This is an H1", "This is the first paragraph.", "This is the second paragraph."},
		},
		{
			id:         "accessibility-semantic-unit-split",
			roles:      []string{"P", "P", "Figure", "P"},
			mcids:      [][]int{{1}, {2, 3}, {4}, {5, 6, 7}},
			taggedRuns: []string{"FAIL", "This is the first line of a paragraph.", "Second line of the paragraph.", "Place holder Image", "Third line of the paragraph.", "Fifth line of the paragraph."},
		},
		{
			id:         "accessibility-semantic-visually-separated-split",
			roles:      []string{"P", "P", "P"},
			mcids:      [][]int{{7}, {3}, {6}},
			taggedRuns: []string{"FAIL", "This is repeating text to fill the left column", "and make it overflow to the right column"},
		},
		{
			id:         "accessibility-semantic-multiline-heading-split",
			roles:      []string{"P", "H1", "H1", "P"},
			mcids:      [][]int{{0}, {3}, {4}, {2}},
			taggedRuns: []string{"FAIL", "Two-line", "Heading", "This is the paragraph."},
		},
		{
			id:         "accessibility-semantic-table-headers-as-headings",
			roles:      []string{"P", "Table"},
			mcids:      [][]int{{2}, nil},
			taggedRuns: []string{"Fail", "2018", "2019", "2020", "Number", "10", "20", "30", "Percentage", "11%", "12%", "13%"},
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			assert := func() {
				roots, err := doc.StructureTree()
				if err != nil {
					t.Fatal(err)
				}
				if len(roots) != 1 || roots[0].Role() != "Document" {
					t.Fatalf("structure roots = %#v, want one Document", roots)
				}
				children, err := roots[0].ChildrenCopy()
				if err != nil {
					t.Fatal(err)
				}
				roles := make([]string, len(children))
				mcids := make([][]int, len(children))
				for index, child := range children {
					roles[index] = child.Role()
					contents, err := child.ContentsCopy()
					if err != nil {
						t.Fatal(err)
					}
					for _, content := range contents {
						if content.HasMCID() {
							mcids[index] = append(mcids[index], content.MCID())
						}
					}
				}
				if !reflect.DeepEqual(roles, test.roles) {
					t.Fatalf("Document child roles = %v, want %v", roles, test.roles)
				}
				if !reflect.DeepEqual(mcids, test.mcids) {
					t.Fatalf("Document child MCIDs = %v, want %v", mcids, test.mcids)
				}
				pages, err := doc.CollectPages()
				if err != nil {
					t.Fatal(err)
				}
				if len(pages) != 1 {
					t.Fatalf("pages = %d, want 1", len(pages))
				}
				text, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions())
				if err != nil {
					t.Fatal(err)
				}
				assertPDFAssociationTextOrder(t, text, test.taggedRuns)
			}
			assert()
			doc.ReleaseTransientCaches()
			assert()
		})
	}
}

func TestPDFAssociationTableHeadersRemainHeadingsInsideCells(t *testing.T) {
	for _, fixtureID := range []string{"accessibility-semantic-table-headers-as-headings", "accessibility-heading-table-cells"} {
		t.Run(fixtureID, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, fixtureID)
			defer closePDFAssociationFixture(t, doc)
			assert := func() {
				roots, err := doc.StructureTree()
				if err != nil {
					t.Fatal(err)
				}
				if len(roots) != 1 {
					t.Fatalf("structure roots = %d, want 1", len(roots))
				}
				children, err := roots[0].ChildrenCopy()
				if err != nil {
					t.Fatal(err)
				}
				if len(children) != 2 || children[1].Role() != "Table" {
					t.Fatalf("Document children = %#v, want P and Table", children)
				}
				rows, err := children[1].ChildrenCopy()
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != 3 {
					t.Fatalf("table rows = %d, want 3", len(rows))
				}
				wantNested := [][]string{{"", "H1", "H1", "H1"}, {"H2", "P", "P", "P"}, {"H2", "P", "P", "P"}}
				for rowIndex, row := range rows {
					if row.Role() != "TR" {
						t.Fatalf("row %d role = %q, want TR", rowIndex, row.Role())
					}
					cells, err := row.ChildrenCopy()
					if err != nil {
						t.Fatal(err)
					}
					if len(cells) != 4 {
						t.Fatalf("row %d cells = %d, want 4", rowIndex, len(cells))
					}
					for cellIndex, cell := range cells {
						if cell.Role() != "TD" {
							t.Fatalf("row %d cell %d role = %q, want TD", rowIndex, cellIndex, cell.Role())
						}
						nested, err := cell.ChildrenCopy()
						if err != nil {
							t.Fatal(err)
						}
						want := wantNested[rowIndex][cellIndex]
						if want == "" {
							if len(nested) != 0 {
								t.Fatalf("row %d cell %d children = %#v, want none", rowIndex, cellIndex, nested)
							}
						} else if len(nested) != 1 || nested[0].Role() != want {
							t.Fatalf("row %d cell %d children = %#v, want one %s", rowIndex, cellIndex, nested, want)
						}
					}
				}
				for _, element := range collectPDFAssociationStructure(t, doc) {
					if element.Role() == "TH" {
						t.Fatal("invalid table unexpectedly exposes a TH element")
					}
				}
			}
			assert()
			doc.ReleaseTransientCaches()
			assert()
		})
	}
}

func TestPDFAssociationListSemantics(t *testing.T) {
	t.Run("unordered list", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "accessibility-list-unordered")
		defer closePDFAssociationFixture(t, doc)
		elements := collectPDFAssociationStructure(t, doc)
		var roles []string
		var list *document.StructElement
		for _, element := range elements {
			roles = append(roles, element.Role())
			if element.Role() == "L" {
				copy := element
				list = &copy
			}
		}
		if list == nil {
			t.Fatalf("list role missing from %v", roles)
		}
		if got := list.AttributesCopy()[document.Name("ListNumbering")]; got != document.Name("Disc") {
			t.Fatalf("ListNumbering = %#v, want Disc", got)
		}
		for _, want := range []string{"LI", "Lbl", "LBody"} {
			if !containsPDFAssociationString(roles, want) {
				t.Fatalf("list role %q missing from %v", want, roles)
			}
		}
	})
	t.Run("list tagged as paragraphs", func(t *testing.T) {
		doc := openPDFAssociationFixture(t, "accessibility-list-as-paragraphs")
		defer closePDFAssociationFixture(t, doc)
		if got, want := rootChildRoles(t, doc), []string{"P", "P", "P", "P", "P"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("root child roles = %v, want %v", got, want)
		}
	})
}

func TestPDFAssociationListFixtureStructures(t *testing.T) {
	const absent = "<absent>"
	tests := []struct {
		id        string
		signature string
		numbering []string
		tagged    string
	}{
		{
			id:        "accessibility-list-upper-roman",
			signature: "Document(L(LI(Lbl,LBody(P)),LI(Lbl,LBody(P)),LI(Lbl,LBody(P))))",
			numbering: []string{"UpperRoman"},
			tagged:    "I.List item one\nII.List item two\nIII.List item three",
		},
		{
			id:        "accessibility-list-decimal",
			signature: "Document(L(LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody)))",
			numbering: []string{"Decimal"},
			tagged:    "1.Apple\n2.Orange\n3.Lemon",
		},
		{
			id:        "accessibility-list-decorative-image-labels",
			signature: "Document(L(LI(Lbl(Span),LBody(P)),LI(Lbl(Span),LBody(P)),LI(Lbl(Span),LBody(P))))",
			// The pinned PDF explicitly authors Disc even though its README says
			// ListNumbering should be absent or None. Its bullet ActualText also
			// decodes with a trailing NUL. Preserve actual file semantics here;
			// these assertions are not a conformance endorsement.
			numbering: []string{"Disc"},
			tagged:    "List item 1 List item 2 List item 3",
		},
		{
			id:        "accessibility-list-multilevel",
			signature: "Document(L(LI(Lbl,LBody(P,L(LI(Lbl,LBody(P)),LI(Lbl,LBody(P))))),LI(Lbl,LBody(P,L(LI(Lbl,LBody(P)),LI(Lbl,LBody(P)),LI(Lbl,LBody(P)))))))",
			numbering: []string{"Decimal", "LowerAlpha", "LowerAlpha"},
			tagged:    "1.Vegetables\na.Potato\nb.Onion\n2.Fruits\na.Apple\nb.Orange\nc.Lemon",
		},
		{
			id:        "accessibility-list-semantic-image-labels",
			signature: "Document(P,L(LI(Lbl(Figure),LBody),LI(Lbl(Figure),LBody)))",
			// The pinned PDF explicitly authors None while its README describes
			// the default/absent form. Assert bytes-on-disk semantics only.
			numbering: []string{"None"},
			tagged:    "Vegetable Servings\n 1 cup\n 2 cups",
		},
		{
			id:        "accessibility-list-numbering-absent",
			signature: "Document(P,L(LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody)),P,L(LI(LBody),LI(LBody),LI(LBody)),P,L(LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody)))",
			numbering: []string{absent, absent, absent},
			tagged:    "A bulleted list:\n✓ Item 1\n✓ Item 2\n✓ Item 3\nA list without labels: Item 1 Item 2 Item 3\nA definition list:\nACCC: Acme Competition & Consumer Commission\nACL: Acme Consumer Law\nCAFG: Consumer Affairs Focus Group\nCDRAC: Compliance and Dispute Resolution Advisory Committee\nDBCA: Dome Building Contractual Agreements",
		},
		{
			id:        "accessibility-list-numbering-none",
			signature: "Document(P,L(LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody)),P,L(LI(LBody),LI(LBody),LI(LBody)),P,L(LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody)))",
			numbering: []string{"None", "None", "None"},
			tagged:    "A bulleted list:\n✓ Item 1\n✓ Item 2\n✓ Item 3\nA list without labels: Item 1 Item 2 Item 3\nA definition list:\nACCC: Acme Competition & Consumer Commission\nACL: Acme Consumer Law\nCAFG: Consumer Affairs Focus Group\nCDRAC: Compliance and Dispute Resolution Advisory Committee\nDBCA: Dome Building Contractual Agreements",
		},
		{
			id:        "accessibility-list-caption",
			signature: "Document(L(Caption(P),LI(Lbl,LBody(P)),LI(Lbl,LBody(P)),LI(Lbl,LBody(P))))",
			numbering: []string{absent},
			tagged:    "Caption for the list below\n• List item 1\n• List item 2\n• List item 3",
		},
		{
			id:        "accessibility-list-inline",
			signature: "Document(P(L(LI(LBody),LI(LBody),LI(LBody),LI(LBody))),P(L(LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody))))",
			numbering: []string{absent, "LowerRoman"},
			tagged:    "Pets are a great addition to any family, especially families with\nchildren. This is an example of an inline list that lists different\npets: cats, dogs, fish and hamsters\n. Many people who have pets enjoy playing with them.\nA balance diet is important to a healthy lifestyle. It is important to\neat lots of fruit and vegetables. This is an example of an inline\nnumbered list that lists different fruits: (i) apple, (ii) orange,\n(iii)\nwatermelon and (iv) cherry\n. Some fruits are available all year while others grow in just a\nsingle season.",
		},
		{
			id:        "accessibility-list-hierarchical",
			signature: "Document(L(LI(Lbl,LBody(P)),L(LI(Lbl,LBody(P)),LI(Lbl,LBody(P))),LI(Lbl,LBody(P)),L(LI(Lbl,LBody(P)),LI(Lbl,LBody(P)),LI(Lbl,LBody(P)))))",
			numbering: []string{"Decimal", "LowerAlpha", "LowerAlpha"},
			tagged:    "1.Vegetables\na.Potato\nb.Onion\n2.Fruits\na.Apple\nb.Orange\nc.Lemon",
		},
		{
			id:        "accessibility-list-item-substructure",
			signature: "Document(L(LI(Lbl,LBody(P,P)),LI(Lbl,LBody(P,P)),LI(Lbl,LBody),LI(Lbl,LBody)))",
			numbering: []string{"Decimal"},
			tagged:    "1.This is item 1 of a numbered list\nThis is the second paragraph of item 1.\n2.This is item 2 of a numbered list\nThis is the second paragraph of item 2.\n3.This is item 3 of a numbered list\n4.This is item 4 of a numbered list",
		},
		{
			id:        "accessibility-list-description",
			signature: "Document(L(LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody)))",
			numbering: []string{"None"},
			tagged:    "PDF/UAPDF for accessibility\nPDF/A PDF for long-term preservation\nPDF/E PDF for engineering workflows\nPDF/X PDF for graphic arts\nPDF/VT PDF for variable data",
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			assert := func() {
				roots, err := doc.StructureTree()
				if err != nil {
					t.Fatal(err)
				}
				if got := pdfAssociationStructureSignature(t, roots); got != test.signature {
					t.Fatalf("structure signature = %q, want %q", got, test.signature)
				}

				var numbering []string
				for _, element := range collectPDFAssociationStructure(t, doc) {
					if element.Role() != "L" {
						continue
					}
					value, present := element.AttributesCopy()[document.Name("ListNumbering")]
					if !present {
						numbering = append(numbering, absent)
						continue
					}
					name, ok := value.(document.Name)
					if !ok {
						t.Fatalf("ListNumbering = %#v, want a name", value)
					}
					numbering = append(numbering, string(name))
				}
				if !reflect.DeepEqual(numbering, test.numbering) {
					t.Fatalf("ListNumbering values = %v, want %v", numbering, test.numbering)
				}

				pages, err := doc.CollectPages()
				if err != nil {
					t.Fatal(err)
				}
				if len(pages) != 1 {
					t.Fatalf("page count = %d, want 1", len(pages))
				}
				if tagged, err := pages[0].ExtractTextTagged(doc, document.DefaultTextExtractionOptions()); err != nil || tagged != test.tagged {
					t.Fatalf("tagged text = %q, %v; want %q", tagged, err, test.tagged)
				}

				switch test.id {
				case "accessibility-list-decorative-image-labels":
					var actualText []string
					for _, element := range collectPDFAssociationStructure(t, doc) {
						if element.Role() == "Span" {
							actualText = append(actualText, element.ActualText())
						}
					}
					if want := []string{"•\x00", "•\x00", "•\x00"}; !reflect.DeepEqual(actualText, want) {
						t.Fatalf("decorative label ActualText = %q, want %q", actualText, want)
					}
				case "accessibility-list-semantic-image-labels":
					var alternateDescriptions []string
					for _, element := range collectPDFAssociationStructure(t, doc) {
						if element.Role() == "Figure" {
							alternateDescriptions = append(alternateDescriptions, element.AlternateDescription())
						}
					}
					if want := []string{"broccoli", "tomato"}; !reflect.DeepEqual(alternateDescriptions, want) {
						t.Fatalf("semantic label alternate descriptions = %q, want %q", alternateDescriptions, want)
					}
				case "accessibility-list-inline":
					for _, element := range collectPDFAssociationStructure(t, doc) {
						switch element.Role() {
						case "L", "LI", "Lbl", "LBody":
							if placement := element.AttributesCopy()[document.Name("Placement")]; placement != document.Name("Inline") {
								t.Fatalf("%s Placement = %#v, want Inline", element.Role(), placement)
							}
						}
					}
				}
			}
			assert()
			doc.ReleaseTransientCaches()
			assert()
		})
	}
}

func TestPDFAssociationListFailureFixtureStructures(t *testing.T) {
	const absent = "<absent>"
	tests := []struct {
		id        string
		signature string
		numbering []string
		tagged    []string
	}{
		{
			id:        "accessibility-list-numbering-wrong-for-bullets",
			signature: "Document(P,L(LI(Lbl,LBody(P)),LI(Lbl,LBody(P)),LI(Lbl,LBody(P))))",
			numbering: []string{"Square"},
			tagged:    []string{"Fail\n•Apple\n•Orange\n•Lemon"},
		},
		{
			id:        "accessibility-list-labels-not-tagged",
			signature: "Document(P,L(LI(LBody),LI(LBody),LI(LBody)))",
			numbering: []string{"Decimal"},
			tagged:    []string{"FAIL\n1. 5 apples\n2. 9 oranges\n3. 3 lemons"},
		},
		{
			id:        "accessibility-list-ordered-numbering-absent",
			signature: "Document(P,L(LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody)))",
			numbering: []string{absent},
			tagged:    []string{"FAIL\n1.Apple\n2.Orange\n3.Lemon"},
		},
		{
			id:        "accessibility-list-ordered-numbering-disc",
			signature: "Document(P,L(LI(Lbl,LBody),LI(Lbl,LBody),LI(Lbl,LBody)))",
			numbering: []string{"Disc"},
			tagged:    []string{"FAIL\n1.Apple\n2.Orange\n3.Lemon"},
		},
		{
			id:        "accessibility-list-lbody-missing",
			signature: "Document(P,L(LI(Lbl,P),LI(Lbl,P),LI(Lbl,P),LI(Lbl,P)))",
			numbering: []string{"Decimal"},
			tagged:    []string{"FAIL\n1.This is item 1 of a numbered list\n2.This is item 2 of a numbered list\n3.This is item 3 of a numbered list\n4.This is item 4 of a numbered list"},
		},
		{
			id:        "accessibility-list-nested-under-li",
			signature: "Document(P,L(LI(Lbl,LBody(P),L(LI(Lbl,LBody(P)),LI(Lbl,LBody(P)))),LI(Lbl,LBody(P),L(LI(Lbl,LBody(P)),LI(Lbl,LBody(P)),LI(Lbl,LBody(P))))))",
			numbering: []string{"Decimal", "LowerAlpha", "LowerAlpha"},
			tagged:    []string{"FAIL\n1.Vegetables\na.Potato\nb.Onion\n2.Fruits\na.Apple\nb.Orange\nc.Lemon"},
		},
		{
			id:        "accessibility-list-substructure-outside-list",
			signature: "Document(P,L(LI(Lbl,LBody(P))),P,L(LI(Lbl,LBody(P))),P,L(LI(Lbl,LBody(P)),LI(Lbl,LBody(P))))",
			numbering: []string{"Decimal", "Decimal", "Decimal"},
			tagged:    []string{"FAIL\n1.This is item 1 of a numbered list\nThis is the second paragraph of item 1.\n2.This is item 2 of a numbered list\nThis is the second paragraph of item 2.\n3.This is item 3 of a numbered list\n4.This is item 4 of a numbered list"},
		},
		{
			id:        "accessibility-list-substructure-as-li",
			signature: "Document(P,L(LI(Lbl,LBody(P)),LI,LI(Lbl,LBody(P)),LI,LI(Lbl,LBody(P)),LI(Lbl,LBody(P))))",
			numbering: []string{"Decimal"},
			tagged:    []string{"FAIL\n1.This is item 1 of a numbered list\nThis is the second paragraph of item 1.\n2.This is item 2 of a numbered list\nThis is the second paragraph of item 2.\n3.This is item 3 of a numbered list\n4.This is item 4 of a numbered list"},
		},
		{
			id:        "accessibility-list-split-across-pages",
			numbering: []string{"Decimal", "Decimal"},
			tagged: []string{
				pdfAssociationSplitListFailureText(1, 20),
				pdfAssociationSplitListFailureText(21, 25),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			doc := openPDFAssociationFixture(t, test.id)
			defer closePDFAssociationFixture(t, doc)
			assert := func() {
				roots, err := doc.StructureTree()
				if err != nil {
					t.Fatal(err)
				}
				if test.signature != "" {
					if got := pdfAssociationStructureSignature(t, roots); got != test.signature {
						t.Fatalf("structure signature = %q, want %q", got, test.signature)
					}
				} else {
					assertPDFAssociationSplitListFailure(t, doc, roots)
				}

				var numbering []string
				for _, element := range collectPDFAssociationStructure(t, doc) {
					if element.Role() != "L" {
						continue
					}
					value, present := element.AttributesCopy()[document.Name("ListNumbering")]
					if !present {
						numbering = append(numbering, absent)
						continue
					}
					name, ok := value.(document.Name)
					if !ok {
						t.Fatalf("ListNumbering = %#v, want a name", value)
					}
					numbering = append(numbering, string(name))
				}
				if !reflect.DeepEqual(numbering, test.numbering) {
					t.Fatalf("ListNumbering values = %v, want %v", numbering, test.numbering)
				}

				pages, err := doc.CollectPages()
				if err != nil {
					t.Fatal(err)
				}
				if len(pages) != len(test.tagged) {
					t.Fatalf("page count = %d, want %d", len(pages), len(test.tagged))
				}
				for index, page := range pages {
					got, err := page.ExtractTextTagged(doc, document.DefaultTextExtractionOptions())
					if err != nil || got != test.tagged[index] {
						t.Fatalf("page %d tagged text = %q, %v; want %q", index, got, err, test.tagged[index])
					}
				}

				if test.id == "accessibility-list-substructure-as-li" {
					var bareTexts []string
					for _, element := range collectPDFAssociationStructure(t, doc) {
						children, err := element.ChildrenCopy()
						if err != nil {
							t.Fatal(err)
						}
						if element.Role() == "LI" && len(children) == 0 {
							text, err := element.Text(doc)
							if err != nil {
								t.Fatal(err)
							}
							bareTexts = append(bareTexts, strings.TrimSpace(text))
						}
					}
					if want := []string{"This is the second paragraph of item 1.", "This is the second paragraph of item 2."}; !reflect.DeepEqual(bareTexts, want) {
						t.Fatalf("bare LI text = %q, want %q", bareTexts, want)
					}
				}
			}
			assert()
			doc.ReleaseTransientCaches()
			assert()
		})
	}
}

func pdfAssociationSplitListFailureText(first, last int) string {
	lines := make([]string, 0, last-first+2)
	if first == 1 {
		lines = append(lines, "FAIL")
	}
	for number := first; number <= last; number++ {
		lines = append(lines, fmt.Sprintf("%d. A list which spreads over two pages is tagged as two separate L", number))
	}
	return strings.Join(lines, "\n")
}

func assertPDFAssociationSplitListFailure(t *testing.T, doc *document.Document, roots []document.StructElement) {
	t.Helper()
	if got := pdfAssociationStructureSignature(t, roots); !strings.HasPrefix(got, "Document(P,L(") {
		t.Fatalf("structure signature = %q, want Document with paragraph and split lists", got)
	}
	if len(roots) != 1 {
		t.Fatalf("structure roots = %d, want 1", len(roots))
	}
	children, err := roots[0].ChildrenCopy()
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 3 {
		t.Fatalf("root child count = %d, want 3", len(children))
	}
	if roles := []string{children[0].Role(), children[1].Role(), children[2].Role()}; !reflect.DeepEqual(roles, []string{"P", "L", "L"}) {
		t.Fatalf("root child roles = %v, want [P L L]", roles)
	}
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	for listIndex, wantItems := range []int{20, 5} {
		items, err := children[listIndex+1].ChildrenCopy()
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != wantItems {
			t.Fatalf("list %d item count = %d, want %d", listIndex, len(items), wantItems)
		}
		for itemIndex, item := range items {
			itemChildren, err := item.ChildrenCopy()
			if err != nil {
				t.Fatal(err)
			}
			if item.Role() != "LI" || len(itemChildren) != 2 || itemChildren[0].Role() != "Lbl" || itemChildren[1].Role() != "LBody" {
				t.Fatalf("list %d item %d = %s(%s), want LI(Lbl,LBody)", listIndex, itemIndex, item.Role(), pdfAssociationStructureSignature(t, itemChildren))
			}
		}
		refs := pdfAssociationStructurePageRefs(t, children[listIndex+1])
		if len(refs) != 1 || !refs[pages[listIndex].Ref()] {
			t.Fatalf("list %d page refs = %v, want only page %v", listIndex, refs, pages[listIndex].Ref())
		}
	}
}

func pdfAssociationStructureSignature(t *testing.T, elements []document.StructElement) string {
	t.Helper()
	parts := make([]string, len(elements))
	for index, element := range elements {
		children, err := element.ChildrenCopy()
		if err != nil {
			t.Fatal(err)
		}
		parts[index] = element.Role()
		if len(children) > 0 {
			parts[index] += "(" + pdfAssociationStructureSignature(t, children) + ")"
		}
	}
	return strings.Join(parts, ",")
}

func collectPDFAssociationStructure(t *testing.T, doc *document.Document) []document.StructElement {
	t.Helper()
	roots, err := doc.StructureTree()
	if err != nil {
		t.Fatal(err)
	}
	var elements []document.StructElement
	var walk func([]document.StructElement)
	walk = func(nodes []document.StructElement) {
		for _, node := range nodes {
			elements = append(elements, node)
			children, err := node.ChildrenCopy()
			if err != nil {
				t.Fatal(err)
			}
			walk(children)
		}
	}
	walk(roots)
	return elements
}

func rootChildRoles(t *testing.T, doc *document.Document) []string {
	t.Helper()
	roots, err := doc.StructureTree()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || roots[0].Role() != "Document" {
		t.Fatalf("structure roots = %#v, want one Document", roots)
	}
	children, err := roots[0].ChildrenCopy()
	if err != nil {
		t.Fatal(err)
	}
	roles := make([]string, len(children))
	for i, child := range children {
		roles[i] = child.Role()
	}
	return roles
}

func containsPDFAssociationString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func findPDFAssociationStructureRole(t *testing.T, doc *document.Document, role string) *document.StructElement {
	t.Helper()
	for _, element := range collectPDFAssociationStructure(t, doc) {
		if element.Role() == role {
			copy := element
			return &copy
		}
	}
	return nil
}

func pdfAssociationStructurePageRefs(t *testing.T, element document.StructElement) map[document.Ref]bool {
	t.Helper()
	refs := make(map[document.Ref]bool)
	var walk func(document.StructElement)
	walk = func(node document.StructElement) {
		if node.HasPage() {
			refs[node.Page()] = true
		}
		contents, err := node.ContentsCopy()
		if err != nil {
			t.Fatal(err)
		}
		for _, content := range contents {
			if content.HasPage() {
				refs[content.Page()] = true
			}
		}
		children, err := node.ChildrenCopy()
		if err != nil {
			t.Fatal(err)
		}
		for _, child := range children {
			walk(child)
		}
	}
	walk(element)
	return refs
}

func assertPDFAssociationTextOrder(t *testing.T, text string, runs []string) {
	t.Helper()
	text = strings.Join(strings.Fields(text), " ")
	position := 0
	for _, run := range runs {
		offset := strings.Index(text[position:], run)
		if offset < 0 {
			t.Fatalf("tagged text does not contain %q after byte %d: %q", run, position, text)
		}
		position += offset + len(run)
	}
}
