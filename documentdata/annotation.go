package documentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// Annotation is the document-independent value portion of one PDF
// annotation. Document-bound action, page, and ParentTree resolution remain
// in document.Annotation.
type Annotation struct {
	subtype      string
	page         primitives.Ref
	hasPage      bool
	inReplyTo    primitives.Ref
	hasInReplyTo bool
	popup        primitives.Ref
	hasPopup     bool
	rect         [4]float64
	contents     string
	dict         primitives.Dict
	uri          string
	dest         primitives.Object
	action       primitives.Dict
	actionKind   string
	appearance   primitives.Dict
	quadPoints   [][2]float64
	border       [3]float64
	hasBorder    bool
	color        []float64
	name         string
	modified     string
	parentKey    int
	hasParentKey bool
	flags        int
	hasFlags     bool
}

// AnnotationSpec supplies the dependency-free fields used to construct an
// owned Annotation value. The constructor copies all maps, objects, and
// slices in the specification.
type AnnotationSpec struct {
	Subtype      string
	Page         primitives.Ref
	HasPage      bool
	InReplyTo    primitives.Ref
	HasInReplyTo bool
	Popup        primitives.Ref
	HasPopup     bool
	Rect         [4]float64
	Contents     string
	Dict         primitives.Dict
	URI          string
	Dest         primitives.Object
	Action       primitives.Dict
	ActionKind   string
	Appearance   primitives.Dict
	QuadPoints   [][2]float64
	Border       [3]float64
	HasBorder    bool
	Color        []float64
	Name         string
	Modified     string
	ParentKey    int
	HasParentKey bool
	Flags        int
	HasFlags     bool
}

// NewAnnotation constructs an owned dependency-free annotation value.
func NewAnnotation(spec AnnotationSpec) Annotation {
	return Annotation{
		subtype: spec.Subtype, page: spec.Page, hasPage: spec.HasPage,
		inReplyTo: spec.InReplyTo, hasInReplyTo: spec.HasInReplyTo,
		popup: spec.Popup, hasPopup: spec.HasPopup, rect: spec.Rect,
		contents: spec.Contents, dict: cloneAnnotationDict(spec.Dict),
		uri: spec.URI, dest: cloneObject(spec.Dest),
		action: cloneAnnotationDict(spec.Action), actionKind: spec.ActionKind,
		appearance: cloneAnnotationDict(spec.Appearance),
		quadPoints: cloneAnnotationPoints(spec.QuadPoints), border: spec.Border,
		hasBorder: spec.HasBorder, color: cloneAnnotationFloats(spec.Color),
		name: spec.Name, modified: spec.Modified, parentKey: spec.ParentKey,
		hasParentKey: spec.HasParentKey, flags: spec.Flags, hasFlags: spec.HasFlags,
	}
}

func (a Annotation) Subtype() string { return a.subtype }

// Page returns the owning page reference when one is present.
func (a Annotation) Page() (primitives.Ref, bool) { return a.page, a.hasPage }

func (a Annotation) HasPage() bool                     { return a.hasPage }
func (a Annotation) InReplyTo() (primitives.Ref, bool) { return a.inReplyTo, a.hasInReplyTo }
func (a Annotation) HasInReplyTo() bool                { return a.hasInReplyTo }
func (a Annotation) Popup() (primitives.Ref, bool)     { return a.popup, a.hasPopup }
func (a Annotation) HasPopup() bool                    { return a.hasPopup }
func (a Annotation) Rect() [4]float64                  { return a.rect }
func (a Annotation) Contents() string                  { return a.contents }
func (a Annotation) URI() string                       { return a.uri }
func (a Annotation) ActionKind() string                { return a.actionKind }
func (a Annotation) Border() [3]float64                { return a.border }
func (a Annotation) HasBorder() bool                   { return a.hasBorder }
func (a Annotation) Name() string                      { return a.name }
func (a Annotation) Modified() string                  { return a.modified }
func (a Annotation) ParentKey() (int, bool)            { return a.parentKey, a.hasParentKey }
func (a Annotation) HasParentKey() bool                { return a.hasParentKey }
func (a Annotation) Flags() (int, bool)                { return a.flags, a.hasFlags }
func (a Annotation) HasFlags() bool                    { return a.hasFlags }

// WithPage returns a copy with page ownership replaced.
func (a Annotation) WithPage(page primitives.Ref, hasPage bool) Annotation {
	a.page, a.hasPage = page, hasPage
	return a
}

func (a Annotation) QuadPointsCopy() [][2]float64    { return cloneAnnotationPoints(a.quadPoints) }
func (a Annotation) ColorCopy() []float64            { return cloneAnnotationFloats(a.color) }
func (a Annotation) DictCopy() primitives.Dict       { return cloneAnnotationDict(a.dict) }
func (a Annotation) ActionCopy() primitives.Dict     { return cloneAnnotationDict(a.action) }
func (a Annotation) DestCopy() primitives.Object     { return cloneObject(a.dest) }
func (a Annotation) AppearanceCopy() primitives.Dict { return cloneAnnotationDict(a.appearance) }

// Finalize returns an independent annotation snapshot.
func (a Annotation) Finalize() Annotation {
	return NewAnnotation(AnnotationSpec{
		Subtype: a.subtype, Page: a.page, HasPage: a.hasPage,
		InReplyTo: a.inReplyTo, HasInReplyTo: a.hasInReplyTo,
		Popup: a.popup, HasPopup: a.hasPopup, Rect: a.rect,
		Contents: a.contents, Dict: a.dict, URI: a.uri, Dest: a.dest,
		Action: a.action, ActionKind: a.actionKind, Appearance: a.appearance,
		QuadPoints: a.quadPoints, Border: a.border, HasBorder: a.hasBorder,
		Color: a.color, Name: a.name, Modified: a.modified,
		ParentKey: a.parentKey, HasParentKey: a.hasParentKey,
		Flags: a.flags, HasFlags: a.hasFlags,
	})
}

func (a Annotation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Subtype      string
		Page         primitives.Ref
		HasPage      bool
		InReplyTo    primitives.Ref
		HasInReplyTo bool
		Popup        primitives.Ref
		HasPopup     bool
		Rect         [4]float64
		Contents     string
		Dict         primitives.Dict `json:"Dict"`
		URI          string
		Action       primitives.Dict   `json:"Action"`
		Dest         primitives.Object `json:"Dest"`
		ActionKind   string
		Appearance   primitives.Dict `json:"Appearance"`
		QuadPoints   [][2]float64    `json:"QuadPoints"`
		Border       [3]float64
		HasBorder    bool
		Color        []float64 `json:"Color"`
		Name         string
		Modified     string
		ParentKey    int
		HasParentKey bool
		Flags        int
		HasFlags     bool
	}{
		Subtype: a.subtype, Page: a.page, HasPage: a.hasPage,
		InReplyTo: a.inReplyTo, HasInReplyTo: a.hasInReplyTo,
		Popup: a.popup, HasPopup: a.hasPopup, Rect: a.rect,
		Contents: a.contents, Dict: a.dict, URI: a.uri, Action: a.action,
		Dest: a.dest, ActionKind: a.actionKind, Appearance: a.appearance,
		QuadPoints: a.quadPoints, Border: a.border, HasBorder: a.hasBorder,
		Color: a.color, Name: a.name, Modified: a.modified,
		ParentKey: a.parentKey, HasParentKey: a.hasParentKey,
		Flags: a.flags, HasFlags: a.hasFlags,
	})
}

func cloneAnnotationDict(value primitives.Dict) primitives.Dict {
	if value == nil {
		return nil
	}
	return cloneObject(value).(primitives.Dict)
}

func cloneAnnotationPoints(value [][2]float64) [][2]float64 {
	if value == nil {
		return nil
	}
	out := make([][2]float64, len(value))
	copy(out, value)
	return out
}

func cloneAnnotationFloats(value []float64) []float64 {
	if value == nil {
		return nil
	}
	out := make([]float64, len(value))
	copy(out, value)
	return out
}
