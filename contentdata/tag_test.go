package contentdata

import (
	"testing"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestTagOwnsMutableState(t *testing.T) {
	properties := primitives.Dict{primitives.Name("P"): primitives.String("value")}
	markedProperties := primitives.Dict{primitives.Name("M"): primitives.String("marked")}
	markedStack := []MarkedContentContext{
		NewMarkedContentContext("Span", primitives.Dict{primitives.Name("S"): primitives.String("stack")}, "", 3, true),
	}
	value := NewTag(TagSpec{
		Name: "Point", Page: primitives.Ref{Object: 7}, HasPage: true,
		Properties: properties, ActualText: "replacement", MCID: 5, HasMCID: true,
		MarkedTag: "P", MarkedProperties: markedProperties, MarkedStack: markedStack,
	})

	properties[primitives.Name("P")] = primitives.String("changed")
	markedProperties[primitives.Name("M")] = primitives.String("changed")
	markedStack[0] = MarkedContentContext{}
	if got := string(value.PropertiesCopy()[primitives.Name("P")].(primitives.String)); got != "value" {
		t.Fatalf("properties = %q", got)
	}
	if got := string(value.MarkedPropertiesCopy()[primitives.Name("M")].(primitives.String)); got != "marked" {
		t.Fatalf("marked properties = %q", got)
	}
	if got := value.MarkedStackCopy()[0].Tag(); got != "Span" {
		t.Fatalf("marked stack tag = %q", got)
	}
	if value.Name() != "Point" || value.Page().Object != 7 || !value.HasPage() || value.ActualText() != "replacement" || value.MCID() != 5 || !value.HasMCID() || value.MarkedTag() != "P" {
		t.Fatalf("scalar metadata was not preserved: %#v", value)
	}
}

func TestBorrowedTagFinalizeDetachesMutableState(t *testing.T) {
	properties := primitives.Dict{primitives.Name("P"): primitives.String("value")}
	value := NewTagBorrowed(TagSpec{Properties: properties})
	finalized := value.Finalize()
	properties[primitives.Name("P")] = primitives.String("changed")
	if got := string(finalized.PropertiesCopy()[primitives.Name("P")].(primitives.String)); got != "value" {
		t.Fatalf("finalized properties = %q", got)
	}
}
