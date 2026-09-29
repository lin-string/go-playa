package contentdata

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestContentOpValueAndFinalizeOwnership(t *testing.T) {
	operands := []primitives.Object{
		primitives.Name("F1"),
		primitives.Array{primitives.String("original")},
	}
	value := NewContentOp("TJ", operands, 17)

	if value.Operator() != "TJ" || value.Offset() != 17 {
		t.Fatalf("unexpected content operation metadata: %#v", value)
	}
	borrowed := value.OperandsBorrowed()
	if len(borrowed) != 2 || borrowed[0] != primitives.Name("F1") {
		t.Fatalf("unexpected borrowed operands: %#v", borrowed)
	}
	copy := value.OperandsCopy()
	copy[1].(primitives.Array)[0] = primitives.String("changed")
	if got := value.OperandsCopy()[1].(primitives.Array)[0].(primitives.String); !bytes.Equal([]byte(got), []byte("original")) {
		t.Fatalf("OperandsCopy changed the source: %q", got)
	}

	finalized := value.Finalize()
	finalizedCopy := finalized.OperandsCopy()
	finalizedCopy[1].(primitives.Array)[0] = primitives.String("changed again")
	if got := finalized.OperandsCopy()[1].(primitives.Array)[0].(primitives.String); !bytes.Equal([]byte(got), []byte("original")) {
		t.Fatalf("Finalize did not preserve independent operands: %q", got)
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal content operation: %v", err)
	}
	var projection struct {
		Operator string
		Offset   int
		Operands []json.RawMessage
	}
	if err := json.Unmarshal(encoded, &projection); err != nil {
		t.Fatalf("unmarshal content operation: %v", err)
	}
	if projection.Operator != "TJ" || projection.Offset != 17 || len(projection.Operands) != 2 {
		t.Fatalf("unexpected JSON projection: %s", encoded)
	}
}
