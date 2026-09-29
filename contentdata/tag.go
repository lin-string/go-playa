package contentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// Tag is the dependency-free value portion of one MP or DP marked-content
// point. ParentTree resolution remains owned by document.TagObject.
type Tag struct {
	name             string
	page             primitives.Ref
	hasPage          bool
	properties       primitives.Dict
	actualText       string
	mcid             int
	hasMCID          bool
	markedTag        string
	markedStack      []MarkedContentContext
	markedProperties primitives.Dict
	gstate           GraphicsState
}

// TagSpec supplies the dependency-free fields used to construct a Tag.
type TagSpec struct {
	Name             string
	Page             primitives.Ref
	HasPage          bool
	Properties       primitives.Dict
	ActualText       string
	MCID             int
	HasMCID          bool
	MarkedTag        string
	MarkedStack      []MarkedContentContext
	MarkedProperties primitives.Dict
	GState           GraphicsState
}

// NewTag constructs an owned marked-content point value.
func NewTag(spec TagSpec) Tag {
	return Tag{
		name: spec.Name, page: spec.Page, hasPage: spec.HasPage,
		properties: cloneDict(spec.Properties), actualText: spec.ActualText,
		mcid: spec.MCID, hasMCID: spec.HasMCID, markedTag: spec.MarkedTag,
		markedStack:      cloneMarkedStack(spec.MarkedStack),
		markedProperties: cloneDict(spec.MarkedProperties), gstate: spec.GState.Finalize(),
	}
}

// NewTagBorrowed constructs a low-allocation value over data owned by the
// caller. Use a Copy accessor or Finalize before retaining it.
func NewTagBorrowed(spec TagSpec) Tag {
	return Tag{
		name: spec.Name, page: spec.Page, hasPage: spec.HasPage,
		properties: spec.Properties, actualText: spec.ActualText,
		mcid: spec.MCID, hasMCID: spec.HasMCID, markedTag: spec.MarkedTag,
		markedStack: spec.MarkedStack, markedProperties: spec.MarkedProperties,
		gstate: spec.GState,
	}
}

func (t Tag) Name() string                            { return t.name }
func (t Tag) Page() primitives.Ref                    { return t.page }
func (t Tag) HasPage() bool                           { return t.hasPage }
func (t Tag) PropertiesCopy() primitives.Dict         { return cloneDict(t.properties) }
func (t Tag) ActualText() string                      { return t.actualText }
func (t Tag) MCID() int                               { return t.mcid }
func (t Tag) HasMCID() bool                           { return t.hasMCID }
func (t Tag) MarkedTag() string                       { return t.markedTag }
func (t Tag) MarkedPropertiesCopy() primitives.Dict   { return cloneDict(t.markedProperties) }
func (t Tag) MarkedStackCopy() []MarkedContentContext { return cloneMarkedStack(t.markedStack) }
func (t Tag) GState() GraphicsState                   { return t.gstate.Finalize() }

// MarkedStackBorrowed returns the parser-owned enclosing context slice.
func (t Tag) MarkedStackBorrowed() []MarkedContentContext {
	return t.markedStack
}

// Finalize returns an independent marked-content point snapshot.
func (t Tag) Finalize() Tag {
	return NewTag(TagSpec{
		Name: t.name, Page: t.page, HasPage: t.hasPage, Properties: t.properties,
		ActualText: t.actualText, MCID: t.mcid, HasMCID: t.hasMCID,
		MarkedTag: t.markedTag, MarkedStack: t.markedStack,
		MarkedProperties: t.markedProperties, GState: t.gstate,
	})
}

func (t Tag) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name             string
		Page             primitives.Ref
		HasPage          bool
		Properties       primitives.Dict `json:"Properties"`
		ActualText       string
		MCID             int
		HasMCID          bool
		MarkedTag        string
		MarkedStack      []MarkedContentContext `json:"MarkedStack"`
		MarkedProperties primitives.Dict        `json:"MarkedProperties"`
		GState           GraphicsState          `json:"gstate"`
	}{
		Name: t.name, Page: t.page, HasPage: t.hasPage, Properties: t.properties,
		ActualText: t.actualText, MCID: t.mcid, HasMCID: t.hasMCID,
		MarkedTag: t.markedTag, MarkedStack: t.markedStack,
		MarkedProperties: t.markedProperties, GState: t.gstate,
	})
}
