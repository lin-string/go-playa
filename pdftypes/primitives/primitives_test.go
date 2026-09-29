package primitives

import "testing"

func TestPrimitiveValueHelpers(t *testing.T) {
	if got, ok := NumberValue(Number(3.5)); !ok || got != 3.5 {
		t.Fatalf("NumberValue = %v, %v", got, ok)
	}
	if got, ok := IntValue(Number(7)); !ok || got != 7 {
		t.Fatalf("IntValue = %v, %v", got, ok)
	}
	if _, ok := IntValue(Number(1.5)); ok {
		t.Fatal("fractional number accepted as integer")
	}
	if got, ok := NameValue(Name("Width")); !ok || got != "Width" {
		t.Fatalf("NameValue = %q, %v", got, ok)
	}
	ref := Ref{Object: 4, Generation: 2}
	if got, ok := RefValue(ref); !ok || got != ref {
		t.Fatalf("RefValue = %#v, %v", got, ok)
	}
}
