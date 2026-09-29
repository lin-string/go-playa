package primitives_test

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestPrimitiveValuesPreservePDFSemantics(t *testing.T) {
	value := []byte("source")
	copyValue := primitives.CloneBytes(value)
	copyValue[0] = 'S'
	if string(value) != "source" || string(copyValue) != "Source" {
		t.Fatalf("CloneBytes shared source storage: source=%q copy=%q", value, copyValue)
	}
	if got, ok := primitives.NumberValue(primitives.Number(3.5)); !ok || got != 3.5 {
		t.Fatalf("NumberValue = %v, %v", got, ok)
	}
	if got, ok := primitives.IntValue(primitives.Number(7)); !ok || got != 7 {
		t.Fatalf("IntValue = %v, %v", got, ok)
	}
	ref := primitives.Ref{Object: 4, Generation: 2}
	if got := ref.String(); got != "4 2 R" {
		t.Fatalf("Ref.String() = %q", got)
	}
}
