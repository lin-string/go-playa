package documentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestDestinationEntryOwnsValueSnapshots(t *testing.T) {
	value := primitives.Dict{primitives.Name("D"): primitives.String("original")}
	entry := documentdata.NewDestinationEntry("chapter", value)

	if entry.Name() != "chapter" {
		t.Fatalf("destination name = %q", entry.Name())
	}
	copyValue := entry.ValueCopy().(primitives.Dict)
	copyValue[primitives.Name("D")] = primitives.String("changed")
	if got := string(entry.ValueCopy().(primitives.Dict)[primitives.Name("D")].(primitives.String)); got != "original" {
		t.Fatalf("destination value was not isolated: %v", got)
	}
	if snapshot := entry.Finalize(); snapshot.Name() != entry.Name() {
		t.Fatalf("finalized destination name = %q", snapshot.Name())
	}
}
