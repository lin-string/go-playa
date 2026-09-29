package document

import (
	"fmt"
	"sync"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
)

// FilterAll drives text on the caller goroutine and fans the same operator
// stream into three sparse stateful interpreters. Auxiliary workers acknowledge
// only operators that may emit an object, preserving source order and early
// stop behavior without reparsing content prefixes.
type allMessageKind uint8

const (
	allReady allMessageKind = iota
	allEvent
	allDone
	allError
)

type allMessage struct {
	kind   allMessageKind
	object ContentObject
	err    error
}

type allInput struct {
	op           ContentOp
	operands     [6]Object
	operandCount uint8
	inline       bool
	barrier      bool
}

type allWorkerState struct {
	done chan struct{}
	err  error
}

func yieldAllContentObject(yield func(ContentObject, error) bool, consumerYielding *bool, object ContentObject, err error) bool {
	*consumerYielding = true
	accepted := yield(object, err)
	*consumerYielding = false
	return accepted
}

func runAllTextInterpreter(consumerYielding *bool, run func()) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if *consumerYielding {
				panic(recovered)
			}
			err = recoveredPanicError("content interpreter", recovered)
		}
	}()
	run()
	return nil
}

func sendAllInterpreterInput(input chan<- allInput, done <-chan struct{}, message allInput) bool {
	// Keep the ordinary buffered path equivalent to a channel send. The done
	// select is needed only under backpressure, where a stopped worker could
	// otherwise leave the coordinator blocked on a full FIFO.
	select {
	case input <- message:
		return true
	default:
	}
	select {
	case input <- message:
		return true
	case <-done:
		return false
	}
}

func runAllInterpreterWorker(
	input <-chan allInput,
	messages chan<- allMessage,
	state *allWorkerState,
	run func(func() (ContentOp, bool), func(ContentObject)) error,
) {
	var runErr error
	defer func() {
		if recovered := recover(); recovered != nil {
			runErr = recoveredPanicError("content interpreter", recovered)
		}
		state.err = runErr
		close(state.done)
		if runErr != nil {
			messages <- allMessage{kind: allError, err: runErr}
		}
		messages <- allMessage{kind: allDone}
	}()
	current := ContentOp{}
	var currentOperands [6]Object
	acknowledge := false
	next := func() (ContentOp, bool) {
		if acknowledge {
			messages <- allMessage{kind: allReady}
		}
		message, ok := <-input
		if !ok {
			return ContentOp{}, false
		}
		current = message.operation(&currentOperands)
		acknowledge = message.barrier
		return current, true
	}
	runErr = run(next, func(object ContentObject) {
		messages <- allMessage{kind: allEvent, object: object}
	})
}

func allAsyncScalar(value Object) bool {
	switch value.(type) {
	case nil, Null, Bool, Number, Name, Keyword, Ref:
		return true
	default:
		return false
	}
}

func newAllInput(op ContentOp, barrier bool) allInput {
	input := allInput{op: op, barrier: barrier}
	if barrier {
		return input
	}
	operands := op.operandsValue()
	if len(operands) <= len(input.operands) {
		inline := true
		for _, operand := range operands {
			if !allAsyncScalar(operand) {
				inline = false
				break
			}
		}
		if inline {
			input.inline = true
			input.operandCount = uint8(len(operands))
			copy(input.operands[:], operands)
			return input
		}
	}
	input.op.data = input.op.data.Finalize()
	return input
}

func (input *allInput) operation(operands *[6]Object) ContentOp {
	op := input.op
	if input.inline {
		copy(operands[:], input.operands[:input.operandCount])
		op.setOperands(operands[:input.operandCount])
	}
	return op
}

// waitForAllInterpreterReady drains one readiness acknowledgment from each
// active worker. A worker may report a recovered panic before becoming ready,
// so the coordinator must handle allError here instead of waiting forever.
func waitForAllInterpreterReady(messages []chan allMessage, active []bool) (error, bool) {
	for index := range messages {
		if !active[index] {
			continue
		}
		ready := false
		for !ready {
			message := <-messages[index]
			switch message.kind {
			case allReady:
				ready = true
			case allError:
				return message.err, false
			case allDone:
				return nil, true
			}
		}
	}
	return nil, false
}

// allGraphicsStateOperator reports the operators consumed by
// applyGraphicsState, applyExternalGraphicsState, or applyResourceColorSpace.
// Routing these to every stateful interpreter preserves its public graphics
// snapshots without broadcasting unrelated text, path, image, and tag work.
func allGraphicsStateOperator(operator string) bool {
	switch operator {
	case "cm", "w", "Tf", "J", "j", "M", "d",
		"RG", "rg", "G", "g", "K", "k", "CS", "cs", "SC", "SCN", "sc", "scn",
		"Tr", "Ts", "Tc", "Tw", "Tz", "TL", "TD", "CA", "ca", "BM", "SA", "AIS",
		"TK", "OP", "op", "OPM", "HT", "SMask", "UseBlackPtComp", "ri", "i", "gs":
		return true
	default:
		return false
	}
}

func allWorkerAccepts(worker int, op ContentOp) bool {
	operator := op.operatorValue()
	if allGraphicsStateOperator(operator) {
		return true
	}
	switch operator {
	case "q", "Q", "BMC", "BDC", "EMC":
		return true
	}
	switch worker {
	case 1:
		return pathOperatorPreservesContext(operator)
	case 2:
		return operator == "BI" || operator == "Do"
	case 3:
		return operator == "MP" || operator == "DP"
	}
	return false
}

func allWorkerBarrier(worker int, operator string) bool {
	switch worker {
	case 1:
		return isPathPaintOperator(operator)
	case 2:
		return operator == "BI" || operator == "Do"
	case 3:
		return operator == "MP" || operator == "DP"
	default:
		return false
	}
}

func (p Page) streamingAllContentObjects(d *Document, flatten bool, opts contentconfig.Options, yield func(ContentObject, error) bool) {
	p.streamingContentObjects(d, flatten, opts, true, false, yield)
}

func (p Page) streamingLayoutContentObjects(d *Document, yield func(ContentObject, error) bool) {
	p.streamingContentObjects(d, false, contentconfig.Options{Filter: FilterAll}, false, true, yield)
}

func (p Page) streamingContentObjects(d *Document, flatten bool, opts contentconfig.Options, includeTags, layoutTextOnly bool, yield func(ContentObject, error) bool) {
	fonts := map[string]*Font{}
	allowed := make(map[string]struct{}, len(opts.RestrictOps))
	for _, name := range opts.RestrictOps {
		allowed[name] = struct{}{}
	}
	fontLookup := func(name string) *Font {
		if font, ok := fonts[name]; ok {
			return font
		}
		font := d.pageFontByName(p, name)
		fonts[name] = font
		return font
	}

	// Text is the common case and drives the operator stream directly. Keeping
	// it on the coordinator goroutine avoids a channel round trip for nearly
	// every text operator while the three sparse interpreters retain their
	// independent state machines.
	workerCount := 2
	if includeTags {
		workerCount++
	}
	inputs := make([]chan allInput, workerCount)
	messages := make([]chan allMessage, workerCount)
	workerStates := make([]allWorkerState, workerCount)
	var stopOnce sync.Once
	stopWorkers := func() {
		stopOnce.Do(func() {
			for _, input := range inputs {
				close(input)
			}
		})
	}
	var workers sync.WaitGroup
	defer func() {
		stopWorkers()
		workers.Wait()
	}()
	for i := range inputs {
		// State-only operators can run ahead within this bounded FIFO. Emitting
		// operators are barriers, so their events are drained before parsing the
		// next operator.
		inputs[i] = make(chan allInput, 64)
		messages[i] = make(chan allMessage, 8)
		workerStates[i].done = make(chan struct{})
	}
	start := func(index int, run func(func() (ContentOp, bool), func(ContentObject)) error) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			runAllInterpreterWorker(inputs[index], messages[index], &workerStates[index], run)
		}()
	}
	start(0, func(next func() (ContentOp, bool), emit func(ContentObject)) error {
		interpretPathsNext(d, next, allowed, func(path PathObject) bool {
			path.setPage(p.ref)
			emit(ContentObject{kind: ContentPath, path: &path})
			return true
		})
		return nil
	})
	start(1, func(next func() (ContentOp, bool), emit func(ContentObject)) error {
		var interpretErr error
		interpretImagesNext(d, p.ref, next, allowed, func(image ImageObject) bool {
			emit(ContentObject{kind: ContentImage, image: &image})
			return true
		}, func(err error) {
			interpretErr = err
		})
		return interpretErr
	})
	if includeTags {
		start(2, func(next func() (ContentOp, bool), emit func(ContentObject)) error {
			var properties Dict
			propertiesReady := false
			propertiesFor := func() (Dict, error) {
				if !propertiesReady {
					var err error
					properties, err = d.pagePropertiesChecked(p)
					if err != nil {
						return nil, err
					}
					propertiesReady = true
				}
				return properties, nil
			}
			return interpretTagsNext(d, propertiesFor, p.ref, next, allowed, func(tag TagObject) bool {
				emit(ContentObject{kind: ContentTag, tag: &tag})
				return true
			})
		})
	}

	active := make([]bool, len(messages))
	iterator := newContentOpIterator(d, p, flatten)
	marked := []markedContentFrame{}
	markedForms := [][]markedContentFrame{}
	gstate := newGraphicsState()
	savedGStates := []graphicsState{}
	pending := make([]ContentObject, 0, 4)
	var iteratorErr error
	var workerErr error
	var textErr error
	consumerStopped := false
	consumerYielding := false
	emit := yield
	flushPending := func() bool {
		for _, object := range pending {
			if !emit(object, nil) {
				consumerStopped = true
				pending = pending[:0]
				return false
			}
		}
		pending = pending[:0]
		return true
	}
	sendWorker := func(index int, message allInput) bool {
		if sendAllInterpreterInput(inputs[index], workerStates[index].done, message) {
			return true
		}
		workerErr = workerStates[index].err
		if workerErr == nil {
			workerErr = fmt.Errorf("playa: content interpreter stopped before the operator stream ended")
		}
		return false
	}
	next := func() (ContentOp, bool) {
		// Text events for the previous operator have already been yielded when
		// the text interpreter asks for another operator. Flushing sparse-worker
		// events here preserves text/path/image/tag order without making the
		// operator stream eager.
		if consumerStopped || iteratorErr != nil || workerErr != nil || textErr != nil || !flushPending() {
			return ContentOp{}, false
		}
		op, err, ok := iterator.next()
		if err != nil {
			iteratorErr = err
			return ContentOp{}, false
		}
		if !ok {
			return ContentOp{}, false
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
			message := newAllInput(op, false)
			for index := range inputs {
				active[index] = false
				if !sendWorker(index, message) {
					return ContentOp{}, false
				}
			}
		} else {
			applyMarkedContent(&marked, op)
			switch op.operatorValue() {
			case "q":
				savedGStates = append(savedGStates, gstate.Clone())
			case "Q":
				if len(savedGStates) > 0 {
					gstate = savedGStates[len(savedGStates)-1]
					savedGStates = savedGStates[:len(savedGStates)-1]
				}
			default:
				applyGraphicsState(&gstate, op)
				applyExternalGraphicsState(d, &gstate, op)
				applyResourceColorSpace(d, &gstate, op)
			}
			var asyncInput allInput
			asyncReady := false
			for index := range active {
				// Sparse worker zero is the path interpreter, corresponding to
				// worker one in allWorkerAccepts; image and tag follow it.
				worker := index + 1
				accepts := allWorkerAccepts(worker, op)
				active[index] = accepts && allWorkerBarrier(worker, op.operatorValue())
				if accepts {
					if active[index] {
						if !sendWorker(index, newAllInput(op, true)) {
							return ContentOp{}, false
						}
						continue
					}
					if !asyncReady {
						asyncInput = newAllInput(op, false)
						asyncReady = true
					}
					if !sendWorker(index, asyncInput) {
						return ContentOp{}, false
					}
				}
			}
		}
		for index := range messages {
			if !active[index] {
				continue
			}
			for {
				message := <-messages[index]
				switch message.kind {
				case allEvent:
					pending = append(pending, message.object)
				case allReady:
				case allDone:
					workerErr = fmt.Errorf("playa: content interpreter stopped before the operator stream ended")
					return ContentOp{}, false
				case allError:
					workerErr = message.err
					return ContentOp{}, false
				}
				if message.kind == allReady {
					break
				}
			}
		}
		if !flatten && op.operatorValue() == "Do" {
			object, present, err := directFormObject(d, p, op, gstate)
			if err != nil {
				workerErr = err
				return ContentOp{}, false
			}
			if present {
				context := currentMarkedContent(marked)
				object.setMarkedContent(context, marked)
				pending = append(pending, ContentObject{kind: ContentXObject, xobject: &object})
			}
		}
		return op, true
	}
	emit = func(object ContentObject, err error) bool {
		return yieldAllContentObject(yield, &consumerYielding, object, err)
	}
	if err := runAllTextInterpreter(&consumerYielding, func() {
		interpretTextWithFontResolverNextErrorPage(d, p.ref, next, fontLookup, false, layoutTextOnly, allowed, func(text TextObject) bool {
			if consumerStopped || iteratorErr != nil || workerErr != nil || textErr != nil {
				return false
			}
			if !emit(ContentObject{kind: ContentText, text: &text}, nil) {
				consumerStopped = true
				return false
			}
			return true
		}, func(err error) bool {
			textErr = err
			return false
		})
	}); err != nil {
		textErr = err
	}
	emit = yield
	stopWorkers()
	workers.Wait()
	if consumerStopped {
		return
	}
	if workerErr == nil {
		for index := range workerStates {
			if workerStates[index].err != nil {
				workerErr = workerStates[index].err
				break
			}
		}
	}
	if iteratorErr != nil {
		yield(ContentObject{}, iteratorErr)
		return
	}
	if workerErr != nil {
		yield(ContentObject{}, workerErr)
		return
	}
	if textErr != nil {
		yield(ContentObject{}, textErr)
		return
	}
	flushPending()
}

func directFormObject(d *Document, p Page, op ContentOp, gstate graphicsState) (XObjectObject, bool, error) {
	if len(op.operandsValue()) == 0 || op.resources == nil {
		return XObjectObject{}, false, nil
	}
	name, ok := op.operandsValue()[0].(Name)
	if !ok {
		return XObjectObject{}, false, nil
	}
	rawXObjects, present := op.resources[Name("XObject")]
	if !present {
		return XObjectObject{}, false, nil
	}
	xobjectsValue, resolved := d.resolveIndirectChain(rawXObjects)
	if !resolved {
		return XObjectObject{}, false, fmt.Errorf("playa: XObject resources could not be resolved")
	}
	xobjects, ok := xobjectsValue.(Dict)
	if !ok {
		return XObjectObject{}, false, fmt.Errorf("playa: XObject resources are not a dictionary")
	}
	value := xobjects[name]
	if value == nil {
		return XObjectObject{}, false, nil
	}
	streamValue, resolved := d.resolveIndirectChain(value)
	if !resolved {
		return XObjectObject{}, false, fmt.Errorf("playa: XObject %q could not be resolved", name)
	}
	stream, ok := streamValue.(Stream)
	if !ok {
		return XObjectObject{}, false, nil
	}
	streamDict := stream.DictBorrowed()
	rawSubtype, present := streamDict[Name("Subtype")]
	if present {
		subtypeValue, resolved := d.resolveIndirectChain(rawSubtype)
		if !resolved {
			return XObjectObject{}, false, fmt.Errorf("playa: XObject %q subtype could not be resolved", name)
		}
		if subtypeValue != Name("Form") {
			return XObjectObject{}, false, nil
		}
	} else {
		return XObjectObject{}, false, nil
	}
	parentKey, hasParentKey := formParentKey(d, stream)
	matrix := resolveFormMatrix(d, streamDict[Name("Matrix")])
	childState := formGraphicsState(d, stream, gstate.Clone())
	object := newXObjectObject(contentdata.XObjectSpec{Name: string(name), Page: p.ref, HasPage: p.ref != (Ref{}), Stream: stream, Matrix: matrix, BBox: formBBoxForPage(d, p, stream, gstate.ctm.Mul(matrix)), Group: formGroup(d, stream), ParentKey: parentKey, HasParentKey: hasParentKey, Path: string(name), GState: childState.publicValue()})
	if ref, ok := value.(Ref); ok {
		object.updateData(func(spec *contentdata.XObjectSpec) { spec.Ref = ref })
	}
	rawResources, present := streamDict[Name("Resources")]
	if !present {
		object.updateData(func(spec *contentdata.XObjectSpec) {
			spec.Resources, spec.ResourceContext, spec.HasDeclaredResources = op.resources, op.resources, true
		})
		return object, true, nil
	}
	if ref, ok := value.(Ref); ok {
		if err, cached := d.cachedXObjectResourceError(ref); cached {
			return XObjectObject{}, false, err
		}
		if resources, cached := d.cachedXObjectResource(ref); cached {
			object.updateData(func(spec *contentdata.XObjectSpec) {
				spec.Resources, spec.DeclaredResources, spec.ResourceContext = resources, resources, resources
				spec.HasDeclaredResources = true
			})
			return object, true, nil
		}
	}
	resourcesValue, resolved := d.resolveIndirectChain(rawResources)
	if !resolved {
		return XObjectObject{}, false, fmt.Errorf("playa: XObject %q Resources could not be resolved", name)
	}
	if resources, ok := resourcesValue.(Dict); ok {
		object.updateData(func(spec *contentdata.XObjectSpec) {
			spec.Resources, spec.DeclaredResources, spec.ResourceContext = resources, resources, resources
			spec.HasDeclaredResources = true
		})
	} else {
		return XObjectObject{}, false, fmt.Errorf("playa: XObject %q Resources is not a dictionary", name)
	}
	return object, true, nil
}
