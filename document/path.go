package document

import (
	"encoding/json"
	"iter"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
)

// PathObject is the geometry emitted by a path painting operator. RawSegments
// retain Playa's user-space representation; BBox is already in device space.
type PathObject struct {
	data         contentdata.Path
	parentKey    int
	hasParentKey bool
}

func newPathObject(spec contentdata.PathSpec) PathObject {
	return PathObject{data: contentdata.NewPathBorrowed(spec)}
}

func newPathObjectWithContext(spec contentdata.PathSpec, context *contentdata.PathContext) PathObject {
	return PathObject{data: contentdata.NewPathBorrowedWithContext(spec, context)}
}

func pathSpec(p PathObject) contentdata.PathSpec {
	return p.data.SpecBorrowed()
}

func (p *PathObject) setPage(page Ref) {
	if p == nil {
		return
	}
	spec := pathSpec(*p)
	spec.Page, spec.HasPage = page, page != (Ref{})
	p.data = contentdata.NewPathBorrowed(spec)
}

func (p PathObject) Page() Ref         { return p.data.Page() }
func (p PathObject) HasPage() bool     { return p.data.HasPage() }
func (p PathObject) Stroke() bool      { return p.data.Stroke() }
func (p PathObject) Fill() bool        { return p.data.Fill() }
func (p PathObject) EvenOdd() bool     { return p.data.EvenOdd() }
func (p PathObject) Clip() bool        { return p.data.Clip() }
func (p PathObject) ClipEvenOdd() bool { return p.data.ClipEvenOdd() }
func (p PathObject) BBox() [4]float64  { return p.data.BBox() }
func (p PathObject) X0() float64       { return bboxX0(p.BBox()) }
func (p PathObject) Y0() float64       { return bboxY0(p.BBox()) }
func (p PathObject) X1() float64       { return bboxX1(p.BBox()) }
func (p PathObject) Y1() float64       { return bboxY1(p.BBox()) }
func (p PathObject) Width() float64    { return bboxWidth(p.BBox()) }
func (p PathObject) Height() float64   { return bboxHeight(p.BBox()) }
func (p PathObject) IsEmpty() bool     { return bboxIsEmpty(p.BBox()) }
func (p PathObject) IsHoverlap(other BBoxProvider) bool {
	return bboxIsHoverlap(p.BBox(), other)
}
func (p PathObject) HDistance(other BBoxProvider) float64 {
	return bboxHDistance(p.BBox(), other)
}
func (p PathObject) Hoverlap(other BBoxProvider) float64 {
	return bboxHoverlap(p.BBox(), other)
}
func (p PathObject) IsVOverlap(other BBoxProvider) bool {
	return bboxIsVOverlap(p.BBox(), other)
}
func (p PathObject) VDistance(other BBoxProvider) float64 {
	return bboxVDistance(p.BBox(), other)
}
func (p PathObject) VOverlap(other BBoxProvider) float64 {
	return bboxVOverlap(p.BBox(), other)
}
func (p PathObject) GState() GraphicsState { return p.data.GState() }
func (p PathObject) MarkedTag() string     { return p.data.MarkedTag() }
func (p PathObject) ActualText() string    { return p.data.ActualText() }
func (p PathObject) MCID() int             { return p.data.MCID() }
func (p PathObject) HasMCID() bool         { return p.data.HasMCID() }

// Len returns the number of children yielded by a path object. Paths are leaf
// content objects in Playa's interpreter model.
func (p PathObject) Len() int { return 0 }

// RawSegmentsCopy returns independent user-space path segments.
func (p PathObject) RawSegmentsCopy() []geometry.PathSegment {
	return p.data.RawSegmentsCopy()
}

// SegmentsCopy returns independent device-space path segments.
func (p PathObject) SegmentsCopy() []geometry.PathSegment {
	return p.data.SegmentsCopy()
}

// MarkedPropertiesCopy returns independent path marked-content properties.
func (p PathObject) MarkedPropertiesCopy() Dict { return p.data.MarkedPropertiesCopy() }

// MarkedStackCopy returns an independent path marked-content stack.
func (p PathObject) MarkedStackCopy() []MarkedContentContext {
	return p.data.MarkedStackCopy()
}

func (p PathObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Page             Ref                    `json:"Page"`
		HasPage          bool                   `json:"HasPage"`
		Stroke           bool                   `json:"Stroke"`
		Fill             bool                   `json:"Fill"`
		EvenOdd          bool                   `json:"EvenOdd"`
		Clip             bool                   `json:"Clip"`
		ClipEvenOdd      bool                   `json:"ClipEvenOdd"`
		BBox             [4]float64             `json:"BBox"`
		GState           GraphicsState          `json:"gstate"`
		MarkedTag        string                 `json:"marked_tag,omitempty"`
		ActualText       string                 `json:"actual_text,omitempty"`
		MCID             int                    `json:"mcid,omitempty"`
		HasMCID          bool                   `json:"has_mcid"`
		RawSegments      []geometry.PathSegment `json:"RawSegments"`
		Segments         []geometry.PathSegment `json:"Segments"`
		MarkedProperties Dict                   `json:"marked_properties,omitempty"`
		MarkedStack      []MarkedContentContext `json:"marked_stack,omitempty"`
	}{
		Page: p.Page(), HasPage: p.HasPage(), Stroke: p.Stroke(), Fill: p.Fill(), EvenOdd: p.EvenOdd(),
		Clip: p.Clip(), ClipEvenOdd: p.ClipEvenOdd(), BBox: p.BBox(), GState: p.GState(),
		MarkedTag: p.MarkedTag(), ActualText: p.ActualText(), MCID: p.MCID(), HasMCID: p.HasMCID(),
		RawSegments: p.RawSegmentsCopy(), Segments: p.SegmentsCopy(), MarkedProperties: p.MarkedPropertiesCopy(), MarkedStack: p.MarkedStackCopy(),
	})
}

// PageObject resolves the page associated with a page-level path.
func (p PathObject) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !p.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(p.Page())
}

// Parent resolves the structure element associated with this path's MCID.
func (p PathObject) Parent(d *Document) *StructElement {
	parent, _ := p.ParentWithError(d)
	return parent
}

// ParentWithError resolves this path's ParentTree element and reports
// malformed page structure data.
func (p PathObject) ParentWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	stack := p.data.SpecBorrowed().MarkedStack
	for index := len(stack) - 1; index >= 0; index-- {
		if stack[index].HasMCID() {
			return d.contentParentWithContext(p.Page(), stack[index].MCID(), true, p.parentKey, p.hasParentKey)
		}
	}
	return d.contentParentWithContext(p.Page(), p.MCID(), p.HasMCID(), p.parentKey, p.hasParentKey)
}

func InterpretPaths(ops []ContentOp) []PathObject {
	index := 0
	out := make([]PathObject, 0, initialPathCapacity(ops))
	interpretPathsNext(nil, func() (ContentOp, bool) {
		if index >= len(ops) {
			return ContentOp{}, false
		}
		op := ops[index]
		index++
		return op, true
	}, nil, func(path PathObject) bool {
		out = append(out, path)
		return true
	})
	return out
}

func initialPathCapacity(ops []ContentOp) int {
	const sampleLimit = 512
	limit := len(ops)
	if limit > sampleLimit {
		limit = sampleLimit
	}
	count := 0
	for index := range limit {
		if isPathPaintOperator(ops[index].operatorValue()) {
			count++
		}
	}
	return count
}

// InterpretPathsSeq lazily interprets painted paths in content-stream order.
// The sequence is repeatable and stops interpreting when the consumer stops
// yielding.
func InterpretPathsSeq(ops []ContentOp) iter.Seq[PathObject] {
	return func(yield func(PathObject) bool) {
		index := 0
		interpretPathsNext(nil, func() (ContentOp, bool) {
			if index >= len(ops) {
				return ContentOp{}, false
			}
			op := ops[index]
			index++
			return op, true
		}, nil, yield)
	}
}

func pathOperatorPreservesContext(operator string) bool {
	switch operator {
	case "m", "l", "c", "v", "y", "h", "re", "S", "s", "f", "F", "f*", "B", "B*", "b", "b*", "n", "W", "W*":
		return true
	default:
		return false
	}
}

func pathOperatorPreservesGraphicsShared(operator string) bool {
	if pathOperatorPreservesContext(operator) {
		return true
	}
	switch operator {
	case "q", "Q", "cm", "BMC", "BDC", "EMC", "MP", "DP", "BX", "EX":
		return true
	default:
		return false
	}
}

func textShowHasGlyphs(op ContentOp) bool {
	operands := op.operandsValue()
	switch op.operatorValue() {
	case "Tj", "'", "\"":
		if len(operands) == 0 {
			return false
		}
		value, ok := operands[len(operands)-1].(String)
		return ok && len(value) != 0
	case "TJ":
		if len(operands) == 0 {
			return false
		}
		values, ok := operands[0].(Array)
		if !ok {
			return false
		}
		for _, value := range values {
			if text, ok := value.(String); ok && len(text) != 0 {
				return true
			}
		}
	}
	return false
}

func interpretPathsNext(d *Document, next func() (ContentOp, bool), restrictOps map[string]struct{}, yield func(PathObject) bool) {
	ctm := identity()
	type saved struct {
		ctm              geometry.Matrix
		gstate           graphicsState
		graphicsRevision uint64
	}
	stack := []saved{}
	gstate := newGraphicsState()
	marked := []markedContentFrame{}
	markedForms := [][]markedContentFrame{}
	path := []geometry.PathSegment{}
	emitCurrent := true
	clipPending, clipEvenOdd := false, false
	textClipPending := false
	haveCurrent := false
	haveSubpath := false
	stopped := false
	objectParentKey := 0
	objectHasParentKey := false
	type contextKey struct {
		revision                                 uint64
		stroke, fill, evenOdd, clip, clipEvenOdd bool
	}
	var stateRevision uint64
	var graphicsRevision uint64
	var nextGraphicsRevision uint64
	advanceGraphicsRevision := func() {
		nextGraphicsRevision++
		graphicsRevision = nextGraphicsRevision
	}
	var cachedContextKey contextKey
	var cachedContext *contentdata.PathContext
	var cachedGraphicsRevision uint64
	var cachedGraphics GraphicsState
	var haveCachedGraphics bool

	xy := func(a []Object, at int) ([2]float64, bool) {
		if at+1 >= len(a) {
			return [2]float64{}, false
		}
		x, okx := finiteNumberValue(a[at])
		y, oky := finiteNumberValue(a[at+1])
		return [2]float64{x, y}, okx && oky
	}
	add := func(operator string, points ...[2]float64) {
		path = append(path, geometry.NewPathSegment(operator, points...))
	}
	closePath := func() {
		if haveCurrent && haveSubpath {
			add("h")
		}
	}
	paint := func(stroke, fill, evenOdd bool) {
		if len(path) == 0 || (!stroke && !fill) {
			path = nil
			return
		}
		minX, minY, maxX, maxY := 0.0, 0.0, 0.0, 0.0
		first := true
		include := func(x, y float64) {
			if first {
				minX, maxX, minY, maxY = x, x, y, y
				first = false
				return
			}
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
		var cursor [2]float64
		haveCursor := false
		for _, seg := range path {
			points := seg.PointsCopy()
			switch seg.Operator() {
			case "m", "l":
				if len(points) > 0 {
					cursor = points[len(points)-1]
					haveCursor = true
					include(cursor[0], cursor[1])
				}
			case "v", "y":
				for _, p := range points {
					include(p[0], p[1])
				}
				if len(points) > 0 {
					cursor = points[len(points)-1]
					haveCursor = true
				}
			case "c":
				if haveCursor && len(points) >= 3 {
					for _, p := range append([][2]float64{cursor}, points[:3]...) {
						include(p[0], p[1])
					}
					cursor = points[2]
					haveCursor = true
				}
			default:
				for _, p := range points {
					include(p[0], p[1])
				}
			}
		}
		if !first {
			// Playa bounds the user-space path first, then transforms the four
			// corners. Transforming each point directly produces a tighter but
			// observably different box for rotated or skewed CTMs.
			userMinX, userMinY, userMaxX, userMaxY := minX, minY, maxX, maxY
			first = true
			for _, point := range [][2]float64{{userMinX, userMinY}, {userMinX, userMaxY}, {userMaxX, userMinY}, {userMaxX, userMaxY}} {
				x, y, ok := ctm.PointFinite(point[0], point[1])
				if !ok {
					path = nil
					return
				}
				if first {
					minX, maxX, minY, maxY = x, x, y, y
					first = false
					continue
				}
				minX, maxX = min(minX, x), max(maxX, x)
				minY, maxY = min(minY, y), max(maxY, y)
			}
			if clipPending {
				// A clipping path becomes part of the graphics state when the
				// current path is painted, not only when it is discarded with n.
				gstate.clipDepth++
				gstate.clipEvenOdd = clipEvenOdd
				stateRevision++
				advanceGraphicsRevision()
			}
			device := make([]geometry.PathSegment, 0, len(path))
			for _, seg := range path {
				segmentPoints := seg.PointsCopy()
				points := make([][2]float64, 0, len(segmentPoints))
				for _, p := range segmentPoints {
					x, y, ok := ctm.PointFinite(p[0], p[1])
					if !ok {
						path = nil
						return
					}
					points = append(points, [2]float64{x, y})
				}
				device = append(device, geometry.NewPathSegment(seg.Operator(), points...))
			}
			m := currentMarkedContent(marked)
			key := contextKey{revision: stateRevision, stroke: stroke, fill: fill, evenOdd: evenOdd, clip: clipPending, clipEvenOdd: clipEvenOdd}
			spec := contentdata.PathSpec{RawSegments: append([]geometry.PathSegment(nil), path...), Segments: device, BBox: [4]float64{minX, minY, maxX, maxY}}
			if cachedContext == nil || key != cachedContextKey {
				if !haveCachedGraphics || cachedGraphicsRevision != graphicsRevision {
					objectState := gstate.Clone()
					cachedGraphics = objectState.publicValue()
					cachedGraphicsRevision = graphicsRevision
					haveCachedGraphics = true
				}
				spec.Stroke, spec.Fill, spec.EvenOdd = stroke, fill, evenOdd
				spec.Clip, spec.ClipEvenOdd = clipPending, clipEvenOdd
				spec.GState = cachedGraphics.WithCTM(gstate.ctm)
				spec.MarkedTag, spec.MarkedProperties = m.Tag, m.Properties
				spec.MarkedStack = markedStackCopy(markedContextStack(marked))
				spec.ActualText, spec.MCID, spec.HasMCID = m.ActualText, m.MCID, m.HasMCID
				cachedContext = contentdata.NewPathContextBorrowed(spec)
				cachedContextKey = key
			}
			object := newPathObjectWithContext(spec, cachedContext)
			object.parentKey, object.hasParentKey = objectParentKey, objectHasParentKey
			if emitCurrent && !yield(object) {
				stopped = true
			}
		}
		path = nil
		haveCurrent, haveSubpath = false, false
		clipPending, clipEvenOdd = false, false
	}

	for !stopped {
		op, ok := next()
		if !ok {
			break
		}
		if op.formBoundary != 0 {
			stateRevision++
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
		objectParentKey, objectHasParentKey = op.parentKey, op.hasParentKey
		if !pathOperatorPreservesContext(op.operatorValue()) {
			stateRevision++
		}
		if !pathOperatorPreservesGraphicsShared(op.operatorValue()) {
			advanceGraphicsRevision()
		}
		if isPathPaintOperator(op.operatorValue()) {
			emitCurrent = len(restrictOps) == 0
			if !emitCurrent {
				_, emitCurrent = restrictOps[op.operatorValue()]
			}
		}
		applyMarkedContent(&marked, op)
		if op.operatorValue() != "q" && op.operatorValue() != "Q" && op.operatorValue() != "cm" {
			applyGraphicsState(&gstate, op)
			applyExternalGraphicsState(d, &gstate, op)
			applyResourceColorSpace(d, &gstate, op)
		}
		switch op.operatorValue() {
		case "q":
			stack = append(stack, saved{ctm: ctm, gstate: gstate.Clone(), graphicsRevision: graphicsRevision})
		case "Q":
			if len(stack) != 0 {
				s := stack[len(stack)-1]
				ctm = s.ctm
				gstate = s.gstate
				graphicsRevision = s.graphicsRevision
				stack = stack[:len(stack)-1]
			}
		case "cm":
			if len(op.operandsValue()) >= 6 {
				var m geometry.Matrix
				valid := true
				for i := range m {
					m[i], valid = finiteNumberValue(op.operandsValue()[i])
					if !valid {
						break
					}
				}
				if valid {
					if product, ok := ctm.MulFinite(m); ok {
						ctm = product
						gstate.ctm = ctm
					}
				}
			}
		case "BT":
			textClipPending = false
		case "Tj", "TJ", "'", "\"":
			if gstate.renderMode >= 4 && textShowHasGlyphs(op) {
				textClipPending = true
			}
		case "ET":
			if textClipPending {
				// Text clipping accumulates glyph outlines throughout the text
				// object and intersects the current clipping path only at ET.
				gstate.clipDepth++
				gstate.clipEvenOdd = false
				stateRevision++
				advanceGraphicsRevision()
				textClipPending = false
			}
		case "m":
			if p, ok := xy(op.operandsValue(), 0); ok {
				add("m", p)
				haveCurrent, haveSubpath = true, true
			}
		case "l":
			if p, ok := xy(op.operandsValue(), 0); ok {
				add("l", p)
				haveCurrent = true
			}
		case "c":
			if p1, ok1 := xy(op.operandsValue(), 0); ok1 {
				if p2, ok2 := xy(op.operandsValue(), 2); ok2 {
					if p3, ok3 := xy(op.operandsValue(), 4); ok3 {
						add("c", p1, p2, p3)
						haveCurrent = true
					}
				}
			}
		case "v":
			if haveCurrent {
				if p2, ok2 := xy(op.operandsValue(), 0); ok2 {
					if p3, ok3 := xy(op.operandsValue(), 2); ok3 {
						add("v", p2, p3)
					}
				}
			}
		case "y":
			if p1, ok1 := xy(op.operandsValue(), 0); ok1 {
				if p3, ok3 := xy(op.operandsValue(), 2); ok3 {
					add("y", p1, p3)
					haveCurrent = true
				}
			}
		case "h":
			closePath()
		case "re":
			if p, ok := xy(op.operandsValue(), 0); ok && len(op.operandsValue()) >= 4 {
				w, okw := finiteNumberValue(op.operandsValue()[2])
				h, okh := finiteNumberValue(op.operandsValue()[3])
				if okw && okh {
					p1 := p
					p2 := [2]float64{p[0] + w, p[1]}
					p3 := [2]float64{p[0] + w, p[1] + h}
					p4 := [2]float64{p[0], p[1] + h}
					add("m", p1)
					add("l", p2)
					add("l", p3)
					add("l", p4)
					add("h")
					haveCurrent, haveSubpath = true, true
				}
			}
		case "S":
			paint(true, false, false)
		case "s":
			closePath()
			paint(true, false, false)
		case "f", "F":
			paint(false, true, false)
		case "f*":
			paint(false, true, true)
		case "B":
			paint(true, true, false)
		case "B*":
			paint(true, true, true)
		case "b":
			closePath()
			paint(true, true, false)
		case "b*":
			closePath()
			paint(true, true, true)
		case "n":
			if clipPending {
				gstate.clipDepth++
				gstate.clipEvenOdd = clipEvenOdd
				stateRevision++
				advanceGraphicsRevision()
				path = nil
				clipPending, clipEvenOdd = false, false
			} else {
				path = nil
			}
			haveCurrent, haveSubpath = false, false
		case "W":
			clipPending = true
			clipEvenOdd = false
		case "W*":
			clipPending = true
			clipEvenOdd = true
		}
	}
}

func (d *Document) PagePaths(p Page) ([]PathObject, error) {
	var out []PathObject
	for path, err := range d.PagePathsSeq(p) {
		if err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, nil
}

// PagePathsSeq is the lazy document-level path traversal used by Page.Paths.
func (d *Document) PagePathsSeq(p Page) iter.Seq2[PathObject, error] {
	return p.paths(d, true, contentconfig.Options{Filter: FilterPath})
}
