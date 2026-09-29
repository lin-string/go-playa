package document

import (
	"fmt"
	"iter"

	"github.com/lin-string/go-playa/contentdata"
)

// ExtGStateObject represents one resource-backed graphics-state selection.
type ExtGStateObject struct {
	data contentdata.ExtGState
}

// Name returns the selected ExtGState resource name.
func (e ExtGStateObject) Name() string { return e.data.Name() }

// Page returns the associated page reference.
func (e ExtGStateObject) Page() Ref { return e.data.Page() }

// HasPage reports whether Page identifies a page.
func (e ExtGStateObject) HasPage() bool { return e.data.HasPage() }

// GState returns an independent graphics-state value.
func (e ExtGStateObject) GState() GraphicsState { return e.data.GState() }

// DictCopy returns an independent copy of the ExtGState resource dictionary.
func (e ExtGStateObject) DictCopy() Dict { return e.data.DictCopy() }

// Finalize returns an independent snapshot of the resource and applied state.
func (e ExtGStateObject) Finalize() ExtGStateObject { e.data = e.data.Finalize(); return e }

func (e ExtGStateObject) MarshalJSON() ([]byte, error) { return e.data.MarshalJSON() }

// ExtGStates lazily yields graphics-state resources selected by page content.
func (p Page) ExtGStates(d *Document) iter.Seq2[ExtGStateObject, error] {
	return p.extGStates(d, true)
}

// ExtGStates yields graphics-state resources directly selected by this Form.
func (x XObjectObject) ExtGStates(d *Document) iter.Seq2[ExtGStateObject, error] {
	return func(yield func(ExtGStateObject, error) bool) {
		if err := x.validate(d); err != nil {
			yield(ExtGStateObject{}, err)
			return
		}
		for object, err := range x.pageView(d).extGStates(d, false) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (p Page) extGStates(d *Document, flatten bool) iter.Seq2[ExtGStateObject, error] {
	return func(yield func(ExtGStateObject, error) bool) {
		if d == nil {
			yield(ExtGStateObject{}, errNilDocument)
			return
		}
		if !p.hasCachedResources(d) {
			if raw, present := p.dict[Name("Resources")]; present {
				resourcesValue, resourcesResolved := d.resolveIndirectChain(raw)
				if !resourcesResolved {
					yield(ExtGStateObject{}, fmt.Errorf("playa: ExtGState resources could not be resolved"))
					return
				}
				if resources, ok := resourcesValue.(Dict); ok {
					if extGState, present := resources[Name("ExtGState")]; present {
						if _, resolved := d.resolveIndirectChain(extGState); !resolved {
							yield(ExtGStateObject{}, fmt.Errorf("playa: ExtGState resources could not be resolved"))
							return
						}
					}
				}
			}
		}
		iterator := newContentOpIterator(d, p, flatten)
		gstate := newGraphicsState()
		saved := []graphicsState{}
		for {
			op, err, ok := iterator.next()
			if err != nil {
				yield(ExtGStateObject{}, err)
				return
			}
			if !ok {
				return
			}
			switch op.operatorValue() {
			case "q":
				saved = append(saved, gstate.Clone())
			case "Q":
				if len(saved) > 0 {
					gstate = saved[len(saved)-1]
					saved = saved[:len(saved)-1]
				}
			default:
				applyGraphicsState(&gstate, op)
				applyExternalGraphicsState(d, &gstate, op)
				applyResourceColorSpace(d, &gstate, op)
			}
			if op.operatorValue() != "gs" {
				continue
			}
			state, err := p.extGStateFromOp(d, op, gstate)
			if err != nil {
				yield(ExtGStateObject{}, err)
				return
			}
			if !yield(state, nil) {
				return
			}
		}
	}
}

func (p Page) extGStateFromOp(d *Document, op ContentOp, gstate graphicsState) (ExtGStateObject, error) {
	if len(op.operandsValue()) == 0 {
		return ExtGStateObject{}, fmt.Errorf("playa: gs operator has no resource name")
	}
	name, ok := op.operandsValue()[0].(Name)
	if !ok {
		return ExtGStateObject{}, fmt.Errorf("playa: gs operator resource name is not a name")
	}
	resourcesValue, resourcesResolved := d.resolveIndirectChain(op.resources[Name("ExtGState")])
	if !resourcesResolved && op.resources[Name("ExtGState")] != nil {
		return ExtGStateObject{}, fmt.Errorf("playa: ExtGState resources could not be resolved")
	}
	resources, ok := resourcesValue.(Dict)
	if !ok {
		return ExtGStateObject{}, fmt.Errorf("playa: ExtGState resources are not a dictionary")
	}
	value, exists := resources[name]
	if !exists {
		return ExtGStateObject{}, fmt.Errorf("playa: ExtGState resource %q not found", name)
	}
	resolvedValue, valueResolved := d.resolveIndirectChain(value)
	if !valueResolved {
		return ExtGStateObject{}, fmt.Errorf("playa: ExtGState resource %q could not be resolved", name)
	}
	value = resolvedValue
	dict, ok := value.(Dict)
	if !ok {
		return ExtGStateObject{}, fmt.Errorf("playa: ExtGState resource %q is not a dictionary", name)
	}
	return ExtGStateObject{data: contentdata.NewExtGState(contentdata.ExtGStateSpec{
		Name: string(name), Page: p.ref, HasPage: p.ref != (Ref{}),
		GState: gstate.publicValue(), Dict: dict,
	})}, nil
}
