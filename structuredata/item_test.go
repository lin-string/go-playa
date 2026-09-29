package structuredata_test

import (
	"testing"

	"github.com/lin-string/go-playa/structureconfig"
	"github.com/lin-string/go-playa/structuredata"
)

func TestItemOwnsContentPayload(t *testing.T) {
	content := structuredata.NewContent(structuredata.ContentSpec{
		Kind:    structureconfig.MarkedContent,
		MCID:    4,
		HasMCID: true,
	})
	item := structuredata.NewItem(structuredata.ItemSpec{
		Kind:    structureconfig.ItemMarkedContent,
		Content: content,
	})
	if item.Kind() != structureconfig.ItemMarkedContent {
		t.Fatalf("item kind = %q", item.Kind())
	}
	got, ok := item.ContentCopy()
	if !ok || got.MCID() != 4 || !got.HasMCID() {
		t.Fatalf("item content = %#v, ok=%v", got, ok)
	}
	if item.Finalize().Kind() != item.Kind() {
		t.Fatal("item finalize lost kind")
	}
}
