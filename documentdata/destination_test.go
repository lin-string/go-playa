package documentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestDestinationOwnsParameterSnapshots(t *testing.T) {
	params := []primitives.Object{primitives.Dict{primitives.Name("X"): primitives.String("original")}}
	destination := documentdata.NewDestination(primitives.Ref{Object: 3}, true, 0, false, "XYZ", params)
	params[0].(primitives.Dict)[primitives.Name("X")] = primitives.String("source changed")

	if page, ok := destination.PageRef(); !ok || page.Object != 3 {
		t.Fatalf("destination page = %#v, %v", page, ok)
	}
	if destination.View() != "XYZ" {
		t.Fatalf("destination view = %q", destination.View())
	}
	copyParams := destination.ParamsCopy()
	copyParams[0].(primitives.Dict)[primitives.Name("X")] = primitives.String("changed")
	if got := string(destination.ParamsCopy()[0].(primitives.Dict)[primitives.Name("X")].(primitives.String)); got != "original" {
		t.Fatalf("destination parameters were not isolated from constructor input: %q", got)
	}
	if snapshot := destination.Finalize(); snapshot.View() != destination.View() {
		t.Fatalf("finalized destination view = %q", snapshot.View())
	}
}
