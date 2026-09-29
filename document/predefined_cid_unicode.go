package document

import "github.com/lin-string/go-playa/fontdata"

func predefinedCIDUnicode(ordering string, vertical ...bool) map[int]string {
	useVertical := len(vertical) > 0 && vertical[0]
	mapping, err := fontdata.LoadPredefinedUnicodeMap(ordering, useVertical)
	if err != nil {
		return nil
	}
	return mapping.MappingCopy()
}

func cmapUnicodeCodePoint(cmapName string, code []byte) (uint32, bool) {
	return fontdata.CMapUnicodeCodePoint(cmapName, code)
}
