package document

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"math"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/imagedata"
)

func TestImageParentWithErrorReportsMalformedParentTree(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Number(1)}}
	image := ImageObject{parentKey: 7, hasParentKey: true}
	if parent, err := image.ParentWithError(d); err == nil || parent != nil {
		t.Fatalf("image parent = %#v, err = %v", parent, err)
	}
	if parent := image.Parent(d); parent != nil {
		t.Fatalf("compatibility image parent = %#v", parent)
	}
}

func TestImageColorSpaceAccessorsDoNotExposeMutableStorage(t *testing.T) {
	info := newImageColorSpace("Indexed", 0)
	info.data = info.data.WithBase(newImageColorSpace("DeviceRGB", 3).data).WithColorants([]string{"SpotA"}).WithSpec(Array{Name("Indexed"), Name("DeviceRGB"), Number(255)})
	info.lookup = []byte{1, 2, 3}

	base, ok := info.BaseCopy()
	if !ok || base.Name() != "DeviceRGB" {
		t.Fatalf("base copy = %#v, %v", base, ok)
	}
	colorants := info.ColorantsCopy()
	colorants[0] = "Changed"
	lookup := info.LookupCopy()
	lookup[0] = 9
	if base.Name() != "DeviceRGB" || info.ColorantsCopy()[0] != "SpotA" || info.lookup[0] != 1 {
		t.Fatalf("color space accessors exposed mutable storage: %#v", info)
	}
	spec, ok := info.SpecCopy().(Array)
	if !ok || len(spec) != 3 || spec[0] != Name("Indexed") {
		t.Fatalf("color space spec = %#v", info.SpecCopy())
	}
	spec[0] = Name("changed")
	if got := info.SpecCopy().(Array)[0]; got != Name("Indexed") {
		t.Fatalf("color space spec was not owned: %#v", got)
	}

	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"Base"`, `"Colorants"`, `"Lookup"`, `"Spec"`} {
		if !bytes.Contains(encoded, []byte(field)) {
			t.Fatalf("JSON projection omitted %s: %s", field, encoded)
		}
	}
}

func TestImageObjectJSONIncludesPrivateMetadata(t *testing.T) {
	image := ImageObject{
		name: "Im1", inline: true, offset: 12, page: Ref{Object: 7}, hasPage: true,
		width: 2, height: 3, bpc: 8, colorSpace: "DeviceRGB", components: 3,
		indexedComponents: 3, indexedHigh: 255, imageMask: true, hasMask: true,
		hasSoftMask: true, bbox: [4]float64{1, 2, 3, 4}, parentKey: 5, hasParentKey: true,
		markedTag: "Figure", actualText: "image", mcid: 9, hasMCID: true,
	}
	data, err := json.Marshal(image)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"Name":"Im1"`, `"Inline":true`, `"Offset":12`, `"HasPage":true`, `"Width":2`, `"Height":3`, `"BPC":8`, `"ColorSpace":"DeviceRGB"`, `"ImageMask":true`, `"HasMask":true`, `"HasSoftMask":true`, `"BBox":[1,2,3,4]`, `"HasParentKey":true`, `"marked_tag":"Figure"`, `"actual_text":"image"`, `"has_mcid":true`} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("image JSON omitted %s: %s", want, data)
		}
	}
}

func TestImageObjectExposesRawBufferAndDictionaryAccess(t *testing.T) {
	im := ImageObject{data: []byte{1, 2}, dict: Dict{Name("Width"): Number(2), Name("Meta"): Dict{Name("Value"): String("original")}}}
	buffer := im.Buffer()
	if string(buffer) != string([]byte{1, 2}) {
		t.Fatalf("buffer = %v", buffer)
	}
	buffer[0] = 9
	if im.data[0] != 1 {
		t.Fatal("buffer aliases image data")
	}
	if !im.Has(Name("Width")) || im.Has(Name("Height")) {
		t.Fatal("image dictionary membership mismatch")
	}
	value, ok := im.Get(Name("Width"))
	if !ok || value != Number(2) {
		t.Fatalf("image dictionary value = %v, %v", value, ok)
	}
	value, ok = im.Get(Name("Meta"))
	if !ok {
		t.Fatal("image dictionary entry is missing")
	}
	value.(Dict)[Name("Value")] = String("changed")
	value, _ = im.Get(Name("Meta"))
	if got := value.(Dict)[Name("Value")].(String); string(got) != "original" {
		t.Fatalf("image dictionary value was exposed: %#v", value)
	}
}

func TestImageObjectDecodedStreamBufferAppliesFilters(t *testing.T) {
	im := ImageObject{
		data:    []byte("68656c6c6f>"),
		filters: []string{"ASCIIHexDecode"},
	}
	decoded, err := im.DecodedStreamBufferWithError()
	if err != nil || string(decoded) != "hello" {
		t.Fatalf("decoded stream = %q, err=%v", decoded, err)
	}
	decoded[0] = 'X'
	if string(im.data) != "68656c6c6f>" {
		t.Fatalf("decoded stream aliases encoded data: %q", im.data)
	}
	if got := string(im.DecodedStreamBuffer()); got != "hello" {
		t.Fatalf("compatibility decoded stream = %q", got)
	}
}

func TestImageObjectDecodedStreamBufferDigestAvoidsReturningPayload(t *testing.T) {
	im := ImageObject{data: []byte("hello")}
	length, digest, err := im.DecodedStreamBufferDigestWithError()
	if err != nil {
		t.Fatal(err)
	}
	if length != 5 || digest != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("digest = %d/%q", length, digest)
	}
}

func TestImageFromStreamDoesNotShareDictionary(t *testing.T) {
	stream := newStream(Dict{Name("Meta"): Dict{Name("Value"): String("original")}}, nil)
	image := imageFromStream(&Document{}, "", stream)
	stream.DictBorrowed()[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	value, _ := image.Get(Name("Meta"))
	if got := value.(Dict)[Name("Value")].(String); string(got) != "original" {
		t.Fatalf("image dictionary shares source stream: %q", got)
	}
}

func TestImageFromStreamBorrowsEncodedBytesUntilFinalize(t *testing.T) {
	data := []byte{1, 2, 3}
	image := imageFromStream(&Document{}, "", newStream(nil, data))
	data[0] = 9
	if got := image.Buffer(); !bytes.Equal(got, []byte{9, 2, 3}) {
		t.Fatalf("borrowed image bytes = %v", got)
	}
	snapshot := image.Finalize()
	data[0] = 7
	if got := snapshot.Buffer(); !bytes.Equal(got, []byte{9, 2, 3}) {
		t.Fatalf("finalized image retained borrowed bytes = %v", got)
	}
}

func TestImageBuildWaiterReusesUncachedCompletedImage(t *testing.T) {
	ref := Ref{Object: 9}
	d := &Document{imageCache: map[Ref]ImageObject{}, imageBuilds: map[Ref]*imageBuild{}, cacheOptionsConfigured: true, cacheOptions: cacheconfig.Options{ImageBytes: 0}}
	_, build, owner := d.acquireImageBuild(ref)
	if !owner || build == nil {
		t.Fatalf("first image build = %#v, owner=%v", build, owner)
	}
	_, waiterBuild, waiterOwner := d.acquireImageBuild(ref)
	if waiterOwner || waiterBuild != build {
		t.Fatalf("image waiter = %#v, owner=%v", waiterBuild, waiterOwner)
	}
	image := ImageObject{width: 2, height: 3, data: []byte{1, 2}}
	d.finishImageBuild(ref, image)
	select {
	case <-build.done:
	default:
		t.Fatal("completed image build did not notify waiter")
	}
	if waiterBuild.image.width != 2 || waiterBuild.image.height != 3 {
		t.Fatalf("waiter image = %#v", waiterBuild.image)
	}
	if len(d.imageCache) != 0 {
		t.Fatalf("oversized-free uncached image unexpectedly retained: %#v", d.imageCache)
	}
}

func TestImageFromStreamPreservesEmptyData(t *testing.T) {
	image := imageFromStream(&Document{}, "", newStream(nil, make([]byte, 0)))
	if image.data == nil || image.Buffer() == nil {
		t.Fatal("empty image stream data became nil")
	}
}

func TestImageFromStreamDoesNotTreatInvalidBPCAsDefault(t *testing.T) {
	for _, value := range []Number{Number(math.NaN()), Number(math.Inf(1)), Number(-1), Number(3)} {
		image := imageFromStream(&Document{}, "", newStream(Dict{
			Name("Width"):  Number(1),
			Name("Height"): Number(1),
			Name("BPC"):    value,
		}, nil))
		if image.bpc == 1 {
			t.Fatalf("invalid BPC %v was converted to default %d", value, image.bpc)
		}
	}
	image := imageFromStream(&Document{}, "", newStream(Dict{
		Name("Width"):  Number(1),
		Name("Height"): Number(1),
	}, nil))
	if image.bpc != 1 {
		t.Fatalf("missing BPC default = %d, want 1", image.bpc)
	}
}

func TestDecodedImagePixCopyDoesNotExposeSource(t *testing.T) {
	image := imagedata.BorrowedDecodedImage(0, 0, 0, 0, []byte{1, 2}, "")
	pix := image.PixCopy()
	pix[0] = 9
	if got := image.PixCopy(); len(got) != 2 || got[0] != 1 {
		t.Fatal("decoded image pixel copy aliases source")
	}
}

func TestDecodedImageFinalizeDoesNotSharePixels(t *testing.T) {
	image := imagedata.BorrowedDecodedImage(0, 0, 0, 0, []byte{1, 2}, "")
	copy := image.Finalize()
	pix := copy.PixCopy()
	pix[0] = 9
	if got := image.PixCopy(); len(got) != 2 || got[0] != 1 {
		t.Fatalf("decoded image snapshot shares pixels: %v", got)
	}
}

func TestImageSnapshotsPreserveEmptyByteAndFilterSlices(t *testing.T) {
	image := ImageObject{
		data:          make([]byte, 0),
		decodedData:   make([]byte, 0),
		decode:        make([]float64, 0),
		indexedLookup: make([]byte, 0),
		filters:       make([]string, 0),
	}
	if image.Buffer() == nil || image.DecodedBuffer() == nil || image.DecodeCopy() == nil || image.IndexedLookupCopy() == nil || image.FiltersCopy() == nil {
		t.Fatal("empty image copies became nil")
	}
	snapshot := image.Finalize()
	if snapshot.data == nil || snapshot.decodedData == nil || snapshot.decode == nil || snapshot.indexedLookup == nil || snapshot.filters == nil {
		t.Fatalf("empty image slices were not preserved: %#v", snapshot)
	}
	pixels := imagedata.BorrowedDecodedImage(0, 0, 0, 0, make([]byte, 0), "")
	if pixels.PixCopy() == nil || pixels.Finalize().PixCopy() == nil {
		t.Fatal("empty decoded image pixels became nil")
	}
}

func TestImageRawFiltersCopyPreservesFilterKeySpelling(t *testing.T) {
	image := ImageObject{filters: []string{"AHx"}}
	if filters := image.RawFiltersCopy(); filters != nil {
		t.Fatalf("abbreviated filter projection = %#v, want nil", filters)
	}

	image.hasRawFilter = true
	filters := image.RawFiltersCopy()
	if len(filters) != 1 || filters[0] != "AHx" {
		t.Fatalf("full filter projection = %#v, want [AHx]", filters)
	}
	filters[0] = "mutated"
	if image.filters[0] != "AHx" {
		t.Fatal("raw filter projection aliases image filters")
	}
}

func TestImageSnapshotsPreserveEmptyMarkedStack(t *testing.T) {
	image := ImageObject{markedStack: make([]markedContentContext, 0)}
	snapshot := image.Finalize()
	if snapshot.markedStack == nil {
		t.Fatal("empty image marked-content stack became nil")
	}
}

func TestImagesEmpty(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	if got, err := d.Images(Page{dict: Dict{}}); err != nil || got != nil {
		t.Fatalf("images = %#v, err = %v", got, err)
	}
}

func TestPageImagesFollowMultiLevelIndirectResourceChains(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Ref{Object: 2}
	d.objects[Ref{Object: 2}] = Dict{Name("XObject"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Dict{Name("Im"): Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = Ref{Object: 5}
	d.objects[Ref{Object: 5}] = newStream(Dict{Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1), Name("ColorSpace"): Name("DeviceGray"), Name("BitsPerComponent"): Number(8)}, []byte{0})
	p := Page{dict: Dict{Name("Resources"): Ref{Object: 1}}}

	var images []ImageObject
	for image, err := range d.ImagesSeq(p) {
		if err != nil {
			t.Fatalf("ImagesSeq() error = %v", err)
		}
		images = append(images, image)
	}
	if len(images) != 1 || images[0].name != "Im" || images[0].width != 1 || images[0].height != 1 {
		t.Fatalf("ImagesSeq() = %#v", images)
	}
}

func TestImagesSeqReusesValidatedPageResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("XObject"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("Im"): Ref{Object: 3}},
		{Object: 3}: newStream(Dict{Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1), Name("ColorSpace"): Name("DeviceGray"), Name("BitsPerComponent"): Number(8)}, []byte{0}),
	}}
	p := Page{ref: Ref{Object: 9}, dict: Dict{Name("Resources"): Ref{Object: 1}}}
	if _, err := p.ResourcesWithError(d); err != nil {
		t.Fatal(err)
	}
	d.objects[Ref{Object: 1}] = Dict{}
	var images []ImageObject
	for image, err := range d.ImagesSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		images = append(images, image)
	}
	if len(images) != 1 || images[0].name != "Im" {
		t.Fatalf("ImagesSeq did not reuse validated resources: %#v", images)
	}
}

func TestImageObjectFollowsMultiLevelIndirectMetadata(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = newStream(Dict{
		Name("Subtype"):          Name("Image"),
		Name("Width"):            Ref{Object: 2},
		Name("Height"):           Ref{Object: 3},
		Name("BitsPerComponent"): Ref{Object: 4},
		Name("ColorSpace"):       Ref{Object: 5},
		Name("Decode"):           Ref{Object: 6},
	}, []byte{0})
	d.objects[Ref{Object: 2}] = Ref{Object: 7}
	d.objects[Ref{Object: 3}] = Ref{Object: 8}
	d.objects[Ref{Object: 4}] = Ref{Object: 9}
	d.objects[Ref{Object: 5}] = Ref{Object: 10}
	d.objects[Ref{Object: 6}] = Ref{Object: 11}
	d.objects[Ref{Object: 7}] = Number(2)
	d.objects[Ref{Object: 8}] = Number(3)
	d.objects[Ref{Object: 9}] = Number(8)
	d.objects[Ref{Object: 10}] = Name("DeviceGray")
	d.objects[Ref{Object: 11}] = Array{Number(0), Number(1)}
	p := Page{dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Im"): Ref{Object: 1}}},
	}}

	var images []ImageObject
	for image, err := range d.ImagesSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		images = append(images, image)
	}
	if len(images) != 1 || images[0].width != 2 || images[0].height != 3 || images[0].bpc != 8 || images[0].colorSpace != "DeviceGray" {
		t.Fatalf("image metadata = %#v", images)
	}
}

func TestImagesReportMalformedResourceRoots(t *testing.T) {
	tests := []struct {
		name string
		page Page
	}{
		{name: "resources", page: Page{dict: Dict{Name("Resources"): Number(1)}}},
		{name: "xobject", page: Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Number(1)}}}},
		{name: "xobject entry", page: Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Bad"): Number(1)}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range (&Document{}).ImagesSeq(test.page) {
				if err == nil {
					t.Fatal("malformed resource root produced an image")
				}
				return
			}
			t.Fatal("malformed resource root produced no error")
		})
	}
}

func TestImagesReportUnresolvedResourceRoots(t *testing.T) {
	tests := []struct {
		name string
		page Page
		want string
	}{
		{name: "resources", page: Page{dict: Dict{Name("Resources"): Ref{Object: 99}}}, want: "playa: Resources could not be resolved"},
		{name: "xobject", page: Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Ref{Object: 99}}}}, want: "playa: XObject resources could not be resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range (&Document{}).ImagesSeq(test.page) {
				if err == nil || err.Error() != test.want {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
				return
			}
			t.Fatal("unresolved image resource produced no error")
		})
	}
}

func TestImagesReportUnresolvedResourceEntry(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: Dict{Name("XObject"): Dict{Name("Im"): Ref{Object: 99}}}}}
	p := Page{dict: Dict{Name("Resources"): Ref{Object: 1}}}
	for image, err := range d.ImagesSeq(p) {
		if err == nil || image.name != "" {
			t.Fatalf("unresolved image = %#v, err=%v", image, err)
		}
		return
	}
	t.Fatal("unresolved image produced no error")
}

func TestImagesReportUnresolvedSubtype(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: newStream(Dict{Name("Subtype"): Ref{Object: 99}, Name("Width"): Number(1)}, []byte{1})}}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Im"): Ref{Object: 1}}}}}
	for image, err := range d.ImagesSeq(p) {
		if err == nil || image.name != "" {
			t.Fatalf("unresolved subtype image = %#v, err=%v", image, err)
		}
		return
	}
	t.Fatal("unresolved subtype produced no error")
}

func TestImagesReportUnresolvedMetadataReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: newStream(Dict{
		Name("Subtype"): Name("Image"), Name("Width"): Ref{Object: 99}, Name("Height"): Number(1),
	}, []byte{1})}}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Im"): Ref{Object: 1}}}}}
	for image, err := range d.ImagesSeq(p) {
		if err == nil || image.name != "" {
			t.Fatalf("unresolved image metadata = %#v, err=%v", image, err)
		}
		return
	}
	t.Fatal("unresolved image metadata produced no error")
}

func TestImagesReportUnresolvedColorSpaceReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: newStream(Dict{
		Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1),
		Name("ColorSpace"): Ref{Object: 99},
	}, []byte{1})}}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Im"): Ref{Object: 1}}}}}
	for image, err := range d.ImagesSeq(p) {
		if err == nil || image.name != "" {
			t.Fatalf("unresolved color space image = %#v, err=%v", image, err)
		}
		return
	}
	t.Fatal("unresolved color space produced no error")
}

func TestImagesReportUnresolvedDecodeReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: newStream(Dict{
		Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1),
		Name("ColorSpace"): Name("DeviceGray"), Name("Decode"): Array{Number(0), Ref{Object: 99}},
	}, []byte{1})}}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{Name("Im"): Ref{Object: 1}}}}}
	for image, err := range d.ImagesSeq(p) {
		if err == nil || image.name != "" {
			t.Fatalf("unresolved decode image = %#v, err=%v", image, err)
		}
		return
	}
	t.Fatal("unresolved decode produced no error")
}

func TestImageFinalizeDoesNotShareCachedFields(t *testing.T) {
	ref := Ref{Object: 7}
	stream := newStream(Dict{Name("Subtype"): Name("Image"), Name("Width"): Number(1)}, []byte{1, 2})
	d := &Document{}
	first := d.cachedImage("Im", ref, stream)
	snapshot := first.Finalize()
	snapshot.data[0] = 9
	snapshot.dict[Name("Width")] = Number(2)
	value, _ := first.Get(Name("Width"))
	if value != Number(1) {
		t.Fatalf("first image width = %v", value)
	}
	second := d.cachedImage("Im", ref, stream)
	if second.data[0] != 1 {
		t.Fatalf("image cache data was exposed: %v", second.data)
	}
	value, _ = second.Get(Name("Width"))
	if value != Number(1) {
		t.Fatalf("image cache dictionary was exposed: %#v", value)
	}
}

func TestImageFinalizeDoesNotShareLazyDecodeCaches(t *testing.T) {
	first := ImageObject{
		data:               []byte{1, 2},
		decodedCache:       &imageDecodeCache{data: []byte{3}},
		indexedLookupCache: &imageDecodeCache{data: []byte{4}},
	}
	snapshot := first.Finalize()
	if snapshot.decodedCache != nil || snapshot.indexedLookupCache != nil {
		t.Fatalf("image snapshot retained lazy cache pointers: decoded=%p indexed=%p", snapshot.decodedCache, snapshot.indexedLookupCache)
	}
}

func TestImageColorSpaceFinalizeDoesNotShareLookupCache(t *testing.T) {
	original := newImageColorSpace("Indexed", 0)
	original.lookup = []byte{1, 2}
	original.lookupCache = &imageDecodeCache{data: []byte{3}}
	snapshot := original.Finalize()
	if snapshot.lookupCache != nil {
		t.Fatalf("color-space snapshot retained lookup cache: %p", snapshot.lookupCache)
	}
	if got := snapshot.LookupCopy(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("color-space snapshot lookup data = %v", got)
	}
}

func TestImageColorSpaceSnapshotsPreserveEmptySlices(t *testing.T) {
	original := newImageColorSpace("", 0)
	original.data = original.data.WithColorants(make([]string, 0))
	original.lookup = make([]byte, 0)
	original.lookupData = make([]byte, 0)
	original.lookupFilters = make([]string, 0)
	snapshot := original.Finalize()
	if snapshot.ColorantsCopy() == nil || snapshot.lookup == nil || snapshot.lookupData == nil || snapshot.lookupFilters == nil {
		t.Fatalf("empty color-space slices were not preserved: %#v", snapshot)
	}
	if original.ColorantsCopy() == nil || original.LookupCopy() == nil {
		t.Fatal("empty color-space copies became nil")
	}
}

func TestImageColorSpaceLookupLoaderPreservesEmptyData(t *testing.T) {
	lookup, data, _, _ := (&Document{}).resolveLookup(String(make([]byte, 0)))
	if lookup == nil || data != nil {
		t.Fatalf("empty direct lookup = %v, data = %v", lookup, data)
	}
	lookup, data, _, _ = (&Document{}).resolveLookup(newStream(nil, make([]byte, 0)))
	if lookup == nil || data != nil {
		t.Fatalf("empty stream lookup = %v, data = %v", lookup, data)
	}
}

func TestImageColorSpaceLookupReadsReadyEmptyCache(t *testing.T) {
	info := newImageColorSpace("Indexed", 0)
	info.lookupData = make([]byte, 0)
	info.lookupFilters = []string{"FlateDecode"}
	info.lookupCache = &imageDecodeCache{ready: true, data: make([]byte, 0)}
	if lookup := info.LookupCopy(); lookup == nil {
		t.Fatal("ready empty color-space lookup cache was reported as unavailable")
	}
}

func TestImageColorSpaceLookupPreservesEmptyUnfilteredData(t *testing.T) {
	info := ImageColorSpace{lookupData: make([]byte, 0)}
	if lookup := info.LookupCopy(); lookup == nil {
		t.Fatal("empty unfiltered color-space lookup data became nil")
	}
}

func TestImageColorSpaceFinalizeWithErrorReportsLookupFailures(t *testing.T) {
	original := newImageColorSpace("Indexed", 0)
	original.lookupData = []byte{1, 2, 3}
	original.lookupFilters = []string{"UnknownFilter"}
	if snapshot, err := original.FinalizeWithError(); err == nil || snapshot.lookupData != nil {
		t.Fatalf("color-space snapshot = %#v, err=%v", snapshot, err)
	}
}

func TestImageFinalizeWithErrorReportsIndexedLookupFailures(t *testing.T) {
	original := ImageObject{
		colorSpace:           "Indexed",
		indexedLookupData:    []byte{1, 2, 3},
		indexedLookupFilters: []string{"UnknownFilter"},
	}
	if snapshot, err := original.FinalizeWithError(); err == nil || snapshot.indexedLookupData != nil {
		t.Fatalf("image snapshot = %#v, err=%v", snapshot, err)
	}
}

func TestFilteredLargeImageDoesNotRetainDecodedCache(t *testing.T) {
	image := imageFromStream(&Document{}, "large", newStream(Dict{
		Name("Width"): Number(10000), Name("Height"): Number(10000),
		Name("BitsPerComponent"): Number(8), Name("ColorSpace"): Name("DeviceGray"),
		Name("Filter"): Name("FlateDecode"),
	}, []byte{1}))
	if image.decodedCache != nil {
		t.Fatal("large filtered image retained a decoded cache")
	}
}

func TestSixteenBitImageCacheBudgetUsesTwoBytesPerSample(t *testing.T) {
	image := imageFromStream(&Document{}, "16-bit", newStream(Dict{
		Name("Width"): Number(5000), Name("Height"): Number(5000),
		Name("BitsPerComponent"): Number(16), Name("ColorSpace"): Name("DeviceGray"),
		Name("Filter"): Name("FlateDecode"),
	}, []byte{1}))
	if image.decodedCache != nil {
		t.Fatal("16-bit image retained a decoded cache beyond the byte budget")
	}
}

func TestPageImagesPreserveOwningPage(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("Resources"): Dict{Name("XObject"): Dict{Name("Im"): Ref{Object: 4}}}},
			{Object: 4}: newStream(Dict{Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1), Name("BPC"): Number(8), Name("CS"): Name("G")}, []byte{7}),
		},
	}
	p, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	images, err := d.Images(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || !images[0].hasPage || images[0].page != p.ref {
		t.Fatalf("images = %#v", images)
	}
	owned, err := images[0].PageObject(d)
	if err != nil || owned.ref != p.ref {
		t.Fatalf("page = %#v, err = %v", owned, err)
	}
}

func TestImagesDoesNotPublishPartialMalformedResources(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{
		Name("Im1"): newStream(Dict{Name("Subtype"): Name("Image"), Name("W"): Number(1), Name("H"): Number(1), Name("CS"): Name("G")}, []byte{1}),
		Name("Im2"): Number(1),
	}}}}
	if got, err := d.Images(p); err == nil || got != nil {
		t.Fatalf("images from malformed resources = %#v, err = %v", got, err)
	}
}

func TestUnpackImageData(t *testing.T) {
	// Four 2-bit samples packed into one byte, followed by a padded row.
	got := UnpackImageData([]byte{0x1b}, 2, 4, 1, 1)
	want := []byte{0, 1, 2, 3}
	if string(got) != string(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnpackImageDataResetsSampleCountPerRow(t *testing.T) {
	got := UnpackImageData([]byte{0x1b, 0xe4}, 2, 4, 2, 1)
	want := []byte{0, 1, 2, 3, 3, 2, 1, 0}
	if string(got) != string(want) {
		t.Fatalf("multi-row unpack = %v, want %v", got, want)
	}
}

func TestUnpackImageDataBoundsCapacityByInput(t *testing.T) {
	got := UnpackImageData([]byte{0xff}, 1, 1<<30, 1<<30, 1)
	if len(got) != 8 {
		t.Fatalf("unpacked huge image = %d samples, want 8", len(got))
	}
}

func TestUnpackImageDataRejectsOverflowingSampleGeometry(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("overflowing image geometry caused panic: %v", recovered)
		}
	}()
	if got := UnpackImageData([]byte{0xff}, 4, maxInt/2, 2, 2); got != nil {
		t.Fatalf("overflowing image geometry returned %d samples", len(got))
	}
}

func TestDecodeSampleRejectsUnsupportedBitDepth(t *testing.T) {
	im := ImageObject{bpc: 32, decode: []float64{0, 1}}
	if got := im.DecodeSample(0, 7); got != 7 {
		t.Fatalf("unsupported bit depth sample = %v, want 7", got)
	}
}

func TestImageColorSpaceComponents(t *testing.T) {
	if colorSpaceComponents("DeviceRGB") != 3 || colorSpaceComponents("Lab") != 3 || colorSpaceComponents("DeviceCMYK") != 4 {
		t.Fatal("named colorspace component count mismatch")
	}
	if got := arrayColorSpaceComponents(Array{Name("Indexed"), Name("DeviceRGB")}); got != 1 {
		t.Fatalf("indexed component count = %d", got)
	}
}

func TestICCColorSpaceInfersComponentsFromProfileHeader(t *testing.T) {
	data := make([]byte, 20)
	copy(data[16:], []byte("RGB "))
	profile := newStream(Dict{}, data)
	d := &Document{}
	info := d.describeImageColorSpace(Array{Name("ICCBased"), profile})
	if info.Name() != "ICCBased" || info.Components() != 3 || info.ProfileN() != 3 {
		t.Fatalf("ICC inferred colorspace = %#v", info)
	}
	if got := d.resolvedColorSpaceComponents(Array{Name("ICCBased"), profile}); got != 3 {
		t.Fatalf("ICC inferred components = %d, want 3", got)
	}
}

func TestICCColorSpaceInfersNCLRComponentsFromProfileHeader(t *testing.T) {
	data := make([]byte, 20)
	copy(data[16:], []byte("5CLR"))
	profile := newStream(Dict{}, data)
	info := (&Document{}).describeImageColorSpace(Array{Name("ICCBased"), profile})
	if info.Components() != 5 || info.ProfileN() != 5 {
		t.Fatalf("ICC nCLR inferred colorspace = %#v", info)
	}
}

func TestICCColorSpaceFallsBackToThreeComponentsForUnknownProfile(t *testing.T) {
	data := make([]byte, 40)
	copy(data[16:], []byte("XYZ "))
	copy(data[36:], []byte("acsp"))
	profile := newStream(Dict{}, data)
	info := (&Document{}).describeImageColorSpace(Array{Name("ICCBased"), profile})
	if info.Components() != 3 || info.ProfileN() != 3 {
		t.Fatalf("ICC unknown profile inferred colorspace = %#v", info)
	}
}

func TestImageColorSpaceFollowsMultiLevelIndirectComponents(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}:  Ref{Object: 2},
		{Object: 2}:  Array{Ref{Object: 3}, Ref{Object: 4}, Ref{Object: 5}, Ref{Object: 6}},
		{Object: 3}:  Ref{Object: 7},
		{Object: 4}:  Ref{Object: 8},
		{Object: 5}:  Ref{Object: 9},
		{Object: 6}:  Ref{Object: 10},
		{Object: 7}:  Name("Indexed"),
		{Object: 8}:  Name("DeviceRGB"),
		{Object: 9}:  Number(1),
		{Object: 10}: String{0, 1, 2, 3, 4, 5},
	}}
	info := d.describeImageColorSpace(Ref{Object: 1})
	if info.Name() != "Indexed" || info.Components() != 1 || info.High() != 1 || info.ProfileN() != 0 {
		t.Fatalf("multi-level indexed color space = %#v", info)
	}
	base, ok := info.BaseCopy()
	if !ok || base.Name() != "DeviceRGB" || base.Components() != 3 {
		t.Fatalf("multi-level indexed base = %#v, %v", base, ok)
	}
	if got := info.LookupCopy(); !bytes.Equal(got, []byte{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("multi-level indexed lookup = %v", got)
	}
	if got := d.resolvedColorSpaceComponents(Ref{Object: 1}); got != 1 {
		t.Fatalf("multi-level color-space components = %d, want 1", got)
	}
}

func TestICCColorSpaceIgnoresUnreasonableComponentCount(t *testing.T) {
	data := make([]byte, 20)
	copy(data[16:], []byte("RGB "))
	profile := newStream(Dict{Name("N"): Number(1 << 60)}, data)
	info := (&Document{}).describeImageColorSpace(Array{Name("ICCBased"), profile})
	if info.Components() != 3 || info.ProfileN() != 3 {
		t.Fatalf("invalid ICC component count = %#v, want RGB fallback", info)
	}
}

func TestImageColorSpaceRejectsMalformedICCProfiles(t *testing.T) {
	d := &Document{}
	for _, profile := range []Object{
		Number(1),
		newStream(Dict{}, []byte("short")),
		newStream(Dict{}, append(make([]byte, 16), []byte("ABCD")...)),
	} {
		space := Array{Name("ICCBased"), profile}
		if info := d.describeImageColorSpace(space); info.Name() != "" || info.Components() != 0 {
			t.Fatalf("malformed ICC profile was described: %#v", info)
		}
		if components := d.resolvedColorSpaceComponents(space); components != 0 {
			t.Fatalf("malformed ICC profile has %d components", components)
		}
	}
}

func TestPageImagesFollowsContentOrderAndSkipsUnusedResources(t *testing.T) {
	gray := newStream(Dict{Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1), Name("BitsPerComponent"): Number(8), Name("ColorSpace"): Name("DeviceGray")}, []byte{7})
	rgb := newStream(Dict{Name("Subtype"): Name("Image"), Name("Width"): Number(2), Name("Height"): Number(1), Name("BitsPerComponent"): Number(8), Name("ColorSpace"): Name("DeviceRGB")}, []byte{1, 2, 3, 4, 5, 6})
	content := newStream(nil, []byte("q 2 0 0 3 10 20 cm /Im2 Do Q BI /W 1 /H 1 /BPC 8 /CS /DeviceGray ID x EI /Im1 Do"))
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("Contents"):  content,
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Im1"): gray, Name("Im2"): rgb, Name("Unused"): gray}},
	}}
	got, err := d.PageImages(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].name != "Im2" || got[1].name != "" || got[2].name != "Im1" {
		t.Fatalf("unexpected images: %#v", got)
	}
	if got[0].bbox != [4]float64{10, 20, 12, 23} || got[1].bbox != [4]float64{0, 0, 1, 1} {
		t.Fatalf("unexpected image bboxes: %#v", got)
	}
	if got[0].inline || !got[1].inline || got[2].inline {
		t.Fatalf("image source flags = [%v %v %v]", got[0].inline, got[1].inline, got[2].inline)
	}
	if got[0].offset <= 0 || got[1].offset <= 0 || got[2].offset <= got[0].offset {
		t.Fatalf("image offsets = [%d %d %d]", got[0].offset, got[1].offset, got[2].offset)
	}
}

func TestPageImagesSequenceStreamsWithoutMaterializingAllImages(t *testing.T) {
	image := newStream(Dict{Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1), Name("BPC"): Number(8), Name("CS"): Name("G")}, []byte{7})
	d := &Document{objects: map[Ref]Object{}}
	p := Page{dict: Dict{
		Name("Contents"):  newStream(nil, []byte("/Im Do BI /W 1 /H 1 /BPC 8 /CS /G ID x EI")),
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Im"): image}},
	}}
	var names []string
	for got, err := range p.Images(d) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, got.name)
	}
	if len(names) != 2 || names[0] != "Im" || names[1] != "" {
		t.Fatalf("image sequence = %#v", names)
	}
}

func TestPageImagesSequenceIsRepeatableAndStopsEarly(t *testing.T) {
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): newStream(nil, []byte(
		"BI /W 1 /H 1 /BPC 8 /CS /G ID x EI BI /W 1 /H 1 /BPC 8 /CS /G ID y EI",
	))}}
	first := 0
	for image, err := range p.Images(d) {
		if err != nil {
			t.Fatal(err)
		}
		if image.width != 1 {
			t.Fatalf("first image = %#v", image)
		}
		first++
		break
	}
	second := 0
	for image, err := range p.Images(d) {
		if err != nil {
			t.Fatal(err)
		}
		if image.width != 1 {
			t.Fatalf("repeated image = %#v", image)
		}
		second++
	}
	if first != 1 || second != 2 {
		t.Fatalf("image sequence = first %d, second %d", first, second)
	}
}

func TestPageImagesSequenceReusesRepeatedImageResources(t *testing.T) {
	d := &Document{
		objects: map[Ref]Object{
			{Object: 7}: newStream(Dict{Name("Subtype"): Name("Image"), Name("W"): Number(1), Name("H"): Number(1), Name("CS"): Name("G")}, []byte{1}),
		},
	}
	p := Page{ref: Ref{Object: 1}, dict: Dict{
		Name("Resources"): Dict{Name("XObject"): Dict{Name("Im1"): Ref{Object: 7}}},
		Name("Contents"):  newStream(nil, []byte("/Im1 Do /Im1 Do")),
	}}
	var count int
	for image, err := range d.PageImagesSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		if image.name != "Im1" {
			t.Fatalf("image name = %q", image.name)
		}
		count++
	}
	if count != 2 || len(d.imageCache) != 1 {
		t.Fatalf("images=%d cache=%d", count, len(d.imageCache))
	}
}

func TestPageImagesSequenceReusesImageAcrossIndirectReferenceChains(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: newStream(Dict{Name("Subtype"): Name("Image"), Name("W"): Number(1), Name("H"): Number(1), Name("CS"): Name("G")}, []byte{1}),
	}}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{
		Name("First"): Ref{Object: 1}, Name("Second"): Ref{Object: 2},
	}}}}
	var names []string
	for image, err := range d.ImagesSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, image.name)
	}
	if len(names) != 2 || names[0] != "First" || names[1] != "Second" || len(d.imageCache) != 1 {
		t.Fatalf("shared indirect image aliases = %#v cache=%d", names, len(d.imageCache))
	}
}

func TestPageImagesSequencePreservesAliasNamesWithSharedResource(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 7}: newStream(Dict{Name("Subtype"): Name("Image"), Name("W"): Number(1), Name("H"): Number(1), Name("CS"): Name("G")}, []byte{1}),
	}}
	p := Page{dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{
		Name("First"): Ref{Object: 7}, Name("Second"): Ref{Object: 7},
	}}}}
	var names []string
	for image, err := range d.ImagesSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, image.name)
	}
	if len(names) != 2 || names[0] != "First" || names[1] != "Second" {
		t.Fatalf("shared image aliases = %#v", names)
	}
}

func TestDirectImageSequenceStopsBeforeLaterResources(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 7}: newStream(Dict{Name("Subtype"): Name("Image"), Name("W"): Number(1), Name("H"): Number(1), Name("CS"): Name("G")}, []byte{1}),
		{Object: 8}: newStream(Dict{Name("Subtype"): Name("Image"), Name("W"): Number(1), Name("H"): Number(1), Name("CS"): Name("G")}, []byte{2}),
	}}
	p := Page{ref: Ref{Object: 1}, dict: Dict{Name("Resources"): Dict{Name("XObject"): Dict{
		Name("Im1"): Ref{Object: 7}, Name("Im2"): Ref{Object: 8},
	}}}}
	count := 0
	for image, err := range d.ImagesSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		if image.name != "Im1" {
			t.Fatalf("first image = %q", image.name)
		}
		count++
		break
	}
	if count != 1 || len(d.imageCache) != 1 {
		t.Fatalf("count=%d cache=%d", count, len(d.imageCache))
	}
}

func TestPageImagesSequenceDefersLaterStreamErrors(t *testing.T) {
	content := newStream(nil, []byte("BI /W 1 /H 1 /BPC 8 /CS /G ID x EI"))
	d := &Document{}
	p := Page{dict: Dict{Name("Contents"): Array{content, newStream(nil, []byte("("))}}}
	count := 0
	for image, err := range d.PageImagesSeq(p) {
		if err != nil {
			t.Fatal(err)
		}
		if image.width != 1 {
			t.Fatalf("image = %#v", image)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatalf("images = %d", count)
	}
}

func TestIndexedImageSamples(t *testing.T) {
	im := ImageObject{
		width: 2, height: 1, bpc: 1, components: 1,
		colorSpace: "Indexed", indexedComponents: 3,
		indexedLookup: []byte{0, 0, 0, 10, 20, 30},
		decodedData:   []byte{0x40}, // indices 0,1 followed by row padding
	}
	data, components := im.Samples()
	if components != 3 || string(data) != string([]byte{0, 0, 0, 10, 20, 30}) {
		t.Fatalf("samples=%v components=%d", data, components)
	}
}

func TestIndexedImageSamplesRejectsOverflowingPaletteWidth(t *testing.T) {
	im := ImageObject{
		bpc: 8, components: 1, colorSpace: "Indexed", indexedComponents: int(^uint(0) >> 1),
		decodedData: []byte{0}, indexedLookup: []byte{0},
	}
	data, components := im.Samples()
	if data != nil || components != im.indexedComponents {
		t.Fatalf("overflowing indexed samples = %v, %d", data, components)
	}
}

func TestIndexedImageSamplesRejectsPaletteOffsetOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	paletteComponents := maxInt/256 + 1
	im := ImageObject{
		bpc: 8, components: 1, colorSpace: "Indexed", indexedComponents: paletteComponents,
		decodedData: []byte{255}, indexedLookup: []byte{0},
	}
	data, components := im.Samples()
	if data != nil || components != paletteComponents {
		t.Fatalf("overflowing indexed palette offset = %v, %d", data, components)
	}
}

func TestResolvedColorSpaceComponents(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	icc := newStream(Dict{Name("N"): Number(4)}, nil)
	if got := d.resolvedColorSpaceComponents(Array{Name("ICCBased"), icc}); got != 4 {
		t.Fatalf("ICCBased components = %d", got)
	}
	if got := d.resolvedColorSpaceComponents(Array{Name("DeviceN"), Array{Name("C1"), Name("C2")}}); got != 2 {
		t.Fatalf("DeviceN components = %d", got)
	}
	if got := d.resolvedColorSpaceComponents(Array{Name("Separation"), Name("Spot"), Name("DeviceRGB"), Null{}}); got != 1 {
		t.Fatalf("Separation components = %d", got)
	}
	if got := d.resolvedColorSpaceComponents(Array{Name("Pattern"), Name("DeviceRGB")}); got != 4 {
		t.Fatalf("Pattern components = %d", got)
	}
}

func TestColorSpaceRejectsSpecializedExtraValues(t *testing.T) {
	d := &Document{}
	spaces := []Object{
		Array{Name("ICCBased"), Stream{}, Name("extra")},
		Array{Name("Separation"), Name("Spot"), Name("DeviceRGB"), Null{}, Number(1)},
		Array{Name("DeviceN"), Array{Name("C1")}, Name("DeviceRGB"), Null{}, Number(1)},
		Array{Name("Pattern"), Name("DeviceRGB"), Number(1)},
	}
	for _, space := range spaces {
		if info := d.describeImageColorSpace(space); info.Components() != 0 {
			t.Fatalf("malformed color space was described: %#v", info)
		}
		if components := d.resolvedColorSpaceComponents(space); components != 0 {
			t.Fatalf("malformed color space components = %d for %v", components, space)
		}
	}
}

func TestColorSpaceRejectsMalformedCalibratedArrays(t *testing.T) {
	d := &Document{}
	for _, space := range []Object{
		Array{Name("CalGray")},
		Array{Name("CalGray"), Dict{}},
		Array{Name("CalRGB"), Dict{}},
		Array{Name("Lab"), Dict{}},
		Array{Name("CalGray"), Dict{}, Number(1)},
		Array{Name("CalRGB"), Dict{}, Number(1)},
		Array{Name("Lab"), Dict{}, Number(1)},
	} {
		if info := d.describeImageColorSpace(space); info.Name() != "" || info.Components() != 0 {
			t.Fatalf("malformed calibrated color space was described: %#v", info)
		}
		if components := d.resolvedColorSpaceComponents(space); components != 0 {
			t.Fatalf("malformed calibrated color space components = %d", components)
		}
	}
}

func TestColorSpaceRejectsMalformedCalibratedParameters(t *testing.T) {
	whitePoint := Array{Number(1), Number(1), Number(1)}
	for _, space := range []Object{
		Array{Name("CalGray"), Dict{
			Name("WhitePoint"): whitePoint, Name("BlackPoint"): Array{Number(0), Number(0)},
		}},
		Array{Name("CalRGB"), Dict{
			Name("WhitePoint"): whitePoint, Name("Gamma"): Array{Number(1), Number(1)},
		}},
		Array{Name("CalRGB"), Dict{
			Name("WhitePoint"): whitePoint, Name("Matrix"): Array{Number(1)},
		}},
		Array{Name("Lab"), Dict{
			Name("WhitePoint"): whitePoint, Name("Range"): Array{Number(0), Number(1), Number(0)},
		}},
	} {
		if info := (&Document{}).describeImageColorSpace(space); info.Name() != "" || info.Components() != 0 {
			t.Fatalf("malformed calibrated parameters were accepted: %#v", info)
		}
	}
}

func TestColorSpaceRejectsInvalidCalibratedWhitePoint(t *testing.T) {
	d := &Document{}
	for _, whitePoint := range []Array{
		{Number(0), Number(1), Number(1)},
		{Number(1), Number(0.9), Number(1)},
		{Number(1), Number(1), Number(-1)},
	} {
		space := Array{Name("CalGray"), Dict{Name("WhitePoint"): whitePoint}}
		if info := d.describeImageColorSpace(space); info.Name() != "" || info.Components() != 0 {
			t.Fatalf("invalid calibrated WhitePoint was accepted: %#v", info)
		}
	}
}

func TestColorSpaceRejectsInvalidCalibratedParameterValues(t *testing.T) {
	whitePoint := Array{Number(1), Number(1), Number(1)}
	for _, space := range []Object{
		Array{Name("CalGray"), Dict{
			Name("WhitePoint"): whitePoint, Name("BlackPoint"): Array{Number(-1), Number(0), Number(0)},
		}},
		Array{Name("CalRGB"), Dict{
			Name("WhitePoint"): whitePoint, Name("Gamma"): Array{Number(-1), Number(1), Number(1)},
		}},
		Array{Name("Lab"), Dict{
			Name("WhitePoint"): whitePoint, Name("Range"): Array{Number(1), Number(0), Number(0), Number(1)},
		}},
	} {
		if info := (&Document{}).describeImageColorSpace(space); info.Name() != "" || info.Components() != 0 {
			t.Fatalf("invalid calibrated parameter values were accepted: %#v", info)
		}
	}
}

func TestColorSpaceRejectsMalformedSeparationAndPatternBases(t *testing.T) {
	d := &Document{}
	spaces := []Object{
		Array{Name("Separation"), Number(1), Name("DeviceRGB"), Null{}},
		Array{Name("Separation"), Name("Spot"), Name("UnknownSpace"), Null{}},
		Array{Name("DeviceN"), Array{Name("Spot")}, Name("UnknownSpace"), Null{}},
		Array{Name("Pattern"), Name("UnknownSpace")},
	}
	for _, space := range spaces {
		if info := d.describeImageColorSpace(space); info.Name() != "" || info.Components() != 0 {
			t.Fatalf("malformed Separation/Pattern color space was described: %#v", info)
		}
		if components := d.resolvedColorSpaceComponents(space); components != 0 {
			t.Fatalf("malformed Separation/Pattern color space components = %d", components)
		}
	}
}

func TestColorSpaceRejectsNestedPatternBase(t *testing.T) {
	space := Array{Name("Pattern"), Name("Pattern")}
	info := (&Document{}).describeImageColorSpace(space)
	if info.Name() != "" || info.Components() != 0 {
		t.Fatalf("nested Pattern color space was accepted: %#v", info)
	}
	if components := (&Document{}).resolvedColorSpaceComponents(space); components != 0 {
		t.Fatalf("nested Pattern color space has %d components", components)
	}
}

func TestImageColorSpaceSeparationIsSingleComponent(t *testing.T) {
	d := &Document{}
	info := d.describeImageColorSpace(Array{Name("Separation"), Array{Name("SpotA"), Name("SpotB")}, Name("DeviceRGB"), Null{}})
	base, hasBase := info.BaseCopy()
	if info.Components() != 1 || len(info.ColorantsCopy()) != 2 || !hasBase || base.Name() != "DeviceRGB" {
		t.Fatalf("separation color space = %#v", info)
	}
}

func TestImageColorSpaceDeviceNPreservesAlternateBase(t *testing.T) {
	d := &Document{}
	info := d.describeImageColorSpace(Array{
		Name("DeviceN"), Array{Name("Cyan"), Name("Spot")}, Name("DeviceRGB"), Null{},
	})
	base, hasBase := info.BaseCopy()
	if info.Components() != 2 || len(info.ColorantsCopy()) != 2 || !hasBase || base.Name() != "DeviceRGB" {
		t.Fatalf("DeviceN color space = %#v", info)
	}
}

func TestImageColorSpacePreservesIndexedAndDecodeMetadata(t *testing.T) {
	d := &Document{}
	s := newStream(Dict{
		Name("Subtype"): Name("Image"), Name("Width"): Number(1), Name("Height"): Number(1),
		Name("BitsPerComponent"): Number(8),
		Name("ColorSpace"):       Array{Name("Indexed"), Name("DeviceRGB"), Number(1), String([]byte{1, 2, 3, 4, 5, 6})},
		Name("Decode"):           Array{Number(1), Number(0)},
	}, []byte{0})
	im := imageFromStream(d, "", s)
	base, hasBase := im.colorSpaceInfo.BaseCopy()
	if im.colorSpaceInfo.Name() != "Indexed" || !hasBase || base.Name() != "DeviceRGB" || im.colorSpaceInfo.High() != 1 {
		t.Fatalf("color space info = %#v", im.colorSpaceInfo)
	}
	if len(im.colorSpaceInfo.lookup) != 6 || len(im.decode) != 2 || im.decode[0] != 1 || im.decode[1] != 0 {
		t.Fatalf("image metadata = %#v", im)
	}
}

func TestImageColorSpaceRejectsIndexedExtraValues(t *testing.T) {
	info := (&Document{}).describeImageColorSpace(Array{
		Name("Indexed"), Name("DeviceRGB"), Number(1), String([]byte{1, 2, 3}), Number(20),
	})
	if info.Name() != "" || info.Components() != 0 {
		t.Fatalf("malformed Indexed color space was accepted: %#v", info)
	}
}

func TestImageColorSpaceRejectsIndexedHighOutsideByteRange(t *testing.T) {
	for _, high := range []Object{Number(-1), Number(256), Number(1.5)} {
		info := (&Document{}).describeImageColorSpace(Array{
			Name("Indexed"), Name("DeviceRGB"), high, String([]byte{1, 2, 3}),
		})
		if info.Name() != "" || info.Components() != 0 {
			t.Fatalf("invalid Indexed hival %#v was accepted: %#v", high, info)
		}
	}
}

func TestImageColorSpaceRejectsIndexedInvalidBase(t *testing.T) {
	for _, base := range []Object{Name("UnknownSpace"), Number(1)} {
		space := Array{Name("Indexed"), base, Number(1), String([]byte{1, 2, 3})}
		if info := (&Document{}).describeImageColorSpace(space); info.Name() != "" || info.Components() != 0 {
			t.Fatalf("invalid Indexed base %#v was accepted: %#v", base, info)
		}
		if components := (&Document{}).resolvedColorSpaceComponents(space); components != 0 {
			t.Fatalf("invalid Indexed base %#v has %d components", base, components)
		}
	}
}

func TestImageColorSpaceRejectsMalformedDeviceNNames(t *testing.T) {
	d := &Document{}
	for _, names := range []Object{Array{}, Array{String("C1")}, Number(1)} {
		space := Array{Name("DeviceN"), names, Name("DeviceRGB"), Null{}}
		if info := d.describeImageColorSpace(space); info.Name() != "" || info.Components() != 0 {
			t.Fatalf("malformed DeviceN names %#v were accepted: %#v", names, info)
		}
		if components := d.resolvedColorSpaceComponents(space); components != 0 {
			t.Fatalf("malformed DeviceN names %#v have %d components", names, components)
		}
	}
}

func TestImageDecodedCacheRejectsOverflowingDimensions(t *testing.T) {
	if imageDecodedCacheAllowed(ImageObject{
		width:      1 << 62,
		height:     1 << 62,
		bpc:        8,
		components: 1,
	}, 32<<20) {
		t.Fatal("image cache allowed overflowing dimensions")
	}
}

func TestImageColorSpaceIndexedLookupDecodeIsLazy(t *testing.T) {
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	_, _ = writer.Write([]byte{1, 2, 3})
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	lookup := newStream(Dict{Name("Filter"): Name("FlateDecode")}, encoded.Bytes())
	info := (&Document{}).describeImageColorSpace(Array{Name("Indexed"), Name("DeviceRGB"), Number(0), lookup})
	if len(info.lookup) != 0 || len(info.lookupData) == 0 {
		t.Fatalf("indexed color space decoded during construction: %#v", info)
	}
	if got := info.LookupCopy(); !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("lazy color-space lookup = %v", got)
	}
}

func TestImageDecodeCacheDoesNotRetainOversizedResult(t *testing.T) {
	data := bytes.Repeat([]byte{1}, imageCacheLimit+1)
	cache := &imageDecodeCache{limit: imageCacheLimit}
	decoded, err := cache.decode(data, nil, nil)
	if err != nil || len(decoded) != len(data) {
		t.Fatalf("decoded oversized result = len %d, err %v", len(decoded), err)
	}
	if cache.ready || cache.data != nil {
		t.Fatal("oversized decoded result was retained")
	}
}

func TestImageDecodeCacheZeroBudgetDoesNotRetainResult(t *testing.T) {
	cache := &imageDecodeCache{limit: 0}
	decoded, err := cache.decode([]byte("uncached image samples"), nil, nil)
	if err != nil || !bytes.Equal(decoded, []byte("uncached image samples")) {
		t.Fatalf("decoded zero-budget result = %q, err %v", decoded, err)
	}
	if cache.ready || cache.data != nil {
		t.Fatal("zero-budget decoded result was retained")
	}
}

func TestImageDecodeCacheConcurrentConsumersShareResult(t *testing.T) {
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	_, _ = writer.Write([]byte("shared image samples"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	cache := &imageDecodeCache{limit: imageCacheLimit}
	const readers = 32
	results := make([][]byte, readers)
	errs := make([]error, readers)
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = cache.decode(encoded.Bytes(), []string{"FlateDecode"}, nil)
		}(i)
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || !bytes.Equal(results[i], []byte("shared image samples")) {
			t.Fatalf("consumer %d result = %q, err = %v", i, results[i], errs[i])
		}
	}
	if !cache.ready || !bytes.Equal(cache.data, []byte("shared image samples")) {
		t.Fatalf("shared decode cache = ready:%v data:%q", cache.ready, cache.data)
	}
}

func TestLargeIndexedLookupDoesNotRetainDecodeCache(t *testing.T) {
	lookup := newStream(Dict{Name("Filter"): Name("FlateDecode")}, []byte{0x78, 0x9c})
	info := (&Document{}).describeImageColorSpace(Array{
		Name("Indexed"), Name("DeviceRGB"), Number(1 << 30), lookup,
	})
	if info.lookupCache != nil {
		t.Fatal("large indexed lookup retained a decoded cache")
	}
}

func TestImageDecodeAndMaskMetadata(t *testing.T) {
	d := &Document{}
	s := newStream(Dict{
		Name("ImageMask"): Bool(true), Name("Mask"): Ref{Object: 8, Generation: 0}, Name("SMask"): Ref{Object: 9, Generation: 0},
		Name("Decode"): Array{Number(1), Number(0)}, Name("BitsPerComponent"): Number(1),
	}, []byte{0})
	im := imageFromStream(d, "", s)
	if !im.imageMask || !im.hasMask || !im.hasSoftMask || im.DecodeSample(0, 0) != 1 || im.DecodeSample(0, 1) != 0 {
		t.Fatalf("mask/decode metadata = %#v", im)
	}
}

func TestImageIgnoresMalformedDecodeArray(t *testing.T) {
	s := newStream(Dict{
		Name("Width"): Number(1), Name("Height"): Number(1), Name("BPC"): Number(8),
		Name("CS"): Name("G"), Name("Decode"): Array{Number(1), Number(0), String("invalid")},
	}, []byte{0})
	if im := imageFromStream(&Document{}, "", s); im.decode != nil {
		t.Fatalf("malformed image decode array was retained: %#v", im.decode)
	}
}

func TestImageIgnoresNonFiniteDecodeArray(t *testing.T) {
	for _, value := range []Number{Number(math.NaN()), Number(math.Inf(1))} {
		s := newStream(Dict{
			Name("Width"): Number(1), Name("Height"): Number(1), Name("BPC"): Number(8),
			Name("CS"): Name("G"), Name("Decode"): Array{value, Number(0)},
		}, []byte{0})
		if im := imageFromStream(&Document{}, "", s); im.decode != nil {
			t.Fatalf("non-finite image decode value %v was retained: %#v", value, im.decode)
		}
	}
}

func TestImageStoresDecodeParmsForDeferredDecoding(t *testing.T) {
	im := imageFromStream(&Document{}, "", newStream(Dict{
		Name("Filter"):      Name("FlateDecode"),
		Name("DecodeParms"): Dict{Name("Predictor"): Number(12), Name("Columns"): Number(2), Name("Colors"): Number(3)},
	}, []byte{1, 2, 3}))
	if len(im.filterParms) != 1 || im.filterParms[0][Name("Predictor")] != Number(12) {
		t.Fatalf("filter parameters = %#v", im.filterParms)
	}
}

func TestImageFollowsMultiLevelIndirectStreamFilters(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Name("FlateDecode"),
		{Object: 3}: Ref{Object: 4},
		{Object: 4}: Dict{Name("Predictor"): Number(12), Name("Columns"): Number(1)},
	}}
	im := imageFromStream(d, "Im", newStream(Dict{
		Name("Width"):            Number(1),
		Name("Height"):           Number(1),
		Name("ColorSpace"):       Name("DeviceGray"),
		Name("BitsPerComponent"): Number(8),
		Name("Filter"):           Ref{Object: 1},
		Name("DecodeParms"):      Ref{Object: 3},
	}, []byte{7}))
	if len(im.filters) != 1 || im.filters[0] != "FlateDecode" {
		t.Fatalf("multi-level filters = %#v", im.filters)
	}
	if len(im.filterParms) != 1 || im.filterParms[0][Name("Predictor")] != Number(12) {
		t.Fatalf("multi-level filter parameters = %#v", im.filterParms)
	}
}

func TestImageSamplesDecodeFiltersLazily(t *testing.T) {
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	_, _ = writer.Write([]byte{7, 8})
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	im := imageFromStream(&Document{}, "", newStream(Dict{
		Name("Width"): Number(2), Name("Height"): Number(1), Name("BitsPerComponent"): Number(8),
		Name("ColorSpace"): Name("DeviceGray"), Name("Filter"): Name("FlateDecode"),
	}, encoded.Bytes()))
	if len(im.decodedData) != 0 {
		t.Fatalf("image decoded during construction: %v", im.decodedData)
	}
	samples, components := im.Samples()
	if components != 1 || !bytes.Equal(samples, []byte{7, 8}) {
		t.Fatalf("lazy image samples = %v, components = %d", samples, components)
	}
	if im.decodedCache == nil || !bytes.Equal(im.decodedCache.data, []byte{7, 8}) {
		t.Fatalf("image decode cache = %#v", im.decodedCache)
	}
	second, _ := im.Samples()
	if !bytes.Equal(second, samples) {
		t.Fatalf("cached image samples = %v", second)
	}
}

func TestImageDecodedStreamBufferRecoversCorruptLZWPrefix(t *testing.T) {
	image := ImageObject{
		data:    []byte{0x80, 0x10, 0x65, 0x80},
		filters: []string{"LZWDecode"},
	}
	if recovered := image.DecodedStreamBuffer(); string(recovered) != "A" {
		t.Fatalf("recovered image stream prefix = %q, want %q", recovered, "A")
	}
	if _, err := image.DecodedStreamBufferWithError(); err == nil {
		t.Fatal("strict image stream accessor accepted corrupt LZW data")
	}
}

func TestImageDecodedBufferReadsReadyDecodeCache(t *testing.T) {
	im := ImageObject{decodedCache: &imageDecodeCache{ready: true, data: make([]byte, 0)}}
	if data := im.DecodedBuffer(); data == nil {
		t.Fatal("ready empty decode cache was reported as unavailable")
	}
}

func TestImageCacheBudgetControlsDeferredDecodeCaches(t *testing.T) {
	d := &Document{
		cacheOptions:           cacheconfig.Options{ImageBytes: 0},
		cacheOptionsConfigured: true,
	}
	image := imageFromStream(d, "", newStream(Dict{
		Name("Width"): Number(1), Name("Height"): Number(1), Name("BitsPerComponent"): Number(8),
		Name("ColorSpace"): Name("DeviceGray"), Name("Filter"): Name("FlateDecode"),
	}, []byte{0x78, 0x9c, 0x63, 0x60, 0x04, 0x00, 0x00, 0x02, 0x00, 0x01}))
	if image.decodedCache != nil {
		t.Fatal("zero ImageBytes retained a deferred image decode cache")
	}

	lookup := newStream(Dict{Name("Filter"): Name("FlateDecode")}, []byte{0x78, 0x9c, 0x63, 0x60, 0x04, 0x00, 0x00, 0x02, 0x00, 0x01})
	space := d.describeImageColorSpace(Array{Name("Indexed"), Name("DeviceGray"), Number(1), lookup})
	if space.lookupCache != nil {
		t.Fatal("zero ImageBytes retained a deferred Indexed lookup cache")
	}
}

func TestIndexedPaletteDecodeIsLazy(t *testing.T) {
	var encoded bytes.Buffer
	writer := zlib.NewWriter(&encoded)
	_, _ = writer.Write([]byte{10, 20})
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	palette := newStream(Dict{Name("Filter"): Name("FlateDecode")}, encoded.Bytes())
	im := imageFromStream(&Document{}, "", newStream(Dict{
		Name("Width"): Number(2), Name("Height"): Number(1), Name("BitsPerComponent"): Number(8),
		Name("ColorSpace"): Array{Name("Indexed"), Name("DeviceGray"), Number(1), palette},
	}, []byte{0, 1}))
	if len(im.indexedLookup) != 0 || len(im.indexedLookupData) == 0 {
		t.Fatalf("indexed palette decoded during construction: %#v", im)
	}
	samples, components := im.Samples()
	if components != 1 || !bytes.Equal(samples, []byte{10, 20}) {
		t.Fatalf("lazy indexed samples = %v, components = %d", samples, components)
	}
}

func TestIndexedLookupReadsReadyEmptyCache(t *testing.T) {
	im := ImageObject{
		indexedLookupData:    make([]byte, 0),
		indexedLookupFilters: []string{"FlateDecode"},
		indexedLookupCache:   &imageDecodeCache{ready: true, data: make([]byte, 0)},
	}
	if lookup := im.IndexedLookupCopy(); lookup == nil {
		t.Fatal("ready empty indexed lookup cache was reported as unavailable")
	}
}

func TestIndexedLookupPreservesEmptyUnfilteredData(t *testing.T) {
	im := ImageObject{indexedLookupData: make([]byte, 0)}
	if lookup := im.IndexedLookupCopy(); lookup == nil {
		t.Fatal("empty unfiltered indexed lookup data became nil")
	}
}

func TestUnfilteredImageReusesRawSampleStorage(t *testing.T) {
	im := imageFromStream(&Document{}, "", newStream(nil, []byte{1, 2, 3}))
	if len(im.decodedData) != 3 || &im.decodedData[0] != &im.data[0] {
		t.Fatalf("unfiltered image duplicated samples: data=%p decoded=%p", &im.data[0], &im.decodedData[0])
	}
}

func TestImageColorSpaceArrayResolvesName(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 1}: Name("DeviceRGB")}}
	im := imageFromStream(d, "Im1", newStream(Dict{
		Name("Width"):      Number(1),
		Name("Height"):     Number(1),
		Name("ColorSpace"): Array{Ref{Object: 1}},
	}, []byte{1, 2, 3}))
	if im.colorSpace != "DeviceRGB" || im.components != 3 {
		t.Fatalf("color space = %q components=%d", im.colorSpace, im.components)
	}
}

func TestImageColorSpaceRetainsRawSpecification(t *testing.T) {
	spec := Array{Name("Indexed"), Name("DeviceRGB"), Number(1), String([]byte{1, 2, 3, 4, 5, 6})}
	im := imageFromStream(&Document{}, "Im1", newStream(Dict{
		Name("Width"):      Number(1),
		Name("Height"):     Number(1),
		Name("ColorSpace"): spec,
	}, []byte{1, 2, 3}))
	got, ok := im.colorSpaceInfo.SpecCopy().(Array)
	if !ok || len(got) != len(spec) || got[0] != Name("Indexed") {
		t.Fatalf("raw color-space specification = %#v", im.colorSpaceInfo.SpecCopy())
	}
	got[0] = Name("changed")
	if retained := im.colorSpaceInfo.SpecCopy().(Array)[0]; retained != Name("Indexed") {
		t.Fatalf("raw color-space specification was not owned: %#v", retained)
	}
}

func TestImageColorSpaceResolvesResourceAlias(t *testing.T) {
	d := &Document{}
	im := imageFromStream(d, "Im", newStream(Dict{
		Name("Width"): Number(1), Name("Height"): Number(1), Name("ColorSpace"): Name("CS1"),
	}, []byte{7}), Dict{Name("ColorSpace"): Dict{Name("CS1"): Name("DeviceRGB")}})
	if im.colorSpace != "DeviceRGB" || im.components != 3 || im.colorSpaceInfo.Name() != "DeviceRGB" {
		t.Fatalf("resource color-space alias = %#v", im)
	}
}

func TestCachedImageRefreshesResourceColorSpaceAlias(t *testing.T) {
	d := &Document{imageCache: map[Ref]ImageObject{}}
	ref := Ref{Object: 9}
	stream := newStream(Dict{
		Name("Width"): Number(1), Name("Height"): Number(1), Name("ColorSpace"): Name("CS1"),
	}, []byte{7})
	first := d.cachedImage("Im", ref, stream, Dict{Name("ColorSpace"): Dict{Name("CS1"): Name("DeviceGray")}})
	second := d.cachedImage("Im", ref, stream, Dict{Name("ColorSpace"): Dict{Name("CS1"): Name("DeviceRGB")}})
	if first.colorSpace != "DeviceGray" || second.colorSpace != "DeviceRGB" || second.components != 3 {
		t.Fatalf("cached resource color-space aliases = %#v, %#v", first, second)
	}
}

func TestCachedImageHasMemoryBudget(t *testing.T) {
	d := &Document{imageCache: map[Ref]ImageObject{}}
	ref := Ref{Object: 99}
	image := d.cachedImage("Large", ref, newStream(nil, bytes.Repeat([]byte{1}, imageCacheLimit+1)))
	if len(image.data) != imageCacheLimit+1 {
		t.Fatalf("large image data length = %d", len(image.data))
	}
	if len(d.imageCache) != 0 || d.imageCacheBytes != 0 {
		t.Fatalf("large image was cached: entries=%d bytes=%d", len(d.imageCache), d.imageCacheBytes)
	}
}

func TestDecodeImageReturnsUnifiedSamples(t *testing.T) {
	d := &Document{objects: map[Ref]Object{}}
	image, err := d.DecodeImage(ImageObject{width: 2, height: 1, bpc: 8, components: 1, colorSpace: "DeviceGray", data: []byte{3, 4}})
	if err != nil || image.Width() != 2 || image.Height() != 1 || image.Components() != 1 || image.Bits() != 8 || string(image.PixCopy()) != string([]byte{3, 4}) {
		t.Fatalf("decoded image=%#v err=%v", image, err)
	}
}

func TestDecodeImagePropagatesIndexedPaletteDecodeError(t *testing.T) {
	d := &Document{}
	im := ImageObject{
		width: 1, height: 1, bpc: 8, components: 1,
		colorSpace: "Indexed", indexedComponents: 1,
		data: []byte{0}, indexedLookupData: []byte{1},
		indexedLookupFilters: []string{"UnsupportedFilter"},
	}
	if _, err := d.DecodeImage(im); err == nil {
		t.Fatal("DecodeImage accepted an invalid indexed palette filter")
	}
}

func TestImageSamplesWithErrorPropagatesFilterError(t *testing.T) {
	im := ImageObject{width: 1, height: 1, bpc: 8, components: 1, filters: []string{"UnsupportedFilter"}, data: []byte{1}}
	if _, _, err := im.SamplesWithError(); err == nil {
		t.Fatal("SamplesWithError accepted an unsupported image filter")
	}
}

func TestImageColorSpaceLookupWithErrorPropagatesFilterError(t *testing.T) {
	info := ImageColorSpace{
		lookupData: []byte{1}, lookupFilters: []string{"UnsupportedFilter"},
	}
	if _, err := info.LookupWithError(); err == nil {
		t.Fatal("LookupWithError accepted an unsupported lookup filter")
	}
}

func TestImageSamplesWithErrorRejectsInvalidShape(t *testing.T) {
	for name, image := range map[string]ImageObject{
		"negative width":      {width: -1, height: 1, bpc: 8, components: 1, data: []byte{1}},
		"negative height":     {width: 1, height: -1, bpc: 8, components: 1, data: []byte{1}},
		"negative components": {width: 1, height: 1, bpc: 8, components: -1, data: []byte{1}},
		"invalid bit depth":   {width: 1, height: 1, bpc: 3, components: 1, data: []byte{1}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := image.SamplesWithError(); err == nil {
				t.Fatal("invalid image shape was accepted")
			}
		})
	}
}

func TestImageFromStreamReadsInlineImageAbbreviations(t *testing.T) {
	s := newStream(Dict{
		Name("W"):   Number(2),
		Name("H"):   Number(3),
		Name("BPC"): Number(8),
		Name("CS"):  Name("G"),
		Name("IM"):  Bool(true),
	}, []byte{1, 2, 3})
	im := imageFromStream(&Document{}, "", s)
	if im.width != 2 || im.height != 3 || im.bpc != 8 || im.colorSpace != "DeviceGray" || !im.imageMask || im.components != 1 {
		t.Fatalf("inline image = %#v", im)
	}
}

func TestPageImagesCarriesGraphicsState(t *testing.T) {
	content := newStream(nil, []byte("q 2 w 0.1 0.2 0.3 RG 4 0 0 5 10 20 cm BI /W 1 /H 1 /BPC 8 /CS /G ID x EI Q"))
	d := &Document{objects: map[Ref]Object{}}
	got, err := d.PageImages(Page{dict: Dict{Name("Contents"): content}})
	if err != nil || len(got) != 1 {
		t.Fatalf("images=%#v err=%v", got, err)
	}
	if got[0].gstate.lineWidth != 2 || got[0].gstate.strokeColor.Space() != "DeviceRGB" || got[0].gstate.ctm != (geometry.Matrix{4, 0, 0, 5, 10, 20}) {
		t.Fatalf("image graphics state = %#v", got[0].gstate)
	}
}

func TestPageImagesIgnoresNonFiniteMatrix(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("cm", []Object{Number(math.Inf(1)), Number(0), Number(0), Number(1), Number(0), Number(0)}, 0),
		newContentOpBorrowed("BI", []Object{newStream(Dict{Name("W"): Number(1), Name("H"): Number(1), Name("BPC"): Number(8), Name("CS"): Name("G")}, nil)}, 0),
	}
	var got ImageObject
	count := 0
	index := 0
	interpretImagesNext(&Document{}, Ref{}, func() (ContentOp, bool) {
		if index == len(ops) {
			return ContentOp{}, false
		}
		op := ops[index]
		index++
		return op, true
	}, nil, func(image ImageObject) bool {
		got = image
		count++
		return true
	}, func(error) {})
	if count != 1 || got.bbox != [4]float64{0, 0, 1, 1} {
		t.Fatalf("non-finite image matrix changed image bbox: count=%d bbox=%v", count, got.bbox)
	}
}

func TestPageImagesRejectsOverflowingTransformedBBox(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("cm", []Object{Number(math.MaxFloat64), Number(0), Number(0), Number(1), Number(math.MaxFloat64), Number(0)}, 0),
		newContentOpBorrowed("BI", []Object{newStream(Dict{Name("W"): Number(1), Name("H"): Number(1), Name("BPC"): Number(8), Name("CS"): Name("G")}, nil)}, 0),
	}
	count := 0
	index := 0
	interpretImagesNext(&Document{}, Ref{}, func() (ContentOp, bool) {
		if index == len(ops) {
			return ContentOp{}, false
		}
		op := ops[index]
		index++
		return op, true
	}, nil, func(ImageObject) bool {
		count++
		return true
	}, func(error) {})
	if count != 0 {
		t.Fatalf("overflowing image bbox was emitted: count=%d", count)
	}
}

func TestPageImagesIgnoresOverflowingCTMProduct(t *testing.T) {
	ops := []ContentOp{
		newContentOpBorrowed("cm", []Object{Number(math.MaxFloat64), Number(0), Number(0), Number(1), Number(0), Number(0)}, 0),
		newContentOpBorrowed("cm", []Object{Number(math.MaxFloat64), Number(0), Number(0), Number(1), Number(0), Number(0)}, 0),
		newContentOpBorrowed("BI", []Object{newStream(Dict{Name("W"): Number(1), Name("H"): Number(1), Name("BPC"): Number(8), Name("CS"): Name("G")}, nil)}, 0),
	}
	var got ImageObject
	count := 0
	index := 0
	interpretImagesNext(&Document{}, Ref{}, func() (ContentOp, bool) {
		if index == len(ops) {
			return ContentOp{}, false
		}
		op := ops[index]
		index++
		return op, true
	}, nil, func(image ImageObject) bool {
		got = image
		count++
		return true
	}, func(error) {})
	if count != 1 || got.gstate.ctm[0] != math.MaxFloat64 {
		t.Fatalf("overflowing image CTM changed output: count=%d image=%#v", count, got)
	}
}

func TestPageImagesCarriesMarkedContentAssociation(t *testing.T) {
	content := newStream(nil, []byte("/Figure << /MCID 4 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G ID x EI EMC"))
	d := &Document{objects: map[Ref]Object{}}
	got, err := d.PageImages(Page{dict: Dict{Name("Contents"): content}})
	if err != nil || len(got) != 1 {
		t.Fatalf("images=%#v err=%v", got, err)
	}
	if !got[0].hasMCID || got[0].mcid != 4 || got[0].markedTag != "Figure" {
		t.Fatalf("image marked-content association = %#v", got[0])
	}
}
