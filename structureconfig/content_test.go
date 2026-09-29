package structureconfig_test

import (
	"testing"

	"github.com/lin-string/go-playa/structureconfig"
)

func TestPublicStructureContentKindsRemainStable(t *testing.T) {
	if structureconfig.MarkedContent != "marked_content" || structureconfig.Object != "object" {
		t.Fatalf("structure content kinds = %q, %q", structureconfig.MarkedContent, structureconfig.Object)
	}
}

func TestPublicStructureItemKindsRemainStable(t *testing.T) {
	if structureconfig.ItemElement != "element" || structureconfig.ItemMarkedContent != "marked_content" || structureconfig.ItemObject != "object" {
		t.Fatalf("structure item kinds = %q, %q, %q", structureconfig.ItemElement, structureconfig.ItemMarkedContent, structureconfig.ItemObject)
	}
}
