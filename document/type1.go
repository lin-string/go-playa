package document

import (
	"fmt"
	"sync/atomic"

	"github.com/lin-string/go-playa/fontdata"
)

const type1PathCacheLimit = 64 << 10

// parseType1Encoding reads the cleartext Encoding assignments commonly found
// in embedded Type 1 font headers. It intentionally ignores executable
// PostScript and only accepts the well-formed `dup code /name put` pattern.
func parseType1Encoding(data []byte) map[byte]string {
	return fontdata.ParseType1Encoding(data)
}

func parseType1EexecEncoding(data []byte, length1 int) map[byte]string {
	return fontdata.ParseType1EexecEncoding(data, length1)
}

func applyType1Encoding(d *Document, f *Font, desc Dict) {
	streamValue, _ := d.resolveIndirectChain(desc[Name("FontFile")])
	stream, ok := streamValue.(Stream)
	if !ok {
		return
	}
	streamDict := stream.DictBorrowed()
	filters, parms := streamFiltersWithResolver(streamDict, func(value Object) Object {
		resolved, _ := d.resolveIndirectChain(value)
		return resolved
	})
	f.type1Data = cloneObjectBytes(stream.DataBorrowed())
	f.type1Filters = cloneFontStrings(filters)
	f.type1Parms = cloneFilterParms(parms)
	f.type1Length1 = -1
	lengthValue, _ := d.resolveIndirectChain(streamDict[Name("Length1")])
	if length, ok := IntValue(lengthValue); ok {
		f.type1Length1 = length
	}
	f.type1Parsed = false
}

func (f *Font) ensureType1Encoding() {
	_, _ = f.ensureType1EncodingWithError()
}

func (f *Font) ensureType1EncodingWithError() (bool, error) {
	if f == nil {
		return false, nil
	}
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	f.ensureType1EncodingLocked()
	return f.type1Parsed, f.type1Err
}

func (f *Font) ensureType1EncodingLocked() {
	if f == nil || f.type1Parsed || f.type1Data == nil {
		return
	}
	f.type1Parsed = true
	data := f.type1Data
	defer func() {
		f.type1Data = nil
		f.type1Filters = nil
		f.type1Parms = nil
	}()
	if len(f.type1Filters) > 0 {
		decoded, err := decodeFiltersLimited(data, f.type1Filters, f.type1Parms, decodedFilterExpansionLimit)
		if err != nil {
			f.type1Err = fmt.Errorf("playa: decode embedded Type1: %w", err)
			return
		}
		data = decoded
	}
	cleartext := data
	if f.type1Length1 >= 0 && f.type1Length1 < len(data) {
		cleartext = data[:f.type1Length1]
	}
	assignments := parseType1Encoding(cleartext)
	if len(assignments) == 0 {
		assignments = parseType1EexecEncoding(data, f.type1Length1)
	}
	if assignments == nil {
		assignments = map[byte]string{}
	}
	for code, name := range f.type1Differences {
		assignments[code] = name
	}
	f.type1Differences = nil
	if f.type1Length1 >= 0 && f.type1Length1 < len(data) {
		payload := data[f.type1Length1:]
		if decoded, ok := fontdata.DecodeType1HexEexec(payload); ok {
			payload = decoded
		}
		if len(payload) > 4 {
			decrypted := fontdata.DecryptType1Eexec(payload)
			program := fontdata.ParseType1Program(decrypted[4:])
			f.type1Charstrings = program.CharStringsCopy()
			f.type1Subrs = program.SubrsCopy()
		}
	}
	// The program supplies the complete implicit encoding, not Differences
	// over StandardEncoding. Replace tables so published snapshots stay owned.
	f.encoding = map[byte]rune{}
	f.glyphTexts = map[byte]string{}
	if len(assignments) > 0 {
		f.glyphNames = map[byte]string{}
	} else {
		// An outline-only program can have names supplied by its caller.
		f.glyphNames = cloneByteStringMap(f.glyphNames)
	}
	f.strictEncoding = true
	atomic.StorePointer(&f.widthState, nil)
	for code, name := range assignments {
		f.glyphNames[code] = name
		if text, ok := glyphText(name); ok {
			f.glyphTexts[code] = text
		}
	}
	// Playa's SimpleFont uses StandardEncoding for text when the entire
	// encoding has no meaningful glyphs and there is no ToUnicode map. Keep
	// the raw glyph names for outline selection and the fallback decode-only.
	if len(f.glyphTexts) == 0 && f.toUnicodeData == nil && !f.toUnicodeParsed {
		standard, _ := fontdata.BuiltinEncoding("StandardEncoding")
		for code, value := range standard {
			f.glyphTexts[code] = string(value)
		}
	}
}

func cloneStringByteMap(source map[string][]byte) map[string][]byte {
	if source == nil {
		return nil
	}
	result := make(map[string][]byte, len(source))
	for key, value := range source {
		result[key] = nil
		if len(value) > 0 {
			result[key] = cloneObjectBytes(value)
		}
	}
	return result
}

func (f *Font) type1GlyphPathOps(code []byte) ([]ContentOp, bool) {
	ops, ok, _ := f.type1GlyphPathOpsWithError(code)
	return ops, ok
}

func (f *Font) type1GlyphPathOpsWithError(code []byte) ([]ContentOp, bool, error) {
	if f == nil || len(code) == 0 {
		return nil, false, nil
	}
	f.lazyMutex().Lock()
	defer f.lazyMutex().Unlock()
	f.ensureType1EncodingLocked()
	if f.type1Err != nil {
		return nil, false, f.type1Err
	}
	name := f.glyphNames[code[len(code)-1]]
	if name == "" {
		return nil, false, nil
	}
	if cached, ok := f.type1PathCache[name]; ok {
		return cloneContentOps(cached), true, nil
	}
	charstring := f.type1Charstrings[name]
	if len(charstring) == 0 {
		return nil, false, nil
	}
	resolveSeac := func(charCode byte) []byte {
		return f.type1Charstrings[f.glyphNames[charCode]]
	}
	ops, ok := type1CharStringOpsWithSeac(charstring, f.type1Subrs, resolveSeac)
	if !ok {
		return nil, false, fmt.Errorf("playa: invalid Type1 CharString for glyph %q", name)
	}
	cacheBytes := contentOpsCacheSize(ops)
	if cacheBytes <= f.type1PathCacheBudget() {
		if f.type1PathCache == nil {
			f.type1PathCache = map[string][]ContentOp{}
		}
		f.type1PathCache[name] = cloneContentOps(ops)
		f.type1PathCacheBytes = addCacheSize(f.type1PathCacheBytes, cacheBytes)
	}
	return cloneContentOps(ops), true, nil
}
