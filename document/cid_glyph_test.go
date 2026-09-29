package document

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestParseCIDToGIDMapReadsEveryCompleteEntry(t *testing.T) {
	got := fontdata.ParseCIDToGIDMap([]byte{0, 7, 1, 9, 2})
	if len(got) != 2 || got[0] != 7 || got[1] != 265 {
		t.Fatalf("CIDToGID map = %#v, want entries 0=7 and 1=265", got)
	}
}

func TestParseCIDToGIDMapBoundsCIDDomain(t *testing.T) {
	data := make([]byte, 2*(1<<16)+2)
	data[len(data)-2], data[len(data)-1] = 0, 7
	got := fontdata.ParseCIDToGIDMap(data)
	if len(got) != 1<<16 || got[0] != 0 || got[1<<16-1] != 0 {
		t.Fatalf("bounded CIDToGID map = len:%d last:%d", len(got), got[1<<16-1])
	}
	if _, ok := got[1<<16]; ok {
		t.Fatal("CIDToGID map retained an unreachable 17-bit CID")
	}
}

func TestCIDToGIDMapParsingIsLazy(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cidToGIDData = []byte{0, 7, 0, 9}
	if f.cidToGIDParsed || len(f.cidToGID) != 0 {
		t.Fatal("CIDToGID map parsed during setup")
	}
	if got := f.glyphID(1); got != 9 || !f.cidToGIDParsed {
		t.Fatalf("lazy CIDToGID lookup = %d", got)
	}
}

func TestWidthCodePublishesParsedCIDToGIDState(t *testing.T) {
	font := NewSimpleFont("CID")
	font.cid = true
	font.cidToGIDData = []byte{0, 7, 0, 9}

	if _, err := font.WidthCodeWithError([]byte{1}); err != nil {
		t.Fatal(err)
	}
	state := font.interpreterWidthState()
	if state == nil {
		t.Fatal("WidthCodeWithError did not publish an interpreter state")
	}
	if got := state.glyphID([]byte{1}, 1); got != 9 {
		t.Fatalf("published CIDToGID glyph = %d, want 9", got)
	}
}

func TestFontCIDToGIDCopyIsLazyAndOwned(t *testing.T) {
	font := NewSimpleFont("CID")
	font.cidToGIDData = []byte{0, 7, 1, 9}
	if font.cidToGIDParsed {
		t.Fatal("CIDToGID map parsed during setup")
	}

	got, err := font.CIDToGIDCopyWithError()
	if err != nil {
		t.Fatalf("CIDToGID copy error: %v", err)
	}
	if got[0] != 7 || got[1] != 265 {
		t.Fatalf("CIDToGID copy = %#v, want 0=7 and 1=265", got)
	}
	got[0] = 99
	again, err := font.CIDToGIDCopyWithError()
	if err != nil {
		t.Fatalf("second CIDToGID copy error: %v", err)
	}
	if again[0] != 7 {
		t.Fatalf("CIDToGID copy exposed mutable cache: %#v", again)
	}
}
