package document

import (
	"os"
	"testing"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestVerticalCIDAcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_vertical_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	texts, err := d.PageText(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || texts[0].Text() != "AB" {
		t.Fatalf("vertical CID texts = %#v", texts)
	}
	if len(texts[0].GlyphsCopy()) != 2 {
		t.Fatalf("vertical CID glyph count = %d", len(texts[0].GlyphsCopy()))
	}
	if !texts[0].Vertical() {
		t.Fatal("vertical CID text was not marked vertical")
	}
}

func TestNavigationSemanticsAcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_navigation_semantics.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := d.MetadataXML()
	if err != nil || string(metadata) != "<x:xmpmeta><dc:title>Quarterly report</dc:title></x:xmpmeta>" {
		t.Fatalf("XMP metadata = %q, err=%v", metadata, err)
	}
	labels, err := d.PageLabels()
	if err != nil || len(labels) != 1 || labels[0] != "Section 3" {
		t.Fatalf("page labels = %#v, err=%v", labels, err)
	}
	destinations, err := d.Destinations()
	if err != nil || len(destinations) != 1 {
		t.Fatalf("destinations = %#v, err=%v", destinations, err)
	}

	action, err := d.OpenActionWithError()
	if err != nil || action == nil || action.Kind() != "GoTo" {
		t.Fatalf("open action = %#v, err=%v", action, err)
	}
	outlines := make([]OutlineNode, 0, 1)
	for node, err := range d.Outline() {
		if err != nil {
			t.Fatal(err)
		}
		outlines = append(outlines, node)
	}
	if len(outlines) != 1 || outlines[0].Title() != "Chapter 1" {
		t.Fatalf("outlines = %#v", outlines)
	}

	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	annotations, err := d.CollectAnnotations(page)
	if err != nil || len(annotations) != 4 || annotations[0].ActionKind() != "URI" || annotations[0].URI() != "https://example.com/report" {
		t.Fatalf("annotations = %#v, err=%v", annotations, err)
	}
	note := annotations[1]
	if note.Subtype() != "Text" || note.Contents() != "Review note" || note.Name() != "note-1" || !note.HasPopup() || !note.HasInReplyTo() {
		t.Fatalf("text annotation metadata = %#v", note)
	}
	popup, err := note.PopupAnnotationWithError(d)
	if err != nil || popup == nil || popup.Subtype() != "Popup" {
		t.Fatalf("popup annotation = %#v, err=%v", popup, err)
	}
	parent, err := note.InReplyToAnnotationWithError(d)
	if err != nil || parent == nil || parent.Name() != "parent-note" {
		t.Fatalf("in-reply-to annotation = %#v, err=%v", parent, err)
	}
}

func TestResourceSemanticsAcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_resource_semantics.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}

	count := func(name string, sequence func(func(any, error) bool)) int {
		seen := 0
		sequence(func(value any, sequenceErr error) bool {
			if sequenceErr != nil {
				t.Fatalf("%s sequence error: %v", name, sequenceErr)
			}
			if value == nil {
				t.Fatalf("%s yielded nil value", name)
			}
			seen++
			return true
		})
		return seen
	}
	if got := count("ExtGState", func(yield func(any, error) bool) {
		for value, err := range page.ExtGStates(d) {
			if !yield(value, err) {
				return
			}
		}
	}); got != 2 {
		t.Fatalf("ExtGState selections = %d, want 2", got)
	}
	if got := count("ColorSpace", func(yield func(any, error) bool) {
		for value, err := range page.ColorSpaces(d) {
			if !yield(value, err) {
				return
			}
		}
	}); got != 2 {
		t.Fatalf("ColorSpace selections = %d, want 2", got)
	}
	if got := count("Pattern", func(yield func(any, error) bool) {
		for value, err := range page.Patterns(d) {
			if !yield(value, err) {
				return
			}
		}
	}); got != 1 {
		t.Fatalf("Pattern selections = %d, want 1", got)
	}
	if got := count("Shading", func(yield func(any, error) bool) {
		for value, err := range page.Shadings(d) {
			if !yield(value, err) {
				return
			}
		}
	}); got != 1 {
		t.Fatalf("Shading selections = %d, want 1", got)
	}
	if got := count("Properties", func(yield func(any, error) bool) {
		for value, err := range page.Properties(d) {
			if !yield(value, err) {
				return
			}
		}
	}); got != 2 {
		t.Fatalf("Properties selections = %d, want 2", got)
	}
	if got := count("XObject", func(yield func(any, error) bool) {
		for value, err := range page.XObjects(d) {
			if !yield(value, err) {
				return
			}
		}
	}); got != 1 {
		t.Fatalf("Form XObject selections = %d, want 1", got)
	}
}

func TestVisualSemanticsAcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_visual_semantics.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	count := func(name string, sequence func(func(any, error) bool)) int {
		seen := 0
		sequence(func(value any, sequenceErr error) bool {
			if sequenceErr != nil {
				t.Fatalf("%s sequence error: %v", name, sequenceErr)
			}
			if value == nil {
				t.Fatalf("%s yielded nil value", name)
			}
			seen++
			return true
		})
		return seen
	}
	if got := count("ColorSpace", func(yield func(any, error) bool) {
		for value, sequenceErr := range page.ColorSpaces(d) {
			if !yield(value, sequenceErr) {
				return
			}
		}
	}); got != 6 {
		t.Fatalf("ColorSpace selections = %d, want 6", got)
	}
	if got := count("Pattern", func(yield func(any, error) bool) {
		for value, sequenceErr := range page.Patterns(d) {
			if !yield(value, sequenceErr) {
				return
			}
		}
	}); got != 1 {
		t.Fatalf("Pattern selections = %d, want 1", got)
	}
	if got := count("Shading", func(yield func(any, error) bool) {
		for value, sequenceErr := range page.Shadings(d) {
			if !yield(value, sequenceErr) {
				return
			}
		}
	}); got != 1 {
		t.Fatalf("Shading selections = %d, want 1", got)
	}
	if got := count("ExtGState", func(yield func(any, error) bool) {
		for value, sequenceErr := range page.ExtGStates(d) {
			if !yield(value, sequenceErr) {
				return
			}
		}
	}); got != 2 {
		t.Fatalf("ExtGState selections = %d, want 2", got)
	}

	images := make([]ImageObject, 0, 2)
	for image, sequenceErr := range d.PageImagesSeq(page) {
		if sequenceErr != nil {
			t.Fatal(sequenceErr)
		}
		images = append(images, image)
	}
	if len(images) != 2 || !images[0].Inline() || !images[1].Inline() {
		t.Fatalf("inline images = %#v, want two inline images", images)
	}
	paths := make([]PathObject, 0, 2)
	for path, sequenceErr := range d.PagePathsSeq(page) {
		if sequenceErr != nil {
			t.Fatal(sequenceErr)
		}
		paths = append(paths, path)
	}
	if len(paths) < 2 {
		t.Fatalf("paths = %d, want at least two painted/clipped paths", len(paths))
	}
	flattened := 0
	for _, sequenceErr := range page.Flatten(d, contentconfig.Options{Filter: FilterAll}) {
		if sequenceErr != nil {
			t.Fatal(sequenceErr)
		}
		flattened++
	}
	if flattened < 6 {
		t.Fatalf("flattened objects = %d, want combined content objects", flattened)
	}
}
