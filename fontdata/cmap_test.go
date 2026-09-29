package fontdata

import (
	"io/fs"
	"strconv"
	"strings"
	"testing"
)

func TestPredefinedCMapRegistryCoversEmbeddedResources(t *testing.T) {
	entries, err := fs.ReadDir(CMapFiles, "cmapdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cmap") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".cmap")
		if _, ok := predefinedCMapNames[name]; !ok {
			t.Errorf("embedded CMap %q is missing from the runtime registry", name)
			continue
		}
		if _, err := LoadPredefinedCMap(name); err != nil {
			t.Errorf("embedded CMap %q is not loadable: %v", name, err)
		}
	}
}

func TestParseCMapAndDecodeCodespaces(t *testing.T) {
	cmap, err := ParseCMap([]byte(`/CIDInit /ProcSet findresource begin
12 dict begin begincmap
1 begincodespacerange
<00> <FF>
endcodespacerange
2 begincidchar
<41> 65
<42> 66
endcidchar
endcmap end end`))
	if err != nil {
		t.Fatal(err)
	}
	got := cmap.Decode([]byte{0x41, 0x42, 0x43})
	if len(got) != 3 || got[0].CID() != 65 || got[1].CID() != 66 || got[2].CID() != 0 {
		t.Fatalf("decoded codes: %#v", got)
	}
}

func TestParseCMapUsesCIDZeroForInvalidCodes(t *testing.T) {
	cmap, err := ParseCMap([]byte(`1 begincodespacerange
<41> <42>
endcodespacerange
1 begincidchar
<41> 65
endcidchar`))
	if err != nil {
		t.Fatal(err)
	}
	got := cmap.Decode([]byte{0x40, 0x41, 0x42, 0x43})
	if len(got) != 4 || got[0].CID() != 0 || got[1].CID() != 65 || got[2].CID() != 0 || got[3].CID() != 0 {
		t.Fatalf("invalid CMap codes = %#v, want zero CID fallback", got)
	}
}

func TestCMapDecodeDropsUnmatchedCodesWithoutCodespaces(t *testing.T) {
	cmap, err := ParseCMap([]byte("1 begincidchar\n<41> 65\nendcidchar"))
	if err != nil {
		t.Fatal(err)
	}
	got := cmap.Decode([]byte{0x40, 0x41, 0x42})
	if len(got) != 1 || got[0].CID() != 65 || string(got[0].BytesCopy()) != "A" {
		t.Fatalf("unmatched CMap codes = %#v", got)
	}
}

func TestCMapDecodeSeqIsRepeatableAndStopsEarly(t *testing.T) {
	cmap, err := ParseCMap([]byte(`2 begincodespacerange
<00> <7f>
<8000> <80ff>
endcodespacerange
2 begincidchar
<41> 65
<42> 66
endcidchar`))
	if err != nil {
		t.Fatal(err)
	}
	data := []byte{0x41, 0x42}
	var first []Code
	for code := range cmap.DecodeSeq(data) {
		first = append(first, code.Finalize())
		break
	}
	if len(first) != 1 || first[0].CID() != 65 || string(first[0].BytesCopy()) != "A" {
		t.Fatalf("early CMap sequence = %#v", first)
	}
	var all []int
	for code := range cmap.DecodeSeq(data) {
		all = append(all, code.CID())
	}
	if len(all) != 2 || all[0] != 65 || all[1] != 66 {
		t.Fatalf("repeatable CMap sequence = %#v", all)
	}
}

func TestCMapFinalizePreservesAbsentMapping(t *testing.T) {
	snapshot := (&CMap{}).Finalize()
	if snapshot == nil || snapshot.MappingCopy() != nil {
		t.Fatalf("zero CMap finalized with an allocated mapping: %#v", snapshot.MappingCopy())
	}
	if got := (&CMap{}).MappingCopy(); got != nil {
		t.Fatalf("zero CMap mapping copy = %#v, want nil", got)
	}
	if got := (&CMap{}).CodespacesCopy(); got != nil {
		t.Fatalf("zero CMap codespaces copy = %#v, want nil", got)
	}
}

func TestCMapFinalizePreservesEmptyCodespaces(t *testing.T) {
	cmap := testCMap(make([]CodeSpace, 0), nil, false)
	snapshot := cmap.Finalize()
	if snapshot == nil || snapshot.CodespacesCopy() == nil {
		t.Fatalf("empty codespaces were not preserved: %#v", snapshot)
	}
	if got := cmap.CodespacesCopy(); got == nil {
		t.Fatal("empty codespaces copy became nil")
	}
}

func TestCMapValueSnapshotsPreserveEmptyByteSlices(t *testing.T) {
	space := testCodeSpace(make([]byte, 0), make([]byte, 0))
	spaceSnapshot := space.Finalize()
	if spaceSnapshot.LowCopy() == nil || spaceSnapshot.HighCopy() == nil {
		t.Fatalf("empty codespace bytes were not preserved: %#v", spaceSnapshot)
	}
	if got := space.LowCopy(); got == nil {
		t.Fatal("empty codespace low copy became nil")
	}
	if got := space.HighCopy(); got == nil {
		t.Fatal("empty codespace high copy became nil")
	}

	code := testCode(make([]byte, 0), 0)
	if snapshot := code.Finalize(); snapshot.BytesCopy() == nil {
		t.Fatal("empty code bytes were not preserved")
	}
	if got := code.BytesCopy(); got == nil {
		t.Fatal("empty code copy became nil")
	}
}

func TestCMapUseMergePreservesEmptyCodespaceBytes(t *testing.T) {
	base := testCMap([]CodeSpace{testCodeSpace(make([]byte, 0), make([]byte, 0))}, nil, false)
	merged := testCMap(nil, nil, false)
	mergeTestCMap(merged, base)
	spaces := merged.CodespacesCopy()
	if len(spaces) != 1 || spaces[0].LowCopy() == nil || spaces[0].HighCopy() == nil {
		t.Fatalf("merged empty codespace bytes were collapsed: %#v", spaces)
	}
}

func TestParseCMapRejectsNegativeCIDValues(t *testing.T) {
	for _, source := range []string{
		"begincidchar\n<41> -1\nendcidchar",
		"begincidrange\n<41> <42> -1\nendcidrange",
	} {
		if _, err := ParseCMap([]byte(source)); err == nil {
			t.Fatalf("negative CID value was accepted: %q", source)
		}
	}
}

func TestParseCMapRejectsOversizedCodes(t *testing.T) {
	_, err := ParseCMap([]byte("begincodespacerange\n<0000000001> <0000000002>\nendcodespacerange"))
	if err == nil {
		t.Fatal("oversized CMap code was accepted")
	}
}

func TestParseCMapRejectsReversedCodespace(t *testing.T) {
	_, err := ParseCMap([]byte("begincodespacerange\n<ff> <00>\nendcodespacerange"))
	if err == nil {
		t.Fatal("reversed CMap codespace was accepted")
	}
}

func TestParseCMapRejectsCIDRangeOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	source := "begincidrange\n<41> <42> " + strconv.Itoa(maxInt) + "\nendcidrange"
	if _, err := ParseCMap([]byte(source)); err == nil {
		t.Fatal("overflowing CMap CID range was accepted")
	}
}

func TestParseCMapRejectsReversedCIDRange(t *testing.T) {
	_, err := ParseCMap([]byte("begincidrange\n<ff> <00> 1\nendcidrange"))
	if err == nil {
		t.Fatal("reversed CMap CID range was accepted")
	}
}

func TestParseCMapRejectsTruncatedRecords(t *testing.T) {
	for _, source := range []string{
		"begincodespacerange\n<00>",
		"begincidchar\n<41>",
		"begincidrange\n<41> <42>",
	} {
		if _, err := ParseCMap([]byte(source)); err == nil {
			t.Fatalf("truncated CMap record was accepted: %q", source)
		}
	}
}

func TestParseCMapWillIgnoreIncompleteTrailingRecordsLikePlaya(t *testing.T) {
	tests := []struct {
		name   string
		source string
		check  func(*CMap) bool
	}{
		{
			name:   "record before EOF",
			source: "begincidchar\n<40> 64\n<41>",
			check: func(cmap *CMap) bool {
				return cmap.MappingCopy()[string([]byte{0x40})] == 64 && len(cmap.MappingCopy()) == 1
			},
		},
		{
			name:   "record before section end",
			source: "begincidrange\n<40> <41> 64\n<42> <43>\nendcidrange",
			check: func(cmap *CMap) bool {
				mapping := cmap.MappingCopy()
				return mapping[string([]byte{0x40})] == 64 && mapping[string([]byte{0x41})] == 65 && len(mapping) == 2
			},
		},
		{
			name:   "codespace before section end",
			source: "begincodespacerange\n<00> <ff>\n<01>\nendcodespacerange",
			check: func(cmap *CMap) bool {
				return len(cmap.CodespacesCopy()) == 1
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmap, err := ParseCMap([]byte(tt.source))
			if err != nil {
				t.Fatalf("ParseCMap() error = %v", err)
			}
			if !tt.check(cmap) {
				t.Fatalf("ParseCMap() = %#v, incomplete suffix was not discarded", cmap)
			}
		})
	}
}

func TestParseCMapRejectsMismatchedSectionEnd(t *testing.T) {
	for _, source := range []string{
		"begincodespacerange\n<00> <ff>\nendcidchar",
		"begincidchar\n<41> 65\nendcidrange",
		"begincidrange\n<41> <42> 65\nendcodespacerange",
	} {
		if _, err := ParseCMap([]byte(source)); err == nil {
			t.Fatalf("mismatched CMap section end was accepted: %q", source)
		}
	}
}

func TestParseCMapRejectsUnexpectedSectionEnd(t *testing.T) {
	for _, token := range []string{"endcodespacerange", "endcidchar", "endcidrange"} {
		if _, err := ParseCMap([]byte(token)); err == nil {
			t.Fatalf("unexpected CMap section end was accepted: %s", token)
		}
	}
}

func TestParseCMapRejectsHugeCIDRange(t *testing.T) {
	source := "begincidrange\n<00000000> <ffffffff> 0\nendcidrange"
	if _, err := ParseCMap([]byte(source)); err == nil {
		t.Fatal("huge CMap CID range was accepted")
	}
}

func TestCMapMergeUseCMapKeepsLocalOverrides(t *testing.T) {
	base, err := ParseCMap([]byte("begincodespacerange\n<00> <ff>\nendcodespacerange\nbegincidchar\n<01> 10\nendcidchar"))
	if err != nil {
		t.Fatal(err)
	}
	local, err := ParseCMap([]byte("begincidchar\n<01> 20\nendcidchar"))
	if err != nil {
		t.Fatal(err)
	}
	mergeTestCMap(local, base)
	if local.MappingCopy()[string([]byte{1})] != 20 || len(local.CodespacesCopy()) != 1 || local.Decode([]byte{1})[0].CID() != 20 {
		t.Fatalf("merged cmap = %#v", local)
	}
}

func TestParseCMapRecordsNamedUseCMap(t *testing.T) {
	cmap, err := ParseCMap([]byte("/Identity-H usecmap\n/WMode 1 def"))
	if err != nil {
		t.Fatal(err)
	}
	if cmap.UseCMap() != "Identity-H" || !cmap.Vertical() {
		t.Fatalf("UseCMap = %#v", cmap)
	}
}

func TestParseEncodingCMapIgnoresUseCMap(t *testing.T) {
	cmap, err := ParseEncodingCMap([]byte("/Identity-H usecmap\n1 begincodespacerange\n<00> <ff>\nendcodespacerange"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cmap.UseCMap(); got != "" {
		t.Fatalf("Encoding CMap UseCMap = %q, want unsupported usecmap to be ignored", got)
	}
}

func TestCMapCIDRangeAndLongestCodespace(t *testing.T) {
	cmap, err := ParseCMap([]byte(`2 begincodespacerange
<00> <FF>
<0100> <01FF>
endcodespacerange
1 begincidrange
<0100> <0102> 500
endcidrange`))
	if err != nil {
		t.Fatal(err)
	}
	got := cmap.Decode([]byte{0x01, 0x00, 0x01, 0x02})
	if len(got) != 2 || got[0].CID() != 500 || got[1].CID() != 502 {
		t.Fatalf("decoded range: %#v", got)
	}
}

func TestEncodingCMapUsesAscendingCodeSpaceOrder(t *testing.T) {
	cmap, err := ParseEncodingCMap([]byte(`2 begincodespacerange
<00> <ff>
<0000> <ffff>
endcodespacerange
2 begincidchar
<01> 11
<0102> 22
endcidchar`))
	if err != nil {
		t.Fatal(err)
	}
	got := cmap.Decode([]byte{0x01, 0x02})
	if len(got) != 2 || got[0].CID() != 11 || got[1].CID() != 0 {
		t.Fatalf("Encoding CMap decode = %#v, want ascending code-space order", got)
	}
}

func TestCMapCIDRangeHandlesFourByteBoundary(t *testing.T) {
	cmap, err := ParseCMap([]byte(`1 begincodespacerange
<00ffffff> <01000002>
endcodespacerange
1 begincidrange
<00ffffff> <01000002> 700
endcidrange`))
	if err != nil {
		t.Fatal(err)
	}
	got := cmap.Decode([]byte{0x00, 0xff, 0xff, 0xff, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01, 0x01, 0x00, 0x00, 0x02})
	if len(got) != 4 || got[0].CID() != 700 || got[1].CID() != 701 || got[2].CID() != 702 || got[3].CID() != 703 {
		t.Fatalf("wide CID range = %#v", got)
	}
}

func TestParseCMapAcceptsMultipleEntriesOnOneLine(t *testing.T) {
	c, err := ParseCMap([]byte("2 begincodespacerange\n<00><7f> <8000><80ff>\nendcodespacerange\n2 begincidchar\n<41> 65 <42> 66\nendcidchar"))
	if err != nil {
		t.Fatal(err)
	}
	mapping := c.MappingCopy()
	if len(c.CodespacesCopy()) != 2 || mapping[string([]byte{0x41})] != 65 || mapping[string([]byte{0x42})] != 66 {
		t.Fatalf("parsed CMap = %#v", c)
	}
}

func TestParseCMapAcceptsEntriesAfterBeginOperator(t *testing.T) {
	c, err := ParseCMap([]byte("1 begincodespacerange <00> <ff> endcodespacerange\n1 begincidchar <41> 65 endcidchar"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.CodespacesCopy()) != 1 || c.MappingCopy()[string([]byte{0x41})] != 65 {
		t.Fatalf("parsed inline CMap = %#v", c)
	}
}

func TestParseCMapAcceptsRecordsSplitAcrossLines(t *testing.T) {
	cmap, err := ParseCMap([]byte(`2 begincodespacerange
<00>
<7f>
<8000> <80ff>
endcodespacerange
1 begincidrange
<41> <43>
65
endcidrange`))
	if err != nil {
		t.Fatal(err)
	}
	got := cmap.Decode([]byte{0x41, 0x42, 0x43})
	if len(got) != 3 || got[0].CID() != 65 || got[1].CID() != 66 || got[2].CID() != 67 {
		t.Fatalf("split CMap records = %#v", got)
	}
	spaces := cmap.CodespacesCopy()
	if len(spaces) != 2 || string(spaces[0].LowCopy()) != "\x00" || string(spaces[1].LowCopy()) != "\x80\x00" {
		t.Fatalf("split CMap codespaces = %#v", spaces)
	}
}

func TestParseCMapStreamsLargeCommentsAndRecords(t *testing.T) {
	comment := "%" + strings.Repeat("<deadbeef> 999999 ", 1<<16)
	source := comment + `
	2 begincodespacerange
	<00> <7f>
	<8100> <81ff>
endcodespacerange
2 begincidchar
<41> 65
<8140> 100
endcidchar`

	cmap, err := ParseCMap([]byte(source))
	if err != nil {
		t.Fatalf("ParseCMap with large comment: %v", err)
	}
	decoded := cmap.Decode([]byte{0x41, 0x81, 0x40})
	if len(decoded) != 2 || decoded[0].CID() != 65 || decoded[1].CID() != 100 {
		t.Fatalf("large-comment CMap decode = %#v", decoded)
	}
}

func TestParseCMapRejectsOversizedToken(t *testing.T) {
	source := "begincidchar\n<" + strings.Repeat("a", maxCMapTokenSize) + "> 1\nendcidchar"
	if _, err := ParseCMap([]byte(source)); err == nil {
		t.Fatal("ParseCMap accepted an oversized token")
	}
}

func TestParseCMapRejectsUnterminatedHexToken(t *testing.T) {
	if _, err := ParseCMap([]byte("<deadbeef")); err == nil {
		t.Fatal("ParseCMap accepted an unterminated hex token")
	}
}

func TestParseCMapKeepsFinalRecordWithoutEndOperator(t *testing.T) {
	cmap, err := ParseCMap([]byte("begincidchar <41> 65"))
	if err != nil || cmap.MappingCopy()[string([]byte{0x41})] != 65 {
		t.Fatalf("unterminated final CMap record = %#v, err=%v", cmap, err)
	}
}

func TestParseCMapReadsExactWritingMode(t *testing.T) {
	horizontal, err := ParseCMap([]byte("/WMode 0 def"))
	if err != nil || horizontal.Vertical() {
		t.Fatalf("horizontal WMode = %#v, err=%v", horizontal, err)
	}

	vertical, err := ParseCMap([]byte("/WMode 1 def"))
	if err != nil || !vertical.Vertical() {
		t.Fatalf("vertical WMode = %#v, err=%v", vertical, err)
	}

	nonZero, err := ParseCMap([]byte("/WMode 10 def"))
	if err != nil || !nonZero.Vertical() {
		t.Fatalf("non-zero WMode = %#v, err=%v", nonZero, err)
	}
}

func TestParseCMapTreatsAnyNonZeroWritingModeAsVertical(t *testing.T) {
	cmap, err := ParseCMap([]byte("/WMode 2 def"))
	if err != nil {
		t.Fatal(err)
	}
	if !cmap.Vertical() {
		t.Fatal("non-zero WMode was not treated as vertical")
	}
}

func TestParseCMapRequiresWritingModeDefinition(t *testing.T) {
	cmap, err := ParseCMap([]byte("/WMode 2"))
	if err != nil {
		t.Fatal(err)
	}
	if cmap.Vertical() {
		t.Fatal("WMode without def was applied")
	}
}

func TestCMapDecodeIgnoresInvalidAndDuplicateCodespaces(t *testing.T) {
	cmap := testCMap([]CodeSpace{
		{},
		testCodeSpace([]byte{0x00}, []byte{0xff}),
		testCodeSpace([]byte{0x10}, []byte{0x20}),
		testCodeSpace([]byte{0x00, 0x00}, []byte{0xff}),
	}, map[string]int{string([]byte{0x10}): 42}, false)
	got := cmap.Decode([]byte{0x10})
	if len(got) != 1 || got[0].CID() != 42 {
		t.Fatalf("decoded codespaces = %#v", got)
	}
}

func TestCMapCodespacesCopyDoesNotShareBytes(t *testing.T) {
	cmap := testCMap([]CodeSpace{testCodeSpace([]byte{0x00}, []byte{0xff})}, nil, false)
	copy := cmap.CodespacesCopy()
	if string(copy[0].LowCopy()) != "\x00" || string(copy[0].HighCopy()) != "\xff" {
		t.Fatalf("codespaces copy changed source bytes: %#v", copy)
	}
}

func testCodeSpace(low, high []byte) CodeSpace {
	return NewCodeSpace(low, high)
}

func testCode(value []byte, cid int) Code {
	return NewCode(value, cid)
}

func testCMap(codespaces []CodeSpace, mapping map[string]int, vertical bool) *CMap {
	return &CMap{codespaces: codespaces, mapping: mapping, vertical: vertical}
}

func mergeTestCMap(local, base *CMap) {
	local.mergeUseCMap(base)
}
