package contentdata

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestExtGStateOwnsDependencyFreeSnapshot(t *testing.T) {
	dict := primitives.Dict{primitives.Name("LW"): primitives.Number(2)}
	value := NewExtGState(ExtGStateSpec{
		Name: "GS1", Page: primitives.Ref{Object: 3}, HasPage: true,
		GState: DefaultGraphicsState(), Dict: dict,
	})
	dict[primitives.Name("LW")] = primitives.Number(9)

	if value.Name() != "GS1" || !value.HasPage() || value.Page().Object != 3 {
		t.Fatalf("ExtGState metadata = %#v", value)
	}
	if got, _ := value.DictCopy()[primitives.Name("LW")].(primitives.Number); got != 2 {
		t.Fatalf("DictCopy() LW = %v", got)
	}
	if finalized := value.Finalize(); finalized.Name() != value.Name() {
		t.Fatalf("Finalize() = %#v", finalized)
	}
}
