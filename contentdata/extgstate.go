package contentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// ExtGState is the document-independent value portion of one resource-backed
// graphics-state selection. Resource lookup and content traversal remain in
// document.
type ExtGState struct {
	name    string
	page    primitives.Ref
	hasPage bool
	gstate  GraphicsState
	dict    primitives.Dict
}

// ExtGStateSpec supplies the fields used to construct an owned ExtGState.
type ExtGStateSpec struct {
	Name    string
	Page    primitives.Ref
	HasPage bool
	GState  GraphicsState
	Dict    primitives.Dict
}

// NewExtGState constructs an owned graphics-state resource value.
func NewExtGState(spec ExtGStateSpec) ExtGState {
	return ExtGState{
		name: spec.Name, page: spec.Page, hasPage: spec.HasPage,
		gstate: spec.GState.Finalize(), dict: cloneDict(spec.Dict),
	}
}

func (e ExtGState) Name() string              { return e.name }
func (e ExtGState) Page() primitives.Ref      { return e.page }
func (e ExtGState) HasPage() bool             { return e.hasPage }
func (e ExtGState) GState() GraphicsState     { return e.gstate.Finalize() }
func (e ExtGState) DictCopy() primitives.Dict { return cloneDict(e.dict) }

// Finalize returns an independent snapshot of the resource and applied state.
func (e ExtGState) Finalize() ExtGState {
	return NewExtGState(ExtGStateSpec{
		Name: e.name, Page: e.page, HasPage: e.hasPage,
		GState: e.gstate, Dict: e.dict,
	})
}

func (e ExtGState) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name    string
		Page    primitives.Ref
		HasPage bool
		GState  GraphicsState   `json:"gstate"`
		Dict    primitives.Dict `json:"Dict"`
	}{Name: e.name, Page: e.page, HasPage: e.hasPage, GState: e.gstate, Dict: e.dict})
}
