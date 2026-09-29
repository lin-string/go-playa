// Package structureconfig owns dependency-free structure-content values.
package structureconfig

// ContentKind identifies a non-element item in a structure element's K entry.
type ContentKind string

const (
	MarkedContent ContentKind = "marked_content"
	Object        ContentKind = "object"
)

// ItemKind identifies one direct item in a structure element's /K entry.
// Unlike ContentKind, it also represents nested structure elements.
type ItemKind string

const (
	ItemElement       ItemKind = "element"
	ItemMarkedContent ItemKind = "marked_content"
	ItemObject        ItemKind = "object"
)
