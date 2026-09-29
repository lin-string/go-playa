package contentdata

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestMarkedContentContextOwnsValueData(t *testing.T) {
	properties := primitives.Dict{
		primitives.Name("Role"): primitives.String("paragraph"),
		primitives.Name("Nested"): primitives.Dict{
			primitives.Name("Value"): primitives.String("original"),
		},
	}
	value := NewMarkedContentContext("P", properties, "replacement", 7, true)

	if value.Tag() != "P" || value.ActualText() != "replacement" || value.MCID() != 7 || !value.HasMCID() {
		t.Fatalf("unexpected marked-content metadata: %#v", value)
	}
	copy := value.PropertiesCopy()
	copy[primitives.Name("Role")] = primitives.String("changed")
	nested := copy[primitives.Name("Nested")].(primitives.Dict)
	nested[primitives.Name("Value")] = primitives.String("changed")
	original := value.PropertiesCopy()
	if !bytes.Equal([]byte(original[primitives.Name("Role")].(primitives.String)), []byte("paragraph")) {
		t.Fatalf("PropertiesCopy changed the original value: %#v", original)
	}
	if !bytes.Equal([]byte(original[primitives.Name("Nested")].(primitives.Dict)[primitives.Name("Value")].(primitives.String)), []byte("original")) {
		t.Fatalf("nested PropertiesCopy changed the original value: %#v", original)
	}

	finalized := value.Finalize()
	finalizedCopy := finalized.PropertiesCopy()
	finalizedCopy[primitives.Name("Role")] = primitives.String("changed again")
	if !bytes.Equal([]byte(finalized.PropertiesCopy()[primitives.Name("Role")].(primitives.String)), []byte("paragraph")) {
		t.Fatalf("Finalize did not preserve independent value ownership")
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal marked-content value: %v", err)
	}
	var projection struct {
		Tag        string
		Properties json.RawMessage
		ActualText string
		MCID       int
		HasMCID    bool
	}
	if err := json.Unmarshal(encoded, &projection); err != nil {
		t.Fatalf("unmarshal marked-content projection: %v", err)
	}
	if projection.Tag != "P" || projection.ActualText != "replacement" || projection.MCID != 7 || !projection.HasMCID {
		t.Fatalf("unexpected JSON projection: %s", encoded)
	}
}

func TestMarkedContentContextBorrowedValueFinalizesOnBoundary(t *testing.T) {
	properties := primitives.Dict{primitives.Name("Role"): primitives.String("original")}
	value := NewMarkedContentContextBorrowed("P", properties, "", 0, false)
	properties[primitives.Name("Role")] = primitives.String("borrowed")
	if got := value.PropertiesCopy()[primitives.Name("Role")].(primitives.String); string(got) != "borrowed" {
		t.Fatalf("borrowed properties = %q", got)
	}

	finalized := value.Finalize()
	properties[primitives.Name("Role")] = primitives.String("changed after finalize")
	if got := finalized.PropertiesCopy()[primitives.Name("Role")].(primitives.String); string(got) != "borrowed" {
		t.Fatalf("finalized properties = %q", got)
	}
}
