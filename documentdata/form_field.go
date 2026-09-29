package documentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// FormField is the document-independent value portion of one AcroForm field.
// Field-tree traversal, document association, and lazy child resolution stay
// in document.FormField.
type FormField struct {
	name              string
	fullName          string
	page              primitives.Ref
	hasPage           bool
	parent            primitives.Ref
	hasParent         bool
	fieldType         string
	flags             int
	hasFlags          bool
	value             string
	values            []string
	defaultValue      string
	defaultValues     []string
	defaultAppearance string
	options           []string
	optionValues      []string
	selected          []int
	rect              [4]float64
	hasRect           bool
	isWidget          bool
	dict              primitives.Dict
}

// FormFieldSpec supplies the dependency-free fields used to construct an
// owned FormField value. The constructor copies all maps and slices.
type FormFieldSpec struct {
	Name              string
	FullName          string
	Page              primitives.Ref
	HasPage           bool
	Parent            primitives.Ref
	HasParent         bool
	FieldType         string
	Flags             int
	HasFlags          bool
	Value             string
	Values            []string
	DefaultValue      string
	DefaultValues     []string
	DefaultAppearance string
	Options           []string
	OptionValues      []string
	Selected          []int
	Rect              [4]float64
	HasRect           bool
	IsWidget          bool
	Dict              primitives.Dict
}

// NewFormField constructs an owned dependency-free form-field value.
func NewFormField(spec FormFieldSpec) FormField {
	return FormField{
		name: spec.Name, fullName: spec.FullName, page: spec.Page, hasPage: spec.HasPage,
		parent: spec.Parent, hasParent: spec.HasParent, fieldType: spec.FieldType,
		flags: spec.Flags, hasFlags: spec.HasFlags, value: spec.Value,
		values: cloneStrings(spec.Values), defaultValue: spec.DefaultValue,
		defaultValues: cloneStrings(spec.DefaultValues), defaultAppearance: spec.DefaultAppearance,
		options: cloneStrings(spec.Options), optionValues: cloneStrings(spec.OptionValues),
		selected: cloneInts(spec.Selected), rect: spec.Rect, hasRect: spec.HasRect,
		isWidget: spec.IsWidget, dict: cloneDict(spec.Dict),
	}
}

func (f FormField) Name() string                   { return f.name }
func (f FormField) FullName() string               { return f.fullName }
func (f FormField) Page() (primitives.Ref, bool)   { return f.page, f.hasPage }
func (f FormField) HasPage() bool                  { return f.hasPage }
func (f FormField) Parent() (primitives.Ref, bool) { return f.parent, f.hasParent }
func (f FormField) HasParent() bool                { return f.hasParent }
func (f FormField) FieldType() string              { return f.fieldType }
func (f FormField) Flags() int                     { return f.flags }
func (f FormField) HasFlags() bool                 { return f.hasFlags }
func (f FormField) Value() string                  { return f.value }
func (f FormField) DefaultValue() string           { return f.defaultValue }
func (f FormField) DefaultAppearance() string      { return f.defaultAppearance }
func (f FormField) Rect() [4]float64               { return f.rect }
func (f FormField) HasRect() bool                  { return f.hasRect }
func (f FormField) IsWidget() bool                 { return f.isWidget }
func (f FormField) ValuesCopy() []string           { return cloneStrings(f.values) }
func (f FormField) DefaultValuesCopy() []string    { return cloneStrings(f.defaultValues) }
func (f FormField) OptionsCopy() []string          { return cloneStrings(f.options) }
func (f FormField) OptionValuesCopy() []string     { return cloneStrings(f.optionValues) }
func (f FormField) SelectedCopy() []int            { return cloneInts(f.selected) }
func (f FormField) DictCopy() primitives.Dict      { return cloneDict(f.dict) }

// Finalize returns an independent form-field snapshot.
func (f FormField) Finalize() FormField {
	return NewFormField(FormFieldSpec{
		Name: f.name, FullName: f.fullName, Page: f.page, HasPage: f.hasPage,
		Parent: f.parent, HasParent: f.hasParent, FieldType: f.fieldType,
		Flags: f.flags, HasFlags: f.hasFlags, Value: f.value, Values: f.values,
		DefaultValue: f.defaultValue, DefaultValues: f.defaultValues,
		DefaultAppearance: f.defaultAppearance, Options: f.options,
		OptionValues: f.optionValues, Selected: f.selected, Rect: f.rect,
		HasRect: f.hasRect, IsWidget: f.isWidget, Dict: f.dict,
	})
}

func (f FormField) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name              string
		FullName          string
		Page              primitives.Ref
		HasPage           bool
		Parent            primitives.Ref
		HasParent         bool
		FieldType         string
		Flags             int
		HasFlags          bool
		Value             string
		DefaultValue      string
		DefaultAppearance string
		Rect              [4]float64
		HasRect           bool
		IsWidget          bool
		Dict              primitives.Dict
		Values            []string
		DefaultValues     []string
		Options           []string
		OptionValues      []string
		Selected          []int
	}{
		Name: f.name, FullName: f.fullName, Page: f.page, HasPage: f.hasPage,
		Parent: f.parent, HasParent: f.hasParent, FieldType: f.fieldType,
		Flags: f.flags, HasFlags: f.hasFlags, Value: f.value,
		DefaultValue: f.defaultValue, DefaultAppearance: f.defaultAppearance,
		Rect: f.rect, HasRect: f.hasRect, IsWidget: f.isWidget, Dict: f.dict,
		Values: f.values, DefaultValues: f.defaultValues, Options: f.options,
		OptionValues: f.optionValues, Selected: f.selected,
	})
}

func cloneStrings(value []string) []string {
	if value == nil {
		return nil
	}
	out := make([]string, len(value))
	copy(out, value)
	return out
}

func cloneInts(value []int) []int {
	if value == nil {
		return nil
	}
	out := make([]int, len(value))
	copy(out, value)
	return out
}

func cloneDict(value primitives.Dict) primitives.Dict {
	if value == nil {
		return nil
	}
	return cloneObject(value).(primitives.Dict)
}
