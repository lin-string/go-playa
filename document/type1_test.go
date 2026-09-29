package document

import (
	"encoding/hex"
	"testing"
)

func TestParseType1EncodingReadsDupAssignments(t *testing.T) {
	data := []byte("%!PS-AdobeFont-1.0\n/Encoding 256 array\ndup 65 /Aacute put\ndup 66 /custom put\nreadonly def\n")
	got := parseType1Encoding(data)
	if got[65] != "Aacute" || got[66] != "custom" {
		t.Fatalf("Type1 encoding = %#v", got)
	}
}

func TestParseType1EncodingReadsAssignmentsWithoutDup(t *testing.T) {
	data := []byte("65 /Aacute put\n66 /custom put\n")
	got := parseType1Encoding(data)
	if got[65] != "Aacute" || got[66] != "custom" {
		t.Fatalf("Type1 encoding without dup = %#v", got)
	}
}

func TestApplyType1EncodingPreservesEmptyEmbeddedSlices(t *testing.T) {
	font := NewSimpleFont("Type1")
	applyType1Encoding(&Document{}, font, Dict{
		Name("FontFile"): newStream(Dict{}, make([]byte, 0)),
	})
	if font.type1Data == nil {
		t.Fatal("empty Type1 embedded data became nil")
	}
}

func TestParseType1EncodingRejectsOutOfRangeCodes(t *testing.T) {
	data := []byte("dup 1e100 /tooLarge put\ndup 65 /A put\n")
	got := parseType1Encoding(data)
	if len(got) != 1 || got[65] != "A" {
		t.Fatalf("Type1 encoding accepted an out-of-range code: %#v", got)
	}
}

func TestParseType1EncodingStopsAtLength1(t *testing.T) {
	data := []byte("dup 65 /A put\n")
	stream := newStream(Dict{Name("Length1"): Number(5)}, data)
	d := &Document{}
	f := NewSimpleFont("Type1")
	f.applyEncoding("StandardEncoding")
	applyType1Encoding(d, f, Dict{Name("FontFile"): stream})
	f.ensureType1Encoding()
	if _, ok := f.glyphNames[65]; ok {
		t.Fatalf("encoding parsed past Length1: %#v", f.glyphNames)
	}
}

func TestApplyType1EncodingMapsGlyphText(t *testing.T) {
	stream := newStream(Dict{Name("Length1"): Number(18)}, []byte("dup 65 /Aacute put"))
	d := &Document{}
	f := NewSimpleFont("Type1")
	applyType1Encoding(d, f, Dict{Name("FontFile"): stream})
	if f.type1Parsed {
		t.Fatal("Type1 encoding parsed during font construction")
	}
	f.ensureType1Encoding()
	if f.glyphNames[65] != "Aacute" || f.glyphTexts[65] != "Á" {
		t.Fatalf("Type1 glyph mapping = names %#v texts %#v", f.glyphNames, f.glyphTexts)
	}
}

func TestApplyType1EncodingOverridesImplicitCodeMapping(t *testing.T) {
	stream := newStream(nil, []byte("dup 65 /privateGlyph put\ndup 66 /B put"))
	d := &Document{}
	f := NewSimpleFont("Type1")
	f.applyEncoding("StandardEncoding")
	applyType1Encoding(d, f, Dict{Name("FontFile"): stream})
	if f.type1Parsed {
		t.Fatal("Type1 encoding parsed during font construction")
	}
	got := f.DecodeGlyphs([]byte{65})
	if len(got) != 1 || got[0].Text() != "" {
		t.Fatalf("Type1 custom encoding text = %#v", got)
	}
}

func TestType1PublicQueriesMaterializeEncoding(t *testing.T) {
	stream := newStream(nil, []byte("dup 65 /Aacute put"))
	d := &Document{}
	f := NewSimpleFont("Type1")
	applyType1Encoding(d, f, Dict{Name("FontFile"): stream})
	if _, ok := f.GlyphName(65); !ok {
		t.Fatal("GlyphName did not materialize Type1 encoding")
	}
	if _, ok := f.EncodingValue(65); ok {
		t.Fatal("EncodingValue retained the replaced Type1 mapping")
	}
}

func TestFailedType1EncodingReleasesInputBuffers(t *testing.T) {
	f := NewSimpleFont("Type1")
	f.type1Data = []byte("type1")
	f.type1Filters = []string{"UnsupportedFilter"}
	f.ensureType1Encoding()
	if !f.type1Parsed || f.type1Data != nil || f.type1Filters != nil || f.type1Parms != nil {
		t.Fatalf("failed Type1 parser retained input buffers: parsed=%v data=%d filters=%v parms=%v", f.type1Parsed, len(f.type1Data), f.type1Filters, f.type1Parms)
	}
}

func TestApplyType1EncodingFollowsIndirectFontFileAndLength(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: newStream(Dict{Name("Length1"): Ref{Object: 3}}, []byte("dup 65 /Aacute put")),
		{Object: 3}: Ref{Object: 4},
		{Object: 4}: Number(18),
	}}
	f := NewSimpleFont("Type1")
	applyType1Encoding(d, f, Dict{Name("FontFile"): Ref{Object: 1}})
	if len(f.type1Data) == 0 || f.type1Length1 != 18 || f.type1Parsed {
		t.Fatalf("indirect Type1 program was not retained lazily: %#v", f)
	}
	f.ensureType1Encoding()
	if f.glyphNames[65] != "Aacute" {
		t.Fatalf("indirect Type1 encoding = %#v", f.glyphNames)
	}
}

func TestType1EncodingReadsEexecAssignments(t *testing.T) {
	cleartext := []byte("%!PS-AdobeFont-1.0\neexec\n")
	plain := append([]byte{0, 0, 0, 0}, []byte("/Encoding 256 array\ndup 65 /A put\n")...)
	encrypted := encryptType1Eexec(plain)
	f := NewSimpleFont("Type1")
	f.type1Data = append(append([]byte(nil), cleartext...), encrypted...)
	f.type1Length1 = len(cleartext)
	f.ensureType1Encoding()
	if f.glyphNames[65] != "A" {
		t.Fatalf("eexec Type1 encoding = %#v", f.glyphNames)
	}
}

func TestType1EncodingReadsHexEexecAssignments(t *testing.T) {
	cleartext := []byte("%!PS-AdobeFont-1.0\neexec\n")
	plain := append([]byte{0, 0, 0, 0}, []byte("dup 66 /B put\n")...)
	encoded := []byte(hex.EncodeToString(encryptType1Eexec(plain)))
	f := NewSimpleFont("Type1")
	f.type1Data = append(append([]byte(nil), cleartext...), encoded...)
	f.type1Length1 = len(cleartext)
	f.ensureType1Encoding()
	if f.glyphNames[66] != "B" {
		t.Fatalf("hex eexec Type1 encoding = %#v", f.glyphNames)
	}
}

func TestType1EncodingDoesNotDecryptWithoutEexecMarker(t *testing.T) {
	cleartext := []byte("%!PS-AdobeFont-1.0\n")
	plain := append([]byte{0, 0, 0, 0}, []byte("dup 67 /C put\n")...)
	f := NewSimpleFont("Type1")
	f.type1Data = append(append([]byte(nil), cleartext...), encryptType1Eexec(plain)...)
	f.type1Length1 = len(cleartext)
	f.ensureType1Encoding()
	if _, ok := f.glyphNames[67]; ok {
		t.Fatalf("Type1 decoded data without eexec marker: %#v", f.glyphNames)
	}
}

func encryptType1Eexec(plain []byte) []byte {
	const c1, c2 = uint32(52845), uint32(22719)
	r := uint32(55665)
	out := make([]byte, len(plain))
	for i, value := range plain {
		cipher := uint32(value) ^ (r >> 8)
		out[i] = byte(cipher)
		r = (cipher+r)*c1 + c2
		r &= 0xffff
	}
	return out
}

func TestType1ImplicitEncodingKeepsPublishedSnapshots(t *testing.T) {
	f := NewSimpleFont("Type1")
	f.applyEncoding("StandardEncoding")
	applyType1Encoding(&Document{}, f, Dict{"FontFile": newStream(nil, []byte("dup 66 /B put"))})
	before := f.decodeSnapshot()
	f.lazyMutex().Lock()
	beforeWidths := f.publishWidthStateLocked()
	f.lazyMutex().Unlock()
	f.ensureType1Encoding()
	if before.encoding[65] != 'A' || beforeWidths.encoding[65] != 'A' {
		t.Fatal("Type1 decoding changed a published encoding map")
	}
	if got := f.Decode([]byte("AB")); got != "B" {
		t.Fatalf("decode=%q want B", got)
	}
	if f.widthStateLoad() != nil {
		t.Fatal("stale width snapshot remained published")
	}
}

func TestType1EmptyToUnicodeSuppressesEncodingFallback(t *testing.T) {
	for _, materializeFirst := range []bool{false, true} {
		f := NewSimpleFont("Type1")
		f.toUnicodeData = []byte{}
		applyType1Encoding(&Document{}, f, Dict{"FontFile": newStream(nil, []byte("dup 115 /radicalBigg put"))})
		if materializeFirst {
			f.ensureToUnicode()
		}
		f.ensureType1Encoding()
		if len(f.glyphTexts) != 0 {
			t.Fatalf("empty ToUnicode triggered StandardEncoding fallback: %#v", f.glyphTexts)
		}
		if f.glyphNames[115] != "radicalBigg" {
			t.Fatal("outline name lost")
		}
	}
}
