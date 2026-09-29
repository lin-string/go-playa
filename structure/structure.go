// Package structure exposes tagged-PDF structure-tree models.
package structure

import (
	core "github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/parser"
	"github.com/lin-string/go-playa/pdftypes"
	"github.com/lin-string/go-playa/structureconfig"
)

var ErrPageNotFound = core.ErrPageNotFound
var ErrNilDocument = core.ErrNilDocument

type (
	ParseError         = parser.ParseError
	StructureItem      = core.StructureItem
	StructureItemKind  = structureconfig.ItemKind
	Element            = core.StructElement
	Index              = core.StructureIndex
	Content            = core.StructureContent
	ContentObject      = core.ContentObject
	ContentKind        = structureconfig.ContentKind
	PageStructureEntry = core.PageStructureEntry
	Page               = core.Page
	PDFObject          = pdftypes.Object
	Ref                = pdftypes.Ref
	Dict               = pdftypes.Dict
	Array              = pdftypes.Array
	Stream             = pdftypes.Stream
	Null               = pdftypes.Null
	Bool               = pdftypes.Bool
	Number             = pdftypes.Number
	Name               = pdftypes.Name
	String             = pdftypes.String
	Keyword            = pdftypes.Keyword
)

const (
	StructureMarkedContent     = structureconfig.MarkedContent
	StructureObject            = structureconfig.Object
	StructureItemElement       = structureconfig.ItemElement
	StructureItemMarkedContent = structureconfig.ItemMarkedContent
	StructureItemObject        = structureconfig.ItemObject
)
