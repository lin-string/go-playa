package documentdata_test

import (
	"encoding/json"
	"testing"

	"github.com/lin-string/go-playa/documentdata"
)

func TestPageLabelSpecFormatsAndMarshalsAsAnOwnedValue(t *testing.T) {
	spec := documentdata.NewPageLabelSpec("R", "sec-", 4)
	if got := spec.Format(2); got != "sec-VI" {
		t.Fatalf("PageLabelSpec.Format() = %q, want %q", got, "sec-VI")
	}
	if got := spec.Style(); got != "R" {
		t.Fatalf("PageLabelSpec.Style() = %q, want R", got)
	}
	if got := spec.Prefix(); got != "sec-" {
		t.Fatalf("PageLabelSpec.Prefix() = %q, want sec-", got)
	}
	if got := spec.Start(); got != 4 {
		t.Fatalf("PageLabelSpec.Start() = %d, want 4", got)
	}
	encoded, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("json.Marshal(PageLabelSpec) = %v", err)
	}
	if got := string(encoded); got != `{"Style":"R","Prefix":"sec-","Start":4}` {
		t.Fatalf("PageLabelSpec JSON = %s", got)
	}
}

func TestPageLabelSpecDefaultsStartToOneAndRejectsNegativeOffsets(t *testing.T) {
	spec := documentdata.NewPageLabelSpec("D", "")
	if got := spec.Format(0); got != "1" {
		t.Fatalf("default PageLabelSpec.Format(0) = %q, want 1", got)
	}
	if got := spec.Format(-1); got != "" {
		t.Fatalf("negative PageLabelSpec offset = %q, want empty", got)
	}
}

func TestPageLabelSpecWithoutStyleUsesOnlyPrefix(t *testing.T) {
	spec := documentdata.NewPageLabelSpec("", "٤", 4)
	if got := spec.Format(0); got != "٤" {
		t.Fatalf("prefix-only PageLabelSpec.Format(0) = %q, want %q", got, "٤")
	}
}

func TestXRefEntryExposesImmutableScalarMetadata(t *testing.T) {
	entry := documentdata.NewXRefEntry(7, 2, 123, false, 9, 3, true)
	if entry.Object() != 7 || entry.Generation() != 2 || entry.Offset() != 123 || entry.Free() || entry.ObjectStream() != 9 || entry.ObjectIndex() != 3 || !entry.InObjectStream() {
		t.Fatalf("XRefEntry metadata = %#v", entry)
	}
	if got := entry.Finalize(); got != entry {
		t.Fatalf("XRefEntry.Finalize() = %#v, want equal value", got)
	}
}
