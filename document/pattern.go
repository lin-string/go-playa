package document

import (
	"encoding/json"
	"fmt"
	"iter"

	"github.com/lin-string/go-playa/contentdata"
)

// PatternObject represents one named tiling or shading pattern selected by a
// content color operator.
type PatternObject struct {
	data      contentdata.Pattern
	stream    Stream
	hasStream bool
}

// Name returns the selected pattern resource name.
func (p PatternObject) Name() string { return p.data.Name() }

// Page returns the associated page reference.
func (p PatternObject) Page() Ref { return p.data.Page() }

// HasPage reports whether Page identifies a page.
func (p PatternObject) HasPage() bool { return p.data.HasPage() }

// Stroke reports whether the pattern was selected for stroking.
func (p PatternObject) Stroke() bool { return p.data.Stroke() }

// GState returns an independent graphics-state value.
func (p PatternObject) GState() GraphicsState { return p.data.GState() }

// DictCopy returns an independent copy of the pattern dictionary.
func (p PatternObject) DictCopy() Dict { return p.data.DictCopy() }

// StreamCopy returns an independent copy of the pattern stream, when present.
func (p PatternObject) StreamCopy() (Stream, bool) {
	if !p.hasStream {
		return Stream{}, false
	}
	return p.stream.Finalize(), true
}

// Finalize returns an independent snapshot of the pattern resource and state.
func (p PatternObject) Finalize() PatternObject {
	p.data = p.data.Finalize()
	if p.hasStream {
		p.stream = p.stream.Finalize()
	}
	return p
}

func (p PatternObject) MarshalJSON() ([]byte, error) {
	var stream *Stream
	if p.hasStream {
		copy := p.stream.Finalize()
		stream = &copy
	}
	return json.Marshal(struct {
		Name    string
		Page    Ref
		HasPage bool
		Stroke  bool
		GState  GraphicsState `json:"gstate"`
		Dict    Dict          `json:"Dict"`
		Stream  *Stream       `json:"Stream,omitempty"`
	}{Name: p.Name(), Page: p.Page(), HasPage: p.HasPage(), Stroke: p.Stroke(), GState: p.GState(), Dict: p.DictCopy(), Stream: stream})
}

// Patterns lazily yields named Pattern resources selected in page content
// order. Ordinary colors and unselected resource entries are not emitted.
func (p Page) Patterns(d *Document) iter.Seq2[PatternObject, error] {
	return p.patterns(d, true)
}

// Patterns yields Pattern resources directly selected by this Form XObject.
func (x XObjectObject) Patterns(d *Document) iter.Seq2[PatternObject, error] {
	return func(yield func(PatternObject, error) bool) {
		if err := x.validate(d); err != nil {
			yield(PatternObject{}, err)
			return
		}
		for object, err := range x.pageView(d).patterns(d, false) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (p Page) patterns(d *Document, flatten bool) iter.Seq2[PatternObject, error] {
	return func(yield func(PatternObject, error) bool) {
		if d == nil {
			yield(PatternObject{}, errNilDocument)
			return
		}
		if !p.hasCachedResources(d) {
			if raw, present := p.dict[Name("Resources")]; present {
				resourcesValue, resourcesResolved := d.resolveIndirectChain(raw)
				if !resourcesResolved {
					yield(PatternObject{}, fmt.Errorf("playa: Pattern resources could not be resolved"))
					return
				}
				if resources, ok := resourcesValue.(Dict); ok {
					if patterns, present := resources[Name("Pattern")]; present {
						if _, resolved := d.resolveIndirectChain(patterns); !resolved {
							yield(PatternObject{}, fmt.Errorf("playa: Pattern resources could not be resolved"))
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
				yield(PatternObject{}, err)
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
			if op.operatorValue() != "SCN" && op.operatorValue() != "scn" {
				continue
			}
			stroke := op.operatorValue() == "SCN"
			color := gstate.fillColor
			if stroke {
				color = gstate.strokeColor
			}
			if color.Space() != "Pattern" || color.Pattern() == "" {
				continue
			}
			pattern, err := p.patternFromOp(d, op, color.Pattern(), stroke, gstate)
			if err != nil {
				yield(PatternObject{}, err)
				return
			}
			if !yield(pattern, nil) {
				return
			}
		}
	}
}

func (p Page) patternFromOp(d *Document, op ContentOp, name string, stroke bool, gstate graphicsState) (PatternObject, error) {
	patternsValue, patternsResolved := d.resolveIndirectChain(op.resources[Name("Pattern")])
	if !patternsResolved && op.resources[Name("Pattern")] != nil {
		return PatternObject{}, fmt.Errorf("playa: Pattern resources could not be resolved")
	}
	patterns, ok := patternsValue.(Dict)
	if !ok {
		return PatternObject{}, fmt.Errorf("playa: Pattern resources are not a dictionary")
	}
	value, exists := patterns[Name(name)]
	if !exists {
		return PatternObject{}, fmt.Errorf("playa: pattern resource %q not found", name)
	}
	resolvedValue, valueResolved := d.resolveIndirectChain(value)
	if !valueResolved {
		return PatternObject{}, fmt.Errorf("playa: pattern resource %q could not be resolved", name)
	}
	value = resolvedValue
	spec := contentdata.PatternSpec{Name: name, Page: p.ref, HasPage: p.ref != (Ref{}), Stroke: stroke, GState: gstate.publicValue()}
	switch value := value.(type) {
	case Dict:
		spec.Dict = value
	case Stream:
		spec.Dict = value.DictCopy()
		return PatternObject{data: contentdata.NewPattern(spec), stream: value, hasStream: true}, nil
	default:
		return PatternObject{}, fmt.Errorf("playa: pattern resource %q is not a dictionary or stream", name)
	}
	return PatternObject{data: contentdata.NewPattern(spec)}, nil
}
