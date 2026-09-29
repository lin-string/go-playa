package document

import (
	"encoding/json"
	"fmt"
	"iter"

	"github.com/lin-string/go-playa/contentdata"
)

// ColorSpaceObject represents one named color-space selection in content.
type ColorSpaceObject struct {
	data contentdata.ColorSpaceSelection
	info ImageColorSpace
}

// Name returns the selected resource name.
func (c ColorSpaceObject) Name() string { return c.data.Name() }

// Page returns the owning page reference, when present.
func (c ColorSpaceObject) Page() Ref { return c.data.Page() }

// HasPage reports whether Page identifies an owning page.
func (c ColorSpaceObject) HasPage() bool { return c.data.HasPage() }

// Stroke reports whether the selection applies to the stroke color space.
func (c ColorSpaceObject) Stroke() bool { return c.data.Stroke() }

// Info returns an independent description of the selected color space.
func (c ColorSpaceObject) Info() ImageColorSpace { return c.info.Finalize() }

// SpecCopy returns an independent copy of the original ColorSpace resource.
func (c ColorSpaceObject) SpecCopy() Object { return c.data.SpecCopy() }

// Finalize returns an independent snapshot of the selection and description.
func (c ColorSpaceObject) Finalize() ColorSpaceObject {
	c.data = c.data.Finalize()
	c.info = c.info.Finalize()
	return c
}

// FinalizeWithError returns an independent snapshot and reports deferred
// color-space lookup failures.
func (c ColorSpaceObject) FinalizeWithError() (ColorSpaceObject, error) {
	info, err := c.info.FinalizeWithError()
	if err != nil {
		return ColorSpaceObject{}, err
	}
	clone := c.Finalize()
	clone.info = info
	return clone, nil
}

func (c ColorSpaceObject) MarshalJSON() ([]byte, error) {
	type projection struct {
		Name    string
		Page    Ref
		HasPage bool
		Stroke  bool
		Info    ImageColorSpace
		Spec    Object `json:"Spec"`
	}
	return json.Marshal(projection{Name: c.Name(), Page: c.Page(), HasPage: c.HasPage(), Stroke: c.Stroke(), Info: c.info, Spec: c.SpecCopy()})
}

// ColorSpaces lazily yields named ColorSpace resources selected by page
// content, preserving the source order and CS/cs stroke direction.
func (p Page) ColorSpaces(d *Document) iter.Seq2[ColorSpaceObject, error] {
	return p.colorSpaces(d, true)
}

// ColorSpaces yields ColorSpace resources directly selected by this Form.
func (x XObjectObject) ColorSpaces(d *Document) iter.Seq2[ColorSpaceObject, error] {
	return func(yield func(ColorSpaceObject, error) bool) {
		if err := x.validate(d); err != nil {
			yield(ColorSpaceObject{}, err)
			return
		}
		for object, err := range x.pageView(d).colorSpaces(d, false) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (p Page) colorSpaces(d *Document, flatten bool) iter.Seq2[ColorSpaceObject, error] {
	return func(yield func(ColorSpaceObject, error) bool) {
		if d == nil {
			yield(ColorSpaceObject{}, errNilDocument)
			return
		}
		if !p.hasCachedResources(d) {
			if raw, present := p.dict[Name("Resources")]; present {
				resourcesValue, resourcesResolved := d.resolveIndirectChain(raw)
				if !resourcesResolved {
					yield(ColorSpaceObject{}, fmt.Errorf("playa: ColorSpace resources could not be resolved"))
					return
				}
				if resources, ok := resourcesValue.(Dict); ok {
					if colorSpaces, present := resources[Name("ColorSpace")]; present {
						if _, resolved := d.resolveIndirectChain(colorSpaces); !resolved {
							yield(ColorSpaceObject{}, fmt.Errorf("playa: ColorSpace resources could not be resolved"))
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
				yield(ColorSpaceObject{}, err)
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
			if op.operatorValue() != "CS" && op.operatorValue() != "cs" {
				continue
			}
			if len(op.operandsValue()) == 0 {
				yield(ColorSpaceObject{}, fmt.Errorf("playa: %s operator has no resource name", op.operatorValue()))
				return
			}
			name, ok := op.operandsValue()[0].(Name)
			if !ok {
				yield(ColorSpaceObject{}, fmt.Errorf("playa: %s operator resource name is not a name", op.operatorValue()))
				return
			}
			resourcesValue, resourcesResolved := d.resolveIndirectChain(op.resources[Name("ColorSpace")])
			if !resourcesResolved && op.resources[Name("ColorSpace")] != nil {
				yield(ColorSpaceObject{}, fmt.Errorf("playa: ColorSpace resources could not be resolved"))
				return
			}
			resources, ok := resourcesValue.(Dict)
			spec, exists := Object(name), true
			if !ok || resources[name] == nil {
				if colorSpaceComponents(string(name)) == 0 && name != Name("Pattern") {
					yield(ColorSpaceObject{}, fmt.Errorf("playa: ColorSpace resource %q not found", name))
					return
				}
			} else {
				spec, exists = resources[name]
			}
			if !exists {
				yield(ColorSpaceObject{}, fmt.Errorf("playa: ColorSpace resource %q not found", name))
				return
			}
			resolvedSpec, specResolved := d.resolveIndirectChain(spec)
			if !specResolved && spec != nil {
				yield(ColorSpaceObject{}, fmt.Errorf("playa: ColorSpace resource %q could not be resolved", name))
				return
			}
			spec = resolvedSpec
			info := d.describeImageColorSpace(spec)
			if info.Name() == "" || info.Components() <= 0 {
				yield(ColorSpaceObject{}, fmt.Errorf("playa: ColorSpace resource %q is malformed", name))
				return
			}
			value := ColorSpaceObject{data: contentdata.NewColorSpaceSelection(contentdata.ColorSpaceSelectionSpec{
				Name: string(name), Page: p.ref, HasPage: p.ref != (Ref{}),
				Stroke: op.operatorValue() == "CS", Spec: spec,
			}), info: info}
			if !yield(value, nil) {
				return
			}
		}
	}
}
