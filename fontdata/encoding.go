package fontdata

// BuiltinEncoding returns an owned copy of a PDF built-in simple-font
// encoding. The source tables are generated from the checked-in authoritative
// Adobe and Apache PDFBox resources.
func BuiltinEncoding(name string) (map[byte]rune, bool) {
	var source map[byte]rune
	switch name {
	case "StandardEncoding":
		source = builtinStandardEncoding
	case "MacRomanEncoding":
		source = builtinMacRomanEncoding
	case "WinAnsiEncoding":
		source = builtinWinAnsiEncoding
	case "MacExpertEncoding":
		source = builtinMacExpertEncoding
	case "Symbol":
		source = generatedSymbolEncoding
	case "ZapfDingbats":
		source = generatedZapfDingbatsEncoding
	default:
		return nil, false
	}
	return cloneRuneEncoding(source), true
}

func asciiEncoding(high map[byte]rune) map[byte]rune {
	result := make(map[byte]rune, len(high)+95)
	for code := byte(32); code < 127; code++ {
		result[code] = rune(code)
	}
	for code, value := range high {
		result[code] = value
	}
	return result
}

var builtinStandardEncoding = buildStandardEncoding()
var builtinMacRomanEncoding = asciiEncoding(generatedMacRomanEncoding)
var builtinWinAnsiEncoding = asciiEncoding(generatedWinAnsiEncoding)
var builtinMacExpertEncoding = buildMacExpertEncoding()

func buildStandardEncoding() map[byte]rune {
	result := make(map[byte]rune)
	for code, sid := range CFFStandardEncoding {
		name, ok := CFFSIDName(int(sid), nil)
		if !ok {
			continue
		}
		text, ok := GlyphText(name)
		if runes := []rune(text); ok && len(runes) == 1 {
			result[byte(code)] = runes[0]
		}
	}
	return result
}

func buildMacExpertEncoding() map[byte]rune {
	result := make(map[byte]rune)
	for code, name := range MacExpertEncoding {
		if name == ".notdef" {
			continue
		}
		text, ok := GlyphText(name)
		if runes := []rune(text); ok && len(runes) == 1 {
			result[byte(code)] = runes[0]
		}
	}
	return result
}

func cloneRuneEncoding(source map[byte]rune) map[byte]rune {
	result := make(map[byte]rune, len(source))
	for code, value := range source {
		result[code] = value
	}
	return result
}
