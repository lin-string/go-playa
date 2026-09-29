package contentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestPathOwnsDependencyFreeState(t *testing.T) {
	raw := []geometry.PathSegment{geometry.NewPathSegment("m", [2]float64{1, 2})}
	marked := primitives.Dict{primitives.Name("O"): primitives.String("original")}
	path := contentdata.NewPath(contentdata.PathSpec{
		Page: primitives.Ref{Object: 2}, HasPage: true, RawSegments: raw, Segments: raw,
		Stroke: true, BBox: [4]float64{1, 2, 3, 4}, MarkedProperties: marked, HasMCID: true, MCID: 7,
	})
	raw[0] = geometry.NewPathSegment("l", [2]float64{9, 9})
	marked[primitives.Name("O")] = primitives.String("changed")
	if !path.HasPage() || path.Page() != (primitives.Ref{Object: 2}) || !path.Stroke() || path.MCID() != 7 || !path.HasMCID() {
		t.Fatalf("path metadata = %#v", path)
	}
	if got := path.RawSegmentsCopy()[0].PointsCopy()[0]; got != [2]float64{1, 2} {
		t.Fatalf("path retained caller segment storage: %v", got)
	}
	if value, _ := path.MarkedPropertiesCopy()[primitives.Name("O")].(primitives.String); string(value) != "original" {
		t.Fatalf("path retained caller properties: %q", value)
	}
	snapshot := path.Finalize()
	copy := snapshot.MarkedPropertiesCopy()
	copy[primitives.Name("O")] = primitives.String("snapshot")
	if value, _ := path.MarkedPropertiesCopy()[primitives.Name("O")].(primitives.String); string(value) != "original" {
		t.Fatalf("path finalize shared properties: %q", value)
	}
}

func TestPathSpecBorrowedProjectsWithoutCopying(t *testing.T) {
	raw := []geometry.PathSegment{geometry.NewPathSegment("m", [2]float64{1, 2})}
	device := []geometry.PathSegment{geometry.NewPathSegment("l", [2]float64{3, 4})}
	marked := primitives.Dict{primitives.Name("O"): primitives.String("original")}
	path := contentdata.NewPathBorrowed(contentdata.PathSpec{
		RawSegments: raw, Segments: device, MarkedProperties: marked,
	})
	spec := path.SpecBorrowed()
	spec.RawSegments[0] = geometry.NewPathSegment("m", [2]float64{5, 6})
	spec.MarkedProperties[primitives.Name("O")] = primitives.String("changed")
	if got := path.RawSegmentsCopy()[0].PointsCopy()[0]; got != [2]float64{5, 6} {
		t.Fatalf("borrowed spec copied path segments: %v", got)
	}
	if got, _ := path.MarkedPropertiesCopy()[primitives.Name("O")].(primitives.String); string(got) != "changed" {
		t.Fatalf("borrowed spec copied marked properties: %q", got)
	}
	owned := path.Finalize()
	spec.RawSegments[0] = geometry.NewPathSegment("m", [2]float64{7, 8})
	if got := owned.RawSegmentsCopy()[0].PointsCopy()[0]; got != [2]float64{5, 6} {
		t.Fatalf("finalized path retained borrowed segments: %v", got)
	}
}

func TestPathGeometrySharesBorrowedContext(t *testing.T) {
	path := contentdata.NewPathBorrowed(contentdata.PathSpec{
		Page: primitives.Ref{Object: 9}, HasPage: true,
		GState: contentdata.DefaultGraphicsState(), MarkedTag: "Span",
	})
	raw := []geometry.PathSegment{geometry.NewPathSegment("m", [2]float64{1, 2})}
	device := []geometry.PathSegment{geometry.NewPathSegment("l", [2]float64{3, 4})}
	derived := path.WithGeometryBorrowed(raw, device, [4]float64{1, 2, 3, 4})
	if derived.ContextBorrowed() != path.ContextBorrowed() {
		t.Fatal("derived path duplicated its immutable context")
	}
	if derived.Page() != (primitives.Ref{Object: 9}) || derived.MarkedTag() != "Span" || derived.BBox() != [4]float64{1, 2, 3, 4} {
		t.Fatalf("derived path lost context or geometry: %#v", derived.SpecBorrowed())
	}
}
