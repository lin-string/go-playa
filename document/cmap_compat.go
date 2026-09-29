package document

import "github.com/lin-string/go-playa/fontdata"

// Keep the core names stable while the CMap parser and value types live in
// the dependency-free fontdata package. Public facades continue to alias
// these names during the migration.
type (
	CMap      = fontdata.CMap
	Code      = fontdata.Code
	CodeSpace = fontdata.CodeSpace
)

func ParseCMap(data []byte) (*CMap, error) {
	return fontdata.ParseCMap(data)
}
