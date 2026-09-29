package document

import "testing"

func TestPageSizeDefaultsToUSLetterWithoutMediaBox(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{}}
	width, height := p.Size(d)
	if width != 612 || height != 792 {
		t.Fatalf("size = %v x %v", width, height)
	}
}

func TestPageBBoxFallsBackToMediaBox(t *testing.T) {
	d := &Document{}
	if got := (Page{dict: Dict{}}).BBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("bbox = %v", got)
	}
}
