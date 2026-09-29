package documentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// IndirectObject is the document-independent value yielded by an object
// iterator. The document owns lookup, ordering, and cache coordination; this
// type owns only the reference and its borrowed PDF value.
type IndirectObject struct {
	ref   primitives.Ref
	value primitives.Object
}

// NewIndirectObject constructs an indirect object snapshot from parser data.
// The value remains borrowed until ValueCopy or Finalize is requested.
func NewIndirectObject(ref primitives.Ref, value primitives.Object) IndirectObject {
	return IndirectObject{ref: ref, value: value}
}

// Ref returns the indirect reference identifying this object.
func (o IndirectObject) Ref() primitives.Ref { return o.ref }

// ValueCopy returns an independent copy of the indirect PDF object.
func (o IndirectObject) ValueCopy() primitives.Object { return cloneObject(o.value) }

// Finalize returns an independent snapshot of the indirect object.
func (o IndirectObject) Finalize() IndirectObject {
	return IndirectObject{ref: o.ref, value: cloneObject(o.value)}
}

func (o IndirectObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Ref primitives.Ref `json:"Ref"`
	}{o.ref})
}

// pdfObjectClone is implemented by document-owned PDF object values whose
// representation cannot live in primitives but still participates in an
// indirect-object snapshot.
type pdfObjectClone interface {
	ClonePDFObject() primitives.Object
}

func cloneObject(value primitives.Object) primitives.Object {
	if value == nil {
		return nil
	}
	if cloneable, ok := value.(pdfObjectClone); ok {
		return cloneable.ClonePDFObject()
	}
	switch value := value.(type) {
	case primitives.Array:
		if value == nil {
			return primitives.Array(nil)
		}
		out := make(primitives.Array, len(value))
		for i, item := range value {
			out[i] = cloneObject(item)
		}
		return out
	case primitives.InvalidArray:
		if value == nil {
			return primitives.InvalidArray(nil)
		}
		out := make(primitives.InvalidArray, len(value))
		for i, item := range value {
			out[i] = cloneObject(item)
		}
		return out
	case primitives.Dict:
		if value == nil {
			return primitives.Dict(nil)
		}
		out := make(primitives.Dict, len(value))
		for key, item := range value {
			out[key] = cloneObject(item)
		}
		return out
	case primitives.String:
		return primitives.String(primitives.CloneBytes([]byte(value)))
	default:
		return value
	}
}
