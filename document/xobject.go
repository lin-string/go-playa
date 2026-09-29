package document

import (
	"fmt"

	"github.com/lin-string/go-playa/geometry"
)

func (d *Document) formCycleKey(value Object) formCycleKey {
	ref, hasRef := value.(Ref)
	if hasRef {
		if canonical, ok := d.finalIndirectRef(value); ok {
			ref = canonical
		}
	}
	return formCycleKey{ref: ref, direct: !hasRef}
}

func formBBox(d *Document, stream Stream) [4]float64 {
	bbox, _ := formBBoxValue(d, stream)
	return bbox
}

func formBBoxValue(d *Document, stream Stream) ([4]float64, bool) {
	var bbox [4]float64
	resolved, _ := d.resolveIndirectChain(stream.DictBorrowed()[Name("BBox")])
	values, ok := resolved.(Array)
	if !ok || len(values) != len(bbox) {
		return bbox, false
	}
	for i := range bbox {
		var valid bool
		item, _ := d.resolveIndirectChain(values[i])
		bbox[i], valid = finiteNumberValue(item)
		if !valid {
			return [4]float64{}, false
		}
	}
	if bbox[0] > bbox[2] {
		bbox[0], bbox[2] = bbox[2], bbox[0]
	}
	if bbox[1] > bbox[3] {
		bbox[1], bbox[3] = bbox[3], bbox[1]
	}
	return bbox, true
}

func formBBoxForPage(d *Document, p Page, stream Stream, matrix geometry.Matrix) [4]float64 {
	if bbox, ok := formBBoxValue(d, stream); ok {
		if matrix != (geometry.Matrix{}) {
			if transformed, ok := transformBBox(matrix, bbox); ok {
				return transformed
			}
			return p.CropBox(d)
		}
		return bbox
	}
	return p.CropBox(d)
}

func transformBBox(matrix geometry.Matrix, bbox [4]float64) ([4]float64, bool) {
	points := [4][2]float64{{bbox[0], bbox[1]}, {bbox[0], bbox[3]}, {bbox[2], bbox[1]}, {bbox[2], bbox[3]}}
	minX, minY := 0.0, 0.0
	maxX, maxY := 0.0, 0.0
	for i, point := range points {
		x, y, ok := matrix.PointFinite(point[0], point[1])
		if !ok {
			return [4]float64{}, false
		}
		if i == 0 {
			minX, maxX, minY, maxY = x, x, y, y
			continue
		}
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	return [4]float64{minX, minY, maxX, maxY}, true
}

func formGroup(d *Document, stream Stream) Dict {
	groupValue, _ := d.resolveIndirectChain(stream.DictBorrowed()[Name("Group")])
	group, _ := groupValue.(Dict)
	return group
}

func formGraphicsState(d *Document, stream Stream, g graphicsState) graphicsState {
	group := formGroup(d, stream)
	sectionValue, _ := d.resolveIndirectChain(group[Name("S")])
	section, _ := sectionValue.(Name)
	if section != Name("Transparency") {
		return g
	}
	g.blendMode = "Normal"
	g.blendModes = nil
	g.alpha = 1
	g.strokeAlpha = 1
	g.fillAlpha = 1
	g.softMask = nil
	g.hasSoftMask = false
	return g
}

func formParentKey(d *Document, stream Stream) (int, bool) {
	// Playa accepts either the item-level StructParent or the container-level
	// StructParents entry. StructParent takes precedence when both are present,
	// matching the PDF constraint that the two entries are mutually exclusive.
	streamDict := stream.DictBorrowed()
	raw, present := streamDict[Name("StructParent")]
	if !present {
		raw, present = streamDict[Name("StructParents")]
	}
	if !present {
		return 0, false
	}
	keyValue, _ := d.resolveIndirectChain(raw)
	key, ok := IntValue(keyValue)
	return key, ok && key >= 0
}

func (d *Document) PageContentOps(p Page) ([]ContentOp, error) {
	b, e := p.Content(d)
	if e != nil {
		return nil, e
	}
	ops, e := ParseContent(b)
	if e != nil {
		return nil, e
	}
	ctx := p.dict
	if resources, ok := d.cachedPageResource(p.ref); p.hasPageCacheKey() && ok {
		ctx = make(Dict, len(p.dict)+1)
		for key, value := range p.dict {
			ctx[key] = value
		}
		ctx[Name("Resources")] = resources
	}
	if e := d.validatePropertiesResources(ctx, 0); e != nil {
		return nil, e
	}
	return d.expandOps(pageMatrixOps(p, d, ops), ctx, 0, ""), nil
}

func (d *Document) validatePropertiesResources(ctx Dict, depth int) error {
	return d.validatePropertiesResourcesSeen(ctx, depth, make(map[Ref]int))
}

func (d *Document) validatePropertiesResourcesSeen(ctx Dict, depth int, seen map[Ref]int) error {
	if depth >= 32 {
		return nil
	}
	resources := Dict{}
	if raw, present := ctx[Name("Resources")]; present {
		var ok bool
		resolved, resolvedOK := d.resolveIndirectChain(raw)
		if !resolvedOK {
			return fmt.Errorf("playa: Resources could not be resolved")
		}
		resources, ok = resolved.(Dict)
		if !ok {
			return fmt.Errorf("playa: Resources is not a dictionary")
		}
		if ref, ok := d.finalIndirectRef(raw); ok {
			if previousDepth, visited := seen[ref]; visited && previousDepth <= depth {
				return nil
			}
			seen[ref] = depth
		}
	}
	for _, key := range []Name{"Font", "XObject", "ExtGState", "ColorSpace", "Pattern", "Shading", "Properties"} {
		if raw, present := resources[key]; present {
			resolved, resolvedOK := d.resolveIndirectChain(raw)
			if !resolvedOK {
				return fmt.Errorf("playa: %s resources could not be resolved", key)
			}
			if _, ok := resolved.(Dict); !ok {
				return fmt.Errorf("playa: %s resources are not a dictionary", key)
			}
		}
	}
	if err := d.validateResourceEntries(resources); err != nil {
		return err
	}
	if raw, present := resources[Name("Properties")]; present {
		resolved, resolvedOK := d.resolveIndirectChain(raw)
		if !resolvedOK {
			return fmt.Errorf("playa: Properties resources could not be resolved")
		}
		properties, ok := resolved.(Dict)
		if !ok {
			return fmt.Errorf("playa: Properties resources are not a dictionary")
		}
		for name, value := range properties {
			resolved, resolvedOK := d.resolveIndirectChain(value)
			if !resolvedOK {
				return fmt.Errorf("playa: property resource %q could not be resolved", name)
			}
			if _, ok := markedPropertiesDict(resolved); !ok {
				return fmt.Errorf("playa: property resource %q is not a dictionary", name)
			}
		}
	}
	if raw, present := resources[Name("XObject")]; present {
		resolved, resolvedOK := d.resolveIndirectChain(raw)
		if !resolvedOK {
			return fmt.Errorf("playa: XObject resources could not be resolved")
		}
		xobjects, ok := resolved.(Dict)
		if !ok {
			return fmt.Errorf("playa: XObject resources are not a dictionary")
		}
		for _, value := range xobjects {
			streamValue, streamResolved := d.resolveIndirectChain(value)
			if !streamResolved {
				return fmt.Errorf("playa: XObject resource could not be resolved")
			}
			stream, ok := streamValue.(Stream)
			if !ok {
				continue
			}
			streamDict := stream.DictBorrowed()
			subtypeValue, subtypeResolved := d.resolveIndirectChain(streamDict[Name("Subtype")])
			if streamDict[Name("Subtype")] != nil && !subtypeResolved {
				return fmt.Errorf("playa: XObject subtype could not be resolved")
			}
			if subtypeValue != Name("Form") {
				continue
			}
			if ref, hasRef := value.(Ref); hasRef {
				canonical := ref
				if resolved, ok := d.finalIndirectRef(value); ok {
					canonical = resolved
				}
				// Form resource graphs can share and cycle without recursive Do
				// operations. Revisit only along a shallower path so the depth
				// bound cannot hide resources reachable from another branch.
				if previousDepth, visited := seen[canonical]; visited && previousDepth <= depth+1 {
					continue
				}
				seen[canonical] = depth + 1
				if err, ok := d.cachedXObjectResourceError(ref); ok {
					return err
				}
				if cached, ok := d.cachedXObjectResource(ref); ok {
					if err := d.validatePropertiesResourcesSeen(Dict{Name("Resources"): cached}, depth+1, seen); err != nil {
						return err
					}
					continue
				}
			}
			if err := d.validatePropertiesResourcesSeen(streamDict, depth+1, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Document) validateResourceEntries(resources Dict) error {
	validate := func(key Name, valid func(Object) bool) error {
		raw, present := resources[key]
		if !present {
			return nil
		}
		entriesValue, entriesResolved := d.resolveIndirectChain(raw)
		if !entriesResolved {
			return fmt.Errorf("playa: %s resources could not be resolved", key)
		}
		entries, _ := entriesValue.(Dict)
		for name, value := range entries {
			resolved, resolvedOK := d.resolveIndirectChain(value)
			if !resolvedOK {
				return fmt.Errorf("playa: %s resource %q could not be resolved", key, name)
			}
			if !valid(resolved) {
				return fmt.Errorf("playa: %s resource %q has an invalid value", key, name)
			}
		}
		return nil
	}
	if err := validate(Name("Font"), func(value Object) bool { _, ok := value.(Dict); return ok }); err != nil {
		return err
	}
	if err := validate(Name("XObject"), func(value Object) bool { _, ok := value.(Stream); return ok }); err != nil {
		return err
	}
	if err := validate(Name("ExtGState"), func(value Object) bool { _, ok := value.(Dict); return ok }); err != nil {
		return err
	}
	if err := validate(Name("ColorSpace"), func(value Object) bool {
		switch value.(type) {
		case Name, Array:
			return true
		default:
			return false
		}
	}); err != nil {
		return err
	}
	for _, key := range []Name{"Pattern", "Shading"} {
		if err := validate(key, func(value Object) bool {
			_, dict := value.(Dict)
			_, stream := value.(Stream)
			return dict || stream
		}); err != nil {
			return err
		}
	}
	return nil
}
func (d *Document) expandOps(ops []ContentOp, ctx Dict, depth int, path string) []ContentOp {
	return d.expandOpsSeen(ops, ctx, depth, path, map[formCycleKey]bool{})
}

func (d *Document) expandOpsSeen(ops []ContentOp, ctx Dict, depth int, path string, seen map[formCycleKey]bool) []ContentOp {
	if depth > 32 {
		return ops
	}
	var out []ContentOp
	resValue, _ := d.resolveIndirectChain(ctx[Name("Resources")])
	res, _ := resValue.(Dict)
	xosValue, _ := d.resolveIndirectChain(res[Name("XObject")])
	xos, _ := xosValue.(Dict)
	withResources := func(op ContentOp) ContentOp {
		if res != nil {
			op.resources = res
		}
		return op
	}
	ensureOutput := func(index int) {
		if out == nil {
			out = make([]ContentOp, 0, len(ops))
			for _, op := range ops[:index] {
				out = append(out, withResources(op))
			}
		}
	}
	for index, op := range ops {
		if op.operatorValue() != "Do" || len(op.operandsValue()) == 0 {
			if out != nil {
				out = append(out, withResources(op))
			}
			continue
		}
		name, ok := op.operandsValue()[0].(Name)
		if !ok {
			if out != nil {
				out = append(out, withResources(op))
			}
			continue
		}
		streamValue, _ := d.resolveIndirectChain(xos[name])
		s, ok := streamValue.(Stream)
		if !ok {
			if out != nil {
				out = append(out, withResources(op))
			}
			continue
		}
		sDict := s.DictBorrowed()
		typValue, _ := d.resolveIndirectChain(sDict[Name("Subtype")])
		typ, _ := typValue.(Name)
		if typ != Name("Form") {
			if out != nil {
				out = append(out, withResources(op))
			}
			continue
		}
		key := d.formCycleKey(xos[name])
		if seen[key] {
			// Playa's flatten iterator omits a Form already present in
			// the active parent chain instead of yielding the cycle again.
			ensureOutput(index)
			continue
		}
		seen[key] = true
		raw, e := decodeContentStream(d, xos[name], s)
		if e != nil {
			delete(seen, key)
			if out != nil {
				out = append(out, op)
			}
			continue
		}
		sub, e := ParseContent(raw)
		if e != nil {
			delete(seen, key)
			if out != nil {
				out = append(out, op)
			}
			continue
		}
		wrapped := make([]ContentOp, 0, len(sub)+5)
		wrapped = append(wrapped, ContentOp{formBoundary: formBegin}, newContentOpBorrowed("q", nil, 0))
		matrixValue, _ := d.resolveIndirectChain(sDict[Name("Matrix")])
		if matrix, ok := matrixValue.(Array); ok && validFormMatrix(d, matrix) {
			operands := make([]Object, 6)
			for i := range operands {
				operands[i], _ = d.resolveIndirectChain(matrix[i])
			}
			wrapped = append(wrapped, newContentOpBorrowed("cm", operands, 0))
		}
		wrapped = append(wrapped, sub...)
		wrapped = append(wrapped, ContentOp{formBoundary: formEnd}, newContentOpBorrowed("Q", nil, 0))
		sub = wrapped
		subctx := ctx
		cachedResources, cached := Dict(nil), false
		if ref, ok := xos[name].(Ref); ok {
			cachedResources, cached = d.cachedXObjectResource(ref)
		}
		if cached {
			subctx = mergeDict(ctx, Dict{Name("Resources"): cachedResources})
		} else if r, ok := sDict[Name("Resources")]; ok {
			resolved, _ := d.resolveIndirectChain(r)
			if v, ok := resolved.(Dict); ok {
				subctx = mergeDict(ctx, Dict{Name("Resources"): v})
			}
		}
		fontPrefix := path + string(name) + "/"
		for i := range sub {
			if sub[i].operatorValue() == "Tf" && len(sub[i].operandsValue()) >= 2 {
				if n, ok := sub[i].operandsValue()[0].(Name); ok {
					sub[i].setOperands(append([]Object(nil), sub[i].operandsValue()...))
					sub[i].operandsValue()[0] = Name(fontPrefix + string(n))
				}
			}
		}
		ensureOutput(index)
		out = append(out, d.expandOpsSeen(sub, subctx, depth+1, fontPrefix, seen)...)
		delete(seen, key)
	}
	if out == nil {
		if res != nil {
			out = make([]ContentOp, len(ops))
			for i, op := range ops {
				out[i] = withResources(op)
			}
			return out
		}
		return ops
	}
	return out
}
func (d *Document) PageTextExpanded(p Page) ([]TextObject, error) {
	ops, e := d.PageContentOps(p)
	if e != nil {
		return nil, e
	}
	fonts, e := d.pageFontsExpanded(p)
	if e != nil {
		return nil, fmt.Errorf("playa: page fonts: %w", e)
	}
	properties, err := d.pagePropertiesChecked(p)
	if err != nil {
		return nil, err
	}
	ops = d.expandContentPropertiesWithResources(ops, properties)
	return materializeTextWithDocument(d, p.ref, ops, fonts), nil
}

func materializeTextWithDocument(d *Document, page Ref, ops []ContentOp, fonts map[string]*Font) []TextObject {
	texts := []TextObject{}
	index := 0
	interpretTextWithFontResolverNextPage(d, page, func() (ContentOp, bool) {
		if index >= len(ops) {
			return ContentOp{}, false
		}
		op := ops[index]
		index++
		return op, true
	}, func(name string) *Font {
		return fonts[name]
	}, false, nil, func(text TextObject) bool {
		texts = append(texts, text)
		return true
	})
	return texts
}

// PagePathsExpanded mirrors PageTextExpanded for geometry. Form XObjects are
// expanded by PageContentOps, including their Matrix and graphics-state
// save/restore operators, before path interpretation.
func (d *Document) PagePathsExpanded(p Page) ([]PathObject, error) {
	ops, err := d.PageContentOps(p)
	if err != nil {
		return nil, err
	}
	properties, err := d.pagePropertiesChecked(p)
	if err != nil {
		return nil, err
	}
	ops = d.expandContentPropertiesWithResources(ops, properties)
	paths := []PathObject{}
	index := 0
	interpretPathsNext(d, func() (ContentOp, bool) {
		if index >= len(ops) {
			return ContentOp{}, false
		}
		op := ops[index]
		index++
		return op, true
	}, nil, func(path PathObject) bool {
		paths = append(paths, path)
		return true
	})
	return paths, nil
}
