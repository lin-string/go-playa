package document

import (
	"iter"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
)

// PageTags returns marked-content points in content-stream order. MP and DP
// are points rather than sections, so they do not appear in PageMarkedContent.
func (d *Document) PageTags(p Page) ([]TagObject, error) {
	var tags []TagObject
	for tag, err := range d.PageTagsSeq(p) {
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// PageTagsSeq yields MP/DP points without materializing the full page.
func (d *Document) PageTagsSeq(p Page) iter.Seq2[TagObject, error] {
	return func(yield func(TagObject, error) bool) {
		var properties Dict
		var propertiesErr error
		propertiesReady := false
		propertiesFor := func() (Dict, error) {
			if !propertiesReady {
				properties, propertiesErr = d.pagePropertiesChecked(p)
				propertiesReady = true
			}
			return properties, propertiesErr
		}
		iterator := newContentOpIterator(d, p, true)
		var iteratorErr error
		propertiesErr = nil
		interpretErr := interpretTagsNext(d, propertiesFor, p.ref, func() (ContentOp, bool) {
			op, err, ok := iterator.next()
			if err != nil {
				iteratorErr = err
				return ContentOp{}, false
			}
			return op, ok
		}, nil, func(tag TagObject) bool {
			return yield(tag, nil)
		})
		if interpretErr != nil {
			yield(TagObject{}, interpretErr)
		}
		if iteratorErr != nil {
			yield(TagObject{}, iteratorErr)
		}
	}
}

func (p Page) tags(d *Document, flatten bool, opts contentconfig.Options) iter.Seq2[TagObject, error] {
	return func(yield func(TagObject, error) bool) {
		if d == nil {
			yield(TagObject{}, errNilDocument)
			return
		}
		iterator := newContentOpIterator(d, p, flatten)
		var properties Dict
		var propertiesErr error
		propertiesReady := false
		propertiesFor := func() (Dict, error) {
			if !propertiesReady {
				properties, propertiesErr = d.pagePropertiesChecked(p)
				propertiesReady = true
			}
			return properties, propertiesErr
		}
		allowed := make(map[string]struct{}, len(opts.RestrictOps))
		for _, name := range opts.RestrictOps {
			allowed[name] = struct{}{}
		}
		var iteratorErr error
		interpretErr := interpretTagsNext(d, propertiesFor, p.ref, func() (ContentOp, bool) {
			op, err, ok := iterator.next()
			if err != nil {
				iteratorErr = err
				return ContentOp{}, false
			}
			if !ok {
				return ContentOp{}, false
			}
			return op, true
		}, allowed, func(tag TagObject) bool {
			return yield(tag, nil)
		})
		if interpretErr != nil {
			yield(TagObject{}, interpretErr)
		}
		if iteratorErr != nil {
			yield(TagObject{}, iteratorErr)
		}
	}
}

func interpretTagsNext(d *Document, properties func() (Dict, error), page Ref, next func() (ContentOp, bool), restrictOps map[string]struct{}, yield func(TagObject) bool) error {
	state := newGraphicsState()
	stack := []graphicsState{}
	marked := []markedContentFrame{}
	markedForms := [][]markedContentFrame{}
	for {
		op, ok := next()
		if !ok {
			return nil
		}
		if op.formBoundary != 0 {
			if op.formBoundary == formBegin {
				markedForms = append(markedForms, marked)
				marked = nil
			} else if len(markedForms) > 0 {
				last := len(markedForms) - 1
				marked = markedForms[last]
				markedForms = markedForms[:last]
			}
			continue
		}
		applyMarkedContent(&marked, op)
		switch op.operatorValue() {
		case "q":
			stack = append(stack, state.Clone())
		case "Q":
			if len(stack) > 0 {
				state = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		default:
			applyGraphicsState(&state, op)
			applyExternalGraphicsState(d, &state, op)
			applyResourceColorSpace(d, &state, op)
		}
		if op.operatorValue() != "MP" && op.operatorValue() != "DP" {
			continue
		}
		if len(restrictOps) > 0 {
			if _, ok := restrictOps[op.operatorValue()]; !ok {
				continue
			}
		}
		spec := contentdata.TagSpec{Page: page, HasPage: page != (Ref{}), MCID: -1, GState: state.publicValue()}
		context := currentMarkedContent(marked)
		spec.MarkedTag = context.Tag
		spec.MarkedProperties = context.Properties
		spec.MarkedStack = markedStackCopy(markedContextStack(marked))
		if len(op.operandsValue()) > 0 {
			if name, ok := op.operandsValue()[0].(Name); ok {
				spec.Name = string(name)
			}
		}
		if op.operatorValue() == "DP" && len(op.operandsValue()) > 1 {
			switch value := op.operandsValue()[1].(type) {
			case Dict:
				spec.Properties = resolveTagProperties(d, value)
			case Name:
				propertyMap, err := properties()
				if err != nil {
					return err
				}
				if props, ok := markedPropertiesDict(propertyMap[value]); ok {
					spec.Properties = props
				}
			}
		}
		if spec.Properties != nil {
			if id, ok := IntValue(spec.Properties[Name("MCID")]); ok {
				spec.MCID, spec.HasMCID = id, true
			}
			if text, ok := spec.Properties[Name("ActualText")].(String); ok {
				spec.ActualText = decodePDFText(text)
			}
		}
		tag := newTagObject(spec)
		tag.parentKey, tag.hasParentKey = op.parentKey, op.hasParentKey
		if !yield(tag) {
			return nil
		}
	}
}

func resolveTagProperties(d *Document, props Dict) Dict {
	resolveRef := func(ref Ref) (Object, bool) {
		value, err := d.resolveRefRaw(ref)
		return value, err == nil
	}
	// These fields drive interpretation; retain the other property values as
	// raw PDF objects, including references to metadata streams or dictionaries.
	resolved := cloneDict(props)
	for _, key := range []Name{"MCID", "ActualText"} {
		if value, ok := props[key]; ok {
			resolved[key] = ResolveAll(value, resolveRef)
		}
	}
	return resolved
}
