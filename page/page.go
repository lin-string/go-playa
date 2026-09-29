// Package page exposes page-level PDF models.
package page

import (
	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/coordinates"
	core "github.com/lin-string/go-playa/document"
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
	Page                  = core.Page
	PageLabelSpec         = core.PageLabelSpec
	Annotation            = core.Annotation
	FormField             = core.FormField
	ContentFilter         = contentconfig.Filter
	ContentKind           = contentconfig.Kind
	ContentOptions        = contentconfig.Options
	ContentObject         = core.ContentObject
	ContentOp             = core.ContentOp
	GraphicsState         = core.GraphicsState
	DashPattern           = contentdata.DashPattern
	Color                 = geometry.Color
	PathSegment           = geometry.PathSegment
	XObjectObject         = core.XObjectObject
	TextObject            = core.TextObject
	GlyphObject           = core.GlyphObject
	PathObject            = core.PathObject
	ImageObject           = core.ImageObject
	ShadingObject         = core.ShadingObject
	PatternObject         = core.PatternObject
	ExtGStateObject       = core.ExtGStateObject
	ColorSpaceObject      = core.ColorSpaceObject
	PropertiesObject      = core.PropertiesObject
	TagObject             = core.TagObject
	PageStructureEntry    = core.PageStructureEntry
	PageStructure         = core.PageStructure
	StructureItem         = core.StructureItem
	FontResource          = core.FontResource
	Font                  = core.Font
	FontMetadata          = fontdata.Metadata
	Action                = core.Action
	Destination           = core.Destination
	StructElement         = core.StructElement
	Matrix                = geometry.Matrix
	MarkedContent         = core.MarkedContent
	MarkedContentIndex    = core.MarkedContentIndex
	MarkedContentContext  = core.MarkedContentContext
	ContentSection        = core.ContentSection
	ContentSequence       = core.ContentSequence
	Token                 = core.Token
	TokenKind             = parserconfig.TokenKind
	Stream                = core.Stream
	Object                = pdftypes.Object
	Ref                   = pdftypes.Ref
	Dict                  = pdftypes.Dict
	Array                 = pdftypes.Array
	Null                  = pdftypes.Null
	Bool                  = pdftypes.Bool
	Number                = pdftypes.Number
	Name                  = pdftypes.Name
	String                = pdftypes.String
	Keyword               = pdftypes.Keyword
	DecodedImage          = imagedata.DecodedImage
	ImageColorSpace       = core.ImageColorSpace
	CoordinateSpace       = coordinates.Space
	TextExtractionOptions = textconfig.Options
	LayoutOptions         = layout.Options
	LayoutResult          = core.LayoutResult
	LayoutItem            = core.LayoutItem
	LayoutItemKind        = core.LayoutItemKind
	TextWord              = core.TextWord
	TextLine              = core.TextLine
	TextParagraph         = core.TextParagraph
	TextBox               = core.TextBox
	TextGroup             = core.TextGroup
	TextGroupChild        = core.TextGroupChild
	BBoxProvider          = geometry.BBoxProvider
)

const (
	LayoutTextBox = core.LayoutTextBox
	LayoutImage   = core.LayoutImage
	LayoutPath    = core.LayoutPath
	LayoutXObject = core.LayoutXObject

	FilterAll     = core.FilterAll
	FilterText    = core.FilterText
	FilterPath    = core.FilterPath
	FilterImage   = core.FilterImage
	FilterXObject = core.FilterXObject
	FilterTag     = core.FilterTag

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

func DefaultGraphicsState() GraphicsState                 { return core.DefaultGraphicsState() }
func DefaultContentOptions() ContentOptions               { return contentconfig.DefaultOptions() }
func DefaultLayoutOptions() LayoutOptions                 { return layout.DefaultOptions() }
func DefaultTextExtractionOptions() TextExtractionOptions { return textconfig.DefaultOptions() }
