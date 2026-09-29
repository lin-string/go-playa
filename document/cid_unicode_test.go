package document

import (
	"sync"
	"testing"
)

func TestCIDToUnicodeUsesSourceCodeBeforeCID(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.toUnicode = map[uint16]string{}
	f.cmap = testCMap([]CodeSpace{testCodeSpace([]byte{0x81, 0x40}, []byte{0x81, 0x40})}, map[string]int{string([]byte{0x81, 0x40}): 5}, false)
	f.toUnicode[0x8140] = "文"
	got := f.DecodeGlyphs([]byte{0x81, 0x40})
	if len(got) != 1 || got[0].Text() != "文" || got[0].CID() != 5 {
		t.Fatalf("decoded glyphs = %#v", got)
	}
}

func TestPredefinedCIDUnicodeSupportsAdobeKRAndMangaCollections(t *testing.T) {
	for _, ordering := range []string{"Japan2", "KR", "Manga1"} {
		mapping := predefinedCIDUnicode(ordering)
		if len(mapping) == 0 {
			t.Fatalf("%s CID Unicode mapping is empty", ordering)
		}
	}
	kr := predefinedCIDUnicode("KR")
	if kr[1] == "" {
		t.Fatalf("KR CID 1 has no Unicode fallback")
	}
	manga := predefinedCIDUnicode("Manga1")
	if manga[1] == "" {
		t.Fatalf("Manga1 CID 1 has no Unicode fallback")
	}
}

func TestPredefinedCIDUnicodeSupportsConcurrentBuilds(t *testing.T) {
	const callers = 8
	results := make([]map[int]string, callers)
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := range results {
		go func(i int) {
			defer wg.Done()
			results[i] = predefinedCIDUnicode("Japan1")
		}(i)
	}
	wg.Wait()
	for i, mapping := range results {
		if len(mapping) == 0 || mapping[2980] != "中" {
			t.Fatalf("concurrent mapping %d = %#v", i, mapping)
		}
	}
	results[0][2980] = "mutated"
	if got := predefinedCIDUnicode("Japan1")[2980]; got != "中" {
		t.Fatalf("cached CID Unicode mapping was exposed: %q", got)
	}
}

func TestCIDIdentityFallbackUsesSourceCode(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cmap = testCMap([]CodeSpace{testCodeSpace([]byte{0x81, 0x40}, []byte{0x81, 0x40})}, map[string]int{string([]byte{0x81, 0x40}): 5}, false)
	got := f.DecodeGlyphs([]byte{0x81, 0x40})
	if len(got) != 1 || got[0].Text() != string(rune(0x8140)) {
		t.Fatalf("identity fallback = %#v", got)
	}
}

func TestCIDEmbeddedNULUnicodeIsPreserved(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cmap = testCMap([]CodeSpace{testCodeSpace([]byte{0, 0}, []byte{0xff, 0xff})}, map[string]int{string([]byte{0, 1}): 1}, false)
	f.glyphIDToUnicode = map[int]string{1: "\x00"}
	got := f.DecodeGlyphs([]byte{0, 1})
	if len(got) != 1 || got[0].Text() != "\x00" {
		t.Fatalf("embedded NUL Unicode = %#v, want U+0000", got)
	}
}

func TestCIDEmbeddedNULUnicodeMatchesPlaya(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cmap = testCMap([]CodeSpace{testCodeSpace([]byte{0, 0}, []byte{0xff, 0xff})}, map[string]int{string([]byte{0, 1}): 1}, false)
	f.glyphIDToUnicode = map[int]string{1: "\x00"}
	got := f.DecodeGlyphs([]byte{0, 1})
	if len(got) != 1 || got[0].Text() != "\x00" {
		t.Fatalf("embedded NUL Unicode = %#v, want Playa-compatible NUL", got)
	}
}

func TestCIDAdobeCollectionFallbackUsesCIDUnicode(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cmap, _ = loadPredefinedCMap("UniJIS-UCS2-H")
	f.cidToUnicode = predefinedCIDUnicode("Japan1")
	got := f.DecodeGlyphs([]byte{0x4e, 0x2d})
	if len(got) != 1 || got[0].CID() != 2980 || got[0].Text() != "中" {
		t.Fatalf("Adobe CID fallback = %#v", got)
	}
}

func TestCIDAdobeCollectionFallbackCountsAsUnicodeMapping(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cidToUnicode = map[int]string{2980: "中"}
	if !f.hasUnicodeMapping([]byte{0x4e, 0x2d}, 2980) {
		t.Fatal("Adobe CID fallback was reported as unmapped")
	}
}

func TestCIDAdobeCollectionFallbackIncludesSupplementaryUnicode(t *testing.T) {
	got := predefinedCIDUnicode("Japan1")[12269]
	if got != string(rune(0x1b132)) {
		t.Fatalf("supplementary CID fallback = %q, want U+1B132", got)
	}
}

func TestCMapUnicodeCodePointUsesDeclaredEncoding(t *testing.T) {
	for _, test := range []struct {
		name string
		code []byte
		want rune
	}{
		{name: "UniJIS-UTF8-H", code: []byte{0xe4, 0xb8, 0xad}, want: '中'},
		{name: "UniJIS-UTF16-H", code: []byte{0xd8, 0x2c, 0xdd, 0x32}, want: 0x1b132},
		{name: "UniJIS-UTF32-H", code: []byte{0x00, 0x01, 0xb1, 0x32}, want: 0x1b132},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := cmapUnicodeCodePoint(test.name, test.code)
			if !ok || got != uint32(test.want) {
				t.Fatalf("Unicode code point = U+%X, ok=%v; want U+%X", got, ok, test.want)
			}
		})
	}
}

func TestCMapUnicodeCodePointPreservesUTF16ReplacementCharacter(t *testing.T) {
	got, ok := cmapUnicodeCodePoint("UniJIS-UTF16-H", []byte{0xff, 0xfd})
	if !ok || got != 0xfffd {
		t.Fatalf("UTF-16 replacement code point = U+%X, ok=%v", got, ok)
	}
}

func TestCMapUnicodeCodePointRejectsUTF16UnpairedSurrogate(t *testing.T) {
	if got, ok := cmapUnicodeCodePoint("UniJIS-UTF16-H", []byte{0xd8, 0x00}); ok {
		t.Fatalf("unpaired UTF-16 surrogate mapped to U+%X", got)
	}
}

func TestCIDToGIDMapSelectsEmbeddedGlyphUnicode(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cmap = testCMap([]CodeSpace{testCodeSpace([]byte{0, 0}, []byte{0xff, 0xff})}, map[string]int{string([]byte{0, 1}): 5}, false)
	f.cidToGID = map[int]int{5: 7}
	f.glyphIDToUnicode = map[int]string{7: "A"}
	got := f.DecodeGlyphs([]byte{0, 1})
	if len(got) != 1 || got[0].CID() != 5 || got[0].Text() != "A" {
		t.Fatalf("CIDToGID glyph selection = %#v", got)
	}
}
