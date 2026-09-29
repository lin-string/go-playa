package contentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestXObjectOwnsDependencyFreeStateAndFinalizes(t *testing.T) {
	stream := pdftypes.NewStream(primitives.Dict{
		primitives.Name("Marker"): primitives.String("original"),
	}, []byte("q"))
	resources := primitives.Dict{
		primitives.Name("XObject"): primitives.String("resource"),
	}
	value := contentdata.NewXObject(contentdata.XObjectSpec{
		Name: "Form", Ref: primitives.Ref{Object: 7}, Page: primitives.Ref{Object: 3}, HasPage: true,
		Stream: stream, Matrix: geometry.Matrix{1, 0, 0, 1, 10, 20}, BBox: [4]float64{0, 0, 100, 200},
		Resources: resources, HasDeclaredResources: true, DeclaredResources: resources,
		ResourceContext: resources, Path: "Form", MarkedTag: "P",
		MarkedProperties: primitives.Dict{primitives.Name("MCID"): primitives.Number(4)}, MCID: 4, HasMCID: true,
	})

	resources[primitives.Name("XObject")] = primitives.String("changed")
	gotResource, ok := value.ResourcesCopy()[primitives.Name("XObject")].(primitives.String)
	if !ok || string(gotResource) != "resource" {
		t.Fatalf("NewXObject retained caller resources: %#v", gotResource)
	}
	gotMarker, ok := value.StreamCopy().DictCopy()[primitives.Name("Marker")].(primitives.String)
	if !ok || string(gotMarker) != "original" {
		t.Fatalf("NewXObject retained caller stream: %#v", gotMarker)
	}

	snapshot := value.Finalize()
	if snapshot.Name() != "Form" || snapshot.Ref() != (primitives.Ref{Object: 7}) || snapshot.Page() != (primitives.Ref{Object: 3}) || !snapshot.HasPage() {
		t.Fatalf("XObject scalar metadata = %#v", snapshot)
	}
	if snapshot.Path() != "Form" || snapshot.MarkedTag() != "P" || !snapshot.HasDeclaredResources() || !snapshot.HasMCID() || snapshot.MCID() != 4 {
		t.Fatalf("XObject metadata = %#v", snapshot)
	}
	if got := snapshot.StreamCopy().DataBorrowed(); string(got) != "q" {
		t.Fatalf("XObject stream = %q", got)
	}
}
