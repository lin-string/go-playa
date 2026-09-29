package document

import (
	"encoding/binary"
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestParseTrueTypeHorizontalMetricsScalesAndRepeats(t *testing.T) {
	metrics := fontdata.ParseTrueTypeHorizontalMetrics(testTrueTypeWithHorizontalMetrics())
	if got := metrics[1]; got != 750 {
		t.Fatalf("glyph 1 width = %v, want 750", got)
	}
	if got := metrics[2]; got != 750 {
		t.Fatalf("glyph 2 repeated width = %v, want 750", got)
	}
}

func TestParseTrueTypeHorizontalMetricsRejectsIncompleteTables(t *testing.T) {
	data := testTrueTypeWithHorizontalMetrics()
	binary.BigEndian.PutUint32(data[12+3*16+12:], 6)
	if metrics := fontdata.ParseTrueTypeHorizontalMetrics(data); metrics != nil {
		t.Fatalf("truncated hmtx returned partial metrics: %#v", metrics)
	}

	data = testTrueTypeWithHorizontalMetrics()
	hheaOffset := binary.BigEndian.Uint32(data[12+16+8:])
	binary.BigEndian.PutUint16(data[hheaOffset+34:], 4)
	if metrics := fontdata.ParseTrueTypeHorizontalMetrics(data); metrics != nil {
		t.Fatalf("invalid hhea metric count returned metrics: %#v", metrics)
	}
}

func TestCIDWidthIgnoresEmbeddedGlyphMetrics(t *testing.T) {
	f := NewSimpleFont("CID")
	f.cid = true
	f.cidToGID = map[int]int{5: 1}
	f.glyphIDWidths = map[int]float64{1: 750}
	if got := f.HDisp(5); got != 0.5 || f.WidthCode([]byte{0, 5}) != 500 {
		t.Fatalf("embedded glyph width affected CID layout: HDisp %v WidthCode %v", got, f.WidthCode([]byte{0, 5}))
	}
}

func TestSimpleTrueTypeWidthUsesEmbeddedGlyphMetricsWhenPDFWidthIsMissing(t *testing.T) {
	f := NewSimpleFont("Test")
	f.fontType = "TrueType"
	f.encoding['A'] = 'A'
	f.glyphIDToUnicode = map[int]string{7: "A"}
	f.glyphIDWidths = map[int]float64{7: 620}

	if got := f.WidthCode([]byte{'A'}); got != 620 {
		t.Fatalf("embedded simple-font width = %v, want 620", got)
	}
}

func TestSimpleTrueTypeExplicitPDFWidthOverridesEmbeddedGlyphMetrics(t *testing.T) {
	f := NewSimpleFont("Test")
	f.fontType = "TrueType"
	f.encoding['A'] = 'A'
	f.widths['A'] = 700
	f.glyphIDToUnicode = map[int]string{7: "A"}
	f.glyphIDWidths = map[int]float64{7: 620}

	if got := f.WidthCode([]byte{'A'}); got != 700 {
		t.Fatalf("explicit simple-font width = %v, want 700", got)
	}
}

func testTrueTypeWithHorizontalMetrics() []byte {
	tables := map[string][]byte{
		"head": func() []byte {
			data := make([]byte, 20)
			binary.BigEndian.PutUint16(data[18:], 1000)
			return data
		}(),
		"hhea": func() []byte {
			data := make([]byte, 36)
			binary.BigEndian.PutUint16(data[34:], 2)
			return data
		}(),
		"maxp": func() []byte {
			data := make([]byte, 6)
			binary.BigEndian.PutUint16(data[4:], 3)
			return data
		}(),
		"hmtx": {0x01, 0xf4, 0, 0, 0x02, 0xee, 0, 0, 0, 0, 0, 0},
	}
	order := []string{"head", "hhea", "maxp", "hmtx"}
	data := make([]byte, 12+16*len(order))
	binary.BigEndian.PutUint32(data, 0x00010000)
	binary.BigEndian.PutUint16(data[4:], uint16(len(order)))
	for i, tag := range order {
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
		offset := len(data)
		table := tables[tag]
		data = append(data, table...)
		entry := 12 + i*16
		copy(data[entry:entry+4], tag)
		binary.BigEndian.PutUint32(data[entry+8:], uint32(offset))
		binary.BigEndian.PutUint32(data[entry+12:], uint32(len(table)))
	}
	return data
}
