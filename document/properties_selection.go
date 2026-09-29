package document

import (
	"fmt"
	"iter"

	"github.com/lin-string/go-playa/contentdata"
)

// PropertiesObject is the dependency-free value for a named BDC or DP
// Properties resource. Resource lookup and lazy page traversal remain here.
type PropertiesObject = contentdata.PropertiesObject

func newPropertiesObject(name, operator string, page Ref, hasPage bool, dict Dict) PropertiesObject {
	return contentdata.NewPropertiesObject(name, operator, page, hasPage, dict)
}

// Properties lazily yields named Properties resources selected by BDC and DP.
func (p Page) Properties(d *Document) iter.Seq2[PropertiesObject, error] {
	return p.properties(d, true)
}

// Properties yields named Properties resources selected directly by this Form.
func (x XObjectObject) Properties(d *Document) iter.Seq2[PropertiesObject, error] {
	return x.pageView(d).properties(d, false)
}

func (p Page) properties(d *Document, flatten bool) iter.Seq2[PropertiesObject, error] {
	return func(yield func(PropertiesObject, error) bool) {
		if d == nil {
			yield(PropertiesObject{}, errNilDocument)
			return
		}
		iterator := newContentOpIterator(d, p, flatten)
		for {
			op, err, ok := iterator.next()
			if err != nil {
				yield(PropertiesObject{}, err)
				return
			}
			if !ok {
				return
			}
			if op.operatorValue() != "BDC" && op.operatorValue() != "DP" {
				continue
			}
			if len(op.operandsValue()) < 2 {
				yield(PropertiesObject{}, fmt.Errorf("playa: %s operator has no Properties name", op.operatorValue()))
				return
			}
			name, ok := op.propertyName, op.hasPropertyName
			if !ok {
				continue
			}
			resourcesValue, resourcesResolved := d.resolveIndirectChain(op.resources[Name("Properties")])
			if !resourcesResolved {
				yield(PropertiesObject{}, fmt.Errorf("playa: Properties resources could not be resolved"))
				return
			}
			resources, ok := resourcesValue.(Dict)
			if !ok {
				yield(PropertiesObject{}, fmt.Errorf("playa: Properties resources are not a dictionary"))
				return
			}
			value, exists := resources[name]
			if !exists {
				yield(PropertiesObject{}, fmt.Errorf("playa: Properties resource %q not found", name))
				return
			}
			resolved, valueResolved := d.resolveIndirectChain(value)
			if !valueResolved {
				yield(PropertiesObject{}, fmt.Errorf("playa: property resource %q could not be resolved", name))
				return
			}
			dict, ok := markedPropertiesDict(resolved)
			if !ok {
				yield(PropertiesObject{}, fmt.Errorf("playa: Properties resource %q is not a dictionary or stream", name))
				return
			}
			object := newPropertiesObject(string(name), op.operatorValue(), p.ref, p.ref != (Ref{}), dict)
			if !yield(object, nil) {
				return
			}
		}
	}
}
