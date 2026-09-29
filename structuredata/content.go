// Package structuredata owns dependency-free tagged-PDF structure values.
package structuredata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes"
	"github.com/lin-string/go-playa/pdftypes/primitives"
	"github.com/lin-string/go-playa/structureconfig"
)

// Content is the document-independent value portion of one structure
// content item. Page lookup, marked-content extraction, and object resolution
// remain owned by document.
type Content struct {
	kind      structureconfig.ContentKind
	mcid      int
	hasMCID   bool
	page      primitives.Ref
	hasPage   bool
	stream    pdftypes.Stream
	hasStream bool
	objectRef primitives.Ref
	hasObject bool
	dict      primitives.Dict
}

// ContentSpec supplies the fields used to construct an owned Content value.
// NewContent recursively copies dictionaries and stream storage.
type ContentSpec struct {
	Kind      structureconfig.ContentKind
	MCID      int
	HasMCID   bool
	Page      primitives.Ref
	HasPage   bool
	Stream    pdftypes.Stream
	HasStream bool
	ObjectRef primitives.Ref
	HasObject bool
	Dict      primitives.Dict
}

// NewContent constructs an owned dependency-free structure content value.
func NewContent(spec ContentSpec) Content {
	return Content{
		kind: spec.Kind, mcid: spec.MCID, hasMCID: spec.HasMCID,
		page: spec.Page, hasPage: spec.HasPage, stream: spec.Stream.Finalize(),
		hasStream: spec.HasStream, objectRef: spec.ObjectRef,
		hasObject: spec.HasObject, dict: cloneDict(spec.Dict),
	}
}

func (c Content) Kind() structureconfig.ContentKind { return c.kind }
func (c Content) MCID() int                         { return c.mcid }
func (c Content) HasMCID() bool                     { return c.hasMCID }
func (c Content) Page() primitives.Ref              { return c.page }
func (c Content) HasPage() bool                     { return c.hasPage }
func (c Content) HasStream() bool                   { return c.hasStream }
func (c Content) ObjectRef() primitives.Ref         { return c.objectRef }
func (c Content) HasObject() bool                   { return c.hasObject }

// StreamCopy returns an independent copy of the optional marked-content
// stream.
func (c Content) StreamCopy() pdftypes.Stream {
	if !c.hasStream {
		return pdftypes.Stream{}
	}
	return c.stream.Finalize()
}

// DictCopy returns an independent copy of the source structure dictionary.
func (c Content) DictCopy() primitives.Dict { return cloneDict(c.dict) }

// Finalize returns an independent structure content snapshot.
func (c Content) Finalize() Content {
	return NewContent(ContentSpec{
		Kind: c.kind, MCID: c.mcid, HasMCID: c.hasMCID,
		Page: c.page, HasPage: c.hasPage, Stream: c.stream,
		HasStream: c.hasStream, ObjectRef: c.objectRef, HasObject: c.hasObject,
		Dict: c.dict,
	})
}

func (c Content) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind      structureconfig.ContentKind
		MCID      int
		HasMCID   bool
		Page      primitives.Ref
		HasPage   bool
		Stream    pdftypes.Stream
		HasStream bool
		ObjectRef primitives.Ref
		HasObject bool
		Dict      primitives.Dict
	}{
		Kind: c.kind, MCID: c.mcid, HasMCID: c.hasMCID,
		Page: c.page, HasPage: c.hasPage, Stream: c.stream,
		HasStream: c.hasStream, ObjectRef: c.objectRef, HasObject: c.hasObject,
		Dict: c.dict,
	})
}

func cloneDict(value primitives.Dict) primitives.Dict {
	if value == nil {
		return nil
	}
	return cloneObject(value).(primitives.Dict)
}

func cloneObject(value primitives.Object) primitives.Object {
	switch value := value.(type) {
	case primitives.Array:
		out := make(primitives.Array, len(value))
		for i, item := range value {
			out[i] = cloneObject(item)
		}
		return out
	case primitives.InvalidArray:
		out := make(primitives.InvalidArray, len(value))
		for i, item := range value {
			out[i] = cloneObject(item)
		}
		return out
	case primitives.Dict:
		out := make(primitives.Dict, len(value))
		for key, item := range value {
			out[key] = cloneObject(item)
		}
		return out
	case pdftypes.Stream:
		return value.Finalize()
	default:
		return value
	}
}
