package documentdata

import "github.com/lin-string/go-playa/pdftypes/primitives"

// DestinationEntry is one named destination value from a PDF document.
// Destination resolution remains document-owned because it needs page-tree
// and coordinate-space context; this type owns only the name/value pair.
type DestinationEntry struct {
	name  string
	value primitives.Object
}

// NewDestinationEntry constructs a named destination entry from parser data.
func NewDestinationEntry(name string, value primitives.Object) DestinationEntry {
	return DestinationEntry{name: name, value: value}
}

// Name returns the named destination key.
func (e DestinationEntry) Name() string { return e.name }

// ValueCopy returns an independent copy of the destination value.
func (e DestinationEntry) ValueCopy() primitives.Object { return cloneObject(e.value) }

// Finalize returns an independent snapshot of the destination entry.
func (e DestinationEntry) Finalize() DestinationEntry {
	return DestinationEntry{name: e.name, value: cloneObject(e.value)}
}
