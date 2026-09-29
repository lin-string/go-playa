package fontdata

import "testing"

func TestParseCIDToGIDMapReadsEveryCompleteEntry(t *testing.T) {
	got := ParseCIDToGIDMap([]byte{0, 7, 1, 9, 2})
	if len(got) != 2 || got[0] != 7 || got[1] != 265 {
		t.Fatalf("CIDToGID map = %#v, want entries 0=7 and 1=265", got)
	}
}

func TestParseCIDToGIDMapBoundsCIDDomain(t *testing.T) {
	data := make([]byte, 2*(1<<16)+2)
	data[len(data)-2], data[len(data)-1] = 0, 7
	got := ParseCIDToGIDMap(data)
	if len(got) != 1<<16 || got[0] != 0 || got[1<<16-1] != 0 {
		t.Fatalf("bounded CIDToGID map = len:%d last:%d", len(got), got[1<<16-1])
	}
	if _, ok := got[1<<16]; ok {
		t.Fatal("CIDToGID map retained an unreachable 17-bit CID")
	}
}
