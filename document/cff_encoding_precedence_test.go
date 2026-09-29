package document

import (
	"reflect"
	"testing"
)

// Codes 76,77,78 select L,space,A. The glyphs have distinct widths and outlines.
func cffPrecedenceProgram() []byte {
	data := appendCFFIndex([]byte{1, 0, 4, 4}, []byte("Precedence"))
	topAt := len(data) + 5
	data = appendCFFIndex(data, []byte{0, 17, 0, 15, 0, 16})
	data = appendCFFIndex(data, []byte("L"), []byte("space"), []byte("A"))
	data = appendCFFIndex(data)
	charset := len(data)
	data = append(data, 0, 1, 135, 1, 136, 1, 137)
	enc := len(data)
	data = append(data, 0, 3, 76, 77, 78)
	chars := len(data)
	proc := func(width, size byte) []byte {
		return []byte{139 + width, 139, 139, 21, 139 + size, 139, 5, 139, 139 + size, 5, 14}
	}
	data = appendCFFIndex(data, []byte{14}, proc(50, 10), proc(60, 20), proc(70, 30))
	data[topAt] = byte(139 + chars)
	data[topAt+2] = byte(139 + charset)
	data[topAt+4] = byte(139 + enc)
	return data
}

func TestCFFEncodingPrecedenceStaysLazyAndOwnsSnapshots(t *testing.T) {
	tests := []struct {
		name     string
		encoding Object
		text     string
		gid      int
		width    float64
	}{
		{"explicit base differences", Dict{"BaseEncoding": Name("StandardEncoding"), "Differences": Array{Number(76), Name("space")}}, " MNA", 2, 60},
		{"implicit base differences", Dict{"Differences": Array{Number(76), Name("space")}}, "  A", 2, 60},
		{"explicit base", Name("StandardEncoding"), "LMNA", 1, 50},
		{"implicit base", nil, "L A", 1, 50},
		{"unknown difference", Dict{"Differences": Array{Number(76), Name("privateGlyph")}}, " A", 0, 0},
	}
	for _, subtype := range []Name{"Type1", "MMType1"} {
		for _, tc := range tests {
			t.Run(string(subtype)+"/"+tc.name, func(t *testing.T) {
				for _, finalizeFirst := range []bool{false, true} {
					d := &Document{}
					spec := Dict{"Subtype": subtype, "BaseFont": Name("Precedence"), "FontDescriptor": Dict{"FontFile3": newStream(Dict{"Subtype": Name("Type1C")}, cffPrecedenceProgram())}}
					if tc.encoding != nil {
						spec["Encoding"] = tc.encoding
					}
					font, err := d.GetFontWithError(0, spec)
					if err != nil {
						t.Fatal(err)
					}
					if font.cffParsed {
						t.Fatal("font construction eagerly parsed CFF")
					}
					prior := font.decodeSnapshot()
					priorEncoding := cloneByteRuneMap(prior.encoding)
					priorNames := cloneByteStringMap(prior.glyphNames)
					priorText := cloneByteStringMap(prior.glyphTexts)
					font.lazyMutex().Lock()
					priorWidths := font.publishWidthStateLocked()
					font.lazyMutex().Unlock()
					oldGIDs := cloneByteIntMap(priorWidths.cffGlyphIDs)
					oldWidths := cloneByteFloatMap(priorWidths.widths)
					original := font
					if finalizeFirst {
						font = font.Finalize()
					}
					for repeat := 0; repeat < 2; repeat++ {
						if got := font.Decode([]byte{76, 77, 78, 65}); got != tc.text {
							t.Errorf("decode=%q want%q (finalizeFirst=%v)", got, tc.text, finalizeFirst)
						}
						if font.cffErr != nil {
							t.Fatal(font.cffErr)
						}
						if got := font.glyphIDForCode([]byte{76}, 76); got != tc.gid {
							t.Errorf("GID=%d want%d", got, tc.gid)
						}
						if tc.gid != 0 {
							if got, err := font.WidthWithError(76); err != nil || got != tc.width {
								t.Errorf("width=%v,%v want%v", got, err, tc.width)
							}
						}
					}
					if !reflect.DeepEqual(prior.encoding, priorEncoding) || !reflect.DeepEqual(prior.glyphNames, priorNames) || !reflect.DeepEqual(prior.glyphTexts, priorText) {
						t.Error("lazy CFF parsing mutated a published decode snapshot")
					}
					if !reflect.DeepEqual(priorWidths.cffGlyphIDs, oldGIDs) || !reflect.DeepEqual(priorWidths.widths, oldWidths) {
						t.Error("lazy CFF parsing mutated a published width snapshot")
					}
					owned := font.Finalize()
					original.applyDifferences(Array{Number(76), Name("A")})
					if got := owned.Decode([]byte{76, 77, 78, 65}); got != tc.text {
						t.Errorf("finalized encoding changed with original: %q", got)
					}
				}
			})
		}
	}
}

func TestCFFDifferencesSelectOutlineGlyph(t *testing.T) {
	d := &Document{}
	fontSpec := Dict{"Subtype": Name("Type1"), "BaseFont": Name("Precedence"), "Encoding": Dict{"BaseEncoding": Name("StandardEncoding"), "Differences": Array{Number(76), Name("space")}}, "FontDescriptor": Dict{"FontFile3": newStream(Dict{"Subtype": Name("Type1C")}, cffPrecedenceProgram())}}
	page := Page{dict: Dict{"Resources": Dict{"Font": Dict{"F1": fontSpec}}, "Contents": newStream(nil, []byte("BT /F1 1000 Tf (L) Tj ET"))}}
	for repeat := 0; repeat < 2; repeat++ {
		glyphs := 0
		for glyph, err := range page.Glyphs(d) {
			if err != nil {
				t.Fatal(err)
			}
			glyphs++
			if glyph.GID() != 2 || glyph.Text() != " " {
				t.Errorf("glyph GID/text=%d/%q", glyph.GID(), glyph.Text())
			}
			for _, g := range []GlyphObject{glyph, glyph.Finalize()} {
				paths := 0
				for path, err := range g.PathsSeq() {
					if err != nil {
						t.Fatal(err)
					}
					paths++
					if got := path.BBox(); got != [4]float64{0, 0, 20, 20} {
						t.Errorf("outline bounds=%v want space glyph20x20", got)
					}
				}
				if paths != 1 {
					t.Fatalf("outline paths=%d", paths)
				}
			}
		}
		if glyphs != 1 {
			t.Fatalf("glyphs=%d", glyphs)
		}
	}
}

func TestCFFExplicitBaseUsesCanonicalGlyphIdentity(t *testing.T) {
	for _, base := range []string{"StandardEncoding", "WinAnsiEncoding", "MacRomanEncoding", "MacExpertEncoding"} {
		for _, canonicalPresent := range []bool{true, false} {
			t.Run(base+map[bool]string{true: "/canonical present", false: "/canonical absent"}[canonicalPresent], func(t *testing.T) {
				code := byte(65)
				name, alias, text := "A", "uni0041", "A"
				if base == "MacExpertEncoding" {
					code = 97
					name, alias, text = "Asmall", "uniF761", "\uf761"
				}
				font := NewSimpleFont("CFF aliases")
				font.applyEncoding(base)
				names := []string{".notdef", alias}
				want := 0
				font.glyphIDToUnicode = map[int]string{1: text}
				if canonicalPresent {
					names = append(names, name)
					want = 2
					font.glyphIDToUnicode[2] = text
				}
				applyCFFCodeNames(font, map[byte]string{code: alias}, names)
				if got := font.glyphIDForCode([]byte{code}, int(code)); got != want {
					t.Fatalf("base=%s glyph=%d want canonical %s GID%d", base, got, name, want)
				}
			})
		}
	}
}

func TestCFFMetadataPreservesPublishedGlyphTables(t *testing.T) {
	font := NewSimpleFont("CFF snapshot")
	font.cffData = cffPrecedenceProgram()
	font.glyphIDToUnicode = map[int]string{99: "prior"}
	font.glyphIDWidths = map[int]float64{99: 123}
	beforeDecode := font.decodeSnapshot()
	font.lazyMutex().Lock()
	beforeWidth := font.publishWidthStateLocked()
	font.lazyMutex().Unlock()
	if _, err := font.ensureCFFWithError(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeDecode.glyphIDToUnicode, map[int]string{99: "prior"}) || !reflect.DeepEqual(beforeWidth.glyphIDToUnicode, map[int]string{99: "prior"}) {
		t.Fatal("lazy metadata mutated published glyph text tables")
	}
	if !reflect.DeepEqual(beforeWidth.glyphIDWidths, map[int]float64{99: 123}) {
		t.Fatal("lazy metadata mutated published glyph width table")
	}
	if font.widthStateLoad() != nil {
		t.Fatal("stale width state remains published after CFF load")
	}
}

func TestCFFWithoutCharsetPreservesExistingGlyphMapping(t *testing.T) {
	font := NewSimpleFont("CFF2")
	font.glyphUnicodeToID = map[rune]int{'A': 9}
	applyCFFCodeNames(font, nil, nil)
	if got := font.glyphIDForCode([]byte{65}, 65); got != 9 {
		t.Fatalf("font without a CFF charset lost existing glyph mapping: %d", got)
	}
}
