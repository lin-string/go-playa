package document

import (
	"encoding/json"
	"fmt"
	"iter"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/geometry"
)

const actionScriptCacheLimit = 32 << 20
const actionCacheLimit = 32 << 20
const actionErrorCacheLimit = 64 << 10
const destinationErrorCacheLimit = 64 << 10

// Destination is the normalized target of a PDF destination array. Params
// retains the view arguments (for example FitH's top coordinate) without
// forcing callers to understand every PDF view variant.
type Destination struct {
	data documentdata.Destination
}

func newDestination(page Ref, hasPage bool, pageIndex int, hasPageIndex bool, view string, params []Object) *Destination {
	return &Destination{data: documentdata.NewDestination(page, hasPage, pageIndex, hasPageIndex, view, params)}
}

func (t Destination) PageRef() (Ref, bool)   { return t.data.PageRef() }
func (t Destination) PageIndex() (int, bool) { return t.data.PageIndex() }
func (t Destination) View() string           { return t.data.View() }
func (t Destination) NCoords() int           { return t.data.NCoords() }

// ParamsCopy returns independent destination view parameters.
func (t Destination) ParamsCopy() []Object {
	return t.data.ParamsCopy()
}

func (t Destination) MarshalJSON() ([]byte, error) {
	return t.data.MarshalJSON()
}

// Top returns the destination's device-space top coordinate when specified.
func (t Destination) Top(d *Document) (float64, bool) {
	top, err := t.TopWithError(d)
	return top, err == nil
}

// TopWithError returns the destination's device-space top coordinate and
// reports malformed page geometry or explicit view parameters.
func (t Destination) TopWithError(d *Document) (float64, error) {
	_, matrix, err := t.destinationTransformWithError(d)
	if err != nil {
		return 0, err
	}
	if t.View() == "FitR" {
		position, err := t.PosWithError(d)
		if err != nil {
			return 0, err
		}
		_, present, err := t.resolveDestinationParam(d, 3)
		if err != nil {
			return 0, err
		}
		if !present {
			return 0, fmt.Errorf("playa: destination top parameter is not specified")
		}
		return position[1], nil
	}
	index := -1
	switch t.View() {
	case "XYZ":
		index = 1
	case "FitH", "FitBH":
		index = 0
	case "FitR":
		index = 3
	}
	value, ok, err := t.resolveDestinationParam(d, index)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("playa: destination top parameter is not specified")
	}
	number, ok := finiteNumberValue(value)
	if !ok {
		return 0, fmt.Errorf("playa: destination top parameter is invalid")
	}
	_, transformed, ok := matrix.PointFinite(0, number)
	if !ok {
		return 0, fmt.Errorf("playa: destination top coordinate is invalid")
	}
	return transformed, nil
}

// Left returns the destination's device-space left coordinate when specified.
func (t Destination) Left(d *Document) (float64, bool) {
	left, err := t.LeftWithError(d)
	return left, err == nil
}

// LeftWithError returns the destination's device-space left coordinate and
// reports malformed page geometry or explicit view parameters.
func (t Destination) LeftWithError(d *Document) (float64, error) {
	page, matrix, err := t.destinationTransformWithError(d)
	if err != nil {
		return 0, err
	}
	if t.View() == "FitR" {
		position, err := t.PosWithError(d)
		if err != nil {
			return 0, err
		}
		_, present, err := t.resolveDestinationParam(d, 0)
		if err != nil {
			return 0, err
		}
		if !present {
			return 0, fmt.Errorf("playa: destination left parameter is not specified")
		}
		return position[0], nil
	}
	index := -1
	switch t.View() {
	case "XYZ", "FitV", "FitBV":
		index = 0
	}
	value, ok, err := t.resolveDestinationParam(d, index)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("playa: destination left parameter is not specified")
	}
	number, ok := finiteNumberValue(value)
	if !ok {
		return 0, fmt.Errorf("playa: destination left parameter is invalid")
	}
	y := 0.0
	if t.View() == "FitV" || t.View() == "FitBV" {
		var heightErr error
		_, y, heightErr = page.SizeWithError(d)
		if heightErr != nil {
			return 0, heightErr
		}
	}
	transformed, _, ok := matrix.PointFinite(number, y)
	if !ok {
		return 0, fmt.Errorf("playa: destination left coordinate is invalid")
	}
	return transformed, nil
}

// Pos returns the destination's device-space anchor point. Unspecified
// coordinates default to the corresponding page edge.
func (t Destination) Pos(d *Document) ([2]float64, bool) {
	position, err := t.PosWithError(d)
	return position, err == nil
}

// PosWithError returns the destination's device-space anchor point and
// reports unresolved page or geometry values instead of collapsing them into
// an unspecified position.
func (t Destination) PosWithError(d *Document) ([2]float64, error) {
	page, matrix, err := t.destinationTransformWithError(d)
	if err != nil {
		return [2]float64{}, err
	}
	mediaBox, err := page.MediaBoxWithError(d)
	if err != nil {
		return [2]float64{}, err
	}
	box := [4]float64{0, 0, mediaBox[2] - mediaBox[0], mediaBox[3] - mediaBox[1]}
	x, y := box[0], box[3]
	if value, valid := t.destinationParam(d, 0); valid && (t.View() == "XYZ" || t.View() == "FitV" || t.View() == "FitBV" || t.View() == "FitR") {
		resolved, resolvedOK := d.resolveIndirectChain(value)
		if !resolvedOK {
			return [2]float64{}, fmt.Errorf("playa: destination x parameter could not be resolved")
		}
		if number, ok := finiteNumberValue(resolved); ok {
			x = number
		}
	}
	if value, valid := t.destinationParam(d, 1); valid && t.View() == "XYZ" {
		resolved, resolvedOK := d.resolveIndirectChain(value)
		if !resolvedOK {
			return [2]float64{}, fmt.Errorf("playa: destination y parameter could not be resolved")
		}
		if number, ok := finiteNumberValue(resolved); ok {
			y = number
		}
	}
	if value, valid := t.destinationParam(d, 0); valid && (t.View() == "FitH" || t.View() == "FitBH") {
		resolved, resolvedOK := d.resolveIndirectChain(value)
		if !resolvedOK {
			return [2]float64{}, fmt.Errorf("playa: destination top parameter could not be resolved")
		}
		if number, ok := finiteNumberValue(resolved); ok {
			y = number
		}
	}
	if value, valid := t.destinationParam(d, 3); valid && t.View() == "FitR" {
		resolved, resolvedOK := d.resolveIndirectChain(value)
		if !resolvedOK {
			return [2]float64{}, fmt.Errorf("playa: destination bottom parameter could not be resolved")
		}
		if number, ok := finiteNumberValue(resolved); ok {
			y = number
		}
	}
	px, py, ok := matrix.PointFinite(x, y)
	if !ok {
		return [2]float64{}, fmt.Errorf("playa: destination position is invalid")
	}
	return [2]float64{px, py}, nil
}

// BBox returns the device-space rectangle from the destination anchor to the
// opposite page edge, or the explicit rectangle for a FitR destination.
func (t Destination) BBox(d *Document) ([4]float64, bool) {
	bbox, err := t.BBoxWithError(d)
	return bbox, err == nil
}

// BBoxWithError returns the device-space destination rectangle and reports
// unresolved page geometry or explicit destination parameters.
func (t Destination) BBoxWithError(d *Document) ([4]float64, error) {
	page, matrix, err := t.destinationTransformWithError(d)
	if err != nil {
		return [4]float64{}, err
	}
	mediaBox, err := page.MediaBoxWithError(d)
	if err != nil {
		return [4]float64{}, err
	}
	box := [4]float64{0, 0, mediaBox[2] - mediaBox[0], mediaBox[3] - mediaBox[1]}
	if t.View() == "FitR" {
		for index, target := range []*float64{&box[0], &box[1], &box[2], &box[3]} {
			if value, valid := t.destinationParam(d, index); valid {
				resolved, resolvedOK := d.resolveIndirectChain(value)
				if !resolvedOK {
					return [4]float64{}, fmt.Errorf("playa: destination rectangle parameter could not be resolved")
				}
				if number, ok := finiteNumberValue(resolved); ok {
					*target = number
				}
			}
		}
	} else {
		anchor, err := t.PosWithError(d)
		if err != nil {
			return [4]float64{}, err
		}
		right, bottom, ok := matrix.PointFinite(box[2], box[1])
		if !ok {
			return [4]float64{}, fmt.Errorf("playa: destination bbox is invalid")
		}
		return normalizeBBox([4]float64{anchor[0], anchor[1], right, bottom}), nil
	}
	points := [4][2]float64{{box[0], box[1]}, {box[2], box[1]}, {box[2], box[3]}, {box[0], box[3]}}
	firstX, firstY, ok := matrix.PointFinite(points[0][0], points[0][1])
	if !ok {
		return [4]float64{}, fmt.Errorf("playa: destination bbox is invalid")
	}
	out := [4]float64{firstX, firstY, firstX, firstY}
	for _, point := range points[1:] {
		x, y, ok := matrix.PointFinite(point[0], point[1])
		if !ok {
			return [4]float64{}, fmt.Errorf("playa: destination bbox is invalid")
		}
		if x < out[0] {
			out[0] = x
		}
		if y < out[1] {
			out[1] = y
		}
		if x > out[2] {
			out[2] = x
		}
		if y > out[3] {
			out[3] = y
		}
	}
	return out, nil
}

func normalizeBBox(box [4]float64) [4]float64 {
	if box[0] > box[2] {
		box[0], box[2] = box[2], box[0]
	}
	if box[1] > box[3] {
		box[1], box[3] = box[3], box[1]
	}
	return box
}

// Zoom returns the XYZ zoom factor. Zero and null mean unchanged.
func (t Destination) Zoom(d *Document) (float64, bool) {
	zoom, err := t.ZoomWithError(d)
	return zoom, err == nil && zoom != 0
}

// ZoomWithError returns an explicit XYZ zoom and reports malformed values.
// A missing, null, or zero zoom remains the unchanged-zoom case.
func (t Destination) ZoomWithError(d *Document) (float64, error) {
	if t.View() != "XYZ" {
		return 0, fmt.Errorf("playa: destination zoom is not applicable")
	}
	value, present, err := t.resolveDestinationParam(d, 2)
	if err != nil {
		return 0, err
	}
	if !present {
		return 0, fmt.Errorf("playa: destination zoom is not specified")
	}
	number, ok := finiteNumberValue(value)
	if !ok {
		return 0, fmt.Errorf("playa: destination zoom is invalid")
	}
	return number, nil
}

func (t Destination) destinationTransformWithError(d *Document) (Page, geometry.Matrix, error) {
	if d == nil {
		return Page{}, geometry.Matrix{}, errNilDocument
	}
	page, err := t.PageObject(d)
	if err != nil {
		return Page{}, geometry.Matrix{}, err
	}
	matrix, err := page.MatrixInWithError(d, page.coordinateSpace(d))
	if err != nil {
		return Page{}, geometry.Matrix{}, err
	}
	return page, matrix, nil
}

func (t Destination) destinationParam(d *Document, index int) (Object, bool) {
	value, present, err := t.resolveDestinationParam(d, index)
	return value, present && err == nil
}

func (t Destination) resolveDestinationParam(d *Document, index int) (Object, bool, error) {
	if d == nil || index < 0 || index >= t.data.ParamsCount() {
		return nil, false, nil
	}
	param, ok := t.data.ParamCopy(index)
	if !ok {
		return nil, false, nil
	}
	value, resolved := d.resolveIndirectChain(param)
	if !resolved {
		return nil, false, fmt.Errorf("playa: destination parameter %d could not be resolved", index)
	}
	if _, null := value.(Null); null {
		return nil, false, nil
	}
	return value, true, nil
}

// PageObject resolves the destination's page reference or zero-based page
// index to a document page.
func (t Destination) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if page, ok := t.PageRef(); ok {
		return d.PageByRef(page)
	}
	if pageIndex, ok := t.PageIndex(); ok {
		return d.PageAt(pageIndex)
	}
	return Page{}, ErrPageNotFound
}

// Action is the normalized form of a PDF action dictionary. RawCopy exposes the
// original dictionary for action-specific fields not represented here.
type Action struct {
	data           documentdata.Action
	destination    *Destination
	scriptErr      error
	next           []*Action
	nextRaw        Object
	nextErr        error
	nextItems      []Object
	nextSeen       map[Ref]bool
	nextCursor     int
	nextLazyDone   bool
	nextItemsReady bool
	document       *Document
	actionRef      Ref
	hasActionRef   bool
	nextReady      bool
}

func newAction(kind string) *Action {
	return &Action{data: documentdata.NewAction(kind, "", "", "", "", nil)}
}

func newActionWithRaw(raw Dict) *Action {
	return &Action{data: documentdata.NewAction("", "", "", "", "", raw)}
}

func (a Action) Kind() string   { return a.data.Kind() }
func (a Action) URI() string    { return a.data.URI() }
func (a Action) File() string   { return a.data.File() }
func (a Action) Name() string   { return a.data.Name() }
func (a Action) Script() string { return a.data.Script() }

// ScriptWithError returns the JavaScript source and reports deferred stream
// resolution or filter-decoding failures.
func (a *Action) ScriptWithError() (string, error) {
	materialized, err := a.materializedErr()
	if err != nil {
		return "", err
	}
	if materialized == nil {
		return "", nil
	}
	return materialized.Script(), nil
}

// RawCopy returns an independent copy of the original action dictionary.
func (a Action) RawCopy() Dict { return a.data.RawCopy() }

// DestinationCopy returns an independent normalized action destination.
func (a Action) DestinationCopy() *Destination { return cloneDestination(a.destination) }

// NextCopy returns independent actions in the action chain.
func (a *Action) NextCopy() ([]*Action, error) {
	if a.document != nil {
		a.document.actionMu.Lock()
		defer a.document.actionMu.Unlock()
	}
	materialized := a.materializedLocked()
	if materialized == nil {
		return nil, nil
	}
	if materialized.nextErr != nil {
		return nil, materialized.nextErr
	}
	if materialized == nil || len(materialized.next) == 0 {
		return nil, nil
	}
	next := make([]*Action, len(materialized.next))
	for i, action := range materialized.next {
		clone, err := finalizeActionStrict(action)
		if err != nil {
			return nil, err
		}
		next[i] = clone
	}
	return next, nil
}

// NextSeq lazily yields borrowed actions in the action chain. Call Finalize
// on an item when an independent snapshot is required.
func (a *Action) NextSeq() iter.Seq2[*Action, error] {
	return func(yield func(*Action, error) bool) {
		cached := 0
		for {
			var ready []*Action
			var err error
			var done bool
			if a.document != nil {
				a.document.actionMu.Lock()
			}
			source := a.nextSourceLocked()
			if source != nil {
				ready = append(ready, source.next[cached:]...)
				cached += len(ready)
				if source.scriptErr != nil {
					err = source.scriptErr
				} else if source.nextErr != nil {
					err = source.nextErr
				} else {
					done = source.nextReady || source.nextLazyDone
				}
			} else {
				done = true
			}
			if a.document != nil {
				a.document.actionMu.Unlock()
			}
			for _, action := range ready {
				if !yield(action, nil) {
					return
				}
			}
			if err != nil {
				yield(nil, err)
				return
			}
			if done {
				return
			}

			if a.document != nil {
				a.document.actionMu.Lock()
			}
			source = a.nextSourceLocked()
			var action *Action
			if source == nil {
				done = true
			} else {
				action, err, done = source.nextLazyNextLocked()
				if action != nil {
					source.next = append(source.next, action)
					cached++
				}
			}
			if a.document != nil {
				a.document.actionMu.Unlock()
			}
			if err != nil {
				yield(nil, err)
				return
			}
			if action != nil && !yield(action, nil) {
				return
			}
			if done {
				return
			}
		}
	}
}

func (a *Action) MarshalJSON() ([]byte, error) {
	materialized, err := a.materializedErr()
	if err != nil {
		return nil, err
	}
	if materialized == nil {
		return json.Marshal((*Action)(nil))
	}
	return json.Marshal(&struct {
		Kind        string
		URI         string
		File        string
		Name        string
		Script      string
		Raw         Dict         `json:"Raw"`
		Next        []*Action    `json:"Next"`
		Destination *Destination `json:"Destination"`
	}{Kind: materialized.Kind(), URI: materialized.URI(), File: materialized.File(), Name: materialized.Name(), Script: materialized.Script(), Raw: materialized.RawCopy(), Next: materialized.next, Destination: materialized.destination})
}

func cloneDestination(destination *Destination) *Destination {
	if destination == nil {
		return nil
	}
	clone := *destination
	clone.data = destination.data.Finalize()
	return &clone
}

// Finalize returns an independent snapshot of this destination.
func (t Destination) Finalize() Destination {
	clone := cloneDestination(&t)
	if clone == nil {
		return Destination{}
	}
	return *clone
}

func cloneAction(action *Action) *Action {
	if action == nil {
		return nil
	}
	clone := *action
	clone.data = action.data.Finalize()
	clone.destination = cloneDestination(action.destination)
	if action.next != nil {
		clone.next = make([]*Action, len(action.next))
		for i, next := range action.next {
			clone.next[i] = cloneAction(next)
		}
	}
	return &clone
}

func (a *Action) materializeNext() {
	if a == nil {
		return
	}
	if a.document != nil {
		a.document.actionMu.Lock()
		defer a.document.actionMu.Unlock()
	}
	a.materializeNextLocked()
}

func (a *Action) materializeNextLocked() {
	if a == nil || a.nextReady {
		return
	}
	if a.document != nil && a.hasActionRef {
		a.document.cacheMu.RLock()
		canonical, ok := a.document.actionCache[a.actionRef]
		a.document.cacheMu.RUnlock()
		if ok && canonical != nil && canonical != a {
			canonical.materializeNextLocked()
			a.next = canonical.next
			a.nextErr = canonical.nextErr
			a.nextReady = canonical.nextReady
			return
		}
	}
	for !a.nextLazyDone {
		action, err, done := a.nextLazyNextLocked()
		if action != nil {
			a.next = append(a.next, action)
		}
		if err != nil || done {
			break
		}
	}
	a.nextReady = true
	if a.hasActionRef && a.document != nil {
		a.document.cacheMu.Lock()
		if a.document.actionCache == nil {
			a.document.actionCache = map[Ref]*Action{}
		}
		a.document.actionCache[a.actionRef] = a
		a.document.cacheMu.Unlock()
	}
}

func (a *Action) nextSourceLocked() *Action {
	if a == nil {
		return nil
	}
	if a.document != nil && a.hasActionRef {
		a.document.cacheMu.RLock()
		canonical := a.document.actionCache[a.actionRef]
		a.document.cacheMu.RUnlock()
		if canonical != nil && canonical != a {
			return canonical
		}
	}
	return a
}

func (a *Action) prepareNextItemsLocked() {
	if a.nextItemsReady {
		return
	}
	a.nextItemsReady = true
	a.nextLazyDone = true
	if a.document == nil || a.nextRaw == nil {
		return
	}
	seen := map[Ref]bool{}
	if a.hasActionRef {
		seen[a.actionRef] = true
	}
	next, resolved := a.document.resolveIndirectChain(a.nextRaw)
	if !resolved {
		a.nextErr = fmt.Errorf("playa: /Next entry could not be resolved")
		return
	}
	if items, ok := next.(Array); ok {
		a.nextItems = append([]Object(nil), items...)
	} else {
		a.nextItems = []Object{a.nextRaw}
	}
	a.nextSeen = seen
	a.nextLazyDone = false
}

func (a *Action) nextLazyNextLocked() (*Action, error, bool) {
	if a == nil {
		return nil, nil, true
	}
	a.prepareNextItemsLocked()
	if a.nextErr != nil {
		a.nextReady = true
		return nil, a.nextErr, true
	}
	if a.nextCursor >= len(a.nextItems) {
		a.nextLazyDone = true
		a.nextReady = true
		return nil, nil, true
	}
	item := a.nextItems[a.nextCursor]
	a.nextCursor++
	action, err := a.document.resolveNextAction(item, a.nextSeen)
	if err != nil {
		a.nextErr = err
		a.nextLazyDone = true
		a.nextReady = true
		return nil, err, true
	}
	if action == nil {
		return nil, nil, false
	}
	return action, nil, false
}

func (a *Action) materializedErr() (*Action, error) {
	if a == nil {
		return nil, nil
	}
	if a.document != nil {
		a.document.actionMu.Lock()
		defer a.document.actionMu.Unlock()
	}
	materialized := a.materializedLocked()
	if materialized == nil {
		return nil, nil
	}
	if materialized.scriptErr != nil {
		return materialized, materialized.scriptErr
	}
	return materialized, materialized.nextErr
}

func (a *Action) materializedLocked() *Action {
	if a.document != nil && a.hasActionRef {
		a.document.cacheMu.RLock()
		canonical, ok := a.document.actionCache[a.actionRef]
		a.document.cacheMu.RUnlock()
		if ok && canonical != nil {
			canonical.materializeNextLocked()
			return canonical
		}
	}
	a.materializeNextLocked()
	return a
}

func (d *Document) resolveNextAction(value Object, seen map[Ref]bool) (*Action, error) {
	if ref, ok := value.(Ref); ok {
		if canonical, canonicalOK := d.finalIndirectRef(value); canonicalOK {
			ref = canonical
		}
		if seen[ref] {
			return nil, nil
		}
	}
	resolved, resolvedOK := d.resolveIndirectChain(value)
	if !resolvedOK {
		return nil, fmt.Errorf("playa: /Next entry could not be resolved")
	}
	if _, ok := resolved.(Array); ok {
		if destination := d.ResolveDestination(resolved); destination != nil {
			action := newAction("GoTo")
			action.destination = destination
			return action, nil
		}
		return nil, fmt.Errorf("playa: malformed action destination in /Next")
	}
	if _, ok := resolved.(Dict); !ok {
		return nil, fmt.Errorf("playa: /Next entry is not an action dictionary")
	}
	if action := d.resolveAction(value, seen); action != nil {
		return action, nil
	}
	return nil, fmt.Errorf("playa: malformed action in /Next")
}

// Finalize returns an independent snapshot of this action and its chain.
func (a *Action) FinalizeWithError() (Action, error) {
	if a.document != nil {
		a.document.actionMu.Lock()
		defer a.document.actionMu.Unlock()
		clone, err := finalizeActionStrict(a.materializedLocked())
		if err != nil {
			return Action{}, err
		}
		if clone == nil {
			return Action{}, nil
		}
		return *clone, nil
	}
	clone, err := finalizeActionStrict(a.materializedLocked())
	if err != nil {
		return Action{}, err
	}
	if clone == nil {
		return Action{}, nil
	}
	return *clone, nil
}

func (a *Action) Finalize() Action {
	clone, _ := a.FinalizeWithError()
	return clone
}

func finalizeActionStrict(action *Action) (*Action, error) {
	if action == nil {
		return nil, nil
	}
	if action.scriptErr != nil {
		return nil, action.scriptErr
	}
	action.materializeNextLocked()
	if action.nextErr != nil {
		return nil, action.nextErr
	}
	clone := cloneAction(action)
	clone.document = nil
	clone.nextRaw = nil
	clone.nextItems = nil
	clone.nextSeen = nil
	clone.nextCursor = 0
	clone.actionRef = Ref{}
	clone.hasActionRef = false
	clone.nextReady = true
	for i, next := range action.next {
		child, err := finalizeActionStrict(next)
		if err != nil {
			return nil, err
		}
		clone.next[i] = child
	}
	return clone, nil
}

func finalizeAction(action *Action) *Action {
	if action == nil {
		return nil
	}
	action.materializeNext()
	clone := cloneAction(action)
	clone.document = nil
	clone.nextRaw = nil
	clone.nextItems = nil
	clone.nextSeen = nil
	clone.nextCursor = 0
	clone.actionRef = Ref{}
	clone.hasActionRef = false
	clone.nextReady = true
	for i, next := range clone.next {
		clone.next[i] = finalizeAction(next)
	}
	return clone
}

// PageObject resolves a local GoTo action's destination to a document page.
// Remote and embedded-file actions intentionally do not resolve against the
// current document.
func (a Action) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if a.Kind() != "GoTo" || a.destination == nil {
		return Page{}, ErrPageNotFound
	}
	return a.destination.PageObject(d)
}

// ResolveAction resolves a PDF action dictionary without executing it.
func (d *Document) ResolveAction(value Object) *Action {
	return d.resolveAction(value, map[Ref]bool{})
}

// ResolveActionWithError resolves the root action and reports malformed root
// data. The /Next chain remains lazy; its errors are reported by NextSeq or
// FinalizeWithError when the caller chooses to consume it.
func (d *Document) ResolveActionWithError(value Object) (result *Action, err error) {
	ref, hasRef := d.finalIndirectRef(value)
	if hasRef {
		d.cacheMu.RLock()
		cachedErr, found := d.actionErrors[ref]
		d.cacheMu.RUnlock()
		if found {
			return nil, cachedErr
		}
		defer func() {
			if err == nil {
				return
			}
			errorBytes := len(err.Error())
			if errorBytes > d.cacheLimits().ActionErrorBytes {
				return
			}
			d.cacheMu.Lock()
			if d.actionErrors == nil {
				d.actionErrors = map[Ref]error{}
			}
			if cachedErr, found := d.actionErrors[ref]; found {
				err = cachedErr
			} else if cacheFits(d.actionErrorBytes, errorBytes, d.cacheLimits().ActionErrorBytes) {
				d.actionErrors[ref] = err
				d.actionErrorBytes += errorBytes
			}
			d.cacheMu.Unlock()
		}()
	}
	resolved, ok := d.resolveIndirectChain(value)
	if !ok {
		return nil, fmt.Errorf("playa: action is not a dictionary")
	}
	if _, ok := resolved.(Array); ok {
		destination, err := d.ResolveDestinationWithError(resolved)
		if err != nil {
			return nil, err
		}
		if destination == nil {
			return nil, fmt.Errorf("playa: malformed direct action destination")
		}
		action := newAction("GoTo")
		action.destination = destination
		return action, nil
	}
	action, ok := resolved.(Dict)
	if !ok {
		return nil, fmt.Errorf("playa: action is not a dictionary")
	}
	subtype, subtypeResolved := d.resolveIndirectChain(action[Name("S")])
	if !subtypeResolved {
		return nil, fmt.Errorf("playa: action /S is not a name")
	}
	if _, ok := subtype.(Name); !ok {
		return nil, fmt.Errorf("playa: action /S is not a name")
	}
	resolvedAction := d.ResolveAction(value)
	if resolvedAction == nil {
		return nil, fmt.Errorf("playa: malformed action dictionary")
	}
	if resolvedAction.scriptErr != nil {
		return nil, resolvedAction.scriptErr
	}
	validateActionField := func(key Name, want func(Object) bool, message string) error {
		raw, present := action[key]
		if !present {
			return nil
		}
		field, resolved := d.resolveIndirectChain(raw)
		if !resolved {
			return fmt.Errorf("playa: action /%s could not be resolved", key)
		}
		if !want(field) {
			return fmt.Errorf("playa: action /%s %s", key, message)
		}
		return nil
	}
	switch resolvedAction.Kind() {
	case "URI":
		if err := validateActionField(Name("URI"), func(value Object) bool { _, ok := value.(String); return ok }, "is not a string"); err != nil {
			return nil, err
		}
	case "Named":
		if err := validateActionField(Name("N"), func(value Object) bool { _, ok := value.(Name); return ok }, "is not a name"); err != nil {
			return nil, err
		}
	case "JavaScript":
		if err := validateActionField(Name("JS"), func(value Object) bool {
			_, stringOK := value.(String)
			_, streamOK := value.(Stream)
			return stringOK || streamOK
		}, "is not a string or stream"); err != nil {
			return nil, err
		}
	case "GoToR", "GoToE", "Launch":
		if err := validateActionField(Name("F"), func(value Object) bool {
			_, stringOK := value.(String)
			_, dictOK := value.(Dict)
			return stringOK || dictOK
		}, "is not a file specification"); err != nil {
			return nil, err
		}
	}
	if resolvedAction.Kind() == "GoTo" || resolvedAction.Kind() == "GoToR" || resolvedAction.Kind() == "GoToE" {
		destination, err := d.ResolveDestinationWithError(action[Name("D")])
		if err != nil {
			return nil, err
		}
		if destination == nil {
			return nil, fmt.Errorf("playa: malformed action destination")
		}
	}
	return resolvedAction, nil
}

func (d *Document) resolveAction(value Object, seen map[Ref]bool) *Action {
	var ref Ref
	hasRef := false
	if candidate, ok := value.(Ref); ok {
		ref = candidate
		if canonical, ok := d.finalIndirectRef(value); ok {
			ref = canonical
		}
		if seen[ref] {
			return nil
		}
		hasRef = true
		d.cacheMu.RLock()
		cached, found := d.actionCache[ref]
		d.cacheMu.RUnlock()
		if found {
			return cached
		}
		seen[ref] = true
		defer delete(seen, ref)
	}
	resolved, resolvedOK := d.resolveIndirectChain(value)
	if !resolvedOK {
		return nil
	}
	if _, ok := resolved.(Array); ok {
		if destination := d.ResolveDestination(resolved); destination != nil {
			action := newAction("GoTo")
			action.destination = destination
			return action
		}
	}
	action, ok := resolved.(Dict)
	if !ok {
		return nil
	}
	kindValue, ok := d.resolveIndirectChain(action[Name("S")])
	kind, kindOK := kindValue.(Name)
	if !ok || !kindOK {
		return nil
	}
	kindName := string(kind)
	var destination *Destination
	var uri, file, name, script string
	var scriptErr error
	switch kindName {
	case "GoTo", "GoToR", "GoToE":
		destination = d.ResolveDestination(action[Name("D")])
		fileValue, _ := d.resolveIndirectChain(action[Name("F")])
		file = actionFileName(d, fileValue)
	case "URI":
		if uriValue, resolved := d.resolveIndirectChain(action[Name("URI")]); resolved {
			if uriString, ok := uriValue.(String); ok {
				uri = decodePDFText(uriString)
			}
		}
	case "Named":
		nameValue, _ := d.resolveIndirectChain(action[Name("N")])
		name = actionName(nameValue)
	case "Launch":
		fileValue, _ := d.resolveIndirectChain(action[Name("F")])
		file = actionFileName(d, fileValue)
	case "JavaScript":
		rawScript := action[Name("JS")]
		if ref, ok := rawScript.(Ref); ok {
			if canonical, canonicalOK := d.finalIndirectRef(rawScript); canonicalOK {
				ref = canonical
			}
			d.cacheMu.RLock()
			cached, found := d.actionScriptCache[ref]
			d.cacheMu.RUnlock()
			if found {
				script = cached
				break
			}
		}
		scriptValue, scriptResolved := d.resolveIndirectChain(rawScript)
		if !scriptResolved {
			scriptErr = fmt.Errorf("playa: action /JS could not be resolved")
			break
		}
		if scriptString, ok := scriptValue.(String); ok {
			script = decodePDFText(scriptString)
		} else if scriptStream, ok := scriptValue.(Stream); ok {
			resolveScript := func(value Object) Object {
				resolved, _ := d.resolveIndirectChain(value)
				return resolved
			}
			filters, parms := streamFiltersWithResolver(scriptStream.DictBorrowed(), resolveScript)
			if data, err := decodeFiltersLimited(scriptStream.DataBorrowed(), filters, parms, decodedFilterExpansionLimit); err == nil {
				script = decodePDFText(String(data))
			} else {
				scriptErr = fmt.Errorf("playa: action /JS stream could not be decoded: %w", err)
			}
		} else {
			scriptErr = fmt.Errorf("playa: action /JS is not a string or stream")
		}
		scriptLimit := d.cacheLimits().ActionScriptBytes
		if ref, ok := rawScript.(Ref); ok && scriptErr == nil && len(script) <= scriptLimit {
			if canonical, canonicalOK := d.finalIndirectRef(rawScript); canonicalOK {
				ref = canonical
			}
			d.cacheMu.Lock()
			if cacheFits(d.actionScriptCacheBytes, len(script), scriptLimit) {
				if d.actionScriptCache == nil {
					d.actionScriptCache = map[Ref]string{}
				}
				d.actionScriptCache[ref] = script
				d.actionScriptCacheBytes += len(script)
			}
			d.cacheMu.Unlock()
		}
	}
	out := &Action{
		data:        documentdata.NewAction(kindName, uri, file, name, script, action),
		destination: destination,
		scriptErr:   scriptErr,
		document:    d,
	}
	if hasRef {
		out.actionRef, out.hasActionRef = ref, true
	}
	if rawNext, exists := action[Name("Next")]; exists {
		out.nextRaw = rawNext
	}
	if hasRef {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if cached, found := d.actionCache[ref]; found {
			return cached
		}
		actionBytes := nameTreeObjectSize(action, 8) + len(script)
		if cacheFits(d.actionCacheBytes, actionBytes, d.cacheLimits().ActionBytes) {
			if d.actionCache == nil {
				d.actionCache = map[Ref]*Action{}
			}
			d.actionCache[ref] = out
			d.actionCacheBytes += actionBytes
		}
	}
	return out
}

// OpenAction returns the catalog action that should be applied when opening
// the document, without executing it.
func (d *Document) OpenAction() *Action {
	action, _ := d.OpenActionWithError()
	return action
}

// OpenActionWithError resolves the catalog open action without executing it.
// A catalog without /OpenAction is reported as a nil action and nil error.
func (d *Document) OpenActionWithError() (*Action, error) {
	d.cacheMu.RLock()
	if d.openActionReady {
		action := cloneAction(d.openActionCache)
		d.cacheMu.RUnlock()
		return action, nil
	}
	if d.openActionErrReady {
		err := d.openActionErr
		d.cacheMu.RUnlock()
		return nil, err
	}
	d.cacheMu.RUnlock()
	cacheError := func(err error) (*Action, error) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.openActionReady {
			return cloneAction(d.openActionCache), nil
		}
		if d.openActionErrReady {
			return nil, d.openActionErr
		}
		d.openActionErr = err
		d.openActionErrReady = true
		return nil, err
	}
	cacheAction := func(action *Action) (*Action, error) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.openActionReady {
			return cloneAction(d.openActionCache), nil
		}
		if d.openActionErrReady {
			return nil, d.openActionErr
		}
		d.openActionCache = action
		d.openActionReady = true
		return cloneAction(action), nil
	}
	catalog, err := d.catalogWithError()
	if err != nil {
		return cacheError(err)
	}
	if catalog == nil {
		return cacheAction(nil)
	}
	value, present := catalog[Name("OpenAction")]
	if !present {
		return cacheAction(nil)
	}
	action, err := d.ResolveActionWithError(value)
	if err != nil {
		return cacheError(err)
	}
	return cacheAction(action)
}

func actionFileName(d *Document, value Object) string {
	resolve := func(value Object) Object {
		if d == nil {
			return value
		}
		resolved, _ := d.resolveIndirectChain(value)
		return resolved
	}
	value = resolve(value)
	switch value := value.(type) {
	case String:
		return decodePDFText(value)
	case Name:
		return string(value)
	case Dict:
		if file, ok := resolve(value[Name("UF")]).(String); ok {
			return decodePDFText(file)
		}
		if file, ok := resolve(value[Name("F")]).(String); ok {
			return decodePDFText(file)
		}
	}
	return ""
}

func actionName(value Object) string {
	switch value := value.(type) {
	case Name:
		return string(value)
	case String:
		return decodePDFText(value)
	default:
		return ""
	}
}

// ResolveDestination resolves a direct destination array or a named
// destination from the document catalog. Errors from named-destination trees
// are suppressed for compatibility; use ResolveDestinationWithError when the
// caller needs to distinguish a missing destination from a malformed tree.
func (d *Document) ResolveDestination(value Object) *Destination {
	target, _ := d.ResolveDestinationWithError(value)
	return target
}

// ResolveDestinationWithError resolves a direct destination array or a named
// destination and reports malformed named-destination trees.
func (d *Document) ResolveDestinationWithError(value Object) (result *Destination, err error) {
	ref, hasRef := d.finalIndirectRef(value)
	if hasRef {
		d.cacheMu.RLock()
		cachedErr, found := d.destinationErrors[ref]
		d.cacheMu.RUnlock()
		if found {
			return nil, cachedErr
		}
		defer func() {
			if err == nil {
				return
			}
			errorBytes := len(err.Error())
			if errorBytes > d.cacheLimits().DestinationErrorBytes {
				return
			}
			d.cacheMu.Lock()
			if d.destinationErrors == nil {
				d.destinationErrors = map[Ref]error{}
			}
			if cachedErr, found := d.destinationErrors[ref]; found {
				err = cachedErr
			} else if cacheFits(d.destinationErrorBytes, errorBytes, d.cacheLimits().DestinationErrorBytes) {
				d.destinationErrors[ref] = err
				d.destinationErrorBytes += errorBytes
			}
			d.cacheMu.Unlock()
		}()
	}
	resolved, ok := d.resolveIndirectChain(value)
	if !ok {
		return nil, fmt.Errorf("playa: destination could not be resolved")
	}
	value = resolved
	if name, ok := value.(String); ok {
		var err error
		value, err = d.destinationByNameWithError(decodePDFText(name))
		if err != nil {
			return nil, err
		}
	} else if name, ok := value.(Name); ok {
		var err error
		value, err = d.destinationByNameWithError(string(name))
		if err != nil {
			return nil, err
		}
	}
	value, ok = d.resolveIndirectChain(value)
	if !ok {
		return nil, fmt.Errorf("playa: destination could not be resolved")
	}
	if dict, ok := value.(Dict); ok {
		rawDestination, present := dict[Name("D")]
		if !present {
			return nil, nil
		}
		value, ok = d.resolveIndirectChain(rawDestination)
		if !ok {
			return nil, fmt.Errorf("playa: destination /D could not be resolved")
		}
	}
	array, ok := value.(Array)
	if !ok || len(array) < 2 {
		return nil, nil
	}
	viewValue, ok := d.resolveIndirectChain(array[1])
	if !ok {
		return nil, fmt.Errorf("playa: destination view could not be resolved")
	}
	view, _ := viewValue.(Name)
	if expected, known := destinationParameterCount(view); known && len(array)-2 != expected {
		return nil, fmt.Errorf("playa: destination %s has %d parameters, want %d", view, len(array)-2, expected)
	}
	var page Ref
	var hasPage bool
	var pageIndex int
	var hasPageIndex bool
	if pageRef, ok := d.finalIndirectRef(array[0]); ok {
		page, hasPage = pageRef, true
	} else if pageValue, pageResolved := d.resolveIndirectChain(array[0]); pageResolved {
		pageNumber, ok := IntValue(pageValue)
		if !ok {
			pageIndex, hasPageIndex = 0, true
		} else {
			pageIndex, hasPageIndex = pageNumber-1, true
			if pageIndex < 0 {
				pageIndex = 0
			}
		}
	} else {
		pageIndex, hasPageIndex = 0, true
	}
	viewName := ""
	if view, ok := viewValue.(Name); ok {
		viewName = string(view)
	}
	return newDestination(page, hasPage, pageIndex, hasPageIndex, viewName, array[2:]), nil
}

func destinationParameterCount(view Name) (int, bool) {
	switch view {
	case Name("XYZ"):
		return 3, true
	case Name("FitH"), Name("FitBH"), Name("FitV"), Name("FitBV"):
		return 1, true
	case Name("FitR"):
		return 4, true
	case Name("Fit"), Name("FitB"):
		return 0, true
	default:
		return 0, false
	}
}

func (d *Document) destinationByName(name string) Object {
	value, _ := d.destinationByNameWithError(name)
	return value
}

func (d *Document) destinationByNameWithError(name string) (Object, error) {
	d.cacheMu.RLock()
	ready := d.destinationsReady && d.destinationsCacheable && d.destinationsErr == nil
	cachedValue, found := d.destinationsCache[name]
	if !ready {
		cachedValue, found = d.destinationCache[name]
	}
	d.cacheMu.RUnlock()
	if ready {
		if found {
			return cachedValue, nil
		}
		return nil, d.destinationsErr
	}
	if found {
		return cachedValue, nil
	}
	var value Object
	var treeErr error
	for entry, err := range d.DestinationsSeq() {
		if err != nil {
			treeErr = err
			break
		}
		if entry.Name() == name {
			value = entry.ValueCopy()
			break
		}
	}
	if treeErr != nil {
		return nil, treeErr
	}
	d.cacheDestination(name, value)
	return value, nil
}
