package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"iter"
	"regexp"
	"sort"
	"sync"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/coordinates"
	"github.com/lin-string/go-playa/documentconfig"
	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/internal/filebuffer"
	pdfparser "github.com/lin-string/go-playa/parser"
)

// IndirectObject is the stable object-level unit exposed by Document.
type IndirectObject = documentdata.IndirectObject

func newIndirectObject(ref Ref, value Object) IndirectObject {
	return documentdata.NewIndirectObject(ref, value)
}

type inlineImageKey struct {
	page   Ref
	stream int
	offset int
}

type decodedStreamDecode struct {
	done chan struct{}
	data []byte
	err  error
}

type fontBuild struct {
	done chan struct{}
	font *Font
	err  error
}

type imageBuild struct {
	done  chan struct{}
	image ImageObject
}

type objectStreamBuild struct {
	done chan struct{}
}

const objectStreamErrorCacheLimit = 64 << 10
const objectErrorCacheByteLimit = 64 << 10

type Document struct {
	cacheMu                   sync.RWMutex
	fontLifecycle             sync.RWMutex
	decodedStreamLifecycle    sync.RWMutex
	objectStreamLifecycle     sync.RWMutex
	objectStreamBuilds        map[int]*objectStreamBuild
	objectStreamErrors        map[int]error
	objectStreamErrorBytes    int
	objectErrorBytes          int
	objectCacheBytes          int
	pageBuildMu               sync.Mutex
	actionMu                  sync.Mutex
	data                      []byte
	releaseData               func() error
	version                   string
	space                     coordinates.Space
	encrypted                 bool
	encryptionKey             []byte
	encryptionRevision        int
	encryptionMethod          string
	streamEncryptionMethod    string
	stringEncryptionMethod    string
	printable                 bool
	modifiable                bool
	extractable               bool
	metadataExcluded          bool
	metadataRefs              map[Ref]bool
	decrypted                 map[Ref]bool
	objects                   map[Ref]Object
	scannedObjects            bool
	recoveryErr               error
	trailer                   Dict
	catalogCache              Dict
	catalogReady              bool
	catalogErr                error
	catalogErrReady           bool
	infoCache                 Dict
	infoReady                 bool
	infoErr                   error
	infoErrReady              bool
	namesCache                Dict
	namesReady                bool
	namesErr                  error
	namesErrReady             bool
	xrefs                     map[Ref]xrefEntry
	xrefHistory               map[Ref]xrefEntry
	expandedObjStms           map[int]bool
	fontCache                 map[Ref]*Font
	fontCacheBytes            int
	fontCacheOrder            []Ref
	fontCacheSizes            map[Ref]int
	trueTypePrograms          map[Ref]*trueTypeProgramState
	trueTypeProgramOwners     map[Ref]Ref
	fontErrors                map[Ref]error
	fontErrorBytes            int
	fontBuilds                map[Ref]*fontBuild
	imageBuilds               map[Ref]*imageBuild
	pageFontCache             map[Ref]map[string]*Font
	pageFontCacheBytes        int
	pageFontCacheOrder        []Ref
	pageFontCacheSizes        map[Ref]int
	pagePropCache             map[Ref]Dict
	pagePropCacheBytes        int
	pagePropErrors            map[Ref]error
	pagePropErrorBytes        int
	pageResourceErrors        map[Ref]error
	pageResourceErrorBytes    int
	pageResourceCache         map[Ref]Dict
	pageResourceCacheBytes    int
	pageUserUnitCache         map[Ref]float64
	pageUserUnitCacheBytes    int
	pageUserUnitErrors        map[Ref]error
	pageUserUnitErrorBytes    int
	pageRotationCache         map[Ref]int
	pageRotationCacheBytes    int
	pageRotationErrors        map[Ref]error
	pageRotationErrorBytes    int
	pageBoxCache              map[pageBoxCacheKey]pageBoxCacheEntry
	pageBoxCacheBytes         int
	xobjectResourceErrors     map[Ref]error
	xobjectResourceErrorBytes int
	xobjectResourceCache      map[Ref]Dict
	xobjectResourceCacheBytes int
	contentRootErrors         map[Ref]error
	contentRootErrorBytes     int
	imageCache                map[Ref]ImageObject
	imageCacheBytes           int
	inlineImageCache          map[inlineImageKey]Stream
	inlineImageCacheBytes     int
	decodedStreamCache        map[Ref][]byte
	decodedStreamCacheBytes   int
	decodedStreamErrors       map[Ref]error
	decodedStreamErrorBytes   int
	decodedStreamInflight     map[Ref]*decodedStreamDecode
	objectErrors              map[Ref]error
	pageLabelCache            []pageLabelItem
	pageLabelCacheBytes       int
	pageLabelsReady           bool
	pageLabelsErr             error
	pageLabelsValuesMu        sync.Mutex
	pageLabelsValuesReady     bool
	pageLabelsValuesBytes     int
	pageLabelsValues          []string
	pageLabelsValuesErr       error
	pageCountCache            int
	pageCountReady            bool
	metadataXMLMu             sync.Mutex
	metadataXMLReady          bool
	metadataXMLCacheable      bool
	metadataXMLCache          []byte
	metadataXMLErr            error
	metadataMu                sync.Mutex
	metadataReady             bool
	metadataCacheable         bool
	metadataCache             Metadata
	metadataErr               error
	openActionCache           *Action
	openActionReady           bool
	openActionErr             error
	openActionErrReady        bool
	actionScriptCache         map[Ref]string
	actionScriptCacheBytes    int
	actionCache               map[Ref]*Action
	actionCacheBytes          int
	actionErrors              map[Ref]error
	actionErrorBytes          int
	nameTreeCache             map[string][]NameTreeEntry
	nameTreeReady             map[string]bool
	nameTreeErrors            map[string]error
	nameTreeErrorBytes        int
	destinationCache          map[string]Object
	destinationCacheBytes     int
	destinationErrors         map[Ref]error
	destinationErrorBytes     int
	destinationsMu            sync.Mutex
	destinationsCache         map[string]Object
	destinationsCacheBytes    int
	destinationsCacheable     bool
	destinationsReady         bool
	destinationsErr           error
	destinationsRootErr       error
	destinationsRootErrReady  bool
	taggedRanksMu             sync.Mutex
	taggedRanks               map[Ref]taggedPageRanks
	taggedRanksReady          bool
	taggedRanksErr            error
	structureRoleMapCache     map[string]string
	structureRoleMapReady     bool
	structureRoleMapErr       error
	structureRootErr          error
	structureRootErrReady     bool
	parentTreeCache           map[int]Object
	parentTreeCacheBytes      int
	parentTreeMissing         map[int]bool
	parentTreeErrors          map[int]error
	parentTreeErrorBytes      int
	pageStructureCache        map[Ref]PageStructure
	pageStructureCacheBytes   int
	pageStructureReady        map[Ref]bool
	pageStructureErrors       map[Ref]error
	pageStructureErrorBytes   int
	annotationCache           map[Ref]Annotation
	annotationCacheBytes      int
	annotationCacheSizes      map[Ref]int
	annotationErrors          map[Ref]error
	annotationErrorBytes      int
	annotationRootErrors      map[Ref]error
	annotationRootErrorBytes  int
	outlineCache              map[Ref]OutlineNode
	outlineCacheBytes         int
	outlineCacheSizes         map[Ref]int
	outlineRootErr            error
	outlineRootErrReady       bool
	formTopCache              map[Ref]FormField
	formTopCacheBytes         int
	formTopCacheSizes         map[Ref]int
	formFieldsErr             error
	formFieldsErrReady        bool
	pageCache                 map[Ref]Page
	pageCacheBytes            int
	pageMissing               map[Ref]bool
	pageMissingBytes          int
	pagesCache                []Page
	pagesCacheBytes           int
	pagesReady                bool
	pagesErr                  error
	pagesErrReady             bool
	objectRefs                []Ref
	objectRefsReady           bool
	contentXRefRecovered      bool
	recoveredTrailerSize      int
	lookupCache               map[int]Ref
	lookupFound               map[int]bool
	lookupCacheBytes          int
	lookupIndexReady          bool
	lookupIndexDisabled       bool
	cacheOptions              cacheconfig.Options
	cacheOptionsConfigured    bool
}

func cloneDict(value Dict) Dict {
	if value == nil {
		return nil
	}
	return cloneGraphicsObject(value).(Dict)
}

const (
	CoordinateSpacePage    = coordinates.Page
	CoordinateSpaceScreen  = coordinates.Screen
	CoordinateSpaceDefault = coordinates.Default
	CoordinateSpaceUser    = coordinates.User
)

// WithCoordinateSpace selects the coordinate-space convention for geometry.
func WithCoordinateSpace(space coordinates.Space) documentconfig.OpenOption {
	return func(options *documentconfig.Options) { options.Space = space }
}

// WithPassword supplies a PDF password, including an explicitly empty one.
func WithPassword(password string) documentconfig.OpenOption {
	return func(options *documentconfig.Options) { options.Password, options.PasswordSet = password, true }
}

// WithCacheOptions replaces all retained-cache budgets for the opened document.
func WithCacheOptions(cache cacheconfig.Options) documentconfig.OpenOption {
	return func(options *documentconfig.Options) { options.Cache, options.CacheSet = cache, true }
}

func defaultCacheOptions() cacheconfig.Options {
	return cacheconfig.Options{
		ObjectBytes: 64 << 20, ObjectErrorBytes: objectErrorCacheByteLimit, ObjectStreamErrorBytes: objectStreamErrorCacheLimit, ImageBytes: 32 << 20, InlineImageBytes: 32 << 20, DecodedStreamBytes: 32 << 20, DecodedErrorBytes: 64 << 10,
		FontBytes: 32 << 20, FontErrorBytes: fontErrorCacheLimit, CFFPathBytes: cffPathCacheLimit, TrueTypePathBytes: trueTypePathCacheLimit, Type1PathBytes: type1PathCacheLimit, Type3Bytes: type3CacheLimit, PageFontBytes: 32 << 20, PagePropertyBytes: 32 << 20, PagePropertyErrorBytes: pagePropErrorCacheLimit, PageResourceBytes: 32 << 20, PageResourceErrorBytes: pageResourceErrorCacheLimit, XObjectResourceBytes: 32 << 20, XObjectResourceErrorBytes: xobjectResourceErrorCacheLimit, PageUserUnitBytes: pageUserUnitCacheLimit, PageUserUnitErrorBytes: pageUserUnitErrorCacheLimit, PageRotationBytes: pageRotationCacheLimit, PageRotationErrorBytes: pageRotationErrorCacheLimit, PageBoxBytes: pageBoxCacheLimit, ContentRootErrorBytes: contentRootErrorLimit, NameTreeBytes: nameTreeCacheLimit, NameTreeErrorBytes: nameTreeErrorCacheLimit,
		DestinationBytes: destinationCacheLimit, DestinationErrorBytes: destinationErrorCacheLimit, DestinationsRootErrorBytes: destinationsRootErrorCacheLimit, ActionScriptBytes: actionScriptCacheLimit, ActionBytes: actionCacheLimit, ActionErrorBytes: actionErrorCacheLimit, MetadataBytes: 32 << 20,
		MetadataXMLBytes: 32 << 20, ParentTreeBytes: parentTreeCacheLimit, ParentTreeErrorBytes: parentTreeErrorCacheLimit, PageStructureBytes: pageStructureCacheLimit, PageStructureErrorBytes: pageStructureErrorCacheLimit, AnnotationBytes: 32 << 20, AnnotationErrorBytes: annotationErrorCacheLimit, AnnotationRootErrorBytes: annotationRootErrorCacheLimit,
		OutlineBytes: outlineCacheLimit, FormFieldBytes: formTopCacheLimit, PageBytes: 32 << 20, PagesBytes: 32 << 20,
		PageLabelRuleBytes: pageLabelCacheLimit, PageLabelsValuesBytes: pageLabelsValuesLimit, LookupBytes: 32 << 20, MissingPageBytes: 32 << 20,
	}
}

// DefaultCacheOptions returns the default retained-cache budgets. Callers can
// copy it, adjust selected fields, and pass it to WithCacheOptions.
func DefaultCacheOptions() cacheconfig.Options { return defaultCacheOptions() }

// DefaultDocumentOptions returns a complete set of options matching Open's
// defaults. Callers can adjust the returned value and pass it to
// WithDocumentOptions before opening a document.
func DefaultDocumentOptions() documentconfig.Options {
	return documentconfig.Options{Space: CoordinateSpaceScreen, Cache: defaultCacheOptions(), CacheSet: true}
}

// WithDocumentOptions replaces the complete document-open configuration.
func WithDocumentOptions(options documentconfig.Options) documentconfig.OpenOption {
	return func(target *documentconfig.Options) { *target = options }
}

func (d *Document) cacheLimits() cacheconfig.Options {
	if d.cacheOptionsConfigured {
		return d.cacheOptions
	}
	return defaultCacheOptions()
}

func (d *Document) storeObjectLocked(ref Ref, value Object) {
	if d.objects == nil {
		d.objects = map[Ref]Object{}
	}
	if previous, ok := d.objects[ref]; ok {
		previousSize := nameTreeObjectSize(previous, 8)
		if d.objectCacheBytes >= previousSize {
			d.objectCacheBytes -= previousSize
		} else {
			d.objectCacheBytes = 0
		}
	}
	d.objects[ref] = value
	d.objectCacheBytes = addCacheSize(d.objectCacheBytes, nameTreeObjectSize(value, 8))
}

func (d *Document) maybeEvictObjectCache() {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if (d.scannedObjects && d.xrefs == nil) || d.objectCacheBytes <= d.cacheLimits().ObjectBytes {
		return
	}
	// Object values are rebuildable from the source bytes and xref index. Keep
	// the current caller's local result alive, but release all retained values
	// once the parse that exceeded the budget has completed.
	d.objects = map[Ref]Object{}
	d.expandedObjStms = map[int]bool{}
	d.decrypted = map[Ref]bool{}
	d.objectCacheBytes = 0
}

var ErrEncrypted = errors.New("playa: encrypted PDF")
var ErrObjectNotFound = errors.New("playa: object not found")

// Close releases resources owned by d.
//
// Path-opened documents release their read-only file mapping here. Close is
// intentionally idempotent and also clears all derived caches.
func (d *Document) Close() error {
	d.pageBuildMu.Lock()
	defer d.pageBuildMu.Unlock()
	d.pageLabelsValuesMu.Lock()
	defer d.pageLabelsValuesMu.Unlock()
	d.metadataXMLMu.Lock()
	defer d.metadataXMLMu.Unlock()
	d.metadataMu.Lock()
	defer d.metadataMu.Unlock()
	d.destinationsMu.Lock()
	defer d.destinationsMu.Unlock()
	d.taggedRanksMu.Lock()
	defer d.taggedRanksMu.Unlock()
	d.actionMu.Lock()
	defer d.actionMu.Unlock()
	d.fontLifecycle.Lock()
	defer d.fontLifecycle.Unlock()
	d.decodedStreamLifecycle.Lock()
	defer d.decodedStreamLifecycle.Unlock()
	d.objectStreamLifecycle.Lock()
	defer d.objectStreamLifecycle.Unlock()
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	releaseData := d.releaseData
	d.releaseData = nil
	d.data = nil
	d.version = ""
	d.space = ""
	d.encrypted = false
	d.encryptionKey = nil
	d.encryptionRevision = 0
	d.encryptionMethod = ""
	d.streamEncryptionMethod = ""
	d.stringEncryptionMethod = ""
	d.printable = false
	d.modifiable = false
	d.extractable = false
	d.metadataExcluded = false
	d.metadataRefs = nil
	d.trailer = nil
	d.catalogCache = nil
	d.catalogReady = false
	d.catalogErr = nil
	d.catalogErrReady = false
	d.infoCache = nil
	d.infoReady = false
	d.infoErr = nil
	d.infoErrReady = false
	d.namesCache = nil
	d.namesReady = false
	d.namesErr = nil
	d.namesErrReady = false
	d.xrefs = nil
	d.xrefHistory = nil
	d.objects = map[Ref]Object{}
	d.objectCacheBytes = 0
	d.scannedObjects = false
	d.contentXRefRecovered = false
	d.recoveredTrailerSize = 0
	d.recoveryErr = nil
	d.expandedObjStms = map[int]bool{}
	d.objectStreamBuilds = nil
	d.objectStreamErrors = nil
	d.objectStreamErrorBytes = 0
	d.decrypted = map[Ref]bool{}
	d.fontCache = map[Ref]*Font{}
	d.fontCacheBytes = 0
	d.fontCacheOrder = nil
	d.fontCacheSizes = nil
	d.trueTypePrograms = nil
	d.trueTypeProgramOwners = nil
	d.fontErrors = nil
	d.fontErrorBytes = 0
	d.fontBuilds = map[Ref]*fontBuild{}
	d.imageBuilds = map[Ref]*imageBuild{}
	d.pageFontCache = map[Ref]map[string]*Font{}
	d.pageFontCacheBytes = 0
	d.pageFontCacheOrder = nil
	d.pageFontCacheSizes = nil
	d.pagePropCache = map[Ref]Dict{}
	d.pagePropCacheBytes = 0
	d.pagePropErrors = nil
	d.pagePropErrorBytes = 0
	d.pageResourceErrors = nil
	d.pageResourceErrorBytes = 0
	d.pageResourceCache = nil
	d.pageResourceCacheBytes = 0
	d.pageUserUnitCache = nil
	d.pageUserUnitCacheBytes = 0
	d.pageUserUnitErrors = nil
	d.pageUserUnitErrorBytes = 0
	d.pageRotationCache = nil
	d.pageRotationCacheBytes = 0
	d.pageRotationErrors = nil
	d.pageRotationErrorBytes = 0
	d.pageBoxCache = nil
	d.pageBoxCacheBytes = 0
	d.xobjectResourceErrors = nil
	d.xobjectResourceErrorBytes = 0
	d.xobjectResourceCache = nil
	d.xobjectResourceCacheBytes = 0
	d.contentRootErrors = nil
	d.contentRootErrorBytes = 0
	d.imageCache = map[Ref]ImageObject{}
	d.imageCacheBytes = 0
	d.inlineImageCache = map[inlineImageKey]Stream{}
	d.inlineImageCacheBytes = 0
	d.decodedStreamCache = map[Ref][]byte{}
	d.decodedStreamCacheBytes = 0
	d.decodedStreamErrors = map[Ref]error{}
	d.decodedStreamErrorBytes = 0
	d.decodedStreamInflight = map[Ref]*decodedStreamDecode{}
	d.objectErrors = nil
	d.objectErrorBytes = 0
	d.pageLabelCache = nil
	d.pageLabelCacheBytes = 0
	d.pageLabelsReady = false
	d.pageLabelsErr = nil
	d.pageLabelsValuesReady = false
	d.pageLabelsValuesBytes = 0
	d.pageLabelsValues = nil
	d.pageLabelsValuesErr = nil
	d.pageCountCache = 0
	d.pageCountReady = false
	d.metadataXMLReady = false
	d.metadataXMLCacheable = false
	d.metadataXMLCache = nil
	d.metadataXMLErr = nil
	d.metadataReady = false
	d.metadataCacheable = false
	d.metadataCache = nil
	d.metadataErr = nil
	d.openActionCache = nil
	d.openActionReady = false
	d.openActionErr = nil
	d.openActionErrReady = false
	d.actionScriptCache = nil
	d.actionScriptCacheBytes = 0
	d.actionCache = nil
	d.actionCacheBytes = 0
	d.actionErrors = nil
	d.actionErrorBytes = 0
	d.nameTreeCache = nil
	d.nameTreeReady = nil
	d.nameTreeErrors = nil
	d.nameTreeErrorBytes = 0
	d.destinationCache = nil
	d.destinationCacheBytes = 0
	d.destinationErrors = nil
	d.destinationErrorBytes = 0
	d.destinationsCacheBytes = 0
	d.destinationsCacheable = false
	d.destinationsCache = nil
	d.destinationsReady = false
	d.destinationsErr = nil
	d.destinationsRootErr = nil
	d.destinationsRootErrReady = false
	// Close releases every derived index, including the compact logical text
	// order retained across ordinary transient-cache releases.
	d.taggedRanks = nil
	d.taggedRanksReady = false
	d.taggedRanksErr = nil
	d.structureRoleMapCache = nil
	d.structureRoleMapReady = false
	d.structureRoleMapErr = nil
	d.structureRootErr = nil
	d.structureRootErrReady = false
	d.parentTreeCache = nil
	d.parentTreeCacheBytes = 0
	d.parentTreeMissing = nil
	d.parentTreeErrors = nil
	d.parentTreeErrorBytes = 0
	d.pageStructureCache = nil
	d.pageStructureCacheBytes = 0
	d.pageStructureReady = nil
	d.pageStructureErrors = nil
	d.pageStructureErrorBytes = 0
	d.annotationCache = nil
	d.annotationCacheBytes = 0
	d.annotationCacheSizes = nil
	d.annotationErrors = nil
	d.annotationErrorBytes = 0
	d.annotationRootErrors = nil
	d.annotationRootErrorBytes = 0
	d.outlineCache = nil
	d.outlineCacheBytes = 0
	d.outlineCacheSizes = nil
	d.outlineRootErr = nil
	d.outlineRootErrReady = false
	d.formTopCache = nil
	d.formTopCacheBytes = 0
	d.formTopCacheSizes = nil
	d.formFieldsErr = nil
	d.formFieldsErrReady = false
	d.pageCache = nil
	d.pageCacheBytes = 0
	d.pageMissing = nil
	d.pageMissingBytes = 0
	d.pagesCache = nil
	d.pagesCacheBytes = 0
	d.pagesReady = false
	d.pagesErr = nil
	d.pagesErrReady = false
	d.objectRefs = nil
	d.objectRefsReady = false
	d.resetLookupIndexLocked()
	if releaseData != nil {
		return releaseData()
	}
	return nil
}

// WasRecovered reports whether opening or lazy object resolution required
// damaged-object scanning because the cross-reference data was unusable.
func (d *Document) WasRecovered() bool {
	d.cacheMu.RLock()
	defer d.cacheMu.RUnlock()
	return d.scannedObjects
}

// RecoveryError returns the cross-reference parsing error that caused object
// scanning to be used, or nil when the document opened through its xref data.
// The document remains usable when this method returns a non-nil error.
func (d *Document) RecoveryError() error {
	return d.recoveryErr
}

// ReleaseTransientCaches drops parsed object and page-level caches while
// retaining the original PDF bytes, xref indexes, and compact document-wide
// indexes that contain no borrowed parsed objects. Subsequent object lookups
// are re-parsed lazily, so this is useful for bounded-memory sequential scans.
// It does not change the default repeatable caching behavior of Document.
func (d *Document) ReleaseTransientCaches() {
	d.pageBuildMu.Lock()
	defer d.pageBuildMu.Unlock()
	d.pageLabelsValuesMu.Lock()
	defer d.pageLabelsValuesMu.Unlock()
	d.metadataXMLMu.Lock()
	defer d.metadataXMLMu.Unlock()
	d.metadataMu.Lock()
	defer d.metadataMu.Unlock()
	d.destinationsMu.Lock()
	defer d.destinationsMu.Unlock()
	d.taggedRanksMu.Lock()
	defer d.taggedRanksMu.Unlock()
	d.actionMu.Lock()
	defer d.actionMu.Unlock()
	d.fontLifecycle.Lock()
	defer d.fontLifecycle.Unlock()
	d.decodedStreamLifecycle.Lock()
	defer d.decodedStreamLifecycle.Unlock()
	d.objectStreamLifecycle.Lock()
	defer d.objectStreamLifecycle.Unlock()
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	// Recovery scans now retain offsets in xrefs, so parsed objects remain
	// rebuildable just like objects from a regular xref table. Keep the old
	// fallback for manually constructed recovery documents without xrefs.
	if !d.scannedObjects || d.xrefs != nil {
		d.objects = map[Ref]Object{}
		d.expandedObjStms = map[int]bool{}
		d.objectCacheBytes = 0
	}
	d.objectStreamBuilds = nil
	d.objectStreamErrors = nil
	d.objectStreamErrorBytes = 0
	d.catalogCache = nil
	d.catalogReady = false
	d.catalogErr = nil
	d.catalogErrReady = false
	d.infoCache = nil
	d.infoReady = false
	d.infoErr = nil
	d.infoErrReady = false
	d.namesCache = nil
	d.namesReady = false
	d.namesErr = nil
	d.namesErrReady = false
	d.decrypted = map[Ref]bool{}
	// Font objects may retain large CMaps, embedded programs, and glyph caches.
	// They are immutable after construction but fully reconstructible from data;
	// release them with page state so long scans cannot accumulate one font set
	// per page or subset.
	d.fontCache = map[Ref]*Font{}
	d.fontCacheBytes = 0
	d.fontCacheOrder = nil
	d.fontCacheSizes = nil
	d.trueTypePrograms = nil
	d.trueTypeProgramOwners = nil
	d.fontErrors = nil
	d.fontErrorBytes = 0
	d.fontBuilds = map[Ref]*fontBuild{}
	d.imageBuilds = map[Ref]*imageBuild{}
	d.pageFontCache = map[Ref]map[string]*Font{}
	d.pageFontCacheBytes = 0
	d.pageFontCacheOrder = nil
	d.pageFontCacheSizes = nil
	d.pagePropCache = map[Ref]Dict{}
	d.pagePropCacheBytes = 0
	d.pagePropErrors = nil
	d.pagePropErrorBytes = 0
	d.pageResourceErrors = nil
	d.pageResourceErrorBytes = 0
	d.pageResourceCache = nil
	d.pageResourceCacheBytes = 0
	d.pageUserUnitCache = nil
	d.pageUserUnitCacheBytes = 0
	d.pageUserUnitErrors = nil
	d.pageUserUnitErrorBytes = 0
	d.pageRotationCache = nil
	d.pageRotationCacheBytes = 0
	d.pageRotationErrors = nil
	d.pageRotationErrorBytes = 0
	d.pageBoxCache = nil
	d.pageBoxCacheBytes = 0
	d.xobjectResourceErrors = nil
	d.xobjectResourceErrorBytes = 0
	d.xobjectResourceCache = nil
	d.xobjectResourceCacheBytes = 0
	d.contentRootErrors = nil
	d.contentRootErrorBytes = 0
	d.imageCache = map[Ref]ImageObject{}
	d.imageCacheBytes = 0
	d.inlineImageCache = map[inlineImageKey]Stream{}
	d.inlineImageCacheBytes = 0
	d.decodedStreamCache = map[Ref][]byte{}
	d.decodedStreamCacheBytes = 0
	d.decodedStreamErrors = map[Ref]error{}
	d.decodedStreamErrorBytes = 0
	d.decodedStreamInflight = map[Ref]*decodedStreamDecode{}
	d.objectErrors = nil
	d.objectErrorBytes = 0
	d.actionScriptCache = nil
	d.actionScriptCacheBytes = 0
	d.actionCache = nil
	d.actionCacheBytes = 0
	d.actionErrors = nil
	d.actionErrorBytes = 0
	d.nameTreeCache = nil
	d.nameTreeReady = nil
	d.nameTreeErrors = nil
	d.nameTreeErrorBytes = 0
	d.destinationCache = nil
	d.destinationCacheBytes = 0
	d.destinationErrors = nil
	d.destinationErrorBytes = 0
	d.destinationsCacheBytes = 0
	d.destinationsCacheable = false
	d.destinationsCache = nil
	d.destinationsReady = false
	d.destinationsErr = nil
	d.destinationsRootErr = nil
	d.destinationsRootErrReady = false
	// The logical tagged-text order is a compact document-wide index containing
	// only page references and integer ranks. Retain it so bounded-memory page
	// scans do not rebuild the complete structure tree after every page. Errors
	// remain retryable after release, like the other transient parse failures.
	if d.taggedRanksErr != nil {
		d.taggedRanks = nil
		d.taggedRanksReady = false
		d.taggedRanksErr = nil
	}
	d.structureRoleMapCache = nil
	d.structureRoleMapReady = false
	d.structureRoleMapErr = nil
	d.structureRootErr = nil
	d.structureRootErrReady = false
	d.parentTreeCache = nil
	d.parentTreeCacheBytes = 0
	d.parentTreeMissing = nil
	d.parentTreeErrors = nil
	d.parentTreeErrorBytes = 0
	d.pageStructureCache = nil
	d.pageStructureCacheBytes = 0
	d.pageStructureReady = nil
	d.pageStructureErrors = nil
	d.pageStructureErrorBytes = 0
	d.annotationCache = nil
	d.annotationCacheBytes = 0
	d.annotationCacheSizes = nil
	d.annotationErrors = nil
	d.annotationErrorBytes = 0
	d.annotationRootErrors = nil
	d.annotationRootErrorBytes = 0
	d.outlineCache = nil
	d.outlineCacheBytes = 0
	d.outlineCacheSizes = nil
	d.outlineRootErr = nil
	d.outlineRootErrReady = false
	d.formTopCache = nil
	d.formTopCacheBytes = 0
	d.formTopCacheSizes = nil
	d.formFieldsErr = nil
	d.formFieldsErrReady = false
	d.pageLabelCache = nil
	d.pageLabelCacheBytes = 0
	d.pageLabelsReady = false
	d.pageLabelsErr = nil
	d.pageLabelsValuesReady = false
	d.pageLabelsValuesBytes = 0
	d.pageLabelsValues = nil
	d.pageLabelsValuesErr = nil
	d.pageCountCache = 0
	d.pageCountReady = false
	d.metadataXMLReady = false
	d.metadataXMLCacheable = false
	d.metadataXMLCache = nil
	d.metadataXMLErr = nil
	d.metadataReady = false
	d.metadataCacheable = false
	d.metadataCache = nil
	d.metadataErr = nil
	d.openActionCache = nil
	d.openActionReady = false
	d.openActionErr = nil
	d.openActionErrReady = false
	d.pageCache = nil
	d.pageCacheBytes = 0
	d.pageMissing = nil
	d.pageMissingBytes = 0
	d.pagesCache = nil
	d.pagesCacheBytes = 0
	d.pagesReady = false
	d.pagesErr = nil
	d.pagesErrReady = false
	d.objectRefs = nil
	d.objectRefsReady = false
	d.resetLookupIndexLocked()
}

// Open reads a PDF into an immutable document. The initial implementation
// accepts both conventional xref PDFs and recoverable object scans; xref
// parsing is selected when present and the scan remains a safe fallback for
// damaged files, matching Playa's recovery-oriented behavior.
func Open(path string, options ...documentconfig.OpenOption) (*Document, error) {
	b, release, e := filebuffer.Open(path)
	if e != nil {
		return nil, e
	}
	d, e := openBytesWithOptions(b, applyOpenOptions(options), false)
	if e != nil {
		return nil, errors.Join(e, release())
	}
	d.releaseData = release
	return d, nil
}
func OpenBytes(data []byte, options ...documentconfig.OpenOption) (*Document, error) {
	return openBytesWithOptions(data, applyOpenOptions(options), true)
}

func applyOpenOptions(options []documentconfig.OpenOption) documentconfig.Options {
	optionsValue := documentconfig.Options{}
	for _, option := range options {
		if option != nil {
			option(&optionsValue)
		}
	}
	return optionsValue
}

func openBytesWithOptions(data []byte, opts documentconfig.Options, copyData bool) (*Document, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("playa: empty PDF")
	}
	if !bytes.Contains(data, []byte("%PDF-")) {
		return nil, fmt.Errorf("playa: missing PDF header")
	}
	space := opts.Space
	if space == "" {
		space = CoordinateSpaceScreen
	}
	ownedData := data
	if copyData {
		ownedData = cloneObjectBytes(data)
	}
	d := &Document{data: ownedData, version: pdfVersion(data), space: space, cacheOptions: opts.Cache, cacheOptionsConfigured: opts.CacheSet, objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}, decrypted: map[Ref]bool{}, metadataRefs: map[Ref]bool{}, fontCache: map[Ref]*Font{}, fontBuilds: map[Ref]*fontBuild{}, imageBuilds: map[Ref]*imageBuild{}, pageFontCache: map[Ref]map[string]*Font{}, pagePropCache: map[Ref]Dict{}, imageCache: map[Ref]ImageObject{}, inlineImageCache: map[inlineImageKey]Stream{}, decodedStreamCache: map[Ref][]byte{}, decodedStreamErrors: map[Ref]error{}, decodedStreamInflight: map[Ref]*decodedStreamDecode{}}
	if entries, trailer, e := d.parseXRef(); e == nil {
		d.trailer = trailer
		d.xrefs = entries
	} else {
		if scanErr := d.scanObjects(); scanErr != nil {
			return nil, scanErr
		}
		d.scannedObjects = true
		d.recoveryErr = e
	}
	if _, encrypted := d.trailer[Name("Encrypt")]; encrypted {
		if e := d.configureSecurityWithPresence(opts.Password, opts.PasswordSet || opts.Password != ""); e != nil {
			return nil, e
		}
	}
	return d, nil
}

// CoordinateSpace returns the geometry convention selected when the document
// was opened.
func (d *Document) CoordinateSpace() coordinates.Space {
	if d.space == "" {
		// A zero-value Document is used by low-level model tests and has no
		// open-time options; retain the historical page-space behavior there.
		return CoordinateSpacePage
	}
	return d.space
}

// IsPrintable reports whether the document permissions allow printing.
// Unencrypted documents have no encryption permission restrictions.
func (d *Document) IsPrintable() bool {
	return (!d.encrypted || d.printable)
}

// IsModifiable reports whether the document permissions allow modification.
// Unencrypted documents have no encryption permission restrictions.
func (d *Document) IsModifiable() bool {
	return (!d.encrypted || d.modifiable)
}

// IsExtractable reports whether the document permissions allow content
// extraction. Unencrypted documents have no encryption permission restrictions.
func (d *Document) IsExtractable() bool {
	return (!d.encrypted || d.extractable)
}

func (d *Document) catalog() Dict {
	catalog, _ := d.catalogWithError()
	return catalog
}

func (d *Document) catalogWithError() (Dict, error) {
	if d == nil {
		return nil, errNilDocument
	}
	d.cacheMu.RLock()
	if d.catalogReady {
		catalog := d.catalogCache
		d.cacheMu.RUnlock()
		return catalog, nil
	}
	if d.catalogErrReady {
		err := d.catalogErr
		d.cacheMu.RUnlock()
		return nil, err
	}
	d.cacheMu.RUnlock()
	cacheError := func(err error) (Dict, error) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.catalogReady {
			return d.catalogCache, nil
		}
		if d.catalogErrReady {
			return nil, d.catalogErr
		}
		d.catalogErr = err
		d.catalogErrReady = true
		return nil, err
	}
	cacheCatalog := func(catalog Dict) (Dict, error) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.catalogReady {
			return d.catalogCache, nil
		}
		if d.catalogErrReady {
			return nil, d.catalogErr
		}
		d.catalogCache = catalog
		d.catalogReady = true
		return catalog, nil
	}
	root, ok := d.trailer[Name("Root")]
	if !ok {
		return cacheCatalog(nil)
	}
	value, ok := d.resolveIndirectChain(root)
	if !ok {
		return cacheError(fmt.Errorf("playa: catalog root is not a dictionary"))
	}
	catalog, ok := value.(Dict)
	if !ok {
		return cacheError(fmt.Errorf("playa: catalog root is not a dictionary"))
	}
	return cacheCatalog(catalog)
}

// Catalog returns an independent copy of the resolved document catalog.
func (d *Document) Catalog() Dict {
	catalog, _ := d.CatalogWithError()
	return catalog
}

// CatalogWithError returns an independent copy of the document catalog and
// reports a malformed explicit trailer root. A missing root is nil, nil.
func (d *Document) CatalogWithError() (Dict, error) {
	catalog, err := d.catalogWithError()
	if err != nil {
		return nil, err
	}
	return cloneDict(catalog), nil
}

// Info returns the resolved document information dictionary, if present.
// Metadata provides common fields as decoded strings; Info retains the
// original PDF object values for callers that need their exact types.
func (d *Document) Info() Dict {
	info, _ := d.InfoWithError()
	return info
}

// InfoWithError returns an independent copy of the document information
// dictionary and reports a malformed explicit trailer /Info value.
func (d *Document) InfoWithError() (Dict, error) {
	d.cacheMu.RLock()
	if d.infoReady {
		info := d.infoCache
		d.cacheMu.RUnlock()
		return cloneDict(info), nil
	}
	if d.infoErrReady {
		err := d.infoErr
		d.cacheMu.RUnlock()
		return nil, err
	}
	d.cacheMu.RUnlock()
	cacheError := func(err error) (Dict, error) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.infoReady {
			return cloneDict(d.infoCache), nil
		}
		if d.infoErrReady {
			return nil, d.infoErr
		}
		d.infoErr = err
		d.infoErrReady = true
		return nil, err
	}
	cacheInfo := func(info Dict) (Dict, error) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.infoReady {
			return cloneDict(d.infoCache), nil
		}
		if d.infoErrReady {
			return nil, d.infoErr
		}
		d.infoCache = info
		d.infoReady = true
		return cloneDict(info), nil
	}
	rawInfo, present := d.trailer[Name("Info")]
	if !present {
		return cacheInfo(nil)
	}
	infoValue, ok := d.resolveIndirectChain(rawInfo)
	if !ok {
		return cacheError(fmt.Errorf("playa: document Info is not a dictionary"))
	}
	info, ok := infoValue.(Dict)
	if !ok {
		return cacheError(fmt.Errorf("playa: document Info is not a dictionary"))
	}
	return cacheInfo(info)
}

// Names returns the document catalog's /Names dictionary, if present.
func (d *Document) Names() Dict {
	names, _ := d.NamesWithError()
	return names
}

// NamesWithError returns an independent copy of the catalog's /Names
// dictionary and reports a malformed explicit /Names value. An absent value
// is represented by a nil dictionary and nil error.
func (d *Document) NamesWithError() (Dict, error) {
	d.cacheMu.RLock()
	if d.namesReady {
		names := d.namesCache
		d.cacheMu.RUnlock()
		return cloneDict(names), nil
	}
	if d.namesErrReady {
		err := d.namesErr
		d.cacheMu.RUnlock()
		return nil, err
	}
	d.cacheMu.RUnlock()
	cacheError := func(err error) (Dict, error) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.namesReady {
			return cloneDict(d.namesCache), nil
		}
		if d.namesErrReady {
			return nil, d.namesErr
		}
		d.namesErr = err
		d.namesErrReady = true
		return nil, err
	}
	cacheNames := func(names Dict) (Dict, error) {
		d.cacheMu.Lock()
		defer d.cacheMu.Unlock()
		if d.namesReady {
			return cloneDict(d.namesCache), nil
		}
		if d.namesErrReady {
			return nil, d.namesErr
		}
		d.namesCache = names
		d.namesReady = true
		return cloneDict(names), nil
	}
	catalog, err := d.catalogWithError()
	if err != nil {
		return nil, err
	}
	if catalog == nil {
		return cacheNames(nil)
	}
	rawNames, present := catalog[Name("Names")]
	if !present {
		return cacheNames(nil)
	}
	namesValue, ok := d.resolveIndirectChain(rawNames)
	if !ok {
		return cacheError(fmt.Errorf("playa: catalog Names is not a dictionary"))
	}
	names, ok := namesValue.(Dict)
	if !ok {
		return cacheError(fmt.Errorf("playa: catalog Names is not a dictionary"))
	}
	return cacheNames(names)
}

// IsTagged reports whether the catalog's MarkInfo dictionary marks the PDF as
// tagged.
func (d *Document) IsTagged() bool {
	catalog := d.catalog()
	markInfoValue, _ := d.resolveIndirectChain(catalog[Name("MarkInfo")])
	markInfo, _ := markInfoValue.(Dict)
	markedValue, _ := d.resolveIndirectChain(markInfo[Name("Marked")])
	marked, _ := markedValue.(Bool)
	return bool(marked)
}

// PDFVersion returns the catalog version when present, otherwise the header
// version declared by the PDF file.
func (d *Document) PDFVersion() string {
	catalog := d.catalog()
	versionValue, _ := d.resolveIndirectChain(catalog[Name("Version")])
	if version, ok := versionValue.(Name); ok {
		return string(version)
	}
	return d.version
}

func pdfVersion(data []byte) string {
	start := bytes.Index(data, []byte("%PDF-"))
	if start < 0 {
		return ""
	}
	start += len("%PDF-")
	end := start
	for end < len(data) && data[end] != '\r' && data[end] != '\n' && data[end] != ' ' && data[end] != '\t' {
		end++
	}
	return string(data[start:end])
}

func (d *Document) loadObjectStream(entries map[Ref]xrefEntry, objstm int) (err error) {
	d.objectStreamLifecycle.RLock()
	defer d.objectStreamLifecycle.RUnlock()
	for {
		d.cacheMu.Lock()
		if d.expandedObjStms[objstm] {
			d.cacheMu.Unlock()
			return nil
		}
		if cachedErr := d.objectStreamErrors[objstm]; cachedErr != nil {
			d.cacheMu.Unlock()
			return cachedErr
		}
		if build := d.objectStreamBuilds[objstm]; build != nil {
			d.cacheMu.Unlock()
			<-build.done
			continue
		}
		if d.objectStreamBuilds == nil {
			d.objectStreamBuilds = map[int]*objectStreamBuild{}
		}
		build := &objectStreamBuild{done: make(chan struct{})}
		d.objectStreamBuilds[objstm] = build
		d.cacheMu.Unlock()
		defer func() {
			d.cacheMu.Lock()
			delete(d.objectStreamBuilds, objstm)
			close(build.done)
			d.cacheMu.Unlock()
		}()
		break
	}
	ref := Ref{Object: objstm, Generation: 0}
	resolveObjectStreamRef := func(value Object) Object {
		if candidate, ok := value.(Ref); ok {
			if entry, exists := entries[candidate]; exists && entry.objectStream() == objstm {
				return value
			}
		}
		return d.resolveRaw(value)
	}
	resolveObjectStreamChain := func(value Object) (Object, bool) {
		seen := map[Ref]bool{}
		for {
			ref, ok := value.(Ref)
			if !ok {
				return value, true
			}
			if seen[ref] {
				return nil, false
			}
			seen[ref] = true
			if entry, exists := entries[ref]; exists && entry.objectStream() == objstm {
				return nil, false
			}
			value = d.resolveRaw(value)
		}
	}
	ent, ok := entries[ref]
	if !ok || ent.isFree() {
		return fmt.Errorf("playa: object stream %d not found", objstm)
	}
	if ent.offset <= 0 || ent.offset >= len(d.data) {
		return fmt.Errorf("playa: object stream %d has invalid offset", objstm)
	}
	defer func() {
		if err != nil {
			err = pdfparser.NewParseErrorWithObject(err, ref, ent.offset, "object stream")
			d.cacheMu.Lock()
			if d.objectStreamErrors == nil {
				d.objectStreamErrors = map[int]error{}
			}
			if len(err.Error()) <= d.cacheLimits().ObjectStreamErrorBytes && cacheFits(d.objectStreamErrorBytes, len(err.Error()), d.cacheLimits().ObjectStreamErrorBytes) {
				d.objectStreamErrors[objstm] = err
				d.objectStreamErrorBytes += len(err.Error())
			}
			d.cacheMu.Unlock()
			return
		}
		d.cacheMu.Lock()
		d.expandedObjStms[objstm] = true
		// Expanding an object stream fills values for compact members whose
		// object-number mappings are already present in the xref index. Object
		// enumeration can change from unavailable to available, but the reusable
		// object-number lookup index remains valid.
		d.objectRefs = nil
		d.objectRefsReady = false
		d.pageMissing = nil
		d.pageMissingBytes = 0
		d.pageCountCache = 0
		d.pageCountReady = false
		d.pagesCache = nil
		d.pagesCacheBytes = 0
		d.pagesReady = false
		d.pagesErr = nil
		d.pagesErrReady = false
		d.cacheMu.Unlock()
	}()
	body, ok := indirectBody(d.data, ent.offset, resolveObjectStreamRef)
	if !ok {
		return fmt.Errorf("playa: object stream %d body not found", objstm)
	}
	header := bytes.Index(body, []byte("obj"))
	if header < 0 {
		return fmt.Errorf("playa: object stream %d header not found", objstm)
	}
	o, e := parseObjectBodyBorrowedWithResolver(bytes.TrimSpace(body[header+len("obj"):]), resolveObjectStreamRef)
	if e != nil {
		return e
	}
	if d.encrypted && !d.isEncryptionObject(ref, o) {
		decrypted, err := d.decryptObjectWithMetadataContextWithError(ref, o, d.isCatalogRef(ref))
		if err != nil {
			return err
		}
		o = decrypted
	}
	s, ok := o.(Stream)
	if !ok {
		return fmt.Errorf("playa: object stream %d is not a stream", objstm)
	}
	streamDict := s.DictBorrowed()
	typeValue, _ := resolveObjectStreamChain(streamDict[Name("Type")])
	typ, ok := typeValue.(Name)
	if !ok || typ != Name("ObjStm") {
		return fmt.Errorf("playa: object stream %d has invalid Type", objstm)
	}
	f, p := streamFiltersWithResolver(streamDict, resolveObjectStreamRef)
	raw, e := decodeFiltersLimited(s.DataBorrowed(), f, p, decodedFilterExpansionLimit)
	if e != nil {
		return e
	}
	nValue, _ := resolveObjectStreamChain(streamDict[Name("N")])
	firstValue, _ := resolveObjectStreamChain(streamDict[Name("First")])
	n, nOK := IntValue(nValue)
	first, firstOK := IntValue(firstValue)
	if !nOK || !firstOK || n < 0 || first < 0 {
		return fmt.Errorf("playa: invalid object stream bounds")
	}
	// Reject impossible counts before malformed-stream recovery, whose header
	// walk is proportional to the declared object count.
	if n > len(raw)/3 {
		return fmt.Errorf("playa: object stream count exceeds header data")
	}
	if first > len(raw) {
		recoveredFirst, recovered := recoverObjectStreamHeaderEnd(raw, n)
		if !recovered {
			return fmt.Errorf("playa: invalid object stream bounds")
		}
		first = recoveredFirst
	}
	hp := NewObjectParser(raw[:first])
	ids := make([]int, 0, n)
	offs := make([]int, 0, n)
	seenIDs := make(map[int]struct{}, n)
	for i := 0; i < n; i++ {
		a, e := hp.Parse()
		if e != nil {
			return e
		}
		b, e := hp.Parse()
		if e != nil {
			return e
		}
		id, idOK := IntValue(a)
		off, offOK := IntValue(b)
		if !idOK || !offOK || id < 0 || off < 0 {
			return fmt.Errorf("playa: invalid object stream header")
		}
		if _, exists := seenIDs[id]; exists {
			return fmt.Errorf("playa: duplicate object stream object %d", id)
		}
		seenIDs[id] = struct{}{}
		ids = append(ids, id)
		offs = append(offs, off)
	}
	if remaining := bytes.TrimSpace(raw[hp.Lexer().Pos():first]); len(remaining) != 0 {
		return fmt.Errorf("playa: trailing object stream header data")
	}
	for _, token := range hp.PendingTokens() {
		if token.Offset() < first {
			return fmt.Errorf("playa: trailing object stream header data")
		}
	}
	objectDataLength := len(raw) - first
	for i, offset := range offs {
		if offset < 0 || offset >= objectDataLength || (i > 0 && offset <= offs[i-1]) {
			return fmt.Errorf("playa: invalid object stream offsets")
		}
	}
	for i, id := range ids {
		entry, present := entries[Ref{Object: id, Generation: 0}]
		if !present || entry.objectStream() != objstm {
			continue
		}
		if entry.objectIndex() != i {
			return fmt.Errorf("playa: object stream index mismatch for object %d", id)
		}
		start := first + offs[i]
		stop := len(raw)
		if i+1 < len(offs) {
			stop = first + offs[i+1]
		}
		if start < 0 || start >= stop || stop > len(raw) {
			return fmt.Errorf("playa: invalid object stream object bounds")
		}
		v, e := NewObjectParser(bytes.TrimSpace(raw[start:stop])).Parse()
		if e != nil {
			return e
		}
		if d.encrypted && !d.isEncryptionObject(Ref{Object: id, Generation: 0}, v) {
			decrypted, err := d.decryptObjectWithMetadataContextWithError(Ref{Object: id, Generation: 0}, v, false)
			if err != nil {
				return err
			}
			v = decrypted
		}
		d.cacheMu.Lock()
		d.storeObjectLocked(Ref{Object: id, Generation: 0}, v)
		d.cacheMu.Unlock()
	}
	return nil
}

func recoverObjectStreamHeaderEnd(raw []byte, count int) (int, bool) {
	parser := NewObjectParser(raw)
	for i := 0; i < count; i++ {
		if _, err := parser.Parse(); err != nil {
			return 0, false
		}
		if _, err := parser.Parse(); err != nil {
			return 0, false
		}
	}
	first := parser.Lexer().Pos()
	for _, token := range parser.PendingTokens() {
		if token.Offset() < first {
			first = token.Offset()
		}
	}
	if first <= 0 || first >= len(raw) || len(bytes.TrimSpace(raw[first:])) == 0 {
		return 0, false
	}
	return first, true
}

// indirectBody returns exactly one indirect object. Stream bytes are binary,
// so searching for the first endobj is not safe; use the declared stream
// length whenever possible.
func indirectBody(data []byte, offset int, resolvers ...func(Object) Object) ([]byte, bool) {
	if offset < 0 || offset >= len(data) {
		return nil, false
	}
	rest := data[offset:]
	header := bytes.Index(rest, []byte("obj"))
	if header < 0 {
		return nil, false
	}
	bodyStart := header + len("obj")
	body := rest[bodyStart:]
	parser := NewObjectParser(body)
	obj, err := parser.Parse()
	if err == nil {
		token, tokenErr := parser.NextToken()
		if tokenErr == nil && token.Kind() == TokenKeyword && token.Text() == "endobj" {
			return rest[:bodyStart+token.Offset()], true
		}
		if tokenErr == nil && token.Kind() == TokenKeyword && token.Text() == "stream" {
			streamAt := bodyStart + token.Offset()
			if dict, ok := obj.(Dict); ok {
				length := dict[Name("Length")]
				if len(resolvers) > 0 {
					resolve := resolvers[0]
					seen := map[Ref]bool{}
					for {
						ref, ok := length.(Ref)
						if !ok || seen[ref] {
							break
						}
						seen[ref] = true
						resolved := resolve(length)
						if resolved == nil {
							break
						}
						length = resolved
					}
				}
				if n, ok := IntValue(length); ok {
					start := streamAt + len("stream")
					if start < len(rest) && rest[start] == '\r' {
						start++
					}
					if start < len(rest) && rest[start] == '\n' {
						start++
					}
					if n >= 0 && start <= len(rest) && n <= len(rest)-start {
						end := start + n
						endStream := bytes.Index(rest[end:], []byte("endstream"))
						if endStream >= 0 {
							endObj := bytes.Index(rest[end+endStream+len("endstream"):], []byte("endobj"))
							if endObj >= 0 {
								return rest[:end+endStream+len("endstream")+endObj], true
							}
						}
					}
				}
			}
		}
	}
	end := bytes.Index(body, []byte("endobj"))
	if end < 0 {
		return nil, false
	}
	return rest[:bodyStart+end], true
}

var objRE = regexp.MustCompile(`(?m)(\d+)\s+(\d+)\s+obj\s*`)

func xrefObjectOffset(data []byte, offset int) int {
	const maxLeadingWhitespace = 32
	end := offset + maxLeadingWhitespace
	if end > len(data) {
		end = len(data)
	}
	for offset < end && isContentWhitespace(data[offset]) {
		offset++
	}
	return offset
}

func parseScannedObjectHeader(header []byte) (int, int, bool) {
	parseNumber := func(at int) (int, int, bool) {
		if at >= len(header) || header[at] < '0' || header[at] > '9' {
			return 0, at, false
		}
		value := 0
		maxInt := int(^uint(0) >> 1)
		for at < len(header) && header[at] >= '0' && header[at] <= '9' {
			digit := int(header[at] - '0')
			if value > (maxInt-digit)/10 {
				return 0, at, false
			}
			value = value*10 + digit
			at++
		}
		return value, at, true
	}
	id, at, ok := parseNumber(0)
	if !ok || at >= len(header) || header[at] != ' ' && header[at] != '\t' && header[at] != '\r' && header[at] != '\n' && header[at] != '\f' {
		return 0, 0, false
	}
	for at < len(header) && (header[at] == ' ' || header[at] == '\t' || header[at] == '\r' || header[at] == '\n' || header[at] == '\f') {
		at++
	}
	generation, at, ok := parseNumber(at)
	if !ok || at >= len(header) || header[at] != ' ' && header[at] != '\t' && header[at] != '\r' && header[at] != '\n' && header[at] != '\f' {
		return 0, 0, false
	}
	for at < len(header) && (header[at] == ' ' || header[at] == '\t' || header[at] == '\r' || header[at] == '\n' || header[at] == '\f') {
		at++
	}
	if at+3 > len(header) || !bytes.Equal(header[at:at+3], []byte("obj")) {
		return 0, 0, false
	}
	return id, generation, true
}

func (d *Document) scanObjects() error {
	if d.objects == nil {
		d.objects = map[Ref]Object{}
	}
	if d.xrefs == nil {
		d.xrefs = map[Ref]xrefEntry{}
	}
	lastObjectEnd := 0
	for offset := 0; offset < len(d.data); {
		match := objRE.FindIndex(d.data[offset:])
		if match == nil {
			break
		}
		m := [2]int{offset + match[0], offset + match[1]}
		if objectHeaderHiddenRange(d.data, offset, m[0]) {
			offset = m[1]
			continue
		}
		id, gen, ok := parseScannedObjectHeader(d.data[m[0]:m[1]])
		if !ok {
			offset = m[1]
			continue
		}
		body, ok := indirectBody(d.data, m[0], d.resolveRaw)
		if !ok {
			offset = m[1]
			continue
		}
		// Advance past the complete object body. This prevents object-looking
		// bytes inside a stream from being mistaken for nested indirect objects.
		offset = m[0] + len(body)
		if offset > lastObjectEnd {
			lastObjectEnd = offset
		}
		at := bytes.Index(body, []byte("obj"))
		if at < 0 {
			continue
		}
		o, e := parseObjectBodyBorrowedWithResolver(bytes.TrimSpace(body[at+len("obj"):]), d.resolveRaw)
		if e != nil {
			continue
		}
		ref := Ref{Object: id, Generation: gen}
		// Damaged files can contain repeated indirect declarations. Recovery
		// keeps the first declaration, matching the object-stream index pass
		// and avoiding later garbage replacing an already usable object.
		if _, exists := d.xrefs[ref]; exists {
			continue
		}
		d.xrefs[ref] = newXRefEntry(m[0], false, false, true)
		d.storeObjectLocked(ref, o)
	}
	d.indexScannedObjectStreams()
	trailer := lastPDFKeyword(d.data, []byte("trailer"))
	if trailer >= lastObjectEnd {
		body := d.data[trailer+len("trailer"):]
		if startxref := lastPDFKeyword(body, []byte("startxref")); startxref >= 0 {
			body = body[:startxref]
		}
		parser := NewObjectParser(bytes.TrimSpace(body))
		if value, err := parser.Parse(); err == nil {
			trailing, trailingErr := parser.NextToken()
			if trailingErr != nil || (trailing.Kind() != TokenEOF && (trailing.Kind() != TokenKeyword || trailing.Text() != "startxref")) {
				return nil
			}
			if dict, ok := value.(Dict); ok {
				d.trailer = dict
			}
		}
	}
	return nil
}

// recoverContentObjectNumber mirrors Playa's lazy fallback-xref rebuild for a
// structurally stale ordinary-object entry. It scans physical objects in
// source order, repairs only the requested object-number entry, and leaves
// filter/decryption failures observable to the caller.
type recoveredXRefObject struct {
	ref    Ref
	offset int
}

func (d *Document) recoverContentObjectNumber(objectNumber int) (Object, bool, error) {
	candidates := make(map[int]recoveredXRefObject)
	for offset := 0; offset < len(d.data); {
		match := objRE.FindIndex(d.data[offset:])
		if match == nil {
			break
		}
		start := offset + match[0]
		headerEnd := offset + match[1]
		id, generation, ok := parseScannedObjectHeader(d.data[start:headerEnd])
		if !ok {
			offset = headerEnd
			continue
		}
		body, ok := indirectBody(d.data, start, d.resolveRaw)
		if !ok {
			offset = headerEnd
			continue
		}
		offset = start + len(body)
		ref := Ref{Object: id, Generation: generation}
		current, exists := candidates[id]
		if exists && generation < current.ref.Generation {
			continue
		}
		candidates[id] = recoveredXRefObject{ref: ref, offset: start}
	}
	candidate, found := candidates[objectNumber]
	if !found {
		return nil, false, nil
	}
	candidateValue, parsed := d.recoveredObjectValue(candidate, candidates)
	if !parsed {
		return nil, false, nil
	}
	compactEntries := d.recoveredObjectStreamXRefs(candidates)
	if d.encrypted && !d.isEncryptionObject(candidate.ref, candidateValue) {
		var err error
		candidateValue, err = d.decryptObjectWithMetadataContextWithError(candidate.ref, candidateValue, d.isCatalogRef(candidate.ref))
		if err != nil {
			return nil, false, err
		}
	}
	d.cacheMu.Lock()
	rebuilt := make(map[Ref]xrefEntry, len(candidates))
	for _, recovered := range candidates {
		rebuilt[recovered.ref] = newXRefEntry(recovered.offset, false, false, true)
	}
	for ref, entry := range compactEntries {
		if _, ordinary := candidates[ref.Object]; !ordinary {
			rebuilt[ref] = entry
		}
	}
	d.xrefs = rebuilt
	d.xrefHistory = nil
	d.objects = map[Ref]Object{}
	d.objectCacheBytes = 0
	d.decrypted = map[Ref]bool{}
	d.storeObjectLocked(candidate.ref, candidateValue)
	d.contentXRefRecovered = true
	d.recoveredTrailerSize = len(rebuilt)
	d.scannedObjects = true
	d.objectRefs = nil
	d.objectRefsReady = false
	if d.encrypted {
		d.decrypted[candidate.ref] = true
	}
	d.objectErrors = nil
	d.objectErrorBytes = 0
	d.expandedObjStms = map[int]bool{}
	d.objectStreamErrors = nil
	d.objectStreamErrorBytes = 0
	d.resetLookupIndexLocked()
	d.cacheMu.Unlock()
	d.cacheLookup(objectNumber, candidate.ref, true)
	return candidateValue, true, nil
}

func (d *Document) recoveredObjectValue(candidate recoveredXRefObject, candidates map[int]recoveredXRefObject) (Object, bool) {
	identity := func(value Object) Object { return value }
	body, ok := indirectBody(d.data, candidate.offset, identity)
	if !ok {
		return nil, false
	}
	header := bytes.Index(body, []byte("obj"))
	if header < 0 {
		return nil, false
	}
	value, err := parseObjectBodyBorrowedWithResolver(bytes.TrimSpace(body[header+len("obj"):]), identity)
	return value, err == nil
}

func (d *Document) recoveredObjectStreamXRefs(candidates map[int]recoveredXRefObject) map[Ref]xrefEntry {
	ordered := make([]recoveredXRefObject, 0, len(candidates))
	for _, candidate := range candidates {
		ordered = append(ordered, candidate)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].offset < ordered[j].offset })
	entries := make(map[Ref]xrefEntry)
	for _, candidate := range ordered {
		value, parsed := d.recoveredObjectValue(candidate, candidates)
		stream, ok := value.(Stream)
		if !parsed || !ok {
			continue
		}
		dict := stream.DictBorrowed()
		if typ, ok := d.resolveRecoveredObject(dict[Name("Type")], candidates).(Name); !ok || typ != Name("ObjStm") {
			continue
		}
		resolve := func(value Object) Object { return d.resolveRecoveredObject(value, candidates) }
		filters, parms := streamFiltersWithResolver(dict, resolve)
		raw, err := decodeFiltersLimited(stream.DataBorrowed(), filters, parms, decodedFilterExpansionLimit)
		if err != nil {
			continue
		}
		n, ok := IntValue(resolve(dict[Name("N")]))
		if !ok || n <= 0 {
			continue
		}
		parser := NewObjectParser(raw)
		var values []Object
		for {
			value, err := parser.Parse()
			if err != nil {
				break
			}
			values = append(values, value)
		}
		if n > len(values)/2 {
			n = len(values) / 2
		}
		for index := 0; index < n; index++ {
			id, ok := IntValue(values[index*2])
			if !ok || id < 0 {
				continue
			}
			entries[Ref{Object: id}] = newCompressedXRefEntry(candidate.ref.Object, index, false, true)
		}
	}
	return entries
}

func (d *Document) resolveRecoveredObject(value Object, candidates map[int]recoveredXRefObject) Object {
	seen := map[int]bool{}
	for {
		ref, ok := value.(Ref)
		if !ok || seen[ref.Object] {
			return value
		}
		seen[ref.Object] = true
		candidate, ok := candidates[ref.Object]
		if !ok {
			return value
		}
		resolved, ok := d.recoveredObjectValue(candidate, candidates)
		if !ok {
			return value
		}
		value = resolved
	}
}

// indexScannedObjectStreams rebuilds the compressed-object part of the xref
// index when recovery had to scan indirect objects instead of reading a valid
// xref section. The stream expansion path already validates and parses the
// object bodies; this pass only recovers the compact header index it needs.
func (d *Document) indexScannedObjectStreams() {
	objstms := make([]Ref, 0)
	for ref, entry := range d.xrefs {
		if entry.isFree() || entry.isCompressed() {
			continue
		}
		if _, ok := d.objects[ref].(Stream); ok {
			objstms = append(objstms, ref)
		}
	}
	sort.Slice(objstms, func(i, j int) bool {
		if objstms[i].Object != objstms[j].Object {
			return objstms[i].Object < objstms[j].Object
		}
		return objstms[i].Generation < objstms[j].Generation
	})
	for _, ref := range objstms {
		stream, ok := d.objects[ref].(Stream)
		if !ok {
			continue
		}
		streamDict := stream.DictBorrowed()
		typeValue := d.resolveRaw(streamDict[Name("Type")])
		if typ, ok := typeValue.(Name); !ok || typ != Name("ObjStm") {
			continue
		}
		filters, parms := streamFiltersWithResolver(streamDict, d.resolveRaw)
		raw, err := decodeFiltersLimited(stream.DataBorrowed(), filters, parms, decodedFilterExpansionLimit)
		if err != nil {
			continue
		}
		nValue := d.resolveRaw(streamDict[Name("N")])
		firstValue := d.resolveRaw(streamDict[Name("First")])
		n, nOK := IntValue(nValue)
		first, firstOK := IntValue(firstValue)
		if !nOK || !firstOK || n < 0 || first < 0 || first > len(raw) || n > len(raw)/3 {
			continue
		}
		parser := NewObjectParser(raw[:first])
		ids := make([]int, 0, n)
		offsets := make([]int, 0, n)
		seen := make(map[int]struct{}, n)
		valid := true
		for i := 0; i < n; i++ {
			idValue, idErr := parser.Parse()
			offsetValue, offsetErr := parser.Parse()
			id, idOK := IntValue(idValue)
			offset, offsetOK := IntValue(offsetValue)
			if idErr != nil || offsetErr != nil || !idOK || !offsetOK || id < 0 || offset < 0 {
				valid = false
				break
			}
			if _, exists := seen[id]; exists {
				valid = false
				break
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
			offsets = append(offsets, offset)
		}
		if !valid {
			continue
		}
		for i := range ids {
			if i > 0 && offsets[i] <= offsets[i-1] {
				valid = false
				break
			}
			if offsets[i] >= len(raw)-first {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		for i, id := range ids {
			compressedRef := Ref{Object: id}
			if _, exists := d.xrefs[compressedRef]; exists {
				// Recovery is best effort, but it must remain deterministic for
				// malformed files that declare one object in multiple ObjStms.
				// Preserve the first source-order declaration instead of letting
				// map iteration decide which body is visible.
				continue
			}
			d.xrefs[compressedRef] = newCompressedXRefEntry(ref.Object, i, false, true)
		}
	}
}

func lastPDFKeyword(data, keyword []byte) int {
	for end := len(data); ; {
		index := bytes.LastIndex(data[:end], keyword)
		if index < 0 {
			return -1
		}
		if (index == 0 || !pdfIdentifierByte(data[index-1])) && !objectHeaderHidden(data, index) {
			return index
		}
		end = index
	}
}

func pdfIdentifierByte(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '_'
}

func objectHeaderHidden(data []byte, offset int) bool {
	return objectHeaderHiddenRange(data, 0, offset)
}

func objectHeaderHiddenRange(data []byte, begin, end int) bool {
	if begin < 0 || end < begin || end > len(data) {
		return false
	}
	stringDepth := 0
	escaped := false
	hexString := false
	comment := false
	for index := begin; index < end; index++ {
		value := data[index]
		if comment {
			if value == '\n' || value == '\r' {
				comment = false
			}
			continue
		}
		if stringDepth > 0 {
			if escaped {
				escaped = false
				continue
			}
			switch value {
			case '\\':
				escaped = true
			case '(':
				stringDepth++
			case ')':
				stringDepth--
			}
			continue
		}
		if hexString {
			if value == '>' {
				hexString = false
			}
			continue
		}
		switch value {
		case '%':
			comment = true
		case '(':
			stringDepth = 1
		case '<':
			if index == begin || data[index-1] != '<' {
				if index+1 >= len(data) || data[index+1] != '<' {
					hexString = true
				}
			}
		}
	}
	return comment || stringDepth > 0 || hexString
}

func parseObjectBody(body []byte) (Object, error) {
	return parseObjectBodyWithResolver(body, nil)
}

func parseObjectBodyWithResolver(body []byte, resolve func(Object) Object) (Object, error) {
	return parseObjectBodyWithStreamMode(body, resolve, false)
}

func parseObjectBodyBorrowedWithResolver(body []byte, resolve func(Object) Object) (Object, error) {
	return parseObjectBodyWithStreamMode(body, resolve, true)
}

func parseObjectBodyWithStreamMode(body []byte, resolve func(Object) Object, borrowStream bool) (Object, error) {
	resolveChain := func(value Object) Object {
		if resolve == nil {
			return value
		}
		seen := map[Ref]bool{}
		for {
			ref, ok := value.(Ref)
			if !ok {
				return value
			}
			if seen[ref] {
				return value
			}
			seen[ref] = true
			resolved := resolve(value)
			if resolved == nil {
				return resolved
			}
			value = resolved
		}
	}
	p := NewObjectParser(body)
	o, err := p.Parse()
	if err != nil {
		return nil, err
	}
	token, err := p.NextToken()
	if err != nil {
		return nil, err
	}
	if token.Kind() == TokenKeyword && token.Text() == "stream" {
		dict, ok := o.(Dict)
		if !ok {
			return nil, withParseContext(fmt.Errorf("playa: stream prefix is not dictionary"), token.Offset(), "stream")
		}
		at := token.Offset()
		start := at + len("stream")
		if start < len(body) && body[start] == '\r' {
			start++
		}
		if start < len(body) && body[start] == '\n' {
			start++
		}
		length := dict[Name("Length")]
		length = resolveChain(length)
		if n, ok := IntValue(length); ok && n >= 0 && start <= len(body) && n <= len(body)-start {
			end := start + n
			data := body[start:end]
			if !borrowStream {
				data = cloneStreamBytes(data)
			}
			return newStream(dict, data), nil
		}
		end := bytes.LastIndex(body, []byte("endstream"))
		if end < start {
			return nil, withParseContext(fmt.Errorf("playa: malformed stream"), at, "stream")
		}
		data := body[start:end]
		if !borrowStream {
			data = cloneStreamBytes(data)
		}
		return newStream(dict, data), nil
	}
	if token.Kind() != TokenEOF {
		return nil, wrapParseError(fmt.Errorf("trailing object data"), token.Offset(), "object")
	}
	return o, nil
}
func (d *Document) Object(ref Ref) (Object, bool) {
	o, err := d.ObjectWithError(ref)
	return o, err == nil
}

// ObjectWithError returns an independent snapshot of ref and preserves the
// typed resolution or parse error for callers that need diagnostics.
func (d *Document) ObjectWithError(ref Ref) (Object, error) {
	o, err := d.resolveRefRaw(ref)
	if err != nil {
		return nil, err
	}
	o, err = d.publicObjectValue(ref, o)
	if err != nil {
		return nil, err
	}
	return cloneGraphicsObject(o), nil
}

// Objects returns borrowed indirect objects in physical source order for a
// document opened from bytes. Duplicate revisions are retained, and members
// of an object stream follow that stream object. Map-only synthetic documents
// use indexed reference order. Call IndirectObject.Finalize or ValueCopy when
// an independent snapshot is required.
//
// The sequence can be traversed more than once and stops as soon as the
// consumer stops yielding.
func (d *Document) Objects() iter.Seq2[IndirectObject, error] {
	if len(d.data) > 0 {
		return d.sourceObjects()
	}
	return d.indexedObjects()
}

func (d *Document) indexedObjects() iter.Seq2[IndirectObject, error] {
	return func(yield func(IndirectObject, error) bool) {
		refs := d.objectReferences()
		for _, ref := range refs {
			value, err := d.resolveRefRaw(ref)
			if err != nil {
				yield(IndirectObject{}, err)
				return
			}
			value, err = d.publicObjectValue(ref, value)
			if err != nil {
				yield(IndirectObject{}, err)
				return
			}
			if !yield(newIndirectObject(ref, value), nil) {
				return
			}
		}
	}
}

// sourceObjects follows Playa's document.objects contract: indirect objects
// are yielded in physical source order, duplicate revisions are retained, and
// objects embedded in an object stream immediately follow that stream object.
// The traversal is intentionally lazy; it keeps no second copy of the source
// index and stops scanning as soon as the consumer stops yielding.
func (d *Document) sourceObjects() iter.Seq2[IndirectObject, error] {
	return func(yield func(IndirectObject, error) bool) {
		for offset := 0; offset < len(d.data); {
			match := objRE.FindIndex(d.data[offset:])
			if match == nil {
				return
			}
			start := offset + match[0]
			headerEnd := offset + match[1]
			if objectHeaderHiddenRange(d.data, offset, start) {
				offset = headerEnd
				continue
			}
			id, generation, ok := parseScannedObjectHeader(d.data[start:headerEnd])
			if !ok {
				offset = headerEnd
				continue
			}
			body, ok := indirectBody(d.data, start, d.resolveRaw)
			if !ok {
				offset = headerEnd
				continue
			}
			offset = start + len(body)
			objectStart := bytes.Index(body, []byte("obj"))
			if objectStart < 0 {
				continue
			}
			value, err := parseObjectBodyBorrowedWithResolver(bytes.TrimSpace(body[objectStart+len("obj"):]), d.resolveRaw)
			if err != nil {
				yield(IndirectObject{}, err)
				return
			}
			ref := Ref{Object: id, Generation: generation}
			value, err = d.publicObjectValue(ref, value)
			if err != nil {
				yield(IndirectObject{}, err)
				return
			}
			if !yield(newIndirectObject(ref, value), nil) {
				return
			}
			stream, ok := value.(Stream)
			if !ok {
				continue
			}
			embedded, err := d.sourceObjectStreamObjects(ref, stream)
			if err != nil {
				yield(IndirectObject{}, err)
				return
			}
			for _, object := range embedded {
				if !yield(object, nil) {
					return
				}
			}
		}
	}
}

func (d *Document) sourceObjectStreamObjects(ref Ref, stream Stream) ([]IndirectObject, error) {
	streamDict := stream.DictBorrowed()
	typeValue := d.resolveRaw(streamDict[Name("Type")])
	if typ, ok := typeValue.(Name); !ok || typ != Name("ObjStm") {
		return nil, nil
	}
	filters, parms := streamFiltersWithResolver(streamDict, d.resolveRaw)
	raw, err := decodeFiltersLimited(stream.DataBorrowed(), filters, parms, decodedFilterExpansionLimit)
	if err != nil {
		return nil, err
	}
	n, nOK := IntValue(d.resolveRaw(streamDict[Name("N")]))
	first, firstOK := IntValue(d.resolveRaw(streamDict[Name("First")]))
	if !nOK || !firstOK || n < 0 || first < 0 || first > len(raw) || n > len(raw)/3 {
		return nil, fmt.Errorf("playa: invalid object stream bounds for %s", ref)
	}
	header := NewObjectParser(raw[:first])
	ids := make([]int, 0, n)
	offsets := make([]int, 0, n)
	for index := 0; index < n; index++ {
		idValue, idErr := header.Parse()
		offsetValue, offsetErr := header.Parse()
		id, idOK := IntValue(idValue)
		objectOffset, offsetOK := IntValue(offsetValue)
		if idErr != nil || offsetErr != nil || !idOK || !offsetOK || id < 0 || objectOffset < 0 {
			return nil, fmt.Errorf("playa: invalid object stream header for %s", ref)
		}
		ids = append(ids, id)
		offsets = append(offsets, objectOffset)
	}
	objectDataLength := len(raw) - first
	objects := make([]IndirectObject, 0, n)
	for index, id := range ids {
		objectOffset := offsets[index]
		endOffset := objectDataLength
		if index+1 < len(offsets) {
			endOffset = offsets[index+1]
		}
		start := first + objectOffset
		end := first + endOffset
		if objectOffset < 0 || endOffset <= objectOffset || start < first || end > len(raw) {
			return nil, fmt.Errorf("playa: invalid object stream object bounds for %s", ref)
		}
		value, parseErr := NewObjectParser(bytes.TrimSpace(raw[start:end])).Parse()
		if parseErr != nil {
			return nil, parseErr
		}
		childRef := Ref{Object: id}
		value, parseErr = d.publicObjectValue(childRef, value)
		if parseErr != nil {
			return nil, parseErr
		}
		objects = append(objects, newIndirectObject(childRef, value))
	}
	return objects, nil
}

// ObjectCount returns the number of currently indexed indirect objects.
// Like Playa's len(document), the value can change after lazy xref loading or
// recovery discovers additional objects.
func (d *Document) ObjectCount() int {
	return len(d.objectReferences())
}

// ObjectRefs yields object references in stable numeric/generation order.
// References are produced from the same lazy index as Objects and the
// sequence can be traversed repeatedly without retaining object values.
func (d *Document) ObjectRefs() iter.Seq2[Ref, error] {
	return func(yield func(Ref, error) bool) {
		for _, ref := range d.objectReferences() {
			if !yield(ref, nil) {
				return
			}
		}
	}
}

func (d *Document) objectReferences() []Ref {
	d.cacheMu.RLock()
	if d.objectRefsReady {
		refs := d.objectRefs
		d.cacheMu.RUnlock()
		return refs
	}
	d.cacheMu.RUnlock()

	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if d.objectRefsReady {
		return d.objectRefs
	}
	refs := make([]Ref, 0, len(d.objects)+len(d.xrefs))
	seen := make(map[Ref]bool, len(d.objects)+len(d.xrefs))
	for r, entry := range d.xrefs {
		if entry.isFree() || seen[r] {
			continue
		}
		seen[r] = true
		refs = append(refs, r)
	}
	for r := range d.objects {
		if seen[r] {
			continue
		}
		seen[r] = true
		refs = append(refs, r)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Object != refs[j].Object {
			return refs[i].Object < refs[j].Object
		}
		return refs[i].Generation < refs[j].Generation
	})
	d.objectRefs = refs
	d.objectRefsReady = true
	return refs
}

// CollectObjects materializes Objects for adapters that need a complete list.
func (d *Document) CollectObjects() ([]IndirectObject, error) {
	out := []IndirectObject{}
	for object, err := range d.Objects() {
		if err != nil {
			return nil, err
		}
		out = append(out, object)
	}
	return out, nil
}
func (d *Document) Trailer() Dict {
	d.cacheMu.RLock()
	defer d.cacheMu.RUnlock()
	trailer := cloneDict(d.trailer)
	if d.contentXRefRecovered {
		if trailer == nil {
			trailer = Dict{}
		}
		trailer[Name("Size")] = Number(d.recoveredTrailerSize)
	}
	return trailer
}

// Buffer returns a defensive copy of the original PDF bytes. It is useful for
// callers that need to inspect raw streams or retain offsets from Tokens.
func (d *Document) Buffer() []byte { return cloneObjectBytes(d.data) }

// BufferDigest returns the original PDF length and SHA-256 without allocating
// a defensive copy of the complete document buffer.
func (d *Document) BufferDigest() (int, string) {
	digest := sha256.Sum256(d.data)
	return len(d.data), hex.EncodeToString(digest[:])
}

// Tokens lexes the original PDF buffer in source order.
//
// A fresh lexer is created for every traversal. Raw token bytes are copied so
// yielded tokens remain independent of the document storage, and a consumer
// that stops early never lexes the remainder of the file.
func (d *Document) Tokens() iter.Seq2[Token, error] {
	return func(yield func(Token, error) bool) {
		lexer := NewSourceLexer(d.data)
		for {
			token, err := lexer.Next()
			if err != nil {
				// Playa's document-wide lexer treats an unterminated token in
				// arbitrary stream bytes as end-of-input. Document.tokens is a
				// raw source view, so do not turn random encrypted stream bytes
				// into a document failure.
				return
			}
			if token.Kind() == TokenEOF {
				return
			}
			token = token.Finalize()
			if !yield(token, nil) {
				return
			}
		}
	}
}

// TokensFrom lexes the original PDF buffer beginning at an exact source byte
// offset. It is the bounded counterpart to Tokens for xref-addressed internal
// consumers and preserves absolute token offsets without copying the document.
func (d *Document) TokensFrom(offset int) iter.Seq2[Token, error] {
	return func(yield func(Token, error) bool) {
		if offset < 0 || offset > len(d.data) {
			yield(Token{}, fmt.Errorf("playa: source token offset %d is out of range", offset))
			return
		}
		lexer := NewSourceLexer(d.data)
		lexer.SetPos(offset)
		for {
			token, err := lexer.Next()
			if err != nil {
				return
			}
			if token.Kind() == TokenEOF {
				return
			}
			token = token.Finalize()
			if !yield(token, nil) {
				return
			}
		}
	}
}

// CollectTokens materializes Tokens for callers that need a complete list.
func (d *Document) CollectTokens() ([]Token, error) {
	var out []Token
	for token, err := range d.Tokens() {
		if err != nil {
			return nil, err
		}
		out = append(out, token)
	}
	return out, nil
}

func (d *Document) RevisionCount() int {
	if d.trailer == nil {
		return 0
	}
	count := 1
	trailer := d.trailer
	seen := map[int]bool{}
	for {
		prevValue, _ := d.resolveIndirectChain(trailer[Name("Prev")])
		prev, ok := IntValue(prevValue)
		if !ok || prev <= 0 || seen[prev] {
			break
		}
		seen[prev] = true
		_, older, err := d.parseXRefAt(prev)
		if err != nil {
			break
		}
		count++
		trailer = older
	}
	return count
}

func (d *Document) PreviousXRefOffset() (int, bool) {
	if d.trailer == nil {
		return 0, false
	}
	v, ok := d.trailer[Name("Prev")]
	if !ok {
		return 0, false
	}
	resolved, _ := d.resolveIndirectChain(v)
	n, ok := IntValue(resolved)
	return n, ok
}

func (d *Document) XRefHistory() []int {
	out := []int{}
	seen := map[int]bool{}
	off, _ := d.PreviousXRefOffset()
	for off > 0 && !seen[off] {
		seen[off] = true
		out = append(out, off)
		_, trailer, err := d.parseXRefAt(off)
		if err != nil {
			break
		}
		prevValue, _ := d.resolveIndirectChain(trailer[Name("Prev")])
		off, _ = IntValue(prevValue)
	}
	return out
}
func (d *Document) PageCount() int {
	d.cacheMu.RLock()
	if d.pageCountReady {
		count := d.pageCountCache
		d.cacheMu.RUnlock()
		return count
	}
	d.cacheMu.RUnlock()
	count, err := d.countPages()
	if err != nil {
		return 0
	}
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if d.pageCountReady {
		return d.pageCountCache
	}
	d.pageCountCache = count
	d.pageCountReady = true
	return d.pageCountCache
}
