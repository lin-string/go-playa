package structuredata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// Element is the document-independent value portion of one tagged-PDF
// structure element. Child/content traversal, ParentTree lookup, and page
// resolution remain owned by document.StructElement.
type Element struct {
	typeName      string
	structureType string
	role          string
	rawRole       string
	title         string
	language      string
	alt           string
	actualText    string
	abbreviation  string
	className     string
	attributes    primitives.Dict
	page          primitives.Ref
	hasPage       bool
	parent        primitives.Ref
	hasParent     bool
	mcid          int
	hasMCID       bool
	isMCR         bool
	objectRef     primitives.Ref
	hasObject     bool
	bbox          [4]float64
	hasBBox       bool
	dict          primitives.Dict
	object        primitives.Object
}

// ElementSpec supplies dependency-free fields used to construct an owned
// structure element value. Recursive PDF values are copied by the
// constructor.
type ElementSpec struct {
	Type          string
	StructureType string
	Role          string
	RawRole       string
	Title         string
	Language      string
	Alt           string
	ActualText    string
	Abbreviation  string
	ClassName     string
	Attributes    primitives.Dict
	Page          primitives.Ref
	HasPage       bool
	Parent        primitives.Ref
	HasParent     bool
	MCID          int
	HasMCID       bool
	IsMCR         bool
	ObjectRef     primitives.Ref
	HasObject     bool
	BBox          [4]float64
	HasBBox       bool
	Dict          primitives.Dict
	Object        primitives.Object
}

// NewElement constructs an owned dependency-free structure element value.
func NewElement(spec ElementSpec) Element {
	return Element{
		typeName: spec.Type, structureType: spec.StructureType, role: spec.Role,
		rawRole: spec.RawRole, title: spec.Title, language: spec.Language,
		alt: spec.Alt, actualText: spec.ActualText, abbreviation: spec.Abbreviation,
		className: spec.ClassName, attributes: cloneDict(spec.Attributes),
		page: spec.Page, hasPage: spec.HasPage, parent: spec.Parent,
		hasParent: spec.HasParent, mcid: spec.MCID, hasMCID: spec.HasMCID,
		isMCR: spec.IsMCR, objectRef: spec.ObjectRef, hasObject: spec.HasObject,
		bbox: spec.BBox, hasBBox: spec.HasBBox, dict: cloneDict(spec.Dict),
		object: cloneObject(spec.Object),
	}
}

func (e Element) Type() string                    { return e.typeName }
func (e Element) StructureType() string           { return e.structureType }
func (e Element) Role() string                    { return e.role }
func (e Element) RawRole() string                 { return e.rawRole }
func (e Element) Title() string                   { return e.title }
func (e Element) Language() string                { return e.language }
func (e Element) AlternateDescription() string    { return e.alt }
func (e Element) ActualText() string              { return e.actualText }
func (e Element) AbbreviationExpansion() string   { return e.abbreviation }
func (e Element) ClassName() string               { return e.className }
func (e Element) Page() primitives.Ref            { return e.page }
func (e Element) HasPage() bool                   { return e.hasPage }
func (e Element) Parent() primitives.Ref          { return e.parent }
func (e Element) HasParent() bool                 { return e.hasParent }
func (e Element) MCID() int                       { return e.mcid }
func (e Element) HasMCID() bool                   { return e.hasMCID }
func (e Element) IsMCR() bool                     { return e.isMCR }
func (e Element) ObjectRef() primitives.Ref       { return e.objectRef }
func (e Element) HasObject() bool                 { return e.hasObject }
func (e Element) BBox() [4]float64                { return e.bbox }
func (e Element) HasBBox() bool                   { return e.hasBBox }
func (e Element) AttributesCopy() primitives.Dict { return cloneDict(e.attributes) }
func (e Element) DictCopy() primitives.Dict       { return cloneDict(e.dict) }
func (e Element) ObjectCopy() primitives.Object   { return cloneObject(e.object) }

// Finalize returns an independent structure element value snapshot.
func (e Element) Finalize() Element {
	return NewElement(ElementSpec{
		Type: e.typeName, StructureType: e.structureType, Role: e.role,
		RawRole: e.rawRole, Title: e.title, Language: e.language, Alt: e.alt,
		ActualText: e.actualText, Abbreviation: e.abbreviation, ClassName: e.className,
		Attributes: e.attributes, Page: e.page, HasPage: e.hasPage,
		Parent: e.parent, HasParent: e.hasParent, MCID: e.mcid, HasMCID: e.hasMCID,
		IsMCR: e.isMCR, ObjectRef: e.objectRef, HasObject: e.hasObject,
		BBox: e.bbox, HasBBox: e.hasBBox, Dict: e.dict, Object: e.object,
	})
}

func (e Element) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type          string
		StructureType string
		Role          string
		RawRole       string
		Title         string
		Language      string
		Alt           string
		ActualText    string
		Abbreviation  string
		ClassName     string
		Page          primitives.Ref
		HasPage       bool
		Parent        primitives.Ref
		HasParent     bool
		MCID          int
		HasMCID       bool
		IsMCR         bool
		ObjectRef     primitives.Ref
		HasObject     bool
		BBox          [4]float64
		HasBBox       bool
		Attributes    primitives.Dict
		Dict          primitives.Dict
		Object        primitives.Object
	}{
		Type: e.typeName, StructureType: e.structureType, Role: e.role, RawRole: e.rawRole,
		Title: e.title, Language: e.language, Alt: e.alt, ActualText: e.actualText,
		Abbreviation: e.abbreviation, ClassName: e.className, Page: e.page,
		HasPage: e.hasPage, Parent: e.parent, HasParent: e.hasParent, MCID: e.mcid,
		HasMCID: e.hasMCID, IsMCR: e.isMCR, ObjectRef: e.objectRef, HasObject: e.hasObject,
		BBox: e.bbox, HasBBox: e.hasBBox, Attributes: e.attributes, Dict: e.dict, Object: e.object,
	})
}
