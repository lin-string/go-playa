package fontdata

import "testing"

func TestCloneType1BytesPreservesSnapshotEmptyContract(t *testing.T) {
	for _, source := range [][]byte{nil, {}, {14, 15}} {
		first, second := cloneType1Bytes(source), cloneType1Bytes(source)
		if (first == nil) != (len(source) == 0) || (second == nil) != (len(source) == 0) {
			t.Fatalf("copies = %#v, %#v; want nil for empty input", first, second)
		}
		if len(source) > 0 {
			first[0] = 9
			if second[0] != 14 || source[0] != 14 {
				t.Fatal("ownership copies share byte storage")
			}
		}
	}
}

func TestType1SnapshotsPreserveEmptyEntriesAndIsolateAliases(t *testing.T) {
	for _, data := range [][]byte{nil, {}, {14, 15}} {
		program := &Type1Program{
			charstrings: map[string][]byte{"A": data},
			subrs:       [][]byte{data},
		}
		charstring := program.CharString("A")
		charstrings := program.CharStringsCopy()
		subrs := program.SubrsCopy()
		if _, present := charstrings["A"]; !present || len(subrs) != 1 {
			t.Fatal("snapshot dropped an empty entry")
		}
		for _, snapshot := range [][]byte{charstring, charstrings["A"], subrs[0]} {
			if (snapshot == nil) != (len(data) == 0) {
				t.Fatalf("snapshot = %#v; want empty input collapsed to nil", snapshot)
			}
		}
		if len(data) > 0 {
			charstring[0], charstrings["A"][0], subrs[0][0] = 1, 2, 3
			if data[0] != 14 || program.CharString("A")[0] != 14 {
				t.Fatal("snapshot mutated program storage")
			}
			if charstring[0] != 1 || charstrings["A"][0] != 2 {
				t.Fatal("snapshots share storage with one another")
			}
		}
		if program.CharString("missing") != nil {
			t.Fatal("missing charstring must remain nil")
		}
	}
	if got := (&Type1Program{}).SubrsCopy(); got != nil {
		t.Fatalf("absent subrs = %#v, want nil", got)
	}
	if got := (&Type1Program{subrs: [][]byte{}}).SubrsCopy(); got == nil {
		t.Fatal("present empty subrs must remain nonnil")
	}
}

func TestCMapOwnedBytesPreserveNilEmptyAndIsolateAliases(t *testing.T) {
	for _, data := range [][]byte{nil, {}, {1, 2}} {
		space := NewCodeSpace(data, data)
		code := NewCode(data, 7)
		glyph := NewDecodedGlyph("A", data, 7)
		copies := [][]byte{space.LowCopy(), space.HighCopy(), space.Finalize().LowCopy(), code.BytesCopy(), code.Finalize().BytesCopy(), glyph.BytesCopy(), glyph.Finalize().BytesCopy()}
		for _, snapshot := range copies {
			if (snapshot == nil) != (data == nil) {
				t.Fatalf("snapshot nil = %v, want %v", snapshot == nil, data == nil)
			}
			if len(snapshot) > 0 {
				snapshot[0] = 9
			}
		}
		if len(data) > 0 {
			if data[0] != 1 || space.LowCopy()[0] != 1 || code.BytesCopy()[0] != 1 || glyph.BytesCopy()[0] != 1 {
				t.Fatal("snapshot mutated original byte storage")
			}
			data[0] = 8
			if space.LowCopy()[0] != 1 || code.BytesCopy()[0] != 1 || glyph.BytesCopy()[0] != 1 {
				t.Fatal("constructor retained borrowed byte storage")
			}
		}
	}
}
