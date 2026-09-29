package contentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// Path is the dependency-free value portion of one painted path. Page
// resolution and ParentTree lookup remain owned by document.PathObject.
type Path struct {
	rawSegments []geometry.PathSegment
	segments    []geometry.PathSegment
	bbox        [4]float64
	context     *PathContext
}

// PathContext is immutable graphics and marked-content state shared by
// geometry fragments split from one painted PDF path.
type PathContext struct {
	page             primitives.Ref
	hasPage          bool
	stroke           bool
	fill             bool
	evenOdd          bool
	clip             bool
	clipEvenOdd      bool
	gstate           GraphicsState
	markedTag        string
	markedProperties primitives.Dict
	markedStack      []MarkedContentContext
	actualText       string
	mcid             int
	hasMCID          bool
}

// PathSpec supplies the dependency-free fields used to construct a Path.
type PathSpec struct {
	Page             primitives.Ref
	HasPage          bool
	RawSegments      []geometry.PathSegment
	Segments         []geometry.PathSegment
	Stroke           bool
	Fill             bool
	EvenOdd          bool
	Clip             bool
	ClipEvenOdd      bool
	BBox             [4]float64
	GState           GraphicsState
	MarkedTag        string
	MarkedProperties primitives.Dict
	MarkedStack      []MarkedContentContext
	ActualText       string
	MCID             int
	HasMCID          bool
}

func pathContextFromSpec(spec PathSpec, owned bool) *PathContext {
	context := &PathContext{
		page: spec.Page, hasPage: spec.HasPage,
		stroke: spec.Stroke, fill: spec.Fill, evenOdd: spec.EvenOdd,
		clip: spec.Clip, clipEvenOdd: spec.ClipEvenOdd,
		gstate: spec.GState, markedTag: spec.MarkedTag,
		markedProperties: spec.MarkedProperties, markedStack: spec.MarkedStack,
		actualText: spec.ActualText, mcid: spec.MCID, hasMCID: spec.HasMCID,
	}
	if owned {
		context.gstate = spec.GState.Finalize()
		context.markedProperties = cloneDict(spec.MarkedProperties)
		context.markedStack = cloneMarkedStack(spec.MarkedStack)
	}
	return context
}

func newPathWithContext(spec PathSpec, context *PathContext) Path {
	return Path{rawSegments: spec.RawSegments, segments: spec.Segments, bbox: spec.BBox, context: context}
}

// NewPath constructs an owned path value.
func NewPath(spec PathSpec) Path {
	owned := spec
	owned.RawSegments = clonePathSegments(spec.RawSegments)
	owned.Segments = clonePathSegments(spec.Segments)
	return newPathWithContext(owned, pathContextFromSpec(spec, true))
}

// NewPathBorrowed constructs a low-allocation value over slices owned by the
// caller. Use Finalize or the Copy accessors before retaining it.
func NewPathBorrowed(spec PathSpec) Path {
	return NewPathBorrowedWithContext(spec, NewPathContextBorrowed(spec))
}

// NewPathContextBorrowed constructs immutable shared path state over
// caller-owned values.
func NewPathContextBorrowed(spec PathSpec) *PathContext {
	return pathContextFromSpec(spec, false)
}

// NewPathBorrowedWithContext constructs borrowed path geometry sharing an
// immutable context with other paths from the same graphics state.
func NewPathBorrowedWithContext(spec PathSpec, context *PathContext) Path {
	return newPathWithContext(spec, context)
}

// WithGeometryBorrowed returns a path over caller-owned geometry while sharing
// the source path's immutable graphics and marked-content context.
func (p Path) WithGeometryBorrowed(raw, device []geometry.PathSegment, bbox [4]float64) Path {
	return Path{rawSegments: raw, segments: device, bbox: bbox, context: p.context}
}

// ContextBorrowed returns the immutable context shared by derived path
// geometry. Finalize the path before retaining mutable values from it.
func (p Path) ContextBorrowed() *PathContext { return p.context }

// SpecBorrowed returns the complete construction view without copying mutable
// slices or dictionaries. Callers must finalize the path before retaining the
// returned state beyond the lifetime of its owner.
func (p Path) SpecBorrowed() PathSpec {
	if p.context == nil {
		return PathSpec{RawSegments: p.rawSegments, Segments: p.segments, BBox: p.bbox}
	}
	return PathSpec{
		Page: p.context.page, HasPage: p.context.hasPage, RawSegments: p.rawSegments, Segments: p.segments,
		Stroke: p.context.stroke, Fill: p.context.fill, EvenOdd: p.context.evenOdd, Clip: p.context.clip,
		ClipEvenOdd: p.context.clipEvenOdd, BBox: p.bbox, GState: p.context.gstate,
		MarkedTag: p.context.markedTag, MarkedProperties: p.context.markedProperties,
		MarkedStack: p.context.markedStack, ActualText: p.context.actualText,
		MCID: p.context.mcid, HasMCID: p.context.hasMCID,
	}
}

func (p Path) Page() primitives.Ref {
	if p.context == nil {
		return primitives.Ref{}
	}
	return p.context.page
}
func (p Path) HasPage() bool                           { return p.context != nil && p.context.hasPage }
func (p Path) RawSegmentsCopy() []geometry.PathSegment { return clonePathSegments(p.rawSegments) }
func (p Path) SegmentsCopy() []geometry.PathSegment    { return clonePathSegments(p.segments) }
func (p Path) Stroke() bool                            { return p.context != nil && p.context.stroke }
func (p Path) Fill() bool                              { return p.context != nil && p.context.fill }
func (p Path) EvenOdd() bool                           { return p.context != nil && p.context.evenOdd }
func (p Path) Clip() bool                              { return p.context != nil && p.context.clip }
func (p Path) ClipEvenOdd() bool                       { return p.context != nil && p.context.clipEvenOdd }
func (p Path) BBox() [4]float64                        { return p.bbox }
func (p Path) GState() GraphicsState {
	if p.context == nil {
		return GraphicsState{}
	}
	return p.context.gstate.Finalize()
}
func (p Path) MarkedTag() string {
	if p.context == nil {
		return ""
	}
	return p.context.markedTag
}
func (p Path) MarkedPropertiesCopy() primitives.Dict {
	if p.context == nil {
		return nil
	}
	return cloneDict(p.context.markedProperties)
}
func (p Path) MarkedStackCopy() []MarkedContentContext {
	if p.context == nil {
		return nil
	}
	return cloneMarkedStack(p.context.markedStack)
}
func (p Path) ActualText() string {
	if p.context == nil {
		return ""
	}
	return p.context.actualText
}
func (p Path) MCID() int {
	if p.context == nil {
		return 0
	}
	return p.context.mcid
}
func (p Path) HasMCID() bool { return p.context != nil && p.context.hasMCID }

// Finalize returns an independent path snapshot.
func (p Path) Finalize() Path {
	return NewPath(p.SpecBorrowed())
}

func (p Path) MarshalJSON() ([]byte, error) {
	spec := p.SpecBorrowed()
	return json.Marshal(struct {
		Page             primitives.Ref
		HasPage          bool
		Stroke           bool
		Fill             bool
		EvenOdd          bool
		Clip             bool
		ClipEvenOdd      bool
		BBox             [4]float64
		GState           GraphicsState
		MarkedTag        string
		ActualText       string
		MCID             int
		HasMCID          bool
		RawSegments      []geometry.PathSegment
		Segments         []geometry.PathSegment
		MarkedProperties primitives.Dict
		MarkedStack      []MarkedContentContext
	}{
		Page: spec.Page, HasPage: spec.HasPage, Stroke: spec.Stroke, Fill: spec.Fill,
		EvenOdd: spec.EvenOdd, Clip: spec.Clip, ClipEvenOdd: spec.ClipEvenOdd, BBox: spec.BBox,
		GState: spec.GState, MarkedTag: spec.MarkedTag, ActualText: spec.ActualText,
		MCID: spec.MCID, HasMCID: spec.HasMCID, RawSegments: spec.RawSegments,
		Segments: spec.Segments, MarkedProperties: spec.MarkedProperties, MarkedStack: spec.MarkedStack,
	})
}

func clonePathSegments(values []geometry.PathSegment) []geometry.PathSegment {
	if values == nil {
		return nil
	}
	clone := make([]geometry.PathSegment, len(values))
	for i, value := range values {
		clone[i] = geometry.ClonePathSegment(value)
	}
	return clone
}

func cloneMarkedStack(values []MarkedContentContext) []MarkedContentContext {
	if values == nil {
		return nil
	}
	clone := make([]MarkedContentContext, len(values))
	for i, value := range values {
		clone[i] = value.Finalize()
	}
	return clone
}
