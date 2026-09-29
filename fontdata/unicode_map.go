package fontdata

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf16"
	"unicode/utf8"
)

// UnicodeMap is an immutable CID-to-Unicode map loaded from the synchronized
// Adobe Unicode CMap resources. The backing map is never exposed; callers that
// need an owned snapshot can use MappingCopy or Finalize.
type UnicodeMap struct {
	name     string
	vertical bool
	mapping  map[int]string
}

type unicodeMapKey struct {
	ordering string
	vertical bool
}

type unicodeMapLoad struct {
	done    chan struct{}
	unicode *UnicodeMap
	err     error
}

var (
	predefinedUnicodeMapMu    sync.RWMutex
	predefinedUnicodeMapCache = map[unicodeMapKey]*UnicodeMap{}
	predefinedUnicodeMapLoads = map[unicodeMapKey]*unicodeMapLoad{}
)

// Name returns the canonical Adobe CID collection name.
func (m *UnicodeMap) Name() string {
	return m.name
}

// Vertical reports whether this map is the vertical-writing variant.
func (m *UnicodeMap) Vertical() bool { return m.vertical }

// Lookup returns the Unicode text for one CID without exposing the map.
func (m *UnicodeMap) Lookup(cid int) (string, bool) {
	value, ok := m.mapping[cid]
	return value, ok
}

// Unicode returns the Unicode text for one CID, or an empty string when the
// CID is not present. This is the read-only equivalent of Playa's get_unichr.
func (m *UnicodeMap) Unicode(cid int) string {
	value, _ := m.Lookup(cid)
	return value
}

// MappingCopy returns an independent copy of the complete CID mapping.
func (m *UnicodeMap) MappingCopy() map[int]string {
	if m.mapping == nil {
		return nil
	}
	out := make(map[int]string, len(m.mapping))
	for cid, value := range m.mapping {
		out[cid] = value
	}
	return out
}

// Finalize returns an independent immutable snapshot of the UnicodeMap.
func (m *UnicodeMap) Finalize() *UnicodeMap {
	return &UnicodeMap{name: m.name, vertical: m.vertical, mapping: m.MappingCopy()}
}

// CacheBytes estimates the retained resource footprint for bounded caches.
func (m *UnicodeMap) CacheBytes() int {
	size := 96 + len(m.name)
	for cid, value := range m.mapping {
		size += 24 + len(value) + intSize(cid)
	}
	return size
}

// LoadPredefinedUnicodeMap returns the canonical cached Unicode map for an
// Adobe CID collection. Both "Adobe-Japan1" and the shorter "Japan1" form
// are accepted because PDF dictionaries commonly expose the Ordering alone.
func LoadPredefinedUnicodeMap(name string, vertical bool) (*UnicodeMap, error) {
	ordering, ok := predefinedUnicodeOrdering(name)
	if !ok {
		return nil, fmt.Errorf("playa: predefined UnicodeMap %q not found", name)
	}
	key := unicodeMapKey{ordering: ordering, vertical: vertical}

	predefinedUnicodeMapMu.RLock()
	if cached, ok := predefinedUnicodeMapCache[key]; ok {
		predefinedUnicodeMapMu.RUnlock()
		return cached, nil
	}
	if loading, ok := predefinedUnicodeMapLoads[key]; ok {
		predefinedUnicodeMapMu.RUnlock()
		<-loading.done
		return loading.unicode, loading.err
	}
	predefinedUnicodeMapMu.RUnlock()

	predefinedUnicodeMapMu.Lock()
	if cached, ok := predefinedUnicodeMapCache[key]; ok {
		predefinedUnicodeMapMu.Unlock()
		return cached, nil
	}
	if loading, ok := predefinedUnicodeMapLoads[key]; ok {
		predefinedUnicodeMapMu.Unlock()
		<-loading.done
		return loading.unicode, loading.err
	}
	loading := &unicodeMapLoad{done: make(chan struct{})}
	predefinedUnicodeMapLoads[key] = loading
	predefinedUnicodeMapMu.Unlock()

	unicode, err := buildPredefinedUnicodeMap(ordering, vertical)

	predefinedUnicodeMapMu.Lock()
	if err == nil {
		if cached, ok := predefinedUnicodeMapCache[key]; ok {
			unicode = cached
		} else {
			predefinedUnicodeMapCache[key] = unicode
		}
	}
	loading.unicode = unicode
	loading.err = err
	delete(predefinedUnicodeMapLoads, key)
	close(loading.done)
	predefinedUnicodeMapMu.Unlock()
	return unicode, err
}

func predefinedUnicodeOrdering(name string) (string, bool) {
	ordering := strings.TrimPrefix(name, "Adobe-")
	if ordering == name && strings.Contains(name, "-") {
		// Only the Adobe registry is represented by this generated resource
		// set. Reject unrelated registry-ordering names instead of silently
		// selecting a potentially wrong character collection.
		return "", false
	}
	if _, ok := PredefinedCIDUnicodeMapName(ordering); !ok {
		return "", false
	}
	return ordering, true
}

func buildPredefinedUnicodeMap(ordering string, vertical bool) (*UnicodeMap, error) {
	baseName, ok := PredefinedCIDUnicodeMapName(ordering)
	if !ok {
		return nil, fmt.Errorf("playa: predefined UnicodeMap ordering %q not found", ordering)
	}
	cmapName := baseName
	if vertical {
		verticalName := strings.TrimSuffix(baseName, "-H") + "-V"
		if _, exists := predefinedCMapNames[verticalName]; exists {
			cmapName = verticalName
		}
	}
	cmap, err := LoadPredefinedCMap(cmapName)
	if err != nil {
		return nil, fmt.Errorf("playa: load predefined UnicodeMap CMap %q: %w", cmapName, err)
	}
	mapping := cmap.MappingCopy()
	codes := make([]string, 0, len(mapping))
	for code := range mapping {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	reverse := make(map[int]string)
	for _, code := range codes {
		codePoint, ok := CMapUnicodeCodePoint(cmapName, []byte(code))
		if !ok {
			continue
		}
		text := string(rune(codePoint))
		cid := mapping[code]
		if previous, exists := reverse[cid]; !exists || text < previous {
			reverse[cid] = text
		}
	}
	return &UnicodeMap{name: "Adobe-" + ordering, vertical: vertical, mapping: reverse}, nil
}

func intSize(value int) int {
	if value < 0 {
		value = -value
	}
	if value < 1<<8 {
		return 1
	}
	if value < 1<<16 {
		return 2
	}
	if value < 1<<32 {
		return 4
	}
	return 8
}

// CMapUnicodeCodePoint validates and decodes one source code from an Adobe
// Unicode CMap according to its UTF-8/16/32 collection naming convention.
func CMapUnicodeCodePoint(cmapName string, code []byte) (uint32, bool) {
	if strings.Contains(cmapName, "UTF8") {
		if !utf8.Valid(code) {
			return 0, false
		}
		runeValue, size := utf8.DecodeRune(code)
		return uint32(runeValue), size == len(code)
	}
	if strings.Contains(cmapName, "UTF16") {
		if len(code) == 0 || len(code)%2 != 0 {
			return 0, false
		}
		units := make([]uint16, len(code)/2)
		for i := range units {
			units[i] = binary.BigEndian.Uint16(code[i*2:])
		}
		if len(units) == 1 && units[0] >= 0xd800 && units[0] <= 0xdfff {
			return 0, false
		}
		decoded := utf16.Decode(units)
		if len(decoded) != 1 {
			return 0, false
		}
		if decoded[0] == utf8.RuneError && (len(units) != 1 || units[0] != 0xfffd) {
			return 0, false
		}
		return uint32(decoded[0]), true
	}
	if strings.Contains(cmapName, "UTF32") {
		if len(code) != 4 {
			return 0, false
		}
		value := binary.BigEndian.Uint32(code)
		if value > 0x10ffff || (value >= 0xd800 && value <= 0xdfff) {
			return 0, false
		}
		return value, true
	}
	switch len(code) {
	case 1:
		return uint32(code[0]), true
	case 2:
		return uint32(binary.BigEndian.Uint16(code)), true
	case 4:
		value := binary.BigEndian.Uint32(code)
		if value > 0x10ffff || (value >= 0xd800 && value <= 0xdfff) {
			return 0, false
		}
		return value, true
	default:
		return 0, false
	}
}
