package document

import "github.com/lin-string/go-playa/fontdata"

func testCodeSpace(low, high []byte) CodeSpace {
	return fontdata.NewCodeSpace(low, high)
}

func testCMap(codespaces []CodeSpace, mapping map[string]int, vertical bool) *CMap {
	return fontdata.NewCMap(codespaces, mapping, vertical, "")
}
