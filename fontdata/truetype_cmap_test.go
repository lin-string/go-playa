package fontdata

import (
	"encoding/binary"
	"testing"
)

func TestParseTrueTypeCMapGlyphsSelectsFormat12(t *testing.T) {
	format := make([]byte, 28)
	binary.BigEndian.PutUint16(format[0:2], 12)
	binary.BigEndian.PutUint32(format[4:8], uint32(len(format)))
	binary.BigEndian.PutUint32(format[12:16], 1)
	binary.BigEndian.PutUint32(format[16:20], 0x20000)
	binary.BigEndian.PutUint32(format[20:24], 0x20001)
	binary.BigEndian.PutUint32(format[24:28], 7)

	cmap := make([]byte, 4+8+len(format))
	binary.BigEndian.PutUint16(cmap[2:4], 1)
	binary.BigEndian.PutUint32(cmap[8:12], 12)
	copy(cmap[12:], format)

	data := make([]byte, 12+16+len(cmap))
	binary.BigEndian.PutUint16(data[4:6], 1)
	copy(data[12:16], []byte("cmap"))
	binary.BigEndian.PutUint32(data[20:24], uint32(28))
	binary.BigEndian.PutUint32(data[24:28], uint32(len(cmap)))
	copy(data[28:], cmap)

	got := ParseTrueTypeCMapGlyphs(data)
	if got[7] != string(rune(0x20000)) || got[8] != string(rune(0x20001)) {
		t.Fatalf("format 12 glyph map = %#v", got)
	}
}

func TestParseTrueTypeCMapGlyphsPrefersFormat4OverModernFormats(t *testing.T) {
	format12 := make([]byte, 28)
	binary.BigEndian.PutUint16(format12[0:2], 12)
	binary.BigEndian.PutUint32(format12[4:8], uint32(len(format12)))
	binary.BigEndian.PutUint32(format12[12:16], 1)
	binary.BigEndian.PutUint32(format12[16:20], 0x20000)
	binary.BigEndian.PutUint32(format12[20:24], 0x20000)
	binary.BigEndian.PutUint32(format12[24:28], 7)

	format4 := make([]byte, 32)
	binary.BigEndian.PutUint16(format4[0:2], 4)
	binary.BigEndian.PutUint16(format4[2:4], uint16(len(format4)))
	binary.BigEndian.PutUint16(format4[6:8], 4)
	binary.BigEndian.PutUint16(format4[8:10], 4)
	binary.BigEndian.PutUint16(format4[10:12], 1)
	binary.BigEndian.PutUint16(format4[12:14], 0)
	binary.BigEndian.PutUint16(format4[14:16], 0x41)
	binary.BigEndian.PutUint16(format4[16:18], 0xffff)
	binary.BigEndian.PutUint16(format4[18:20], 0)
	binary.BigEndian.PutUint16(format4[20:22], 0x41)
	binary.BigEndian.PutUint16(format4[22:24], 0xffff)
	binary.BigEndian.PutUint16(format4[24:26], 0xffc6)
	binary.BigEndian.PutUint16(format4[26:28], 1)

	cmap := make([]byte, 4+16+len(format12)+len(format4))
	binary.BigEndian.PutUint16(cmap[2:4], 2)
	// A modern Unicode subtable appears first in this font.
	binary.BigEndian.PutUint16(cmap[4:6], 0)
	binary.BigEndian.PutUint16(cmap[6:8], 4)
	binary.BigEndian.PutUint32(cmap[8:12], 20)
	// Playa's supported legacy format 4 subtable must take precedence.
	binary.BigEndian.PutUint16(cmap[12:14], 3)
	binary.BigEndian.PutUint16(cmap[14:16], 1)
	binary.BigEndian.PutUint32(cmap[16:20], uint32(20+len(format12)))
	copy(cmap[20:], format12)
	copy(cmap[20+len(format12):], format4)

	data := make([]byte, 12+16+len(cmap))
	binary.BigEndian.PutUint16(data[4:6], 1)
	copy(data[12:16], []byte("cmap"))
	binary.BigEndian.PutUint32(data[20:24], 28)
	binary.BigEndian.PutUint32(data[24:28], uint32(len(cmap)))
	copy(data[28:], cmap)

	got := ParseTrueTypeCMapGlyphs(data)
	if got[7] != "A" {
		t.Fatalf("format 4 did not override modern cmap = %#v", got)
	}
}

func TestParseTrueTypeCMapGlyphsRejectsMalformedTable(t *testing.T) {
	if got := ParseTrueTypeCMapGlyphs([]byte("short")); got != nil {
		t.Fatalf("short font returned %#v", got)
	}
}

func TestTrueTypeCMapMappedSpanIsBoundedByGlyphIDs(t *testing.T) {
	if got := trueTypeCMapMappedSpan(0, 0x10ffff, 0xffff); got != 1 {
		t.Fatalf("glyph 0xffff span = %d, want 1", got)
	}
	if got := trueTypeCMapMappedSpan(0, 0x10ffff, 0x10000); got != 0 {
		t.Fatalf("unrepresentable glyph span = %d, want 0", got)
	}
	if got := trueTypeCMapMappedSpan(0x10000, 0x10010, 0); got != 17 {
		t.Fatalf("bounded glyph span = %d, want 17", got)
	}
}
