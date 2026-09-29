// Package image exposes image objects, color-space metadata, and stream decoding.
package image

import (
	core "github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/imagedata"
	"github.com/lin-string/go-playa/parser"
	"github.com/lin-string/go-playa/pdftypes"
)

var ErrPageNotFound = core.ErrPageNotFound
var ErrNilDocument = core.ErrNilDocument

type (
	ParseError      = parser.ParseError
	ImageObject     = core.ImageObject
	ColorSpace      = imagedata.ColorSpace
	DecodedImage    = imagedata.DecodedImage
	ImageColorSpace = core.ImageColorSpace
	Page            = core.Page
	StructElement   = core.StructElement
	Object          = pdftypes.Object
	Ref             = pdftypes.Ref
	Dict            = pdftypes.Dict
	Array           = pdftypes.Array
	Stream          = pdftypes.Stream
	Null            = pdftypes.Null
	Bool            = pdftypes.Bool
	Number          = pdftypes.Number
	Name            = pdftypes.Name
	String          = pdftypes.String
	Keyword         = pdftypes.Keyword
)

// DecodeFilters decodes a PDF image or stream filter chain in order.
func DecodeFilters(data []byte, filters []string, parameters []pdftypes.Dict) ([]byte, error) {
	return parser.DecodeFilters(data, filters, parameters)
}

// UnpackData expands packed 1-, 2-, or 4-bit image samples while preserving
// row boundaries. Other bit depths are returned unchanged.
func UnpackData(data []byte, bitsPerComponent, width, height, components int) []byte {
	return imagedata.Unpack(data, bitsPerComponent, width, height, components)
}
