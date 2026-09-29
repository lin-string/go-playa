package page_test

import (
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/internal/testfixture"
	"github.com/lin-string/go-playa/page"
)

func TestPublicPageSequences(t *testing.T) {
	if page.ErrPageNotFound == nil {
		t.Fatal("ErrPageNotFound is nil")
	}
	if page.ErrNilDocument == nil {
		t.Fatal("ErrNilDocument is nil")
	}
	var _ *page.ParseError
	textOptions := page.DefaultTextExtractionOptions()
	if textOptions.BBox != nil {
		t.Fatalf("default text extraction options = %#v", textOptions)
	}
	layout := page.DefaultLayoutOptions()
	if layout.WordMargin != 0.1 || layout.LineMargin != 0.5 || layout.LineOverlap != 0.5 || layout.CharMargin != 2 || layout.BoxesFlow == nil || *layout.BoxesFlow != 0.5 {
		t.Fatalf("default layout options = %#v", layout)
	}
	options := page.DefaultContentOptions()
	if options.Filter != page.FilterAll || options.RestrictOps != nil {
		t.Fatalf("default content options = %#v", options)
	}
	var _ page.FontMetadata
	var _ page.Action
	var _ page.Destination
	var _ page.StructElement
	var _ page.ContentSection
	var _ page.ContentSequence
	var _ page.DashPattern
	var _ page.LayoutItem
	var _ page.LayoutItemKind
	var _ page.BBoxProvider
	_ = page.LayoutTextBox
	_ = page.LayoutImage
	_ = page.LayoutPath
	_ = page.LayoutXObject
	var _ page.Object
	var _ page.Ref
	var _ page.Dict
	var _ page.Array
	var _ page.Null
	var _ page.Bool
	var _ page.Number
	var _ page.Name
	var _ page.String
	var _ page.Keyword
	var _ page.DecodedImage
	var _ page.ImageColorSpace
	var _ page.Stream
	var _ page.Matrix
	var _ page.ContentKind
	var _ page.TokenKind
	_ = page.DefaultGraphicsState
	var p page.Page
	var x page.XObjectObject
	var d *document.Document
	_ = x.BBox
	_ = x.Name
	_ = x.Ref
	_ = x.Page
	_ = x.HasPage
	_ = x.StreamCopy
	_ = x.Matrix
	_ = x.ParentKey
	_ = x.HasParentKey
	_ = x.Path
	_ = x.MarkedTag
	_ = x.GState
	_ = x.Buffer
	_ = x.Get
	_ = x.Has
	_ = x.GroupCopy
	_ = x.ResourcesCopy
	_ = x.ResourcesWithError
	_ = x.ParentKey
	_ = x.HasParentKey
	_ = x.Fonts
	_ = x.FontsSeq
	_ = x.MarkedContent
	_ = x.MarkedContentByMCIDSeq
	_ = x.MarkedContentIndex
	_ = x.MarkedContentSequence
	_ = x.Len
	_ = (page.MarkedContentIndex{}).RootsCopy
	_ = (page.MarkedContentIndex{}).ByMCIDCopy
	_ = p.MarkedContentSequence
	_ = (page.ContentSection{}).MCID
	_ = (page.ContentSection{}).HasMCID
	_ = (page.ContentSection{}).Len
	_ = (page.ContentSection{}).ObjectsSeq
	_ = (page.ContentSection{}).ObjectsCopy
	_ = (page.ContentSection{}).ObjectsCopyWithError
	_ = (page.ContentSection{}).TextsSeq
	_ = (page.ContentSection{}).TextsCopy
	_ = (page.ContentSection{}).Finalize
	_ = (page.ContentSection{}).FinalizeWithError
	_ = (page.ContentSequence{}).Len
	_ = (page.ContentSequence{}).At
	_ = (page.ContentSequence{}).PageOrderSeq
	_ = (page.ContentSequence{}).PageOrderCopy
	_ = (page.ContentSequence{}).Finalize
	_ = (page.ContentSequence{}).FinalizeWithError
	_ = x.StructureSeq
	_ = x.Structure
	_ = (page.PageStructure{}).ElementsCopy
	_ = (page.PageStructure{}).ElementsCopyWithError
	_ = (page.PageStructure{}).ByMCIDCopy
	_ = (page.PageStructure{}).ByMCIDCopyWithError
	_ = (page.PageStructure{}).Finalize
	_ = (page.PageStructure{}).FinalizeWithError
	_ = (page.PageStructureEntry{}).ElementCopy
	_ = (page.PageStructureEntry{}).ElementsCopy
	_ = (page.PageStructureEntry{}).ElementCopyWithError
	_ = (page.PageStructureEntry{}).ElementsCopyWithError
	_ = (page.PageStructureEntry{}).Finalize
	_ = (page.PageStructureEntry{}).FinalizeWithError
	_ = (page.ContentObject{}).TextCopy
	_ = (page.ContentObject{}).Kind
	_ = (page.ContentObject{}).TextCopyWithError
	_ = (page.ContentObject{}).PathCopy
	_ = (page.ContentObject{}).ImageCopy
	_ = (page.ContentObject{}).TagCopy
	_ = (page.ContentObject{}).XObjectCopy
	_ = (page.ContentObject{}).ExtGStateCopy
	_ = (page.ContentObject{}).ColorSpaceCopy
	_ = (page.ContentObject{}).PatternCopy
	_ = (page.ContentObject{}).ShadingCopy
	_ = (page.ContentObject{}).PropertiesCopy
	_ = (page.ContentObject{}).Len
	_ = (page.TextObject{}).FontCopy
	_ = (page.TextObject{}).Len
	_ = (page.TextObject{}).FontCopyWithError
	_ = (page.TextObject{}).GlyphsCopyWithError
	_ = (page.GlyphObject{}).FontCopy
	_ = (page.GlyphObject{}).FontCopyWithError
	_ = (page.GlyphObject{}).ContentSeq
	_ = (page.GlyphObject{}).Len
	_ = (page.TextWord{}).Text
	_ = (page.TextWord{}).BBox
	_ = (page.TextWord{}).IsHoverlap
	_ = (page.TextWord{}).HDistance
	_ = (page.TextWord{}).Hoverlap
	_ = (page.TextWord{}).IsVOverlap
	_ = (page.TextWord{}).VDistance
	_ = (page.TextWord{}).VOverlap
	_ = (page.TextLine{}).Text
	_ = (page.TextLine{}).BBox
	_ = (page.TextLine{}).IsHoverlap
	_ = (page.TextLine{}).HDistance
	_ = (page.TextLine{}).Hoverlap
	_ = (page.TextLine{}).IsVOverlap
	_ = (page.TextLine{}).VDistance
	_ = (page.TextLine{}).VOverlap
	_ = (page.TextLine{}).Vertical
	_ = (page.TextParagraph{}).Text
	_ = (page.TextParagraph{}).BBox
	_ = (page.TextParagraph{}).Vertical
	_ = (page.TextParagraph{}).IsHoverlap
	_ = (page.TextParagraph{}).HDistance
	_ = (page.TextParagraph{}).Hoverlap
	_ = (page.TextParagraph{}).IsVOverlap
	_ = (page.TextParagraph{}).VDistance
	_ = (page.TextParagraph{}).VOverlap
	_ = (page.TextBox{}).Text
	_ = (page.TextBox{}).BBox
	_ = (page.TextBox{}).Vertical
	_ = (page.TextBox{}).Index
	_ = (page.TextBox{}).IsHoverlap
	_ = (page.TextBox{}).HDistance
	_ = (page.TextBox{}).Hoverlap
	_ = (page.TextBox{}).IsVOverlap
	_ = (page.TextBox{}).VDistance
	_ = (page.TextBox{}).VOverlap
	_ = (page.TextGroup{}).Text
	_ = (page.TextGroup{}).BBox
	_ = (page.TextGroup{}).Vertical
	_ = (page.TextGroup{}).ChildrenSeq
	_ = (page.TextGroup{}).ChildrenCopy
	_ = (page.TextGroup{}).ChildrenCopyWithError
	_ = (page.TextGroup{}).IsHoverlap
	_ = (page.TextGroup{}).HDistance
	_ = (page.TextGroup{}).Hoverlap
	_ = (page.TextGroup{}).IsVOverlap
	_ = (page.TextGroup{}).VDistance
	_ = (page.TextGroup{}).VOverlap
	_ = (page.TextGroupChild{}).IsBox
	_ = (page.TextGroupChild{}).IsGroup
	_ = (page.TextGroupChild{}).BoxCopy
	_ = (page.TextGroupChild{}).GroupCopy
	_ = (page.FontResource{}).FontCopy
	_ = (page.FontResource{}).FontCopyWithError
	_ = (page.FontResource{}).Name
	_ = (page.FontResource{}).Finalize
	_ = (page.FontResource{}).FinalizeWithError
	_ = (page.Annotation{}).ActionValueCopy
	_ = (page.Annotation{}).ActionValueCopyWithError
	_ = (page.Annotation{}).DestinationWithError
	_ = (page.Annotation{}).DestCopy
	_ = (page.Annotation{}).Subtype
	_ = (page.Annotation{}).Page
	_ = (page.Annotation{}).HasPage
	_ = (page.Annotation{}).InReplyTo
	_ = (page.Annotation{}).HasInReplyTo
	_ = (page.Annotation{}).Popup
	_ = (page.Annotation{}).HasPopup
	_ = (page.Annotation{}).Rect
	_ = (page.Annotation{}).Contents
	_ = (page.Annotation{}).URI
	_ = (page.Annotation{}).ActionKind
	_ = (page.Annotation{}).Border
	_ = (page.Annotation{}).HasBorder
	_ = (page.Annotation{}).Name
	_ = (page.Annotation{}).Modified
	_ = (page.Annotation{}).ParentKey
	_ = (page.Annotation{}).HasParentKey
	_ = (page.Annotation{}).Flags
	_ = (page.Annotation{}).HasFlags
	_ = x.Texts
	_ = x.Glyphs
	_ = x.Paths
	_ = x.Images
	_ = x.Shadings
	_ = x.Patterns
	_ = x.ExtGStates
	_ = x.ColorSpaces
	_ = x.Properties
	_ = x.Tags
	_ = x.Parent
	_ = x.PageObject
	_ = x.Finalize
	_ = p.Finalize
	_ = p.ResourcesWithError
	_ = p.Layout
	var _ page.LayoutOptions
	var _ page.LayoutResult
	var _ page.TextWord
	var _ page.TextLine
	var _ page.TextParagraph
	var _ page.TextBox
	var _ page.TextGroup
	_ = (page.TextWord{}).GlyphsSeq
	_ = (page.TextLine{}).WordsSeq
	_ = (page.TextParagraph{}).LinesSeq
	_ = (page.TextBox{}).LinesSeq
	_ = (page.TextGroup{}).BoxesSeq
	_ = (page.Annotation{}).Finalize
	_ = (page.Annotation{}).FinalizeWithError
	_ = (page.Annotation{}).QuadPointsCopy
	_ = (page.Annotation{}).ColorCopy
	_ = (page.Annotation{}).DictCopy
	_ = (page.Annotation{}).ActionCopy
	_ = (page.Annotation{}).AppearanceCopy
	_ = (page.MarkedContent{}).Finalize
	_ = (page.MarkedContentIndex{}).Finalize
	_ = (page.FormField{}).Finalize
	_ = (page.FormField{}).FinalizeWithError
	_ = (page.FormField{}).Name
	_ = (page.FormField{}).FullName
	_ = (page.FormField{}).Page
	_ = (page.FormField{}).HasPage
	_ = (page.FormField{}).Parent
	_ = (page.FormField{}).HasParent
	_ = (page.FormField{}).FieldType
	_ = (page.FormField{}).Flags
	_ = (page.FormField{}).HasFlags
	_ = (page.FormField{}).Value
	_ = (page.FormField{}).DefaultValue
	_ = (page.FormField{}).DefaultAppearance
	_ = (page.FormField{}).Rect
	_ = (page.FormField{}).HasRect
	_ = (page.FormField{}).IsWidget
	_ = (page.StructElement{}).Type
	_ = (page.StructElement{}).StructureType
	_ = (page.StructElement{}).Role
	_ = (page.StructElement{}).RawRole
	_ = (page.StructElement{}).Title
	_ = (page.StructElement{}).Language
	_ = (page.StructElement{}).AlternateDescription
	_ = (page.StructElement{}).ActualText
	_ = (page.StructElement{}).AbbreviationExpansion
	_ = (page.StructElement{}).ClassName
	_ = (page.StructElement{}).Page
	_ = (page.StructElement{}).HasPage
	_ = (page.StructElement{}).Parent
	_ = (page.StructElement{}).HasParent
	_ = (page.StructElement{}).MCID
	_ = (page.StructElement{}).HasMCID
	_ = (page.StructElement{}).IsMCR
	_ = (page.StructElement{}).ObjectRef
	_ = (page.StructElement{}).HasObject
	_ = (page.StructElement{}).BBox
	_ = (page.StructElement{}).HasBBox
	_ = (page.FormField{}).ValuesCopy
	_ = (page.FormField{}).DefaultValuesCopy
	_ = (page.FormField{}).OptionsCopy
	_ = (page.FormField{}).OptionValuesCopy
	_ = (page.FormField{}).SelectedCopy
	kidsCopy, kidsCopyErr := (page.FormField{}).KidsCopy()
	_, _ = kidsCopy, kidsCopyErr
	_ = (page.FormField{}).DictCopy
	_ = (page.Annotation{}).Parent
	_ = (page.Annotation{}).ParentWithError
	_ = (page.Annotation{}).PageObject
	_ = (page.Annotation{}).BBox
	_ = (page.Annotation{}).Destination
	_ = (page.Annotation{}).InReplyToAnnotation
	_ = (page.Annotation{}).InReplyToAnnotationWithError
	_ = (page.Annotation{}).PopupAnnotation
	_ = (page.Annotation{}).PopupAnnotationWithError
	_ = (page.FormField{}).PageObject
	_ = (page.FormField{}).ParentFieldWithError
	_ = (page.ImageObject{}).PageObject
	_ = (page.ImageObject{}).Name
	_ = (page.ImageObject{}).Inline
	_ = (page.ImageObject{}).Offset
	_ = (page.ImageObject{}).Page
	_ = (page.ImageObject{}).HasPage
	_ = (page.ImageObject{}).Width
	_ = (page.ImageObject{}).Height
	_ = (page.ImageObject{}).BPC
	_ = (page.ImageObject{}).ColorSpace
	_ = (page.ImageObject{}).Components
	_ = (page.ImageObject{}).IndexedComponents
	_ = (page.ImageObject{}).IndexedHigh
	_ = (page.ImageObject{}).ImageMask
	_ = (page.ImageObject{}).HasMask
	_ = (page.ImageObject{}).HasSoftMask
	_ = (page.ImageObject{}).BBox
	_ = (page.ImageObject{}).ParentKey
	_ = (page.ImageObject{}).HasParentKey
	_ = (page.ImageObject{}).GState
	_ = (page.ImageObject{}).MarkedTag
	_ = (page.ImageObject{}).ActualText
	_ = (page.ImageObject{}).MCID
	_ = (page.ImageObject{}).HasMCID
	_ = (page.ImageObject{}).FinalizeWithError
	_ = (page.ImageObject{}).IndexedLookupWithError
	_ = (page.ImageObject{}).ColorSpaceInfoCopyWithError
	_ = (page.ShadingObject{}).DictCopy
	_ = (page.ShadingObject{}).StreamCopy
	_ = (page.ShadingObject{}).Finalize
	_ = (page.PatternObject{}).DictCopy
	_ = (page.PatternObject{}).StreamCopy
	_ = (page.PatternObject{}).Finalize
	_ = (page.ExtGStateObject{}).DictCopy
	_ = (page.ExtGStateObject{}).Finalize
	_ = (page.ColorSpaceObject{}).SpecCopy
	_ = (page.ColorSpaceObject{}).Finalize
	_ = (page.ColorSpaceObject{}).FinalizeWithError
	_ = (page.PropertiesObject{}).DictCopy
	_ = (page.PropertiesObject{}).Finalize
	_ = (page.TextObject{}).Parent
	_ = (page.TextObject{}).ParentWithError
	_ = (page.GlyphObject{}).Parent
	_ = (page.GlyphObject{}).ParentWithError
	_ = (*page.Font).GlyphBBoxWithError
	_ = (page.PathObject{}).Parent
	_ = (page.PathObject{}).Len
	_ = (page.PathObject{}).ParentWithError
	_ = (page.PathObject{}).Page
	_ = (page.PathObject{}).HasPage
	_ = (page.PathObject{}).Stroke
	_ = (page.PathObject{}).Fill
	_ = (page.PathObject{}).EvenOdd
	_ = (page.PathObject{}).Clip
	_ = (page.PathObject{}).ClipEvenOdd
	_ = (page.PathObject{}).BBox
	_ = (page.PathObject{}).GState
	_ = (page.PathObject{}).MarkedTag
	_ = (page.PathObject{}).ActualText
	_ = (page.PathObject{}).MCID
	_ = (page.PathObject{}).HasMCID
	_ = (page.ImageObject{}).ParentWithError
	_ = (page.ImageObject{}).Len
	_ = (page.TagObject{}).ParentWithError
	_ = (page.TagObject{}).Len
	_ = (page.ContentObject{}).ParentWithError
	_ = (page.XObjectObject{}).ParentWithError
	_ = (page.TagObject{}).Parent
	_ = (page.TagObject{}).PropertiesCopy
	_ = (page.MarkedContent{}).Parent
	_ = (page.ContentObject{}).PageObject
	_ = (page.ContentObject{}).Parent
	_ = (page.Color{}).ValuesCopy
	_ = (page.Color{}).Finalize
	_ = (page.GraphicsState{}).Finalize
	_ = (page.GraphicsState{}).Dash
	_ = (page.PathSegment{}).PointsCopy
	_ = (page.PathSegment{}).Finalize
	assertSeq2[page.ContentObject](p.Interp(d, page.ContentOptions{}))
	assertSeq2[page.ContentObject](p.Flatten(d, page.ContentOptions{}))
	assertSeq2[page.TextObject](p.Texts(d))
	assertSeq2[page.GlyphObject](p.Glyphs(d))
	assertSeq2[page.PathObject](p.Paths(d))
	assertSeq2[page.ImageObject](p.Images(d))
	assertSeq2[page.ShadingObject](p.Shadings(d))
	assertSeq2[page.PatternObject](p.Patterns(d))
	assertSeq2[page.ExtGStateObject](p.ExtGStates(d))
	assertSeq2[page.ColorSpaceObject](p.ColorSpaces(d))
	assertSeq2[page.PropertiesObject](p.Properties(d))
	assertSeq2[page.XObjectObject](p.XObjects(d))
	assertSeq2[page.Annotation](p.Annotations(d))
	_ = p.CollectAnnotations
	assertSeq2[page.TagObject](p.Tags(d))
	assertSeq2[page.PageStructureEntry](p.StructureSeq(d))
	assertSeq2[page.FontResource](p.FontsSeq(d))
	assertSeq2[page.Stream](p.Streams(d))
	assertSeq2[page.Token](p.Tokens(d))
	assertSeq2[page.ContentOp](p.Contents(d))
	_ = (page.Stream{}).Buffer
	_ = (page.Stream{}).DecodedBuffer
	_ = (page.Stream{}).DecodedBufferWithError
	_ = (page.Stream{}).DecodedBufferWithDocument
	_ = (page.Stream{}).DecodedBufferWithDocumentWithError
	_ = (page.Stream{}).DictCopy
	_ = (page.Stream{}).Get
	_ = (page.Stream{}).Has
	_ = (page.Stream{}).Finalize
	_ = (page.Token{}).Finalize
	_ = p.ExtractText
	_ = (page.TextObject{}).PageObject
	_ = (page.GlyphObject{}).PageObject
	_ = (page.PathObject{}).PageObject
	_ = p.ExtractTextTagged
	_ = p.ExtractTextUntagged
	_ = p.MediaBox
	_ = p.Index
	_ = p.Space
	_ = p.CropBox
	_ = p.BBoxWithError
	_ = p.MediaBoxWithError
	_ = p.CropBoxWithError
	_ = p.UserUnitWithError
	_ = p.RotationWithError
	_ = p.ParentKeyWithError
	_ = p.SizeWithError
	_ = p.MatrixInWithError
	_ = p.SetInitialCTM
	_ = p.SetInitialCTMWithError
	_ = p.DictCopy
	_ = p.Get
	_ = p.Has
	_ = (page.PageLabelSpec{}).Format
	_ = (page.Page{}).LabelWithError
	_ = (page.MarkedContent{}).PageObject
	_ = testing.Short
}

func TestPublicPageMetadataIsReadOnly(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_page_labels.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if p.Number() != 1 || p.Index() != 0 || p.Ref().Object == 0 {
		t.Fatalf("page identity = number:%d index:%d ref:%v", p.Number(), p.Index(), p.Ref())
	}
	if p.Space() != document.CoordinateSpaceScreen {
		t.Fatalf("page space = %q, want screen", p.Space())
	}
}

func TestPublicPageTextsSequenceIsRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var first []string
	for text, err := range p.Texts(doc) {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, text.Text())
		break
	}
	if len(first) != 1 || first[0] != "中国" {
		t.Fatalf("first page text sequence values = %#v", first)
	}
	var repeated []string
	for text, err := range p.Texts(doc) {
		if err != nil {
			t.Fatal(err)
		}
		repeated = append(repeated, text.Text())
	}
	if len(repeated) != 1 || repeated[0] != "中国" {
		t.Fatalf("repeat page text sequence values = %#v", repeated)
	}
}

func TestPublicPageGlyphsSequenceIsRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var first []string
	for glyph, err := range p.Glyphs(doc) {
		if err != nil {
			t.Fatal(err)
		}
		first = append(first, glyph.Chars())
		break
	}
	if len(first) != 1 || first[0] != "中" {
		t.Fatalf("first page glyph sequence values = %#v", first)
	}
	var repeated []string
	for glyph, err := range p.Glyphs(doc) {
		if err != nil {
			t.Fatal(err)
		}
		repeated = append(repeated, glyph.Chars())
	}
	if len(repeated) != 2 || repeated[0] != "中" || repeated[1] != "国" {
		t.Fatalf("repeat page glyph sequence values = %#v", repeated)
	}
}

func TestPublicPageImagesSequenceIsRepeatableAndOwned(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_rgb_image.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var first page.ImageObject
	count := 0
	for image, err := range p.Images(doc) {
		if err != nil {
			t.Fatal(err)
		}
		first = image
		count++
	}
	if count != 1 || first.Name() != "Im1" || first.Width() != 2 || first.Height() != 1 || first.ColorSpace() != "DeviceRGB" {
		t.Fatalf("first page image sequence value = %#v, count=%d", first, count)
	}
	second, err := first.FinalizeWithError()
	if err != nil || second.Name() != first.Name() {
		t.Fatalf("image finalize = %#v, err=%v", second, err)
	}
	var repeated int
	for image, err := range p.Images(doc) {
		if err != nil || image.Name() != "Im1" {
			t.Fatalf("repeat page image = %#v, err=%v", image, err)
		}
		repeated++
	}
	if repeated != 1 {
		t.Fatalf("repeat page image count = %d", repeated)
	}
}

func TestPublicPageFontsSequenceIsRepeatableAndSnapshotable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var first page.FontResource
	count := 0
	for resource, err := range p.FontsSeq(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			first = resource
		}
		count++
	}
	if count == 0 || first.Name() == "" {
		t.Fatalf("page font sequence = %#v, count=%d", first, count)
	}
	metadata, ok := first.Metadata()
	if !ok || metadata.Name() == "" {
		t.Fatalf("font metadata = %#v, ok=%v", metadata, ok)
	}
	if snapshot, err := first.FinalizeWithError(); err != nil || snapshot.Name() != first.Name() {
		t.Fatalf("font snapshot = %#v, err=%v", snapshot, err)
	}
	repeated := 0
	for resource, err := range p.FontsSeq(doc) {
		if err != nil || resource.Name() == "" {
			t.Fatalf("repeat page font = %#v, err=%v", resource, err)
		}
		repeated++
	}
	if repeated != count {
		t.Fatalf("repeat page font count = %d, want %d", repeated, count)
	}
}

func TestPublicPageContentsSequenceIsRepeatableAndEarlyStoppable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	first := 0
	for _, err := range p.Contents(doc) {
		if err != nil {
			t.Fatal(err)
		}
		first++
		break
	}
	if first != 1 {
		t.Fatalf("early page content count = %d", first)
	}
	repeated := 0
	for _, err := range p.Contents(doc) {
		if err != nil {
			t.Fatal(err)
		}
		repeated++
	}
	if repeated == 0 {
		t.Fatal("repeat page content sequence was empty")
	}
}

func TestPublicPageMarkedContentSequenceIsRepeatableAndOwned(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_tagged_text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var first page.MarkedContent
	count := 0
	for marked, err := range p.MarkedContent(doc) {
		if err != nil {
			t.Fatal(err)
		}
		first = marked
		count++
		break
	}
	if count != 1 || first.Tag() != "P" || !first.HasMCID() || first.MCID() != 0 {
		t.Fatalf("first marked content = %#v, count=%d", first, count)
	}
	children := first.ChildrenCopy()
	if len(children) != 0 {
		t.Fatalf("unexpected marked content children = %#v", children)
	}
	repeated := 0
	for marked, err := range p.MarkedContent(doc) {
		if err != nil || marked.Tag() != "P" || marked.MCID() != 0 {
			t.Fatalf("repeat marked content = %#v, err=%v", marked, err)
		}
		repeated++
	}
	if repeated != 1 {
		t.Fatalf("repeat marked content count = %d", repeated)
	}
}

func TestPublicPageLayoutReturnsRepeatableOwnedResult(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	first, err := p.Layout(doc, page.DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	lines := first.LinesCopy()
	if len(lines) == 0 || lines[0].Text() == "" {
		t.Fatalf("layout lines = %#v", lines)
	}
	snapshot, err := first.FinalizeWithError()
	if err != nil || len(snapshot.LinesCopy()) != len(lines) {
		t.Fatalf("layout snapshot = %#v, err=%v", snapshot, err)
	}
	second, err := p.Layout(doc, page.DefaultLayoutOptions())
	if err != nil {
		t.Fatal(err)
	}
	if got := second.LinesCopy(); len(got) != len(lines) || got[0].Text() != lines[0].Text() {
		t.Fatalf("repeat layout lines = %#v, want %#v", got, lines)
	}
}

func TestPublicPageStructureSequenceIsRepeatableAndSnapshotable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_tagged_text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var first page.PageStructureEntry
	count := 0
	for entry, err := range p.StructureSeq(doc) {
		if err != nil {
			t.Fatal(err)
		}
		first = entry
		count++
		break
	}
	if count != 1 || first.Index() != 0 {
		t.Fatalf("first page structure entry = %#v, count=%d", first, count)
	}
	if element := first.ElementCopy(); element == nil || element.Role() != "P" || element.Title() != "Financial summary" {
		t.Fatalf("page structure element = %#v", element)
	}
	repeated := 0
	for entry, err := range p.StructureSeq(doc) {
		if err != nil || entry.Index() != 0 {
			t.Fatalf("repeat page structure entry = %#v, err=%v", entry, err)
		}
		repeated++
	}
	if repeated == 0 {
		t.Fatal("repeat page structure sequence was empty")
	}
}

func TestPublicPageTaggedAndUntaggedTextExtraction(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_tagged_text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	tagged, err := p.ExtractTextTagged(doc, page.DefaultTextExtractionOptions())
	if err != nil || tagged != "AB" {
		t.Fatalf("tagged text = %q, err=%v", tagged, err)
	}
	untagged, err := p.ExtractTextUntagged(doc, page.DefaultTextExtractionOptions())
	if err != nil || untagged != "AB" {
		t.Fatalf("untagged text = %q, err=%v", untagged, err)
	}
}

func TestPublicPageStreamsAndTokensAreRepeatable(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	streams := 0
	for stream, err := range p.Streams(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if stream.Buffer() == nil {
			t.Fatal("stream buffer is nil")
		}
		streams++
		break
	}
	if streams != 1 {
		t.Fatalf("early stream count = %d", streams)
	}
	repeatedStreams := 0
	for _, err := range p.Streams(doc) {
		if err != nil {
			t.Fatal(err)
		}
		repeatedStreams++
	}
	if repeatedStreams == 0 {
		t.Fatal("repeat stream sequence was empty")
	}
	tokens := 0
	for token, err := range p.Tokens(doc) {
		if err != nil {
			t.Fatal(err)
		}
		_ = token.Finalize()
		tokens++
		break
	}
	if tokens != 1 {
		t.Fatalf("early token count = %d", tokens)
	}
	repeatedTokens := 0
	for _, err := range p.Tokens(doc) {
		if err != nil {
			t.Fatal(err)
		}
		repeatedTokens++
	}
	if repeatedTokens == 0 {
		t.Fatal("repeat token sequence was empty")
	}
}

func TestPublicFormXObjectSequencesAndFlattening(t *testing.T) {
	pdf := []byte("%PDF-1.4\n" +
		"1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n" +
		"2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj\n" +
		"3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] " +
		"/Resources << /XObject << /Fm1 5 0 R >> >> /Contents 6 0 R >> endobj\n" +
		"5 0 obj << /Type /XObject /Subtype /Form /BBox [0 0 100 100] " +
		"/Resources << /Font << /F1 7 0 R >> >> >>\n" +
		"stream\nBT /F1 12 Tf 10 10 Td (Form text) Tj ET\nendstream\nendobj\n" +
		"6 0 obj << >> stream\nq /Fm1 Do Q\nendstream\nendobj\n" +
		"7 0 obj << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> endobj\n" +
		"trailer << /Root 1 0 R >>\n%%EOF\n")
	doc, err := document.OpenBytes(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}

	first := 0
	for xobject, err := range p.XObjects(doc) {
		if err != nil {
			t.Fatal(err)
		}
		if xobject.Name() != "Fm1" || xobject.Ref().Object != 5 {
			t.Fatalf("form xobject = %#v", xobject)
		}
		first++
		break
	}
	if first != 1 {
		t.Fatalf("early form xobject count = %d", first)
	}

	repeated := 0
	for xobject, err := range p.XObjects(doc) {
		if err != nil {
			t.Fatal(err)
		}
		formTexts := 0
		for text, err := range xobject.Texts(doc) {
			if err != nil {
				t.Fatal(err)
			}
			if text.Text() != "Form text" {
				t.Fatalf("form text = %q", text.Text())
			}
			formTexts++
		}
		if formTexts != 1 {
			t.Fatalf("form text count = %d", formTexts)
		}
		repeated++
	}
	if repeated != 1 {
		t.Fatalf("repeat form xobject count = %d", repeated)
	}

	interpKinds := 0
	for object, err := range p.Interp(doc, page.DefaultContentOptions()) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() == page.ContentXObject {
			interpKinds++
		}
	}
	if interpKinds != 1 {
		t.Fatalf("Interp XObject count = %d", interpKinds)
	}
	flattenedText := 0
	for object, err := range p.Flatten(doc, page.DefaultContentOptions()) {
		if err != nil {
			t.Fatal(err)
		}
		if object.Kind() == page.ContentText {
			flattenedText++
		}
	}
	if flattenedText != 1 {
		t.Fatalf("Flatten text count = %d", flattenedText)
	}
}

func TestPublicAnnotationMetadataIsReadOnly(t *testing.T) {
	annotation := page.Annotation{}
	if annotation.Subtype() != "" || annotation.Page() != (page.Ref{}) || annotation.HasPage() || annotation.InReplyTo() != (page.Ref{}) || annotation.HasInReplyTo() || annotation.Popup() != (page.Ref{}) || annotation.HasPopup() || annotation.Rect() != [4]float64{} || annotation.Contents() != "" || annotation.URI() != "" || annotation.ActionKind() != "" || annotation.Border() != [3]float64{} || annotation.HasBorder() || annotation.Name() != "" || annotation.Modified() != "" || annotation.ParentKey() != 0 || annotation.HasParentKey() || annotation.Flags() != 0 || annotation.HasFlags() {
		t.Fatalf("zero annotation metadata = %#v", annotation)
	}
}

func TestPublicStructureElementMetadataIsReadOnly(t *testing.T) {
	element := page.StructElement{}
	if element.Type() != "" || element.StructureType() != "" || element.Role() != "" || element.RawRole() != "" || element.Title() != "" || element.Language() != "" || element.AlternateDescription() != "" || element.ActualText() != "" || element.AbbreviationExpansion() != "" || element.ClassName() != "" || element.Page() != (page.Ref{}) || element.HasPage() || element.Parent() != (page.Ref{}) || element.HasParent() || element.MCID() != 0 || element.HasMCID() || element.IsMCR() || element.ObjectRef() != (page.Ref{}) || element.HasObject() || element.BBox() != [4]float64{} || element.HasBBox() {
		t.Fatalf("zero structure element metadata = %#v", element)
	}
}

func TestPublicImageMetadataIsReadOnly(t *testing.T) {
	image := page.ImageObject{}
	if image.Name() != "" || image.Inline() || image.Offset() != 0 || image.Page() != (page.Ref{}) || image.HasPage() || image.Width() != 0 || image.Height() != 0 || image.BPC() != 0 || image.ColorSpace() != "" || image.Components() != 0 || image.IndexedComponents() != 0 || image.IndexedHigh() != 0 || image.ImageMask() || image.HasMask() || image.HasSoftMask() || image.BBox() != [4]float64{} || image.ParentKey() != 0 || image.HasParentKey() || image.MarkedTag() != "" || image.ActualText() != "" || image.MCID() != 0 || image.HasMCID() {
		t.Fatalf("zero image metadata = %#v", image)
	}
	if image.GState().CTM() != (page.Matrix{}) {
		t.Fatalf("zero image graphics state = %#v", image.GState())
	}
}

func TestPublicPathMetadataIsReadOnly(t *testing.T) {
	path := page.PathObject{}
	if path.Page() != (page.Ref{}) || path.HasPage() || path.Stroke() || path.Fill() || path.EvenOdd() || path.Clip() || path.ClipEvenOdd() || path.BBox() != [4]float64{} || path.MarkedTag() != "" || path.ActualText() != "" || path.MCID() != 0 || path.HasMCID() {
		t.Fatalf("zero path metadata = %#v", path)
	}
	if path.GState().CTM() != (page.Matrix{}) {
		t.Fatalf("zero path graphics state = %#v", path.GState())
	}
}

func assertSeq2[T any](seq func(func(T, error) bool)) {}
