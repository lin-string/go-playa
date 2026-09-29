package document

import "testing"

func TestAnnotationsReportUnresolvedIndirectRoot(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	page := Page{ref: Ref{Object: 7}, dict: Dict{Name("Annots"): Ref{Object: 99}}}

	if _, err := d.CollectAnnotations(page); err == nil || err.Error() != "playa: annotations root could not be resolved" {
		t.Fatalf("unresolved Annots root error = %v", err)
	}
	_, firstErr := d.CollectAnnotations(page)
	if firstErr == nil || d.annotationRootErrors[page.ref] == nil || d.annotationRootErrors[page.ref] != firstErr {
		t.Fatalf("cached Annots root error = %v, want %v", d.annotationRootErrors[page.ref], firstErr)
	}
}

func TestAnnotationsReportUnresolvedIndirectEntry(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Array{Ref{Object: 99}},
	}}
	page := Page{dict: Dict{Name("Annots"): Ref{Object: 1}}}

	if _, err := d.CollectAnnotations(page); err == nil || err.Error() != "playa: annotation could not be resolved" {
		t.Fatalf("unresolved annotation entry error = %v", err)
	}
}

func TestAnnotationsResolveIndirectCommonFields(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.trailer = Dict{}
	d.objects[Ref{Object: 2}] = String("comment")
	d.objects[Ref{Object: 3}] = String("annotation-id")
	d.objects[Ref{Object: 4}] = Name("Text")
	d.objects[Ref{Object: 1}] = Dict{
		Name("Subtype"):  Ref{Object: 4},
		Name("Rect"):     Array{Number(0), Number(0), Number(10), Number(10)},
		Name("Contents"): Ref{Object: 2},
		Name("NM"):       Ref{Object: 3},
	}
	d.objects[Ref{Object: 5}] = Array{Ref{Object: 1}}
	page := Page{dict: Dict{Name("Annots"): Ref{Object: 5}}}
	anns, err := d.CollectAnnotations(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Subtype() != "Text" || anns[0].Contents() != "comment" || anns[0].Name() != "annotation-id" {
		t.Fatalf("annotations = %#v", anns)
	}
}

func TestAnnotationsFollowMultiLevelIndirectRootsAndEntries(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Ref{Object: 2}
	d.objects[Ref{Object: 2}] = Array{Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Ref{Object: 4}
	d.objects[Ref{Object: 4}] = Dict{
		Name("Subtype"):  Ref{Object: 5},
		Name("Rect"):     Ref{Object: 6},
		Name("Contents"): Ref{Object: 7},
	}
	d.objects[Ref{Object: 5}] = Ref{Object: 8}
	d.objects[Ref{Object: 6}] = Array{Number(0), Number(0), Number(10), Number(10)}
	d.objects[Ref{Object: 7}] = Ref{Object: 9}
	d.objects[Ref{Object: 8}] = Name("Text")
	d.objects[Ref{Object: 9}] = String("indirect")
	p := Page{dict: Dict{Name("Annots"): Ref{Object: 1}}}
	anns, err := d.CollectAnnotations(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Subtype() != "Text" || anns[0].Contents() != "indirect" {
		t.Fatalf("annotations = %#v", anns)
	}
}

func TestAnnotationsFollowMultiLevelIndirectDisplayAndActionFields(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	put := func(n int, value Object) { d.objects[Ref{Object: n}] = value }
	put(1, Array{Ref{Object: 2}})
	put(2, Dict{
		Name("Subtype"): Ref{Object: 3}, Name("Rect"): Ref{Object: 4},
		Name("QuadPoints"): Ref{Object: 5}, Name("Border"): Ref{Object: 6},
		Name("C"): Ref{Object: 7}, Name("A"): Ref{Object: 8},
		Name("StructParent"): Ref{Object: 15}, Name("F"): Ref{Object: 16},
	})
	put(3, Name("Link"))
	put(4, Ref{Object: 9})
	put(5, Array{Number(0), Number(0), Number(10), Number(0), Number(10), Number(10), Number(0), Number(10)})
	put(6, Array{Number(0), Number(0), Number(2), Ref{Object: 10}})
	put(7, Array{Ref{Object: 11}, Number(0.5), Number(0)})
	put(8, Ref{Object: 12})
	put(9, Array{Number(20), Number(30), Number(10), Number(5)})
	put(10, Array{Number(1), Number(2)})
	put(11, Number(1))
	put(12, Dict{Name("S"): Ref{Object: 13}, Name("URI"): Ref{Object: 14}})
	put(13, Name("URI"))
	put(14, String("https://example.com"))
	put(15, Number(4))
	put(16, Number(4))

	anns, err := d.CollectAnnotations(Page{dict: Dict{Name("Annots"): Ref{Object: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(anns) != 1 || anns[0].Rect() != [4]float64{20, 30, 10, 5} || anns[0].ActionKind() != "URI" || anns[0].URI() != "https://example.com" {
		t.Fatalf("indirect annotation fields = %#v", anns)
	}
	if len(anns[0].QuadPointsCopy()) != 4 || anns[0].Border() != [3]float64{0, 0, 2} || len(anns[0].ColorCopy()) != 3 || !anns[0].HasParentKey() || anns[0].Flags() != 4 {
		t.Fatalf("indirect annotation display fields = %#v", anns[0])
	}
}

func TestAnnotationsFollowMultiLevelIndirectRelationFields(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	put := func(n int, value Object) { d.objects[Ref{Object: n}] = value }
	put(1, Array{Ref{Object: 2}})
	put(2, Dict{
		Name("Subtype"): Name("Text"),
		Name("Rect"):    Array{Number(0), Number(0), Number(10), Number(10)},
		Name("IRT"):     Ref{Object: 3},
		Name("Popup"):   Ref{Object: 5},
	})
	put(3, Ref{Object: 4})
	put(4, Dict{Name("Subtype"): Name("Text")})
	put(5, Ref{Object: 6})
	put(6, Dict{Name("Subtype"): Name("Popup")})

	annotations, err := d.CollectAnnotations(Page{dict: Dict{Name("Annots"): Ref{Object: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(annotations) != 1 || !annotations[0].HasInReplyTo() || annotations[0].InReplyTo() != (Ref{Object: 4}) || !annotations[0].HasPopup() || annotations[0].Popup() != (Ref{Object: 6}) {
		t.Fatalf("indirect annotation relations = %#v", annotations)
	}
}
