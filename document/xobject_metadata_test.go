package document

import (
	"math"
	"testing"

	"github.com/lin-string/go-playa/geometry"
)

func TestFormMetadataPreservesBBoxAndGroup(t *testing.T) {
	d := &Document{}
	stream := newStream(Dict{
		Name("BBox"):         Array{Number(20), Number(40), Number(10), Number(30)},
		Name("Group"):        Dict{Name("S"): Name("Transparency")},
		Name("StructParent"): Number(12),
	}, nil)
	if got := formBBox(d, stream); got != [4]float64{10, 30, 20, 40} {
		t.Fatalf("bbox = %v", got)
	}
	if got := formGroup(d, stream); got[Name("S")] != Name("Transparency") {
		t.Fatalf("group = %#v", got)
	}
	if key, ok := formParentKey(d, stream); key != 12 || !ok {
		t.Fatalf("parent key = %d, present=%v", key, ok)
	}
}

func TestFormBBoxRejectsExtraValues(t *testing.T) {
	d := &Document{}
	stream := newStream(Dict{Name("BBox"): Array{Number(0), Number(0), Number(10), Number(10), Number(20)}}, nil)
	if got := formBBox(d, stream); got != [4]float64{} {
		t.Fatalf("malformed form BBox was accepted: %v", got)
	}
}

func TestFormBBoxRejectsNonFiniteValues(t *testing.T) {
	d := &Document{}
	stream := newStream(Dict{Name("BBox"): Array{Number(0), Number(0), Number(math.Inf(1)), Number(10)}}, nil)
	if got := formBBox(d, stream); got != [4]float64{} {
		t.Fatalf("non-finite form BBox was accepted: %v", got)
	}
}

func TestFormParentKeyRejectsNegativeValues(t *testing.T) {
	d := &Document{}
	if key, ok := formParentKey(d, newStream(Dict{Name("StructParent"): Number(-1)}, nil)); ok {
		t.Fatalf("negative StructParent accepted as %d", key)
	}
}

func TestFormParentKeyRejectsAbsentValue(t *testing.T) {
	if key, ok := formParentKey(&Document{}, newStream(nil, nil)); ok {
		t.Fatalf("absent form parent key accepted as %d", key)
	}
}

func TestFormBBoxFallsBackToPageCropBox(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("CropBox"): Array{Number(10), Number(20), Number(210), Number(320)}}}
	if got := formBBoxForPage(d, p, Stream{}, geometry.Matrix{}); got != [4]float64{10, 20, 210, 320} {
		t.Fatalf("fallback bbox = %v", got)
	}
}

func TestFormBBoxUsesFormMatrix(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("CropBox"): Array{Number(0), Number(0), Number(100), Number(100)}}}
	stream := newStream(Dict{Name("BBox"): Array{Number(0), Number(0), Number(2), Number(3)}}, nil)
	matrix := geometry.Matrix{2, 0, 0, 3, 10, 20}
	if got := formBBoxForPage(d, p, stream, matrix); got != [4]float64{10, 20, 14, 29} {
		t.Fatalf("transformed bbox = %v", got)
	}
}

func TestTransformBBoxRejectsOverflowingMatrixProducts(t *testing.T) {
	box, ok := transformBBox(geometry.Matrix{math.MaxFloat64, 0, 0, 1, math.MaxFloat64, 0}, [4]float64{0, 0, 1, 1})
	if ok || box != [4]float64{} {
		t.Fatalf("overflowing transformed bbox = %v, valid=%v", box, ok)
	}
}

func TestXObjectStreamAccessors(t *testing.T) {
	x := testXObjectStream(newStream(Dict{
		Name("Subtype"): Name("Form"),
		Name("Length"):  Number(15),
	}, []byte("q 1 0 0 1 0 0 cm")))
	buffer := x.Buffer()
	if string(buffer) != "q 1 0 0 1 0 0 cm" {
		t.Fatalf("buffer = %q", buffer)
	}
	buffer[0] = 'X'
	if x.data.StreamBorrowed().DataBorrowed()[0] != 'q' {
		t.Fatal("Buffer did not return a defensive copy")
	}
	if value, ok := x.Get(Name("Length")); !ok || value != Number(15) {
		t.Fatalf("Length = %v, %v", value, ok)
	}
	if !x.Has(Name("Subtype")) || x.Has(Name("Missing")) {
		t.Fatal("unexpected dictionary membership")
	}
}

func TestXObjectGetDoesNotExposeNestedDictionary(t *testing.T) {
	x := testXObjectStream(newStream(Dict{Name("Meta"): Dict{Name("Value"): String("original")}}, nil))
	value, ok := x.Get(Name("Meta"))
	if !ok {
		t.Fatal("xobject dictionary entry is missing")
	}
	value.(Dict)[Name("Value")] = String("changed")
	value, _ = x.Get(Name("Meta"))
	if got := value.(Dict)[Name("Value")].(String); string(got) != "original" {
		t.Fatalf("xobject dictionary value was exposed: %#v", value)
	}
}
