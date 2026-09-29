package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
)

func TestContentOpClonePreservesAbsentOperands(t *testing.T) {
	cloned := cloneContentOps([]ContentOp{newContentOpBorrowed("q", nil, 0)})
	if cloned == nil || cloned[0].operandsValue() != nil {
		t.Fatalf("cloned operands = %#v, want nil", cloned[0].operandsValue())
	}
}

func TestContentBorrowedPayloadsAvoidDeepCopies(t *testing.T) {
	textValue := newTestText(contentdata.TextSpec{}, NewSimpleFont("Helvetica"), nil)
	text := &textValue
	path := &PathObject{}
	image := &ImageObject{}
	tag := &TagObject{}
	xobject := &XObjectObject{}
	extgstate := &ExtGStateObject{}
	colorSpace := &ColorSpaceObject{}
	pattern := &PatternObject{}
	shading := &ShadingObject{}
	properties := &PropertiesObject{}
	values := []ContentObject{
		{kind: ContentText, text: text},
		{kind: ContentPath, path: path},
		{kind: ContentImage, image: image},
		{kind: ContentTag, tag: tag},
		{kind: ContentXObject, xobject: xobject},
		{kind: ContentExtGState, extgstate: extgstate},
		{kind: ContentColorSpace, colorspace: colorSpace},
		{kind: ContentPattern, pattern: pattern},
		{kind: ContentShading, shading: shading},
		{kind: ContentProperties, properties: properties},
	}
	if values[0].TextBorrowed() != text || values[1].PathBorrowed() != path ||
		values[2].ImageBorrowed() != image || values[3].TagBorrowed() != tag ||
		values[4].XObjectBorrowed() != xobject || values[5].ExtGStateBorrowed() != extgstate ||
		values[6].ColorSpaceBorrowed() != colorSpace || values[7].PatternBorrowed() != pattern ||
		values[8].ShadingBorrowed() != shading || values[9].PropertiesBorrowed() != properties {
		t.Fatal("borrowed content payload did not preserve the parser-owned view")
	}
	if values[0].TextCopy() == text || values[4].XObjectCopy() == xobject {
		t.Fatal("copy accessors returned the borrowed payload")
	}
}

func TestContentFinalizeWithErrorReportsDeferredFontFailures(t *testing.T) {
	font := NewSimpleFont("CFF")
	font.cffData = []byte{1, 0, 4}

	glyph := newTestGlyphWithFont(font)
	if snapshot, err := glyph.FinalizeWithError(); err == nil || snapshot.font != nil {
		t.Fatalf("glyph finalize = %#v, err=%v", snapshot, err)
	}

	text := newTestText(contentdata.TextSpec{}, font, []GlyphObject{newTestGlyphWithFont(font)})
	if snapshot, err := text.FinalizeWithError(); err == nil || snapshot.font != nil || snapshot.glyphs != nil {
		t.Fatalf("text finalize = %#v, err=%v", snapshot, err)
	}

	content := ContentObject{kind: ContentText, text: &text}
	if err := text.ValidateWithError(); err == nil {
		t.Fatal("text validation did not report deferred font failure")
	}
	if snapshot, err := content.FinalizeWithError(); err == nil || snapshot.text != nil {
		t.Fatalf("content finalize = %#v, err=%v", snapshot, err)
	}
}

func TestContentFinalizeWithErrorReportsDeferredImageFailures(t *testing.T) {
	image := ImageObject{
		colorSpace:           "Indexed",
		indexedLookupData:    []byte{1, 2, 3},
		indexedLookupFilters: []string{"UnsupportedFilter"},
	}
	content := ContentObject{kind: ContentImage, image: &image}
	if snapshot, err := content.FinalizeWithError(); err == nil || snapshot.image != nil {
		t.Fatalf("content image finalize = %#v, err=%v", snapshot, err)
	}
}

func TestContentFinalizeWithErrorReturnsStableImageSnapshot(t *testing.T) {
	image := ImageObject{
		colorSpace:    "Indexed",
		indexedLookup: []byte{1, 2, 3},
		indexedLookupCache: &imageDecodeCache{
			data: []byte{1, 2, 3},
		},
	}
	content := ContentObject{kind: ContentImage, image: &image}
	snapshot, err := content.FinalizeWithError()
	if err != nil || snapshot.image == nil {
		t.Fatalf("content image finalize = %#v, err=%v", snapshot, err)
	}
	if snapshot.image.indexedLookupCache != nil {
		t.Fatal("content image snapshot retained the lazy lookup cache")
	}
}

func TestContentFinalizeWithErrorReportsDeferredColorSpaceFailures(t *testing.T) {
	colorSpace := ColorSpaceObject{
		data: contentdata.NewColorSpaceSelection(contentdata.ColorSpaceSelectionSpec{Name: "CS1"}),
		info: func() ImageColorSpace {
			value := newImageColorSpace("Indexed", 0)
			value.lookupData = []byte{1, 2, 3}
			value.lookupFilters = []string{"UnsupportedFilter"}
			return value
		}(),
	}
	content := ContentObject{kind: ContentColorSpace, colorspace: &colorSpace}
	if snapshot, err := content.FinalizeWithError(); err == nil || snapshot.colorspace != nil {
		t.Fatalf("content color-space finalize = %#v, err=%v", snapshot, err)
	}
}

func TestContentFinalizeWithErrorReturnsStableTextSnapshot(t *testing.T) {
	font := NewSimpleFont("Helvetica")
	text := newTestText(contentdata.TextSpec{}, font, []GlyphObject{newTestGlyphWithFont(font)})

	snapshot, err := text.FinalizeWithError()
	if err != nil || snapshot.font == nil || len(snapshot.glyphs) != 1 || snapshot.glyphs[0].font == nil {
		t.Fatalf("text snapshot = %#v, err=%v", snapshot, err)
	}
	if snapshot.font.document != nil || snapshot.glyphs[0].font.document != nil {
		t.Fatal("text snapshot retained document ownership")
	}
}

func TestContentCopiesWithErrorReportDeferredFontFailures(t *testing.T) {
	font := NewSimpleFont("CFF")
	font.cffData = []byte{1, 0, 4}
	text := newTestText(contentdata.TextSpec{}, font, []GlyphObject{newTestGlyphWithFont(font)})

	if glyphs, err := text.GlyphsCopyWithError(); err == nil || glyphs != nil {
		t.Fatalf("glyph copy = %#v, err=%v", glyphs, err)
	}
	content := ContentObject{kind: ContentText, text: &text}
	if copy, err := content.TextCopyWithError(); err == nil || copy != nil {
		t.Fatalf("text copy = %#v, err=%v", copy, err)
	}
}

func TestLayoutFinalizeWithErrorPropagatesNestedFontFailures(t *testing.T) {
	font := NewSimpleFont("CFF")
	font.cffData = []byte{1, 0, 4}
	glyph := newTestGlyphWithFont(font)
	word := TextWord{glyphs: []GlyphObject{glyph}}
	line := TextLine{glyphs: []GlyphObject{glyph}, words: []TextWord{word}}
	paragraph := TextParagraph{lines: []TextLine{line}}
	box := TextBox{lines: []TextLine{line}}
	group := TextGroup{boxes: []TextBox{box}}
	result := LayoutResult{lines: []TextLine{line}, paragraphs: []TextParagraph{paragraph}, textBoxes: []TextBox{box}, textGroups: []TextGroup{group}}

	if snapshot, err := word.FinalizeWithError(); err == nil || snapshot.glyphs != nil {
		t.Fatalf("word finalize = %#v, err=%v", snapshot, err)
	}
	if snapshot, err := line.FinalizeWithError(); err == nil || snapshot.glyphs != nil {
		t.Fatalf("line finalize = %#v, err=%v", snapshot, err)
	}
	if snapshot, err := result.FinalizeWithError(); err == nil || snapshot.lines != nil {
		t.Fatalf("layout finalize = %#v, err=%v", snapshot, err)
	}
}
