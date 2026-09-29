package documentdata

import "encoding/json"

// XRefEntry is one raw cross-reference entry. It contains only immutable
// scalar metadata and is independent of a document's parser and caches.
type XRefEntry struct {
	object         int
	generation     int
	offset         int
	free           bool
	objectStream   int
	objectIndex    int
	inObjectStream bool
}

// NewXRefEntry constructs a cross-reference entry from parser metadata.
func NewXRefEntry(object, generation, offset int, free bool, objectStream, objectIndex int, inObjectStream bool) XRefEntry {
	return XRefEntry{
		object: object, generation: generation, offset: offset, free: free,
		objectStream: objectStream, objectIndex: objectIndex, inObjectStream: inObjectStream,
	}
}

func (e XRefEntry) Object() int          { return e.object }
func (e XRefEntry) Generation() int      { return e.generation }
func (e XRefEntry) Offset() int          { return e.offset }
func (e XRefEntry) Free() bool           { return e.free }
func (e XRefEntry) ObjectStream() int    { return e.objectStream }
func (e XRefEntry) ObjectIndex() int     { return e.objectIndex }
func (e XRefEntry) InObjectStream() bool { return e.inObjectStream }
func (e XRefEntry) Finalize() XRefEntry  { return e }

func (e XRefEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Object         int  `json:"Object"`
		Generation     int  `json:"Generation"`
		Offset         int  `json:"Offset"`
		Free           bool `json:"Free"`
		ObjectStream   int  `json:"ObjectStream"`
		ObjectIndex    int  `json:"ObjectIndex"`
		InObjectStream bool `json:"InObjectStream"`
	}{e.object, e.generation, e.offset, e.free, e.objectStream, e.objectIndex, e.inObjectStream})
}
