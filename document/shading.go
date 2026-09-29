package document

import (
	"encoding/json"
	"fmt"
	"iter"

	"github.com/lin-string/go-playa/contentdata"
)

// ShadingObject represents one invocation of a named page shading resource.
type ShadingObject struct {
	data      contentdata.Shading
	stream    Stream
	hasStream bool
}

// Name returns the selected shading resource name.
func (s ShadingObject) Name() string { return s.data.Name() }

// Page returns the associated page reference.
func (s ShadingObject) Page() Ref { return s.data.Page() }

// HasPage reports whether Page identifies a page.
func (s ShadingObject) HasPage() bool { return s.data.HasPage() }

// GState returns an independent graphics-state value.
func (s ShadingObject) GState() GraphicsState { return s.data.GState() }

// DictCopy returns an independent copy of the shading dictionary.
func (s ShadingObject) DictCopy() Dict { return s.data.DictCopy() }

// StreamCopy returns an independent copy of the shading stream, when present.
func (s ShadingObject) StreamCopy() (Stream, bool) {
	if !s.hasStream {
		return Stream{}, false
	}
	return s.stream.Finalize(), true
}

// Finalize returns an independent snapshot of the shading resource and state.
func (s ShadingObject) Finalize() ShadingObject {
	s.data = s.data.Finalize()
	if s.hasStream {
		s.stream = s.stream.Finalize()
	}
	return s
}

func (s ShadingObject) MarshalJSON() ([]byte, error) {
	var stream *Stream
	if s.hasStream {
		copy := s.stream.Finalize()
		stream = &copy
	}
	return json.Marshal(struct {
		Name    string
		Page    Ref
		HasPage bool
		GState  GraphicsState `json:"gstate"`
		Dict    Dict          `json:"Dict"`
		Stream  *Stream       `json:"Stream,omitempty"`
	}{Name: s.Name(), Page: s.Page(), HasPage: s.HasPage(), GState: s.GState(), Dict: s.DictCopy(), Stream: stream})
}

// Shadings lazily yields shading invocations in content-stream order.
func (p Page) Shadings(d *Document) iter.Seq2[ShadingObject, error] {
	return p.shadings(d, true)
}

func (p Page) shadings(d *Document, flatten bool) iter.Seq2[ShadingObject, error] {
	return func(yield func(ShadingObject, error) bool) {
		if d == nil {
			yield(ShadingObject{}, errNilDocument)
			return
		}
		if !p.hasCachedResources(d) {
			if raw, present := p.dict[Name("Resources")]; present {
				resourcesValue, resourcesResolved := d.resolveIndirectChain(raw)
				if !resourcesResolved {
					yield(ShadingObject{}, fmt.Errorf("playa: Shading resources could not be resolved"))
					return
				}
				if resources, ok := resourcesValue.(Dict); ok {
					if shadings, present := resources[Name("Shading")]; present {
						if _, resolved := d.resolveIndirectChain(shadings); !resolved {
							yield(ShadingObject{}, fmt.Errorf("playa: Shading resources could not be resolved"))
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
				yield(ShadingObject{}, err)
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
			if op.operatorValue() != "sh" {
				continue
			}
			shading, err := p.shadingFromOp(d, op, gstate)
			if err != nil {
				yield(ShadingObject{}, err)
				return
			}
			if !yield(shading, nil) {
				return
			}
		}
	}
}

// Shadings yields shading invocations directly contained in this Form
// XObject, resolving names against the Form's own resource dictionary.
func (x XObjectObject) Shadings(d *Document) iter.Seq2[ShadingObject, error] {
	return func(yield func(ShadingObject, error) bool) {
		if err := x.validate(d); err != nil {
			yield(ShadingObject{}, err)
			return
		}
		for object, err := range x.pageView(d).shadings(d, false) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (p Page) shadingFromOp(d *Document, op ContentOp, gstate graphicsState) (ShadingObject, error) {
	if len(op.operandsValue()) == 0 {
		return ShadingObject{}, fmt.Errorf("playa: sh operator has no resource name")
	}
	name, ok := op.operandsValue()[0].(Name)
	if !ok {
		return ShadingObject{}, fmt.Errorf("playa: sh operator resource name is not a name")
	}
	shadingsValue, shadingsResolved := d.resolveIndirectChain(op.resources[Name("Shading")])
	if !shadingsResolved && op.resources[Name("Shading")] != nil {
		return ShadingObject{}, fmt.Errorf("playa: Shading resources could not be resolved")
	}
	shadings, ok := shadingsValue.(Dict)
	if !ok {
		return ShadingObject{}, fmt.Errorf("playa: Shading resources are not a dictionary")
	}
	value, exists := shadings[name]
	if !exists {
		return ShadingObject{}, fmt.Errorf("playa: shading resource %q not found", name)
	}
	resolvedValue, valueResolved := d.resolveIndirectChain(value)
	if !valueResolved {
		return ShadingObject{}, fmt.Errorf("playa: shading resource %q could not be resolved", name)
	}
	value = resolvedValue
	spec := contentdata.ShadingSpec{Name: string(name), Page: p.ref, HasPage: p.ref != (Ref{}), GState: gstate.publicValue()}
	switch value := value.(type) {
	case Dict:
		spec.Dict = value
	case Stream:
		spec.Dict = value.DictCopy()
		return ShadingObject{data: contentdata.NewShading(spec), stream: value, hasStream: true}, nil
	default:
		return ShadingObject{}, fmt.Errorf("playa: shading resource %q is not a dictionary or stream", name)
	}
	return ShadingObject{data: contentdata.NewShading(spec)}, nil
}
