package document

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rc4"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/documentdata"
)

func TestErrEncryptedDoesNotClaimUnsupportedFeature(t *testing.T) {
	if ErrEncrypted.Error() != "playa: encrypted PDF" {
		t.Fatalf("ErrEncrypted = %q", ErrEncrypted)
	}
}

func TestDocumentPermissionsDefaultToAllowed(t *testing.T) {
	d, err := OpenBytes([]byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !d.IsPrintable() || !d.IsModifiable() || !d.IsExtractable() {
		t.Fatalf("unencrypted permissions = printable:%v modifiable:%v extractable:%v", d.IsPrintable(), d.IsModifiable(), d.IsExtractable())
	}
}

func TestPermissionsForPDFPermissionBits(t *testing.T) {
	printable, modifiable, extractable := permissionsFor(4 | 16)
	if !printable || modifiable || !extractable {
		t.Fatalf("permission bits = printable:%v modifiable:%v extractable:%v", printable, modifiable, extractable)
	}
}

func TestOpenBytesOwnsACopyOfInput(t *testing.T) {
	data := []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n")
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	data[0] = 'X'
	if d.data[0] != '%' {
		t.Fatalf("document input was not copied: %q", d.data[:8])
	}
}

func TestOpenOwnsFileBackingUntilClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mapped.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if d.releaseData == nil {
		t.Fatal("Open did not retain a file-backing release hook")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if d.releaseData != nil {
		t.Fatal("Close retained the file-backing release hook")
	}
	if err := d.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestDocumentCloseReleasesDerivedCaches(t *testing.T) {
	d := &Document{
		data:               []byte("pdf bytes"),
		version:            "1.7",
		space:              CoordinateSpaceScreen,
		encrypted:          true,
		encryptionKey:      []byte("key"),
		encryptionRevision: 4,
		encryptionMethod:   "AESV2",
		printable:          true,
		modifiable:         true,
		extractable:        true,
		metadataExcluded:   true,
		metadataRefs:       map[Ref]bool{{Object: 8}: true},
		trailer:            Dict{Name("Root"): Ref{Object: 1}},
		xrefs:              map[Ref]xrefEntry{{Object: 9}: {}},
		objects:            map[Ref]Object{{Object: 1}: Dict{}},
		fontCache:          map[Ref]*Font{{Object: 2}: NewSimpleFont("F")},
		pageCache:          map[Ref]Page{{Object: 3}: {}},
		pagesCache:         []Page{{}},
		pagesErr:           errors.New("cached"), pagesErrReady: true,
		metadataCache:         Metadata{"Title": "cached"},
		decodedStreamCache:    map[Ref][]byte{{Object: 4}: []byte("data")},
		objectErrors:          map[Ref]error{{Object: 10}: errors.New("cached")},
		fontErrors:            map[Ref]error{{Object: 12}: errors.New("cached")},
		actionCache:           map[Ref]*Action{{Object: 5}: {}},
		nameTreeCache:         map[string][]NameTreeEntry{"JavaScript": {documentdata.NewNameTreeEntry("script", nil)}},
		nameTreeReady:         map[string]bool{"JavaScript": true},
		nameTreeErrors:        map[string]error{"Broken": errors.New("cached")},
		pagePropErrors:        map[Ref]error{{Object: 11}: errors.New("cached")},
		pageResourceErrors:    map[Ref]error{{Object: 13}: errors.New("cached")},
		xobjectResourceErrors: map[Ref]error{{Object: 14}: errors.New("cached")},
		parentTreeErrors:      map[int]error{7: errors.New("cached")},
		pageStructureErrors:   map[Ref]error{{Object: 15}: errors.New("cached")},
		formFieldsErr:         errors.New("cached"), formFieldsErrReady: true,
		outlineRootErr: errors.New("cached"), outlineRootErrReady: true,
		catalogErr: errors.New("cached"), catalogErrReady: true,
		infoErr: errors.New("cached"), infoErrReady: true,
		namesErr: errors.New("cached"), namesErrReady: true,
		pageStructureCache: map[Ref]PageStructure{{Object: 6}: {}},
		pageStructureReady: map[Ref]bool{{Object: 6}: true},
		taggedRanks: map[Ref]taggedPageRanks{
			{Object: 3}: {byMCID: map[int]int{4: 0}},
		},
		taggedRanksReady: true,
		taggedRanksErr:   errors.New("cached"),
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if d.namesErr != nil || d.namesErrReady {
		t.Fatalf("Names error cache remains after Close: %v/%v", d.namesErrReady, d.namesErr)
	}
	if d.catalogCache != nil || d.catalogReady || d.infoCache != nil || d.infoReady || d.namesCache != nil || d.namesReady {
		t.Fatalf("root success caches remain after Close")
	}
	if d.openActionCache != nil || d.openActionReady || d.openActionErr != nil || d.openActionErrReady {
		t.Fatalf("open action cache remains after Close")
	}
	if d.actionErrors != nil {
		t.Fatalf("action error cache remains after Close")
	}
	if d.destinationErrors != nil || d.destinationErrorBytes != 0 {
		t.Fatalf("destination error cache remains after Close")
	}
	if d.destinationsRootErr != nil || d.destinationsRootErrReady {
		t.Fatalf("DestinationsSeq root error remains after Close")
	}
	if d.structureRootErr != nil || d.structureRootErrReady {
		t.Fatalf("StructureTree root error remains after Close")
	}
	if d.taggedRanks != nil || d.taggedRanksReady || d.taggedRanksErr != nil {
		t.Fatalf("tagged-text rank cache remains after Close")
	}
	if d.contentRootErrors != nil || d.contentRootErrorBytes != 0 {
		t.Fatalf("content root error cache remains after Close")
	}
	if d.nameTreeErrorBytes != 0 {
		t.Fatalf("name-tree error cache bytes remain after Close: %d", d.nameTreeErrorBytes)
	}
	if d.data != nil || d.version != "" || d.space != "" || d.encrypted || d.encryptionKey != nil || d.encryptionRevision != 0 || d.encryptionMethod != "" || d.printable || d.modifiable || d.extractable || d.metadataExcluded || d.metadataRefs != nil || d.trailer != nil || d.catalogErr != nil || d.catalogErrReady || len(d.objects) != 0 || d.objectCacheBytes != 0 || len(d.fontCache) != 0 || d.fontErrors != nil || d.fontErrorBytes != 0 || d.pageCache != nil || d.pagesCache != nil || d.pagesErr != nil || d.pagesErrReady || d.metadataCache != nil || len(d.decodedStreamCache) != 0 || d.objectErrors != nil || d.objectErrorBytes != 0 || d.objectStreamErrors != nil || d.objectStreamErrorBytes != 0 || d.actionCache != nil || d.nameTreeCache != nil || d.nameTreeErrors != nil || d.pagePropErrors != nil || d.pagePropErrorBytes != 0 || d.pageResourceErrors != nil || d.pageResourceErrorBytes != 0 || d.xobjectResourceErrors != nil || d.xobjectResourceErrorBytes != 0 || d.annotationErrors != nil || d.annotationErrorBytes != 0 || d.structureRoleMapErr != nil || d.parentTreeErrors != nil || d.parentTreeErrorBytes != 0 || d.pageStructureErrors != nil || d.pageStructureErrorBytes != 0 || d.formFieldsErr != nil || d.formFieldsErrReady || d.outlineRootErr != nil || d.outlineRootErrReady || d.pageStructureCache != nil {
		t.Fatalf("document resources remain after Close: data=%v key=%v metadataRefs=%v trailer=%v xrefs=%v objects=%d fonts=%d pages=%v pageList=%v metadata=%v streams=%d actions=%v names=%v structures=%v", d.data, d.encryptionKey, d.metadataRefs, d.trailer, d.xrefs, len(d.objects), len(d.fontCache), d.pageCache, d.pagesCache, d.metadataCache, len(d.decodedStreamCache), d.actionCache, d.nameTreeCache, d.pageStructureCache)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseTransientCachesRetainsDocumentSource(t *testing.T) {
	d := &Document{
		data:            []byte("source"),
		objects:         map[Ref]Object{{Object: 1}: Number(1)},
		expandedObjStms: map[int]bool{2: true},
		fontCache:       map[Ref]*Font{{Object: 3}: NewSimpleFont("F")},
		pageCache:       map[Ref]Page{{Object: 4}: {}},
		pagesCache:      []Page{{ref: Ref{Object: 4}}},
		pagesReady:      true,
		pagesErr:        errors.New("cached"), pagesErrReady: true,
		decodedStreamCache: map[Ref][]byte{{Object: 5}: []byte("stream")},
		objectErrors:       map[Ref]error{{Object: 10}: errors.New("cached")},
		fontErrors:         map[Ref]error{{Object: 12}: errors.New("cached")}, fontErrorBytes: 6,
		actionScriptCache: map[Ref]string{{Object: 6}: "script"}, actionScriptCacheBytes: 6,
		actionCache:      map[Ref]*Action{{Object: 7}: {}},
		objectCacheBytes: 128,
		nameTreeCache:    map[string][]NameTreeEntry{"Names": {}}, nameTreeReady: map[string]bool{"Names": true},
		nameTreeErrors: map[string]error{"Broken": errors.New("cached")},
		pagePropErrors: map[Ref]error{{Object: 11}: errors.New("cached")}, pagePropErrorBytes: 6,
		pageResourceErrors: map[Ref]error{{Object: 13}: errors.New("cached")}, pageResourceErrorBytes: 6,
		xobjectResourceErrors: map[Ref]error{{Object: 14}: errors.New("cached")}, xobjectResourceErrorBytes: 6,
		parentTreeErrors: map[int]error{7: errors.New("cached")}, parentTreeErrorBytes: 6,
		pageStructureErrors: map[Ref]error{{Object: 15}: errors.New("cached")}, pageStructureErrorBytes: 6,
		formFieldsErr: errors.New("cached"), formFieldsErrReady: true,
		outlineRootErr: errors.New("cached"), outlineRootErrReady: true,
		catalogErr: errors.New("cached"), catalogErrReady: true,
		infoErr: errors.New("cached"), infoErrReady: true,
		namesErr: errors.New("cached"), namesErrReady: true,
		destinationCache: map[string]Object{"dest": String("cached")}, destinationsCache: map[string]Object{"dest": String("cached")}, destinationsReady: true,
		metadataXMLReady: true, metadataXMLCacheable: true, metadataXMLCache: []byte("cached metadata"),
		metadataReady: true, metadataCacheable: true, metadataCache: Metadata{"Title": "cached info"},
		taggedRanks: map[Ref]taggedPageRanks{
			{Object: 4}: {byMCID: map[int]int{5: 0}},
		},
		taggedRanksReady: true,
		taggedRanksErr:   errors.New("cached"),
	}
	d.ReleaseTransientCaches()
	if d.namesErr != nil || d.namesErrReady {
		t.Fatalf("Names error cache remains after ReleaseTransientCaches: %v/%v", d.namesErrReady, d.namesErr)
	}
	if d.catalogCache != nil || d.catalogReady || d.infoCache != nil || d.infoReady || d.namesCache != nil || d.namesReady {
		t.Fatalf("root success caches remain after ReleaseTransientCaches")
	}
	if d.openActionCache != nil || d.openActionReady || d.openActionErr != nil || d.openActionErrReady {
		t.Fatalf("open action cache remains after ReleaseTransientCaches")
	}
	if d.actionErrors != nil {
		t.Fatalf("action error cache remains after ReleaseTransientCaches")
	}
	if d.destinationErrors != nil || d.destinationErrorBytes != 0 {
		t.Fatalf("destination error cache remains after ReleaseTransientCaches")
	}
	if d.destinationsRootErr != nil || d.destinationsRootErrReady {
		t.Fatalf("DestinationsSeq root error remains after ReleaseTransientCaches")
	}
	if d.structureRootErr != nil || d.structureRootErrReady {
		t.Fatalf("StructureTree root error remains after ReleaseTransientCaches")
	}
	if d.taggedRanks != nil || d.taggedRanksReady || d.taggedRanksErr != nil {
		t.Fatalf("tagged-text rank cache remains after ReleaseTransientCaches")
	}
	if d.contentRootErrors != nil || d.contentRootErrorBytes != 0 {
		t.Fatalf("content root error cache remains after ReleaseTransientCaches")
	}
	if d.nameTreeErrorBytes != 0 {
		t.Fatalf("name-tree error cache bytes remain after ReleaseTransientCaches: %d", d.nameTreeErrorBytes)
	}
	if string(d.data) != "source" || len(d.objects) != 0 || d.objectCacheBytes != 0 || len(d.expandedObjStms) != 0 || len(d.fontCache) != 0 || d.fontErrors != nil || d.fontErrorBytes != 0 || len(d.pageCache) != 0 || len(d.pagesCache) != 0 || d.pagesReady || d.pagesErr != nil || d.pagesErrReady || len(d.decodedStreamCache) != 0 || len(d.actionScriptCache) != 0 || d.actionScriptCacheBytes != 0 || d.objectErrors != nil || d.objectErrorBytes != 0 || d.objectStreamErrors != nil || d.objectStreamErrorBytes != 0 || len(d.actionCache) != 0 || len(d.nameTreeCache) != 0 || d.nameTreeErrors != nil || d.pagePropErrors != nil || d.pagePropErrorBytes != 0 || d.pageResourceErrors != nil || d.pageResourceErrorBytes != 0 || d.xobjectResourceErrors != nil || d.xobjectResourceErrorBytes != 0 || d.annotationErrors != nil || d.annotationErrorBytes != 0 || d.parentTreeErrors != nil || d.parentTreeErrorBytes != 0 || d.pageStructureErrors != nil || d.pageStructureErrorBytes != 0 || d.formFieldsErr != nil || d.formFieldsErrReady || d.outlineRootErr != nil || d.outlineRootErrReady || d.catalogErr != nil || d.catalogErrReady || len(d.destinationCache) != 0 || len(d.destinationsCache) != 0 || d.metadataXMLReady || d.metadataXMLCacheable || d.metadataXMLCache != nil || d.metadataReady || d.metadataCacheable || d.metadataCache != nil {
		t.Fatalf("transient caches remain: data=%q objects=%d object-streams=%d fonts=%d pages=%d page-list=%d ready=%v streams=%d scripts=%d script-bytes=%d actions=%d names=%d destinations=%d/%d metadata-ready=%v metadata-cacheable=%v metadata-bytes=%d info-ready=%v info-cacheable=%v info=%v", d.data, len(d.objects), len(d.expandedObjStms), len(d.fontCache), len(d.pageCache), len(d.pagesCache), d.pagesReady, len(d.decodedStreamCache), len(d.actionScriptCache), d.actionScriptCacheBytes, len(d.actionCache), len(d.nameTreeCache), len(d.destinationCache), len(d.destinationsCache), d.metadataXMLReady, d.metadataXMLCacheable, len(d.metadataXMLCache), d.metadataReady, d.metadataCacheable, d.metadataCache)
	}
}

func TestReleaseTransientCachesSynchronizesLazyActions(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage"), Name("Next"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("S"): Name("Named"), Name("N"): Name("LastPage")},
	}}
	a := d.ResolveAction(Ref{Object: 1})
	if a == nil {
		t.Fatal("root action was not resolved")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 32; j++ {
				_, _ = a.NextCopy()
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 32; j++ {
				d.ReleaseTransientCaches()
			}
		}()
	}
	wg.Wait()
}

func TestReleaseTransientCachesPreservesScannedObjects(t *testing.T) {
	pdf := []byte("%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n")
	d, err := OpenBytes(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.xrefs) != 3 {
		t.Fatalf("recovery xref index size = %d, want 3", len(d.xrefs))
	}
	d.ReleaseTransientCaches()
	if len(d.objects) != 0 {
		t.Fatalf("recovered object cache retained %d entries", len(d.objects))
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := page.MediaBox(d); got != [4]float64{0, 0, 612, 792} {
		t.Fatalf("scanned page box after cache release = %v", got)
	}
}

func TestScanObjectsIgnoresObjectLikeBytesInsideStreams(t *testing.T) {
	payload := "99 0 obj\n<< /Type /Page >>\nendobj\n"
	pdf := "%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n" +
		fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n", len(payload), payload)
	d, err := OpenBytes([]byte(pdf))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.xrefs[Ref{Object: 99}]; ok {
		t.Fatal("object-like stream bytes were indexed as an indirect object")
	}
	if _, ok := d.Object(Ref{Object: 99}); ok {
		t.Fatal("object-like stream bytes were indexed as an indirect object")
	}
}

func TestOpenBytesDefersXRefObjectsUntilResolve(t *testing.T) {
	objects := []string{
		"1 0 obj\n<< /Type /Catalog >>\nendobj\n",
		"2 0 obj\n<< /Value 42 >>\nendobj\n",
	}
	pdf := "%PDF-1.7\n"
	offsets := []int{0}
	for _, object := range objects {
		offsets = append(offsets, len(pdf))
		pdf += object
	}
	xref := len(pdf)
	pdf += fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		pdf += fmt.Sprintf("%010d 00000 n \n", offset)
	}
	pdf += fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)

	d, err := OpenBytes([]byte(pdf))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.objects) != 0 {
		t.Fatalf("opened with %d parsed objects, want lazy zero", len(d.objects))
	}
	value, err := d.ResolveRef(Ref{Object: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := value.(Dict); !ok || len(d.objects) != 1 {
		t.Fatalf("resolved value = %#v, cached objects = %d", value, len(d.objects))
	}
}

func TestOpenOptionsSelectCoordinateSpace(t *testing.T) {
	d, err := OpenBytes([]byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n"), WithCoordinateSpace(CoordinateSpaceScreen))
	if err != nil {
		t.Fatal(err)
	}
	if got := d.CoordinateSpace(); got != CoordinateSpaceScreen {
		t.Fatalf("coordinate space = %q, want %q", got, CoordinateSpaceScreen)
	}
}

func TestFunctionalOpenOptionsConfigureCacheBudgets(t *testing.T) {
	cache := DefaultCacheOptions()
	cache.PageBytes = 0
	cache.PagesBytes = 0
	d, err := OpenBytes([]byte("%PDF-1.4\n1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj\n3 0 obj << /Type /Page /Parent 2 0 R >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF\n"), WithCoordinateSpace(CoordinateSpacePage), WithCacheOptions(cache))
	if err != nil {
		t.Fatal(err)
	}
	if d.CoordinateSpace() != CoordinateSpacePage || !d.cacheOptionsConfigured || d.cacheLimits().PageBytes != 0 {
		t.Fatalf("open options not applied: space=%q configured=%v page-cache=%d", d.CoordinateSpace(), d.cacheOptionsConfigured, d.cacheLimits().PageBytes)
	}
	pages, err := d.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 || pages[0].space != CoordinateSpacePage || pages[0].index != 0 {
		t.Fatalf("opened page model = %#v, want page space and zero-based index", pages)
	}
	if len(d.pageCache) != 0 || d.pagesReady {
		t.Fatalf("disabled page caches retained entries: pages=%d ready=%v", len(d.pageCache), d.pagesReady)
	}
}

func TestObjectCacheBudgetEvictsParsedObjects(t *testing.T) {
	pdf := []byte("%PDF-1.4\n1 0 obj << /Type /Catalog /Payload (a deliberately large parsed value) >> endobj\n")
	d := &Document{
		data: pdf, xrefs: map[Ref]xrefEntry{{Object: 1}: {offset: 9}},
		objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}, decrypted: map[Ref]bool{},
		cacheOptions: cacheconfig.Options{ObjectBytes: 8}, cacheOptionsConfigured: true,
	}
	value, err := d.ResolveRef(Ref{Object: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := value.(Dict); !ok {
		t.Fatalf("resolved value = %#v, want dict", value)
	}
	if len(d.objects) != 0 {
		t.Fatalf("parsed object cache retained %d entries after budget eviction", len(d.objects))
	}

	if _, err := d.ResolveRef(Ref{Object: 1}); err != nil {
		t.Fatalf("re-resolving after eviction: %v", err)
	}
}

func TestOpenDefaultsToScreenCoordinateSpace(t *testing.T) {
	d, err := OpenBytes([]byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := d.CoordinateSpace(); got != CoordinateSpaceScreen {
		t.Fatalf("default coordinate space = %q, want %q", got, CoordinateSpaceScreen)
	}

	d, err = OpenBytes([]byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := d.CoordinateSpace(); got != CoordinateSpaceScreen {
		t.Fatalf("empty options coordinate space = %q, want %q", got, CoordinateSpaceScreen)
	}
}

func TestDocumentMetadataProperties(t *testing.T) {
	d, err := OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Version /1.7 /MarkInfo << /Marked true >> /Names << /Dests <<>> >> >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.PDFVersion() != "1.7" {
		t.Fatalf("PDFVersion = %q", d.PDFVersion())
	}
	if !d.IsTagged() || d.Catalog() == nil || d.Names() == nil {
		t.Fatalf("catalog metadata not resolved: tagged=%v catalog=%v names=%v", d.IsTagged(), d.Catalog(), d.Names())
	}
}

func TestCatalogWithErrorFollowsIndirectRootChain(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Ref{Object: 2},
			{Object: 2}: Dict{Name("Type"): Name("Catalog")},
		},
	}
	catalog, err := d.CatalogWithError()
	if err != nil || catalog[Name("Type")] != Name("Catalog") {
		t.Fatalf("indirect catalog = %#v, err = %v", catalog, err)
	}
}

func TestDocumentSubrootsFollowIndirectChains(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}, Name("Info"): Ref{Object: 8}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{
				Name("Names"):      Ref{Object: 2},
				Name("PageLabels"): Ref{Object: 3},
				Name("AcroForm"):   Ref{Object: 4},
				Name("Pages"):      Ref{Object: 11},
			},
			{Object: 2}:  Ref{Object: 5},
			{Object: 5}:  Dict{Name("Dests"): Ref{Object: 6}},
			{Object: 3}:  Ref{Object: 7},
			{Object: 7}:  Dict{Name("Nums"): Array{Number(0), Dict{Name("P"): String("p")}}},
			{Object: 4}:  Ref{Object: 9},
			{Object: 9}:  Dict{Name("Fields"): Array{}},
			{Object: 6}:  Dict{Name("Names"): Array{}},
			{Object: 8}:  Ref{Object: 10},
			{Object: 10}: Dict{Name("Title"): String("title")},
			{Object: 11}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 12}}, Name("Count"): Number(1)},
			{Object: 12}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 11}, Name("MediaBox"): Array{Number(0), Number(0), Number(612), Number(792)}},
		},
	}
	if names, err := d.NamesWithError(); err != nil || names[Name("Dests")] == nil {
		t.Fatalf("indirect Names = %#v, err = %v", names, err)
	}
	if info, err := d.InfoWithError(); err != nil || info[Name("Title")] == nil {
		t.Fatalf("indirect Info = %#v, err = %v", info, err)
	}
	labels, err := d.PageLabels()
	if err != nil || len(labels) != 1 || labels[0] != "p" {
		t.Fatalf("indirect PageLabels = %#v, err = %v", labels, err)
	}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 0 {
		t.Fatalf("indirect AcroForm fields = %#v, err = %v", fields, err)
	}
}

func TestNamesWithErrorReportsMalformedCatalogValue(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("Names"): Number(1)}}}
	_, firstErr := d.NamesWithError()
	_, secondErr := d.NamesWithError()
	if firstErr == nil || secondErr == nil || firstErr != secondErr {
		t.Fatalf("malformed Names errors = %v/%v, want one cached error", firstErr, secondErr)
	}
	if d.namesErr != firstErr || !d.namesErrReady {
		t.Fatalf("cached Names error = %v/%v", d.namesErrReady, d.namesErr)
	}
	if names, err := d.NamesWithError(); err == nil || names != nil {
		t.Fatalf("malformed Names = %#v, err = %v", names, err)
	}
	if names := d.Names(); names != nil {
		t.Fatalf("compatibility Names = %#v, want nil", names)
	}
}

func TestCatalogAndInfoWithErrorReportMalformedRoots(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Number(1), Name("Info"): String("invalid")}}
	if catalog, err := d.CatalogWithError(); err == nil || catalog != nil {
		t.Fatalf("malformed catalog = %#v, err = %v", catalog, err)
	}
	if info, err := d.InfoWithError(); err == nil || info != nil {
		t.Fatalf("malformed info = %#v, err = %v", info, err)
	}
	if d.Catalog() != nil || d.Info() != nil {
		t.Fatal("compatibility accessors exposed malformed roots")
	}
}

func TestCatalogAndInfoCacheTerminalErrors(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Number(1), Name("Info"): String("invalid")}}
	_, firstCatalogErr := d.CatalogWithError()
	_, secondCatalogErr := d.CatalogWithError()
	_, firstInfoErr := d.InfoWithError()
	_, secondInfoErr := d.InfoWithError()
	if firstCatalogErr == nil || secondCatalogErr == nil || firstInfoErr == nil || secondInfoErr == nil {
		t.Fatalf("root errors = %v, %v, %v, %v; want all non-nil", firstCatalogErr, secondCatalogErr, firstInfoErr, secondInfoErr)
	}
	if firstCatalogErr != secondCatalogErr || firstInfoErr != secondInfoErr {
		t.Fatalf("root errors were not cached: catalog=%p/%p info=%p/%p", firstCatalogErr, secondCatalogErr, firstInfoErr, secondInfoErr)
	}
	if d.catalogErr != firstCatalogErr || !d.catalogErrReady || d.infoErr != firstInfoErr || !d.infoErrReady {
		t.Fatalf("cached root state = catalog=%v/%v info=%v/%v", d.catalogErrReady, d.catalogErr, d.infoErrReady, d.infoErr)
	}
}

func TestNamesWithErrorReportsMalformedCatalogRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Number(1)}}
	if names, err := d.NamesWithError(); err == nil || names != nil {
		t.Fatalf("malformed catalog names = %#v, err=%v", names, err)
	}
}

func TestMetadataReturnsIndependentCachedCopies(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Info"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Title"): String("cached")},
		},
	}

	first := d.Metadata()
	first["Title"] = "mutated"
	second := d.Metadata()
	if second["Title"] != "cached" {
		t.Fatalf("metadata cache was exposed: %#v", second)
	}
}

func TestDocumentDictionariesAreIndependentCopies(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}, Name("Info"): Ref{Object: 2}, Name("Custom"): Dict{Name("Value"): String("original")}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Type"): Name("Catalog"), Name("Names"): Dict{Name("Value"): String("original")}},
			{Object: 2}: Dict{Name("Title"): String("original")},
		},
	}
	d.Catalog()[Name("Type")] = Name("Changed")
	d.Info()[Name("Title")] = String("Changed")
	d.Names()[Name("Value")] = String("Changed")
	d.Trailer()[Name("Custom")].(Dict)[Name("Value")] = String("Changed")
	if d.Catalog()[Name("Type")] != Name("Catalog") || string(d.Info()[Name("Title")].(String)) != "original" || string(d.Names()[Name("Value")].(String)) != "original" || string(d.Trailer()[Name("Custom")].(Dict)[Name("Value")].(String)) != "original" {
		t.Fatal("document dictionary API exposed internal state")
	}
}

func TestDocumentRootDictionariesCacheSuccessfulResolution(t *testing.T) {
	d := &Document{
		trailer: Dict{
			Name("Root"): Ref{Object: 1},
			Name("Info"): Ref{Object: 2},
		},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Type"): Name("Catalog"), Name("Names"): Ref{Object: 3}},
			{Object: 2}: Dict{Name("Title"): String("cached")},
			{Object: 3}: Dict{Name("Value"): String("names")},
		},
	}
	firstCatalog := d.Catalog()
	firstInfo := d.Info()
	firstNames := d.Names()
	if !d.catalogReady || !d.infoReady || !d.namesReady {
		t.Fatalf("root cache readiness = catalog=%v info=%v names=%v", d.catalogReady, d.infoReady, d.namesReady)
	}
	firstCatalog[Name("Type")] = Name("changed")
	firstInfo[Name("Title")] = String("changed")
	firstNames[Name("Value")] = String("changed")
	info, names := d.Info(), d.Names()
	title, titleOK := info[Name("Title")].(String)
	value, valueOK := names[Name("Value")].(String)
	if d.Catalog()[Name("Type")] != Name("Catalog") || !titleOK || string(title) != "cached" || !valueOK || string(value) != "names" {
		t.Fatal("successful root caches exposed mutable internal dictionaries")
	}
}

func TestDocumentPDFVersionFallsBackToHeader(t *testing.T) {
	d, err := OpenBytes([]byte("%PDF-1.6\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.PDFVersion() != "1.6" {
		t.Fatalf("PDFVersion = %q, want header version", d.PDFVersion())
	}
}

func TestMetadataXMLDecodesCatalogMetadataStream(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Metadata"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: newStream(Dict{Name("Type"): Name("Metadata"), Name("Subtype"): Name("XML")}, []byte("<x:xmpmeta/>")),
		},
	}
	got, err := d.MetadataXML()
	if err != nil || string(got) != "<x:xmpmeta/>" {
		t.Fatalf("metadata XML = %q, err=%v", got, err)
	}
}

func TestDecryptObjectUsesObjectSpecificRC4Key(t *testing.T) {
	d := &Document{encrypted: true, encryptionKey: []byte{1, 2, 3, 4, 5}}
	ref := Ref{Object: 7, Generation: 2}
	key := d.objectKey(ref)
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	encrypted := make([]byte, 5)
	cipher.XORKeyStream(encrypted, []byte("hello"))
	got, ok := d.decryptObject(ref, String(encrypted)).(String)
	if !ok || string(got) != "hello" {
		t.Fatalf("decrypted object = %#v", got)
	}
}

func TestConfigureSecurityAcceptsStandardRevisionTwoPassword(t *testing.T) {
	password := "secret"
	owner := []byte("owner-key")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKey([]byte(password), owner, permissions, id)
	user, err := rc4Bytes(key, passwordPadding)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(1), Name("R"): Number(2),
			Name("O"): String(owner), Name("U"): String(user), Name("P"): Number(permissions),
		}},
	}
	if err := d.configureSecurity(password); err != nil {
		t.Fatal(err)
	}
	if !d.encrypted || len(d.encryptionKey) != 5 {
		t.Fatalf("security state = encrypted %v key length %d", d.encrypted, len(d.encryptionKey))
	}
	if !d.IsPrintable() || !d.IsModifiable() || !d.IsExtractable() {
		t.Fatalf("R2 permissions = printable:%v modifiable:%v extractable:%v", d.IsPrintable(), d.IsModifiable(), d.IsExtractable())
	}
	if err := d.configureSecurity("wrong"); err != ErrInvalidPassword {
		t.Fatalf("wrong password error = %v", err)
	}
}

func TestConfigureSecurityAcceptsStandardRevisionThreePassword(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKeyRevision([]byte(password), owner, permissions, id, 16, 3)
	user, err := userEntryRevision(key, id, 3)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(2), Name("R"): Number(3), Name("Length"): Number(128),
			Name("O"): String(owner), Name("U"): String(user), Name("P"): Number(permissions),
		}},
	}
	if err := d.configureSecurity(password); err != nil {
		t.Fatal(err)
	}
	if len(d.encryptionKey) != 16 {
		t.Fatalf("R3 key length = %d, want 16", len(d.encryptionKey))
	}
}

func TestConfigureSecurityAcceptsIntermediateRevisionThreeKeyLength(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKeyRevision([]byte(password), owner, permissions, id, 7, 3)
	user, err := userEntryRevision(key, id, 3)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(2), Name("R"): Number(3), Name("Length"): Number(56),
			Name("O"): String(owner), Name("U"): String(user), Name("P"): Number(permissions),
		}},
	}
	if err := d.configureSecurity(password); err != nil {
		t.Fatal(err)
	}
	if len(d.encryptionKey) != 7 {
		t.Fatalf("R3 intermediate key length = %d, want 7", len(d.encryptionKey))
	}
}

func TestConfigureSecurityDefaultsRevisionThreeToFortyBitKey(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKeyRevision([]byte(password), owner, permissions, id, 5, 3)
	user, err := userEntryRevision(key, id, 3)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(2), Name("R"): Number(3),
			Name("O"): String(owner), Name("U"): String(user), Name("P"): Number(permissions),
		}},
	}
	if err := d.configureSecurity(password); err != nil {
		t.Fatal(err)
	}
	if len(d.encryptionKey) != 5 {
		t.Fatalf("default R3 key length = %d, want 5", len(d.encryptionKey))
	}
}

func TestConfigureSecurityRejectsMalformedEncryptMetadata(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKeyRevision([]byte(password), owner, permissions, id, 16, 3)
	user, err := userEntryRevision(key, id, 3)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(2), Name("R"): Number(3), Name("Length"): Number(128),
			Name("O"): String(owner), Name("U"): String(user), Name("P"): Number(permissions), Name("EncryptMetadata"): Number(1),
		}},
	}
	if err := d.configureSecurity(password); err != ErrUnsupportedEncryption {
		t.Fatalf("malformed EncryptMetadata error = %v, want %v", err, ErrUnsupportedEncryption)
	}
}

func TestConfigureSecurityRejectsCyclicEncryptMetadataReference(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKeyRevision([]byte(password), owner, permissions, id, 16, 3)
	user, err := userEntryRevision(key, id, 3)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{
			{Object: 5, Generation: 0}: Dict{
				Name("Filter"): Name("Standard"), Name("V"): Number(2), Name("R"): Number(3),
				Name("Length"): Number(128), Name("O"): String(owner), Name("U"): String(user),
				Name("P"): Number(permissions), Name("EncryptMetadata"): Ref{Object: 8, Generation: 0},
			},
			{Object: 8, Generation: 0}: Ref{Object: 9, Generation: 0},
			{Object: 9, Generation: 0}: Ref{Object: 8, Generation: 0},
		},
	}
	if err := d.configureSecurity(password); err != ErrUnsupportedEncryption {
		t.Fatalf("cyclic EncryptMetadata error = %v, want %v", err, ErrUnsupportedEncryption)
	}
	if d.encrypted || d.encryptionKey != nil || d.metadataExcluded {
		t.Fatalf("cyclic EncryptMetadata changed security state: encrypted=%v key=%v excluded=%v", d.encrypted, d.encryptionKey, d.metadataExcluded)
	}
}

func TestDecryptObjectUsesAESV2ObjectKey(t *testing.T) {
	d := &Document{encrypted: true, encryptionRevision: 4, encryptionKey: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}}
	ref := Ref{Object: 7, Generation: 2}
	block, err := aes.NewCipher(d.objectKey(ref))
	if err != nil {
		t.Fatal(err)
	}
	iv := []byte("0123456789abcdef")
	plain := []byte("hello AES")
	plain = append(plain, bytes.Repeat([]byte{byte(aes.BlockSize - len(plain)%aes.BlockSize)}, aes.BlockSize-len(plain)%aes.BlockSize)...)
	ciphertext := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, plain)
	encrypted := append(append([]byte(nil), iv...), ciphertext...)
	got, ok := d.decryptObject(ref, String(encrypted)).(String)
	if !ok || string(got) != "hello AES" {
		t.Fatalf("decrypted AES object = %#v", got)
	}
}

func TestResolveRefReportsMalformedEncryptedAESObject(t *testing.T) {
	d := &Document{
		encrypted:              true,
		encryptionRevision:     4,
		encryptionMethod:       "AESV2",
		stringEncryptionMethod: "AESV2",
		encryptionKey:          []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		objects:                map[Ref]Object{{Object: 7}: String("short")},
		decrypted:              map[Ref]bool{},
	}
	if _, err := d.ResolveRef(Ref{Object: 7}); err == nil {
		t.Fatal("ResolveRef accepted malformed AES ciphertext")
	}
}

func TestConfigureSecurityAcceptsStandardRevisionFourAESV2(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKeyRevisionWithMetadata([]byte(password), owner, permissions, id, 16, 4, true)
	user, err := userEntryRevision(key, id, 4)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(4), Name("R"): Number(4), Name("Length"): Number(128),
			Name("O"): String(owner), Name("U"): String(user), Name("P"): Number(permissions),
			Name("CF"): Dict{Name("StdCF"): Dict{Name("CFM"): Name("AESV2")}}, Name("StmF"): Name("StdCF"), Name("StrF"): Name("StdCF"),
		}},
	}
	if err := d.configureSecurity(password); err != nil {
		t.Fatal(err)
	}
	if d.encryptionRevision != 4 || len(d.encryptionKey) != 16 {
		t.Fatalf("R4 security state = revision %d key length %d", d.encryptionRevision, len(d.encryptionKey))
	}
}

func TestConfigureSecurityAcceptsStandardRevisionFourRC4(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKeyRevisionWithMetadata([]byte(password), owner, permissions, id, 16, 4, true)
	user, err := userEntryRevision(key, id, 4)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(4), Name("R"): Number(4), Name("Length"): Number(128),
			Name("O"): String(owner), Name("U"): String(user), Name("P"): Number(permissions),
			Name("CF"): Dict{Name("StdCF"): Dict{Name("CFM"): Name("V2")}}, Name("StmF"): Name("StdCF"), Name("StrF"): Name("StdCF"),
		}},
	}
	if err := d.configureSecurity(password); err != nil {
		t.Fatal(err)
	}
	if d.encryptionMethod != "V2" {
		t.Fatalf("R4 RC4 method = %q", d.encryptionMethod)
	}

	ref := Ref{Object: 7, Generation: 2}
	plain := String("hello RC4")
	ciphertext, err := rc4Bytes(d.objectKey(ref), []byte(plain))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := d.decryptObject(ref, String(ciphertext)).(String)
	if !ok || string(got) != string(plain) {
		t.Fatalf("decrypted RC4 object = %#v", got)
	}
}

func TestConfigureSecuritySupportsIdentityStreamCryptFilter(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKeyRevisionWithMetadata([]byte(password), owner, permissions, id, 16, 4, true)
	user, err := userEntryRevision(key, id, 4)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(4), Name("R"): Number(4), Name("Length"): Number(128),
			Name("O"): String(owner), Name("U"): String(user), Name("P"): Number(permissions),
			Name("CF"): Dict{Name("StdCF"): Dict{Name("CFM"): Name("V2")}}, Name("StmF"): Name("Identity"), Name("StrF"): Name("StdCF"),
		}},
	}
	if err := d.configureSecurity(password); err != nil {
		t.Fatal(err)
	}
	if d.streamEncryptionMethod != "Identity" || d.stringEncryptionMethod != "V2" {
		t.Fatalf("crypt filters = stream:%q string:%q", d.streamEncryptionMethod, d.stringEncryptionMethod)
	}
	ref := Ref{Object: 7, Generation: 2}
	ciphertext, err := rc4Bytes(d.objectKeyForMethod(ref, "V2"), []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := d.decryptObject(ref, String(ciphertext)).(String)
	if !ok || string(got) != "hello" {
		t.Fatalf("decrypted string = %#v", got)
	}
	stream, ok := d.decryptObject(ref, newStream(Dict{}, []byte("plain"))).(Stream)
	if !ok || string(stream.DataBorrowed()) != "plain" {
		t.Fatalf("identity stream = %#v", stream)
	}
}

func TestConfigureSecurityAcceptsR4NoneCryptFilter(t *testing.T) {
	password := "secret"
	owner := []byte("owner-entry-with-32-byte-padding----")
	id := []byte("file-id")
	permissions := -4
	key := encryptionKeyRevisionWithMetadata([]byte(password), owner, permissions, id, 16, 4, true)
	user, err := userEntryRevision(key, id, 4)
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String(id)}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(4), Name("R"): Number(4), Name("Length"): Number(128),
			Name("O"): String(owner), Name("U"): String(user), Name("P"): Number(permissions),
			Name("CF"): Dict{Name("StdCF"): Dict{Name("CFM"): Name("None")}}, Name("StmF"): Name("StdCF"), Name("StrF"): Name("StdCF"),
		}},
	}
	if err := d.configureSecurity(password); err != nil {
		t.Fatal(err)
	}
	if d.streamEncryptionMethod != "Identity" || d.stringEncryptionMethod != "Identity" {
		t.Fatalf("R4 None crypt filter methods = stream:%q string:%q", d.streamEncryptionMethod, d.stringEncryptionMethod)
	}
	ref := Ref{Object: 7, Generation: 2}
	plain := String("not encrypted")
	got, ok := d.decryptObject(ref, plain).(String)
	if !ok || string(got) != string(plain) {
		t.Fatalf("R4 None string = %q, want %q", got, plain)
	}
}

func TestConfigureSecurityDefaultsR4CryptFiltersToIdentity(t *testing.T) {
	stream, stringFilter, ok := standardR4CryptFilters(&Document{}, Dict{
		Name("CF"): Dict{Name("StdCF"): Dict{Name("CFM"): Name("V2")}},
	})
	if !ok || stream != "Identity" || stringFilter != "Identity" {
		t.Fatalf("R4 omitted crypt filters = stream:%q string:%q ok:%v", stream, stringFilter, ok)
	}
}

func TestSecurityHelpersFollowMultiLevelIndirectValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{Name("StdCF"): Ref{Object: 3}},
		{Object: 3}: Dict{Name("CFM"): Ref{Object: 4}},
		{Object: 4}: Name("V2"),
		{Object: 6}: Ref{Object: 7}, {Object: 7}: Number(-4),
		{Object: 8}: Ref{Object: 9}, {Object: 9}: Name("StdCF"),
	}}
	filters, stringFilter, ok := standardR4CryptFilters(d, Dict{
		Name("CF"): Ref{Object: 1}, Name("StmF"): Ref{Object: 8}, Name("StrF"): Ref{Object: 8},
	})
	if !ok || filters != "V2" || stringFilter != "V2" {
		t.Fatalf("indirect crypt filters = %q/%q/%v", filters, stringFilter, ok)
	}
	permissions, ok := securityPermissions(d, Dict{Name("P"): Ref{Object: 6}})
	if !ok || permissions != -4 {
		t.Fatalf("indirect permissions = %d/%v", permissions, ok)
	}
}

func TestStandardAESV3DefaultsCryptFiltersToStdCF(t *testing.T) {
	if !standardAESV3(&Document{}, Dict{
		Name("CF"): Dict{Name("StdCF"): Dict{Name("CFM"): Name("AESV3")}},
	}) {
		t.Fatal("R5/R6 omitted crypt filters should default to StdCF")
	}
}

func TestEncryptionRejectsMalformedExplicitFilterNames(t *testing.T) {
	d := &Document{}
	if _, _, ok := standardR4CryptFilters(d, Dict{Name("StmF"): Number(1)}); ok {
		t.Fatal("R4 accepted a non-name StmF value")
	}
	if standardAESV3(d, Dict{
		Name("CF"):   Dict{Name("StdCF"): Dict{Name("CFM"): Name("AESV3")}},
		Name("StrF"): Number(1),
	}) {
		t.Fatal("R5 accepted a non-name StrF value")
	}
}

func TestSecurityPermissionsRequireSigned32BitInteger(t *testing.T) {
	d := &Document{}
	for _, value := range []Object{Number(2147483648), Number(-2147483649), Number(1.5), Number(math.Inf(1)), Number(math.NaN())} {
		if _, ok := securityPermissions(d, Dict{Name("P"): value}); ok {
			t.Fatalf("permissions value %#v was accepted", value)
		}
	}
	if value, ok := securityPermissions(d, Dict{Name("P"): Number(-4)}); !ok || value != -4 {
		t.Fatalf("valid permissions = %d, ok=%v", value, ok)
	}
}

func TestConfigureSecurityAcceptsStandardRevisionFiveAES256(t *testing.T) {
	password := []byte("secret")
	fileKey := []byte("01234567890123456789012345678901")
	userValidationSalt := []byte("uvsalt01")
	userKeySalt := []byte("uksalt01")
	ownerValidationSalt := []byte("ovsalt01")
	ownerKeySalt := []byte("oksalt01")
	uHash := sha256.Sum256(append(append([]byte(nil), password...), userValidationSalt...))
	u := append(append(append([]byte(nil), uHash[:]...), userValidationSalt...), userKeySalt...)
	oInput := append(append(append([]byte(nil), password...), ownerValidationSalt...), u...)
	oHash := sha256.Sum256(oInput)
	o := append(append(append([]byte(nil), oHash[:]...), ownerValidationSalt...), ownerKeySalt...)
	userKey := sha256.Sum256(append(append([]byte(nil), password...), userKeySalt...))
	ownerKey := sha256.Sum256(append(append(append([]byte(nil), password...), ownerKeySalt...), u...))
	ue := encryptAESNoPadding(userKey[:], fileKey)
	oe := encryptAESNoPadding(ownerKey[:], fileKey)
	permsPlain := make([]byte, 16)
	permissions := int32(-4)
	binary.LittleEndian.PutUint32(permsPlain, uint32(permissions))
	for i := 4; i < 8; i++ {
		permsPlain[i] = 0xff
	}
	permsPlain[8] = 'T'
	copy(permsPlain[9:], []byte("adb"))
	permsPlain[12] = 'T'
	perms := encryptAESNoPadding(fileKey, permsPlain)
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String("file-id")}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(5), Name("R"): Number(5),
			Name("O"): String(o), Name("U"): String(u), Name("OE"): String(oe), Name("UE"): String(ue), Name("Perms"): String(perms), Name("P"): Number(permissions),
			Name("CF"): Dict{Name("StdCF"): Dict{Name("CFM"): Name("AESV3")}},
		}},
	}
	if err := d.configureSecurity(string(password)); err != nil {
		t.Fatal(err)
	}
	if d.encryptionRevision != 5 || string(d.encryptionKey) != string(fileKey) {
		t.Fatalf("R5 security state = revision %d key %q", d.encryptionRevision, d.encryptionKey)
	}
	if d.streamEncryptionMethod != "AESV3" || d.stringEncryptionMethod != "AESV3" {
		t.Fatalf("R5 omitted crypt-filter defaults = stream:%q string:%q", d.streamEncryptionMethod, d.stringEncryptionMethod)
	}
	enc := d.objects[Ref{Object: 5, Generation: 0}].(Dict)
	permsPlain[8] = 'F'
	enc[Name("Perms")] = String(encryptAESNoPadding(fileKey, permsPlain))
	d.objects[Ref{Object: 8, Generation: 0}] = Bool(false)
	enc[Name("EncryptMetadata")] = Ref{Object: 8, Generation: 0}
	if err := d.configureSecurity(string(password)); err != nil {
		t.Fatalf("indirect R5 EncryptMetadata was rejected: %v", err)
	}
	if !d.metadataExcluded {
		t.Fatal("indirect R5 EncryptMetadata=false did not exclude metadata")
	}
	enc = d.objects[Ref{Object: 5, Generation: 0}].(Dict)
	enc[Name("EncryptMetadata")] = Number(1)
	if err := d.configureSecurity(string(password)); err != ErrUnsupportedEncryption {
		t.Fatalf("R5 malformed EncryptMetadata error = %v, want %v", err, ErrUnsupportedEncryption)
	}
}

func TestConfigureSecurityAcceptsStandardRevisionSixAES256(t *testing.T) {
	password := "sec\u00adret-r6"
	preparedPassword, err := revisionSixPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	fileKey := []byte("01234567890123456789012345678901")
	userValidationSalt := []byte("uvsalt06")
	userKeySalt := []byte("uksalt06")
	ownerValidationSalt := []byte("ovsalt06")
	ownerKeySalt := []byte("oksalt06")
	uHash := revisionSixHash(preparedPassword, userValidationSalt, nil)
	u := append(append(append([]byte(nil), uHash...), userValidationSalt...), userKeySalt...)
	oHash := revisionSixHash(preparedPassword, ownerValidationSalt, u)
	o := append(append(append([]byte(nil), oHash...), ownerValidationSalt...), ownerKeySalt...)
	userKey := revisionSixHash(preparedPassword, userKeySalt, nil)
	ownerKey := revisionSixHash(preparedPassword, ownerKeySalt, u)
	ue := encryptAESNoPadding(userKey, fileKey)
	oe := encryptAESNoPadding(ownerKey, fileKey)
	permsPlain := make([]byte, 16)
	permissions := int32(-4)
	binary.LittleEndian.PutUint32(permsPlain, uint32(permissions))
	for i := 4; i < 8; i++ {
		permsPlain[i] = 0xff
	}
	permsPlain[8] = 'F'
	copy(permsPlain[9:], []byte("adb"))
	permsPlain[12] = 'T'
	perms := encryptAESNoPadding(fileKey, permsPlain)
	d := &Document{
		trailer: Dict{Name("Encrypt"): Ref{Object: 5, Generation: 0}, Name("ID"): Array{String("file-id")}},
		objects: map[Ref]Object{{Object: 5, Generation: 0}: Dict{
			Name("Filter"): Name("Standard"), Name("V"): Number(5), Name("R"): Number(6),
			Name("O"): String(o), Name("U"): String(u), Name("OE"): String(oe), Name("UE"): String(ue), Name("Perms"): String(perms), Name("P"): Number(permissions), Name("EncryptMetadata"): Bool(false),
			Name("CF"): Dict{Name("StdCF"): Dict{Name("CFM"): Name("AESV3")}},
		}},
	}
	if err := d.configureSecurity(password); err != nil {
		t.Fatal(err)
	}
	if d.encryptionRevision != 6 || string(d.encryptionKey) != string(fileKey) {
		t.Fatalf("R6 security state = revision %d key %q", d.encryptionRevision, d.encryptionKey)
	}
	if d.streamEncryptionMethod != "AESV3" || d.stringEncryptionMethod != "AESV3" {
		t.Fatalf("R6 omitted crypt-filter defaults = stream:%q string:%q", d.streamEncryptionMethod, d.stringEncryptionMethod)
	}
	if err := d.configureSecurity("secret-r6"); err != nil {
		t.Fatalf("equivalent normalized R6 password was rejected: %v", err)
	}
	enc := d.objects[Ref{Object: 5, Generation: 0}].(Dict)
	d.objects[Ref{Object: 8, Generation: 0}] = Bool(false)
	enc[Name("EncryptMetadata")] = Ref{Object: 8, Generation: 0}
	if err := d.configureSecurity(password); err != nil {
		t.Fatalf("indirect R6 EncryptMetadata was rejected: %v", err)
	}
	if !d.metadataExcluded {
		t.Fatal("indirect R6 EncryptMetadata=false did not exclude metadata")
	}
	enc[Name("EncryptMetadata")] = Bool(true)
	if revisionFivePermissions(fileKey, enc, d, true) {
		t.Fatal("metadata flag mismatch was accepted")
	}
}

func TestRevisionSixPasswordAppliesSASLPrep(t *testing.T) {
	got, err := revisionSixPassword("I\u00adX\u00a0\u212b")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "IX Å" {
		t.Fatalf("SASLprep password = %q, want %q", got, "IX Å")
	}
	if _, err := revisionSixPassword("secret\x07"); err == nil {
		t.Fatal("SASLprep accepted an ASCII control character")
	}
}

func TestRevisionSixPasswordUsesUnicode32NormalizationCorrections(t *testing.T) {
	tests := []struct {
		input rune
		want  rune
	}{
		{input: '\U0002f868', want: '\U0002136a'},
		{input: '\U0002f874', want: '\u5f33'},
		{input: '\U0002f91f', want: '\u43ab'},
		{input: '\U0002f95f', want: '\u7aae'},
		{input: '\U0002f9bf', want: '\u4d57'},
	}
	for _, test := range tests {
		got, err := revisionSixPassword(string(test.input))
		if err != nil {
			t.Fatalf("revisionSixPassword(U+%04X): %v", test.input, err)
		}
		if string(got) != string(test.want) {
			t.Errorf("revisionSixPassword(U+%04X) = %U, want %U", test.input, []rune(string(got)), test.want)
		}
	}
}

func TestRevisionSixHashMatchesStandardVector(t *testing.T) {
	want := "8c880bdc90ef9e6fd13193cfc07f56d5ff8d39e3f375d00198ef01fd19a06ecd"
	if got := fmt.Sprintf("%x", revisionSixHash([]byte("password"), []byte("12345678"), []byte("userdata"))); got != want {
		t.Fatalf("R6 hash = %s, want %s", got, want)
	}
}

func TestRevisionFivePlusObjectKeyUsesFileKeyForAESV3(t *testing.T) {
	fileKey := []byte("01234567890123456789012345678901")
	ref := Ref{Object: 7, Generation: 2}
	for _, revision := range []int{5, 6} {
		d := &Document{encrypted: true, encryptionKey: fileKey, encryptionRevision: revision, encryptionMethod: "AESV3"}
		if got := d.objectKey(ref); !bytes.Equal(got, fileKey) {
			t.Errorf("R%d AESV3 object key = %x, want file key %x", revision, got, fileKey)
		}
	}
}

func TestRevisionFiveDecryptsAES256StringObject(t *testing.T) {
	d := &Document{
		encrypted:          true,
		encryptionKey:      []byte("01234567890123456789012345678901"),
		encryptionRevision: 5,
		encryptionMethod:   "AESV3",
	}
	ref := Ref{Object: 7, Generation: 2}
	plain := []byte("AES-256 object")
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	plain = append(plain, bytes.Repeat([]byte{byte(padding)}, padding)...)
	iv := []byte("0123456789abcdef")
	block, err := aes.NewCipher(d.objectKey(ref))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := append([]byte(nil), iv...)
	ciphertext = append(ciphertext, make([]byte, len(plain))...)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext[aes.BlockSize:], plain)
	got, ok := d.decryptObject(ref, String(ciphertext)).(String)
	if !ok || string(got) != "AES-256 object" {
		t.Fatalf("decrypted object = %q, want %q", got, "AES-256 object")
	}
}

func TestDecryptObjectLeavesExcludedMetadataStreamPlain(t *testing.T) {
	d := &Document{
		encrypted:          true,
		encryptionKey:      []byte{1, 2, 3, 4, 5},
		encryptionRevision: 2,
		metadataExcluded:   true,
	}
	ref := Ref{Object: 9}
	plain := []byte("<x:xmpmeta/>")
	stream := newStream(Dict{Name("Type"): Name("Metadata"), Name("Subtype"): Name("XML")}, plain)
	got, ok := d.decryptObject(ref, stream).(Stream)
	if !ok || string(got.DataBorrowed()) != string(plain) {
		t.Fatalf("metadata stream = %#v, want plaintext %q", got, plain)
	}
}

func TestDecryptObjectDecryptsExcludedMetadataStreamDictionary(t *testing.T) {
	d := &Document{
		encrypted:          true,
		encryptionKey:      []byte{1, 2, 3, 4, 5},
		encryptionRevision: 2,
		metadataExcluded:   true,
	}
	ref := Ref{Object: 9}
	key := d.objectKey(ref)
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	encrypted := make([]byte, len("metadata-dict"))
	cipher.XORKeyStream(encrypted, []byte("metadata-dict"))
	stream := newStream(Dict{Name("Type"): Name("Metadata"), Name("Subtype"): Name("XML"), Name("Label"): String(encrypted)}, []byte("<x:xmpmeta/>"))
	got, ok := d.decryptObject(ref, stream).(Stream)
	if !ok || string(got.DataBorrowed()) != "<x:xmpmeta/>" {
		t.Fatalf("metadata stream data = %#v", got)
	}
	if label, ok := got.DictBorrowed()[Name("Label")].(String); !ok || string(label) != "metadata-dict" {
		t.Fatalf("metadata stream dictionary label = %#v, want decrypted string", got.DictBorrowed()[Name("Label")])
	}
}

func TestDecryptObjectRecognizesIndirectExcludedMetadataStream(t *testing.T) {
	d := &Document{
		encrypted:          true,
		encryptionKey:      []byte{1, 2, 3, 4, 5},
		encryptionRevision: 2,
		metadataExcluded:   true,
		objects: map[Ref]Object{
			{Object: 1}: Name("Metadata"),
			{Object: 2}: Name("XML"),
		},
	}
	plain := []byte("<x:xmpmeta/>")
	stream := newStream(Dict{Name("Type"): Ref{Object: 1}, Name("Subtype"): Ref{Object: 2}}, plain)
	got, ok := d.decryptObject(Ref{Object: 9}, stream).(Stream)
	if !ok || string(got.DataBorrowed()) != string(plain) {
		t.Fatalf("metadata stream = %#v, want plaintext %q", got, plain)
	}
}

func TestRecordMetadataRefFollowsIndirectReferenceChains(t *testing.T) {
	d := &Document{
		trailer:          Dict{Name("Root"): Ref{Object: 1}},
		objects:          map[Ref]Object{{Object: 1}: Dict{Name("Metadata"): Ref{Object: 2}}, {Object: 2}: Ref{Object: 3}, {Object: 3}: newStream(nil, []byte("<x:xmpmeta/>"))},
		metadataExcluded: true,
	}
	d.recordMetadataRef()
	if !d.metadataRefs[Ref{Object: 2}] || !d.metadataRefs[Ref{Object: 3}] {
		t.Fatalf("metadata references = %#v", d.metadataRefs)
	}
	plain := []byte("<x:xmpmeta/>")
	got, ok := d.decryptObject(Ref{Object: 3}, newStream(nil, plain)).(Stream)
	if !ok || string(got.DataBorrowed()) != string(plain) {
		t.Fatalf("metadata stream = %#v, want plaintext %q", got, plain)
	}
}

func TestRecordMetadataRefFollowsIndirectCatalogAndTypeChains(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Ref{Object: 2},
			{Object: 2}: Dict{Name("Metadata"): Ref{Object: 3}},
			{Object: 3}: Ref{Object: 4},
			{Object: 4}: newStream(Dict{Name("Type"): Ref{Object: 5}, Name("Subtype"): Ref{Object: 6}}, []byte("<x:xmpmeta/>")),
			{Object: 5}: Name("Metadata"),
			{Object: 6}: Name("XML"),
		},
		metadataExcluded: true,
	}
	d.recordMetadataRef()
	if !d.metadataRefs[Ref{Object: 3}] || !d.metadataRefs[Ref{Object: 4}] {
		t.Fatalf("metadata references = %#v", d.metadataRefs)
	}
	plain := []byte("<x:xmpmeta/>")
	got, ok := d.decryptObject(Ref{Object: 4}, newStream(Dict{Name("Type"): Ref{Object: 5}, Name("Subtype"): Ref{Object: 6}}, plain)).(Stream)
	if !ok || string(got.DataBorrowed()) != string(plain) {
		t.Fatalf("metadata stream = %#v, want plaintext %q", got, plain)
	}
}

func TestRecordMetadataRefStopsIndirectCycle(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Metadata"): Ref{Object: 2}},
			{Object: 2}: Ref{Object: 3},
			{Object: 3}: Ref{Object: 2},
		},
		metadataExcluded: true,
	}
	d.recordMetadataRef()
	if !d.metadataRefs[Ref{Object: 2}] || !d.metadataRefs[Ref{Object: 3}] || len(d.metadataRefs) != 2 {
		t.Fatalf("cyclic metadata references = %#v, want exactly the visited cycle", d.metadataRefs)
	}
}

func TestRecordMetadataRefStopsCatalogCycle(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Ref{Object: 2},
			{Object: 2}: Ref{Object: 1},
		},
		metadataExcluded: true,
	}
	d.recordMetadataRef()
	if len(d.metadataRefs) != 0 {
		t.Fatalf("catalog cycle recorded metadata references = %#v, want empty", d.metadataRefs)
	}
}

func TestDecryptObjectSkipsCatalogMetadataWithoutTypeHints(t *testing.T) {
	ref := Ref{Object: 9}
	d := &Document{
		trailer:          Dict{Name("Root"): Ref{Object: 1}},
		objects:          map[Ref]Object{{Object: 1}: Dict{Name("Metadata"): ref}},
		metadataExcluded: true,
	}
	d.recordMetadataRef()
	plain := []byte("<x:xmpmeta/>")
	stream := newStream(Dict{}, plain)
	got, ok := d.decryptObject(ref, stream).(Stream)
	if !ok || string(got.DataBorrowed()) != string(plain) {
		t.Fatalf("catalog metadata stream = %#v, want plaintext %q", got, plain)
	}
}

func TestDecryptObjectSkipsDirectCatalogMetadataWithoutTypeHints(t *testing.T) {
	d := &Document{
		encrypted:          true,
		encryptionKey:      []byte{1, 2, 3, 4, 5},
		encryptionRevision: 2,
		metadataExcluded:   true,
		trailer:            Dict{Name("Root"): Ref{Object: 1}},
	}
	plain := []byte("<x:xmpmeta/>")
	key := d.objectKey(Ref{Object: 1})
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	encryptedLabel := make([]byte, len("metadata-dict"))
	cipher.XORKeyStream(encryptedLabel, []byte("metadata-dict"))
	catalog := Dict{Name("Metadata"): newStream(Dict{Name("Label"): String(encryptedLabel)}, plain)}
	got, ok := d.decryptObject(Ref{Object: 1}, catalog).(Dict)
	if !ok {
		t.Fatalf("catalog = %#v", got)
	}
	stream, ok := got[Name("Metadata")].(Stream)
	if !ok || string(stream.DataBorrowed()) != string(plain) {
		t.Fatalf("direct catalog metadata = %#v, want plaintext %q", got[Name("Metadata")], plain)
	}
	if label, ok := stream.DictBorrowed()[Name("Label")].(String); !ok || string(label) != "metadata-dict" {
		t.Fatalf("direct catalog metadata dictionary label = %#v, want decrypted string", stream.DictBorrowed()[Name("Label")])
	}
}

func TestDecryptObjectDoesNotExcludeNestedMetadataKey(t *testing.T) {
	d := &Document{
		encrypted:          true,
		encryptionKey:      []byte{1, 2, 3, 4, 5},
		encryptionRevision: 2,
		metadataExcluded:   true,
		trailer:            Dict{Name("Root"): Ref{Object: 1}},
	}
	ref := Ref{Object: 2}
	plain := []byte("nested metadata")
	cipher, err := rc4.NewCipher(d.objectKey(ref))
	if err != nil {
		t.Fatal(err)
	}
	encrypted := make([]byte, len(plain))
	cipher.XORKeyStream(encrypted, plain)
	object := Dict{Name("Metadata"): newStream(Dict{}, encrypted)}
	got, ok := d.decryptObject(ref, object).(Dict)
	if !ok {
		t.Fatalf("nested object = %#v", got)
	}
	stream, ok := got[Name("Metadata")].(Stream)
	if !ok || string(stream.DataBorrowed()) != string(plain) {
		t.Fatalf("nested metadata stream = %#v, want plaintext %q", got[Name("Metadata")], plain)
	}
}

func encryptAESNoPadding(key, plain []byte) []byte {
	block, _ := aes.NewCipher(key)
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(out, plain)
	return out
}
