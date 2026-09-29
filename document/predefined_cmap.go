package document

import "github.com/lin-string/go-playa/fontdata"

var predefinedCMapFiles = fontdata.CMapFiles

func loadPredefinedCMap(name string) (*fontdata.CMap, error) {
	return fontdata.LoadPredefinedCMap(name)
}

func cloneCMap(cmap *fontdata.CMap) *fontdata.CMap { return cmap.Finalize() }

// LoadPredefinedCMap returns the cached immutable Adobe public CMap.
func LoadPredefinedCMap(name string) (*fontdata.CMap, error) {
	return loadPredefinedCMap(name)
}

func identityPredefinedCMap(name string) (*fontdata.CMap, bool) {
	switch name {
	case "DLIdent-H":
		return fontdata.NewIdentityCMap(false, 2), true
	case "DLIdent-V":
		return fontdata.NewIdentityCMap(true, 2), true
	case "OneByteIdentityH":
		return fontdata.NewIdentityCMap(false, 1), true
	case "OneByteIdentityV":
		return fontdata.NewIdentityCMap(true, 1), true
	default:
		return nil, false
	}
}
