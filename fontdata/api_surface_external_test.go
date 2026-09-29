package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestPublicFontDataParsesCMapsAndOwnsSnapshots(t *testing.T) {
	cmap, err := fontdata.ParseCMap([]byte("1 begincodespacerange\n<00> <FF>\nendcodespacerange"))
	if err != nil || cmap == nil {
		t.Fatalf("ParseCMap() = %#v, %v", cmap, err)
	}
	if got := cmap.Finalize(); got == nil {
		t.Fatal("CMap.Finalize returned nil")
	}
}

func TestPublicFontDataExposesEncodingCMapParser(t *testing.T) {
	cmap, err := fontdata.ParseEncodingCMap([]byte("1 begincodespacerange\n<00> <FF>\nendcodespacerange"))
	if err != nil || cmap == nil {
		t.Fatalf("ParseEncodingCMap() = %#v, %v", cmap, err)
	}
}

func TestPublicFontDataParsesCIDToGIDMaps(t *testing.T) {
	mapping := fontdata.ParseCIDToGIDMap([]byte{0, 7, 1, 9, 2})
	if len(mapping) != 2 || mapping[0] != 7 || mapping[1] != 265 {
		t.Fatalf("ParseCIDToGIDMap() = %#v", mapping)
	}
}

func TestPublicFontDataParsesTrueTypeHorizontalMetrics(t *testing.T) {
	if metrics := fontdata.ParseTrueTypeHorizontalMetrics(nil); metrics != nil {
		t.Fatalf("ParseTrueTypeHorizontalMetrics(nil) = %#v, want nil", metrics)
	}
}

func TestPublicFontDataParsesCFFIndexes(t *testing.T) {
	data := []byte{0, 0, 0, 2, 1, 1, 2, 3, 'A', 'B'}
	items, end, ok := fontdata.ParseCFF2Index(data, 0)
	if !ok || end != len(data) || len(items) != 2 || string(items[0]) != "A" || string(items[1]) != "B" {
		t.Fatalf("ParseCFF2Index() = %#v, end=%d, ok=%v", items, end, ok)
	}
}

func TestPublicFontDataParsesCFFFDSelect(t *testing.T) {
	data := []byte{0, 2, 3}
	fdByGlyph := fontdata.ParseCFFFDSelect(data, 0, 2)
	if len(fdByGlyph) != 2 || fdByGlyph[0] != 2 || fdByGlyph[1] != 3 {
		t.Fatalf("ParseCFFFDSelect() = %#v", fdByGlyph)
	}
}

func TestPublicFontDataParsesCFFScalars(t *testing.T) {
	if value, end, ok := fontdata.ParseCFFNumber([]byte{139}, 0); !ok || value != 0 || end != 1 {
		t.Fatalf("ParseCFFNumber() = %v, %d, %v", value, end, ok)
	}
	if op, end, ok := fontdata.ParseCFFOperator([]byte{12, 7}, 0, false); !ok || op != 1207 || end != 2 {
		t.Fatalf("ParseCFFOperator() = %d, %d, %v", op, end, ok)
	}
}

func TestPublicFontDataParsesCFF2VariationStore(t *testing.T) {
	data := []byte{
		0, 1, 0, 0, 0, 22, 0, 1, 0, 0, 0, 12,
		0, 1, 0, 1, 0, 1, 0, 0, 0xff, 0xce,
		0, 1, 0, 1, 0, 0, 0x40, 0, 0x40, 0,
	}
	store := fontdata.ParseCFF2VariationStore(data, 0)
	if store == nil || store.RegionCount() != 1 || store.DataCount() != 1 {
		t.Fatalf("ParseCFF2VariationStore() = %#v", store)
	}
}

func TestPublicFontDataParsesCFFNumericDomains(t *testing.T) {
	if value, ok := fontdata.ParseCFFInteger(7); !ok || value != 7 {
		t.Fatalf("ParseCFFInteger() = %d, %v", value, ok)
	}
	if index, ok := fontdata.ParseCFFSubroutineIndex(-107, 1); !ok || index != 0 {
		t.Fatalf("ParseCFFSubroutineIndex() = %d, %v", index, ok)
	}
}

func TestPublicFontDataExposesDecodedGlyphSnapshots(t *testing.T) {
	glyph := fontdata.NewDecodedGlyph("A", []byte{0x41}, 65)
	if glyph.Text() != "A" || glyph.CID() != 65 {
		t.Fatalf("decoded glyph identity = text:%q cid:%d", glyph.Text(), glyph.CID())
	}
	bytes := glyph.BytesCopy()
	bytes[0] = 0
	if got := glyph.BytesCopy(); len(got) != 1 || got[0] != 0x41 {
		t.Fatalf("decoded glyph bytes = %#v", got)
	}
	if got := glyph.BytesBorrowed(); len(got) != 1 || got[0] != 0x41 {
		t.Fatalf("decoded glyph borrowed bytes = %#v", got)
	}
	if finalized := glyph.Finalize(); finalized.Text() != "A" || finalized.CID() != 65 {
		t.Fatalf("decoded glyph finalization = %#v", finalized)
	}
}

func TestPublicFontDataExposesReadOnlyCore14Metrics(t *testing.T) {
	metric, ok := fontdata.LookupStandardFontMetric("Helvetica")
	if !ok {
		t.Fatal("LookupStandardFontMetric(Helvetica) returned false")
	}
	if metric.Ascent() != 718 || metric.Descent() != -207 || metric.BBox() != [4]float64{-166, -225, 1000, 931} {
		t.Fatalf("Helvetica metrics = ascent %v, descent %v, bbox %v", metric.Ascent(), metric.Descent(), metric.BBox())
	}
	if metric.ItalicAngle() != 0 {
		t.Fatalf("Helvetica italic angle = %v, want 0", metric.ItalicAngle())
	}
	if width, ok := metric.Width('A'); !ok || width != 667 {
		t.Fatalf("Helvetica A width = %v, %v", width, ok)
	}

	widths := metric.WidthsCopy()
	widths['A'] = -1
	if width, ok := metric.Width('A'); !ok || width != 667 {
		t.Fatalf("metric width changed through copy = %v, %v", width, ok)
	}
	finalized := metric.Finalize()
	finalizedWidths := finalized.WidthsCopy()
	finalizedWidths['A'] = -2
	if width, ok := finalized.Width('A'); !ok || width != 667 {
		t.Fatalf("finalized metric width changed through copy = %v, %v", width, ok)
	}
	if width, ok := fontdata.LookupZapfDingbatsWidth(33); !ok || width <= 0 {
		t.Fatalf("LookupZapfDingbatsWidth(33) = %v, %v", width, ok)
	}
	if _, ok := fontdata.LookupStandardFontMetric("missing"); ok {
		t.Fatal("LookupStandardFontMetric(missing) returned true")
	}
}

func TestPublicFontDataExposesReadOnlyCIDUnicodeResourceLookup(t *testing.T) {
	name, ok := fontdata.PredefinedCIDUnicodeMapName("Japan1")
	if !ok || name != "UniJIS-UTF32-H" {
		t.Fatalf("PredefinedCIDUnicodeMapName(Japan1) = %q, %v", name, ok)
	}
	if name, ok := fontdata.PredefinedCIDUnicodeMapName("unknown"); ok || name != "" {
		t.Fatalf("PredefinedCIDUnicodeMapName(unknown) = %q, %v", name, ok)
	}
}
