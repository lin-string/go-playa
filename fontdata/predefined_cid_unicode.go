package fontdata

import (
	"sync"
)

var (
	predefinedCIDUnicodeMu    sync.RWMutex
	predefinedCIDUnicodeCache = map[string]map[int]string{}
)

// PredefinedCIDUnicode returns a copy of the standard CID collection's
// reverse Unicode mapping. It is only a fallback; document ToUnicode data and
// embedded font cmap data are resolved earlier.
func PredefinedCIDUnicode(ordering string) map[int]string {
	predefinedCIDUnicodeMu.RLock()
	if cached, ok := predefinedCIDUnicodeCache[ordering]; ok {
		result := cloneCIDUnicode(cached)
		predefinedCIDUnicodeMu.RUnlock()
		return result
	}
	predefinedCIDUnicodeMu.RUnlock()

	unicodeMap, err := LoadPredefinedUnicodeMap("Adobe-"+ordering, false)
	if err != nil {
		return nil
	}
	reverse := unicodeMap.MappingCopy()

	predefinedCIDUnicodeMu.Lock()
	if cached, ok := predefinedCIDUnicodeCache[ordering]; ok {
		result := cloneCIDUnicode(cached)
		predefinedCIDUnicodeMu.Unlock()
		return result
	}
	predefinedCIDUnicodeCache[ordering] = reverse
	result := cloneCIDUnicode(reverse)
	predefinedCIDUnicodeMu.Unlock()
	return result
}

func cloneCIDUnicode(source map[int]string) map[int]string {
	if source == nil {
		return nil
	}
	clone := make(map[int]string, len(source))
	for cid, text := range source {
		clone[cid] = text
	}
	return clone
}
