package document

import (
	"math"
	"testing"

	"github.com/lin-string/go-playa/geometry"
)

func TestResolveFontBBoxSupportsIndirectArray(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 2}] = Array{Number(-20), Number(-200), Number(900), Number(700)}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):        Name("Type1"),
		Name("BaseFont"):       Name("Test"),
		Name("FontDescriptor"): Ref{Object: 3},
	}
	d.objects[Ref{Object: 3}] = Dict{Name("FontBBox"): Ref{Object: 2}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil || !f.hasFontBBox || f.fontBBox != [4]float64{-20, -200, 900, 700} {
		t.Fatalf("font bbox = %#v", f)
	}
}

func TestResolveFontBBoxFollowsIndirectArrayMembers(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Subtype"): Name("Type1"), Name("BaseFont"): Name("Test"), Name("FontDescriptor"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("FontBBox"): Ref{Object: 3}},
		{Object: 3}: Array{Ref{Object: 4}, Ref{Object: 5}, Number(900), Ref{Object: 6}},
		{Object: 4}: Number(-20), {Object: 5}: Number(-200), {Object: 6}: Number(700),
	}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil || fonts["F"] == nil || fonts["F"].fontBBox != [4]float64{-20, -200, 900, 700} {
		t.Fatalf("font bbox members = %#v, err=%v", fonts["F"], err)
	}
}

func TestFontDescriptorRejectsBBoxWithExtraValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("Subtype"):        Name("Type1"),
			Name("BaseFont"):       Name("Test"),
			Name("FontDescriptor"): Dict{Name("FontBBox"): Array{Number(0), Number(0), Number(10), Number(10), Number(20)}},
		},
	}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if font := fonts["F"]; font == nil || font.hasFontBBox {
		t.Fatalf("malformed descriptor FontBBox was accepted: %#v", font)
	}
}

func TestType3RejectsBBoxWithExtraValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("Subtype"):  Name("Type3"),
			Name("FontBBox"): Array{Number(0), Number(0), Number(10), Number(10), Number(20)},
		},
	}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if font := fonts["F"]; font == nil || font.hasFontBBox {
		t.Fatalf("malformed Type3 FontBBox was accepted: %#v", font)
	}
}

func TestType3RejectsFontMatrixWithExtraValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("Subtype"):    Name("Type3"),
			Name("FontMatrix"): Array{Number(2), Number(0), Number(0), Number(2), Number(10), Number(20), Number(30)},
		},
	}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if font := fonts["F"]; font == nil || font.fontMatrix != (geometry.Matrix{0.001, 0, 0, 0.001, 0, 0}) {
		t.Fatalf("malformed Type3 FontMatrix was accepted: %#v", font)
	}
}

func TestType3RejectsNonFiniteGeometry(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("Subtype"):    Name("Type3"),
			Name("FontMatrix"): Array{Number(math.Inf(1)), Number(0), Number(0), Number(1), Number(0), Number(0)},
			Name("FontBBox"):   Array{Number(0), Number(0), Number(math.NaN()), Number(10)},
		},
	}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	font := fonts["F"]
	if font == nil || font.fontMatrix != (geometry.Matrix{0.001, 0, 0, 0.001, 0, 0}) || font.hasFontBBox {
		t.Fatalf("non-finite Type3 geometry was accepted: %#v", font)
	}
}

func TestFontDescriptorIgnoresNonFiniteMetrics(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("Subtype"):        Name("Type1"),
			Name("BaseFont"):       Name("Test"),
			Name("FontDescriptor"): Dict{Name("Ascent"): Number(math.Inf(1)), Name("MissingWidth"): Number(math.NaN())},
		},
	}}
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	font := fonts["F"]
	if font == nil || math.IsInf(font.ascent, 0) || math.IsNaN(font.defaultWidth) {
		t.Fatalf("non-finite font metrics were accepted: %#v", font)
	}
}

func TestResolveFontDescriptorMetricsThroughIndirectObjects(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):        Name("Type1"),
		Name("BaseFont"):       Name("Test"),
		Name("FontDescriptor"): Ref{Object: 2},
	}
	d.objects[Ref{Object: 2}] = Dict{
		Name("Ascent"):       Ref{Object: 3},
		Name("Descent"):      Ref{Object: 4},
		Name("Flags"):        Ref{Object: 5},
		Name("ItalicAngle"):  Ref{Object: 6},
		Name("StemV"):        Ref{Object: 7},
		Name("MissingWidth"): Ref{Object: 8},
	}
	d.objects[Ref{Object: 3}] = Number(900)
	d.objects[Ref{Object: 4}] = Number(120)
	d.objects[Ref{Object: 5}] = Number(4)
	d.objects[Ref{Object: 6}] = Number(-12)
	d.objects[Ref{Object: 7}] = Number(80)
	d.objects[Ref{Object: 8}] = Number(610)
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	f := fonts["F"]
	if f == nil || f.ascent != 900 || f.descent != -120 || !f.hasFlags || f.flags != 4 || f.italicAngle != -12 || f.stemV != 80 || f.defaultWidth != 610 {
		t.Fatalf("indirect font metrics = %#v", f)
	}
}

func TestResolveIndirectFirstCharForSimpleFontWidths(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):   Name("Type1"),
		Name("BaseFont"):  Name("Test"),
		Name("FirstChar"): Ref{Object: 2},
		Name("Widths"):    Array{Number(611)},
	}
	d.objects[Ref{Object: 2}] = Number(65)
	fonts, err := d.pageFontsDirect(Page{dict: Dict{Name("Resources"): Dict{Name("Font"): Dict{Name("F"): Ref{Object: 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := fonts["F"].widths[65]; got != 611 {
		t.Fatalf("indirect FirstChar width = %v, want 611", got)
	}
}
