package documentdata

import "github.com/lin-string/go-playa/pdftypes/primitives"

// NameTreeEntry is one entry from a PDF name tree. The tree traversal and
// cache remain document-owned; this value contains only borrowed entry data.
type NameTreeEntry struct {
	name  string
	value primitives.Object
}

// NewNameTreeEntry constructs a name-tree entry from parser data.
func NewNameTreeEntry(name string, value primitives.Object) NameTreeEntry {
	return NameTreeEntry{name: name, value: value}
}

// Name returns the name-tree key.
func (e NameTreeEntry) Name() string { return e.name }

// ValueCopy returns an independent copy of the name-tree value.
func (e NameTreeEntry) ValueCopy() primitives.Object { return cloneObject(e.value) }

// Finalize returns an independent snapshot of the name-tree entry.
func (e NameTreeEntry) Finalize() NameTreeEntry {
	return NameTreeEntry{name: e.name, value: cloneObject(e.value)}
}
