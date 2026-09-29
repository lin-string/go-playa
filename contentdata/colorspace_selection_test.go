package contentdata

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestColorSpaceSelectionOwnsDependencyFreeSnapshot(t *testing.T) {
	spec := primitives.Dict{primitives.Name("N"): primitives.String("original")}
	value := NewColorSpaceSelection(ColorSpaceSelectionSpec{
		Name: "CS1", Page: primitives.Ref{Object: 4}, HasPage: true,
		Stroke: true, Spec: spec,
	})
	spec[primitives.Name("N")] = primitives.String("changed")
	if value.Name() != "CS1" || !value.HasPage() || value.Page().Object != 4 || !value.Stroke() {
		t.Fatalf("color-space selection = %#v", value)
	}
	if got, _ := value.SpecCopy().(primitives.Dict)[primitives.Name("N")].(primitives.String); string(got) != "original" {
		t.Fatalf("SpecCopy() = %q", got)
	}
}
