package document

import (
	"math"

	"github.com/lin-string/go-playa/fontdata"
)

const fontCacheLimit = 32 << 20
const cffPathCacheLimit = 4 << 20
const trueTypePathCacheLimit = 4 << 20
const fontErrorCacheLimit = 4096
const pagePropCacheLimit = 32 << 20
const pagePropErrorCacheLimit = 4096
const pageResourceErrorCacheLimit = 4096
const pageUserUnitCacheLimit = 4096
const pageUserUnitErrorCacheLimit = 4096
const pageRotationCacheLimit = 4096
const pageRotationErrorCacheLimit = 4096
const pageBoxCacheLimit = 32 << 10
const xobjectResourceErrorCacheLimit = 4096

func (f *Font) cffPathCacheBudget() int {
	if f != nil && f.document != nil {
		return f.document.cacheLimits().CFFPathBytes
	}
	return cffPathCacheLimit
}

func (f *Font) trueTypePathCacheBudget() int {
	if f != nil && f.document != nil {
		return f.document.cacheLimits().TrueTypePathBytes
	}
	return trueTypePathCacheLimit
}

func (f *Font) type1PathCacheBudget() int {
	if f != nil && f.document != nil {
		return f.document.cacheLimits().Type1PathBytes
	}
	return type1PathCacheLimit
}

func (f *Font) type3CacheBudget() int {
	if f != nil && f.document != nil {
		return f.document.cacheLimits().Type3Bytes
	}
	return type3CacheLimit
}

type pageBoxCacheKey struct {
	page Ref
	box  Name
}

type pageBoxCacheEntry struct {
	value   [4]float64
	present bool
	err     error
}

// cacheFits checks a byte-budget addition without allowing integer overflow
// to turn an over-budget cache into an apparently small one.
func cacheFits(current, addition, limit int) bool {
	return current >= 0 && addition >= 0 && limit >= 0 && current <= limit && addition <= limit-current
}

func cacheMulSize(value, multiplier int) int {
	if value < 0 || multiplier < 0 || (multiplier != 0 && value > math.MaxInt/multiplier) {
		return math.MaxInt
	}
	return value * multiplier
}

func fontCacheSize(font *Font) int {
	if font == nil {
		return 0
	}
	if font.lazyMu == nil {
		return fontCacheSizeLocked(font)
	}
	font.lazyMu.Lock()
	defer font.lazyMu.Unlock()
	return fontCacheSizeLocked(font)
}

func fontCacheSizeLocked(font *Font) int {
	size := 2048
	for _, addition := range []int{len(font.name), len(font.subtype), len(font.fontType), len(font.toUnicodeData), len(font.trueTypeData), len(font.cidToGIDData), len(font.cmapData), len(font.cffData), len(font.type1Data)} {
		size = addCacheSize(size, addition)
	}
	size = addCacheSize(size, cacheMulSize(len(font.toUnicodeSpaces), 32))
	size = addCacheSize(size, cacheMulSize(len(font.variationCoords), 8))
	for _, values := range [][][]byte{font.cffLocalSubrs, font.cffGlobalSubrs} {
		for _, value := range values {
			size = addCacheSize(size, addCacheSize(len(value), 32))
		}
	}
	for _, values := range font.cffCharstrings {
		size = addCacheSize(size, addCacheSize(len(values), 32))
	}
	for name, value := range font.type1Charstrings {
		size = addCacheSize(size, addCacheSize(len(name)+len(value), 32))
	}
	for _, value := range font.type1Subrs {
		size = addCacheSize(size, addCacheSize(len(value), 32))
	}
	for _, addition := range []int{
		cacheMulSize(len(font.encoding), 16), cacheMulSize(len(font.widths), 16), cacheMulSize(len(font.standardWidths), 24),
		cacheMulSize(len(font.cidWidths), 24), cacheMulSize(len(font.verticalWidths), 24), cacheMulSize(len(font.verticalPositions), 32),
		cacheMulSize(len(font.glyphTexts), 32), cacheMulSize(len(font.glyphNames), 32), cacheMulSize(len(font.toUnicode), 32),
		cacheMulSize(len(font.glyphIDToUnicode), 32), cacheMulSize(len(font.glyphUnicodeToID), 24), cacheMulSize(len(font.cidToGID), 24),
		cacheMulSize(len(font.glyphIDWidths), 24), cacheMulSize(len(font.cidToUnicode), 32),
		cacheMulSize(len(font.charWidths), 32), cacheMulSize(len(font.charBBoxes), 48),
		cacheMulSize(len(font.cffGlyphIDs), 24), cacheMulSize(len(font.cffFDByGlyph), 24), cacheMulSize(len(font.cffFDLocalSubrs), 64),
		cacheMulSize(len(font.cffFDVariationIndex), 24),
	} {
		size = addCacheSize(size, addition)
	}
	for name, proc := range font.charProcs {
		size = addCacheSize(size, addCacheSize(len(name)+64, nameTreeObjectSize(proc, 4)))
	}
	for name, ops := range font.charProcOps {
		size = addCacheSize(size, addCacheSize(len(name)+64, contentOpsCacheSize(ops)))
	}
	size = addCacheSize(size, nameTreeObjectSize(font.resources, 8))
	size = addCacheSize(size, font.type3CacheBytes)
	size = addCacheSize(size, font.cffPathCacheBytes)
	size = addCacheSize(size, font.type1PathCacheBytes)
	size = addCacheSize(size, font.trueTypePathBytes)
	for _, values := range [][]Dict{font.toUnicodeParms, font.trueTypeParms, font.cidToGIDParms, font.cmapFilterParms, font.cffParms, font.type1Parms} {
		for _, value := range values {
			size = addCacheSize(size, nameTreeObjectSize(value, 4))
		}
	}
	if font.cmap != nil {
		size = addCacheSize(size, fontdata.CacheBytes(font.cmap))
	}
	return size
}

func contentOpsCacheSize(ops []ContentOp) int {
	size := cacheMulSize(len(ops), 64)
	for _, op := range ops {
		size = addCacheSize(size, len(op.operatorValue()))
		size = addCacheSize(size, cacheMulSize(len(op.operandsValue()), 32))
		for _, operand := range op.operandsValue() {
			size = addCacheSize(size, nameTreeObjectSize(operand, 4))
		}
		size = addCacheSize(size, nameTreeObjectSize(op.resources, 4))
	}
	return size
}

func pageFontMapCacheSize(fonts map[string]*Font) int {
	size := 128
	for name, font := range fonts {
		size = addCacheSize(size, addCacheSize(addCacheSize(len(name), 32), fontCacheSize(font)))
	}
	return size
}

func (d *Document) cachedFont(ref Ref) (*Font, bool) {
	d.cacheMu.RLock()
	font, ok := d.fontCache[ref]
	d.cacheMu.RUnlock()
	return font, ok
}

func (d *Document) cachedTrueTypeProgram(ref Ref) *trueTypeProgramState {
	d.cacheMu.RLock()
	program := d.trueTypePrograms[ref]
	d.cacheMu.RUnlock()
	return program
}

func (d *Document) storeTrueTypeProgram(ref Ref, owner *Font, program *trueTypeProgramState) *trueTypeProgramState {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if cached := d.trueTypePrograms[ref]; cached != nil {
		return cached
	}
	var ownerRef Ref
	for candidateRef, candidate := range d.fontCache {
		if candidate == owner {
			ownerRef = candidateRef
			break
		}
	}
	if ownerRef == (Ref{}) {
		return program
	}
	if d.trueTypePrograms == nil {
		d.trueTypePrograms = map[Ref]*trueTypeProgramState{}
		d.trueTypeProgramOwners = map[Ref]Ref{}
	}
	d.trueTypePrograms[ref] = program
	d.trueTypeProgramOwners[ref] = ownerRef
	return program
}

// acquireImageBuild reserves construction of one indirect image. A waiter
// receives the in-flight build so it can reuse even an uncached completion.
func (d *Document) acquireImageBuild(ref Ref) (ImageObject, *imageBuild, bool) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if image, ok := d.imageCache[ref]; ok {
		return image, nil, false
	}
	if build, ok := d.imageBuilds[ref]; ok {
		return ImageObject{}, build, false
	}
	if d.imageBuilds == nil {
		d.imageBuilds = map[Ref]*imageBuild{}
	}
	build := &imageBuild{done: make(chan struct{})}
	d.imageBuilds[ref] = build
	return ImageObject{}, build, true
}

func (d *Document) finishImageBuild(ref Ref, image ImageObject) {
	d.cacheMu.Lock()
	limit := d.cacheLimits().ImageBytes
	if cacheFits(d.imageCacheBytes, len(image.data), limit) {
		if d.imageCache == nil {
			d.imageCache = map[Ref]ImageObject{}
		}
		if _, exists := d.imageCache[ref]; !exists {
			d.imageCache[ref] = image
			d.imageCacheBytes += len(image.data)
		}
	}
	if build := d.imageBuilds[ref]; build != nil {
		build.image = image
		delete(d.imageBuilds, ref)
		close(build.done)
	}
	d.cacheMu.Unlock()
}

// acquireFontBuild reserves construction of one indirect font. A waiter
// receives the in-flight completion channel and retries the cache afterward.
func (d *Document) acquireFontBuild(ref Ref) (*Font, *fontBuild, bool) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if font, ok := d.fontCache[ref]; ok {
		return font, nil, false
	}
	if build, ok := d.fontBuilds[ref]; ok {
		return nil, build, false
	}
	if d.fontBuilds == nil {
		d.fontBuilds = map[Ref]*fontBuild{}
	}
	build := &fontBuild{done: make(chan struct{})}
	d.fontBuilds[ref] = build
	return nil, build, true
}

func (d *Document) cachedFontError(ref Ref) (error, bool) {
	d.cacheMu.RLock()
	err, ok := d.fontErrors[ref]
	d.cacheMu.RUnlock()
	return err, ok
}

func (d *Document) cacheFontErrorLocked(ref Ref, err error) error {
	if err == nil || len(err.Error()) > d.cacheLimits().FontErrorBytes || !cacheFits(d.fontErrorBytes, len(err.Error()), d.cacheLimits().FontErrorBytes) {
		return err
	}
	if cached, exists := d.fontErrors[ref]; exists {
		return cached
	}
	if d.fontErrors == nil {
		d.fontErrors = map[Ref]error{}
	}
	d.fontErrors[ref] = err
	d.fontErrorBytes += len(err.Error())
	return err
}

func (d *Document) storeFontError(ref Ref, err error) error {
	if ref == (Ref{}) {
		return err
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	return d.cacheFontErrorLocked(ref, err)
}

func (d *Document) finishFontBuild(ref Ref, font *Font, err error) {
	fontBytes := fontCacheSize(font)
	limit := d.cacheLimits().FontBytes
	d.cacheMu.Lock()
	_ = d.cacheFontErrorLocked(ref, err)
	if font != nil {
		d.storeFontLocked(ref, font, fontBytes, limit)
	}
	build := d.fontBuilds[ref]
	if build != nil {
		build.font, build.err = font, err
		delete(d.fontBuilds, ref)
		close(build.done)
	}
	d.cacheMu.Unlock()
}

func (d *Document) storeFont(ref Ref, font *Font) {
	fontBytes := fontCacheSize(font)
	limit := d.cacheLimits().FontBytes
	if fontBytes > limit {
		return
	}
	d.cacheMu.Lock()
	d.storeFontLocked(ref, font, fontBytes, limit)
	d.cacheMu.Unlock()
}

func (d *Document) storeFontLocked(ref Ref, font *Font, fontBytes, limit int) {
	if font == nil || fontBytes > limit {
		return
	}
	if _, exists := d.fontCache[ref]; exists {
		return
	}
	for !cacheFits(d.fontCacheBytes, fontBytes, limit) {
		if !d.evictOneFontLocked() {
			return
		}
	}
	if d.fontCache == nil {
		d.fontCache = map[Ref]*Font{}
	}
	if d.fontCacheSizes == nil {
		d.fontCacheSizes = map[Ref]int{}
	}
	d.fontCache[ref] = font
	d.fontCacheSizes[ref] = fontBytes
	d.fontCacheOrder = append(d.fontCacheOrder, ref)
	d.fontCacheBytes += fontBytes
}

func (d *Document) evictOneFontLocked() bool {
	for len(d.fontCacheOrder) > 0 {
		ref := d.fontCacheOrder[0]
		d.fontCacheOrder = d.fontCacheOrder[1:]
		_, exists := d.fontCache[ref]
		if !exists {
			continue
		}
		fontBytes := d.fontCacheSizes[ref]
		if fontBytes <= 0 {
			d.fontCache = nil
			d.fontCacheBytes = 0
			d.fontCacheOrder = nil
			d.fontCacheSizes = nil
			d.trueTypePrograms = nil
			d.trueTypeProgramOwners = nil
			return true
		}
		if fontBytes > d.fontCacheBytes {
			// A manually constructed or stale cache must not let accounting
			// underflow. Drop it as one inconsistent unit and rebuild lazily.
			d.fontCache = nil
			d.fontCacheBytes = 0
			d.fontCacheOrder = nil
			d.fontCacheSizes = nil
			d.trueTypePrograms = nil
			d.trueTypeProgramOwners = nil
			return true
		}
		delete(d.fontCache, ref)
		delete(d.fontCacheSizes, ref)
		for programRef, ownerRef := range d.trueTypeProgramOwners {
			if ownerRef == ref {
				delete(d.trueTypeProgramOwners, programRef)
				delete(d.trueTypePrograms, programRef)
			}
		}
		d.fontCacheBytes -= fontBytes
		return true
	}
	// Tests and embedders may construct a Document cache directly. Recover a
	// deterministic oldest candidate even when no insertion order was recorded.
	var oldest Ref
	oldestSet := false
	for ref := range d.fontCache {
		if !oldestSet || ref.Object < oldest.Object || (ref.Object == oldest.Object && ref.Generation < oldest.Generation) {
			oldest, oldestSet = ref, true
		}
	}
	if oldestSet {
		d.fontCacheOrder = append(d.fontCacheOrder, oldest)
		return d.evictOneFontLocked()
	}
	return false
}

func (d *Document) cachedPageFont(ref Ref, name string) (*Font, bool) {
	d.cacheMu.RLock()
	fonts := d.pageFontCache[ref]
	font, ok := fonts[name]
	d.cacheMu.RUnlock()
	return font, ok
}

func (d *Document) cachedPageFonts(ref Ref) (map[string]*Font, bool) {
	d.cacheMu.RLock()
	fonts, ok := d.pageFontCache[ref]
	if ok {
		fonts = cloneFontRefs(fonts)
	}
	d.cacheMu.RUnlock()
	return fonts, ok
}

func (d *Document) storePageFont(ref Ref, name string, font *Font) {
	limit := d.cacheLimits().PageFontBytes
	entryBytes := addCacheSize(addCacheSize(len(name), 32), fontCacheSize(font))
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	existing := d.pageFontCache[ref]
	oldBytes := 0
	if existing != nil {
		if _, exists := existing[name]; exists {
			return
		}
		oldBytes = d.pageFontCacheSizes[ref]
		if oldBytes <= 0 || oldBytes > d.pageFontCacheBytes {
			d.clearPageFontCacheLocked()
			return
		}
	}
	newBytes := addCacheSize(oldBytes, entryBytes)
	if existing == nil {
		newBytes = addCacheSize(newBytes, 128)
	}
	if newBytes > limit {
		return
	}
	for !cacheFits(d.pageFontCacheBytes, newBytes-oldBytes, limit) {
		if !d.evictOnePageFontMapLocked(ref) {
			return
		}
		if existing != nil {
			if _, retained := d.pageFontCache[ref]; !retained {
				existing = nil
				oldBytes = 0
				newBytes = addCacheSize(entryBytes, 128)
				if newBytes > limit {
					return
				}
			}
		}
	}
	addition := newBytes - oldBytes
	if d.pageFontCache == nil {
		d.pageFontCache = map[Ref]map[string]*Font{}
	}
	if d.pageFontCache[ref] == nil {
		d.pageFontCache[ref] = map[string]*Font{}
		d.pageFontCacheOrder = append(d.pageFontCacheOrder, ref)
	}
	if d.pageFontCacheSizes == nil {
		d.pageFontCacheSizes = map[Ref]int{}
	}
	d.pageFontCache[ref][name] = font
	d.pageFontCacheSizes[ref] = newBytes
	d.pageFontCacheBytes += addition
}

func (d *Document) storePageFonts(ref Ref, fonts map[string]*Font) {
	fontBytes := pageFontMapCacheSize(fonts)
	limit := d.cacheLimits().PageFontBytes
	if fontBytes > limit {
		return
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	oldBytes := 0
	_, replacing := d.pageFontCache[ref]
	if replacing {
		oldBytes = d.pageFontCacheSizes[ref]
		if oldBytes <= 0 || oldBytes > d.pageFontCacheBytes {
			d.clearPageFontCacheLocked()
			return
		}
	}
	for !cacheFits(d.pageFontCacheBytes-oldBytes, fontBytes, limit) {
		if !d.evictOnePageFontMapLocked(ref) {
			return
		}
		if replacing {
			if _, retained := d.pageFontCache[ref]; !retained {
				replacing = false
				oldBytes = 0
			}
		}
	}
	if d.pageFontCache == nil {
		d.pageFontCache = map[Ref]map[string]*Font{}
	}
	if d.pageFontCacheSizes == nil {
		d.pageFontCacheSizes = map[Ref]int{}
	}
	if !replacing {
		d.pageFontCacheOrder = append(d.pageFontCacheOrder, ref)
	}
	d.pageFontCache[ref] = fonts
	d.pageFontCacheSizes[ref] = fontBytes
	d.pageFontCacheBytes = d.pageFontCacheBytes - oldBytes + fontBytes
}

func (d *Document) evictOnePageFontMapLocked(protected Ref) bool {
	for attempts := len(d.pageFontCacheOrder); attempts > 0; attempts-- {
		ref := d.pageFontCacheOrder[0]
		d.pageFontCacheOrder = d.pageFontCacheOrder[1:]
		if ref == protected {
			d.pageFontCacheOrder = append(d.pageFontCacheOrder, ref)
			continue
		}
		_, exists := d.pageFontCache[ref]
		if !exists {
			continue
		}
		fontBytes := d.pageFontCacheSizes[ref]
		if fontBytes <= 0 {
			d.clearPageFontCacheLocked()
			return true
		}
		if fontBytes > d.pageFontCacheBytes {
			d.clearPageFontCacheLocked()
			return true
		}
		delete(d.pageFontCache, ref)
		delete(d.pageFontCacheSizes, ref)
		d.pageFontCacheBytes -= fontBytes
		return true
	}
	var oldest Ref
	oldestSet := false
	for ref := range d.pageFontCache {
		if ref == protected {
			continue
		}
		if !oldestSet || ref.Object < oldest.Object || (ref.Object == oldest.Object && ref.Generation < oldest.Generation) {
			oldest, oldestSet = ref, true
		}
	}
	if oldestSet {
		d.pageFontCacheOrder = append(d.pageFontCacheOrder, oldest)
		return d.evictOnePageFontMapLocked(protected)
	}
	return false
}

func (d *Document) clearPageFontCacheLocked() {
	d.pageFontCache = nil
	d.pageFontCacheBytes = 0
	d.pageFontCacheOrder = nil
	d.pageFontCacheSizes = nil
}

func cloneFontRefs(fonts map[string]*Font) map[string]*Font {
	if fonts == nil {
		return nil
	}
	clone := make(map[string]*Font, len(fonts))
	for name, font := range fonts {
		clone[name] = font
	}
	return clone
}

func (d *Document) cachedPageProperties(ref Ref) (Dict, bool) {
	d.cacheMu.RLock()
	properties, ok := d.pagePropCache[ref]
	d.cacheMu.RUnlock()
	return properties, ok
}

func (d *Document) cachedPagePropertiesError(ref Ref) (error, bool) {
	d.cacheMu.RLock()
	err, ok := d.pagePropErrors[ref]
	d.cacheMu.RUnlock()
	return err, ok
}

func (d *Document) storePagePropertiesError(ref Ref, err error) error {
	if err == nil || ref == (Ref{}) || len(err.Error()) > d.cacheLimits().PagePropertyErrorBytes {
		return err
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if cached, exists := d.pagePropErrors[ref]; exists {
		return cached
	}
	if !cacheFits(d.pagePropErrorBytes, len(err.Error()), d.cacheLimits().PagePropertyErrorBytes) {
		return err
	}
	if d.pagePropErrors == nil {
		d.pagePropErrors = map[Ref]error{}
	}
	d.pagePropErrors[ref] = err
	d.pagePropErrorBytes += len(err.Error())
	return err
}

func (d *Document) cachedPageResourceError(ref Ref) (error, bool) {
	d.cacheMu.RLock()
	err, ok := d.pageResourceErrors[ref]
	d.cacheMu.RUnlock()
	return err, ok
}

func (d *Document) storePageResourceError(ref Ref, err error) error {
	if err == nil || ref == (Ref{}) || len(err.Error()) > d.cacheLimits().PageResourceErrorBytes {
		return err
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if cached, exists := d.pageResourceErrors[ref]; exists {
		return cached
	}
	if !cacheFits(d.pageResourceErrorBytes, len(err.Error()), d.cacheLimits().PageResourceErrorBytes) {
		return err
	}
	if d.pageResourceErrors == nil {
		d.pageResourceErrors = map[Ref]error{}
	}
	d.pageResourceErrors[ref] = err
	d.pageResourceErrorBytes += len(err.Error())
	return err
}

func (d *Document) cachedPageResource(ref Ref) (Dict, bool) {
	d.cacheMu.RLock()
	resources, ok := d.pageResourceCache[ref]
	d.cacheMu.RUnlock()
	return resources, ok
}

func (d *Document) storePageResource(ref Ref, resources Dict) {
	if ref == (Ref{}) || resources == nil {
		return
	}
	bytes := nameTreeObjectSize(resources, 8)
	limit := d.cacheLimits().PageResourceBytes
	if bytes > limit {
		return
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if _, exists := d.pageResourceCache[ref]; exists || !cacheFits(d.pageResourceCacheBytes, bytes, limit) {
		return
	}
	if d.pageResourceCache == nil {
		d.pageResourceCache = map[Ref]Dict{}
	}
	d.pageResourceCache[ref] = resources
	d.pageResourceCacheBytes += bytes
}

func (d *Document) cachedPageUserUnit(ref Ref) (float64, bool) {
	d.cacheMu.RLock()
	value, ok := d.pageUserUnitCache[ref]
	d.cacheMu.RUnlock()
	return value, ok
}

func (d *Document) storePageUserUnit(ref Ref, value float64) {
	if ref == (Ref{}) {
		return
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if _, exists := d.pageUserUnitCache[ref]; exists {
		return
	}
	if !cacheFits(d.pageUserUnitCacheBytes, 8, d.cacheLimits().PageUserUnitBytes) {
		return
	}
	if d.pageUserUnitCache == nil {
		d.pageUserUnitCache = map[Ref]float64{}
	}
	d.pageUserUnitCache[ref] = value
	d.pageUserUnitCacheBytes += 8
}

func (d *Document) cachedPageUserUnitError(ref Ref) (error, bool) {
	d.cacheMu.RLock()
	err, ok := d.pageUserUnitErrors[ref]
	d.cacheMu.RUnlock()
	return err, ok
}

func (d *Document) storePageUserUnitError(ref Ref, err error) error {
	if err == nil || ref == (Ref{}) || len(err.Error()) > d.cacheLimits().PageUserUnitErrorBytes {
		return err
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if cached, exists := d.pageUserUnitErrors[ref]; exists {
		return cached
	}
	if !cacheFits(d.pageUserUnitErrorBytes, len(err.Error()), d.cacheLimits().PageUserUnitErrorBytes) {
		return err
	}
	if d.pageUserUnitErrors == nil {
		d.pageUserUnitErrors = map[Ref]error{}
	}
	d.pageUserUnitErrors[ref] = err
	d.pageUserUnitErrorBytes += len(err.Error())
	return err
}

func (d *Document) cachedPageRotation(ref Ref) (int, bool) {
	d.cacheMu.RLock()
	value, ok := d.pageRotationCache[ref]
	d.cacheMu.RUnlock()
	return value, ok
}

func (d *Document) storePageRotation(ref Ref, value int) {
	if ref == (Ref{}) {
		return
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if _, exists := d.pageRotationCache[ref]; exists || !cacheFits(d.pageRotationCacheBytes, 8, d.cacheLimits().PageRotationBytes) {
		return
	}
	if d.pageRotationCache == nil {
		d.pageRotationCache = map[Ref]int{}
	}
	d.pageRotationCache[ref] = value
	d.pageRotationCacheBytes += 8
}

func (d *Document) cachedPageRotationError(ref Ref) (error, bool) {
	d.cacheMu.RLock()
	err, ok := d.pageRotationErrors[ref]
	d.cacheMu.RUnlock()
	return err, ok
}

func (d *Document) storePageRotationError(ref Ref, err error) error {
	if err == nil || ref == (Ref{}) || len(err.Error()) > d.cacheLimits().PageRotationErrorBytes {
		return err
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if cached, exists := d.pageRotationErrors[ref]; exists {
		return cached
	}
	if !cacheFits(d.pageRotationErrorBytes, len(err.Error()), d.cacheLimits().PageRotationErrorBytes) {
		return err
	}
	if d.pageRotationErrors == nil {
		d.pageRotationErrors = map[Ref]error{}
	}
	d.pageRotationErrors[ref] = err
	d.pageRotationErrorBytes += len(err.Error())
	return err
}

func (d *Document) cachedPageBox(ref Ref, box Name) (pageBoxCacheEntry, bool) {
	d.cacheMu.RLock()
	entry, ok := d.pageBoxCache[pageBoxCacheKey{page: ref, box: box}]
	d.cacheMu.RUnlock()
	return entry, ok
}

func (d *Document) storePageBox(ref Ref, box Name, entry pageBoxCacheEntry) pageBoxCacheEntry {
	if ref == (Ref{}) {
		return entry
	}
	size := 64
	if entry.err != nil {
		size += len(entry.err.Error())
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	key := pageBoxCacheKey{page: ref, box: box}
	if cached, exists := d.pageBoxCache[key]; exists {
		return cached
	}
	if !cacheFits(d.pageBoxCacheBytes, size, d.cacheLimits().PageBoxBytes) {
		return entry
	}
	if d.pageBoxCache == nil {
		d.pageBoxCache = map[pageBoxCacheKey]pageBoxCacheEntry{}
	}
	d.pageBoxCache[key] = entry
	d.pageBoxCacheBytes += size
	return entry
}

func (d *Document) cachedXObjectResourceError(ref Ref) (error, bool) {
	d.cacheMu.RLock()
	err, ok := d.xobjectResourceErrors[ref]
	d.cacheMu.RUnlock()
	return err, ok
}

func (d *Document) storeXObjectResourceError(ref Ref, err error) error {
	if err == nil || ref == (Ref{}) || len(err.Error()) > d.cacheLimits().XObjectResourceErrorBytes {
		return err
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if cached, exists := d.xobjectResourceErrors[ref]; exists {
		return cached
	}
	if !cacheFits(d.xobjectResourceErrorBytes, len(err.Error()), d.cacheLimits().XObjectResourceErrorBytes) {
		return err
	}
	if d.xobjectResourceErrors == nil {
		d.xobjectResourceErrors = map[Ref]error{}
	}
	d.xobjectResourceErrors[ref] = err
	d.xobjectResourceErrorBytes += len(err.Error())
	return err
}

func (d *Document) cachedXObjectResource(ref Ref) (Dict, bool) {
	d.cacheMu.RLock()
	resources, ok := d.xobjectResourceCache[ref]
	d.cacheMu.RUnlock()
	return resources, ok
}

func (d *Document) storeXObjectResource(ref Ref, resources Dict) {
	if ref == (Ref{}) || resources == nil {
		return
	}
	bytes := nameTreeObjectSize(resources, 8)
	limit := d.cacheLimits().XObjectResourceBytes
	if bytes > limit {
		return
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if _, exists := d.xobjectResourceCache[ref]; exists || !cacheFits(d.xobjectResourceCacheBytes, bytes, limit) {
		return
	}
	if d.xobjectResourceCache == nil {
		d.xobjectResourceCache = map[Ref]Dict{}
	}
	d.xobjectResourceCache[ref] = resources
	d.xobjectResourceCacheBytes += bytes
}

func (d *Document) storePageProperties(ref Ref, properties Dict) {
	propertyBytes := nameTreeObjectSize(properties, 8)
	limit := d.cacheLimits().PagePropertyBytes
	if propertyBytes > limit {
		return
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	oldBytes := 0
	if old, exists := d.pagePropCache[ref]; exists {
		oldBytes = nameTreeObjectSize(old, 8)
	}
	if oldBytes > d.pagePropCacheBytes {
		d.pagePropCache = nil
		d.pagePropCacheBytes = 0
		return
	}
	if !cacheFits(d.pagePropCacheBytes-oldBytes, propertyBytes, limit) {
		return
	}
	if d.pagePropCache == nil {
		d.pagePropCache = map[Ref]Dict{}
	}
	d.pagePropCache[ref] = properties
	d.pagePropCacheBytes = d.pagePropCacheBytes - oldBytes + propertyBytes
}
