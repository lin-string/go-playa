package document

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestType1CharStringOpsBuildsBasicOutline(t *testing.T) {
	// 139,139 = 0; 139,239 = 100; 21 = rmoveto; 5 = rlineto;
	// 14 = endchar.
	ops, ok := type1CharStringOps([]byte{139, 139, 21, 239, 139, 5, 139, 239, 5, 14}, nil)
	if !ok {
		t.Fatal("Type1 CharString was rejected")
	}
	if len(ops) != 5 || ops[0].operatorValue() != "m" || ops[1].operatorValue() != "l" || ops[2].operatorValue() != "l" {
		t.Fatalf("Type1 path operators = %#v", ops)
	}
	if ops[len(ops)-2].operatorValue() != "h" || ops[len(ops)-1].operatorValue() != "f" {
		t.Fatalf("Type1 outline was not closed and filled: %#v", ops)
	}
}

func TestType1CharStringOpsDivPreservesEarlierOperands(t *testing.T) {
	// 10 20 2 div leaves 10 and 10 on the operand stack for rlineto.
	data := []byte{139, 139, 21, 149, 159, 141, 12, 12, 5, 14}
	ops, ok := type1CharStringOps(data, nil)
	if !ok || len(ops) != 4 || ops[1].operatorValue() != "l" {
		t.Fatalf("Type1 div operators = %#v, ok=%v", ops, ok)
	}
	x, _ := NumberValue(ops[1].operandsValue()[0])
	y, _ := NumberValue(ops[1].operandsValue()[1])
	if x != 10 || y != 10 {
		t.Fatalf("Type1 div endpoint = (%v, %v)", x, y)
	}
}

func TestType1CharStringOpsCallsSubroutine(t *testing.T) {
	// The subroutine draws a horizontal line and returns.
	ops, ok := type1CharStringOps([]byte{139, 139, 21, 139, 10, 14}, [][]byte{{239, 139, 5, 11}})
	if !ok || len(ops) < 3 || ops[1].operatorValue() != "l" {
		t.Fatalf("Type1 subroutine operators = %#v, ok=%v", ops, ok)
	}
}

func TestType1CharStringOpsRejectsMalformedInput(t *testing.T) {
	if _, ok := type1CharStringOps([]byte{139, 12, 6}, nil); ok {
		t.Fatal("Type1 seac without a glyph resolver was accepted")
	}
	if _, ok := type1CharStringOps([]byte{139, 10}, [][]byte{{139, 10}}); ok {
		t.Fatal("recursive Type1 subroutine was accepted")
	}
	// 1.5 must not be truncated to Subr index 1.
	if _, ok := type1CharStringOps([]byte{255, 0, 1, 128, 0, 10, 14}, [][]byte{{139, 11}, {139, 11}}); ok {
		t.Fatal("fractional Type1 subroutine index was accepted")
	}
	if _, ok := type1CharStringOps([]byte{11}, nil); ok {
		t.Fatal("top-level Type1 return was accepted")
	}
	if _, ok := type1CharStringOps([]byte{139, 10, 14}, [][]byte{{14}}); ok {
		t.Fatal("Type1 endchar inside a subroutine was accepted")
	}
	if _, ok := type1CharStringOps([]byte{139, 139, 21}, nil); ok {
		t.Fatal("truncated Type1 glyph was accepted")
	}
	if _, ok := type1CharStringOps([]byte{139, 10, 14}, [][]byte{{139}}); ok {
		t.Fatal("truncated Type1 subroutine was accepted")
	}
	subrs := make([][]byte, type1CharStringMaxDepth+1)
	for i := range subrs {
		if i == len(subrs)-1 {
			subrs[i] = []byte{11}
			continue
		}
		subrs[i] = []byte{byte(139 + i + 1), 10, 11}
	}
	if _, ok := type1CharStringOps([]byte{139, 10, 14}, subrs); ok {
		t.Fatal("Type1 Subr depth limit was not enforced")
	}
}

func TestType1CharStringOpsExpandsSeac(t *testing.T) {
	charstrings := map[byte][]byte{
		65: {139, 139, 21, 239, 139, 5, 14},
		66: {139, 139, 21, 149, 139, 5, 14},
	}
	// seac operands are asb, adx, ady, bchar, achar. The accent x origin
	// is adx-asb rather than adx alone.
	data := []byte{144, 149, 159, 204, 205, 12, 6, 14}
	ops, ok := type1CharStringOpsWithSeac(data, nil, func(code byte) []byte { return charstrings[code] })
	if !ok || len(ops) < 4 {
		t.Fatalf("Type1 seac operators = %#v, ok=%v", ops, ok)
	}
	if x, ok := NumberValue(ops[3].operandsValue()[0]); !ok || x != 15 {
		t.Fatalf("translated Type1 accent x = %v, ok=%v", x, ok)
	}
}

func TestType1CharStringOpsRejectsMalformedSeacComponent(t *testing.T) {
	data := []byte{144, 149, 159, 204, 205, 12, 6, 14}
	_, ok := type1CharStringOpsWithSeac(data, nil, func(code byte) []byte {
		if code == 65 {
			return []byte{140, 140, 142, 12, 16, 14} // unpopped OtherSubr result
		}
		return []byte{139, 139, 21, 14}
	})
	if ok {
		t.Fatal("malformed Type1 seac component was accepted")
	}
}

func TestType1CharStringOpsSupportsFlexVariants(t *testing.T) {
	flex := make([]byte, 13)
	for i := range flex {
		flex[i] = 239
	}
	flex = append(flex, 12, 35, 14)
	ops, ok := type1CharStringOps(flex, nil)
	if !ok || len(ops) != 4 || ops[0].operatorValue() != "c" || ops[1].operatorValue() != "c" {
		t.Fatalf("Type1 flex operators = %#v, ok=%v", ops, ok)
	}

	hflex := []byte{239, 239, 239, 239, 239, 239, 239, 12, 34, 14}
	ops, ok = type1CharStringOps(hflex, nil)
	if !ok || len(ops) != 4 || ops[0].operatorValue() != "c" || ops[1].operatorValue() != "c" {
		t.Fatalf("Type1 hflex operators = %#v, ok=%v", ops, ok)
	}

	hflex1 := []byte{239, 141, 239, 142, 239, 179, 189, 199, 143, 12, 36, 14}
	ops, ok = type1CharStringOps(hflex1, nil)
	if !ok || len(ops) != 4 || ops[0].operatorValue() != "c" || ops[1].operatorValue() != "c" {
		t.Fatalf("Type1 hflex1 operators = %#v, ok=%v", ops, ok)
	}

	flex1 := []byte{239, 141, 239, 142, 239, 169, 143, 179, 144, 189, 145, 12, 37, 14}
	ops, ok = type1CharStringOps(flex1, nil)
	if !ok || len(ops) != 4 || ops[0].operatorValue() != "c" || ops[1].operatorValue() != "c" {
		t.Fatalf("Type1 flex1 operators = %#v, ok=%v", ops, ok)
	}
}

func TestType1CharStringOpsConsumesHintVariants(t *testing.T) {
	data := []byte{
		239, 139, 1, // hstem
		239, 139, 239, 139, 239, 139, 12, 2, // hstem3
		239, 139, 239, 139, 239, 139, 12, 1, // vstem3
		12, 0, // dotsection
		14,
	}
	ops, ok := type1CharStringOps(data, nil)
	if !ok || len(ops) != 0 {
		t.Fatalf("Type1 hint operators = %#v, ok=%v", ops, ok)
	}
}

func TestType1CharStringOpsExpandsOtherSubrFlex(t *testing.T) {
	data := type1OtherSubrFlexCharString()
	ops, ok := type1CharStringOps(data, nil)
	if !ok || len(ops) != 5 || ops[1].operatorValue() != "c" || ops[2].operatorValue() != "c" {
		t.Fatalf("Type1 OtherSubr Flex operators = %#v, ok=%v", ops, ok)
	}
}

func TestType1GlyphPathsSequenceUsesOtherSubrFlex(t *testing.T) {
	font := NewSimpleFont("Type1")
	font.fontType = "Type1"
	font.glyphNames[65] = "A"
	font.type1Charstrings = map[string][]byte{"A": type1OtherSubrFlexCharString()}
	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())
	count := 0
	for path, err := range glyph.PathsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		count += len(path.RawSegmentsCopy())
	}
	if count != 4 {
		t.Fatalf("Type1 OtherSubr Flex path segments = %d, want 4", count)
	}
}

func type1OtherSubrFlexCharString() []byte {
	data := []byte{139, 139, 21, 139, 140, 12, 16} // rmoveto; 0 1 callothersubr
	for _, point := range [][2]byte{{149, 139}, {159, 149}, {169, 139}, {149, 139}, {159, 129}, {169, 139}} {
		data = append(data, point[0], point[1], 21, 139, 141, 12, 16) // rmoveto; 0 2 callothersubr
	}
	return append(data, 139, 139, 139, 142, 139, 12, 16, 12, 17, 12, 17, 12, 33, 14) // standard Flex result transfer
}

func TestType1CharStringOpsConsumesCounterControlOtherSubrs(t *testing.T) {
	data := []byte{
		239, 149, 141, 151, 12, 16, // 10 10 2 12 callothersubr
		239, 149, 141, 152, 12, 16, // 10 10 2 13 callothersubr
		14,
	}
	if ops, ok := type1CharStringOps(data, nil); !ok || len(ops) != 0 {
		t.Fatalf("Type1 counter-control OtherSubrs = %#v, ok=%v", ops, ok)
	}
}

func TestType1CharStringOpsConsumesOtherSubrResultPop(t *testing.T) {
	data := []byte{140, 140, 142, 12, 16, 12, 17, 14} // 1 1 3 callothersubr pop
	if ops, ok := type1CharStringOps(data, nil); !ok || len(ops) != 0 {
		t.Fatalf("Type1 OtherSubr result pop = %#v, ok=%v", ops, ok)
	}
	if _, ok := type1CharStringOps([]byte{140, 140, 142, 12, 16, 14}, nil); ok {
		t.Fatal("Type1 OtherSubr result left unpopped")
	}
}

func TestParseType1CharstringsDecryptsEmbeddedGlyph(t *testing.T) {
	plain := []byte{139, 139, 21, 14}
	encrypted := encryptType1Charstring(plain, 4)
	data := append([]byte("/CharStrings 1 dict begin /A 8 RD "), encrypted...)
	data = append(data, []byte(" ND end\n")...)
	program := fontdata.ParseType1Program(data)
	charstrings := program.CharStringsCopy()
	if string(charstrings["A"]) != string(plain) {
		t.Fatalf("decoded Type1 CharString = %v, want %v", charstrings["A"], plain)
	}
}

func TestParseType1CharstringsLoadsSubrsAndLenIV(t *testing.T) {
	subr := encryptType1Charstring([]byte{239, 139, 11}, 0)
	data := []byte("/lenIV 0 def /Subrs 1 array dup 0 3 RD ")
	data = append(data, subr...)
	data = append(data, []byte(" ND /CharStrings 1 dict begin /A 4 RD ")...)
	data = append(data, []byte{139, 139, 21, 14}...)
	data = append(data, []byte(" ND end")...)
	program := fontdata.ParseType1Program(data)
	subrs := program.SubrsCopy()
	if len(subrs) != 1 || string(subrs[0]) != string([]byte{239, 139, 11}) {
		t.Fatalf("Type1 subrs = %#v", subrs)
	}
}

func TestParseType1CharstringsBoundsSubrCount(t *testing.T) {
	data := []byte("/Subrs 2147483647 array")
	if subrs := fontdata.ParseType1Program(data).SubrsCopy(); subrs != nil {
		t.Fatalf("oversized Type1 Subrs table was allocated: %d", len(subrs))
	}
}

func TestParseType1CharstringsRejectsOversizedEntryLength(t *testing.T) {
	data := []byte("/Subrs 1 array dup 0 2147483647 RD /CharStrings 0 dict end")
	if subrs := fontdata.ParseType1Program(data).SubrsCopy(); subrs == nil || subrs[0] != nil {
		t.Fatalf("oversized Type1 Subr entry was accepted: %#v", subrs)
	}
	data = []byte("/CharStrings 1 dict begin /A 2147483647 RD ")
	charstrings := fontdata.ParseType1Program(data).CharStringsCopy()
	if len(charstrings) != 0 {
		t.Fatalf("oversized Type1 CharString entry was accepted: %#v", charstrings)
	}
}

func TestType1GlyphPathsSequenceUsesCharString(t *testing.T) {
	font := NewSimpleFont("Type1")
	font.fontType = "Type1"
	font.glyphNames[65] = "A"
	font.type1Charstrings = map[string][]byte{"A": {139, 139, 21, 239, 139, 5, 14}}
	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())
	var paths []PathObject
	for path, err := range glyph.PathsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	if len(paths) != 1 || len(paths[0].RawSegmentsCopy()) == 0 {
		t.Fatalf("Type1 glyph paths = %#v", paths)
	}
}

func TestType1GlyphPathsSequenceReportsMalformedCharString(t *testing.T) {
	font := NewSimpleFont("Type1 malformed")
	font.fontType = "Type1"
	font.glyphNames[65] = "A"
	font.type1Charstrings = map[string][]byte{"A": {139}}
	glyph := newTestGlyphWithCode(font, []byte{65}, 0, identity())
	for _, err := range glyph.PathsSeq() {
		if err == nil {
			t.Fatal("malformed Type1 CharString produced a glyph without error")
		}
		return
	}
	t.Fatal("malformed Type1 CharString produced no error")
}

func TestType1GlyphPathOpsCachesDecodedOutline(t *testing.T) {
	font := NewSimpleFont("Type1")
	font.fontType = "Type1"
	font.glyphNames[65] = "A"
	font.type1Charstrings = map[string][]byte{"A": {139, 139, 21, 239, 139, 5, 14}}
	first, ok := font.type1GlyphPathOps([]byte{65})
	if !ok || len(font.type1PathCache) != 1 {
		t.Fatalf("first Type1 path = %#v, cache=%#v", first, font.type1PathCache)
	}
	second, ok := font.type1GlyphPathOps([]byte{65})
	if !ok || len(second) != len(first) {
		t.Fatalf("cached Type1 path = %#v, first=%#v", second, first)
	}
	if &first[0] == &second[0] {
		t.Fatal("cached Type1 path aliases returned slice")
	}
}

func TestType1LazyEexecDataFeedsGlyphPath(t *testing.T) {
	header := []byte("%!FontType1\n")
	body := []byte("/CharStrings 1 dict begin /A 8 RD ")
	body = append(body, encryptType1Charstring([]byte{139, 139, 21, 14}, 4)...)
	body = append(body, []byte(" ND end")...)
	plain := append([]byte{0, 0, 0, 0}, body...)
	data := append(append([]byte(nil), header...), encryptType1Eexec(plain)...)
	font := &Font{
		fontType:     "Type1",
		type1Data:    data,
		type1Length1: len(header),
		glyphNames:   map[byte]string{65: "A"},
		encoding:     map[byte]rune{},
		glyphTexts:   map[byte]string{},
	}
	ops, ok := font.type1GlyphPathOps([]byte{65})
	if !ok || len(ops) == 0 || len(font.type1Charstrings) != 1 {
		t.Fatalf("lazy Type1 eexec path = %#v, charstrings=%#v", ops, font.type1Charstrings)
	}
}

func encryptType1Charstring(plain []byte, lenIV int) []byte {
	data := append(make([]byte, lenIV), plain...)
	const c1, c2 = uint32(52845), uint32(22719)
	r := uint32(4330)
	for i, value := range data {
		cipher := value ^ byte(r>>8)
		data[i] = cipher
		r = ((uint32(cipher) + r) * c1) + c2
		r &= 0xffff
	}
	return data
}
