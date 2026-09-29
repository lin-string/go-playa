package document

import (
	"bytes"
	"errors"
	"fmt"
	"iter"
	"math"
	"sort"

	"github.com/lin-string/go-playa/coordinates"
	"github.com/lin-string/go-playa/geometry"
)

const pagesCacheLimit = 32 << 20

func pageCacheSize(page Page) int {
	size := addCacheSize(256, len(page.lastModified))
	return addCacheSize(size, nameTreeObjectSize(page.dict, 8))
}

const lookupCacheLimit = 32 << 20
const pageMissingCacheLimit = 32 << 20
const objectErrorCacheLimit = 4096

func (d *Document) resetLookupIndexLocked() {
	d.lookupCache = nil
	d.lookupFound = nil
	d.lookupCacheBytes = 0
	d.lookupIndexReady = false
	d.lookupIndexDisabled = false
}

func (d *Document) buildLookupIndex() {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if d.lookupIndexReady || d.lookupIndexDisabled {
		return
	}
	maxEntries := d.cacheLimits().LookupBytes / 64
	if maxEntries <= 0 {
		d.lookupIndexDisabled = true
		return
	}
	capacity := len(d.xrefs) + len(d.xrefHistory) + len(d.objects)
	if capacity > maxEntries {
		capacity = maxEntries
	}
	refs := make(map[int]Ref, capacity)
	found := make(map[int]bool, capacity)
	add := func(ref Ref) bool {
		current, exists := refs[ref.Object]
		if exists && current.Generation >= ref.Generation {
			return true
		}
		if !exists && len(found) >= maxEntries {
			return false
		}
		refs[ref.Object] = ref
		found[ref.Object] = true
		return true
	}
	for ref, entry := range d.xrefs {
		if !entry.isFree() && !add(ref) {
			d.lookupIndexDisabled = true
			return
		}
	}
	for ref, entry := range d.xrefHistory {
		if _, current := d.xrefs[ref]; current {
			continue
		}
		if !entry.isFree() && !add(ref) {
			d.lookupIndexDisabled = true
			return
		}
	}
	for ref := range d.objects {
		if entry, current := d.xrefs[ref]; current {
			if entry.isFree() {
				continue
			}
		} else if entry, historical := d.xrefHistory[ref]; historical && entry.isFree() {
			continue
		}
		if !add(ref) {
			d.lookupIndexDisabled = true
			return
		}
	}
	d.lookupCache = refs
	d.lookupFound = found
	d.lookupCacheBytes = len(found) * 64
	d.lookupIndexReady = true
}

func (d *Document) cacheLookup(objectNumber int, ref Ref, found bool) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if d.lookupFound == nil {
		d.lookupFound = map[int]bool{}
	}
	if _, exists := d.lookupFound[objectNumber]; exists {
		if found {
			d.lookupFound[objectNumber] = true
			d.lookupCache[objectNumber] = ref
		}
		return
	}
	if !cacheFits(d.lookupCacheBytes, 64, d.cacheLimits().LookupBytes) {
		return
	}
	if d.lookupCache == nil {
		d.lookupCache = map[int]Ref{}
	}
	d.lookupFound[objectNumber] = found
	if found {
		d.lookupCache[objectNumber] = ref
	}
	d.lookupCacheBytes += 64
}

func (d *Document) cacheMissingPage(ref Ref) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if d.pageMissing == nil {
		d.pageMissing = map[Ref]bool{}
	}
	if d.pageMissing[ref] || !cacheFits(d.pageMissingBytes, 64, d.cacheLimits().MissingPageBytes) {
		return
	}
	d.pageMissing[ref] = true
	d.pageMissingBytes += 64
}

func (d *Document) cachePage(ref Ref, page Page) {
	pageBytes := pageCacheSize(page)
	if pageBytes > d.cacheLimits().PageBytes {
		return
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if d.pageCache == nil {
		d.pageCache = map[Ref]Page{}
	}
	if _, exists := d.pageCache[ref]; exists {
		return
	}
	if !cacheFits(d.pageCacheBytes, pageBytes, d.cacheLimits().PageBytes) {
		return
	}
	d.pageCache[ref] = page
	d.pageCacheBytes += pageBytes
}

var ErrPageNotFound = errors.New("playa: page not found")

type Page struct {
	// Number is the one-based display position retained by the original Go
	// API. Index is the zero-based page-tree position used by Playa's page_idx.
	number int
	index  int
	// Space is the device-space convention selected when the document was
	// opened. A zero value falls back to the owning document for low-level
	// manually constructed pages.
	space         coordinates.Space
	ref           Ref
	dict          Dict
	lastModified  string
	fontOrderRef  Ref
	fontOrderPath uint8

	contentMatrix      geometry.Matrix
	hasContentMatrix   bool
	initialRotation    int
	hasInitialRotation bool
	initialGState      graphicsState
	hasInitialGState   bool
	isFormView         bool
}

// hasPageCacheKey distinguishes a real page resource context from a Form's
// synthetic page view. Both keep the owning page reference for public objects,
// but Form resources and metrics must never share its page-keyed caches.
func (p Page) hasPageCacheKey() bool {
	return p.ref != (Ref{}) && !p.isFormView
}

func clonePage(page Page) Page {
	if page.dict != nil {
		page.dict = cloneGraphicsObject(page.dict).(Dict)
	}
	page.initialGState = page.initialGState.Clone()
	return page
}

// Finalize returns an independent snapshot of the page model.
func (p Page) Finalize() Page { return clonePage(p) }

// Number returns the one-based page number retained for display-oriented
// callers.
func (p Page) Number() int { return p.number }

// Index returns Playa's zero-based page index.
func (p Page) Index() int { return p.index }

// Space returns the coordinate-space convention selected for this page.
func (p Page) Space() coordinates.Space { return p.space }

// Ref returns the page's indirect object reference, or the zero reference for
// a directly constructed page.
func (p Page) Ref() Ref { return p.ref }

// LastModified returns the page-tree modification timestamp when present.
func (p Page) LastModified() string { return p.lastModified }

// DictCopy returns an independent copy of the page dictionary.
func (p Page) DictCopy() Dict { return cloneDict(p.dict) }

// Get looks up a page dictionary entry and returns an independent value.
func (p Page) Get(key Name) (Object, bool) {
	value, ok := p.dict[key]
	return cloneGraphicsObject(value), ok
}

// Has reports whether a page dictionary entry exists.
func (p Page) Has(key Name) bool {
	_, ok := p.dict[key]
	return ok
}

func (p Page) BBox(d *Document) [4]float64 {
	b, err := p.BBoxWithError(d)
	if err != nil {
		return p.MediaBox(d)
	}
	return b
}

// BBoxWithError returns the preferred page box and reports malformed explicit
// CropBox or MediaBox values while retaining the default Letter fallback.
func (p Page) BBoxWithError(d *Document) ([4]float64, error) {
	if b, present, err := p.boxWithError(d, Name("CropBox")); present {
		if err != nil {
			return [4]float64{}, err
		}
		return b, nil
	}
	return p.MediaBoxWithError(d)
}

// MediaBox returns the normalized media box, falling back to US Letter when
// the page omits the required PDF MediaBox entry.
func (p Page) MediaBox(d *Document) [4]float64 {
	b, err := p.MediaBoxWithError(d)
	if err != nil {
		return [4]float64{0, 0, 612, 792}
	}
	return b
}

// MediaBoxWithError returns the normalized MediaBox, using the Playa Letter
// fallback only when the page omits the key.
func (p Page) MediaBoxWithError(d *Document) ([4]float64, error) {
	b, present, err := p.boxWithError(d, Name("MediaBox"))
	if !present {
		return [4]float64{0, 0, 612, 792}, nil
	}
	if err != nil {
		return [4]float64{}, err
	}
	return b, nil
}

// CropBox returns the normalized crop box, falling back to the media box.
func (p Page) CropBox(d *Document) [4]float64 {
	b, err := p.CropBoxWithError(d)
	if err != nil {
		return p.MediaBox(d)
	}
	return b
}

// CropBoxWithError returns CropBox when present, otherwise MediaBox, and
// reports malformed explicit values.
func (p Page) CropBoxWithError(d *Document) ([4]float64, error) {
	b, present, err := p.boxWithError(d, Name("CropBox"))
	if present {
		if err != nil {
			return [4]float64{}, err
		}
		return b, nil
	}
	return p.MediaBoxWithError(d)
}

func (p Page) mediaBox(d *Document) [4]float64 {
	return p.MediaBox(d)
}

func (p Page) boxWithError(d *Document, key Name) ([4]float64, bool, error) {
	if d == nil {
		return [4]float64{}, false, errNilDocument
	}
	if p.hasPageCacheKey() {
		if entry, ok := d.cachedPageBox(p.ref, key); ok {
			return entry.value, entry.present, entry.err
		}
	}
	cache := func(value [4]float64, present bool, err error) ([4]float64, bool, error) {
		if p.hasPageCacheKey() {
			entry := d.storePageBox(p.ref, key, pageBoxCacheEntry{value: value, present: present, err: err})
			return entry.value, entry.present, entry.err
		}
		return value, present, err
	}
	raw, present := p.dict[key]
	if !present {
		return cache([4]float64{}, false, nil)
	}
	resolved, ok := d.resolveIndirectChain(raw)
	if !ok {
		return cache([4]float64{}, true, fmt.Errorf("playa: %s could not be resolved", key))
	}
	a, ok := resolved.(Array)
	if !ok || len(a) != 4 {
		return cache([4]float64{}, true, fmt.Errorf("playa: %s is invalid", key))
	}
	var b [4]float64
	for i := 0; i < 4; i++ {
		item, resolved := d.resolveIndirectChain(a[i])
		if !resolved {
			return cache([4]float64{}, true, fmt.Errorf("playa: %s coordinate could not be resolved", key))
		}
		value, ok := NumberValue(item)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
			return cache([4]float64{}, true, fmt.Errorf("playa: %s is invalid", key))
		}
		b[i] = value
	}
	if b[0] > b[2] {
		b[0], b[2] = b[2], b[0]
	}
	if b[1] > b[3] {
		b[1], b[3] = b[3], b[1]
	}
	return cache(b, true, nil)
}

func (p Page) Resources(d *Document) Dict {
	resources, _ := p.ResourcesWithError(d)
	return resources
}

func (p Page) hasCachedResources(d *Document) bool {
	if d == nil || !p.hasPageCacheKey() {
		return false
	}
	_, ok := d.cachedPageResource(p.ref)
	return ok
}

// ResourcesWithError returns an independent page resource dictionary and
// reports malformed or unresolved indirect resources.
func (p Page) ResourcesWithError(d *Document) (Dict, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if p.hasPageCacheKey() {
		if err, ok := d.cachedPageResourceError(p.ref); ok {
			return nil, err
		}
		if resources, ok := d.cachedPageResource(p.ref); ok {
			return cloneDict(resources), nil
		}
	}
	cacheError := func(err error) (Dict, error) {
		if p.hasPageCacheKey() {
			err = d.storePageResourceError(p.ref, err)
		}
		return nil, err
	}
	raw, present := p.dict[Name("Resources")]
	if !present {
		return nil, nil
	}
	value, ok := d.resolveIndirectChain(raw)
	if !ok {
		return cacheError(fmt.Errorf("playa: Resources could not be resolved"))
	}
	res, ok := value.(Dict)
	if !ok {
		return cacheError(fmt.Errorf("playa: Resources is not a dictionary"))
	}
	if err := d.validateResourceEntries(res); err != nil {
		return cacheError(err)
	}
	if p.hasPageCacheKey() {
		d.storePageResource(p.ref, res)
	}
	return cloneDict(res), nil
}

func (p Page) UserUnit(d *Document) float64 {
	n, err := p.UserUnitWithError(d)
	if err != nil {
		return 1
	}
	return n
}

// UserUnitWithError resolves the page user-space scale and reports malformed
// or unresolved explicit values.
func (p Page) UserUnitWithError(d *Document) (float64, error) {
	if d == nil {
		return 0, errNilDocument
	}
	if p.hasPageCacheKey() {
		if err, ok := d.cachedPageUserUnitError(p.ref); ok {
			return 0, err
		}
		if value, ok := d.cachedPageUserUnit(p.ref); ok {
			return value, nil
		}
	}
	raw, present := p.dict[Name("UserUnit")]
	if !present {
		if p.hasPageCacheKey() {
			d.storePageUserUnit(p.ref, 1)
		}
		return 1, nil
	}
	value, resolved := d.resolveIndirectChain(raw)
	if !resolved {
		err := fmt.Errorf("playa: UserUnit could not be resolved")
		if p.hasPageCacheKey() {
			err = d.storePageUserUnitError(p.ref, err)
		}
		return 0, err
	}
	n, ok := finiteNumberValue(value)
	if !ok || n <= 0 {
		err := fmt.Errorf("playa: UserUnit is invalid")
		if p.hasPageCacheKey() {
			err = d.storePageUserUnitError(p.ref, err)
		}
		return 0, err
	}
	if p.hasPageCacheKey() {
		d.storePageUserUnit(p.ref, n)
	}
	return n, nil
}

// Size returns page-space width and height from the MediaBox and UserUnit.
// Rotation is exposed separately and applied by Matrix for content geometry.
func (p Page) Size(d *Document) (float64, float64) {
	w, h, err := p.SizeWithError(d)
	if err != nil {
		b := p.mediaBox(d)
		unit := p.UserUnit(d)
		w, h := (b[2]-b[0])*unit, (b[3]-b[1])*unit
		if math.IsNaN(w) || math.IsInf(w, 0) {
			w = 0
		}
		if math.IsNaN(h) || math.IsInf(h, 0) {
			h = 0
		}
		return w, h
	}
	return w, h
}

// SizeWithError returns the page dimensions and reports malformed explicit
// MediaBox or UserUnit values.
func (p Page) SizeWithError(d *Document) (float64, float64, error) {
	b, err := p.MediaBoxWithError(d)
	if err != nil {
		return 0, 0, err
	}
	unit, err := p.UserUnitWithError(d)
	if err != nil {
		return 0, 0, err
	}
	w, h := (b[2]-b[0])*unit, (b[3]-b[1])*unit
	if math.IsNaN(w) || math.IsInf(w, 0) {
		return 0, 0, fmt.Errorf("playa: page width is invalid")
	}
	if math.IsNaN(h) || math.IsInf(h, 0) {
		return 0, 0, fmt.Errorf("playa: page height is invalid")
	}
	return w, h, nil
}

// Width returns the page width in default user-space units.
func (p Page) Width(d *Document) float64 {
	w, _ := p.Size(d)
	return w
}

// Height returns the page height in default user-space units.
func (p Page) Height(d *Document) float64 {
	_, h := p.Size(d)
	return h
}

// Rotate returns the normalized page rotation in degrees.
func (p Page) Rotate(d *Document) int { return p.Rotation(d) }

// ParentKey returns the page's structure parent-tree key when present.
func (p Page) ParentKey(d *Document) (int, bool) {
	key, present, err := p.ParentKeyWithError(d)
	return key, present && err == nil
}

// ParentKeyWithError resolves StructParents and reports malformed explicit
// values while distinguishing an absent key from a valid zero key.
func (p Page) ParentKeyWithError(d *Document) (int, bool, error) {
	if d == nil {
		return 0, false, errNilDocument
	}
	raw, present := p.dict[Name("StructParents")]
	if !present {
		return 0, false, nil
	}
	resolved, ok := d.resolveIndirectChain(raw)
	if !ok {
		return 0, true, fmt.Errorf("playa: StructParents could not be resolved")
	}
	value, ok := IntValue(resolved)
	if !ok || value < 0 {
		return 0, true, fmt.Errorf("playa: StructParents is invalid")
	}
	return value, true, nil
}

// StructureSeq maps this page's StructParents ParentTree slots to structure
// elements. Empty slots are yielded with a nil Element.
func (p Page) StructureSeq(d *Document) iter.Seq2[PageStructureEntry, error] {
	if d == nil {
		return func(yield func(PageStructureEntry, error) bool) { yield(PageStructureEntry{}, errNilDocument) }
	}
	return p.structureSeq(d)
}

// Fonts returns the page resource fonts, including fonts reachable through
// Form XObjects. It is the Go mapping equivalent of Playa's page.fonts.
func (p Page) Fonts(d *Document) (map[string]*Font, error) {
	if d == nil {
		return nil, errNilDocument
	}
	fonts, err := d.pageFontsExpanded(p)
	if err != nil {
		return nil, err
	}
	return cloneFontMap(fonts), nil
}

// FontsSeq yields the page's direct font resources without materializing the
// mapping or resolving fonts that the consumer does not request.
func (p Page) FontsSeq(d *Document) iter.Seq2[FontResource, error] {
	if d == nil {
		return func(yield func(FontResource, error) bool) { yield(FontResource{}, errNilDocument) }
	}
	return d.PageFontSeq(p)
}
func (p Page) Rotation(d *Document) int {
	n, err := p.RotationWithError(d)
	if err != nil {
		return 0
	}
	return n
}

// RotationWithError resolves and normalizes the explicit page rotation.
func (p Page) RotationWithError(d *Document) (int, error) {
	if p.hasInitialRotation {
		return p.initialRotation, nil
	}
	if d == nil {
		return 0, errNilDocument
	}
	if p.hasPageCacheKey() {
		if err, ok := d.cachedPageRotationError(p.ref); ok {
			return 0, err
		}
		if value, ok := d.cachedPageRotation(p.ref); ok {
			return value, nil
		}
	}
	raw, present := p.dict[Name("Rotate")]
	if !present {
		if p.hasPageCacheKey() {
			d.storePageRotation(p.ref, 0)
		}
		return 0, nil
	}
	rotateValue, resolved := d.resolveIndirectChain(raw)
	if !resolved {
		err := fmt.Errorf("playa: Rotate could not be resolved")
		if p.hasPageCacheKey() {
			err = d.storePageRotationError(p.ref, err)
		}
		return 0, err
	}
	n, ok := IntValue(rotateValue)
	if !ok {
		err := fmt.Errorf("playa: Rotate is invalid")
		if p.hasPageCacheKey() {
			err = d.storePageRotationError(p.ref, err)
		}
		return 0, err
	}
	n %= 360
	if n < 0 {
		n += 360
	}
	if p.hasPageCacheKey() {
		d.storePageRotation(p.ref, n)
	}
	return n, nil
}

// Matrix returns the page-space transform used by Playa-style geometry. It
// translates the selected page box to the origin and applies page rotation.
func (p Page) Matrix(d *Document) geometry.Matrix {
	m, _ := p.MatrixInWithError(d, p.coordinateSpace(d))
	return m
}

func (p Page) coordinateSpace(d *Document) coordinates.Space {
	if p.hasContentMatrix {
		return p.space
	}
	if d != nil && d.space != "" {
		return d.CoordinateSpace()
	}
	if p.space != "" {
		return p.space
	}
	return CoordinateSpacePage
}

// MatrixIn returns the page transform for the requested coordinate space.
// Page preserves the historical bottom-left, box-normalized behavior. Screen
// flips the resulting page space vertically into a top-left origin. Default
// and User expose the PDF's untransformed user coordinates.
func (p Page) MatrixIn(d *Document, space coordinates.Space) geometry.Matrix {
	m, err := p.MatrixInWithError(d, space)
	if err != nil {
		return identity()
	}
	return m
}

// MatrixInWithError returns the page transform and reports malformed page
// geometry instead of silently falling back to identity.
func (p Page) MatrixInWithError(d *Document, space coordinates.Space) (geometry.Matrix, error) {
	if p.hasContentMatrix && space == p.space {
		return p.contentMatrix, nil
	}
	b, err := p.MediaBoxWithError(d)
	if err != nil {
		return geometry.Matrix{}, err
	}
	unit, err := p.UserUnitWithError(d)
	if err != nil {
		return geometry.Matrix{}, err
	}
	rotation, err := p.RotationWithError(d)
	if err != nil {
		return geometry.Matrix{}, err
	}
	minX, minY, maxX, maxY := b[0], b[1], b[2], b[3]
	scale := func(m geometry.Matrix) (geometry.Matrix, bool) {
		m[0] *= unit
		m[1] *= unit
		m[2] *= unit
		m[3] *= unit
		m[4] *= unit
		m[5] *= unit
		for _, value := range m {
			if !cffFiniteValues(value) {
				return geometry.Matrix{}, false
			}
		}
		return m, true
	}
	var matrix geometry.Matrix
	switch rotation {
	case 90:
		matrix = geometry.Matrix{0, -1, 1, 0, -minY, maxX}
	case 180:
		matrix = geometry.Matrix{-1, 0, 0, -1, maxX, maxY}
	case 270:
		matrix = geometry.Matrix{0, 1, -1, 0, maxY, -minX}
	default:
		matrix = geometry.Matrix{1, 0, 0, 1, -minX, -minY}
	}
	if space == CoordinateSpaceDefault || space == CoordinateSpaceUser {
		return geometry.Matrix{1, 0, 0, 1, 0, 0}, nil
	}
	var ok bool
	matrix, ok = scale(matrix)
	if !ok {
		return geometry.Matrix{}, fmt.Errorf("playa: page matrix is invalid")
	}
	if space != CoordinateSpaceScreen {
		return matrix, nil
	}
	height := (maxY - minY) * unit
	if rotation == 90 || rotation == 270 {
		height = (maxX - minX) * unit
	}
	screen, ok := geometry.Matrix{1, 0, 0, -1, 0, height}.MulFinite(matrix)
	if !ok {
		return geometry.Matrix{}, fmt.Errorf("playa: screen matrix is invalid")
	}
	return screen, nil
}

// SetInitialCTM updates the page's initial user-to-device transform, matching
// Playa's Page.set_initial_ctm. The document is required to resolve inherited
// or indirect page geometry; malformed geometry falls back to identity.
func (p *Page) SetInitialCTM(d *Document, space coordinates.Space, rotate int) geometry.Matrix {
	matrix, err := p.SetInitialCTMWithError(d, space, rotate)
	if err != nil {
		return identity()
	}
	return matrix
}

// SetInitialCTMWithError is the error-reporting form of SetInitialCTM.
func (p *Page) SetInitialCTMWithError(d *Document, space coordinates.Space, rotate int) (geometry.Matrix, error) {
	box, err := p.MediaBoxWithError(d)
	if err != nil {
		return geometry.Matrix{}, err
	}
	rotate %= 360
	if rotate < 0 {
		rotate += 360
	}

	matrix := identity()
	width := box[2] - box[0]
	height := box[3] - box[1]
	switch rotate {
	case 90:
		matrix = geometry.Matrix{0, -1, 1, 0, 0, width}
	case 180:
		matrix = geometry.Matrix{-1, 0, 0, -1, width, height}
	case 270:
		matrix = geometry.Matrix{0, 1, -1, 0, height, 0}
	}
	transformed, ok := transformBBox(matrix, box)
	if !ok {
		return geometry.Matrix{}, fmt.Errorf("playa: initial page matrix is invalid")
	}

	switch space {
	case coordinates.Screen:
		matrix, ok = geometry.Matrix{1, 0, 0, -1, -transformed[0], transformed[3]}.MulFinite(matrix)
	case coordinates.Page:
		matrix, ok = geometry.Matrix{1, 0, 0, 1, -transformed[0], -transformed[1]}.MulFinite(matrix)
	case coordinates.Default, coordinates.User:
		matrix = identity()
		ok = true
	default:
		// Playa treats unknown device spaces like default user space.
		matrix = identity()
		ok = true
	}
	if !ok {
		return geometry.Matrix{}, fmt.Errorf("playa: initial page matrix is invalid")
	}

	p.space = space
	p.contentMatrix = matrix
	p.hasContentMatrix = true
	p.initialRotation = rotate
	p.hasInitialRotation = true
	return matrix, nil
}

func pageMatrixOps(p Page, d *Document, ops []ContentOp) []ContentOp {
	m := p.MatrixIn(d, p.coordinateSpace(d))
	if m == identity() {
		return ops
	}
	operands := make([]Object, 6)
	for i, value := range m {
		operands[i] = Number(value)
	}
	out := make([]ContentOp, 0, len(ops)+1)
	out = append(out, newContentOpBorrowed("cm", operands, 0))
	return append(out, ops...)
}

func (d *Document) Resolve(o Object) Object {
	return cloneGraphicsObject(d.resolveRaw(o))
}

// resolveRaw resolves an indirect reference without copying the document-owned
// object. Internal parsing paths use this helper to avoid copying nested PDF
// containers; public callers should use Resolve for an isolated snapshot.
func (d *Document) resolveRaw(o Object) Object {
	if r, ok := o.(Ref); ok {
		if v, err := d.resolveRefRaw(r); err == nil {
			return v
		}
	}
	return o
}

// resolveIndirectValue follows an indirect reference chain for internal
// value parsing while retaining the document-owned object graph.
func (d *Document) resolveIndirectValue(o Object) Object {
	value, _ := d.resolveIndirectChain(o)
	return value
}

// Lookup returns the newest generation of an indirect object number. This is
// the object-number view used by Playa's mapping-style document API; callers
// that need an exact generation can continue to use ResolveRef.
func (d *Document) Lookup(objectNumber int) (Ref, Object, bool) {
	ref, value, err := d.LookupWithError(objectNumber)
	return ref, value, err == nil
}

// LookupWithError returns the newest generation of an indirect object number.
// Missing object numbers return ErrObjectNotFound; malformed or unresolved
// objects return their original resolution error.
func (d *Document) LookupWithError(objectNumber int) (Ref, Object, error) {
	ref, value, err := d.lookupRawWithError(objectNumber)
	if err != nil {
		return Ref{}, nil, err
	}
	value, err = d.publicObjectValue(ref, value)
	if err != nil {
		return Ref{}, nil, err
	}
	return ref, cloneGraphicsObject(value), nil
}

// lookupRawWithError is the borrowed object-number lookup used by internal
// parsing paths. It follows Playa's generation-insensitive ObjRef semantics
// without cloning potentially large stream payloads.
func (d *Document) lookupRawWithError(objectNumber int) (Ref, Object, error) {
	if d == nil {
		return Ref{}, nil, errNilDocument
	}
	d.cacheMu.RLock()
	cached, ok := d.lookupFound[objectNumber]
	ref := d.lookupCache[objectNumber]
	indexed := d.lookupIndexReady
	indexDisabled := d.lookupIndexDisabled
	d.cacheMu.RUnlock()
	if ok {
		if !cached {
			return Ref{}, nil, ErrObjectNotFound
		}
		value, err := d.resolveRefRaw(ref)
		return ref, value, err
	}
	if indexed {
		return Ref{}, nil, ErrObjectNotFound
	}
	if !indexDisabled {
		d.buildLookupIndex()
		d.cacheMu.RLock()
		cached, ok = d.lookupFound[objectNumber]
		ref = d.lookupCache[objectNumber]
		indexed = d.lookupIndexReady
		d.cacheMu.RUnlock()
		if ok && cached {
			value, err := d.resolveRefRaw(ref)
			return ref, value, err
		}
		if indexed {
			return Ref{}, nil, ErrObjectNotFound
		}
	}
	var best Ref
	found := false
	freeRefs := map[Ref]bool{}
	d.cacheMu.RLock()
	for ref, entry := range d.xrefs {
		if ref.Object == objectNumber && entry.isFree() {
			freeRefs[ref] = true
		}
		if ref.Object == objectNumber && !entry.isFree() && (!found || ref.Generation > best.Generation) {
			best, found = ref, true
		}
	}
	for ref, entry := range d.xrefHistory {
		if _, current := d.xrefs[ref]; current {
			continue
		}
		if ref.Object == objectNumber && entry.isFree() {
			freeRefs[ref] = true
		}
		if ref.Object == objectNumber && !entry.isFree() && (!found || ref.Generation > best.Generation) {
			best, found = ref, true
		}
	}
	for ref := range d.objects {
		if ref.Object == objectNumber && !freeRefs[ref] && (!found || ref.Generation > best.Generation) {
			best, found = ref, true
		}
	}
	d.cacheMu.RUnlock()
	if !found {
		d.cacheLookup(objectNumber, Ref{}, false)
		return Ref{}, nil, ErrObjectNotFound
	}
	value, err := d.resolveRefRaw(best)
	if err != nil {
		return Ref{}, nil, err
	}
	d.cacheLookup(objectNumber, best, true)
	return best, value, nil
}

func (d *Document) ResolveRef(ref Ref) (Object, error) {
	value, err := d.resolveRefRaw(ref)
	if err != nil {
		return nil, err
	}
	value, err = d.publicObjectValue(ref, value)
	return cloneGraphicsObject(value), err
}

func (d *Document) publicObjectValue(ref Ref, value Object) (Object, error) {
	if !d.encrypted {
		return value, nil
	}
	d.cacheMu.RLock()
	decrypted := d.decrypted[ref]
	d.cacheMu.RUnlock()
	if decrypted {
		return value, nil
	}
	return d.decryptObjectWithMetadataContextWithError(ref, value, d.isCatalogRef(ref))
}

// resolveRefRaw returns the document-owned object for internal parsing paths.
// Public callers must use ResolveRef, which returns an isolated snapshot.
func (d *Document) resolveRefRaw(ref Ref) (value Object, err error) {
	d.cacheMu.RLock()
	cachedErr, cached := d.objectErrors[ref]
	d.cacheMu.RUnlock()
	if cached {
		return nil, cachedErr
	}
	defer func() {
		if err == nil {
			return
		}
		d.cacheMu.Lock()
		errorBytes := len(err.Error())
		if errorBytes <= d.cacheLimits().ObjectErrorBytes && len(d.objectErrors) < objectErrorCacheLimit && cacheFits(d.objectErrorBytes, errorBytes, d.cacheLimits().ObjectErrorBytes) {
			if d.objectErrors == nil {
				d.objectErrors = map[Ref]error{}
			}
			if _, exists := d.objectErrors[ref]; !exists {
				d.objectErrors[ref] = err
				d.objectErrorBytes += errorBytes
			}
		}
		d.cacheMu.Unlock()
	}()
	defer d.maybeEvictObjectCache()
	d.cacheMu.RLock()
	value, ok := d.objects[ref]
	decrypted := d.decrypted[ref]
	entry, entryOK := d.xrefs[ref]
	if !entryOK {
		entry, entryOK = d.xrefHistory[ref]
	}
	d.cacheMu.RUnlock()
	if ok {
		if d.encrypted && !decrypted && !d.isEncryptionObject(ref, value) {
			decryptedValue, err := d.decryptObjectWithMetadataContextWithError(ref, value, d.isCatalogRef(ref))
			if err != nil {
				return nil, err
			}
			value = decryptedValue
			d.cacheMu.Lock()
			d.storeObjectLocked(ref, value)
			if d.decrypted == nil {
				d.decrypted = map[Ref]bool{}
			}
			d.decrypted[ref] = true
			d.cacheMu.Unlock()
		}
		return value, nil
	}
	if !entryOK || entry.isFree() {
		return nil, fmt.Errorf("playa: object %s not found", ref.String())
	}
	if entry.objectStream() > 0 {
		if err := d.loadObjectStream(d.xrefs, entry.objectStream()); err != nil {
			return nil, err
		}
		d.cacheMu.RLock()
		value, ok := d.objects[ref]
		d.cacheMu.RUnlock()
		if ok {
			return value, nil
		}
		return nil, fmt.Errorf("playa: object %s not found in object stream", ref.String())
	}
	if entry.offset < 0 || entry.offset >= len(d.data) {
		return nil, fmt.Errorf("playa: object %s has invalid offset", ref.String())
	}
	objectOffset := xrefObjectOffset(d.data, entry.offset)
	headerMatch := objRE.FindIndex(d.data[objectOffset:])
	if headerMatch == nil || headerMatch[0] != 0 {
		return nil, fmt.Errorf("playa: object %s header not found", ref.String())
	}
	objectNumber, _, headerOK := parseScannedObjectHeader(d.data[objectOffset : objectOffset+headerMatch[1]])
	if !headerOK || objectNumber != ref.Object {
		return nil, fmt.Errorf("playa: object %s header does not match xref", ref.String())
	}
	body, ok := indirectBody(d.data, objectOffset, d.resolveRaw)
	if !ok {
		return nil, fmt.Errorf("playa: object %s body not found", ref.String())
	}
	header := bytes.Index(body, []byte("obj"))
	if header < 0 {
		return nil, fmt.Errorf("playa: object %s header not found", ref.String())
	}
	value, err = parseObjectBodyBorrowedWithResolver(bytes.TrimSpace(body[header+len("obj"):]), d.resolveRaw)
	if err != nil {
		return nil, wrapObjectParseError(err, ref, entry.offset, "resolve object")
	}
	if d.encrypted && !d.isEncryptionObject(ref, value) {
		decryptedValue, err := d.decryptObjectWithMetadataContextWithError(ref, value, d.isCatalogRef(ref))
		if err != nil {
			return nil, err
		}
		value = decryptedValue
	}
	d.cacheMu.Lock()
	if cached, ok := d.objects[ref]; ok {
		d.cacheMu.Unlock()
		return cached, nil
	}
	if d.decrypted == nil {
		d.decrypted = map[Ref]bool{}
	}
	if d.encrypted {
		d.decrypted[ref] = true
	}
	d.storeObjectLocked(ref, value)
	d.cacheMu.Unlock()
	return value, nil
}

func (d *Document) ResolveObject(o Object) (Object, error) {
	if ref, ok := o.(Ref); ok {
		value, err := d.resolveRefRaw(ref)
		return cloneGraphicsObject(value), err
	}
	if o == nil {
		return nil, fmt.Errorf("playa: nil object")
	}
	return cloneGraphicsObject(o), nil
}

// Pages returns document pages in source page-tree order.
//
// Errors discovered while advancing are yielded once and stop the sequence.
// The sequence can be traversed more than once and stops as soon as the
// consumer stops yielding.
func (d *Document) Pages() iter.Seq2[Page, error] {
	return d.pagesSeq(true)
}

// pagesNoCache yields borrowed page views without retaining page dictionaries
// or the complete page list. It is used by bounded one-pass workers; the
// public Pages sequence keeps its repeatable cache semantics.
func (d *Document) pagesNoCache() iter.Seq2[Page, error] {
	return d.pagesSeq(false)
}

func (d *Document) pagesSeq(cachePages bool) iter.Seq2[Page, error] {
	if d == nil {
		return func(yield func(Page, error) bool) {
			yield(Page{}, errNilDocument)
		}
	}
	return func(yield func(Page, error) bool) {
		if cachePages {
			d.pageBuildMu.Lock()
			d.cacheMu.RLock()
			ready := d.pagesReady
			cachedPages := d.pagesCache
			cachedErr := d.pagesErr
			errReady := d.pagesErrReady
			d.cacheMu.RUnlock()
			d.pageBuildMu.Unlock()
			if errReady {
				yield(Page{}, cachedErr)
				return
			}
			if ready {
				for _, page := range cachedPages {
					if !yield(page, nil) {
						return
					}
				}
				return
			}
		}
		var cached []Page
		cachedBytes := 0
		cacheable := cachePages
		reportError := func(err error) bool {
			if cachePages {
				d.cacheMu.Lock()
				if !d.pagesErrReady {
					d.pagesErr = err
					d.pagesErrReady = true
				}
				d.cacheMu.Unlock()
			}
			return yield(Page{}, err)
		}
		cachePage := func(page Page, err error) bool {
			if err != nil {
				reportError(err)
				return false
			}
			if cacheable {
				pageBytes := pageCacheSize(page)
				if !cacheFits(cachedBytes, pageBytes, d.cacheLimits().PagesBytes) {
					cacheable = false
					cached = nil
					cachedBytes = 0
				} else {
					cachedBytes = addCacheSize(cachedBytes, pageBytes)
					cached = append(cached, page)
				}
			}
			return yield(page, nil)
		}
		cacheComplete := func() {
			if !cacheable {
				return
			}
			d.pageBuildMu.Lock()
			d.cacheMu.Lock()
			if !d.pagesReady {
				d.pagesCache = cached
				d.pagesCacheBytes = cachedBytes
				d.pagesReady = true
			}
			d.cacheMu.Unlock()
			d.pageBuildMu.Unlock()
		}
		root, ok := d.trailer[Name("Root")]
		if !ok {
			for page, err := range d.scanPagesSeq(cachePages) {
				if !cachePage(page, err) {
					return
				}
			}
			cacheComplete()
			return
		}
		catalogValue, ok := d.resolveIndirectChain(root)
		if !ok {
			reportError(fmt.Errorf("playa: catalog is not dictionary"))
			return
		}
		catalog, ok := catalogValue.(Dict)
		if !ok {
			reportError(fmt.Errorf("playa: catalog is not dictionary"))
			return
		}
		ptr, ok := catalog[Name("Pages")]
		if !ok {
			for page, err := range d.scanPagesSeq(cachePages) {
				if !cachePage(page, err) {
					return
				}
			}
			cacheComplete()
			return
		}
		number := 0
		if d.walkPagesSeq(ptr, nil, Ref{}, fontOrderPathNone, map[Ref]bool{}, &number, cachePages, cachePage) {
			cacheComplete()
		}
	}
}

// CollectPages materializes Pages for adapters that need random access.
func (d *Document) CollectPages() ([]Page, error) {
	out := []Page{}
	for page, err := range d.Pages() {
		if err != nil {
			return nil, err
		}
		out = append(out, page)
	}
	return out, nil
}

func (d *Document) countPages() (int, error) {
	root, ok := d.trailer[Name("Root")]
	if !ok {
		return d.countPagesFromSequence()
	}
	catalogValue, ok := d.resolveIndirectChain(root)
	if !ok {
		return 0, fmt.Errorf("playa: catalog is not dictionary")
	}
	catalog, ok := catalogValue.(Dict)
	if !ok {
		return 0, fmt.Errorf("playa: catalog is not dictionary")
	}
	pages, ok := catalog[Name("Pages")]
	if !ok {
		return d.countPagesFromSequence()
	}
	return d.countPageTree(pages, map[Ref]bool{})
}

func (d *Document) countPagesFromSequence() (int, error) {
	count := 0
	for _, err := range d.pagesNoCache() {
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func (d *Document) countPageTree(o Object, seen map[Ref]bool) (int, error) {
	if ref, ok := d.finalIndirectRef(o); ok {
		if seen[ref] {
			return 0, fmt.Errorf("playa: page tree cycle at %s", ref.String())
		}
		seen[ref] = true
	}
	value, resolved := d.resolveIndirectChain(o)
	if !resolved {
		return 0, fmt.Errorf("playa: page node is not dictionary")
	}
	v, ok := value.(Dict)
	if !ok {
		return 0, fmt.Errorf("playa: page node is not dictionary")
	}
	typeRaw, typePresent := v[Name("Type")]
	typeValue, typeResolved := d.resolveIndirectChain(typeRaw)
	if typePresent && !typeResolved {
		return 0, fmt.Errorf("playa: page node Type could not be resolved")
	}
	typ, _ := typeValue.(Name)
	if typ == Name("Page") {
		return 1, nil
	}
	if typePresent && typ != Name("Pages") {
		return 0, fmt.Errorf("playa: page node has invalid Type")
	}
	kidsValue, resolved := d.resolveIndirectChain(v[Name("Kids")])
	kids, ok := kidsValue.(Array)
	if !resolved {
		ok = false
	}
	if !ok {
		return 0, fmt.Errorf("playa: page node has no Kids")
	}
	count := 0
	for _, kid := range kids {
		childCount, err := d.countPageTree(kid, seen)
		if err != nil {
			return 0, err
		}
		count += childCount
	}
	return count, nil
}

// PageAt returns the zero-based page at index without materializing later
// pages.
func (d *Document) PageAt(index int) (Page, error) {
	if index < 0 {
		return Page{}, ErrPageNotFound
	}
	current := 0
	for page, err := range d.Pages() {
		if err != nil {
			return Page{}, err
		}
		if current == index {
			return page, nil
		}
		current++
	}
	return Page{}, ErrPageNotFound
}

// PageByLabel returns the first page whose human-facing label matches label.
func (d *Document) PageByLabel(label string) (Page, error) {
	nextLabel, stop := iter.Pull2(d.PageLabelsSeq())
	defer stop()
	for page, err := range d.Pages() {
		if err != nil {
			return Page{}, err
		}
		pageLabel, labelErr, ok := nextLabel()
		if !ok {
			break
		}
		if labelErr != nil {
			return Page{}, labelErr
		}
		if pageLabel == label {
			return page, nil
		}
	}
	return Page{}, ErrPageNotFound
}

// PageByRef returns the page whose indirect object reference is ref.
func (d *Document) PageByRef(ref Ref) (Page, error) {
	d.cacheMu.RLock()
	if page, ok := d.pageCache[ref]; ok {
		d.cacheMu.RUnlock()
		return page, nil
	}
	if d.pageMissing[ref] {
		d.cacheMu.RUnlock()
		return Page{}, ErrPageNotFound
	}
	d.cacheMu.RUnlock()
	for page, err := range d.Pages() {
		if err != nil {
			return Page{}, err
		}
		if page.ref == ref {
			return page, nil
		}
	}
	d.cacheMissingPage(ref)
	return Page{}, ErrPageNotFound
}

func (d *Document) walkPagesSeq(o Object, parent Dict, inheritedFontOrderRef Ref, inheritedFontOrderPath uint8, seen map[Ref]bool, number *int, cachePages bool, yield func(Page, error) bool) bool {
	if ref, ok := d.finalIndirectRef(o); ok {
		if seen[ref] {
			yield(Page{}, fmt.Errorf("playa: page tree cycle at %s", ref.String()))
			return false
		}
		seen[ref] = true
	}
	value, resolved := d.resolveIndirectChain(o)
	if !resolved {
		yield(Page{}, fmt.Errorf("playa: page node is not dictionary"))
		return false
	}
	v, ok := value.(Dict)
	if !ok {
		yield(Page{}, fmt.Errorf("playa: page node is not dictionary"))
		return false
	}
	merged := mergePageDict(parent, v)
	fontOrderRef, fontOrderPath := inheritedFontOrderRef, inheritedFontOrderPath
	if resources, present := v[Name("Resources")]; present {
		fontOrderRef, fontOrderPath = d.fontResourceOrderSource(o, resources)
	}
	typeRaw, typePresent := merged[Name("Type")]
	typeValue, typeResolved := d.resolveIndirectChain(typeRaw)
	if typePresent && !typeResolved {
		yield(Page{}, fmt.Errorf("playa: page node Type could not be resolved"))
		return false
	}
	typ, _ := typeValue.(Name)
	if typ == Name("Page") {
		*number++
		var pageRef Ref
		if ref, ok := o.(Ref); ok {
			pageRef = ref
		}
		lastModifiedValue, _ := d.resolveIndirectChain(merged[Name("LastModified")])
		lastModified, _ := lastModifiedValue.(String)
		page := Page{number: *number, index: *number - 1, space: d.CoordinateSpace(), ref: pageRef, dict: merged, lastModified: decodePDFText(lastModified), fontOrderRef: fontOrderRef, fontOrderPath: fontOrderPath}
		if cachePages && pageRef != (Ref{}) {
			d.cachePage(pageRef, page)
		}
		return yield(page, nil)
	}
	if typePresent && typ != Name("Pages") {
		yield(Page{}, fmt.Errorf("playa: page node has invalid Type"))
		return false
	}
	kidsValue, resolved := d.resolveIndirectChain(merged[Name("Kids")])
	kids, ok := kidsValue.(Array)
	if !resolved {
		ok = false
	}
	if !ok {
		yield(Page{}, fmt.Errorf("playa: page node has no Kids"))
		return false
	}
	for _, k := range kids {
		if !d.walkPagesSeq(k, merged, fontOrderRef, fontOrderPath, seen, number, cachePages, yield) {
			return false
		}
	}
	return true
}

// pagesReverseSeq yields pages from the end of the page tree without first
// materializing the complete page list. Page numbers are intentionally left
// unset because this traversal is used only for document-level resources.
func (d *Document) pagesReverseSeq() iter.Seq2[Page, error] {
	if d == nil {
		return func(yield func(Page, error) bool) {
			yield(Page{}, errNilDocument)
		}
	}
	return func(yield func(Page, error) bool) {
		root, ok := d.trailer[Name("Root")]
		if !ok {
			refs := make([]Ref, 0, len(d.objects))
			for ref, object := range d.objects {
				if page, ok := object.(Dict); ok {
					typeRaw, present := page[Name("Type")]
					typeValue, resolved := d.resolveIndirectChain(typeRaw)
					if present && !resolved {
						yield(Page{}, fmt.Errorf("playa: page Type could not be resolved"))
						return
					}
					typ, isName := typeValue.(Name)
					if present && !isName {
						yield(Page{}, fmt.Errorf("playa: page Type is invalid"))
						return
					}
					if typ == Name("Page") {
						refs = append(refs, ref)
					}
				}
			}
			sort.Slice(refs, func(i, j int) bool {
				if refs[i].Object != refs[j].Object {
					return refs[i].Object < refs[j].Object
				}
				return refs[i].Generation < refs[j].Generation
			})
			for i := len(refs) - 1; i >= 0; i-- {
				page, _ := d.objects[refs[i]].(Dict)
				if !yield(Page{ref: refs[i], dict: page}, nil) {
					return
				}
			}
			return
		}
		catalogValue, ok := d.resolveIndirectChain(root)
		if !ok {
			yield(Page{}, fmt.Errorf("playa: catalog is not dictionary"))
			return
		}
		catalog, ok := catalogValue.(Dict)
		if !ok {
			yield(Page{}, fmt.Errorf("playa: catalog is not dictionary"))
			return
		}
		pages, ok := catalog[Name("Pages")]
		if !ok {
			for page, err := range d.pagesReverseScanSeq() {
				if err != nil || !yield(page, err) {
					return
				}
			}
			return
		}
		d.walkPagesReverseSeq(pages, nil, Ref{}, fontOrderPathNone, map[Ref]bool{}, yield)
	}
}

func (d *Document) pagesReverseScanSeq() iter.Seq2[Page, error] {
	return func(yield func(Page, error) bool) {
		refs := make([]Ref, 0, len(d.objects))
		for ref, object := range d.objects {
			if page, ok := object.(Dict); ok {
				typeRaw, present := page[Name("Type")]
				typeValue, resolved := d.resolveIndirectChain(typeRaw)
				if present && !resolved {
					yield(Page{}, fmt.Errorf("playa: page Type could not be resolved"))
					return
				}
				typ, isName := typeValue.(Name)
				if present && !isName {
					yield(Page{}, fmt.Errorf("playa: page Type is invalid"))
					return
				}
				if typ == Name("Page") {
					refs = append(refs, ref)
				}
			}
		}
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].Object != refs[j].Object {
				return refs[i].Object < refs[j].Object
			}
			return refs[i].Generation < refs[j].Generation
		})
		for i := len(refs) - 1; i >= 0; i-- {
			page, _ := d.objects[refs[i]].(Dict)
			if !yield(Page{ref: refs[i], dict: page}, nil) {
				return
			}
		}
	}
}

func (d *Document) walkPagesReverseSeq(o Object, parent Dict, inheritedFontOrderRef Ref, inheritedFontOrderPath uint8, seen map[Ref]bool, yield func(Page, error) bool) bool {
	if ref, ok := d.finalIndirectRef(o); ok {
		if seen[ref] {
			yield(Page{}, fmt.Errorf("playa: page tree cycle at %s", ref.String()))
			return false
		}
		seen[ref] = true
	}
	value, resolved := d.resolveIndirectChain(o)
	if !resolved {
		yield(Page{}, fmt.Errorf("playa: page node is not dictionary"))
		return false
	}
	v, ok := value.(Dict)
	if !ok {
		yield(Page{}, fmt.Errorf("playa: page node is not dictionary"))
		return false
	}
	merged := mergePageDict(parent, v)
	fontOrderRef, fontOrderPath := inheritedFontOrderRef, inheritedFontOrderPath
	if resources, present := v[Name("Resources")]; present {
		fontOrderRef, fontOrderPath = d.fontResourceOrderSource(o, resources)
	}
	typeRaw, typePresent := merged[Name("Type")]
	typeValue, typeResolved := d.resolveIndirectChain(typeRaw)
	if typePresent && !typeResolved {
		yield(Page{}, fmt.Errorf("playa: page node Type could not be resolved"))
		return false
	}
	typ, _ := typeValue.(Name)
	if typ == Name("Page") {
		var pageRef Ref
		if ref, ok := o.(Ref); ok {
			pageRef = ref
		}
		lastModifiedValue, _ := d.resolveIndirectChain(merged[Name("LastModified")])
		lastModified, _ := lastModifiedValue.(String)
		page := Page{ref: pageRef, dict: merged, lastModified: decodePDFText(lastModified), fontOrderRef: fontOrderRef, fontOrderPath: fontOrderPath}
		return yield(page, nil)
	}
	if typePresent && typ != Name("Pages") {
		yield(Page{}, fmt.Errorf("playa: page node has invalid Type"))
		return false
	}
	kidsValue, resolved := d.resolveIndirectChain(merged[Name("Kids")])
	kids, ok := kidsValue.(Array)
	if !resolved {
		ok = false
	}
	if !ok {
		yield(Page{}, fmt.Errorf("playa: page node has no Kids"))
		return false
	}
	for i := len(kids) - 1; i >= 0; i-- {
		if !d.walkPagesReverseSeq(kids[i], merged, fontOrderRef, fontOrderPath, seen, yield) {
			return false
		}
	}
	return true
}

func mergeDict(parent, child Dict) Dict {
	out := Dict{}
	for k, v := range parent {
		out[k] = v
	}
	for k, v := range child {
		out[k] = v
	}
	return out
}

// mergePageDict applies only the page-tree attributes that PDF defines as
// inheritable. Content, annotations, and tree bookkeeping belong to the
// concrete page or node and must never leak down from a /Pages dictionary.
func mergePageDict(parent, child Dict) Dict {
	out := Dict{}
	for _, key := range [...]Name{
		Name("Resources"),
		Name("MediaBox"),
		Name("CropBox"),
		Name("Rotate"),
		Name("UserUnit"),
	} {
		if value, ok := parent[key]; ok {
			out[key] = value
		}
	}
	for key, value := range child {
		out[key] = value
	}
	return out
}
func (d *Document) scanPages() []Page {
	out := []Page{}
	for page, err := range d.scanPagesSeq(true) {
		if err == nil {
			out = append(out, page)
		}
	}
	return out
}

func (d *Document) scanPagesSeq(cachePages bool) iter.Seq2[Page, error] {
	if d == nil {
		return func(yield func(Page, error) bool) {
			yield(Page{}, errNilDocument)
		}
	}
	return func(yield func(Page, error) bool) {
		var refs []Ref
		if len(d.xrefs) > 0 {
			refs = d.objectReferences()
		} else {
			refs = make([]Ref, 0, len(d.objects))
			for ref := range d.objects {
				refs = append(refs, ref)
			}
		}
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].Object != refs[j].Object {
				return refs[i].Object < refs[j].Object
			}
			return refs[i].Generation < refs[j].Generation
		})
		number := 0
		for _, ref := range refs {
			var o Object
			if len(d.xrefs) > 0 {
				var err error
				o, err = d.resolveRefRaw(ref)
				if err != nil {
					continue
				}
			} else {
				o = d.objects[ref]
			}
			if v, ok := o.(Dict); ok {
				typeRaw, present := v[Name("Type")]
				typeValue, resolved := d.resolveIndirectChain(typeRaw)
				if present && !resolved {
					yield(Page{}, fmt.Errorf("playa: page Type could not be resolved"))
					return
				}
				typ, isName := typeValue.(Name)
				if present && !isName {
					yield(Page{}, fmt.Errorf("playa: page Type is invalid"))
					return
				}
				if typ == Name("Page") {
					number++
					fontOrderRef, fontOrderPath := d.fontResourceOrderSource(ref, v[Name("Resources")])
					page := Page{number: number, index: number - 1, space: d.CoordinateSpace(), ref: ref, dict: v, fontOrderRef: fontOrderRef, fontOrderPath: fontOrderPath}
					if cachePages {
						d.cachePage(ref, page)
					}
					if !yield(page, nil) {
						return
					}
				}
			}
		}
	}
}
func (p Page) Content(d *Document) ([]byte, error) {
	if d == nil {
		return nil, errNilDocument
	}
	contents := p.dict[Name("Contents")]
	resolvedContents, resolved, resolveErr := d.resolveContentIndirectChain(contents)
	if resolveErr != nil {
		return nil, resolveErr
	}
	if contents != nil && !resolved {
		return nil, nil
	}
	var out []byte
	appendStream := func(value Object, stream Stream) error {
		data, err := decodeContentStream(d, value, stream)
		if err != nil {
			return err
		}
		out = append(out, data...)
		out = append(out, '\n')
		return nil
	}
	add := func(o Object) error {
		resolved, ok, err := d.resolveContentIndirectChain(o)
		if err != nil {
			return err
		}
		if o != nil && !ok {
			return nil
		}
		s, ok := resolved.(Stream)
		if !ok {
			return nil
		}
		return appendStream(o, s)
	}
	switch v := resolvedContents.(type) {
	case Stream:
		if err := appendStream(contents, v); err != nil {
			return nil, err
		}
	case Array:
		for _, x := range v {
			if e := add(x); e != nil {
				return nil, e
			}
		}
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("playa: page Contents is not a stream or array")
	}
	return out, nil
}

// Tokens returns the lexical tokens of this page's concatenated content
// streams, preserving offsets relative to the concatenated page content.
// streamsRaw returns resolved page content streams without copying payloads.
// It is reserved for internal parsing paths that do not expose the streams.
func (p Page) streamsRaw(d *Document) iter.Seq2[Stream, error] {
	return func(yield func(Stream, error) bool) {
		if d == nil {
			yield(Stream{}, errNilDocument)
			return
		}
		value, resolved, resolveErr := d.resolveContentIndirectChain(p.dict[Name("Contents")])
		if resolveErr != nil {
			yield(Stream{}, resolveErr)
			return
		}
		if p.dict[Name("Contents")] != nil && !resolved {
			return
		}
		switch contents := value.(type) {
		case Stream:
			if ref, ok := d.streamRef(p.dict[Name("Contents")]); ok {
				contents = streamWithRef(contents, ref)
			}
			yield(contents, nil)
		case Array:
			for _, item := range contents {
				resolved, ok, resolveErr := d.resolveContentIndirectChain(item)
				if resolveErr != nil {
					yield(Stream{}, resolveErr)
					return
				}
				if item != nil && !ok {
					continue
				}
				stream, ok := resolved.(Stream)
				if !ok {
					continue
				}
				if ref, ok := d.streamRef(item); ok {
					stream = streamWithRef(stream, ref)
				}
				if !yield(stream, nil) {
					return
				}
			}
		case nil:
			return
		default:
			yield(Stream{}, fmt.Errorf("playa: page Contents is not a stream or array"))
			return
		}
	}
}

func (d *Document) streamRef(value Object) (Ref, bool) {
	if d == nil {
		return Ref{}, false
	}
	seen := map[Ref]bool{}
	var last Ref
	for {
		ref, ok := value.(Ref)
		if !ok {
			return last, last != (Ref{})
		}
		if seen[ref] {
			return Ref{}, false
		}
		seen[ref] = true
		last = ref
		resolved, err := d.resolveRefRaw(ref)
		if err != nil {
			return Ref{}, false
		}
		value = resolved
	}
}

// Streams returns borrowed resolved page content streams in source order. Call
// Stream.Finalize when an independent copy is required.
func (p Page) Streams(d *Document) iter.Seq2[Stream, error] {
	return func(yield func(Stream, error) bool) {
		for stream, err := range p.streamsRaw(d) {
			if err != nil {
				if !yield(Stream{}, err) {
					return
				}
				return
			}
			if !yield(stream, nil) {
				return
			}
		}
	}
}

// Tokens returns lexical tokens from decoded page content streams in order.
func (p Page) Tokens(d *Document) iter.Seq2[Token, error] {
	return func(yield func(Token, error) bool) {
		if d == nil {
			yield(Token{}, errNilDocument)
			return
		}
		contents := p.dict[Name("Contents")]
		var values []Object
		resolvedValue, resolved, resolveErr := d.resolveContentIndirectChain(contents)
		if resolveErr != nil {
			yield(Token{}, resolveErr)
			return
		}
		if contents != nil && !resolved {
			return
		}
		values, typeErr := resolvedContentStreams(contents, resolvedValue)
		if typeErr != nil {
			yield(Token{}, typeErr)
			return
		}
		for _, value := range values {
			resolved, ok, resolveErr := d.resolveContentIndirectChain(value)
			if resolveErr != nil {
				yield(Token{}, resolveErr)
				return
			}
			if value != nil && !ok {
				continue
			}
			stream, ok := resolved.(Stream)
			if !ok {
				continue
			}
			data, err := decodeContentStream(d, value, stream)
			if err != nil {
				yield(Token{}, err)
				return
			}
			lexer := NewSourceLexer(data)
			for {
				token, err := lexer.Next()
				if err != nil {
					if recoverTrailingContentParseError(err) {
						break
					}
					yield(Token{}, err)
					return
				}
				if token.Kind() == TokenEOF {
					break
				}
				token = token.Finalize()
				if !yield(token, nil) {
					return
				}
			}
		}
	}
}

// CollectTokens materializes Tokens for callers that need a slice.
func (p Page) CollectTokens(d *Document) ([]Token, error) {
	out := []Token{}
	for token, err := range p.Tokens(d) {
		if err != nil {
			return nil, err
		}
		out = append(out, token)
	}
	return out, nil
}

// ContentOps parses the page's concatenated content streams without expanding
// Form XObjects. Use Document.PageContentOps when expanded content is needed.
func (p Page) ContentOps(d *Document) ([]ContentOp, error) {
	var out []ContentOp
	for op, err := range p.Contents(d) {
		if err != nil {
			return nil, err
		}
		out = append(out, op.Finalize())
	}
	return out, nil
}
