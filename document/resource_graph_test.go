package document

import (
	"strings"
	"testing"
)

func TestResourceValidationRevisitsShallowerAlias(t *testing.T) {
	// The first path reaches the Form at the depth cutoff. A later, shallower
	// path to its alias must still validate the Form's malformed Properties.
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: newStream(Dict{Name("Subtype"): Name("Form"), Name("Resources"): Dict{Name("Properties"): Number(42)}}, nil),
		{Object: 2}: Ref{Object: 1},
	}}
	seen := make(map[Ref]int)
	deep := Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("F"): Ref{Object: 1}}}}
	if err := d.validatePropertiesResourcesSeen(deep, 31, seen); err != nil {
		t.Fatalf("depth cutoff: %v", err)
	}
	shallow := Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Alias"): Ref{Object: 2}}}}
	if err := d.validatePropertiesResourcesSeen(shallow, 0, seen); err == nil || !strings.Contains(err.Error(), "Properties") {
		t.Fatalf("shallower alias validation error = %v, want malformed Properties", err)
	}
}
