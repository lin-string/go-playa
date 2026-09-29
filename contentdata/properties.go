// Package contentdata owns dependency-free content value models.
package contentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// PropertiesObject is a named Properties resource selected by a BDC or DP
// operation. Resource lookup and lazy page traversal remain document-owned.
type PropertiesObject struct {
	name     string
	operator string
	page     primitives.Ref
	hasPage  bool
	dict     primitives.Dict
}

// NewPropertiesObject constructs an owned Properties value.
func NewPropertiesObject(name, operator string, page primitives.Ref, hasPage bool, dict primitives.Dict) PropertiesObject {
	return PropertiesObject{name: name, operator: operator, page: page, hasPage: hasPage, dict: cloneDict(dict)}
}

// Name returns the selected Properties resource name.
func (p PropertiesObject) Name() string { return p.name }

// Operator returns the selecting content operator.
func (p PropertiesObject) Operator() string { return p.operator }

// Page returns the associated page reference.
func (p PropertiesObject) Page() primitives.Ref { return p.page }

// HasPage reports whether Page identifies an owning page.
func (p PropertiesObject) HasPage() bool { return p.hasPage }

// DictCopy returns an independent copy of the resolved Properties dictionary.
func (p PropertiesObject) DictCopy() primitives.Dict { return cloneDict(p.dict) }

// Finalize returns an independent snapshot of the value.
func (p PropertiesObject) Finalize() PropertiesObject {
	p.dict = cloneDict(p.dict)
	return p
}

// MarshalJSON emits the stable public projection used by document content
// compatibility output.
func (p PropertiesObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name     string
		Operator string
		Page     primitives.Ref
		HasPage  bool
		Dict     primitives.Dict
	}{p.name, p.operator, p.page, p.hasPage, p.dict})
}

func cloneDict(value primitives.Dict) primitives.Dict {
	if value == nil {
		return nil
	}
	out := make(primitives.Dict, len(value))
	for key, item := range value {
		out[key] = cloneObject(item)
	}
	return out
}

func cloneObject(value primitives.Object) primitives.Object {
	if cloneable, ok := value.(interface{ ClonePDFObject() primitives.Object }); ok {
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
		return cloneDict(value)
	case primitives.String:
		return primitives.String(primitives.CloneBytes([]byte(value)))
	default:
		return value
	}
}
