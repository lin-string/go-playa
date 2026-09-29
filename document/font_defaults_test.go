package document

import (
	"encoding/binary"
	"strings"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestLoadPredefinedCMapUsesAdobeIdentityResources(t *testing.T) {
	cmap, err := loadPredefinedCMap("Identity-V")
	if err != nil {
		t.Fatal(err)
	}
	if !cmap.Vertical() {
		t.Fatal("Identity-V is not vertical")
	}
	decoded := cmap.Decode([]byte{0x01, 0x02})
	if len(decoded) != 1 || decoded[0].CID() != 0x0102 {
		t.Fatalf("Identity-V decode = %#v", decoded)
	}

	decodedBytes := decoded[0].BytesCopy()
	decodedBytes[0] = 0xff
	again, err := loadPredefinedCMap("Identity-V")
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Decode([]byte{0x01, 0x02})[0].CID(); got != 0x0102 {
		t.Fatalf("cached Identity-V was mutated, CID = %d", got)
	}
}

func TestFontExposesPlayaIdentityMetadata(t *testing.T) {
	font := NewSimpleFont("resource")
	font.name = "DescendantFont"
	font.baseName = "RootFont-Identity-H"
	font.cidCoding = "Adobe-Japan1"

	if got := font.Name(); got != "DescendantFont" {
		t.Fatalf("font name = %q, want DescendantFont", got)
	}
	if got := font.BaseFont(); got != "RootFont-Identity-H" {
		t.Fatalf("base font = %q, want RootFont-Identity-H", got)
	}
	if got := font.CIDCoding(); got != "Adobe-Japan1" {
		t.Fatalf("CID coding = %q, want Adobe-Japan1", got)
	}
}

func TestPredefinedCMapCacheUsesImmutableInstance(t *testing.T) {
	first, err := LoadPredefinedCMap("Identity-H")
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadPredefinedCMap("Identity-H")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("predefined CMap cache did not reuse its immutable instance")
	}
	snapshot := first.Finalize()
	snapshotMapping := snapshot.MappingCopy()
	snapshotMapping["\x00\x01"] = 99
	if first.MappingCopy()["\x00\x01"] == 99 {
		t.Fatal("CMap Finalize exposed cached mapping")
	}
}

func TestPredefinedCMapCacheSupportsConcurrentLoads(t *testing.T) {
	names := []string{"UniJIS-UCS2-H", "UniGB-UCS2-H", "UniCNS-UCS2-H", "UniKS-UCS2-H"}
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			const callers = 8
			results := make([]*CMap, callers)
			errs := make([]error, callers)
			var wg sync.WaitGroup
			wg.Add(callers)
			for i := range results {
				go func(i int) {
					defer wg.Done()
					results[i], errs[i] = loadPredefinedCMap(name)
				}(i)
			}
			wg.Wait()
			for i, cmap := range results {
				if errs[i] != nil {
					t.Fatalf("load %q: %v", name, errs[i])
				}
				if cmap == nil {
					t.Fatalf("load %q returned nil CMap", name)
				}
				if i > 0 && cmap != results[0] {
					t.Fatalf("load %q returned multiple cached instances", name)
				}
			}
		})
	}
}

func TestUseCMapReturnsDocumentOwnedCopy(t *testing.T) {
	canonical, err := loadPredefinedCMap("Identity-H")
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{}
	owned := d.loadUseCMap(Name("Identity-H"))
	if owned == nil {
		t.Fatal("UseCMap returned nil")
	}
	if owned == canonical {
		t.Fatal("UseCMap attached the shared predefined instance")
	}
	ownedMapping := owned.MappingCopy()
	ownedMapping[string([]byte{0x00, 0x01})] = 99
	if canonical.MappingCopy()[string([]byte{0x00, 0x01})] == 99 {
		t.Fatal("UseCMap copy shares the predefined mapping")
	}
}

func TestIdentityCMapAliasesPreserveByteWidth(t *testing.T) {
	for name, wantVertical := range map[string]bool{
		"DLIdent-H": false, "DLIdent-V": true,
		"OneByteIdentityH": false, "OneByteIdentityV": true,
	} {
		cmap, err := loadPredefinedCMap(name)
		if err != nil || cmap.Vertical() != wantVertical {
			t.Fatalf("identity alias %q = %#v, err=%v", name, cmap, err)
		}
		data := []byte{0x01, 0x02}
		decoded := cmap.Decode(data)
		want := 1
		if name == "DLIdent-H" || name == "DLIdent-V" {
			want = 0x0102
		}
		if len(decoded) != 2 && name != "DLIdent-H" && name != "DLIdent-V" {
			t.Fatalf("one-byte alias %q decoded = %#v", name, decoded)
		}
		if len(decoded) == 0 || decoded[0].CID() != want {
			t.Fatalf("identity alias %q first CID = %#v, want %d", name, decoded, want)
		}
	}
}

func TestLoadPredefinedCMapUsesAdobeCJKResources(t *testing.T) {
	entries, err := predefinedCMapFiles.ReadDir("cmapdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".cmap")
		if entry.IsDir() {
			continue
		}
		cmap, err := loadPredefinedCMap(name)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if len(cmap.CodespacesCopy()) == 0 || len(cmap.MappingCopy()) == 0 {
			t.Fatalf("%s has no parsed mapping: %#v", name, cmap)
		}
	}
}

func TestLoadPredefinedCMapUsesAdobeCJKResourceExamples(t *testing.T) {
	for _, name := range []string{
		"UniJIS-UCS2-H", "UniJIS-UCS2-V",
		"UniJIS-UTF16-H", "UniJIS-UTF16-V",
		"UniJIS-UTF8-H", "UniJIS-UTF8-V", "UniJIS-UTF32-H", "UniJIS-UTF32-V",
		"UniJIS2004-UTF8-H", "UniJIS2004-UTF8-V",
		"UniJIS2004-UTF16-H", "UniJIS2004-UTF16-V",
		"UniJIS2004-UTF32-H", "UniJIS2004-UTF32-V",
		"UniJISX0213-UTF32-H", "UniJISX0213-UTF32-V",
		"UniJISX02132004-UTF32-H", "UniJISX02132004-UTF32-V",
		"UniJISPro-UCS2-HW-V", "UniJISPro-UCS2-V", "UniJISPro-UTF8-V",
		"Adobe-Japan1-0", "Adobe-Japan1-1", "Adobe-Japan1-2", "Adobe-Japan1-3", "Adobe-Japan1-4", "Adobe-Japan1-5", "Adobe-Japan1-6", "Adobe-Japan1-7",
		"Adobe-GB1-0", "Adobe-GB1-1", "Adobe-GB1-2", "Adobe-GB1-3", "Adobe-GB1-4", "Adobe-GB1-5", "Adobe-GB1-6",
		"Adobe-CNS1-0", "Adobe-CNS1-1", "Adobe-CNS1-2", "Adobe-CNS1-3", "Adobe-CNS1-4", "Adobe-CNS1-5", "Adobe-CNS1-6", "Adobe-CNS1-7",
		"Adobe-Korea1-0", "Adobe-Korea1-1", "Adobe-Korea1-2",
		"UniGB-UCS2-H", "UniGB-UCS2-V",
		"UniGB-UTF16-H", "UniGB-UTF16-V",
		"UniGB-UTF8-H", "UniGB-UTF8-V", "UniGB-UTF32-H", "UniGB-UTF32-V",
		"GB-EUC-H", "GB-EUC-V",
		"UniCNS-UCS2-H", "UniCNS-UCS2-V",
		"UniCNS-UTF16-H", "UniCNS-UTF16-V",
		"UniCNS-UTF8-H", "UniCNS-UTF8-V", "UniCNS-UTF32-H", "UniCNS-UTF32-V",
		"CNS-EUC-H", "CNS-EUC-V",
		"UniKS-UCS2-H", "UniKS-UCS2-V",
		"UniKS-UTF16-H", "UniKS-UTF16-V",
		"UniKS-UTF8-H", "UniKS-UTF8-V", "UniKS-UTF32-H", "UniKS-UTF32-V",
		"KSC-EUC-H", "KSC-EUC-V",
		"90ms-RKSJ-H", "90ms-RKSJ-V",
	} {
		cmap, err := loadPredefinedCMap(name)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if wantVertical := strings.HasSuffix(name, "-V"); cmap.Vertical() != wantVertical {
			t.Fatalf("%s vertical = %v, want %v", name, cmap.Vertical(), wantVertical)
		}
		if len(cmap.CodespacesCopy()) == 0 || len(cmap.MappingCopy()) == 0 {
			t.Fatalf("%s has no parsed mapping: %#v", name, cmap)
		}
	}
}

func TestPredefinedUTF16CMapDecodesSupplementaryCode(t *testing.T) {
	cmap, err := loadPredefinedCMap("UniJIS-UTF16-H")
	if err != nil {
		t.Fatal(err)
	}
	got := cmap.Decode([]byte{0xd8, 0x2c, 0xdd, 0x32})
	if len(got) != 1 || got[0].CID() != 12269 {
		t.Fatalf("UTF16 supplementary decode = %#v", got)
	}
}

func TestPredefinedUTF8AndUTF32CMapsDecodeSupplementaryCode(t *testing.T) {
	cases := map[string][]byte{
		"UniJIS-UTF8-H":  {0xf0, 0x9b, 0x84, 0xb2},
		"UniJIS-UTF32-H": {0x00, 0x01, 0xb1, 0x32},
	}
	for name, data := range cases {
		cmap, err := loadPredefinedCMap(name)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		got := cmap.Decode(data)
		if len(got) != 1 || got[0].CID() != 12269 {
			t.Fatalf("%s supplementary decode = %#v", name, got)
		}
	}
}

func TestNewSimpleFontUsesPlayaDefaultMetrics(t *testing.T) {
	f := NewSimpleFont("F")
	if f.ascent != 880 || f.descent != -120 {
		t.Fatalf("default metrics = ascent %v descent %v", f.ascent, f.descent)
	}
}

func TestCore14Type1UsesBuiltInMetricsInsteadOfDescriptor(t *testing.T) {
	d := &Document{}
	font, err := d.GetFontWithError(0, Dict{
		Name("Subtype"):  Name("Type1"),
		Name("BaseFont"): Name("Helvetica"),
		Name("FontDescriptor"): Dict{
			Name("Ascent"):       Number(850),
			Name("Descent"):      Number(-220),
			Name("FontBBox"):     Array{Number(-100), Number(-200), Number(900), Number(850)},
			Name("Flags"):        Number(32),
			Name("Leading"):      Number(24),
			Name("CapHeight"):    Number(700),
			Name("ItalicAngle"):  Number(-12),
			Name("StemV"):        Number(80),
			Name("MissingWidth"): Number(321),
		},
		Name("FirstChar"): Number(65),
		Name("Widths"):    Array{Number(123)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if font == nil {
		t.Fatal("Helvetica font was not resolved")
	}
	bbox, hasBBox := font.FontBBox()
	flags, hasFlags := font.Flags()
	if font.Ascent() != 718 || font.Descent() != -207 || !hasBBox || bbox != [4]float64{-166, -225, 1000, 931} ||
		!hasFlags || flags != 0 || font.Leading() != 0 || font.CapHeight() != 718 || font.ItalicAngle() != 0 ||
		font.StemV() != 0 || font.DefaultWidth() != 1000 || font.HDisp(65) != 0.667 {
		t.Fatalf("Helvetica retained PDF descriptor state: %#v", font)
	}
}

func TestCore14Type1RecognitionMatchesPlayaNames(t *testing.T) {
	tests := []struct {
		name        string
		subtype     Name
		ascent      float64
		canonical   string
		italicAngle float64
	}{
		{name: "Helvetica", subtype: Name("Type1"), ascent: 718, canonical: "Helvetica"},
		{name: "Helvetica", subtype: Name("MMType1"), ascent: 718, canonical: "Helvetica"},
		{name: "Arial,Bold", subtype: Name("Type1"), ascent: 718, canonical: "Helvetica-Bold"},
		{name: "CourierNew", subtype: Name("Type1"), ascent: 629, canonical: "Courier"},
		{name: "TimesNewRoman,Italic", subtype: Name("Type1"), ascent: 683, canonical: "Times-Italic", italicAngle: -15.5},
		{name: "Symbol", subtype: Name("Type1"), ascent: 880, canonical: "Symbol"},
		{name: "ZapfDingbats", subtype: Name("Type1"), ascent: 880, canonical: "ZapfDingbats"},
		{name: "ArialMT", subtype: Name("Type1"), ascent: 850, canonical: "ArialMT", italicAngle: -7},
		{name: "ABCDEF+Helvetica", subtype: Name("Type1"), ascent: 850, canonical: "ABCDEF+Helvetica", italicAngle: -7},
		{name: "Custom", subtype: Name("Type1"), ascent: 850, canonical: "Custom", italicAngle: -7},
	}
	for _, test := range tests {
		t.Run(test.name+"/"+string(test.subtype), func(t *testing.T) {
			font, err := (&Document{}).GetFontWithError(0, Dict{
				Name("Subtype"):        test.subtype,
				Name("BaseFont"):       Name(test.name),
				Name("FontDescriptor"): Dict{Name("Ascent"): Number(850), Name("ItalicAngle"): Number(-7)},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := font.Ascent(); got != test.ascent {
				t.Fatalf("ascent = %v, want %v", got, test.ascent)
			}
			if font.Name() != test.canonical || font.BaseFont() != test.canonical || font.ItalicAngle() != test.italicAngle {
				t.Fatalf("identity = name %q base %q italic %v, want %q/%q/%v", font.Name(), font.BaseFont(), font.ItalicAngle(), test.canonical, test.canonical, test.italicAngle)
			}
		})
	}
}

func TestSimpleFontLeavesPlayaSubtypeUnset(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{
		Name("Subtype"): Name("TrueType"), Name("BaseFont"): Name("Helvetica"),
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F"].subtype; got != "" {
		t.Fatalf("TrueType subtype = %q, want empty", got)
	}
}

func TestTrueTypeHelveticaDoesNotReceiveBuiltInDescriptor(t *testing.T) {
	font, err := (&Document{}).GetFontWithError(0, Dict{
		Name("Subtype"):  Name("TrueType"),
		Name("BaseFont"): Name("Helvetica"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if flags, hasFlags := font.Flags(); hasFlags || flags != 0 || font.CapHeight() != 0 {
		t.Fatalf("TrueType Helvetica received built-in descriptor fields: flags=%d present=%v capHeight=%v", flags, hasFlags, font.CapHeight())
	}
}

func TestSymbolicTrueTypeWithoutEncodingDoesNotUseStandardEncoding(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{
		Name("Subtype"):        Name("TrueType"),
		Name("BaseFont"):       Name("Custom"),
		Name("FontDescriptor"): Dict{Name("Flags"): Number(4)},
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil || !f.strictEncoding {
		t.Fatalf("symbolic TrueType encoding state = %#v", f)
	}
	if _, ok := f.EncodingValue('A'); ok {
		t.Fatal("symbolic TrueType unexpectedly received StandardEncoding")
	}
}

func TestType3WithoutEncodingDoesNotUseStandardEncoding(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{
		Name("Subtype"):   Name("Type3"),
		Name("CharProcs"): Dict{Name("A"): newStream(nil, []byte("0 0 d0"))},
		Name("Widths"):    Array{Number(500)},
		Name("FirstChar"): Number(65),
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil || !f.strictEncoding {
		t.Fatalf("Type3 encoding state = %#v", f)
	}
	if _, ok := f.EncodingValue('A'); ok {
		t.Fatal("Type3 without Encoding unexpectedly received StandardEncoding")
	}
}

func TestType3WithoutEncodingUsesPlayaStandardDecodeFallback(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{
		Name("Subtype"):   Name("Type3"),
		Name("CharProcs"): Dict{Name("A"): newStream(nil, []byte("0 0 d0"))},
		Name("Widths"):    Array{Number(500)},
		Name("FirstChar"): Number(65),
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F"].Decode([]byte{'A', '`', 0x80, 0xa1, 0xb2}); got != "A‘¡†" {
		t.Fatalf("Type3 missing Encoding decode = %q, want StandardEncoding fallback", got)
	}
}

func TestType3EncodingDifferencesDoNotInheritASCII(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{
		Name("Subtype"):  Name("Type3"),
		Name("Encoding"): Dict{Name("Differences"): Array{Number(65), Name("A")}},
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if _, ok := f.EncodingValue('A'); ok {
		t.Fatal("Type3 Differences glyph unexpectedly entered Unicode encoding")
	}
	if got := f.Decode([]byte{'A'}); got != "A" {
		t.Fatalf("Type3 Differences decode = %q, want A", got)
	}
	if got := f.Decode([]byte{'B'}); got != "" {
		t.Fatalf("unmapped Type3 code inherited text %q", got)
	}
}

func TestType3FontNameUsesDescriptorBeforeResourceName(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F0"): Dict{
		Name("Subtype"):        Name("Type3"),
		Name("Name"):           Name("DictionaryName"),
		Name("FontDescriptor"): Dict{Name("FontName"): Name("DescriptorName")},
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F0"].name; got != "DescriptorName" {
		t.Fatalf("Type3 font name = %q, want DescriptorName", got)
	}

	fonts, err = d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{
		Name("Subtype"): Name("Type3"),
		Name("Name"):    Name("DictionaryName"),
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F1"].name; got != "DictionaryName" {
		t.Fatalf("Type3 dictionary font name = %q, want DictionaryName", got)
	}

	fonts, err = d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F2"): Dict{
		Name("Subtype"): Name("Type3"),
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F2"].name; got != "unknown" {
		t.Fatalf("unnamed Type3 font name = %q, want unknown", got)
	}
}

func TestSimpleFontNameUsesDescriptorBeforeBaseFont(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F0"): Dict{
		Name("Subtype"):        Name("Type1"),
		Name("BaseFont"):       Name("ABCDEF+TimesNewRoman"),
		Name("FontDescriptor"): Dict{Name("FontName"): Name("TimesNewRoman")},
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F0"].Name(); got != "TimesNewRoman" {
		t.Fatalf("simple font name = %q, want descriptor FontName", got)
	}
	if got := fonts["F0"].BaseFont(); got != "ABCDEF+TimesNewRoman" {
		t.Fatalf("simple font BaseFont = %q, want PDF BaseFont identity", got)
	}
}

func TestType0FontNameUsesDescendantDescriptor(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("Subtype"):         Name("Type0"),
			Name("BaseFont"):        Name("SimSun-4235-Identity-H"),
			Name("DescendantFonts"): Array{Ref{Object: 2}},
		},
		{Object: 2}: Dict{
			Name("Subtype"):  Name("CIDFontType2"),
			Name("BaseFont"): Name("SimSun-4235"),
			Name("CIDSystemInfo"): Dict{
				Name("Registry"): Ref{Object: 4},
				Name("Ordering"): Ref{Object: 5},
			},
			Name("FontDescriptor"): Ref{Object: 3},
		},
		{Object: 3}: Dict{Name("FontName"): Name("SimSun-4235")},
		{Object: 4}: String([]byte("Adobe")),
		{Object: 5}: String([]byte("Japan1")),
	}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{
		Name("Font"): Dict{Name("F"): Ref{Object: 1}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F"].name; got != "SimSun-4235" {
		t.Fatalf("Type0 font name = %q, want descendant descriptor name", got)
	}
	if got := fonts["F"].BaseFont(); got != "SimSun-4235" {
		t.Fatalf("Type0 base font = %q, want descendant BaseFont", got)
	}
	if got := fonts["F"].CIDCoding(); got != "Adobe-Japan1" {
		t.Fatalf("Type0 CID coding = %q, want Adobe-Japan1", got)
	}
}

func TestFontNamesAcceptStringObjects(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{
		Name("Subtype"):        Name("Type3"),
		Name("Name"):           String([]byte("DictionaryName")),
		Name("FontDescriptor"): Dict{Name("FontName"): String([]byte("DescriptorName"))},
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F"].name; got != "DescriptorName" {
		t.Fatalf("string Type3 font name = %q, want DescriptorName", got)
	}

	fonts, err = d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F1"): Dict{
		Name("Subtype"):  Name("Type1"),
		Name("BaseFont"): String([]byte("StringBaseFont")),
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F1"].name; got != "StringBaseFont" {
		t.Fatalf("string BaseFont = %q, want StringBaseFont", got)
	}
}

func TestType0VerticalDefaultsWithoutDW2(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	fontSpec := Dict{
		Name("Subtype"):         Name("Type0"),
		Name("BaseFont"):        Name("Test"),
		Name("Encoding"):        Name("Identity-V"),
		Name("DescendantFonts"): Array{Ref{Object: 2}},
	}
	child := Dict{Name("Subtype"): Name("CIDFontType2")}
	d.objects[Ref{Object: 1}] = fontSpec
	d.objects[Ref{Object: 2}] = child

	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil || f.subtype != "CIDFontType2" || f.defaultVPosition != [2]float64{500, 880} || f.defaultVWidth != -1000 {
		t.Fatalf("vertical defaults = position %v width %v", f.defaultVPosition, f.defaultVWidth)
	}
}

func TestType0VerticalDefaultsIgnoreMalformedDW2(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"): Name("Type0"), Name("Encoding"): Name("Identity-V"),
		Name("DescendantFonts"): Array{Ref{Object: 2}},
	}
	d.objects[Ref{Object: 2}] = Dict{
		Name("Subtype"): Name("CIDFontType2"),
		Name("DW2"):     Array{String("invalid"), Number(-700)},
	}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil || f.defaultVPosition != [2]float64{500, 880} || f.defaultVWidth != -1000 {
		t.Fatalf("malformed DW2 defaults = position %v width %v", f.defaultVPosition, f.defaultVWidth)
	}
}

func TestType0VerticalDefaultsIgnoreDW2WithExtraValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("Subtype"):         Name("Type0"),
			Name("Encoding"):        Name("Identity-V"),
			Name("DescendantFonts"): Array{Ref{Object: 2}},
		},
		{Object: 2}: Dict{
			Name("Subtype"): Name("CIDFontType2"),
			Name("DW2"):     Array{Number(880), Number(-700), Number(20)},
		},
	}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil || f.defaultVPosition != [2]float64{500, 880} || f.defaultVWidth != -1000 {
		t.Fatalf("DW2 with extra values was accepted = position %v width %v", f.defaultVPosition, f.defaultVWidth)
	}
}

func TestType0CIDWidthsIgnoreIndirectEntriesLikePlaya(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 3}] = Array{Number(600)}
	d.objects[Ref{Object: 2}] = Dict{
		Name("Subtype"): Name("CIDFontType2"),
		Name("DW"):      Number(1000),
		Name("W"):       Array{Number(1), Ref{Object: 3}},
	}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):         Name("Type0"),
		Name("Encoding"):        Name("Identity-H"),
		Name("DescendantFonts"): Array{Ref{Object: 2}},
	}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F"].WidthCode([]byte{0, 1}); got != 1000 {
		t.Fatalf("indirect CID width = %v, want Playa fallback 1000", got)
	}
}

func TestCIDWidthRangesRejectOutOfRangeCIDValues(t *testing.T) {
	f := &Font{cidWidths: map[int]float64{}, verticalWidths: map[int]float64{}, verticalPositions: map[int][2]float64{}}
	parseCIDWidths(nil, Array{Number(1), Number(1 << 60), Number(500)}, f)
	if len(f.cidWidths) != 0 {
		t.Fatalf("out-of-range CID widths = %#v", f.cidWidths)
	}

	d := &Document{}
	parseCIDVerticalWidths(d, Array{Number(1), Number(1 << 60), Number(500), Number(0), Number(0)}, f)
	if len(f.verticalWidths) != 0 || len(f.verticalPositions) != 0 {
		t.Fatalf("out-of-range vertical CID widths = %#v positions=%#v", f.verticalWidths, f.verticalPositions)
	}
}

func TestCIDVerticalWidthsDoNotPublishPartialArrays(t *testing.T) {
	d := &Document{}
	f := &Font{verticalWidths: map[int]float64{}, verticalPositions: map[int][2]float64{}}
	parseCIDVerticalWidths(d, Array{Number(1), Array{Number(500), Number(0), Number(880), Number(600)}}, f)
	if len(f.verticalWidths) != 0 || len(f.verticalPositions) != 0 {
		t.Fatalf("partial vertical widths were published: widths=%#v positions=%#v", f.verticalWidths, f.verticalPositions)
	}
}

func TestCIDVerticalWidthsFollowIndirectEntries(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2}, {Object: 2}: Number(1),
		{Object: 3}: Ref{Object: 4}, {Object: 4}: Array{Ref{Object: 5}, Number(0), Ref{Object: 6}},
		{Object: 5}: Number(500), {Object: 6}: Number(880),
	}}
	f := &Font{verticalWidths: map[int]float64{}, verticalPositions: map[int][2]float64{}}
	parseCIDVerticalWidths(d, Array{Ref{Object: 1}, Ref{Object: 3}}, f)
	if f.verticalWidths[1] != 500 || f.verticalPositions[1] != [2]float64{0, 880} {
		t.Fatalf("indirect vertical width = %#v positions=%#v", f.verticalWidths, f.verticalPositions)
	}
}

func TestCIDWidthsDoNotPublishPartialArrays(t *testing.T) {
	f := &Font{cidWidths: map[int]float64{99: 500}}
	parseCIDWidths(nil, Array{Number(1), Array{Number(600), String("invalid")}}, f)
	if len(f.cidWidths) != 1 || f.cidWidths[99] != 500 {
		t.Fatalf("partial CID widths were published: %#v", f.cidWidths)
	}
}

func TestType0CIDSystemInfoSelectsAdobeCIDUnicodeFallback(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):         Name("Type0"),
		Name("Encoding"):        Name("UniJIS-UCS2-H"),
		Name("DescendantFonts"): Array{Ref{Object: 2}},
	}
	d.objects[Ref{Object: 2}] = Dict{
		Name("Subtype"): Name("CIDFontType2"),
		Name("CIDSystemInfo"): Dict{
			Name("Registry"): String("Adobe"),
			Name("Ordering"): String("Japan1"),
		},
	}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	font := fonts["F"]
	if font == nil || font.cidToUnicode[2980] != "中" {
		t.Fatalf("CIDSystemInfo fallback = %#v", font)
	}
}

func TestType0VerticalDefaultPositionUsesDescendantDW(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):         Name("Type0"),
		Name("BaseFont"):        Name("Test"),
		Name("Encoding"):        Name("Identity-V"),
		Name("DescendantFonts"): Array{Ref{Object: 2}},
	}
	d.objects[Ref{Object: 2}] = Dict{
		Name("Subtype"): Name("CIDFontType2"),
		Name("DW"):      Number(1000),
	}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F"].defaultVPosition; got != [2]float64{500, 880} {
		t.Fatalf("vertical default position = %v, want (500,880)", got)
	}
}

func TestStandardFontUsesBuiltInMetricsBeforePDFWidths(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{
		Name("Subtype"):   Name("Type1"),
		Name("BaseFont"):  Name("Times-Roman"),
		Name("FirstChar"): Number(82),
		Name("Widths"):    Array{Number(591)},
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	font := fonts["F"]
	if font == nil {
		t.Fatal("missing font")
	}
	if got := font.WidthCode([]byte("R")); got != 667 {
		t.Fatalf("Times-Roman R width = %v, want built-in 667", got)
	}
	if font.ascent != 683 || font.descent != -217 {
		t.Fatalf("Times-Roman metrics = ascent %v descent %v", font.ascent, font.descent)
	}
}

func TestSubsetStandardFontUsesPDFWidthsBeforeBuiltInMetrics(t *testing.T) {
	d := &Document{}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Dict{
		Name("Subtype"):   Name("Type1"),
		Name("BaseFont"):  Name("Subset+Times-Roman"),
		Name("FirstChar"): Number(80),
		Name("Widths"):    Array{Number(610)},
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	font := fonts["F"]
	if font == nil {
		t.Fatal("missing font")
	}
	if got := font.WidthCode([]byte("P")); got != 610 {
		t.Fatalf("subset Times-Roman P width = %v, want PDF width 610", got)
	}
}

func TestCIDStandardFontUsesDecodedBuiltInWidthsWhenCIDWidthIsMissing(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):         Name("Type0"),
		Name("BaseFont"):        Name("Subset+Arial-BoldMT"),
		Name("Encoding"):        Name("Identity-H"),
		Name("ToUnicode"):       Ref{Object: 4},
		Name("DescendantFonts"): Array{Ref{Object: 2}},
	}
	d.objects[Ref{Object: 2}] = Dict{
		Name("Subtype"): Name("CIDFontType2"),
	}
	d.objects[Ref{Object: 4}] = newStream(Dict{}, []byte("1 beginbfchar\n<0003> <0020>\nendbfchar"))
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	font := fonts["F"]
	if font == nil {
		t.Fatal("missing font")
	}
	if got := font.WidthCode([]byte{0, 3}); got != 278 {
		t.Fatalf("Arial-BoldMT decoded space width = %v, want built-in 278", got)
	}
}

func TestType0CIDFontUsesEmbeddedTrueTypeCMapFallback(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):         Name("Type0"),
		Name("BaseFont"):        Name("Test"),
		Name("Encoding"):        Name("Identity-H"),
		Name("DescendantFonts"): Array{Ref{Object: 2}},
	}
	d.objects[Ref{Object: 2}] = Dict{
		Name("Subtype"):        Name("CIDFontType2"),
		Name("FontDescriptor"): Ref{Object: 3},
	}
	d.objects[Ref{Object: 3}] = Dict{Name("FontFile2"): Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = newStream(Dict{}, testTrueTypeWithFormat4CMap())

	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F"].Decode([]byte{0, 1, 0, 2, 0, 3}); got != "ABC" {
		t.Fatalf("decoded text = %q, want ABC", got)
	}
}

func TestType0IgnoresToUnicodeWhenItAliasesEncoding(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):         Name("Type0"),
		Name("BaseFont"):        Name("AliasToUnicode"),
		Name("Encoding"):        Ref{Object: 4},
		Name("ToUnicode"):       Ref{Object: 4},
		Name("DescendantFonts"): Array{Ref{Object: 2}},
	}
	d.objects[Ref{Object: 2}] = Dict{Name("Subtype"): Name("CIDFontType2")}
	d.objects[Ref{Object: 4}] = newStream(Dict{}, []byte(`
1 begincodespacerange
<0102> <0102>
endcodespacerange
1 begincidchar
<0102> 5
endcidchar`))

	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{
		Name("Font"): Dict{Name("F"): Ref{Object: 1}},
	}}})
	if err != nil {
		t.Fatalf("pageFontsDirect() error = %v", err)
	}
	got := fonts["F"].DecodeGlyphs([]byte{0x01, 0x02})
	if len(got) != 1 || got[0].Text() != string(rune(0x0102)) {
		t.Fatalf("aliased ToUnicode decoded glyphs = %#v, want identity U+0102", got)
	}
}

func TestSimpleTrueTypeBuildsEmbeddedGlyphReverseIndex(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):        Name("TrueType"),
		Name("BaseFont"):       Name("Test"),
		Name("FontDescriptor"): Ref{Object: 2},
	}
	d.objects[Ref{Object: 2}] = Dict{Name("FontFile2"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = newStream(Dict{}, testTrueTypeWithFormat4CMap())

	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	fonts["F"].ensureTrueType()
	if got := fonts["F"].glyphUnicodeToID['A']; got != 1 {
		t.Fatalf("embedded glyph reverse index[A] = %d, want 1", got)
	}
}

func TestSimpleOpenTypeUsesEmbeddedGlyphCMap(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):        Name("TrueType"),
		Name("BaseFont"):       Name("Test"),
		Name("FontDescriptor"): Ref{Object: 2},
	}
	d.objects[Ref{Object: 2}] = Dict{Name("FontFile3"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = newStream(Dict{Name("Subtype"): Name("OpenType")}, testTrueTypeWithFormat4CMap())

	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	fonts["F"].ensureTrueType()
	if got := fonts["F"].glyphUnicodeToID['A']; got != 1 {
		t.Fatalf("OpenType glyph reverse index[A] = %d, want 1", got)
	}
}

func TestEmbeddedFontProgramsFollowMultiLevelDescriptorChains(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):        Name("TrueType"),
		Name("BaseFont"):       Name("Test"),
		Name("FontDescriptor"): Ref{Object: 2},
	}
	d.objects[Ref{Object: 2}] = Ref{Object: 3}
	d.objects[Ref{Object: 3}] = Dict{Name("FontFile2"): Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = Ref{Object: 5}
	d.objects[Ref{Object: 5}] = newStream(Dict{}, testTrueTypeWithFormat4CMap())

	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil || len(f.trueTypeData) == 0 || f.trueTypeParsed {
		t.Fatalf("indirect TrueType program was not retained lazily: %#v", f)
	}
	f.ensureTrueType()
	if f.glyphUnicodeToID['A'] != 1 {
		t.Fatalf("indirect TrueType cmap = %#v", f.glyphUnicodeToID)
	}
}

func TestFontMetricsFollowMultiLevelDescriptorValueChains(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):        Name("TrueType"),
		Name("BaseFont"):       Name("Metrics"),
		Name("FontDescriptor"): Ref{Object: 2},
	}
	d.objects[Ref{Object: 2}] = Dict{
		Name("Ascent"):      Ref{Object: 3},
		Name("Descent"):     Ref{Object: 4},
		Name("FontBBox"):    Ref{Object: 5},
		Name("ItalicAngle"): Ref{Object: 6},
	}
	d.objects[Ref{Object: 3}] = Ref{Object: 7}
	d.objects[Ref{Object: 4}] = Ref{Object: 8}
	d.objects[Ref{Object: 5}] = Ref{Object: 9}
	d.objects[Ref{Object: 6}] = Ref{Object: 10}
	d.objects[Ref{Object: 7}] = Number(900)
	d.objects[Ref{Object: 8}] = Number(200)
	d.objects[Ref{Object: 9}] = Array{Number(-10), Number(-200), Number(1000), Number(900)}
	d.objects[Ref{Object: 10}] = Number(-12)

	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil || f.ascent != 900 || f.descent != -200 || f.italicAngle != -12 || f.fontBBox != [4]float64{-10, -200, 1000, 900} {
		t.Fatalf("multi-level font metrics = %#v", f)
	}
}

func TestCIDEmbeddedCMapPrivateUseFallsBackToSourceCode(t *testing.T) {
	f := NewSimpleFont("Symbol")
	f.cid = true
	f.cmap = identityCMap(false)
	f.glyphIDToUnicode = map[int]string{0x78: "\uf0b7"}
	if got := f.Decode([]byte{0, 0x78}); got != "x" {
		t.Fatalf("decoded private-use glyph = %q, want source-code fallback x", got)
	}
}

func TestTrueTypeCMapRejectsTruncatedDeclaredLengths(t *testing.T) {
	data := testTrueTypeWithFormat4CMap()
	format4Offset := 28 + 12
	binary.BigEndian.PutUint16(data[format4Offset+2:], 0x40)
	if glyphs := fontdata.ParseTrueTypeCMapGlyphs(data); glyphs != nil {
		t.Fatalf("TrueType cmap accepted an oversized subtable length: %#v", glyphs)
	}

	data = testTrueTypeWithFormat4CMap()
	binary.BigEndian.PutUint16(data[28+2:], 100)
	if glyphs := fontdata.ParseTrueTypeCMapGlyphs(data); glyphs != nil {
		t.Fatalf("TrueType cmap accepted truncated encoding records: %#v", glyphs)
	}
}

func TestTrueTypeCMapRejectsOverlappingGroups(t *testing.T) {
	data := make([]byte, 40)
	binary.BigEndian.PutUint16(data[0:2], 12)
	binary.BigEndian.PutUint32(data[4:8], uint32(len(data)))
	binary.BigEndian.PutUint32(data[12:16], 2)
	binary.BigEndian.PutUint32(data[16:20], 10)
	binary.BigEndian.PutUint32(data[20:24], 20)
	binary.BigEndian.PutUint32(data[24:28], 1)
	binary.BigEndian.PutUint32(data[28:32], 20)
	binary.BigEndian.PutUint32(data[32:36], 30)
	binary.BigEndian.PutUint32(data[36:40], 2)
	if glyphs := fontdata.ParseTrueTypeCMapFormat12(data); glyphs != nil {
		t.Fatalf("TrueType cmap accepted overlapping groups: %#v", glyphs)
	}
}

func TestTrueTypeCMapRejectsOutOfBoundsGlyphArrays(t *testing.T) {
	format2 := make([]byte, 518+8)
	binary.BigEndian.PutUint16(format2[0:2], 2)
	binary.BigEndian.PutUint16(format2[2:4], uint16(len(format2)))
	binary.BigEndian.PutUint16(format2[518+2:518+4], 1)
	binary.BigEndian.PutUint16(format2[518+6:518+8], 0xffff)
	if glyphs := fontdata.ParseTrueTypeCMapFormat2(format2); glyphs != nil {
		t.Fatalf("TrueType format 2 accepted an out-of-bounds glyph array: %#v", glyphs)
	}

	data := testTrueTypeWithFormat4CMap()[28+12:]
	binary.BigEndian.PutUint16(data[28:30], 0xffff)
	binary.BigEndian.PutUint16(data[30:32], 0xffff)
	if glyphs := fontdata.ParseTrueTypeCMapFormat4(data); glyphs != nil {
		t.Fatalf("TrueType format 4 accepted an out-of-bounds glyph array: %#v", glyphs)
	}
}

func testTrueTypeWithFormat4CMap() []byte {
	format4 := []byte{
		0x00, 0x04, 0x00, 0x20, 0x00, 0x00, 0x00, 0x04,
		0x00, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x43,
		0xff, 0xff, 0x00, 0x00, 0x00, 0x41, 0xff, 0xff,
		0xff, 0xc0, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00,
	}
	cmap := make([]byte, 12+len(format4))
	binary.BigEndian.PutUint16(cmap[2:4], 1)
	binary.BigEndian.PutUint16(cmap[4:6], 3)
	binary.BigEndian.PutUint16(cmap[6:8], 1)
	binary.BigEndian.PutUint32(cmap[8:12], 12)
	copy(cmap[12:], format4)

	out := make([]byte, 28+len(cmap))
	binary.BigEndian.PutUint32(out[0:4], 0x00010000)
	binary.BigEndian.PutUint16(out[4:6], 1)
	copy(out[12:16], "cmap")
	binary.BigEndian.PutUint32(out[20:24], 28)
	binary.BigEndian.PutUint32(out[24:28], uint32(len(cmap)))
	copy(out[28:], cmap)
	return out
}
