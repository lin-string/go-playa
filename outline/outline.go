// Package outline exposes destinations, actions, and document outlines.
package outline

import (
	core "github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/parser"
	"github.com/lin-string/go-playa/pdftypes"
)

var ErrPageNotFound = core.ErrPageNotFound
var ErrNilDocument = core.ErrNilDocument

type (
	ParseError    = parser.ParseError
	Destination   = core.Destination
	Action        = core.Action
	Page          = core.Page
	StructElement = core.StructElement
	Node          = core.OutlineNode
	Object        = pdftypes.Object
	Ref           = pdftypes.Ref
	Dict          = pdftypes.Dict
	Array         = pdftypes.Array
	Null          = pdftypes.Null
	Bool          = pdftypes.Bool
	Number        = pdftypes.Number
	Name          = pdftypes.Name
	String        = pdftypes.String
	Keyword       = pdftypes.Keyword
)
