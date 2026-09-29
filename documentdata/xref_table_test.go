package documentdata

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestXRefTableOwnsRevisionSnapshot(t *testing.T) {
	entries := []XRefEntry{NewXRefEntry(1, 0, 10, false, 0, 0, false)}
	visible := []XRefEntry{NewXRefEntry(1, 0, 10, false, 0, 0, false)}
	trailer := primitives.Dict{primitives.Name("Size"): primitives.Number(2)}
	table := NewXRefTable("table", 10, entries, visible, 1, trailer)

	entries[0] = NewXRefEntry(99, 0, 99, false, 0, 0, false)
	visible[0] = NewXRefEntry(98, 0, 98, false, 0, 0, false)
	trailer[primitives.Name("Size")] = primitives.Number(99)

	if got := table.EntriesCopy()[0].Object(); got != 1 {
		t.Fatalf("EntriesCopy() object = %d, want 1", got)
	}
	if got := table.VisibleEntriesCopy()[0].Object(); got != 1 {
		t.Fatalf("VisibleEntriesCopy() object = %d, want 1", got)
	}
	if got, ok := primitives.NumberValue(table.TrailerCopy()[primitives.Name("Size")]); !ok || got != 2 {
		t.Fatalf("TrailerCopy() size = %v, %v; want 2", got, ok)
	}
	finalized := table.Finalize()
	if finalized.Kind() != "table" || finalized.Offset() != 10 || finalized.EntryCount() != 1 {
		t.Fatalf("Finalize() metadata = %#v", finalized)
	}

	fallback := NewXRefTable("fallback", 0, nil, nil, 0, nil)
	if fallback.TrailerCopy() != nil {
		t.Fatalf("nil trailer became non-nil: %#v", fallback.TrailerCopy())
	}
}
