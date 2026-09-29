package document

import "testing"

func TestDestinationParamsCopyPreservesAbsentParams(t *testing.T) {
	if got := (Destination{}).ParamsCopy(); got != nil {
		t.Fatalf("destination params = %#v, want nil", got)
	}
}

func TestDestinationNCoordsMatchesPlayaViews(t *testing.T) {
	tests := []struct {
		view string
		want int
	}{
		{view: "XYZ", want: 3},
		{view: "Fit", want: 0},
		{view: "FitH", want: 1},
		{view: "FitBH", want: 1},
		{view: "FitV", want: 1},
		{view: "FitBV", want: 1},
		{view: "FitR", want: 4},
		{view: "Unknown", want: 0},
		{view: "", want: 0},
	}
	for _, test := range tests {
		destination := newDestination(Ref{}, false, 0, false, test.view, nil)
		if got := destination.NCoords(); got != test.want {
			t.Errorf("NCoords(%q) = %d, want %d", test.view, got, test.want)
		}
	}
}

func TestResolveDestinationAcceptsWrappedAndIndirectNamedDestinations(t *testing.T) {
	d := &Document{
		objects: map[Ref]Object{
			{Object: 1}: String("Named"),
			{Object: 2}: Dict{Name("D"): Array{Ref{Object: 9}, Name("Fit")}},
			{Object: 3}: Array{Ref{Object: 8}, Name("FitH"), Number(420)},
		},
		destinationCache: map[string]Object{
			"Named": Ref{Object: 2},
		},
	}
	wrapped := d.ResolveDestination(Ref{Object: 2})
	if wrapped == nil {
		t.Fatalf("wrapped destination = %#v", wrapped)
	}
	page, hasPage := wrapped.PageRef()
	if !hasPage || page.Object != 9 || wrapped.View() != "Fit" {
		t.Fatalf("wrapped destination = %#v", wrapped)
	}
	named := d.ResolveDestination(Ref{Object: 1})
	if named == nil {
		t.Fatalf("named destination = %#v", named)
	}
	page, hasPage = named.PageRef()
	if !hasPage || page.Object != 9 || named.View() != "Fit" {
		t.Fatalf("named destination = %#v", named)
	}
}

func TestResolveDestinationRejectsTruncatedArray(t *testing.T) {
	d := &Document{}
	if got := d.ResolveDestination(Array{Ref{Object: 1}}); got != nil {
		t.Fatalf("truncated destination = %#v", got)
	}
}

func TestResolveDestinationFollowsMultiLevelIndirectDestinationChain(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{Name("D"): Ref{Object: 3}},
		{Object: 3}: Ref{Object: 4},
		{Object: 4}: Array{Ref{Object: 9}, Ref{Object: 5}},
		{Object: 5}: Name("Fit"),
	}}
	got, err := d.ResolveDestinationWithError(Ref{Object: 1})
	if err != nil || got == nil {
		t.Fatalf("indirect destination = %#v, err = %v", got, err)
	}
	page, ok := got.PageRef()
	if !ok || page.Object != 9 || got.View() != "Fit" {
		t.Fatalf("indirect destination = %#v", got)
	}
}

func TestResolveDestinationFollowsMultiLevelIndirectPageReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Ref{Object: 3},
		{Object: 3}: Array{Ref{Object: 4}, Name("Fit")},
		{Object: 4}: Ref{Object: 5},
		{Object: 5}: Dict{Name("Type"): Name("Page")},
	}}
	got, err := d.ResolveDestinationWithError(Ref{Object: 1})
	if err != nil || got == nil {
		t.Fatalf("indirect page destination = %#v, err=%v", got, err)
	}
	page, ok := got.PageRef()
	if !ok || page != (Ref{Object: 5}) {
		t.Fatalf("indirect page reference = %v, %v", page, ok)
	}
}

func TestResolveDestinationWithErrorReportsUnresolvedExplicitReferences(t *testing.T) {
	d := &Document{}
	if _, err := d.ResolveDestinationWithError(Ref{Object: 99}); err == nil {
		t.Fatal("unresolved destination root produced no error")
	}
	if _, err := d.ResolveDestinationWithError(Dict{Name("D"): Ref{Object: 99}}); err == nil {
		t.Fatal("unresolved destination dictionary value produced no error")
	}
	if _, err := d.ResolveDestinationWithError(Array{Ref{Object: 1}, Ref{Object: 99}}); err == nil {
		t.Fatal("unresolved destination view produced no error")
	}
}

func TestDestinationPosWithErrorReportsMalformedPageGeometry(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 3}: Dict{Name("Type"): Name("Page"), Name("MediaBox"): Ref{Object: 99}},
	}}
	destination := newDestination(Ref{Object: 3}, true, 0, false, "Fit", nil)
	if _, err := destination.PosWithError(d); err == nil {
		t.Fatal("expected destination geometry error")
	}
}

func TestDestinationBBoxWithErrorReportsMalformedPageGeometry(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 3}: Dict{Name("Type"): Name("Page"), Name("MediaBox"): Ref{Object: 99}},
	}}
	destination := newDestination(Ref{Object: 3}, true, 0, false, "Fit", nil)
	if _, err := destination.BBoxWithError(d); err == nil {
		t.Fatal("expected destination bbox geometry error")
	}
}

func TestDestinationTopLeftZoomWithErrorReportUnresolvedParameters(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 3}: Dict{Name("Type"): Name("Page")},
	}}
	if _, err := newDestination(Ref{Object: 3}, true, 0, false, "FitH", []Object{Ref{Object: 99}}).TopWithError(d); err == nil {
		t.Fatal("expected unresolved top parameter error")
	}
	if _, err := newDestination(Ref{Object: 3}, true, 0, false, "FitV", []Object{Ref{Object: 99}}).LeftWithError(d); err == nil {
		t.Fatal("expected unresolved left parameter error")
	}
	if _, err := newDestination(Ref{}, false, 0, false, "XYZ", []Object{Null{}, Null{}, Ref{Object: 99}}).ZoomWithError(d); err == nil {
		t.Fatal("expected unresolved zoom parameter error")
	}
}
