// Package content exposes interpreted PDF page-content models.
package content

import (
	"iter"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	core "github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/font"
	"github.com/lin-string/go-playa/fontdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/imagedata"
	"github.com/lin-string/go-playa/layout"
	"github.com/lin-string/go-playa/parser"
	"github.com/lin-string/go-playa/parserconfig"
	"github.com/lin-string/go-playa/pdftypes"
	"github.com/lin-string/go-playa/textconfig"
)

var ErrPageNotFound = core.ErrPageNotFound
var ErrNilDocument = core.ErrNilDocument
var ErrContentMCIDOutOfRange = core.ErrContentMCIDOutOfRange

type (
	ParseError            = parser.ParseError
	ContentOp             = core.ContentOp
	ContentKind           = contentconfig.Kind
	ContentObject         = core.ContentObject
	ContentFilter         = contentconfig.Filter
	ContentOptions        = contentconfig.Options
	XObjectObject         = core.XObjectObject
	GraphicsState         = core.GraphicsState
	DashPattern           = contentdata.DashPattern
	Matrix                = geometry.Matrix
	Color                 = geometry.Color
	TextObject            = core.TextObject
	GlyphObject           = core.GlyphObject
	PathObject            = core.PathObject
	PathSegment           = geometry.PathSegment
	ImageObject           = core.ImageObject
	ShadingObject         = core.ShadingObject
	PatternObject         = core.PatternObject
	ExtGStateObject       = core.ExtGStateObject
	ColorSpaceObject      = core.ColorSpaceObject
	PropertiesObject      = core.PropertiesObject
	MarkedContent         = core.MarkedContent
	MarkedContentIndex    = core.MarkedContentIndex
	MarkedContentContext  = core.MarkedContentContext
	ContentSection        = core.ContentSection
	ContentSequence       = core.ContentSequence
	TagObject             = core.TagObject
	TextExtractionOptions = textconfig.Options
	LayoutOptions         = layout.Options
	LayoutResult          = core.LayoutResult
	LayoutItem            = core.LayoutItem
	LayoutItemKind        = core.LayoutItemKind
	TextLine              = core.TextLine
	TextParagraph         = core.TextParagraph
	TextBox               = core.TextBox
	TextGroup             = core.TextGroup
	TextGroupChild        = core.TextGroupChild
	TokenKind             = parserconfig.TokenKind
	TextWord              = core.TextWord
	Object                = pdftypes.Object
	Ref                   = pdftypes.Ref
	Dict                  = pdftypes.Dict
	Array                 = pdftypes.Array
	Stream                = pdftypes.Stream
	Null                  = pdftypes.Null
	Bool                  = pdftypes.Bool
	Number                = pdftypes.Number
	Name                  = pdftypes.Name
	String                = pdftypes.String
	Keyword               = pdftypes.Keyword
	DecodedImage          = imagedata.DecodedImage
	ImageColorSpace       = core.ImageColorSpace
	FontResource          = core.FontResource
	FontMetadata          = fontdata.Metadata
	PageStructureEntry    = core.PageStructureEntry
	PageStructure         = core.PageStructure
	StructureItem         = core.StructureItem
	Page                  = core.Page
	StructElement         = core.StructElement
)

const (
	LayoutTextBox = core.LayoutTextBox
	LayoutImage   = core.LayoutImage
	LayoutPath    = core.LayoutPath
	LayoutXObject = core.LayoutXObject

	ContentText       = contentconfig.KindText
	ContentPath       = contentconfig.KindPath
	ContentImage      = contentconfig.KindImage
	ContentTag        = contentconfig.KindTag
	ContentXObject    = contentconfig.KindXObject
	ContentExtGState  = contentconfig.KindExtGState
	ContentColorSpace = contentconfig.KindColorSpace
	ContentPattern    = contentconfig.KindPattern
	ContentShading    = contentconfig.KindShading
	ContentProperties = contentconfig.KindProperties

	TokenEOF        = parserconfig.TokenEOF
	TokenNumber     = parserconfig.TokenNumber
	TokenName       = parserconfig.TokenName
	TokenLiteral    = parserconfig.TokenLiteral
	TokenString     = parserconfig.TokenString
	TokenHexString  = parserconfig.TokenHexString
	TokenArrayStart = parserconfig.TokenArrayStart
	TokenArrayEnd   = parserconfig.TokenArrayEnd
	TokenDictStart  = parserconfig.TokenDictStart
	TokenDictEnd    = parserconfig.TokenDictEnd
	TokenKeyword    = parserconfig.TokenKeyword
)

const (
	FilterAll     = core.FilterAll
	FilterText    = core.FilterText
	FilterPath    = core.FilterPath
	FilterImage   = core.FilterImage
	FilterXObject = core.FilterXObject
	FilterTag     = core.FilterTag
)

func Parse(data []byte) ([]ContentOp, error)             { return core.ParseContent(data) }
func InterpretText(operations []ContentOp) []TextObject  { return core.InterpretText(operations) }
func InterpretPaths(operations []ContentOp) []PathObject { return core.InterpretPaths(operations) }
func InterpretTextSeq(operations []ContentOp) iter.Seq[TextObject] {
	return core.InterpretTextSeq(operations)
}
func InterpretPathsSeq(operations []ContentOp) iter.Seq[PathObject] {
	return core.InterpretPathsSeq(operations)
}
func InterpretTextWithFonts(operations []ContentOp, fonts map[string]*font.Font) []TextObject {
	return core.InterpretTextWithFonts(operations, fonts)
}
func InterpretTextWithFontsSeq(operations []ContentOp, fonts map[string]*font.Font) iter.Seq[TextObject] {
	return core.InterpretTextWithFontsSeq(operations, fonts)
}
func ApplyGraphicsState(state *GraphicsState, operation ContentOp) {
	core.ApplyGraphicsState(state, operation)
}

func DefaultGraphicsState() GraphicsState   { return core.DefaultGraphicsState() }
func DefaultContentOptions() ContentOptions { return contentconfig.DefaultOptions() }
func DefaultLayoutOptions() LayoutOptions   { return layout.DefaultOptions() }
func DefaultTextExtractionOptions() TextExtractionOptions {
	return textconfig.DefaultOptions()
}
func ApplyExternalGraphicsState(d *core.Document, state *GraphicsState, operation ContentOp) {
	core.ApplyExternalGraphicsState(d, state, operation)
}
func ApplyResourceColorSpace(d *core.Document, state *GraphicsState, operation ContentOp) {
	core.ApplyResourceColorSpace(d, state, operation)
}
func ExtractMarkedContent(operations []ContentOp) []MarkedContent {
	return core.ExtractMarkedContent(operations)
}
func AnalyzeLayout(objects []TextObject, options LayoutOptions) LayoutResult {
	return core.AnalyzeLayout(objects, options)
}
