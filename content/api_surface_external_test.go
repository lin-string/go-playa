package content_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/lin-string/go-playa/content"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/page"
)

func acceptMarkedContentContext(_ content.MarkedContentContext)  {}
func acceptMarkedContentData(_ contentdata.MarkedContentContext) {}
func acceptPropertiesObject(_ content.PropertiesObject)          {}
func acceptPropertiesData(_ contentdata.PropertiesObject)        {}

func TestMarkedContentContextUsesContentdataValue(t *testing.T) {
	value := contentdata.NewMarkedContentContext("P", nil, "", 0, false)
	acceptMarkedContentContext(value)
	acceptMarkedContentData(content.MarkedContentContext{})
	if value.Tag() != "P" {
		t.Fatalf("marked-content tag = %q", value.Tag())
	}
}

func TestPropertiesObjectUsesContentdataValue(t *testing.T) {
	var value contentdata.PropertiesObject
	acceptPropertiesObject(value)
	acceptPropertiesData(content.PropertiesObject{})
}

func TestPublicContentSequencesAndProjection(t *testing.T) {
	if content.ErrPageNotFound == nil {
		t.Fatal("ErrPageNotFound is nil")
	}
	if content.ErrNilDocument == nil {
		t.Fatal("ErrNilDocument is nil")
	}
	var _ *content.ParseError
	layout := content.DefaultLayoutOptions()
	if layout.WordMargin != 0.1 || layout.LineMargin != 0.5 || layout.LineOverlap != 0.5 || layout.CharMargin != 2 || layout.BoxesFlow == nil || *layout.BoxesFlow != 0.5 {
		t.Fatalf("default layout options = %#v", layout)
	}
	options := content.DefaultContentOptions()
	if options.Filter != content.FilterAll || options.RestrictOps != nil {
		t.Fatalf("default content options = %#v", options)
	}
	var _ content.TextExtractionOptions
	if content.DefaultTextExtractionOptions().BBox != nil {
		t.Fatal("default content text extraction options has a BBox")
	}
	var _ content.Object
	var _ content.Ref
	var _ content.Dict
	var _ content.Array
	var _ content.Stream
	var _ content.Null
	var _ content.Bool
	var _ content.Number
	var _ content.Name
	var _ content.String
	var _ content.Keyword
	var _ content.DecodedImage
	var _ content.ImageColorSpace
	var _ content.FontResource
	var _ content.FontMetadata
	var _ content.PageStructureEntry
	var _ content.PageStructure
	var _ content.ContentSection
	var _ content.ContentSequence
	var _ content.DashPattern
	var _ content.Page
	var _ content.LayoutItem
	var _ content.LayoutItemKind
	_ = content.LayoutTextBox
	_ = content.LayoutImage
	_ = content.LayoutPath
	_ = content.LayoutXObject
	var _ content.StructElement
	_ = (content.XObjectObject{}).FontsSeq
	_ = (content.XObjectObject{}).Name
	_ = (content.XObjectObject{}).Ref
	_ = (content.XObjectObject{}).Page
	_ = (content.XObjectObject{}).HasPage
	_ = (content.XObjectObject{}).StreamCopy
	_ = (content.XObjectObject{}).Matrix
	_ = (content.XObjectObject{}).BBox
	_ = (content.XObjectObject{}).ParentKey
	_ = (content.XObjectObject{}).HasParentKey
	_ = (content.XObjectObject{}).Path
	_ = (content.XObjectObject{}).MarkedTag
	_ = (content.XObjectObject{}).GState
	_ = (content.XObjectObject{}).StructureSeq
	_ = (content.XObjectObject{}).MarkedContentSequence
	_ = (content.XObjectObject{}).Len
	_ = (content.ContentSection{}).MCID
	_ = (content.ContentSection{}).HasMCID
	_ = (content.ContentSection{}).Len
	_ = (content.ContentSection{}).ObjectsSeq
	_ = (content.ContentSection{}).ObjectsCopy
	_ = (content.ContentSection{}).ObjectsCopyWithError
	_ = (content.ContentSection{}).TextsSeq
	_ = (content.ContentSection{}).TextsCopy
	_ = (content.ContentSection{}).Finalize
	_ = (content.ContentSection{}).FinalizeWithError
	_ = (content.ContentSequence{}).Len
	_ = (content.ContentSequence{}).At
	_ = (content.ContentSequence{}).PageOrderSeq
	_ = (content.ContentSequence{}).PageOrderCopy
	_ = (content.ContentSequence{}).Finalize
	_ = (content.ContentSequence{}).FinalizeWithError
	_ = content.DefaultGraphicsState
	var _ content.Matrix
	_ = content.ApplyGraphicsState
	_ = content.ApplyExternalGraphicsState
	_ = content.ApplyResourceColorSpace
	_ = content.InterpretTextWithFonts
	_ = content.InterpretTextWithFontsSeq
	_ = content.InterpretTextSeq
	_ = content.InterpretPathsSeq
	_ = (content.ContentOp{}).OperandsCopy
	_ = (content.ContentOp{}).OperandsSeq
	_ = (content.ContentOp{}).Finalize
	_ = (content.ContentOp{}).Operator
	_ = (content.ContentOp{}).Offset
	_ = (content.ContentObject{}).Len
	assertSeq2[content.GlyphObject]((content.TextObject{}).GlyphsSeq())
	_ = (content.GlyphObject{}).Codes
	_ = (content.GlyphObject{}).Text
	_ = (content.GlyphObject{}).Chars
	_ = (content.GlyphObject{}).Page
	_ = (content.GlyphObject{}).HasPage
	_ = (content.GlyphObject{}).MCID
	_ = (content.GlyphObject{}).HasMCID
	_ = (content.GlyphObject{}).CID
	_ = (content.GlyphObject{}).GID
	_ = (content.GlyphObject{}).FontName
	_ = (content.GlyphObject{}).FontSize
	_ = (content.GlyphObject{}).Size
	_ = (content.GlyphObject{}).FontBase
	_ = (content.GlyphObject{}).TextFont
	_ = (content.GlyphObject{}).Matrix
	_ = (content.GlyphObject{}).Origin
	_ = (content.GlyphObject{}).Displacement
	_ = (content.GlyphObject{}).BBox
	_ = (content.GlyphObject{}).Vertical
	_ = (content.GlyphObject{}).Unmapped
	_ = (content.GlyphObject{}).Invisible
	_ = (content.GlyphObject{}).GState
	_ = (content.GlyphObject{}).MarkedStackCopy
	_ = (content.GlyphObject{}).ContentSeq
	_ = (content.GlyphObject{}).Len
	_ = (content.TextObject{}).Text
	_ = (content.TextObject{}).Len
	_ = (content.TextObject{}).Page
	_ = (content.TextObject{}).HasPage
	_ = (content.TextObject{}).Chars
	_ = (content.TextObject{}).FontName
	_ = (content.TextObject{}).FontSize
	_ = (content.TextObject{}).Size
	_ = (content.TextObject{}).FontBase
	_ = (content.TextObject{}).TextFont
	_ = (content.TextObject{}).Matrix
	_ = (content.TextObject{}).TextMatrix
	_ = (content.TextObject{}).LineMatrix
	_ = (content.TextObject{}).ScalingMatrix
	_ = (content.TextObject{}).Origin
	_ = (content.TextObject{}).Displacement
	_ = (content.TextObject{}).Rotation
	_ = (content.TextObject{}).BBox
	_ = (content.TextObject{}).LineWidth
	_ = (content.TextObject{}).Invisible
	_ = (content.TextObject{}).Vertical
	_ = (content.TextObject{}).Unmapped
	_ = (content.TextObject{}).GState
	_ = (content.TextObject{}).MarkedTag
	_ = (content.TextObject{}).ActualText
	_ = (content.TextObject{}).MCID
	_ = (content.TextObject{}).HasMCID
	_ = (content.TextWord{}).Text
	_ = (content.TextWord{}).BBox
	_ = (content.TextLine{}).Text
	_ = (content.TextLine{}).BBox
	_ = (content.TextLine{}).Vertical
	_ = (content.TextParagraph{}).Text
	_ = (content.TextParagraph{}).BBox
	_ = (content.TextParagraph{}).Vertical
	_ = (content.TextBox{}).Text
	_ = (content.TextBox{}).BBox
	_ = (content.TextBox{}).Vertical
	_ = (content.TextBox{}).Index
	_ = (content.TextGroup{}).Text
	_ = (content.TextGroup{}).BBox
	_ = (content.TextGroup{}).Vertical
	_ = (content.TextGroup{}).ChildrenSeq
	_ = (content.TextGroup{}).ChildrenCopy
	_ = (content.TextGroup{}).ChildrenCopyWithError
	_ = (content.TextGroupChild{}).IsBox
	_ = (content.TextGroupChild{}).IsGroup
	_ = (content.TextGroupChild{}).BoxCopy
	_ = (content.TextGroupChild{}).GroupCopy
	_ = (content.TextObject{}).Finalize()
	_ = (content.TextObject{}).FinalizeWithError
	_ = (content.TextObject{}).FontCopyWithError
	_ = (content.TextObject{}).ArgsCopy
	_ = (content.TextObject{}).ArgsSeq
	_ = (content.TextObject{}).GlyphsCopy
	_ = (content.TextObject{}).GlyphsCopyWithError
	_ = (content.TextObject{}).StrokeColorCopy
	_ = (content.TextObject{}).NonStrokeColorCopy
	_ = (content.TextObject{}).VisualBBoxCopy
	_ = (content.TextObject{}).MarkedPropertiesCopy
	_ = (content.TextObject{}).MarkedStackCopy
	_ = (content.GraphicsState{}).Finalize()
	_ = (content.GraphicsState{}).DashCopy
	_ = (content.GraphicsState{}).BlendModesCopy
	_ = (content.GraphicsState{}).SoftMaskCopy
	_ = (content.GraphicsState{}).HalftoneCopy
	_ = (content.Color{}).Components
	_ = (content.Color{}).ValuesCopy
	_ = (content.Color{}).Finalize
	_ = (content.ImageColorSpace{}).Name
	_ = (content.ImageColorSpace{}).Components
	_ = (content.ImageColorSpace{}).High
	_ = (content.ImageColorSpace{}).ProfileN
	_ = (content.ImageColorSpace{}).SpecCopy
	_ = (content.ShadingObject{}).Name
	_ = (content.ShadingObject{}).Page
	_ = (content.ShadingObject{}).HasPage
	_ = (content.ShadingObject{}).GState
	_ = (content.PatternObject{}).Name
	_ = (content.PatternObject{}).Page
	_ = (content.PatternObject{}).HasPage
	_ = (content.PatternObject{}).Stroke
	_ = (content.PatternObject{}).GState
	_ = (content.ExtGStateObject{}).Name
	_ = (content.ExtGStateObject{}).Page
	_ = (content.ExtGStateObject{}).HasPage
	_ = (content.ExtGStateObject{}).GState
	_ = (content.PropertiesObject{}).Name
	_ = (content.PropertiesObject{}).Operator
	_ = (content.PropertiesObject{}).Page
	_ = (content.PropertiesObject{}).HasPage
	_ = (content.GraphicsState{}).CTM
	_ = (content.GraphicsState{}).FontName
	_ = (content.GraphicsState{}).FontSize
	_ = (content.GraphicsState{}).LineWidth
	_ = (content.GraphicsState{}).LineCap
	_ = (content.GraphicsState{}).LineJoin
	_ = (content.GraphicsState{}).MiterLimit
	_ = (content.GraphicsState{}).Dash
	_ = (content.GraphicsState{}).DashPhase
	_ = (content.GraphicsState{}).StrokeColor
	_ = (content.GraphicsState{}).FillColor
	_ = (content.GraphicsState{}).RenderMode
	_ = (content.GraphicsState{}).CharacterRise
	_ = (content.GraphicsState{}).CharacterSpacing
	_ = (content.GraphicsState{}).WordSpacing
	_ = (content.GraphicsState{}).HorizontalScale
	_ = (content.GraphicsState{}).Leading
	_ = (content.GraphicsState{}).Alpha
	_ = (content.GraphicsState{}).StrokeAlpha
	_ = (content.GraphicsState{}).FillAlpha
	_ = (content.GraphicsState{}).BlendMode
	_ = (content.GraphicsState{}).Intent
	_ = (content.GraphicsState{}).Flatness
	_ = (content.GraphicsState{}).StrokeAdjustment
	_ = (content.GraphicsState{}).AlphaSource
	_ = (content.GraphicsState{}).Knockout
	_ = (content.GraphicsState{}).Overprint
	_ = (content.GraphicsState{}).StrokeOverprint
	_ = (content.GraphicsState{}).OverprintMode
	_ = (content.GraphicsState{}).HasHalftone
	_ = (content.GraphicsState{}).HasSoftMask
	_ = (content.GraphicsState{}).BlackPointComp
	_ = (content.GraphicsState{}).ClipDepth
	_ = (content.GraphicsState{}).ClipEvenOdd
	_ = (content.MarkedContent{}).Tag
	_ = (content.MarkedContent{}).Page
	_ = (content.MarkedContent{}).HasPage
	_ = (content.MarkedContent{}).MCID
	_ = (content.MarkedContent{}).HasMCID
	_ = (content.MarkedContent{}).ActualText
	_ = (content.MarkedContentContext{}).Tag
	_ = (content.MarkedContentContext{}).ActualText
	_ = (content.MarkedContentContext{}).MCID
	_ = (content.MarkedContentContext{}).HasMCID
	_ = (content.GlyphObject{}).Finalize()
	_ = (content.GlyphObject{}).FinalizeWithError
	_ = (content.GlyphObject{}).FontCopyWithError
	_ = (content.PathObject{}).Finalize()
	_ = (content.PathObject{}).Len
	_ = (content.ImageObject{}).Finalize()
	_ = (content.ImageObject{}).Len
	_ = (content.ImageObject{}).FinalizeWithError
	_ = (content.ImageObject{}).IndexedLookupWithError
	_ = (content.ImageObject{}).ColorSpaceInfoCopyWithError
	_ = (content.ImageObject{}).DecodedBuffer
	_ = (content.ImageObject{}).DecodedStreamBuffer
	_ = (content.ImageObject{}).DecodedStreamBufferWithError
	_ = (content.ImageObject{}).DecodeCopy
	_ = (content.ImageObject{}).IndexedLookupCopy
	_ = (content.ImageObject{}).FiltersCopy
	_ = (content.ImageObject{}).FilterParamsCopy
	_ = (content.ImageObject{}).Get
	_ = (content.ImageObject{}).Has
	_ = (content.ImageObject{}).MarkedPropertiesCopy
	_ = (content.ImageObject{}).MarkedStackCopy
	_ = (content.ImageObject{}).MaskCopy
	_ = (content.ImageObject{}).SoftMaskCopy
	_ = (content.ImageObject{}).ColorSpaceInfoCopy
	_ = (content.XObjectObject{}).Finalize()
	_ = (content.TagObject{}).Finalize()
	_ = (content.TagObject{}).Name
	_ = (content.TagObject{}).Page
	_ = (content.TagObject{}).HasPage
	_ = (content.TagObject{}).ActualText
	_ = (content.TagObject{}).MCID
	_ = (content.TagObject{}).HasMCID
	_ = (content.TagObject{}).MarkedTag
	_ = (content.TagObject{}).GState
	_ = (content.ColorSpaceObject{}).Name
	_ = (content.ColorSpaceObject{}).Page
	_ = (content.ColorSpaceObject{}).HasPage
	_ = (content.ColorSpaceObject{}).Stroke
	_ = (content.ColorSpaceObject{}).Info
	_ = (content.ContentObject{}).Finalize()
	_ = (content.ContentObject{}).FinalizeWithError
	_ = (content.ContentObject{}).Kind
	_ = (content.ColorSpaceObject{}).FinalizeWithError
	_ = (content.ContentObject{}).TextCopyWithError
	_ = (content.LayoutResult{}).LinesCopy
	_ = (content.LayoutResult{}).LinesCopyWithError
	_ = (content.LayoutResult{}).LinesSeq
	_ = (content.LayoutResult{}).ParagraphsCopy
	_ = (content.LayoutResult{}).ParagraphsCopyWithError
	_ = (content.LayoutResult{}).ParagraphsSeq
	_ = (content.LayoutResult{}).TextBoxesCopy
	_ = (content.LayoutResult{}).TextBoxesCopyWithError
	_ = (content.LayoutResult{}).TextBoxesSeq
	_ = (content.LayoutResult{}).TextGroupsCopy
	_ = (content.LayoutResult{}).TextGroupsCopyWithError
	_ = (content.LayoutResult{}).TextGroupsSeq
	_ = (content.LayoutResult{}).ItemsCopy
	_ = (content.LayoutResult{}).ItemsCopyWithError
	_ = (content.LayoutResult{}).ItemsSeq
	_ = (content.LayoutItem{}).Kind
	_ = (content.LayoutItem{}).TextBoxCopy
	_ = (content.LayoutItem{}).TextBoxCopyWithError
	_ = (content.LayoutItem{}).ContentCopy
	_ = (content.LayoutItem{}).ContentCopyWithError
	_ = (content.LayoutItem{}).Finalize
	_ = (content.LayoutItem{}).FinalizeWithError
	_ = (content.LayoutResult{}).Finalize
	_ = (content.LayoutResult{}).FinalizeWithError
	_ = (content.TextParagraph{}).LinesCopy
	_ = (content.TextParagraph{}).LinesCopyWithError
	_ = (content.TextParagraph{}).LinesSeq
	_ = (content.TextParagraph{}).WritingMode
	_ = (content.TextParagraph{}).Finalize
	_ = (content.TextParagraph{}).FinalizeWithError
	_ = (content.TextBox{}).LinesCopy
	_ = (content.TextBox{}).LinesCopyWithError
	_ = (content.TextBox{}).LinesSeq
	_ = (content.TextBox{}).WritingMode
	_ = (content.TextBox{}).Finalize
	_ = (content.TextBox{}).FinalizeWithError
	_ = (content.TextGroup{}).WritingMode
	_ = (content.TextGroup{}).BoxesCopy
	_ = (content.TextGroup{}).BoxesCopyWithError
	_ = (content.TextGroup{}).BoxesSeq
	_ = (content.TextGroup{}).Finalize
	_ = (content.TextGroup{}).FinalizeWithError
	_ = (content.TextLine{}).GlyphsCopy
	_ = (content.TextLine{}).GlyphsCopyWithError
	_ = (content.TextLine{}).GlyphsSeq
	_ = (content.TextLine{}).WordsCopy
	_ = (content.TextLine{}).WordsCopyWithError
	_ = (content.TextLine{}).WordsSeq
	_ = (content.TextLine{}).Finalize
	_ = (content.TextLine{}).FinalizeWithError
	_ = (content.TextWord{}).GlyphsCopy
	_ = (content.TextWord{}).GlyphsCopyWithError
	_ = (content.TextWord{}).GlyphsSeq
	_ = (content.TextWord{}).Finalize
	_ = (content.TextWord{}).FinalizeWithError
	assertSeq2[content.PathObject]((content.GlyphObject{}).PathsSeq())
	assertSeq2[content.PathSegment]((content.PathObject{}).SegmentsSeq())
	_ = (content.PathSegment{}).PointsCopy
	_ = (content.PathSegment{}).Operator
	_ = (content.PathSegment{}).Finalize
	_ = (content.PathObject{}).RawSegmentsCopy
	_ = (content.PathObject{}).SegmentsCopy
	_ = (content.PathObject{}).MarkedPropertiesCopy
	_ = (content.PathObject{}).MarkedStackCopy
	_ = (content.TagObject{}).PageObject
	_ = (content.TagObject{}).MarkedPropertiesCopy
	_ = (content.TagObject{}).PropertiesCopy
	_ = (content.TagObject{}).MarkedStackCopy
	_ = (content.TagObject{}).Len
	_ = (content.GlyphObject{}).PageObject
	_ = (content.TextObject{}).Parent
	_ = (content.GlyphObject{}).Parent
	_ = (content.PathObject{}).Parent
	_ = (content.TagObject{}).Parent
	_ = (content.MarkedContent{}).Parent
	_ = (content.MarkedContent{}).PropertiesCopy
	_ = (content.MarkedContentContext{}).PropertiesCopy
	_ = (content.MarkedContentContext{}).Finalize
	_ = (content.MarkedContent{}).OpsCopy
	_ = (content.MarkedContent{}).ChildrenCopy
	_ = (content.MarkedContentIndex{}).Finalize
	_ = (content.ContentObject{}).PageObject
	_ = (content.ContentObject{}).Parent
	_ = (content.ContentObject{}).ObjectType
	_ = (content.ContentObject{}).Len
	_ = (content.ContentObject{}).BBoxValue
	_ = (content.ContentObject{}).GraphicsStateCopy
	_ = (content.ContentObject{}).MatrixValue
	_ = (content.ContentObject{}).MarkedStackCopy
	_ = (content.ContentObject{}).MarkedContext
	_ = (content.ContentObject{}).MCIDValue
	assertSeq2[content.ContentOp]((content.XObjectObject{}).Contents(nil))
	assertSeq2[page.Token]((content.XObjectObject{}).Tokens(nil))
	_ = (content.XObjectObject{}).MarkedPropertiesCopy
	_ = (content.XObjectObject{}).MarkedStackCopy
	_ = (content.XObjectObject{}).GroupCopy
	_ = (content.XObjectObject{}).ResourcesCopy
	_ = (content.XObjectObject{}).Buffer
	_ = (content.XObjectObject{}).DecodedBuffer
	_ = (content.XObjectObject{}).DecodedBufferWithError
	_ = (content.XObjectObject{}).DecodedBufferWithDocument
	_ = (content.XObjectObject{}).DecodedBufferWithDocumentWithError

	text := content.TextObject{}
	encoded, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"text"`, `"chars"`, `"origin"`, `"glyphs"`} {
		if !bytes.Contains(encoded, []byte(key)) {
			t.Fatalf("content JSON missing %s: %s", key, encoded)
		}
	}
}

func TestPublicXObjectMetadataIsReadOnly(t *testing.T) {
	x := content.XObjectObject{}
	if x.Name() != "" || x.Ref() != (content.Ref{}) || x.Page() != (content.Ref{}) {
		t.Fatalf("zero XObject references = name %q ref %#v page %#v", x.Name(), x.Ref(), x.Page())
	}
	if x.HasPage() || x.ParentKey() != 0 || x.HasParentKey() || x.Path() != "" || x.MarkedTag() != "" {
		t.Fatalf("zero XObject metadata = page=%v parent=%d/%v path=%q tag=%q", x.HasPage(), x.ParentKey(), x.HasParentKey(), x.Path(), x.MarkedTag())
	}
	if x.Matrix() != (content.Matrix{}) || x.BBox() != [4]float64{} {
		t.Fatalf("zero XObject geometry = matrix=%#v bbox=%#v", x.Matrix(), x.BBox())
	}
	if stream := x.StreamCopy(); stream.Buffer() != nil {
		t.Fatalf("zero XObject stream = %#v", stream)
	}
}

func TestPublicContentOpMetadataIsReadOnly(t *testing.T) {
	ops, err := content.Parse([]byte("q 1 0 0 1 10 20 cm"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("content operations = %#v", ops)
	}
	if ops[0].Operator() != "q" || ops[0].Offset() != 0 {
		t.Fatalf("content operation metadata = operator:%q offset:%d", ops[0].Operator(), ops[0].Offset())
	}
}

func TestPublicTextAndGlyphMetadataIsReadOnly(t *testing.T) {
	text := content.TextObject{}
	if text.Text() != "" || text.Chars() != "" || text.Page() != (content.Ref{}) || text.HasPage() {
		t.Fatalf("zero text metadata = text:%q chars:%q page:%v hasPage:%v", text.Text(), text.Chars(), text.Page(), text.HasPage())
	}
	if text.GlyphsCopy() != nil || text.ArgsCopy() != nil {
		t.Fatal("zero text unexpectedly has backing collections")
	}
	glyph := content.GlyphObject{}
	if glyph.Text() != "" || glyph.Chars() != "" || glyph.CID() != 0 || glyph.GID() != 0 {
		t.Fatalf("zero glyph metadata = %#v", glyph)
	}
	if glyph.Codes() != nil || glyph.PathsSeq() == nil {
		t.Fatal("zero glyph ownership helpers are not stable")
	}
}

func TestPublicLayoutMetadataIsReadOnly(t *testing.T) {
	word := content.TextWord{}
	line := content.TextLine{}
	paragraph := content.TextParagraph{}
	box := content.TextBox{}
	group := content.TextGroup{}
	if word.Text() != "" || word.BBox() != [4]float64{} || line.Text() != "" || line.BBox() != [4]float64{} || line.Vertical() {
		t.Fatal("zero layout word/line metadata is not stable")
	}
	if paragraph.Text() != "" || paragraph.BBox() != [4]float64{} || paragraph.Vertical() || box.Text() != "" || box.BBox() != [4]float64{} || box.Vertical() || box.Index() != 0 || group.Text() != "" || group.BBox() != [4]float64{} || group.Vertical() {
		t.Fatal("zero layout aggregate metadata is not stable")
	}
}

func TestPublicPathSegmentMetadataIsReadOnly(t *testing.T) {
	ops, err := content.Parse([]byte("0 0 m 1 1 l S"))
	if err != nil {
		t.Fatal(err)
	}
	paths := content.InterpretPaths(ops)
	if len(paths) != 1 {
		t.Fatalf("path objects = %#v", paths)
	}
	segments := paths[0].RawSegmentsCopy()
	if len(segments) != 2 {
		t.Fatalf("raw path segments = %#v", segments)
	}
	if segments[0].Operator() != "m" || segments[1].Operator() != "l" {
		t.Fatalf("path segment operators = %q, %q", segments[0].Operator(), segments[1].Operator())
	}
}

func TestPublicImageColorSpaceMetadataIsReadOnly(t *testing.T) {
	info := content.ImageColorSpace{}
	if info.Name() != "" || info.Components() != 0 || info.High() != 0 || info.ProfileN() != 0 {
		t.Fatalf("zero image color-space metadata = %#v", info)
	}
}

func TestPublicResourceMetadataIsReadOnly(t *testing.T) {
	shading := content.ShadingObject{}
	if shading.Name() != "" || shading.Page() != (content.Ref{}) || shading.HasPage() || shading.GState().DashCopy() != nil {
		t.Fatalf("zero shading metadata = %#v", shading)
	}
	pattern := content.PatternObject{}
	if pattern.Name() != "" || pattern.Page() != (content.Ref{}) || pattern.HasPage() || pattern.Stroke() || pattern.GState().DashCopy() != nil {
		t.Fatalf("zero pattern metadata = %#v", pattern)
	}
	extgstate := content.ExtGStateObject{}
	if extgstate.Name() != "" || extgstate.Page() != (content.Ref{}) || extgstate.HasPage() || extgstate.GState().DashCopy() != nil {
		t.Fatalf("zero ExtGState metadata = %#v", extgstate)
	}
	properties := content.PropertiesObject{}
	if properties.Name() != "" || properties.Operator() != "" || properties.Page() != (content.Ref{}) || properties.HasPage() {
		t.Fatalf("zero properties metadata = %#v", properties)
	}
}

func TestPublicGraphicsStateMetadataIsReadOnly(t *testing.T) {
	state := content.DefaultGraphicsState()
	if state.LineWidth() != 1 || state.MiterLimit() != 10 || state.HorizontalScale() != 1 || state.Alpha() != 1 || state.StrokeAlpha() != 1 || state.FillAlpha() != 1 {
		t.Fatalf("default graphics state metrics = %#v", state)
	}
	if state.CTM() != (content.Matrix{1, 0, 0, 1, 0, 0}) || state.Intent() != "RelativeColorimetric" || !state.Knockout() {
		t.Fatalf("default graphics state identity = %#v", state)
	}
	if state.StrokeColor().ValuesCopy() == nil || state.FillColor().ValuesCopy() == nil {
		t.Fatal("graphics state color accessors returned invalid colors")
	}
}

func TestPublicGraphicsStateExposesDashPattern(t *testing.T) {
	pattern := contentdata.NewDashPattern([]float64{1, 2}, 3)
	if pattern.Len() != 2 || pattern.Phase() != 3 {
		t.Fatalf("dash pattern metadata = len %d phase %v", pattern.Len(), pattern.Phase())
	}
	if value, ok := pattern.At(1); !ok || value != 2 {
		t.Fatalf("dash pattern At(1) = %v, %v", value, ok)
	}
	values := pattern.ValuesCopy()
	values[0] = 99
	if value, ok := pattern.At(0); !ok || value != 1 {
		t.Fatalf("dash pattern leaked mutable values = %v, %v", value, ok)
	}
	if state := content.DefaultGraphicsState(); state.Dash().Phase() != 0 || state.Dash().Len() != 0 {
		t.Fatalf("default dash pattern = %#v", state.Dash())
	}
}

func TestPublicGraphicsStateOperationsUseOwnedDomainValue(t *testing.T) {
	state := content.DefaultGraphicsState()
	operations, err := content.Parse([]byte("2 w 0.25 CA 0.5 ca /Multiply BM"))
	if err != nil {
		t.Fatalf("Parse() = %v", err)
	}
	for _, operation := range operations {
		content.ApplyGraphicsState(&state, operation)
	}
	if state.LineWidth() != 2 || state.StrokeAlpha() != 0.25 || state.FillAlpha() != 0.5 || state.BlendMode() != "Multiply" {
		t.Fatalf("public graphics state operations = %#v", state)
	}
	dash := state.DashCopy()
	dash = append(dash, 9)
	if len(state.DashCopy()) != len(dash)-1 {
		t.Fatal("public graphics state exposed mutable slice storage")
	}
}

func TestPublicMarkedContentMetadataIsReadOnly(t *testing.T) {
	marked := content.MarkedContent{}
	if marked.Tag() != "" || marked.Page() != (content.Ref{}) || marked.HasPage() || marked.MCID() != 0 || marked.HasMCID() || marked.ActualText() != "" {
		t.Fatalf("zero marked-content metadata = %#v", marked)
	}
	context := content.MarkedContentContext{}
	if context.Tag() != "" || context.ActualText() != "" || context.MCID() != 0 || context.HasMCID() {
		t.Fatalf("zero marked-content context = %#v", context)
	}
}

func TestPublicTagAndColorSpaceMetadataIsReadOnly(t *testing.T) {
	tag := content.TagObject{}
	if tag.Name() != "" || tag.Page() != (content.Ref{}) || tag.HasPage() || tag.ActualText() != "" || tag.MCID() != 0 || tag.HasMCID() || tag.MarkedTag() != "" {
		t.Fatalf("zero tag metadata = %#v", tag)
	}
	colorSpace := content.ColorSpaceObject{}
	if colorSpace.Name() != "" || colorSpace.Page() != (content.Ref{}) || colorSpace.HasPage() || colorSpace.Stroke() || colorSpace.Info().Name() != "" {
		t.Fatalf("zero color-space metadata = %#v", colorSpace)
	}
}

func assertSeq2[T any](seq func(func(T, error) bool)) {}
