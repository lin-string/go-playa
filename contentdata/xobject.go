package contentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// XObject is the dependency-free value portion of a Form XObject invocation.
// Document lookup, nested content traversal, and resource inheritance remain
// owned by document.XObjectObject.
type XObject struct {
	name                 string
	ref                  primitives.Ref
	page                 primitives.Ref
	hasPage              bool
	stream               pdftypes.Stream
	matrix               geometry.Matrix
	bbox                 [4]float64
	group                primitives.Dict
	parentKey            int
	hasParentKey         bool
	resources            primitives.Dict
	declaredResources    primitives.Dict
	hasDeclaredResources bool
	resourceContext      primitives.Dict
	path                 string
	markedTag            string
	markedProperties     primitives.Dict
	markedStack          []MarkedContentContext
	mcid                 int
	hasMCID              bool
	gstate               GraphicsState
}

// XObjectSpec supplies the dependency-free fields used to construct an XObject.
type XObjectSpec struct {
	Name                 string
	Ref                  primitives.Ref
	Page                 primitives.Ref
	HasPage              bool
	Stream               pdftypes.Stream
	Matrix               geometry.Matrix
	BBox                 [4]float64
	Group                primitives.Dict
	ParentKey            int
	HasParentKey         bool
	Resources            primitives.Dict
	DeclaredResources    primitives.Dict
	HasDeclaredResources bool
	ResourceContext      primitives.Dict
	Path                 string
	MarkedTag            string
	MarkedProperties     primitives.Dict
	MarkedStack          []MarkedContentContext
	MCID                 int
	HasMCID              bool
	GState               GraphicsState
}

// NewXObject constructs an owned value snapshot.
func NewXObject(spec XObjectSpec) XObject {
	return XObject{
		name: spec.Name, ref: spec.Ref, page: spec.Page, hasPage: spec.HasPage,
		stream: spec.Stream.Finalize(), matrix: spec.Matrix, bbox: spec.BBox,
		group: cloneDict(spec.Group), parentKey: spec.ParentKey, hasParentKey: spec.HasParentKey,
		resources: cloneDict(spec.Resources), declaredResources: cloneDict(spec.DeclaredResources),
		hasDeclaredResources: spec.HasDeclaredResources, resourceContext: cloneDict(spec.ResourceContext),
		path: spec.Path, markedTag: spec.MarkedTag, markedProperties: cloneDict(spec.MarkedProperties),
		markedStack: cloneXObjectMarkedStack(spec.MarkedStack), mcid: spec.MCID, hasMCID: spec.HasMCID,
		gstate: spec.GState.Finalize(),
	}
}

// NewXObjectBorrowed constructs a low-allocation value over caller-owned
// streams, dictionaries, and marked-content values. Use Finalize or the copy
// accessors before retaining the value beyond the source traversal.
func NewXObjectBorrowed(spec XObjectSpec) XObject {
	return XObject{
		name: spec.Name, ref: spec.Ref, page: spec.Page, hasPage: spec.HasPage,
		stream: spec.Stream, matrix: spec.Matrix, bbox: spec.BBox,
		group: spec.Group, parentKey: spec.ParentKey, hasParentKey: spec.HasParentKey,
		resources: spec.Resources, declaredResources: spec.DeclaredResources,
		hasDeclaredResources: spec.HasDeclaredResources, resourceContext: spec.ResourceContext,
		path: spec.Path, markedTag: spec.MarkedTag, markedProperties: spec.MarkedProperties,
		markedStack: spec.MarkedStack, mcid: spec.MCID, hasMCID: spec.HasMCID,
		gstate: spec.GState,
	}
}

func (x XObject) Name() string                                { return x.name }
func (x XObject) Ref() primitives.Ref                         { return x.ref }
func (x XObject) Page() primitives.Ref                        { return x.page }
func (x XObject) HasPage() bool                               { return x.hasPage }
func (x XObject) StreamBorrowed() pdftypes.Stream             { return x.stream }
func (x XObject) StreamCopy() pdftypes.Stream                 { return x.stream.Finalize() }
func (x XObject) Matrix() geometry.Matrix                     { return x.matrix }
func (x XObject) BBox() [4]float64                            { return x.bbox }
func (x XObject) GroupBorrowed() primitives.Dict              { return x.group }
func (x XObject) GroupCopy() primitives.Dict                  { return cloneDict(x.group) }
func (x XObject) ParentKey() int                              { return x.parentKey }
func (x XObject) HasParentKey() bool                          { return x.hasParentKey }
func (x XObject) ResourcesBorrowed() primitives.Dict          { return x.resources }
func (x XObject) ResourcesCopy() primitives.Dict              { return cloneDict(x.resources) }
func (x XObject) DeclaredResourcesBorrowed() primitives.Dict  { return x.declaredResources }
func (x XObject) DeclaredResourcesCopy() primitives.Dict      { return cloneDict(x.declaredResources) }
func (x XObject) HasDeclaredResources() bool                  { return x.hasDeclaredResources }
func (x XObject) ResourceContextBorrowed() primitives.Dict    { return x.resourceContext }
func (x XObject) ResourceContextCopy() primitives.Dict        { return cloneDict(x.resourceContext) }
func (x XObject) Path() string                                { return x.path }
func (x XObject) MarkedTag() string                           { return x.markedTag }
func (x XObject) MarkedPropertiesBorrowed() primitives.Dict   { return x.markedProperties }
func (x XObject) MarkedPropertiesCopy() primitives.Dict       { return cloneDict(x.markedProperties) }
func (x XObject) MarkedStackBorrowed() []MarkedContentContext { return x.markedStack }
func (x XObject) MarkedStackCopy() []MarkedContentContext {
	return cloneXObjectMarkedStack(x.markedStack)
}
func (x XObject) MCID() int                     { return x.mcid }
func (x XObject) HasMCID() bool                 { return x.hasMCID }
func (x XObject) GState() GraphicsState         { return x.gstate.Finalize() }
func (x XObject) GStateBorrowed() GraphicsState { return x.gstate }

// Finalize returns an independent snapshot of the XObject value.
func (x XObject) Finalize() XObject {
	return NewXObject(XObjectSpec{
		Name: x.name, Ref: x.ref, Page: x.page, HasPage: x.hasPage, Stream: x.stream,
		Matrix: x.matrix, BBox: x.bbox, Group: x.group, ParentKey: x.parentKey,
		HasParentKey: x.hasParentKey, Resources: x.resources, DeclaredResources: x.declaredResources,
		HasDeclaredResources: x.hasDeclaredResources, ResourceContext: x.resourceContext,
		Path: x.path, MarkedTag: x.markedTag, MarkedProperties: x.markedProperties,
		MarkedStack: x.markedStack, MCID: x.mcid, HasMCID: x.hasMCID, GState: x.gstate,
	})
}

func (x XObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name             string
		Ref              primitives.Ref
		Page             primitives.Ref
		HasPage          bool
		Stream           pdftypes.Stream
		Matrix           geometry.Matrix
		BBox             [4]float64
		Group            primitives.Dict
		ParentKey        int
		HasParentKey     bool
		Resources        primitives.Dict
		Path             string
		MarkedTag        string
		MarkedProperties primitives.Dict
		MarkedStack      []MarkedContentContext
		MCID             int
		HasMCID          bool
		GState           GraphicsState `json:"gstate"`
	}{
		Name: x.name, Ref: x.ref, Page: x.page, HasPage: x.hasPage, Stream: x.stream,
		Matrix: x.matrix, BBox: x.bbox, Group: x.group, ParentKey: x.parentKey,
		HasParentKey: x.hasParentKey, Resources: x.resources, Path: x.path,
		MarkedTag: x.markedTag, MarkedProperties: x.markedProperties,
		MarkedStack: x.markedStack, MCID: x.mcid, HasMCID: x.hasMCID, GState: x.gstate,
	})
}

func cloneXObjectMarkedStack(values []MarkedContentContext) []MarkedContentContext {
	if values == nil {
		return nil
	}
	result := make([]MarkedContentContext, len(values))
	for i, value := range values {
		result[i] = value.Finalize()
	}
	return result
}
