package fontdata

import (
	"strconv"
	"strings"
)

// legacyGlyphText contains the small set of scoped aliases retained for
// compatibility with historical Playa/PDF font behavior. The Adobe and ITC
// source tables remain authoritative for all other glyph names.
var legacyGlyphText = map[string]string{
	"applelogo":   "\uf8ff",
	"f_f":         "\ufb00",
	"f_f_i":       "\ufb03",
	"f_f_l":       "\ufb04",
	"f_i":         "\ufb01",
	"f_l":         "\ufb02",
	"hyphenminus": "-",
	"nbhyphen":    "\u2011",
}

// GlyphTextWithLegacyAliases resolves a glyph name using the source-first
// mapping plus the small historical aliases required by the PDF compatibility
// path. GlyphText itself intentionally keeps the normative composite-name
// behavior and does not apply these aliases.
func GlyphTextWithLegacyAliases(name string) (string, bool) {
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		name = name[:dot]
	}
	if text, ok := legacyGlyphText[name]; ok {
		return text, true
	}
	return GlyphText(name)
}

// GlyphText resolves a glyph name using the Adobe Glyph List and its
// documented name syntax. The returned string is independent immutable text;
// unknown names and invalid Unicode scalar values return false.
func GlyphText(name string) (string, bool) {
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		name = name[:dot]
	}
	if name == "" {
		return "", false
	}
	if strings.Contains(name, "_") {
		var out strings.Builder
		for _, component := range strings.Split(name, "_") {
			text, ok := GlyphText(component)
			if !ok {
				return "", false
			}
			out.WriteString(text)
		}
		return out.String(), true
	}
	if text, ok := adobeGlyphText[name]; ok {
		return text, true
	}
	if text, ok := zapfDingbatsGlyphText[name]; ok {
		return text, true
	}
	if strings.HasPrefix(name, "u") && !strings.HasPrefix(name, "uni") {
		value := name[1:]
		if len(value) < 4 || len(value) > 6 {
			return "", false
		}
		code, err := strconv.ParseUint(value, 16, 21)
		if err != nil || !validScalar(code) {
			return "", false
		}
		return string(rune(code)), true
	}
	if !strings.HasPrefix(name, "uni") {
		return "", false
	}
	value := name[3:]
	if len(value) == 0 || len(value)%4 != 0 {
		return "", false
	}
	var out strings.Builder
	for offset := 0; offset < len(value); offset += 4 {
		code, err := strconv.ParseUint(value[offset:offset+4], 16, 16)
		if err != nil {
			return "", false
		}
		// Playa decodes uniXXXX sequences with UTF-16 errors="ignore":
		// an isolated surrogate is discarded while valid code units remain.
		if !validScalar(code) {
			continue
		}
		out.WriteRune(rune(code))
	}
	if out.Len() == 0 {
		return "", false
	}
	return out.String(), true
}

func validScalar(value uint64) bool {
	return value <= 0x10ffff && (value < 0xd800 || value > 0xdfff)
}
