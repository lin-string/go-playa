package contentdata

import "github.com/lin-string/go-playa/pdftypes/primitives"

// Pattern is the document-independent value portion of a selected pattern.
type Pattern struct {
	name    string
	page    primitives.Ref
	hasPage bool
	stroke  bool
	gstate  GraphicsState
	dict    primitives.Dict
}

type PatternSpec struct {
	Name    string
	Page    primitives.Ref
	HasPage bool
	Stroke  bool
	GState  GraphicsState
	Dict    primitives.Dict
}

func NewPattern(spec PatternSpec) Pattern {
	return Pattern{
		name: spec.Name, page: spec.Page, hasPage: spec.HasPage, stroke: spec.Stroke,
		gstate: spec.GState.Finalize(), dict: cloneDict(spec.Dict),
	}
}

func (p Pattern) Name() string              { return p.name }
func (p Pattern) Page() primitives.Ref      { return p.page }
func (p Pattern) HasPage() bool             { return p.hasPage }
func (p Pattern) Stroke() bool              { return p.stroke }
func (p Pattern) GState() GraphicsState     { return p.gstate.Finalize() }
func (p Pattern) DictCopy() primitives.Dict { return cloneDict(p.dict) }
func (p Pattern) Finalize() Pattern {
	return NewPattern(PatternSpec{
		Name: p.name, Page: p.page, HasPage: p.hasPage, Stroke: p.stroke,
		GState: p.gstate, Dict: p.dict,
	})
}

// Shading is the document-independent value portion of a selected shading.
type Shading struct {
	name    string
	page    primitives.Ref
	hasPage bool
	gstate  GraphicsState
	dict    primitives.Dict
}

type ShadingSpec struct {
	Name    string
	Page    primitives.Ref
	HasPage bool
	GState  GraphicsState
	Dict    primitives.Dict
}

func NewShading(spec ShadingSpec) Shading {
	return Shading{
		name: spec.Name, page: spec.Page, hasPage: spec.HasPage,
		gstate: spec.GState.Finalize(), dict: cloneDict(spec.Dict),
	}
}

func (s Shading) Name() string              { return s.name }
func (s Shading) Page() primitives.Ref      { return s.page }
func (s Shading) HasPage() bool             { return s.hasPage }
func (s Shading) GState() GraphicsState     { return s.gstate.Finalize() }
func (s Shading) DictCopy() primitives.Dict { return cloneDict(s.dict) }
func (s Shading) Finalize() Shading {
	return NewShading(ShadingSpec{
		Name: s.name, Page: s.page, HasPage: s.hasPage,
		GState: s.gstate, Dict: s.dict,
	})
}
