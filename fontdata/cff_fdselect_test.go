package fontdata

import "testing"

func TestParseCFFFDSelectFormats(t *testing.T) {
	format0 := []byte{0, 2, 3}
	if got := ParseCFFFDSelect(format0, 0, 2); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("format 0 FDSelect = %#v", got)
	}

	format3 := []byte{3, 0, 2, 0, 0, 1, 0, 2, 4, 0, 4}
	if got := ParseCFFFDSelect(format3, 0, 4); len(got) != 4 || got[0] != 1 || got[1] != 1 || got[2] != 4 || got[3] != 4 {
		t.Fatalf("format 3 FDSelect = %#v", got)
	}

	format4 := []byte{4, 0, 0, 0, 2, 0, 0, 0, 0, 0, 7, 0, 0, 0, 1, 0, 9, 0, 0, 0, 4}
	if got := ParseCFFFDSelect(format4, 0, 4); len(got) != 4 || got[0] != 7 || got[1] != 9 || got[2] != 9 || got[3] != 9 {
		t.Fatalf("format 4 FDSelect = %#v", got)
	}
}

func TestParseCFFFDSelectRejectsIncompleteRanges(t *testing.T) {
	if got := ParseCFFFDSelect([]byte{3, 0, 1, 0, 0, 0, 0}, 0, 2); got != nil {
		t.Fatalf("incomplete FDSelect = %#v, want nil", got)
	}
}
