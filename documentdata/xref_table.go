package documentdata

import (
	"encoding/json"
	"iter"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// XRefTable is one immutable cross-reference revision snapshot. Parser
// indexes and document cache coordination remain outside this value model.
type XRefTable struct {
	kind       string
	offset     int
	entries    []XRefEntry
	visible    []XRefEntry
	entryCount int
	trailer    primitives.Dict
}

// NewXRefTable constructs an owned cross-reference revision snapshot.
func NewXRefTable(kind string, offset int, entries, visible []XRefEntry, entryCount int, trailer primitives.Dict) XRefTable {
	return XRefTable{
		kind: kind, offset: offset, entries: cloneXRefEntries(entries),
		visible: cloneXRefEntries(visible), entryCount: entryCount,
		trailer: cloneXRefTrailer(trailer),
	}
}

// Kind returns the xref revision representation, such as "table" or
// "stream".
func (t XRefTable) Kind() string { return t.kind }

// Offset returns the byte offset of the xref revision.
func (t XRefTable) Offset() int { return t.offset }

// EntriesSeq lazily yields immutable scalar xref entries.
func (t XRefTable) EntriesSeq() iter.Seq[XRefEntry] {
	return func(yield func(XRefEntry) bool) {
		for _, entry := range t.entries {
			if !yield(entry) {
				return
			}
		}
	}
}

// EntriesCopy returns an independent copy of all revision entries.
func (t XRefTable) EntriesCopy() []XRefEntry { return cloneXRefEntries(t.entries) }

// VisibleEntriesCopy returns the entries visible through Playa's historical
// mapping traversal. It is distinct from EntriesCopy for hybrid revisions.
func (t XRefTable) VisibleEntriesCopy() []XRefEntry { return cloneXRefEntries(t.visible) }

// EntryCount reports the raw number of entries in the revision.
func (t XRefTable) EntryCount() int { return t.entryCount }

// TrailerCopy returns an independent trailer dictionary.
func (t XRefTable) TrailerCopy() primitives.Dict { return cloneXRefTrailer(t.trailer) }

// Finalize returns an independent xref revision snapshot.
func (t XRefTable) Finalize() XRefTable {
	return NewXRefTable(t.kind, t.offset, t.entries, t.visible, t.entryCount, t.trailer)
}

func (t XRefTable) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind    string          `json:"Kind"`
		Offset  int             `json:"Offset"`
		Entries []XRefEntry     `json:"Entries"`
		Trailer primitives.Dict `json:"Trailer"`
	}{t.kind, t.offset, t.EntriesCopy(), t.trailer})
}

func cloneXRefEntries(value []XRefEntry) []XRefEntry {
	if value == nil {
		return nil
	}
	return append([]XRefEntry(nil), value...)
}

func cloneXRefTrailer(value primitives.Dict) primitives.Dict {
	if value == nil {
		return nil
	}
	return cloneObject(value).(primitives.Dict)
}
