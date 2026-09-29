package contentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
)

func TestExtGStateValuePublicAPI(t *testing.T) {
	value := contentdata.NewExtGState(contentdata.ExtGStateSpec{})
	_ = value.Name
	_ = value.Page
	_ = value.HasPage
	_ = value.GState
	_ = value.DictCopy
	_ = value.Finalize
	_ = testing.Short
}

func TestResourceSelectionValuePublicAPI(t *testing.T) {
	pattern := contentdata.NewPattern(contentdata.PatternSpec{})
	shading := contentdata.NewShading(contentdata.ShadingSpec{})
	_ = pattern.Name
	_ = pattern.Page
	_ = pattern.Stroke
	_ = pattern.GState
	_ = pattern.DictCopy
	_ = pattern.Finalize
	_ = shading.Name
	_ = shading.Page
	_ = shading.GState
	_ = shading.DictCopy
	_ = shading.Finalize
}

func TestColorSpaceSelectionValuePublicAPI(t *testing.T) {
	value := contentdata.NewColorSpaceSelection(contentdata.ColorSpaceSelectionSpec{})
	_ = value.Name
	_ = value.Page
	_ = value.HasPage
	_ = value.Stroke
	_ = value.SpecCopy
	_ = value.Finalize
}

func TestPublicPathAPI(t *testing.T) {
	value := contentdata.NewPath(contentdata.PathSpec{})
	_ = value.Page
	_ = value.HasPage
	_ = value.RawSegmentsCopy
	_ = value.SegmentsCopy
	_ = value.Stroke
	_ = value.Fill
	_ = value.EvenOdd
	_ = value.Clip
	_ = value.ClipEvenOdd
	_ = value.BBox
	_ = value.GState
	_ = value.MarkedTag
	_ = value.MarkedPropertiesCopy
	_ = value.MarkedStackCopy
	_ = value.ActualText
	_ = value.MCID
	_ = value.HasMCID
	_ = value.Finalize
}

func TestPublicTagAPI(t *testing.T) {
	value := contentdata.NewTag(contentdata.TagSpec{})
	_ = value.Name
	_ = value.Page
	_ = value.HasPage
	_ = value.PropertiesCopy
	_ = value.ActualText
	_ = value.MCID
	_ = value.HasMCID
	_ = value.MarkedTag
	_ = value.MarkedPropertiesCopy
	_ = value.MarkedStackCopy
	_ = value.MarkedStackBorrowed
	_ = value.GState
	_ = value.Finalize
}

func TestPublicGlyphAPI(t *testing.T) {
	value := contentdata.NewGlyph(contentdata.GlyphSpec{})
	_ = value.Text
	_ = value.Chars
	_ = value.Page
	_ = value.HasPage
	_ = value.MCID
	_ = value.HasMCID
	_ = value.CodeCopy
	_ = value.CID
	_ = value.GID
	_ = value.FontName
	_ = value.FontSize
	_ = value.Size
	_ = value.FontBase
	_ = value.TextFont
	_ = value.Matrix
	_ = value.Origin
	_ = value.Displacement
	_ = value.BBox
	_ = value.Vertical
	_ = value.Unmapped
	_ = value.Invisible
	_ = value.GState
	_ = value.MarkedStackCopy
	_ = value.Finalize
}

func TestPublicTextAPI(t *testing.T) {
	value := contentdata.NewText(contentdata.TextSpec{})
	_ = value.Text
	_ = value.Page
	_ = value.HasPage
	_ = value.Chars
	_ = value.FontName
	_ = value.FontSize
	_ = value.Size
	_ = value.FontBase
	_ = value.TextFont
	_ = value.Matrix
	_ = value.TextMatrix
	_ = value.LineMatrix
	_ = value.ScalingMatrix
	_ = value.Origin
	_ = value.Displacement
	_ = value.Rotation
	_ = value.BBox
	_ = value.VisualBBoxCopy
	_ = value.LineWidth
	_ = value.Invisible
	_ = value.Vertical
	_ = value.Unmapped
	_ = value.GState
	_ = value.ArgsBorrowed
	_ = value.ArgsCopy
	_ = value.ArgsSeq
	_ = value.StrokeColorCopy
	_ = value.NonStrokeColorCopy
	_ = value.MarkedTag
	_ = value.ActualText
	_ = value.MCID
	_ = value.HasMCID
	_ = value.MarkedPropertiesCopy
	_ = value.MarkedStackCopy
	_ = value.Finalize
}

func TestPublicXObjectAPI(t *testing.T) {
	value := contentdata.NewXObject(contentdata.XObjectSpec{})
	_ = value.Name
	_ = value.Ref
	_ = value.Page
	_ = value.HasPage
	_ = value.StreamBorrowed
	_ = value.StreamCopy
	_ = value.Matrix
	_ = value.BBox
	_ = value.GroupBorrowed
	_ = value.GroupCopy
	_ = value.ParentKey
	_ = value.HasParentKey
	_ = value.ResourcesBorrowed
	_ = value.ResourcesCopy
	_ = value.DeclaredResourcesBorrowed
	_ = value.DeclaredResourcesCopy
	_ = value.HasDeclaredResources
	_ = value.ResourceContextBorrowed
	_ = value.ResourceContextCopy
	_ = value.Path
	_ = value.MarkedTag
	_ = value.MarkedPropertiesBorrowed
	_ = value.MarkedPropertiesCopy
	_ = value.MarkedStackBorrowed
	_ = value.MarkedStackCopy
	_ = value.MCID
	_ = value.HasMCID
	_ = value.GState
	_ = value.GStateBorrowed
	_ = value.Finalize
	_ = testing.Short
}
