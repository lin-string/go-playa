package documentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestNameTreeEntryProvidesReadOnlyMetadataAndOwnedValues(t *testing.T) {
	value := primitives.Dict{primitives.Name("Value"): primitives.String("original")}
	entry := documentdata.NewNameTreeEntry("chapter", value)
	if entry.Name() != "chapter" {
		t.Fatalf("NameTreeEntry.Name() = %q", entry.Name())
	}

	copyValue := entry.ValueCopy().(primitives.Dict)
	copyValue[primitives.Name("Value")] = primitives.String("changed")
	if got := value[primitives.Name("Value")].(primitives.String); string(got) != "original" {
		t.Fatalf("NameTreeEntry.ValueCopy shared storage: %#v", value)
	}
	if finalized := entry.Finalize(); finalized.Name() != entry.Name() {
		t.Fatalf("NameTreeEntry.Finalize changed name: %#v", finalized)
	}
}
