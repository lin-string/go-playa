// Package playa is the compact public entry point for go-playa.
//
// Domain-specific APIs are also available from subpackages such as content,
// document, font, page, pdftypes, outline, and structure.
package playa

import (
	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/coordinates"
	core "github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/documentconfig"
	"github.com/lin-string/go-playa/fontdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/imagedata"
	"github.com/lin-string/go-playa/layout"
	"github.com/lin-string/go-playa/parserconfig"
	"github.com/lin-string/go-playa/pdftypes"
	"github.com/lin-string/go-playa/structureconfig"
	"github.com/lin-string/go-playa/textconfig"
)

type (
	Document              = core.Document
	Page                  = core.Page
	PageList              = core.PageList
	IndirectObject        = core.IndirectObject
	XRefEntry             = core.XRefEntry
	XRefTable             = core.XRefTable
	PageLabelSpec         = core.PageLabelSpec
	Object                = pdftypes.Object
	Ref                   = pdftypes.Ref
	Dict                  = pdftypes.Dict
	Array                 = pdftypes.Array
	Stream                = pdftypes.Stream
	ContentStream         = pdftypes.ContentStream
	ObjRef                = pdftypes.ObjRef
	Null                  = pdftypes.Null
	Bool                  = pdftypes.Bool
	Number                = pdftypes.Number
	Name                  = pdftypes.Name
	String                = pdftypes.String
	Keyword               = pdftypes.Keyword
	ContentObject         = core.ContentObject
	ContentOp             = core.ContentOp
	ContentKind           = contentconfig.Kind
	ContentFilter         = contentconfig.Filter
	ContentOptions        = contentconfig.Options
	GraphicsState         = core.GraphicsState
	DashPattern           = contentdata.DashPattern
	Color                 = geometry.Color
	ColorSpace            = imagedata.ColorSpace
	PathSegment           = geometry.PathSegment
	XObjectObject         = core.XObjectObject
	TextObject            = core.TextObject
	GlyphObject           = core.GlyphObject
	TagObject             = core.TagObject
	PathObject            = core.PathObject
	ImageObject           = core.ImageObject
	DecodedImage          = imagedata.DecodedImage
	ImageColorSpace       = core.ImageColorSpace
	ShadingObject         = core.ShadingObject
	PatternObject         = core.PatternObject
	ExtGStateObject       = core.ExtGStateObject
	ColorSpaceObject      = core.ColorSpaceObject
	PropertiesObject      = core.PropertiesObject
	Annotation            = core.Annotation
	FormField             = core.FormField
	OutlineNode           = core.OutlineNode
	StructElement         = core.StructElement
	StructureContent      = core.StructureContent
	StructureContentKind  = structureconfig.ContentKind
	StructureItemKind     = structureconfig.ItemKind
	StructureItem         = core.StructureItem
	StructureIndex        = core.StructureIndex
	FontResource          = core.FontResource
	FontMetadata          = fontdata.Metadata
	PageStructureEntry    = core.PageStructureEntry
	Font                  = core.Font
	DeviceSpace           = coordinates.Space
	Matrix                = geometry.Matrix
	Point                 = geometry.Point
	Rect                  = geometry.Rect
	BBoxProvider          = geometry.BBoxProvider
	Token                 = core.Token
	TokenKind             = parserconfig.TokenKind
	CoordinateSpace       = coordinates.Space
	CacheOptions          = cacheconfig.Options
	DocumentOptions       = documentconfig.Options
	OpenOption            = documentconfig.OpenOption
	Destination           = core.Destination
	DestinationEntry      = core.DestinationEntry
	NameTreeEntry         = core.NameTreeEntry
	Action                = core.Action
	PageStructure         = core.PageStructure
	ParseError            = core.ParseError
	Metadata              = core.Metadata
	EncryptionInfo        = core.EncryptionInfo
	MarkedContent         = core.MarkedContent
	MarkedContentIndex    = core.MarkedContentIndex
	MarkedContentContext  = core.MarkedContentContext
	ContentSection        = core.ContentSection
	ContentSequence       = core.ContentSequence
	TextExtractionOptions = textconfig.Options
	PageConcurrencyOption = core.PageConcurrencyOption
	TextWord              = core.TextWord
	TextLine              = core.TextLine
	TextParagraph         = core.TextParagraph
	TextBox               = core.TextBox
	TextGroup             = core.TextGroup
	TextGroupChild        = core.TextGroupChild
	LayoutOptions         = layout.Options
	LayoutResult          = core.LayoutResult
	LayoutItem            = core.LayoutItem
	LayoutItemKind        = core.LayoutItemKind
)

var ErrEncrypted = core.ErrEncrypted
var ErrNilDocument = core.ErrNilDocument
var ErrObjectNotFound = core.ErrObjectNotFound
var ErrPageNotFound = core.ErrPageNotFound
var ErrPasswordRequired = core.ErrPasswordRequired
var ErrInvalidPassword = core.ErrInvalidPassword
var ErrUnsupportedEncryption = core.ErrUnsupportedEncryption
var ErrContentMCIDOutOfRange = core.ErrContentMCIDOutOfRange

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

	StructureMarkedContent     = structureconfig.MarkedContent
	StructureObject            = structureconfig.Object
	StructureItemElement       = structureconfig.ItemElement
	StructureItemMarkedContent = structureconfig.ItemMarkedContent
	StructureItemObject        = structureconfig.ItemObject

	CoordinateSpacePage    = coordinates.Page
	CoordinateSpaceScreen  = coordinates.Screen
	CoordinateSpaceDefault = coordinates.Default
	CoordinateSpaceUser    = coordinates.User
)

func Open(path string, options ...OpenOption) (*Document, error) { return core.Open(path, options...) }
func OpenBytes(data []byte, options ...OpenOption) (*Document, error) {
	return core.OpenBytes(data, options...)
}
func WithCoordinateSpace(space CoordinateSpace) OpenOption { return core.WithCoordinateSpace(space) }
func WithPassword(password string) OpenOption              { return core.WithPassword(password) }
func WithCacheOptions(options CacheOptions) OpenOption     { return core.WithCacheOptions(options) }
func DefaultCacheOptions() CacheOptions                    { return core.DefaultCacheOptions() }
func WithDocumentOptions(options DocumentOptions) OpenOption {
	return core.WithDocumentOptions(options)
}
func DefaultDocumentOptions() DocumentOptions { return core.DefaultDocumentOptions() }
func DefaultContentOptions() ContentOptions   { return contentconfig.DefaultOptions() }
func DefaultLayoutOptions() LayoutOptions     { return layout.DefaultOptions() }
func DefaultTextExtractionOptions() TextExtractionOptions {
	return textconfig.DefaultOptions()
}

// WithMaxPageWorkers sets the maximum number of page workers. A non-positive
// value selects GOMAXPROCS; all values are bounded by GOMAXPROCS and page count.
func WithMaxPageWorkers(workers int) PageConcurrencyOption {
	return core.WithMaxPageWorkers(workers)
}

// WithPagesPerWorker sets the page budget for each potential worker. A
// non-positive value uses the default of ten pages per worker.
func WithPagesPerWorker(pages int) PageConcurrencyOption {
	return core.WithPagesPerWorker(pages)
}
func DefaultGraphicsState() GraphicsState { return core.DefaultGraphicsState() }

// NewColorSpace constructs a dependency-free PDF color-space value.
func NewColorSpace(name string, components int) ColorSpace {
	return imagedata.NewColorSpace(name, components)
}

// Resolve follows indirect references using a caller-provided object lookup.
func Resolve(value Object, resolve func(ObjRef) (Object, bool), defaults ...Object) Object {
	return pdftypes.Resolve(value, resolve, defaults...)
}

// ResolveAll recursively copies a PDF object while resolving nested references.
func ResolveAll(value Object, resolve func(ObjRef) (Object, bool)) Object {
	return pdftypes.ResolveAll(value, resolve)
}

// AsObject returns a JSON-friendly metadata view of a PDF primitive graph.
func AsObject(value Object) any { return pdftypes.AsObject(value) }
