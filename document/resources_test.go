package document

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
)

func TestFontEmbeddedResourceLoadersPreserveEmptyData(t *testing.T) {
	trueType := NewSimpleFont("TrueType")
	applyTrueTypeCMap(&Document{}, trueType, Dict{
		Name("FontFile2"): newStream(Dict{}, make([]byte, 0)),
	})
	if trueType.trueTypeData == nil {
		t.Fatal("empty TrueType embedded data became nil")
	}

	cff := NewSimpleFont("Type1")
	applyCFFMetadata(&Document{}, cff, Dict{
		Name("FontFile3"): newStream(Dict{}, make([]byte, 0)),
	})
	if cff.cffData == nil {
		t.Fatal("empty CFF embedded data became nil")
	}
}

func TestFontCachesDoNotRetainOversizedValues(t *testing.T) {
	ref := Ref{Object: 99}
	font := &Font{toUnicodeData: bytes.Repeat([]byte{'x'}, fontCacheLimit+1)}
	d := &Document{fontCache: map[Ref]*Font{}, pageFontCache: map[Ref]map[string]*Font{}}
	d.storeFont(ref, font)
	d.storePageFonts(ref, map[string]*Font{"F1": font})
	if len(d.fontCache) != 0 || d.fontCacheBytes != 0 || len(d.pageFontCache) != 0 || d.pageFontCacheBytes != 0 {
		t.Fatalf("oversized font entered cache: fonts=%d/%d page-fonts=%d/%d", len(d.fontCache), d.fontCacheBytes, len(d.pageFontCache), d.pageFontCacheBytes)
	}
}

func TestFontCacheReplacesOldEntriesWhenBudgetIsFull(t *testing.T) {
	firstRef, secondRef, thirdRef := Ref{Object: 1}, Ref{Object: 2}, Ref{Object: 3}
	first, second, third := NewSimpleFont("first1"), NewSimpleFont("second"), NewSimpleFont("third3")
	limit := fontCacheSize(first) + fontCacheSize(second)
	d := &Document{
		fontCache: map[Ref]*Font{}, cacheOptionsConfigured: true,
		cacheOptions: cacheconfig.Options{FontBytes: limit},
	}
	d.storeFont(firstRef, first)
	d.storeFont(secondRef, second)
	d.storeFont(thirdRef, third)
	if _, ok := d.fontCache[thirdRef]; !ok {
		t.Fatalf("new font was not cached after budget filled: %#v", d.fontCache)
	}
	if _, ok := d.fontCache[firstRef]; ok {
		t.Fatalf("oldest font was not evicted: %#v", d.fontCache)
	}
	if _, ok := d.fontCache[secondRef]; !ok {
		t.Fatalf("newer font was evicted before oldest: %#v", d.fontCache)
	}
	if d.fontCacheBytes < 0 || d.fontCacheBytes > limit {
		t.Fatalf("font cache accounting = %d, limit %d", d.fontCacheBytes, limit)
	}
}

func TestPageFontCacheReplacesOldPagesWhenBudgetIsFull(t *testing.T) {
	firstRef, secondRef, thirdRef := Ref{Object: 1}, Ref{Object: 2}, Ref{Object: 3}
	first := map[string]*Font{"F1": NewSimpleFont("first1")}
	second := map[string]*Font{"F2": NewSimpleFont("second")}
	third := map[string]*Font{"F3": NewSimpleFont("third3")}
	limit := pageFontMapCacheSize(first) + pageFontMapCacheSize(second)
	d := &Document{
		pageFontCache: map[Ref]map[string]*Font{}, cacheOptionsConfigured: true,
		cacheOptions: cacheconfig.Options{PageFontBytes: limit},
	}
	d.storePageFonts(firstRef, first)
	d.storePageFonts(secondRef, second)
	d.storePageFonts(thirdRef, third)
	if _, ok := d.pageFontCache[thirdRef]; !ok {
		t.Fatalf("new page font map was not cached after budget filled: %#v", d.pageFontCache)
	}
	if _, ok := d.pageFontCache[firstRef]; ok {
		t.Fatalf("oldest page font map was not evicted: %#v", d.pageFontCache)
	}
	if _, ok := d.pageFontCache[secondRef]; !ok {
		t.Fatalf("newer page font map was evicted before oldest: %#v", d.pageFontCache)
	}
	if d.pageFontCacheBytes < 0 || d.pageFontCacheBytes > limit {
		t.Fatalf("page font cache accounting = %d, limit %d", d.pageFontCacheBytes, limit)
	}
}

func TestFontCacheSizeCountsType3ProcedurePayloads(t *testing.T) {
	data := bytes.Repeat([]byte{'x'}, 1<<20)
	font := NewSimpleFont("Type3")
	font.type3 = true
	font.charProcs["A"] = newStream(nil, data)
	if got := fontCacheSize(font); got < len(data) {
		t.Fatalf("font cache size = %d, want at least procedure payload %d", got, len(data))
	}
}

func TestPageResourceCacheEvictionClearsInconsistentAccounting(t *testing.T) {
	ref := Ref{Object: 9}
	font := NewSimpleFont("cached")
	d := &Document{
		pageFontCache:      map[Ref]map[string]*Font{ref: {"F1": font}},
		pageFontCacheBytes: 1,
		pagePropCache:      map[Ref]Dict{ref: {Name("Value"): String("cached")}},
		pagePropCacheBytes: 1,
	}
	d.storePageFonts(ref, map[string]*Font{})
	d.storePageProperties(ref, Dict{})
	if d.pageFontCache != nil || d.pageFontCacheBytes != 0 {
		t.Fatalf("page font cache survived inconsistent accounting: cache=%#v bytes=%d", d.pageFontCache, d.pageFontCacheBytes)
	}
	if d.pagePropCache != nil || d.pagePropCacheBytes != 0 {
		t.Fatalf("page property cache survived inconsistent accounting: cache=%#v bytes=%d", d.pagePropCache, d.pagePropCacheBytes)
	}
}

func TestFontSnapshotsDropDocumentOwnership(t *testing.T) {
	d := &Document{}
	font := &Font{name: "F1", document: d}
	if snapshot := font.Finalize(); snapshot == nil || snapshot.document != nil {
		t.Fatalf("font snapshot retained document: %#v", snapshot)
	}
	resource := FontResource{name: "F1", font: font}
	if snapshot := resource.FontCopy(); snapshot == nil || snapshot.document != nil {
		t.Fatalf("font resource copy retained document: %#v", snapshot)
	}
	text := newTestText(contentdata.TextSpec{}, font, nil)
	if snapshot := text.FontCopy(); snapshot == nil || snapshot.document != nil {
		t.Fatalf("text object copy retained document: %#v", snapshot)
	}
	glyph := newTestGlyphWithFont(font)
	if snapshot := glyph.FontCopy(); snapshot == nil || snapshot.document != nil {
		t.Fatalf("glyph object copy retained document: %#v", snapshot)
	}
}

func TestZeroValueFontOwnersDoNotPanic(t *testing.T) {
	if got := (FontResource{}).FontCopy(); got != nil {
		t.Fatalf("zero font resource copy = %#v, want nil", got)
	}
	if got := (FontResource{}).Finalize().FontCopy(); got != nil {
		t.Fatalf("zero font resource final copy = %#v, want nil", got)
	}
	if got := (TextObject{}).FontCopy(); got != nil {
		t.Fatalf("zero text font copy = %#v, want nil", got)
	}
	_ = (TextObject{}).Finalize()
	if got := (GlyphObject{}).FontCopy(); got != nil {
		t.Fatalf("zero glyph font copy = %#v, want nil", got)
	}
	_ = (GlyphObject{}).Finalize()
}

func TestFontResourceMetadataDoesNotMaterializeLazyCMap(t *testing.T) {
	font := &Font{
		name:         "F1",
		defaultWidth: 1000,
		cmapData:     []byte("begincodespacerange\n<00> <ff>\nendcodespacerange"),
		cmapParsed:   false,
		italicAngle:  -12,
		leading:      24,
		capHeight:    700,
		stemV:        80,
		fontMatrix:   geometry.Matrix{0.002, 0, 0, 0.002, 0, 0},
		hasFlags:     true,
		hasFontBBox:  true,
		fontBBox:     [4]float64{-10, -20, 1000, 900},
	}
	metadata, ok := (FontResource{name: "F1", font: font}).Metadata()
	if !ok || metadata.Name() != "F1" || metadata.DefaultWidth() != 1000 || metadata.ItalicAngle() != -12 || metadata.Leading() != 24 || metadata.CapHeight() != 700 || metadata.StemV() != 80 || metadata.FontMatrix() != font.fontMatrix || !metadata.HasFlags() || !metadata.HasBBox() {
		t.Fatalf("font metadata = %#v, ok=%v", metadata, ok)
	}
	if font.cmapParsed || font.cmap != nil || len(font.cmapData) == 0 {
		t.Fatalf("font metadata materialized lazy CMap: parsed=%v cmap=%p data=%d", font.cmapParsed, font.cmap, len(font.cmapData))
	}
}

func TestFontResourceStrictSnapshotsReportDeferredFontFailures(t *testing.T) {
	font := NewSimpleFont("CFF")
	font.cffData = []byte{1, 0, 4}
	resource := FontResource{name: "F1", font: font}

	if snapshot, err := resource.FontCopyWithError(); err == nil || snapshot != nil {
		t.Fatalf("font copy = %#v, err=%v", snapshot, err)
	}
	if snapshot, err := resource.FinalizeWithError(); err == nil || snapshot.font != nil {
		t.Fatalf("resource finalize = %#v, err=%v", snapshot, err)
	}
}

func TestPageFontsReusesIndirectFont(t *testing.T) {
	fontRef := Ref{Object: 7, Generation: 0}
	d := &Document{
		objects: map[Ref]Object{
			fontRef: Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
		},
		fontCache: map[Ref]*Font{},
	}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): fontRef}},
	}}
	first, err := d.PageFonts(p)
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.PageFonts(p)
	if err != nil {
		t.Fatal(err)
	}
	if first["F1"] == nil || second["F1"] == nil || first["F1"].name != second["F1"].name {
		t.Fatal("page font was not reused from the cache")
	}
	if len(d.fontCache) != 1 {
		t.Fatalf("font cache size = %d, want 1", len(d.fontCache))
	}
}

func TestDocumentGetFontBuildsAndCachesFontSpecs(t *testing.T) {
	d := &Document{fontCache: map[Ref]*Font{}}
	spec := Dict{
		Name("Type"):     Name("Font"),
		Name("Subtype"):  Name("Type1"),
		Name("BaseFont"): Name("Helvetica"),
	}

	first, err := d.GetFontWithError(42, spec)
	if err != nil {
		t.Fatalf("GetFontWithError() error = %v", err)
	}
	if first == nil || first.Name() != "Helvetica" || first.DefaultWidth() != 1000 {
		t.Fatalf("font = %#v, name=%q width=%v", first, first.Name(), first.DefaultWidth())
	}

	second, err := d.GetFontWithError(42, Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Times-Roman")})
	if err != nil {
		t.Fatalf("cached GetFontWithError() error = %v", err)
	}
	if second != first {
		t.Fatalf("cached font pointer changed: first=%p second=%p", first, second)
	}
}

func TestDocumentGetFontReturnsPlayaStyleDummyForMissingSpec(t *testing.T) {
	d := &Document{fontCache: map[Ref]*Font{}}
	font, err := d.GetFontWithError(0, nil)
	if err != nil {
		t.Fatalf("GetFontWithError(nil spec) error = %v", err)
	}
	if font == nil || font.Name() != "unknown" || font.BaseFont() != "unknown" || font.DefaultWidth() != 1000 {
		t.Fatalf("dummy font = %#v, name=%q basefont=%q width=%v", font, font.Name(), font.BaseFont(), font.DefaultWidth())
	}
}

func TestDocumentGetFontReturnsCachedDummyForUnknownSubtype(t *testing.T) {
	d := &Document{fontCache: map[Ref]*Font{}}
	spec := Dict{Name("Subtype"): Name("UnknownFont")}
	first, err := d.GetFontWithError(7, spec)
	if err != nil {
		t.Fatalf("GetFontWithError() error = %v", err)
	}
	second, err := d.GetFontWithError(7, Dict{Name("Subtype"): Name("Type3")})
	if err != nil {
		t.Fatalf("cached GetFontWithError() error = %v", err)
	}
	if first != second || first.Name() != "unknown" {
		t.Fatalf("unknown font cache = first %p second %p name=%q", first, second, first.Name())
	}
}

func TestPageFontsFollowMultiLevelIndirectResourceChains(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	d.objects[Ref{Object: 1}] = Ref{Object: 2}
	d.objects[Ref{Object: 2}] = Dict{Name("Font"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Dict{Name("F1"): Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = Ref{Object: 5}
	d.objects[Ref{Object: 5}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}
	p := Page{dict: Dict{Name("Resources"): Ref{Object: 1}}}

	fonts, err := d.PageFonts(p)
	if err != nil {
		t.Fatalf("PageFonts() error = %v", err)
	}
	if fonts["F1"] == nil || fonts["F1"].name != "Helvetica" {
		t.Fatalf("PageFonts() = %#v", fonts)
	}
}

func TestPageFontsReuseCacheAcrossIndirectReferenceChains(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	d.objects[Ref{Object: 1}] = Ref{Object: 2}
	d.objects[Ref{Object: 2}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{
		Name("F1"): Ref{Object: 1}, Name("F2"): Ref{Object: 2},
	}}}}
	fonts, err := d.PageFonts(p)
	if err != nil || fonts["F1"] == nil || fonts["F2"] == nil || len(d.fontCache) != 1 {
		t.Fatalf("fonts = %#v cache=%d err=%v", fonts, len(d.fontCache), err)
	}
}

func TestPageFontsFollowMultiLevelIndirectFontFields(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	put := func(n int, value Object) { d.objects[Ref{Object: n}] = value }
	put(1, Dict{Name("Font"): Dict{Name("F1"): Ref{Object: 2}}})
	put(2, Dict{
		Name("Subtype"):        Ref{Object: 3},
		Name("BaseFont"):       Ref{Object: 4},
		Name("FontDescriptor"): Ref{Object: 5},
		Name("Encoding"):       Ref{Object: 8},
		Name("FirstChar"):      Ref{Object: 12},
		Name("Widths"):         Ref{Object: 13},
		Name("ToUnicode"):      Ref{Object: 14},
		Name("DW"):             Ref{Object: 15},
	})
	put(3, Name("Type1"))
	put(4, Name("IndirectTestFont"))
	put(5, Dict{Name("FontName"): Ref{Object: 6}, Name("Flags"): Ref{Object: 7}})
	put(6, Name("IndirectTestFont"))
	put(7, Number(32))
	put(8, Dict{Name("BaseEncoding"): Ref{Object: 9}, Name("Differences"): Ref{Object: 10}})
	put(9, Name("WinAnsiEncoding"))
	put(10, Array{Ref{Object: 11}, Name("Z")})
	put(11, Number(65))
	put(12, Number(65))
	put(13, Array{Ref{Object: 16}})
	put(14, newStream(nil, []byte("begincmap\nendcmap")))
	put(15, Ref{Object: 17})
	put(16, Ref{Object: 18})
	put(17, Number(1000))
	put(18, Number(600))

	fonts, err := d.PageFonts(Page{dict: Dict{Name("Resources"): Ref{Object: 1}}})
	if err != nil {
		t.Fatalf("PageFonts() error = %v", err)
	}
	f := fonts["F1"]
	if f == nil {
		t.Fatalf("multi-level font fields = %#v", f)
	}
	width, widthOK := f.WidthValue(65)
	if f.name != "IndirectTestFont" || f.subtype != "" || !widthOK || width != 600 {
		t.Fatalf("multi-level font fields = %#v", f)
	}
	if name, ok := f.GlyphName(65); !ok || name != "Z" {
		t.Fatalf("multi-level encoding = %q, %v", name, ok)
	}
	if f.defaultWidth != 1000 {
		t.Fatalf("multi-level font defaults were not retained: %#v", f)
	}
}

func TestPageFontSequenceReportsMalformedResourceRoots(t *testing.T) {
	tests := []struct {
		name string
		page Page
	}{
		{name: "resources", page: Page{dict: Dict{Name("Resources"): Number(1)}}},
		{name: "font", page: Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Number(1)}}}},
		{name: "font entry", page: Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Number(1)}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range (&Document{}).PageFontSeq(test.page) {
				if err == nil {
					t.Fatal("malformed font resource produced a font")
				}
				return
			}
			t.Fatal("malformed font resource produced no error")
		})
	}
}

func TestPageFontSequenceReportsUnresolvedResourceReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Resources"): Ref{Object: 99}}}, want: "playa: Resources could not be resolved"},
		{name: "font", page: Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Ref{Object: 99}}}}, want: "playa: Font resources could not be resolved"},
		{name: "font entry", page: Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Ref{Object: 99}}}}}, want: "playa: font resource \"F1\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range (&Document{}).PageFontSeq(test.page) {
				if err == nil || err.Error() != test.want {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
				return
			}
			t.Fatal("unresolved font resource produced no error")
		})
	}
}

func TestPageFontsReportsUnresolvedResourceReferences(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Resources"): Ref{Object: 99}}}, want: "playa: Resources could not be resolved"},
		{name: "font", page: Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Ref{Object: 99}}}}, want: "playa: Font resources could not be resolved"},
		{name: "font entry", page: Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Ref{Object: 99}}}}}, want: "playa: font resource \"F1\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.page.Fonts(&Document{}); err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPageFontsIncludesFormXObjectResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	p := Page{dict: Dict{
		Name("Resources"): Dict{
			Name("Font"): Dict{Name("FPage"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}},
			Name("XObject"): Dict{Name("X1"): newStream(Dict{
				Name("Subtype"):   Name("Form"),
				Name("Resources"): Dict{Name("Font"): Dict{Name("FForm"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Courier")}}},
			}, []byte("q Q"))},
		},
	}}

	fonts, err := p.Fonts(d)
	if err != nil {
		t.Fatal(err)
	}
	if fonts["FPage"] == nil || fonts["X1/FForm"] == nil {
		t.Fatalf("page fonts = %#v, want direct and Form XObject resources", fonts)
	}
}

func TestPageFontsReportsMalformedNestedFormResources(t *testing.T) {
	tests := []struct {
		name string
		form Stream
	}{
		{name: "form resources", form: newStream(Dict{
			Name("Subtype"): Name("Form"), Name("Resources"): Number(1),
		}, nil)},
		{name: "form font root", form: newStream(Dict{
			Name("Subtype"):   Name("Form"),
			Name("Resources"): Dict{Name("Font"): Number(1)},
		}, nil)},
		{name: "form font entry", form: newStream(Dict{
			Name("Subtype"):   Name("Form"),
			Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Number(1)}},
		}, nil)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &Document{}
			p := Page{dict: Dict{Name("Resources"): Dict{
				Name("XObject"): Dict{Name("Fm"): test.form},
			}}}
			if _, err := p.Fonts(d); err == nil {
				t.Fatalf("malformed nested resource %q produced no error", test.name)
			}
		})
	}
}

func TestPageFontsReportsMalformedXObjectRoot(t *testing.T) {
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Number(1)}}}
	if _, err := p.Fonts(&Document{}); err == nil {
		t.Fatal("malformed XObject root produced no error")
	}
}

func TestPageFontsReportsUnresolvedNestedFontResources(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("Font"): Ref{Object: 99}},
	}, []byte("q Q"))
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}}}}
	if _, err := p.Fonts(&Document{}); err == nil || err.Error() != "playa: Form Font resources at \"Fm/\" could not be resolved" {
		t.Fatalf("unresolved nested font resources error = %v", err)
	}
}

func TestPageFontsReportsUnresolvedFormSubtype(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"):   Ref{Object: 99},
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Courier")}}},
	}, []byte("q Q"))
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}}}}
	if _, err := p.Fonts(&Document{}); err == nil || err.Error() != "playa: Form XObject \"Fm\" subtype could not be resolved" {
		t.Fatalf("unresolved Form subtype error = %v", err)
	}
}

func TestPageTextExpandedReportsMalformedPropertiesResource(t *testing.T) {
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("BT /F1 12 Tf (x) Tj ET")),
		Name("Resources"): Dict{Name("Properties"): Number(1)},
	}}
	if _, err := (&Document{}).PageTextExpanded(p); err == nil {
		t.Fatal("PageTextExpanded silently accepted malformed Properties resource")
	}
}

func TestPageTextExpandedReportsUnresolvedPropertiesResources(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Resources"): Ref{Object: 99}}}, want: "playa: Resources could not be resolved"},
		{name: "properties", page: Page{dict: Dict{Name("Resources"): Dict{Name("Properties"): Ref{Object: 99}}}}, want: "playa: Properties resources could not be resolved"},
		{name: "property entry", page: Page{dict: Dict{Name("Resources"): Dict{Name("Properties"): Dict{Name("P1"): Ref{Object: 99}}}}}, want: "playa: property resource \"P1\" could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := (&Document{}).pagePropertiesChecked(test.page); err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestPagePropertiesCachesTerminalErrors(t *testing.T) {
	ref := Ref{Object: 42}
	d := &Document{pagePropCache: map[Ref]Dict{}}
	p := Page{ref: ref, dict: Dict{
		Name("Resources"): Dict{Name("Properties"): Number(1)},
	}}

	_, firstErr := d.pagePropertiesChecked(p)
	_, secondErr := d.pagePropertiesChecked(p)
	if firstErr == nil || secondErr == nil {
		t.Fatalf("page properties errors = %v, %v; want both non-nil", firstErr, secondErr)
	}
	if firstErr != secondErr {
		t.Fatalf("page properties error was not cached: first=%p second=%p", firstErr, secondErr)
	}
	if got := d.pagePropErrors[ref]; got != firstErr {
		t.Fatalf("cached page properties error = %v, want %v", got, firstErr)
	}
}

func TestPageResourcesCacheSuccessfulResolution(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 9}: Dict{Name("ProcSet"): Array{Name("PDF")}}}}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("Resources"): Ref{Object: 9}}}
	first, err := p.ResourcesWithError(d)
	if err != nil || first == nil {
		t.Fatalf("first page resources = %#v, err=%v", first, err)
	}
	d.objects[Ref{Object: 9}] = Dict{Name("ProcSet"): Array{Name("Image")}}
	second, err := p.ResourcesWithError(d)
	if err != nil || second[Name("ProcSet")].(Array)[0] != Name("PDF") {
		t.Fatalf("cached page resources = %#v, err=%v", second, err)
	}

	uncached := &Document{cacheOptions: cacheconfig.Options{}, cacheOptionsConfigured: true, objects: map[Ref]Object{{Object: 9}: Dict{Name("ProcSet"): Array{Name("PDF")}}}}
	uncachedPage := Page{ref: Ref{Object: 7}, dict: Dict{Name("Resources"): Ref{Object: 9}}}
	if _, err := uncachedPage.ResourcesWithError(uncached); err != nil {
		t.Fatal(err)
	}
	if len(uncached.pageResourceCache) != 0 {
		t.Fatalf("zero-budget resources entered cache: %#v", uncached.pageResourceCache)
	}
}

func TestPageFontSeqReusesValidatedPageResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Font"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("F1"): Ref{Object: 3}},
		{Object: 3}: Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
	}}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("Resources"): Ref{Object: 1}}}
	if _, err := p.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 1}] = Ref{Object: 99}
	for entry, err := range p.FontsSeq(d) {
		if err != nil || entry.name != "F1" {
			t.Fatalf("FontsSeq did not reuse validated resources: %#v, err=%v", entry, err)
		}
		return
	}
	t.Fatal("FontsSeq produced no font")
}

func TestPagePropertiesReusesValidatedPageResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Properties"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("P1"): Dict{Name("MCID"): Number(1)}},
	}}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("Resources"): Ref{Object: 1}}}
	if _, err := p.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 1}] = Ref{Object: 99}
	properties, err := d.pagePropertiesChecked(p)
	if err != nil {
		t.Fatalf("pagePropertiesChecked did not reuse validated resources: %v", err)
	}
	if _, ok := properties[Name("P1")]; !ok {
		t.Fatalf("pagePropertiesChecked omitted cached property: %#v", properties)
	}
}

func TestPageTextExpandedReportsMalformedFormPropertiesResource(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("Properties"): Number(1)},
	}, []byte("BT /F1 12 Tf (x) Tj ET"))
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Fm Do")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Fm"): form}},
	}}
	if _, err := (&Document{}).PageTextExpanded(p); err == nil {
		t.Fatal("PageTextExpanded silently accepted malformed Form Properties resource")
	}
}

func TestPageTextExpandedReportsMalformedColorSpaceResource(t *testing.T) {
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("BT /F1 12 Tf (x) Tj ET")),
		Name("Resources"): Dict{Name("ColorSpace"): Number(1)},
	}}
	if _, err := (&Document{}).PageTextExpanded(p); err == nil {
		t.Fatal("PageTextExpanded silently accepted malformed ColorSpace resource")
	}
}

func TestPageTextExpandedReportsMalformedContentResourceRoots(t *testing.T) {
	for _, key := range []Name{"Font", "ExtGState", "Pattern", "Shading", "Properties", "XObject"} {
		t.Run(string(key), func(t *testing.T) {
			p := Page{dict: Dict{
				Name("Contents"):  newStream(nil, []byte("BT /F1 12 Tf (x) Tj ET")),
				Name("Resources"): Dict{key: Number(1)},
			}}
			if _, err := (&Document{}).PageTextExpanded(p); err == nil {
				t.Fatalf("PageTextExpanded silently accepted malformed %s resource", key)
			}
		})
	}
}

func TestPageTextExpandedReportsMalformedExtGStateEntry(t *testing.T) {
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/GS1 gs BT /F1 12 Tf (x) Tj ET")),
		Name("Resources"): Dict{Name("ExtGState"): Dict{Name("GS1"): Number(1)}},
	}}
	if _, err := (&Document{}).PageTextExpanded(p); err == nil {
		t.Fatal("PageTextExpanded silently accepted malformed ExtGState entry")
	}
}

func TestPageTextExpandedFollowsMultiLevelIndirectExtGStateEntry(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{Name("LW"): Number(3)},
	}}
	p := Page{dict: Dict{
		Name("Contents"): newStream(nil, []byte("/GS gs BT /F 10 Tf (x) Tj ET")),
		Name("Resources"): Dict{
			Name("Font"):      Dict{Name("F"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}},
			Name("ExtGState"): Dict{Name("GS"): Ref{Object: 1}},
		},
	}}
	texts, err := d.PageTextExpanded(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || texts[0].GState().LineWidth() != 3 {
		t.Fatalf("indirect ExtGState = %#v", texts)
	}
}

func TestPageFontsExpandedCachesDirectResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	pageRef := Ref{Object: 3}
	p := Page{ref: pageRef, dict: Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}}},
	}}
	if _, err := p.Fonts(d); err != nil {
		t.Fatal(err)
	}
	if cached := d.pageFontCache[pageRef]; cached == nil || cached["F1"] == nil {
		t.Fatalf("expanded page font cache = %#v, want direct F1 entry", d.pageFontCache)
	}
}

func TestPageFontsDoesNotExposeCachedMap(t *testing.T) {
	fontRef := Ref{Object: 7, Generation: 0}
	d := &Document{
		objects: map[Ref]Object{
			fontRef: Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
		},
		fontCache: map[Ref]*Font{},
	}
	p := Page{ref: Ref{Object: 9}, dict: Dict{
		Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): fontRef}},
	}}
	first, err := d.PageFonts(p)
	if err != nil {
		t.Fatal(err)
	}
	delete(first, "F1")
	second, err := d.PageFonts(p)
	if err != nil {
		t.Fatal(err)
	}
	if second["F1"] == nil {
		t.Fatalf("cached page font map was exposed: %#v", second)
	}
}

func TestPagePropertiesDoesNotExposeCachedDictionary(t *testing.T) {
	ref := Ref{Object: 9}
	d := &Document{
		pagePropCache: map[Ref]Dict{ref: {Name("MCID"): Number(1), Name("Nested"): Dict{Name("Value"): String("original")}}},
	}
	page := Page{ref: ref}
	first := cloneDict(d.pageProperties(page))
	first[Name("MCID")] = Number(2)
	first[Name("Nested")].(Dict)[Name("Value")] = String("changed")
	second := d.pageProperties(page)
	if second[Name("MCID")] != Number(1) {
		t.Fatalf("page property cache was exposed: %#v", second)
	}
	value, _ := second[Name("Nested")].(Dict)[Name("Value")].(String)
	if string(value) != "original" {
		t.Fatalf("nested page property cache was exposed: %#v", second)
	}
}

func TestCloneFontRefsPreservesAbsentMap(t *testing.T) {
	if got := cloneFontRefs(nil); got != nil {
		t.Fatalf("cloned absent font map = %#v, want nil", got)
	}
}

func TestCloneFontMapPreservesAbsentMap(t *testing.T) {
	if got := cloneFontMap(nil); got != nil {
		t.Fatalf("cloned absent resource font map = %#v, want nil", got)
	}
}

func TestPagePropertiesDoesNotRetainOversizedDictionary(t *testing.T) {
	d := &Document{pagePropCache: map[Ref]Dict{}}
	d.storePageProperties(Ref{Object: 9}, Dict{Name("Large"): String(string(make([]byte, pagePropCacheLimit+1)))})
	if len(d.pagePropCache) != 0 || d.pagePropCacheBytes != 0 {
		t.Fatalf("oversized page properties entered cache: entries=%d bytes=%d", len(d.pagePropCache), d.pagePropCacheBytes)
	}
}

func TestPageFontsApplyHelveticaObliqueMetrics(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	d.objects[Ref{Object: 1}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica-Oblique")}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Ref{Object: 1}}}}}
	fonts, err := d.PageFonts(p)
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F1"]
	if f == nil || f.fontBBox != [4]float64{-170, -225, 1116, 931} || f.italicAngle != -12 {
		t.Fatalf("Helvetica-Oblique metrics = %#v", f)
	}
}

func TestPageFontsUseSymbolEncodingByDefault(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	d.objects[Ref{Object: 1}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Symbol")}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Ref{Object: 1}}}}}
	fonts, err := d.PageFonts(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F1"].Decode([]byte("AB")); got != "ΑΒ" {
		t.Fatalf("default Symbol decode = %q", got)
	}
}

func TestPageFontsUseZapfDingbatsEncodingByDefault(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	d.objects[Ref{Object: 1}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("ZapfDingbats")}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Ref{Object: 1}}}}}
	fonts, err := d.PageFonts(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F1"].Decode([]byte{33, 34}); got != "✁✂" {
		t.Fatalf("default ZapfDingbats decode = %q", got)
	}
}

func TestPageFontsMergeNamedUseCMapIntoEmbeddedCMap(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	dict := Dict{
		Name("Subtype"):  Name("Type0"),
		Name("Encoding"): newStream(Dict{Name("UseCMap"): Name("UniJIS-UCS2-H")}, []byte("1 begincodespacerange\n<0000> <ffff>\nendcodespacerange\n1 begincidchar\n<4e2d> 7\nendcidchar")),
	}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): dict}}}}
	fonts, err := d.PageFonts(p)
	if err != nil {
		t.Fatal(err)
	}
	cmap := fonts["F1"].cmap
	if cmap == nil || cmap.MappingCopy()[string([]byte{0x4e, 0x2d})] != 7 {
		t.Fatalf("local CMap mapping = %#v", cmap)
	}
	base, err := loadPredefinedCMap("UniJIS-UCS2-H")
	if err != nil {
		t.Fatal(err)
	}
	if base.MappingCopy()[string([]byte{0x4e, 0x2d})] == cmap.MappingCopy()[string([]byte{0x4e, 0x2d})] {
		t.Fatalf("named UseCMap did not override base mapping: base=%d merged=%d", base.MappingCopy()[string([]byte{0x4e, 0x2d})], cmap.MappingCopy()[string([]byte{0x4e, 0x2d})])
	}
}

func TestEmbeddedCMapResolvesNestedUseCMap(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	cmap := d.loadUseCMap(newStream(nil, []byte("/UniJIS-UCS2-H usecmap\n1 begincodespacerange\n<0000> <ffff>\nendcodespacerange\n1 begincidchar\n<4e2d> 7\nendcidchar")))
	if cmap == nil || cmap.MappingCopy()[string([]byte{0x4e, 0x2d})] != 7 {
		t.Fatalf("nested CMap mapping = %#v", cmap)
	}
	if len(cmap.MappingCopy()) < 9000 {
		t.Fatalf("nested CMap did not inherit predefined mappings: %d entries", len(cmap.MappingCopy()))
	}
}

func TestPageFontByNameParsesOnlyRequestedFont(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	d.objects[Ref{Object: 1}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}
	d.objects[Ref{Object: 2}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Courier")}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{
		Name("F1"): Ref{Object: 1}, Name("F2"): Ref{Object: 2},
	}}}}
	font := d.pageFontByName(p, "F1")
	if font == nil || font.name != "Helvetica" {
		t.Fatalf("font = %#v", font)
	}
	if len(d.fontCache) != 1 {
		t.Fatalf("font cache size = %d, want 1", len(d.fontCache))
	}
}

func TestPageFontsConcurrentSharesIndirectFontBuild(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	d.objects[Ref{Object: 1}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Ref{Object: 1}, Name("F2"): Ref{Object: 1}}}}}
	const readers = 12
	start := make(chan struct{})
	fonts := make(chan *Font, readers)
	errs := make(chan error, readers)
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			resolved, err := d.pageFontsDirect(p)
			if err != nil {
				errs <- err
				return
			}
			if resolved["F1"] != resolved["F2"] {
				errs <- fmt.Errorf("font aliases were not shared")
				return
			}
			fonts <- resolved["F1"]
		}()
	}
	close(start)
	wg.Wait()
	close(fonts)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var first *Font
	for font := range fonts {
		if font == nil {
			t.Fatal("concurrent font build returned nil")
		}
		if first == nil {
			first = font
		} else if font != first {
			t.Fatal("concurrent font build returned distinct cached pointers")
		}
	}
	if len(d.fontBuilds) != 0 {
		t.Fatalf("font builds remain in flight: %d", len(d.fontBuilds))
	}
}

func TestFontBuildWaiterReusesUncachedCompletedFont(t *testing.T) {
	d := &Document{fontCache: map[Ref]*Font{}, fontBuilds: map[Ref]*fontBuild{}}
	ref := Ref{Object: 1}
	_, build, owner := d.acquireFontBuild(ref)
	if !owner || build == nil {
		t.Fatal("font build was not reserved")
	}
	_, waiterBuild, waiterOwner := d.acquireFontBuild(ref)
	if waiterOwner || waiterBuild != build {
		t.Fatalf("font waiter did not join build: owner=%v build=%p want=%p", waiterOwner, waiterBuild, build)
	}
	font := NewSimpleFont("oversized")
	font.toUnicodeData = bytes.Repeat([]byte{'x'}, fontCacheLimit+1)
	d.finishFontBuild(ref, font, nil)

	<-waiterBuild.done
	if waiterBuild.font != font || waiterBuild.err != nil {
		t.Fatalf("completed font result = font=%p err=%v", waiterBuild.font, waiterBuild.err)
	}
}

func TestPageFontsCachesTerminalIndirectFontErrors(t *testing.T) {
	ref := Ref{Object: 1}
	d := &Document{
		objects:   map[Ref]Object{ref: Number(7)},
		fontCache: map[Ref]*Font{},
	}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): ref}}}}

	_, firstErr := d.pageFontsDirect(p)
	_, secondErr := d.pageFontsDirect(p)
	if firstErr == nil || secondErr == nil {
		t.Fatalf("font errors = %v, %v; want both non-nil", firstErr, secondErr)
	}
	if firstErr != secondErr {
		t.Fatalf("font error was not cached: first=%p second=%p", firstErr, secondErr)
	}
	if got := d.fontErrors[ref]; got != firstErr {
		t.Fatalf("cached font error = %v, want %v", got, firstErr)
	}
}

func TestPageFontsHonorErrorCacheBudget(t *testing.T) {
	ref := Ref{Object: 1}
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions:           cacheconfig.Options{FontErrorBytes: 0},
		objects:                map[Ref]Object{ref: Number(7)},
		fontCache:              map[Ref]*Font{},
	}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): ref}}}}
	_, firstErr := d.pageFontsDirect(p)
	_, secondErr := d.pageFontsDirect(p)
	if firstErr == nil || secondErr == nil || firstErr == secondErr {
		t.Fatalf("font errors = %v/%v, want uncached independent errors", firstErr, secondErr)
	}
	if len(d.fontErrors) != 0 {
		t.Fatalf("font error cache retained entry with zero budget: %#v", d.fontErrors)
	}
}

func TestPageFontSequenceResolvesFontsOnDemand(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	d.objects[Ref{Object: 1}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}
	d.objects[Ref{Object: 2}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Courier")}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{
		Name("F1"): Ref{Object: 1}, Name("F2"): Ref{Object: 2},
	}}}}

	count := 0
	for entry, err := range d.PageFontSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		if entry.name != "F1" || entry.font == nil {
			t.Fatalf("font entry = %#v", entry)
		}
		count++
		break
	}
	if count != 1 || len(d.fontCache) != 1 {
		t.Fatalf("first font count = %d, cache = %d", count, len(d.fontCache))
	}
}

func TestPageFontSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, fontCache: map[Ref]*Font{}}
	d.objects[Ref{Object: 1}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}
	d.objects[Ref{Object: 2}] = Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Courier")}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{
		Name("F1"): Ref{Object: 1}, Name("F2"): Ref{Object: 2},
	}}}}

	first := 0
	for entry, err := range d.PageFontSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		if entry.name != "F1" || entry.font == nil {
			t.Fatalf("first font entry = %#v", entry)
		}
		first++
		break
	}
	second := 0
	for entry, err := range d.PageFontSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		if entry.name != "F1" && entry.name != "F2" {
			t.Fatalf("repeated font entry = %#v", entry)
		}
		second++
	}
	if first != 1 || second != 2 {
		t.Fatalf("font sequence counts = first %d, second %d", first, second)
	}
}

func TestPageFontSequencePopulatesPageCache(t *testing.T) {
	fontRef := Ref{Object: 7, Generation: 0}
	d := &Document{objects: map[Ref]Object{
		fontRef: Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
	}, fontCache: map[Ref]*Font{}}
	p := Page{ref: Ref{Object: 1}, dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): fontRef}}}}
	for _, err := range d.PageFontSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if d.pageFontCache[p.ref]["F1"] == nil {
		t.Fatal("page font cache was not populated")
	}
}

func TestPageFontSequenceExcludesFormFonts(t *testing.T) {
	form := newStream(Dict{
		Name("Subtype"):   Name("Form"),
		Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Courier")}}},
	}, nil)
	d := &Document{}
	p := Page{dict: Dict{
		Name("Resources"): Dict{
			Name("Font"):    Dict{Name("P"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")}},
			Name("XObject"): Dict{Name("Fm"): form},
		},
	}}
	var names []string
	for entry, err := range d.PageFontSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, entry.name)
	}
	if len(names) != 1 || names[0] != "P" {
		t.Fatalf("page font names = %#v", names)
	}
}

func TestDocumentFontsUsesLaterPageAndFontOrder(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}, Ref{Object: 4}}},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Resources"): Dict{Name("Font"): Dict{
				Name("F1"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
			}}},
			{Object: 4}: Dict{Name("Type"): Name("Page"), Name("Resources"): Dict{Name("Font"): Dict{
				Name("F2"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Courier")},
			}}},
		},
		fontCache: map[Ref]*Font{},
	}
	var names []string
	for entry, err := range d.FontsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, entry.name)
	}
	if len(names) != 2 || names[0] != "Courier" || names[1] != "Helvetica" {
		t.Fatalf("document font names = %#v", names)
	}
	fonts, err := d.Fonts()
	if err != nil || fonts["Courier"] == nil || fonts["Helvetica"] == nil {
		t.Fatalf("document fonts = %#v, err=%v", fonts, err)
	}
}

func TestDocumentFontsPreservesSourceFontOrderForNameCollisions(t *testing.T) {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /Resources << /Font << /F3 4 0 R /F1 5 0 R >> >> /MediaBox [0 0 10 10] >>\nendobj\n",
		"4 0 obj\n<< /Type /Font /Subtype /Type0 /BaseFont /Shared /Encoding /Identity-H /DescendantFonts [6 0 R] >>\nendobj\n",
		"5 0 obj\n<< /Type /Font /Subtype /TrueType /BaseFont /Shared >>\nendobj\n",
		"6 0 obj\n<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Shared /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> >>\nendobj\n",
	}
	pdf := "%PDF-1.7\n"
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = len(pdf)
		pdf += object
	}
	xref := len(pdf)
	pdf += fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		pdf += fmt.Sprintf("%010d 00000 n \n", offset)
	}
	pdf += fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)

	d, err := OpenBytes([]byte(pdf))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Errorf("close document: %v", err)
		}
	}()
	fonts, err := d.Fonts()
	if err != nil {
		t.Fatal(err)
	}
	font := fonts["Shared"]
	if font == nil || font.IsCID() {
		t.Fatalf("Shared winner = %#v, want later source resource F1 TrueType", font)
	}
}

func TestDocumentFontsTraversesPagePatternResources(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Resources"): Dict{
				Name("Font"):    Dict{Name("F1"): Ref{Object: 4}},
				Name("Pattern"): Dict{Name("P1"): Ref{Object: 5}},
			}},
			{Object: 4}: Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
			{Object: 5}: newStream(Dict{Name("Resources"): Dict{Name("Font"): Dict{
				Name("F2"): Ref{Object: 6},
			}}}, nil),
			{Object: 6}: Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Times-Italic")},
		},
		fontCache: map[Ref]*Font{},
	}
	fonts, err := d.Fonts()
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 2 || fonts["Helvetica"] == nil || fonts["Times-Italic"] == nil {
		t.Fatalf("document fonts = %#v, want page and Pattern fonts", fonts)
	}
}

func TestPageFontReverseSequenceIsLazyAndReversed(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}, pageFontCache: map[Ref]map[string]*Font{}}
	p := Page{ref: Ref{Object: 20}, dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{
		Name("A"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
		Name("B"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Courier")},
	}}}}
	var names []string
	for entry, err := range d.pageFontSeq(p, true) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, entry.name)
		break
	}
	if len(names) != 1 || names[0] != "B" {
		t.Fatalf("reverse first font = %#v", names)
	}
	if cached := d.pageFontCache[p.ref]; cached != nil {
		if _, ok := cached["A"]; ok {
			t.Fatalf("reverse sequence resolved an unconsumed font: %#v", cached)
		}
	}
}

func TestPageFontsDoesNotExposeCachedFontPointers(t *testing.T) {
	ref := Ref{Object: 9}
	d := &Document{
		pageFontCache: map[Ref]map[string]*Font{
			ref: {"F1": {name: "Helvetica", defaultWidth: 1000}},
		},
	}
	p := Page{ref: ref}
	first, err := d.PageFonts(p)
	if err != nil || first["F1"] == nil {
		t.Fatalf("first page fonts = %#v, err=%v", first, err)
	}
	first["F1"].name = "Changed"
	first["F1"].defaultWidth = 1
	second, err := d.PageFonts(p)
	if err != nil || second["F1"].name != "Helvetica" || second["F1"].defaultWidth != 1000 {
		t.Fatalf("cached font pointer was exposed = %#v, err=%v", second, err)
	}
}

func TestPageFontsSnapshotToleratesNilCachedFont(t *testing.T) {
	ref := Ref{Object: 10}
	d := &Document{pageFontCache: map[Ref]map[string]*Font{
		ref: {"Missing": nil},
	}}
	fonts, err := d.PageFonts(Page{ref: ref})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fonts["Missing"]; !ok || fonts["Missing"] != nil {
		t.Fatalf("nil cached font snapshot = %#v", fonts)
	}
}

func TestPageFontByNameCachesNestedFontResolution(t *testing.T) {
	ref := Ref{Object: 12}
	d := &Document{pageFontCache: map[Ref]map[string]*Font{}}
	p := Page{ref: ref, dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{
		Name("F1"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
	}}}}
	first := d.pageFontByName(p, "F1")
	second := d.pageFontByName(p, "F1")
	if first == nil || second != first {
		t.Fatalf("page font cache = first=%p second=%p", first, second)
	}
	if got := d.pageFontCache[ref]["F1"]; got != first {
		t.Fatalf("cached page font = %p, want %p", got, first)
	}
}

func TestDocumentFontsDoesNotExposeCachedFontPointers(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Resources"): Dict{Name("Font"): Dict{
				Name("F1"): Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Helvetica")},
			}}},
		},
		fontCache: map[Ref]*Font{},
	}
	first, err := d.Fonts()
	if err != nil || first["Helvetica"] == nil {
		t.Fatalf("first document fonts = %#v, err=%v", first, err)
	}
	first["Helvetica"].name = "Changed"
	second, err := d.Fonts()
	if err != nil || second["Helvetica"].name != "Helvetica" {
		t.Fatalf("document font cache was exposed = %#v, err=%v", second, err)
	}
}
