package document

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
)

func TestBBoxAccessorsMatchPlayaLTComponent(t *testing.T) {
	bbox := [4]float64{10, 20, 40, 65}
	tests := []struct {
		name string
		got  bboxGeometry
	}{
		{"glyph", newTestGlyphWithBBox(bbox)},
		{"text", newTestText(contentdata.TextSpec{BBox: bbox}, nil, nil)},
		{"path", newPathObject(contentdata.PathSpec{BBox: bbox})},
		{"xobject", testXObject(contentdata.XObjectSpec{BBox: bbox})},
		{"word", newTestTextWord("", bbox)},
		{"line", newTestTextLine("", bbox, false)},
		{"paragraph", newTestTextParagraph("", bbox, false)},
		{"textbox", newTestTextBox("", bbox, false, -1)},
		{"group", newTestTextGroup("", bbox, false)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checkBBoxAccessors(t, test.got, bbox, false)
		})
	}

	image := ImageObject{bbox: bbox}
	if image.BBoxX0() != bbox[0] || image.BBoxY0() != bbox[1] || image.BBoxX1() != bbox[2] || image.BBoxY1() != bbox[3] || image.BBoxWidth() != 30 || image.BBoxHeight() != 45 || image.IsEmpty() {
		t.Fatalf("image bbox accessors do not match %v", bbox)
	}
}

func TestBBoxAccessorsMatchPlayaEmptySemantics(t *testing.T) {
	for _, bbox := range [][4]float64{
		{0, 0, 0, 10},
		{0, 0, 10, 0},
		{10, 10, 5, 20},
	} {
		if got := newTestTextWord("", bbox).IsEmpty(); !got {
			t.Errorf("TextWord bbox %v IsEmpty() = false, want true", bbox)
		}
	}

	line := newTestTextLine(" \t\n", [4]float64{0, 0, 10, 10}, false)
	if !line.IsEmpty() {
		t.Fatal("whitespace-only TextLine IsEmpty() = false, want true")
	}
	line.data = line.data.WithText("A")
	if line.IsEmpty() {
		t.Fatal("non-empty TextLine IsEmpty() = true, want false")
	}
}

type bboxGeometry interface {
	BBox() [4]float64
	X0() float64
	Y0() float64
	X1() float64
	Y1() float64
	Width() float64
	Height() float64
	IsEmpty() bool
}

func checkBBoxAccessors(t *testing.T, got bboxGeometry, want [4]float64, empty bool) {
	t.Helper()
	if bbox := got.BBox(); bbox != want {
		t.Fatalf("BBox() = %v, want %v", bbox, want)
	}
	if got.X0() != want[0] || got.Y0() != want[1] || got.X1() != want[2] || got.Y1() != want[3] {
		t.Fatalf("bbox coordinates = %v, want %v", got.BBox(), want)
	}
	if got.Width() != want[2]-want[0] || got.Height() != want[3]-want[1] {
		t.Fatalf("bbox dimensions = %v x %v, want %v x %v", got.Width(), got.Height(), want[2]-want[0], want[3]-want[1])
	}
	if got.IsEmpty() != empty {
		t.Fatalf("IsEmpty() = %v, want %v", got.IsEmpty(), empty)
	}
}
