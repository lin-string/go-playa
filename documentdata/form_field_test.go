package documentdata

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestFormFieldOwnsDependencyFreeSnapshot(t *testing.T) {
	dict := primitives.Dict{primitives.Name("TU"): primitives.String("original")}
	values := []string{"one", "two"}
	options := []string{"One", "Two"}
	selected := []int{1}
	value := NewFormField(FormFieldSpec{
		Name: "field", FullName: "root.field", FieldType: "Ch", Flags: 4096,
		HasFlags: true, Values: values, Options: options, Selected: selected,
		Rect: [4]float64{1, 2, 3, 4}, HasRect: true, IsWidget: true, Dict: dict,
	})

	dict[primitives.Name("TU")] = primitives.String("changed")
	values[0] = "changed"
	options[0] = "changed"
	selected[0] = 0

	if value.Name() != "field" || value.FullName() != "root.field" || value.FieldType() != "Ch" || !value.HasFlags() || value.Flags() != 4096 {
		t.Fatalf("form field metadata = %#v", value)
	}
	if got := value.ValuesCopy(); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("ValuesCopy() = %#v", got)
	}
	if got := value.OptionsCopy(); len(got) != 2 || got[0] != "One" || got[1] != "Two" {
		t.Fatalf("OptionsCopy() = %#v", got)
	}
	if got := value.SelectedCopy(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("SelectedCopy() = %#v", got)
	}
	if got, _ := value.DictCopy()[primitives.Name("TU")].(primitives.String); string(got) != "original" {
		t.Fatalf("DictCopy() TU = %q", got)
	}

	finalized := value.Finalize()
	if finalized.Name() != value.Name() || finalized.Rect() != value.Rect() || !finalized.IsWidget() {
		t.Fatalf("Finalize() = %#v", finalized)
	}
}
