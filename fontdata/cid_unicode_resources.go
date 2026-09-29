package fontdata

// predefinedCIDUnicodeCMaps maps Adobe CID collection orderings to the
// synchronized UTF-32 horizontal CMaps used for fallback Unicode recovery.
var predefinedCIDUnicodeCMaps = map[string]string{
	"Japan1": "UniJIS-UTF32-H",
	"Japan2": "UniJISX0213-UTF32-H",
	"GB1":    "UniGB-UTF32-H",
	"CNS1":   "UniCNS-UTF32-H",
	"Korea1": "UniKS-UTF32-H",
	"KR":     "UniAKR-UTF32-H",
	"Manga1": "UniManga-UTF32-H",
}

// PredefinedCIDUnicodeMapName returns the synchronized CMap name for an Adobe
// CID collection ordering.
func PredefinedCIDUnicodeMapName(ordering string) (string, bool) {
	name, ok := predefinedCIDUnicodeCMaps[ordering]
	return name, ok
}
