package primitives

import "testing"

func TestRefStringUsesCanonicalIndirectReference(t *testing.T) {
	if got := (Ref{Object: 12, Generation: 3}).String(); got != "12 3 R" {
		t.Fatalf("Ref.String() = %q, want %q", got, "12 3 R")
	}
}
