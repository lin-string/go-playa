package document

import (
	"fmt"
	"strings"
)

type contentFrame struct {
	streams      []Object
	index        int
	parser       *ObjectParser
	data         []byte
	operands     []Object
	ctx          Dict
	prefix       string
	matrix       Array
	ref          Ref
	hasRef       bool
	formKey      formCycleKey
	hasForm      bool
	parentKey    int
	hasParentKey bool
}

// formCycleKey mirrors Playa's active Form identity. Indirect forms use their
// object reference; direct streams have no object id and intentionally share
// one identity, matching Playa's stream-id fallback for direct forms.
type formCycleKey struct {
	ref    Ref
	direct bool
}

const decodedStreamCacheLimit = 32 << 20

const inlineImageCacheLimit = 32 << 20

const decodedStreamErrorLimit = 64 << 10
const contentRootErrorLimit = 64 << 10

func (d *Document) cacheContentRootError(ref Ref, err error) error {
	if ref == (Ref{}) || err == nil || len(err.Error()) > d.cacheLimits().ContentRootErrorBytes {
		return err
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if d.contentRootErrors == nil {
		d.contentRootErrors = map[Ref]error{}
	}
	if cached, ok := d.contentRootErrors[ref]; ok {
		return cached
	}
	if !cacheFits(d.contentRootErrorBytes, len(err.Error()), d.cacheLimits().ContentRootErrorBytes) {
		return err
	}
	d.contentRootErrors[ref] = err
	d.contentRootErrorBytes += len(err.Error())
	return err
}

func (d *Document) cacheInlineImage(key inlineImageKey, image Stream) {
	limit := d.cacheLimits().InlineImageBytes
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if !cacheFits(d.inlineImageCacheBytes, len(image.DataBorrowed()), limit) {
		return
	}
	if d.inlineImageCache == nil {
		d.inlineImageCache = map[inlineImageKey]Stream{}
	}
	if _, exists := d.inlineImageCache[key]; exists {
		return
	}
	d.inlineImageCache[key] = image
	d.inlineImageCacheBytes += len(image.DataBorrowed())
}

// contentOpIterator is the operator-level counterpart of ParseContent. It
// keeps only the current decoded stream and parser alive. Form XObjects are
// pushed as frames when their Do operator is reached, so unused forms are
// never decoded.
type contentOpIterator struct {
	d          *Document
	page       Page
	flatten    bool
	raw        bool
	frames     []contentFrame
	started    bool
	matrixDone bool
	initial    []ContentOp
	pending    []ContentOp
	seen       map[formCycleKey]bool
	initErr    error
}

func newContentOpIterator(d *Document, p Page, flatten ...bool) *contentOpIterator {
	rawValue := p.dict[Name("Contents")]
	value, _, resolveErr := d.resolveContentIndirectChain(rawValue)
	streams, typeErr := resolvedContentStreams(rawValue, value)
	if resolveErr == nil {
		resolveErr = typeErr
	}
	pageRef := p.ref
	it := &contentOpIterator{
		d: d, page: p, flatten: len(flatten) > 0 && flatten[0], seen: map[formCycleKey]bool{},
		initErr: resolveErr,
	}
	if p.hasInitialGState {
		it.initial = graphicsStateOps(p.initialGState)
	}
	resourceContext := p.dict[Name("Resources")]
	if p.hasPageCacheKey() {
		if resources, ok := d.cachedPageResource(pageRef); ok {
			resourceContext = resources
		}
	}
	ctx := Dict{Name("Resources"): resourceContext}
	it.frames = []contentFrame{{streams: streams, ctx: ctx}}
	return it
}

func newRawContentOpIterator(d *Document, p Page) *contentOpIterator {
	it := newContentOpIterator(d, p)
	it.started = true
	it.matrixDone = true
	it.raw = true
	return it
}

func newXObjectOpIterator(d *Document, p Page, value Object, stream Stream, resources Dict) *contentOpIterator {
	return &contentOpIterator{
		d: d, page: p, started: true, seen: map[formCycleKey]bool{},
		matrixDone: true,
		frames:     []contentFrame{{streams: []Object{value}, ctx: Dict{Name("Resources"): resources}}},
	}
}

func contentStreams(value Object) []Object {
	switch value := value.(type) {
	case Stream, Ref:
		return []Object{value}
	case Array:
		return value
	default:
		return nil
	}
}

func resolvedContentStreams(raw, resolved Object) ([]Object, error) {
	if _, ok := resolved.(Stream); ok {
		if _, indirect := raw.(Ref); indirect {
			return []Object{raw}, nil
		}
		return []Object{resolved}, nil
	}
	if streams := contentStreams(resolved); streams != nil {
		return streams, nil
	}
	if resolved == nil {
		if raw != nil {
			return []Object{raw}, nil
		}
		return nil, nil
	}
	return nil, fmt.Errorf("playa: page Contents is not a stream or array")
}

func (it *contentOpIterator) next() (ContentOp, error, bool) {
	if it.initErr != nil {
		err := it.initErr
		it.initErr = nil
		return ContentOp{}, err, false
	}
	if !it.started {
		it.started = true
	}
	if len(it.initial) > 0 {
		op := it.initial[0]
		it.initial = it.initial[1:]
		return op, nil, true
	}
	if !it.matrixDone {
		it.matrixDone = true
		if it.page.hasContentMatrix {
			operands := make([]Object, 6)
			for i, value := range it.page.contentMatrix {
				operands[i] = Number(value)
			}
			return newContentOpBorrowed("cm", operands, 0), nil, true
		}
		if matrix := it.page.MatrixIn(it.d, it.page.coordinateSpace(it.d)); matrix != identity() {
			operands := make([]Object, 6)
			for i, value := range matrix {
				operands[i] = Number(value)
			}
			return newContentOpBorrowed("cm", operands, 0), nil, true
		}
	}
	if len(it.pending) > 0 {
		op := it.pending[0]
		it.pending = it.pending[1:]
		return op, nil, true
	}
	for len(it.frames) > 0 {
		last := len(it.frames) - 1
		frame := &it.frames[last]
		if frame.parser == nil {
			if frame.index >= len(frame.streams) {
				if last == 0 {
					frame.operands = frame.operands[:0]
					return ContentOp{}, nil, false
				}
				it.frames = it.frames[:last]
				if frame.hasForm {
					delete(it.seen, frame.formKey)
				}
				it.pending = append(it.pending, newContentOpBorrowed("Q", nil, 0), ContentOp{formBoundary: formEnd})
				return it.next()
			}
			streamValue := frame.streams[frame.index]
			resolved, resolvedOK, resolveErr := it.d.resolveContentIndirectChain(streamValue)
			stream, ok := resolved.(Stream)
			frame.index++
			if resolveErr != nil {
				return ContentOp{}, resolveErr, false
			}
			if !resolvedOK {
				continue
			}
			if !ok {
				continue
			}
			data, err := decodeContentStream(it.d, streamValue, stream)
			if err != nil {
				return ContentOp{}, err, false
			}
			frame.data = data
			frame.parser = NewObjectParser(data)
		}

		token, err := frame.parser.NextToken()
		if err != nil {
			if recoverTrailingContentParseError(err) {
				frame.parser = nil
				continue
			}
			return ContentOp{}, err, false
		}
		if token.Kind() == TokenEOF {
			frame.parser = nil
			continue
		}
		if token.Kind() == TokenArrayEnd || token.Kind() == TokenDictEnd {
			continue
		}
		if token.Kind() == TokenKeyword && token.Text() != "{" {
			if token.Text() == "BI" {
				key := inlineImageKey{page: it.page.ref, stream: frame.index - 1, offset: token.Offset()}
				var image Stream
				cached := it.page.ref != (Ref{})
				if cached {
					it.d.cacheMu.RLock()
					image, cached = it.d.inlineImageCache[key]
					it.d.cacheMu.RUnlock()
				}
				if !cached {
					op, err := inlineImageFromParser(frame.parser, frame.data, token.Offset())
					if err != nil {
						if recoverTrailingContentParseError(err) {
							frame.parser = nil
							continue
						}
						return ContentOp{}, err, false
					}
					image, _ = op.operandsValue()[0].(Stream)
					if it.page.ref != (Ref{}) {
						it.d.cacheInlineImage(key, image)
					}
				} else if err := skipInlineImageFromParser(frame.parser, frame.data); err != nil {
					if recoverTrailingContentParseError(err) {
						frame.parser = nil
						continue
					}
					return ContentOp{}, err, false
				}
				frame.operands = frame.operands[:0]
				op := newContentOpWithContext("BI", []Object{image}, token.Offset(), it.resources(frame), 0, "", false)
				op.parentKey, op.hasParentKey = frame.parentKey, frame.hasParentKey
				return op, nil, true
			}
			op := newContentOpBorrowed(token.Text(), frame.operands, token.Offset())
			op.parentKey, op.hasParentKey = frame.parentKey, frame.hasParentKey
			if (op.operatorValue() == "BDC" || op.operatorValue() == "DP") && len(op.operandsValue()) > 1 {
				if name, ok := op.operandsValue()[1].(Name); ok {
					op.propertyName, op.hasPropertyName = name, true
				}
			}
			if op.operatorValue() == "Tf" && len(op.operandsValue()) > 0 && frame.prefix != "" {
				if name, ok := op.operandsValue()[0].(Name); ok {
					op.setOperands(append([]Object(nil), op.operandsValue()...))
					op.operandsValue()[0] = Name(frame.prefix + string(name))
				}
			}
			if !it.raw && (op.operatorValue() == "BDC" || op.operatorValue() == "DP") && len(op.operandsValue()) > 1 {
				if name, ok := op.operandsValue()[1].(Name); ok {
					if properties, ok, err := it.propertyFor(frame, name); err != nil {
						return ContentOp{}, err, false
					} else if ok {
						op.setOperands(append([]Object(nil), op.operandsValue()...))
						op.operandsValue()[1] = properties
					}
				}
			}
			if !it.raw {
				rawResources, present := frame.ctx[Name("Resources")]
				if !present || rawResources == nil {
					frame.operands = frame.operands[:0]
					return op, nil, true
				}
				resources, resolved := it.d.resolveIndirectChain(rawResources)
				if !resolved {
					return ContentOp{}, fmt.Errorf("playa: Resources could not be resolved"), false
				}
				var ok bool
				op.resources, ok = resources.(Dict)
				if !ok {
					return ContentOp{}, fmt.Errorf("playa: Resources is not a dictionary"), false
				}
				if err := it.validateResourceOperation(op); err != nil {
					return ContentOp{}, err, false
				}
			}
			// ContentOp operands are borrowed until the iterator advances;
			// Finalize or OperandsCopy provides a stable snapshot.
			frame.operands = frame.operands[:0]
			if op.operatorValue() == "Do" && it.flatten {
				if nested, ok, err := it.formFrame(*frame, op); err != nil {
					return ContentOp{}, err, false
				} else if ok {
					it.pending = append(it.pending, ContentOp{formBoundary: formBegin}, newContentOpBorrowed("q", nil, 0))
					if formValue, _ := it.d.resolveIndirectChain(nested.streams[0]); formValue != nil {
						if form, ok := formValue.(Stream); ok && formGraphicsStateResetNeeded(it.d, form) {
							it.pending = append(it.pending,
								newContentOpBorrowed("BM", []Object{Name("Normal")}, 0),
								newContentOpBorrowed("CA", []Object{Number(1)}, 0),
								newContentOpBorrowed("ca", []Object{Number(1)}, 0),
								newContentOpBorrowed("SMask", []Object{Name("None")}, 0),
							)
						}
					}
					if validFormMatrix(it.d, nested.matrix) {
						operands := make([]Object, 6)
						for i := range operands {
							operands[i], _ = it.d.resolveIndirectChain(nested.matrix[i])
						}
						it.pending = append(it.pending, newContentOpBorrowed("cm", operands, 0))
					}
					it.frames = append(it.frames, nested)
					return it.next()
				}
			}
			return op, nil, true
		}
		object, err := frame.parser.ParseToken(token)
		if err != nil {
			if isRecoverableContentParseError(err) {
				frame.parser = nil
				continue
			}
			return ContentOp{}, err, false
		}
		if contentObjectHasIndirectReference(object) {
			return ContentOp{}, errContentStreamIndirectReference, false
		}
		frame.operands = append(frame.operands, object)
	}
	return ContentOp{}, nil, false
}

func (it *contentOpIterator) validateResourceOperation(op ContentOp) error {
	if len(op.operandsValue()) == 0 || op.resources == nil {
		return nil
	}
	name, ok := op.operandsValue()[0].(Name)
	if !ok {
		return nil
	}
	var key Name
	switch op.operatorValue() {
	case "gs":
		key = Name("ExtGState")
	case "CS", "cs":
		key = Name("ColorSpace")
	case "sh":
		key = Name("Shading")
	default:
		return nil
	}
	raw, present := op.resources[key]
	if !present || raw == nil {
		if op.operatorValue() == "CS" || op.operatorValue() == "cs" {
			return nil
		}
		return nil
	}
	value, resolved := it.d.resolveIndirectChain(raw)
	if !resolved {
		return fmt.Errorf("playa: %s resources could not be resolved", key)
	}
	resources, ok := value.(Dict)
	if !ok {
		return fmt.Errorf("playa: %s resources are not a dictionary", key)
	}
	entry, present := resources[name]
	if !present || entry == nil {
		if op.operatorValue() == "CS" || op.operatorValue() == "cs" {
			if colorSpaceComponents(string(name)) > 0 || name == Name("Pattern") {
				return nil
			}
		}
		return nil
	}
	if _, resolved := it.d.resolveIndirectChain(entry); !resolved {
		if op.operatorValue() == "sh" {
			return fmt.Errorf("playa: shading resource %q could not be resolved", name)
		}
		if op.operatorValue() == "CS" || op.operatorValue() == "cs" {
			return fmt.Errorf("playa: ColorSpace resource %q could not be resolved", name)
		}
		return fmt.Errorf("playa: ExtGState resource %q could not be resolved", name)
	}
	return nil
}

func formGraphicsStateResetNeeded(d *Document, stream Stream) bool {
	group := formGroup(d, stream)
	sectionValue, _ := d.resolveIndirectChain(group[Name("S")])
	section, _ := sectionValue.(Name)
	return section == Name("Transparency")
}

func isRecoverableContentParseError(err error) bool {
	message := err.Error()
	return strings.Contains(message, "dictionary value is missing") ||
		strings.Contains(message, "unterminated array") ||
		strings.Contains(message, "unterminated dictionary") ||
		strings.Contains(message, "unterminated procedure")
}

func decodeContentStream(d *Document, value Object, stream Stream) ([]byte, error) {
	return decodeContentStreamWithDecoder(d, value, stream, func(data []byte, filters []string, parms []Dict) ([]byte, error) {
		return decodeFiltersLenientLimited(data, filters, parms, decodedFilterExpansionLimit)
	})
}

func decodeContentStreamWithDecoder(d *Document, value Object, stream Stream, decoder func([]byte, []string, []Dict) ([]byte, error)) ([]byte, error) {
	d.decodedStreamLifecycle.RLock()
	defer d.decodedStreamLifecycle.RUnlock()
	ref, hasRef := d.finalIndirectRef(value)
	if hasRef {
		d.cacheMu.Lock()
		data, ok := d.decodedStreamCache[ref]
		if ok {
			d.cacheMu.Unlock()
			return data, nil
		}
		if err, ok := d.decodedStreamErrors[ref]; ok {
			d.cacheMu.Unlock()
			return nil, err
		}
		if pending, ok := d.decodedStreamInflight[ref]; ok {
			d.cacheMu.Unlock()
			<-pending.done
			return pending.data, pending.err
		}
		if d.decodedStreamInflight == nil {
			d.decodedStreamInflight = map[Ref]*decodedStreamDecode{}
		}
		pending := &decodedStreamDecode{done: make(chan struct{})}
		d.decodedStreamInflight[ref] = pending
		d.cacheMu.Unlock()
		data, err := decodeContentStreamOnce(stream, d.resolveRaw, decoder)
		d.cacheMu.Lock()
		delete(d.decodedStreamInflight, ref)
		pending.data, pending.err = data, err
		if err != nil {
			errorBytes := len(err.Error())
			limit := d.cacheLimits().DecodedErrorBytes
			if cacheFits(d.decodedStreamErrorBytes, errorBytes, limit) {
				if d.decodedStreamErrors == nil {
					d.decodedStreamErrors = map[Ref]error{}
				}
				d.decodedStreamErrors[ref] = err
				d.decodedStreamErrorBytes += errorBytes
			}
		}
		limit := d.cacheLimits().DecodedStreamBytes
		if err == nil && cacheFits(d.decodedStreamCacheBytes, len(data), limit) {
			if d.decodedStreamCache == nil {
				d.decodedStreamCache = map[Ref][]byte{}
			}
			d.decodedStreamCache[ref] = data
			d.decodedStreamCacheBytes += len(data)
		}
		close(pending.done)
		d.cacheMu.Unlock()
		return data, err
	}
	return decodeContentStreamOnce(stream, d.resolveRaw, decoder)
}

func decodeContentStreamOnce(stream Stream, resolve func(Object) Object, decoder func([]byte, []string, []Dict) ([]byte, error)) ([]byte, error) {
	streamDict := stream.DictBorrowed()
	filters, parms := streamFiltersWithResolver(streamDict, resolve)
	data, err := decoder(stream.DataBorrowed(), filters, parms)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (it *contentOpIterator) resources(frame *contentFrame) Dict {
	value, _ := it.d.resolveIndirectChain(frame.ctx[Name("Resources")])
	resources, _ := value.(Dict)
	return resources
}

func (it *contentOpIterator) propertyFor(frame *contentFrame, name Name) (Dict, bool, error) {
	resourcesRaw, present := frame.ctx[Name("Resources")]
	if !present || resourcesRaw == nil {
		return nil, false, nil
	}
	resourcesValue, resolved := it.d.resolveIndirectChain(resourcesRaw)
	if !resolved {
		return nil, false, fmt.Errorf("playa: Resources could not be resolved")
	}
	resources, ok := resourcesValue.(Dict)
	if !ok {
		return nil, false, fmt.Errorf("playa: Resources is not a dictionary")
	}
	propertiesRaw, present := resources[Name("Properties")]
	if !present || propertiesRaw == nil {
		return nil, false, nil
	}
	rawValue, resolved := it.d.resolveIndirectChain(propertiesRaw)
	if !resolved {
		return nil, false, fmt.Errorf("playa: Properties resources could not be resolved")
	}
	raw, ok := rawValue.(Dict)
	if !ok {
		return nil, false, fmt.Errorf("playa: Properties resources are not a dictionary")
	}
	value, ok := raw[name]
	if !ok {
		return nil, false, nil
	}
	resolvedValue, resolvedOK := it.d.resolveIndirectChain(value)
	if !resolvedOK {
		return nil, false, fmt.Errorf("playa: property resource %q could not be resolved", name)
	}
	resolvedDict, ok := resolvedValue.(Dict)
	if !ok {
		return nil, false, nil
	}
	return resolveTagProperties(it.d, resolvedDict), true, nil
}

func (it *contentOpIterator) formFrame(parent contentFrame, op ContentOp) (contentFrame, bool, error) {
	if len(op.operandsValue()) == 0 {
		return contentFrame{}, false, nil
	}
	name, ok := op.operandsValue()[0].(Name)
	if !ok {
		return contentFrame{}, false, nil
	}
	resourcesValue, resourcesResolved := it.d.resolveIndirectChain(parent.ctx[Name("Resources")])
	if !resourcesResolved && parent.ctx[Name("Resources")] != nil {
		return contentFrame{}, false, fmt.Errorf("playa: Resources could not be resolved")
	}
	resources, _ := resourcesValue.(Dict)
	xobjectsRaw, present := resources[Name("XObject")]
	if !present {
		return contentFrame{}, false, nil
	}
	xobjectsValue, xobjectsResolved := it.d.resolveIndirectChain(xobjectsRaw)
	if !xobjectsResolved {
		return contentFrame{}, false, fmt.Errorf("playa: XObject resources could not be resolved")
	}
	xobjects, ok := xobjectsValue.(Dict)
	if !ok {
		return contentFrame{}, false, fmt.Errorf("playa: XObject resources are not a dictionary")
	}
	value, ok := xobjects[name]
	if !ok {
		return contentFrame{}, false, nil
	}
	streamValue, streamResolved := it.d.resolveIndirectChain(value)
	if !streamResolved {
		return contentFrame{}, false, fmt.Errorf("playa: XObject resource %q could not be resolved", name)
	}
	stream, ok := streamValue.(Stream)
	if !ok {
		return contentFrame{}, false, fmt.Errorf("playa: XObject resource %q is not a stream", name)
	}
	streamDict := stream.DictBorrowed()
	subtypeValue, subtypeResolved := it.d.resolveIndirectChain(streamDict[Name("Subtype")])
	if streamDict[Name("Subtype")] != nil && !subtypeResolved {
		return contentFrame{}, false, fmt.Errorf("playa: XObject resource %q subtype could not be resolved", name)
	}
	if subtypeValue != Name("Form") {
		return contentFrame{}, false, nil
	}
	ref, hasRef := value.(Ref)
	key := it.d.formCycleKey(value)
	if it.seen[key] {
		return contentFrame{}, false, nil
	}
	var cachedResources Dict
	cached := false
	if hasRef {
		if err, ok := it.d.cachedXObjectResourceError(ref); ok {
			return contentFrame{}, false, err
		}
		cachedResources, cached = it.d.cachedXObjectResource(ref)
	}
	if !cached {
		if err := it.d.validatePropertiesResources(streamDict, 0); err != nil {
			return contentFrame{}, false, err
		}
	}
	it.seen[key] = true
	ctx := parent.ctx
	if cached {
		ctx = cloneDict(parent.ctx)
		ctx[Name("Resources")] = cachedResources
	} else {
		resourcesValue, resourcesResolved := it.d.resolveIndirectChain(streamDict[Name("Resources")])
		if streamDict[Name("Resources")] != nil && !resourcesResolved {
			return contentFrame{}, false, fmt.Errorf("playa: Form XObject %q resources could not be resolved", name)
		}
		if resources, ok := resourcesValue.(Dict); ok {
			ctx = cloneDict(parent.ctx)
			ctx[Name("Resources")] = resources
		}
	}
	matrixValue, _ := it.d.resolveIndirectChain(streamDict[Name("Matrix")])
	matrix, _ := matrixValue.(Array)
	parentKey, hasParentKey := formParentKey(it.d, stream)
	return contentFrame{streams: []Object{value}, ctx: ctx, prefix: parent.prefix + string(name) + "/", matrix: matrix, ref: ref, hasRef: hasRef, formKey: key, hasForm: true, parentKey: parentKey, hasParentKey: hasParentKey}, true, nil
}

func inlineImageFromParser(parser *ObjectParser, data []byte, offset int) (ContentOp, error) {
	params, err := inlineImageParamsFromParser(parser)
	if err != nil {
		return ContentOp{}, err
	}
	start := inlineImageDataStart(data, parser.Lexer().Pos(), params)
	end, resume := inlineImageEnd(data, start, params)
	if end < 0 {
		return ContentOp{}, fmt.Errorf("playa: unterminated inline image")
	}
	var image []byte
	if end > start {
		image = cloneObjectBytes(data[start:end])
	}
	parser.Lexer().SetPos(resume)
	return newContentOpBorrowed("BI", []Object{newStreamWithDecodeDict(params, image, effectiveInlineImageParams(params))}, offset), nil
}

// skipInlineImageFromParser consumes a cached inline image without copying
// its bytes. Cache hits still have to advance the lexer past EI before the
// next content operation is read.
func skipInlineImageFromParser(parser *ObjectParser, data []byte) error {
	params, err := inlineImageParamsFromParser(parser)
	if err != nil {
		return err
	}
	start := inlineImageDataStart(data, parser.Lexer().Pos(), params)
	end, resume := inlineImageEnd(data, start, params)
	if end < 0 {
		return fmt.Errorf("playa: unterminated inline image")
	}
	parser.Lexer().SetPos(resume)
	return nil
}

func inlineImageParamsFromParser(parser *ObjectParser) (Dict, error) {
	values := make([]Object, 0)
	for {
		token, err := parser.NextToken()
		if err != nil {
			return nil, err
		}
		if token.Kind() == TokenKeyword && token.Text() == "ID" {
			break
		}
		value, err := parser.ParseToken(token)
		if err != nil {
			return nil, err
		}
		if contentObjectHasIndirectReference(value) {
			return nil, errContentStreamIndirectReference
		}
		values = append(values, value)
	}
	params := Dict{}
	for index := 0; index+1 < len(values); index += 2 {
		value := values[index+1]
		if value == nil {
			continue
		}
		if _, null := value.(Null); null {
			continue
		}
		name, ok := values[index].(Name)
		if !ok {
			return nil, fmt.Errorf("playa: invalid inline image dictionary")
		}
		params[name] = value
	}
	return params, nil
}

func effectiveInlineImageParams(params Dict) Dict {
	effective := cloneDict(params)
	for _, names := range [][2]string{
		{"BPC", "BitsPerComponent"},
		{"CS", "ColorSpace"},
		{"D", "Decode"},
		{"DP", "DecodeParms"},
		{"F", "Filter"},
		{"H", "Height"},
		{"IM", "ImageMask"},
		{"I", "Interpolate"},
		{"L", "Length"},
		{"W", "Width"},
	} {
		if value, present := effective[Name(names[0])]; present {
			effective[Name(names[1])] = value
		}
	}
	return effective
}
