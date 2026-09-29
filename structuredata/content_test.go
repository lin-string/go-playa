package structuredata_test

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes"
	"github.com/lin-string/go-playa/pdftypes/primitives"
	"github.com/lin-string/go-playa/structureconfig"
	"github.com/lin-string/go-playa/structuredata"
)

func TestContentOwnsDependencyFreeStructureState(t *testing.T) {
	value := structuredata.NewContent(structuredata.ContentSpec{
		Kind:      structureconfig.MarkedContent,
		MCID:      7,
		HasMCID:   true,
		Page:      primitives.Ref{Object: 2},
		HasPage:   true,
		Stream:    pdftypes.NewStream(primitives.Dict{primitives.Name("Length"): primitives.Number(1)}, []byte("x")),
		HasStream: true,
		ObjectRef: primitives.Ref{Object: 3},
		HasObject: true,
		Dict:      primitives.Dict{primitives.Name("MCID"): primitives.Number(7)},
	})
	if value.Kind() != structureconfig.MarkedContent || value.MCID() != 7 || !value.HasMCID() || value.Page() != (primitives.Ref{Object: 2}) || !value.HasPage() {
		t.Fatalf("content identity = %#v", value)
	}
	if !value.HasStream() || value.StreamCopy().DataBorrowed()[0] != 'x' || !value.HasObject() || value.ObjectRef() != (primitives.Ref{Object: 3}) {
		t.Fatalf("content resources = %#v", value)
	}
	dict := value.DictCopy()
	dict[primitives.Name("MCID")] = primitives.Number(9)
	if value.DictCopy()[primitives.Name("MCID")] != primitives.Number(7) {
		t.Fatal("content dictionary copy exposed source")
	}
	if value.Finalize().Kind() != value.Kind() {
		t.Fatal("content finalize lost kind")
	}
}
