package structuredata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/structureconfig"
)

// Item is the document-independent payload of one structure element K item.
// Nested element traversal remains owned by document.StructureItem.
type Item struct {
	kind    structureconfig.ItemKind
	content Content
}

// ItemSpec supplies the value fields used to construct an owned Item.
type ItemSpec struct {
	Kind    structureconfig.ItemKind
	Content Content
}

// NewItem constructs an owned dependency-free structure item.
func NewItem(spec ItemSpec) Item {
	return Item{kind: spec.Kind, content: spec.Content.Finalize()}
}

func (i Item) Kind() structureconfig.ItemKind { return i.kind }

// ContentBorrowed returns the immutable content payload without rebuilding
// its recursive storage. It is valid only for the lifetime of this value.
func (i Item) ContentBorrowed() (Content, bool) {
	if i.kind != structureconfig.ItemMarkedContent && i.kind != structureconfig.ItemObject {
		return Content{}, false
	}
	return i.content, true
}

// ContentCopy returns an independent content payload when this item carries
// marked content or an object reference.
func (i Item) ContentCopy() (Content, bool) {
	if i.kind != structureconfig.ItemMarkedContent && i.kind != structureconfig.ItemObject {
		return Content{}, false
	}
	return i.content.Finalize(), true
}

// Finalize returns an independent structure item snapshot.
func (i Item) Finalize() Item {
	return NewItem(ItemSpec{Kind: i.kind, Content: i.content})
}

func (i Item) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind    structureconfig.ItemKind
		Content Content
	}{Kind: i.kind, Content: i.content})
}
