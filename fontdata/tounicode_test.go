package fontdata

import "testing"

func TestParseToUnicodeMapPreservesCodespacesAndDecodesByLength(t *testing.T) {
	data := []byte(`begincmap
1 begincodespacerange
<40> <ff>
<0100> <01ff>
endcodespacerange
2 beginbfchar
<41> <0041>
<0101> <4e2d>
endbfchar
endcmap`)
	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatal(err)
	}
	spaces := toUnicode.CodespacesCopy()
	if len(spaces) != 2 || len(spaces[0].LowCopy()) != 1 || len(spaces[1].LowCopy()) != 2 {
		t.Fatalf("codespaces = %#v", spaces)
	}
	got := toUnicode.Decode([]byte{0x41, 0x01, 0x01})
	if got != "A中" {
		t.Fatalf("decoded ToUnicode = %q, want %q", got, "A中")
	}
}

func TestParseToUnicodeAcceptsNumericBFRangeTargetsLikePlaya(t *testing.T) {
	data := []byte(`begincmap
1 begincodespacerange
<00> <ff>
endcodespacerange
1 beginbfrange
<41> <41> 66
endbfrange
endcmap`)
	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := toUnicode.Decode([]byte{'A'}); got != "B" {
		t.Fatalf("numeric bfrange target = %q, want %q", got, "B")
	}
}

func TestToUnicodeWithoutCodespacesUsesSingleByteFallback(t *testing.T) {
	data := []byte("1 beginbfchar\n<8140> <4e2d>\nendbfchar\n")
	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := toUnicode.Decode([]byte{0x81, 0x40}); got != string(rune(0x81))+"@" {
		t.Fatalf("ToUnicode without codespaces = %q, want byte-wise fallback", got)
	}
}

func TestToUnicodeWithoutCodespacesIgnoresSingleByteMappings(t *testing.T) {
	data := []byte("2 beginbfchar\n<41> <4e2d> <42> <4e3a>\nendbfchar\n")
	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := toUnicode.Decode([]byte{'A', 'B'}); got != "AB" {
		t.Fatalf("ToUnicode without codespaces used mappings = %q, want byte fallback", got)
	}
}

func TestToUnicodeMapUsesPlayaCodeSpaceOrderForOverlappingLengths(t *testing.T) {
	data := []byte(`begincmap
2 begincodespacerange
<01> <01>
<0100> <01ff>
endcodespacerange
2 beginbfchar
<01> <0041>
<0101> <4e2d>
endbfchar
endcmap`)
	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := toUnicode.Decode([]byte{1, 1}); got != "AA" {
		t.Fatalf("overlapping ToUnicode codespaces = %q, want %q", got, "AA")
	}
}

func TestParseToUnicodeStopsAtEndCMap(t *testing.T) {
	data := []byte(`begincmap
1 begincodespacerange
<00> <ff>
endcodespacerange
1 beginbfchar
<01> <0041>
endbfchar
endcmap
1 begincodespacerange
<0100> <01ff>
endcodespacerange
1 beginbfchar
<0102> <0042>
endbfchar`)

	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := toUnicode.Decode([]byte{1, 2}); got != "A\x02" {
		t.Fatalf("ToUnicode parsed data after endcmap = %q, want %q", got, "A\x02")
	}
	if _, ok := toUnicode.Lookup([]byte{2}); ok {
		t.Fatal("ToUnicode retained a mapping after endcmap")
	}
	if spaces := toUnicode.CodespacesCopy(); len(spaces) != 1 || string(spaces[0].LowCopy()) != "\x00" {
		t.Fatalf("ToUnicode retained codespaces after endcmap: %#v", spaces)
	}
}

func TestParseToUnicodeIgnoresNotDefRanges(t *testing.T) {
	data := []byte(`begincmap
1 begincodespacerange
<00> <ff>
endcodespacerange
1 beginnotdefrange
<01> <01> <0041>
endnotdefrange
endcmap`)

	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := toUnicode.Lookup([]byte{1}); ok {
		t.Fatal("ToUnicode retained a beginnotdefrange entry")
	}
	if got := toUnicode.Decode([]byte{1}); got != "\x01" {
		t.Fatalf("beginnotdefrange changed fallback text = %q, want %q", got, "\x01")
	}
}

func TestParseToUnicodeMapRejectsMalformedCodespaceToken(t *testing.T) {
	data := []byte("1 begincodespacerange\n<nothex> <ff>\nendcodespacerange")
	if _, err := ParseToUnicodeMap(data); err == nil {
		t.Fatal("malformed ToUnicode codespace was accepted")
	}
}

func TestParseToUnicodeMapIgnoresMismatchedCodespaceLikePlaya(t *testing.T) {
	data := []byte("1 begincodespacerange\n<00> <0001>\nendcodespacerange\n1 beginbfchar\n<41> <0041>\nendbfchar")
	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatalf("mismatched ToUnicode codespace returned error: %v", err)
	}
	if spaces := toUnicode.CodespacesCopy(); len(spaces) != 0 {
		t.Fatalf("mismatched ToUnicode codespace was retained: %#v", spaces)
	}
	if got := toUnicode.Decode([]byte{0x41}); got != "A" {
		t.Fatalf("fallback after mismatched codespace = %q, want %q", got, "A")
	}
}

func TestParseToUnicodeMapIgnoresReversedCodespaceLikePlaya(t *testing.T) {
	data := []byte("1 begincodespacerange\n<ff> <00>\nendcodespacerange\n1 beginbfchar\n<41> <0041>\nendbfchar")
	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatalf("reversed ToUnicode codespace returned error: %v", err)
	}
	if spaces := toUnicode.CodespacesCopy(); len(spaces) != 0 {
		t.Fatalf("reversed ToUnicode codespace was retained: %#v", spaces)
	}
}

func TestParseToUnicodeMapIgnoresIncompleteCodespaceSuffixLikePlaya(t *testing.T) {
	data := []byte("1 begincodespacerange\n<00> <ff>\n<01>\nendcodespacerange\n1 beginbfchar\n<41> <0041>\n<42>\nendbfchar")
	toUnicode, err := ParseToUnicodeMap(data)
	if err != nil {
		t.Fatalf("ParseToUnicodeMap() error = %v", err)
	}
	if got := toUnicode.Decode([]byte{0x41}); got != "A" {
		t.Fatalf("decoded ToUnicode = %q, want %q", got, "A")
	}
	if spaces := toUnicode.CodespacesCopy(); len(spaces) != 1 || string(spaces[0].LowCopy()) != "\x00" {
		t.Fatalf("codespaces = %#v, want only the complete prefix", spaces)
	}
}

func TestToUnicodeMapDecodeSeqIsRepeatableAndStopsEarly(t *testing.T) {
	toUnicode, err := ParseToUnicodeMap([]byte(`1 begincodespacerange
<00> <ff>
endcodespacerange
2 beginbfchar
<41> <0041>
<42> <0042>
endbfchar`))
	if err != nil {
		t.Fatal(err)
	}
	var first []string
	for value := range toUnicode.DecodeSeq([]byte{0x41, 0x42}) {
		first = append(first, value)
		break
	}
	if len(first) != 1 || first[0] != "A" {
		t.Fatalf("early-stop decode = %#v", first)
	}
	var second []string
	for value := range toUnicode.DecodeSeq([]byte{0x41, 0x42}) {
		second = append(second, value)
	}
	if len(second) != 2 || second[0] != "A" || second[1] != "B" {
		t.Fatalf("repeatable decode = %#v", second)
	}
}

func TestParseToUnicodePreservesCodesAndFiltersWideProjection(t *testing.T) {
	data := []byte("2 beginbfchar\n<0001> <0041> <00010001> <0042>\nendbfchar")
	codes, err := ParseToUnicodeCodes(data)
	if err != nil || codes[string([]byte{0, 1})] != "A" || codes[string([]byte{0, 1, 0, 1})] != "B" {
		t.Fatalf("ToUnicode codes = %#v, err=%v", codes, err)
	}
	values, err := ParseToUnicode(data)
	if err != nil || values[1] != "A" || len(values) != 1 {
		t.Fatalf("ToUnicode values = %#v, err=%v", values, err)
	}
}

func TestParseToUnicodeHandlesRangesAndUTF16Surrogates(t *testing.T) {
	data := []byte("1 beginbfrange\n<0001> <0002> <0041>\nendbfrange\n1 beginbfchar\n<0003> <D83DDE00>\nendbfchar")
	values, err := ParseToUnicode(data)
	if err != nil || values[1] != "A" || values[2] != "B" || values[3] != "😀" {
		t.Fatalf("range/surrogate values = %#v, err=%v", values, err)
	}
}

func TestParseToUnicodeHandlesMultipleRangeRecordsOnOneLineLikePlaya(t *testing.T) {
	data := []byte(`
2 beginbfrange
<0001> <0002> <0041> <0003> <0004> <0043>
endbfrange
2 beginbfrange
<0005> <0006> [<0045> <0046>] <0007> <0008> [<0047> <0048>]
endbfrange
2 begincidrange
<0009> <000a> 73 <000b> <000c> 75
endcidrange`)

	codes, err := ParseToUnicodeCodes(data)
	if err != nil {
		t.Fatalf("ParseToUnicodeCodes returned error: %v", err)
	}
	want := map[string]string{
		string([]byte{0, 1}):  "A",
		string([]byte{0, 2}):  "B",
		string([]byte{0, 3}):  "C",
		string([]byte{0, 4}):  "D",
		string([]byte{0, 5}):  "E",
		string([]byte{0, 6}):  "F",
		string([]byte{0, 7}):  "G",
		string([]byte{0, 8}):  "H",
		string([]byte{0, 9}):  "I",
		string([]byte{0, 10}): "J",
		string([]byte{0, 11}): "K",
		string([]byte{0, 12}): "L",
	}
	if len(codes) != len(want) {
		t.Fatalf("ToUnicode codes = %#v, want %#v", codes, want)
	}
	for source, value := range want {
		if codes[source] != value {
			t.Fatalf("ToUnicode[%x] = %q, want %q", []byte(source), codes[source], value)
		}
	}
}

func TestParseToUnicodeBfRangeArrayUsesAvailableValuesLikePlaya(t *testing.T) {
	data := []byte("1 beginbfrange\n<0001> <0003> [<0041> <0042>]\nendbfrange")
	codes, err := ParseToUnicodeCodes(data)
	if err != nil {
		t.Fatalf("short ToUnicode bfrange array returned error: %v", err)
	}
	if got := codes[string([]byte{0, 1})]; got != "A" {
		t.Fatalf("ToUnicode[1] = %q, want %q", got, "A")
	}
	if got := codes[string([]byte{0, 2})]; got != "B" {
		t.Fatalf("ToUnicode[2] = %q, want %q", got, "B")
	}
	if _, ok := codes[string([]byte{0, 3})]; ok {
		t.Fatal("ToUnicode retained a value beyond the short bfrange array")
	}
}

func TestParseToUnicodeIgnoresIncompleteTrailingRecordsLikePlaya(t *testing.T) {
	data := []byte(`1 beginbfchar
<0001> <0041> <0002>
endbfchar
1 beginbfrange
<0003> <0004> <0043> <0005>
endbfrange
1 begincidchar
<0005> 69 <0006>
endcidchar
1 begincidrange
<0007> <0008> 71 <0009>
endcidrange`)

	codes, err := ParseToUnicodeCodes(data)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		string([]byte{0, 1}): "A",
		string([]byte{0, 3}): "C",
		string([]byte{0, 4}): "D",
		string([]byte{0, 5}): "E",
		string([]byte{0, 7}): "G",
		string([]byte{0, 8}): "H",
	}
	if len(codes) != len(want) {
		t.Fatalf("ToUnicode codes = %#v, want %#v", codes, want)
	}
	for source, value := range want {
		if codes[source] != value {
			t.Fatalf("ToUnicode[%x] = %q, want %q", []byte(source), codes[source], value)
		}
	}
}

func TestParseToUnicodeDropsUnpairedSurrogatesLikePlaya(t *testing.T) {
	data := []byte("3 beginbfchar\n<0001> <D800>\n<0002> <DC00>\n<0003> <D800D800>\nendbfchar")
	values, err := ParseToUnicode(data)
	if err != nil {
		t.Fatalf("ParseToUnicode returned error: %v", err)
	}
	for code, value := range values {
		if value != "" {
			t.Fatalf("ToUnicode[%d] = %q, want unpaired surrogates dropped", code, value)
		}
	}
}

func TestParseToUnicodePadsOddHexTokensLikePlaya(t *testing.T) {
	data := []byte("1 beginbfchar\n<1> <0041>\nendbfchar\n2 beginbfrange\n<2> <3> <0042>\nendbfrange\n1 begincidchar\n<3> 65\nendcidchar")
	codes, err := ParseToUnicodeCodes(data)
	if err != nil {
		t.Fatalf("ParseToUnicodeCodes returned error: %v", err)
	}
	if got := codes[string([]byte{0x10})]; got != "A" {
		t.Fatalf("ToUnicode[0x10] = %q, want %q", got, "A")
	}
	if got := codes[string([]byte{0x20})]; got != "B" {
		t.Fatalf("ToUnicode[0x20] = %q, want %q", got, "B")
	}
	if got := codes[string([]byte{0x21})]; got != "C" {
		t.Fatalf("ToUnicode[0x21] = %q, want %q", got, "C")
	}
	if got := codes[string([]byte{0x30})]; got != "A" {
		t.Fatalf("ToUnicode[0x30] = %q, want %q", got, "A")
	}
}

func TestParseToUnicodeRejectsMalformedAndOverflowingRecords(t *testing.T) {
	for _, data := range []string{
		"1 beginbfchar\n<0000000001> <0041>\nendbfchar",
		"1 begincidchar\n<0000000001> 65\nendcidchar",
		"1 beginbfrange\n<0002> <0001> <0041>\nendbfrange",
		"1 beginbfrange\n<0001> <0002> <FFFF>\nendbfrange",
		"1 beginbfchar\n<0001> <0041FF>\nendbfchar",
	} {
		if _, err := ParseToUnicodeCodes([]byte(data)); err == nil {
			t.Fatalf("malformed ToUnicode was accepted: %q", data)
		}
	}
}
