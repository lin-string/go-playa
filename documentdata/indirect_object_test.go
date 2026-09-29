package documentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestIndirectObjectOwnsAnIndependentPrimitiveSnapshot(t *testing.T) {
	value := primitives.Dict{
		primitives.Name("Text"): primitives.String("original"),
		primitives.Name("Items"): primitives.Array{primitives.Dict{
			primitives.Name("Value"): primitives.String("nested"),
		}},
	}
	object := documentdata.NewIndirectObject(primitives.Ref{Object: 7, Generation: 2}, value)

	if got := object.Ref(); got != (primitives.Ref{Object: 7, Generation: 2}) {
		t.Fatalf("IndirectObject.Ref() = %#v", got)
	}
	copyValue, ok := object.ValueCopy().(primitives.Dict)
	if !ok {
		t.Fatalf("IndirectObject.ValueCopy() type = %T", object.ValueCopy())
	}
	copyValue[primitives.Name("Text")] = primitives.String("changed")
	copyValue[primitives.Name("Items")].(primitives.Array)[0].(primitives.Dict)[primitives.Name("Value")] = primitives.String("changed")

	if got := value[primitives.Name("Text")]; string(got.(primitives.String)) != "original" {
		t.Fatalf("ValueCopy shared top-level storage: %#v", value)
	}
	if got := value[primitives.Name("Items")].(primitives.Array)[0].(primitives.Dict)[primitives.Name("Value")]; string(got.(primitives.String)) != "nested" {
		t.Fatalf("ValueCopy shared nested storage: %#v", value)
	}

	finalized := object.Finalize()
	if finalized.Ref() != object.Ref() {
		t.Fatalf("IndirectObject.Finalize() changed ref: %#v", finalized.Ref())
	}
}
