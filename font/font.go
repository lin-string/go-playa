// Package font exposes PDF font, encoding, and CMap models.
package font

import (
	core "github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/fontdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/parser"
	"github.com/lin-string/go-playa/pdftypes"
)

type (
	ParseError   = parser.ParseError
	Font         = core.Font
	FontMetadata = fontdata.Metadata
	Matrix       = geometry.Matrix
	CMap         = fontdata.CMap
	UnicodeMap   = fontdata.UnicodeMap
	ToUnicodeMap = fontdata.ToUnicodeMap
	Code         = fontdata.Code
	CodeSpace    = fontdata.CodeSpace
	DecodedGlyph = fontdata.DecodedGlyph
	Object       = pdftypes.Object
	Ref          = pdftypes.Ref
	Dict         = pdftypes.Dict
	Array        = pdftypes.Array
	Stream       = pdftypes.Stream
	Null         = pdftypes.Null
	Bool         = pdftypes.Bool
	Number       = pdftypes.Number
	Name         = pdftypes.Name
	String       = pdftypes.String
	Keyword      = pdftypes.Keyword
)

func NewSimple(name string) *Font                   { return core.NewSimpleFont(name) }
func ParseCMap(data []byte) (*CMap, error)          { return fontdata.ParseCMap(data) }
func ParseEncodingCMap(data []byte) (*CMap, error)  { return fontdata.ParseEncodingCMap(data) }
func LoadPredefinedCMap(name string) (*CMap, error) { return fontdata.LoadPredefinedCMap(name) }
func LoadPredefinedUnicodeMap(name string, vertical bool) (*UnicodeMap, error) {
	return fontdata.LoadPredefinedUnicodeMap(name, vertical)
}
func ParseToUnicode(data []byte) (map[uint16]string, error) { return fontdata.ParseToUnicode(data) }

// ParseToUnicodeMap preserves source-code widths and codespace-aware decode
// semantics instead of narrowing the result to the historical uint16 map.
func ParseToUnicodeMap(data []byte) (*ToUnicodeMap, error) {
	return fontdata.ParseToUnicodeMap(data)
}

func ParseWidth(value pdftypes.Object) (float64, error) { return fontdata.ParseWidth(value) }

// ParseToUnicodeCodes preserves the complete source-code bytes from a
// ToUnicode map, including three- and four-byte codes that ParseToUnicode
// cannot represent in its historical uint16-keyed result.
func ParseToUnicodeCodes(data []byte) (map[string]string, error) {
	return fontdata.ParseToUnicodeCodes(data)
}
