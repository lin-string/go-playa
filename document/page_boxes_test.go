package document

import "testing"

func TestPageExposesMediaAndCropBoxesSeparately(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)},
		Name("CropBox"):  Array{Number(30), Number(40), Number(90), Number(180)},
	}}
	if got := p.MediaBox(d); got != [4]float64{10, 20, 110, 220} {
		t.Fatalf("media box = %v", got)
	}
	if got := p.CropBox(d); got != [4]float64{30, 40, 90, 180} {
		t.Fatalf("crop box = %v", got)
	}
}

func TestPageBoxesRejectMalformedRectangles(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{
		Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220), Number(999)},
		Name("CropBox"):  Array{Number(30), String("bad"), Number(90), Number(180)},
	}}
	if got := p.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("malformed media box = %v, want US Letter fallback", got)
	}
	if got := p.CropBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("malformed crop box = %v, want media fallback", got)
	}
}

func TestPageBoxesCacheResolvedValuesAndErrors(t *testing.T) {
	d := &Document{scannedObjects: true, objects: map[Ref]Object{{Object: 9}: Array{Number(10), Number(20), Number(110), Number(220)}}}
	p := Page{ref: Ref{Object: 7}, dict: Dict{Name("MediaBox"): Ref{Object: 9}}}
	first, err := p.MediaBoxWithError(d)
	if err != nil || first != [4]float64{10, 20, 110, 220} {
		t.Fatalf("first media box = %v, err=%v", first, err)
	}
	d.objects[Ref{Object: 9}] = Array{Number(0), Number(0), Number(1), Number(1)}
	second, err := p.MediaBoxWithError(d)
	if err != nil || second != first {
		t.Fatalf("cached media box = %v, err=%v, want %v", second, err, first)
	}

	bad := &Document{}
	badPage := Page{ref: Ref{Object: 8}, dict: Dict{Name("CropBox"): Ref{Object: 99}}}
	_, firstErr := badPage.CropBoxWithError(bad)
	_, secondErr := badPage.CropBoxWithError(bad)
	if firstErr == nil || secondErr != firstErr {
		t.Fatalf("cached crop box error = %v/%v", firstErr, secondErr)
	}
	bad.ReleaseTransientCaches()
	if _, err := badPage.CropBoxWithError(bad); err == nil || err == firstErr {
		t.Fatalf("crop box error survived transient-cache release: %v", err)
	}
}
