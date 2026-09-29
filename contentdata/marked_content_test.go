package contentdata

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestMarkedContentValueOwnsMetadataAndOperations(t *testing.T) {
	properties := primitives.Dict{
		primitives.Name("MCID"): primitives.Number(3),
		primitives.Name("Role"): primitives.String("original"),
	}
	ops := []ContentOp{NewContentOp("Tj", []primitives.Object{primitives.String("A")}, 9)}
	value := NewMarkedContent("P", primitives.Ref{Object: 8}, true, 3, true, "replacement", properties, ops)

	if value.Tag() != "P" || value.Page().Object != 8 || !value.HasPage() || value.MCID() != 3 || !value.HasMCID() || value.ActualText() != "replacement" {
		t.Fatalf("unexpected marked-content metadata: %#v", value)
	}
	propertyCopy := value.PropertiesCopy()
	propertyCopy[primitives.Name("Role")] = primitives.String("changed")
	if got := value.PropertiesCopy()[primitives.Name("Role")].(primitives.String); !bytes.Equal([]byte(got), []byte("original")) {
		t.Fatalf("PropertiesCopy changed the source: %q", got)
	}
	opCopy := value.OpsCopy()
	opCopy[0].OperandsCopy()[0] = primitives.String("changed")
	if got := value.OpsCopy()[0].OperandsCopy()[0].(primitives.String); !bytes.Equal([]byte(got), []byte("A")) {
		t.Fatalf("OpsCopy changed the source: %q", got)
	}

	finalized := value.Finalize()
	finalizedProperties := finalized.PropertiesCopy()
	finalizedProperties[primitives.Name("Role")] = primitives.String("changed again")
	if got := finalized.PropertiesCopy()[primitives.Name("Role")].(primitives.String); !bytes.Equal([]byte(got), []byte("original")) {
		t.Fatalf("Finalize did not preserve independent properties: %q", got)
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal marked-content value: %v", err)
	}
	var projection struct {
		Tag        string
		Page       primitives.Ref
		HasPage    bool
		MCID       int
		HasMCID    bool
		ActualText string
		Properties json.RawMessage
		Ops        []json.RawMessage
	}
	if err := json.Unmarshal(encoded, &projection); err != nil {
		t.Fatalf("unmarshal marked-content value: %v", err)
	}
	if projection.Tag != "P" || projection.Page.Object != 8 || !projection.HasPage || projection.MCID != 3 || !projection.HasMCID || projection.ActualText != "replacement" || len(projection.Ops) != 1 {
		t.Fatalf("unexpected JSON projection: %s", encoded)
	}
}
