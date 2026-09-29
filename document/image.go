package document

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"iter"
	"math"
	"sort"
	"sync"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/imagedata"
)

type imageDecodeCache struct {
	mu     sync.Mutex
	data   []byte
	err    error
	ready  bool
	flight *imageDecodeFlight
	limit  int
}

type imageDecodeFlight struct {
	done chan struct{}
	data []byte
	err  error
}

const imageCacheLimit = 32 << 20

type ImageObject struct {
	name string
	// Inline reports whether this image came from a BI/EI inline image rather
	// than a named Image XObject resource.
	inline bool
	// Offset is the byte offset of the BI or Do operator in its decoded
	// content stream.
	offset                   int
	page                     Ref
	hasPage                  bool
	width, height, bpc       int
	colorSpace               string
	colorSpaceInfo           ImageColorSpace
	decode                   []float64
	components               int
	indexedComponents        int
	indexedHigh              int
	indexedLookup            []byte
	indexedLookupData        []byte
	indexedLookupFilters     []string
	indexedLookupFilterParms []Dict
	indexedLookupCache       *imageDecodeCache
	imageMask                bool
	mask                     Object
	hasMask                  bool
	softMask                 Object
	hasSoftMask              bool
	bbox                     [4]float64
	filters                  []string
	rawFilters               []string
	hasRawFilter             bool
	filterParms              []Dict
	data                     []byte // encoded stream bytes
	decodedData              []byte // decoded sample bytes
	dict                     Dict
	parentKey                int
	hasParentKey             bool
	parentKeyContext         bool
	gstate                   graphicsState
	markedTag                string
	markedProperties         Dict                   `json:"-"`
	markedStack              []markedContentContext `json:"-"`
	actualText               string
	mcid                     int
	hasMCID                  bool
	decodedCache             *imageDecodeCache
}

func (im ImageObject) Name() string           { return im.name }
func (im ImageObject) Inline() bool           { return im.inline }
func (im ImageObject) Offset() int            { return im.offset }
func (im ImageObject) Page() Ref              { return im.page }
func (im ImageObject) HasPage() bool          { return im.hasPage }
func (im ImageObject) Width() int             { return im.width }
func (im ImageObject) Height() int            { return im.height }
func (im ImageObject) BPC() int               { return im.bpc }
func (im ImageObject) ColorSpace() string     { return im.colorSpace }
func (im ImageObject) Components() int        { return im.components }
func (im ImageObject) IndexedComponents() int { return im.indexedComponents }
func (im ImageObject) IndexedHigh() int       { return im.indexedHigh }
func (im ImageObject) ImageMask() bool        { return im.imageMask }
func (im ImageObject) HasMask() bool          { return im.hasMask }
func (im ImageObject) HasSoftMask() bool      { return im.hasSoftMask }
func (im ImageObject) BBox() [4]float64       { return im.bbox }
func (im ImageObject) BBoxX0() float64        { return bboxX0(im.bbox) }
func (im ImageObject) BBoxY0() float64        { return bboxY0(im.bbox) }
func (im ImageObject) BBoxX1() float64        { return bboxX1(im.bbox) }
func (im ImageObject) BBoxY1() float64        { return bboxY1(im.bbox) }
func (im ImageObject) BBoxWidth() float64     { return bboxWidth(im.bbox) }
func (im ImageObject) BBoxHeight() float64    { return bboxHeight(im.bbox) }
func (im ImageObject) IsEmpty() bool          { return bboxIsEmpty(im.bbox) }
func (im ImageObject) IsHoverlap(other BBoxProvider) bool {
	return bboxIsHoverlap(im.bbox, other)
}
func (im ImageObject) HDistance(other BBoxProvider) float64 {
	return bboxHDistance(im.bbox, other)
}
func (im ImageObject) Hoverlap(other BBoxProvider) float64 {
	return bboxHoverlap(im.bbox, other)
}
func (im ImageObject) IsVOverlap(other BBoxProvider) bool {
	return bboxIsVOverlap(im.bbox, other)
}
func (im ImageObject) VDistance(other BBoxProvider) float64 {
	return bboxVDistance(im.bbox, other)
}
func (im ImageObject) VOverlap(other BBoxProvider) float64 {
	return bboxVOverlap(im.bbox, other)
}
func (im ImageObject) ParentKey() int        { return im.parentKey }
func (im ImageObject) HasParentKey() bool    { return im.hasParentKey }
func (im ImageObject) GState() GraphicsState { return im.gstate.publicValue() }
func (im ImageObject) MarkedTag() string     { return im.markedTag }
func (im ImageObject) ActualText() string    { return im.actualText }
func (im ImageObject) MCID() int             { return im.mcid }
func (im ImageObject) HasMCID() bool         { return im.hasMCID }

// Len returns the number of children yielded by an image object. Images are
// leaf content objects in Playa's interpreter model.
func (im ImageObject) Len() int { return 0 }

// DecodedBuffer returns an independent copy of decoded sample bytes when
// they have already been materialized.
func (im ImageObject) DecodedBuffer() []byte {
	if im.decodedData != nil {
		return cloneObjectBytes(im.decodedData)
	}
	if cache := im.decodedCache; cache != nil {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		if cache.ready {
			return cloneObjectBytes(cache.data)
		}
	}
	return nil
}

func (im ImageObject) MarshalJSON() ([]byte, error) {
	indexedLookup := im.indexedLookupBytes()
	return json.Marshal(&struct {
		Name              string                 `json:"Name"`
		Inline            bool                   `json:"Inline"`
		Offset            int                    `json:"Offset"`
		Page              Ref                    `json:"Page"`
		HasPage           bool                   `json:"HasPage"`
		Width             int                    `json:"Width"`
		Height            int                    `json:"Height"`
		BPC               int                    `json:"BPC"`
		ColorSpace        string                 `json:"ColorSpace"`
		Components        int                    `json:"Components"`
		IndexedComponents int                    `json:"IndexedComponents"`
		IndexedHigh       int                    `json:"IndexedHigh"`
		ImageMask         bool                   `json:"ImageMask"`
		HasMask           bool                   `json:"HasMask"`
		HasSoftMask       bool                   `json:"HasSoftMask"`
		BBox              [4]float64             `json:"BBox"`
		ParentKey         int                    `json:"ParentKey"`
		HasParentKey      bool                   `json:"HasParentKey"`
		GState            GraphicsState          `json:"gstate"`
		MarkedTag         string                 `json:"marked_tag,omitempty"`
		ActualText        string                 `json:"actual_text,omitempty"`
		MCID              int                    `json:"mcid,omitempty"`
		HasMCID           bool                   `json:"has_mcid"`
		Data              []byte                 `json:"Data"`
		DecodedData       []byte                 `json:"DecodedData"`
		Decode            []float64              `json:"Decode"`
		IndexedLookup     []byte                 `json:"IndexedLookup"`
		Filters           []string               `json:"Filters"`
		FilterParms       []Dict                 `json:"FilterParms"`
		Dict              Dict                   `json:"Dict"`
		MarkedProperties  Dict                   `json:"marked_properties,omitempty"`
		MarkedStack       []MarkedContentContext `json:"marked_stack,omitempty"`
		Mask              Object                 `json:"Mask"`
		SoftMask          Object                 `json:"SoftMask"`
		ColorSpaceInfo    ImageColorSpace        `json:"ColorSpaceInfo"`
	}{
		Name: im.name, Inline: im.inline, Offset: im.offset, Page: im.page, HasPage: im.hasPage,
		Width: im.width, Height: im.height, BPC: im.bpc, ColorSpace: im.colorSpace,
		Components: im.components, IndexedComponents: im.indexedComponents, IndexedHigh: im.indexedHigh,
		ImageMask: im.imageMask, HasMask: im.hasMask, HasSoftMask: im.hasSoftMask, BBox: im.bbox,
		ParentKey: im.parentKey, HasParentKey: im.hasParentKey, GState: im.gstate.publicValue(),
		MarkedTag: im.markedTag, ActualText: im.actualText, MCID: im.mcid, HasMCID: im.hasMCID,
		Data: im.data, DecodedData: im.decodedData, Decode: im.decode, IndexedLookup: indexedLookup,
		Filters: im.filters, FilterParms: im.filterParms, Dict: im.dict,
		MarkedProperties: im.markedProperties, MarkedStack: markedStackCopy(im.markedStack), Mask: im.mask,
		SoftMask: im.softMask, ColorSpaceInfo: im.colorSpaceInfo,
	})
}

// DecodeCopy returns the image decode interval.
func (im ImageObject) DecodeCopy() []float64 { return cloneGraphicsFloats(im.decode) }

// IndexedLookupCopy returns an independent indexed-color lookup table.
func (im ImageObject) IndexedLookupCopy() []byte {
	return cloneObjectBytes(im.indexedLookupBytes())
}

// IndexedLookupWithError returns an independent indexed-color lookup table
// and preserves deferred palette filter errors.
func (im ImageObject) IndexedLookupWithError() ([]byte, error) {
	data, err := im.indexedLookupDecoded()
	return cloneObjectBytes(data), err
}

// FiltersCopy returns the image filter names.
func (im ImageObject) FiltersCopy() []string { return cloneImageStrings(im.filters) }

// RawFiltersCopy returns filter names only when the source stream used the
// full /Filter key. The result preserves Playa's raw dictionary projection;
// PDF's /F abbreviation remains absent from it.
func (im ImageObject) RawFiltersCopy() []string {
	if !im.hasRawFilter {
		return nil
	}
	if im.rawFilters == nil {
		return cloneImageStrings(im.filters)
	}
	return cloneImageStrings(im.rawFilters)
}

// FilterParamsCopy returns independent image filter parameters.
func (im ImageObject) FilterParamsCopy() []Dict {
	if im.filterParms == nil {
		return nil
	}
	out := make([]Dict, len(im.filterParms))
	for i, parms := range im.filterParms {
		out[i] = cloneDict(parms)
	}
	return out
}

// MarkedPropertiesCopy returns an independent copy of image marked-content
// properties.
func (im ImageObject) MarkedPropertiesCopy() Dict { return cloneDict(im.markedProperties) }

// MarkedStackCopy returns an independent copy of the image marked-content
// stack.
func (im ImageObject) MarkedStackCopy() []MarkedContentContext {
	return markedStackCopy(im.markedStack)
}

// MaskCopy returns an independent image mask value.
func (im ImageObject) MaskCopy() (Object, bool) {
	return cloneGraphicsObject(im.mask), im.hasMask
}

// SoftMaskCopy returns an independent soft-mask value.
func (im ImageObject) SoftMaskCopy() (Object, bool) {
	return cloneGraphicsObject(im.softMask), im.hasSoftMask
}

// ColorSpaceInfoCopy returns an independent image color-space description.
func (im ImageObject) ColorSpaceInfoCopy() ImageColorSpace {
	return cloneImageColorSpace(im.colorSpaceInfo)
}

// ColorSpaceInfoCopyWithError returns an independent color-space description
// and reports deferred lookup-filter errors.
func (im ImageObject) ColorSpaceInfoCopyWithError() (ImageColorSpace, error) {
	return im.colorSpaceInfo.FinalizeWithError()
}

func cloneImageColorSpace(info ImageColorSpace) ImageColorSpace {
	info.lookupCache = nil
	clone := info
	clone.data = info.data.Finalize()
	clone.lookup = cloneObjectBytes(info.lookup)
	clone.lookupData = cloneObjectBytes(info.lookupData)
	clone.lookupFilters = cloneImageStrings(info.lookupFilters)
	clone.lookupFilterParms = cloneFilterParms(info.lookupFilterParms)
	return clone
}

func cloneImageObject(image ImageObject) ImageObject {
	// Finalized image views own their encoded data and must not retain the
	// source object's once-backed lazy decode caches.
	image.indexedLookupCache = nil
	image.decodedCache = nil
	image.colorSpaceInfo = cloneImageColorSpace(image.colorSpaceInfo)
	image.decode = cloneGraphicsFloats(image.decode)
	image.indexedLookup = cloneObjectBytes(image.indexedLookup)
	image.indexedLookupData = cloneObjectBytes(image.indexedLookupData)
	image.indexedLookupFilters = cloneImageStrings(image.indexedLookupFilters)
	if image.indexedLookupFilterParms != nil {
		image.indexedLookupFilterParms = make([]Dict, len(image.indexedLookupFilterParms))
		for i, parms := range image.indexedLookupFilterParms {
			if parms != nil {
				image.indexedLookupFilterParms[i] = cloneGraphicsObject(parms).(Dict)
			}
		}
	}
	image.filters = cloneImageStrings(image.filters)
	image.rawFilters = cloneImageStrings(image.rawFilters)
	if image.filterParms != nil {
		image.filterParms = make([]Dict, len(image.filterParms))
		for i, parms := range image.filterParms {
			if parms != nil {
				image.filterParms[i] = cloneGraphicsObject(parms).(Dict)
			}
		}
	}
	image.data = cloneObjectBytes(image.data)
	image.decodedData = cloneObjectBytes(image.decodedData)
	image.mask = cloneGraphicsObject(image.mask)
	image.softMask = cloneGraphicsObject(image.softMask)
	if image.dict != nil {
		image.dict = cloneGraphicsObject(image.dict).(Dict)
	}
	if image.markedProperties != nil {
		image.markedProperties = cloneGraphicsObject(image.markedProperties).(Dict)
	}
	image.markedStack = cloneMarkedStack(image.markedStack)
	image.gstate = image.gstate.Clone()
	return image
}

// Parent resolves the structure element associated with this image.
func (im ImageObject) Parent(d *Document) *StructElement {
	parent, _ := im.ParentWithError(d)
	return parent
}

// ParentWithError resolves this image's ParentTree element and reports
// malformed ParentTree data.
func (im ImageObject) ParentWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if im.hasParentKey && !im.parentKeyContext {
		return d.parentTreeElementWithError(im.parentKey)
	}
	for index := len(im.markedStack) - 1; index >= 0; index-- {
		if im.markedStack[index].HasMCID() {
			return d.contentParentWithContext(im.page, im.markedStack[index].MCID(), true, im.parentKey, im.hasParentKey)
		}
	}
	return d.contentParentWithContext(im.page, im.mcid, im.hasMCID, im.parentKey, im.hasParentKey)
}

func imageStructParentKey(d *Document, stream Stream) (int, bool) {
	raw, present := stream.DictBorrowed()[Name("StructParent")]
	if !present {
		return 0, false
	}
	resolved, ok := d.resolveIndirectChain(raw)
	if !ok {
		return 0, false
	}
	key, ok := IntValue(resolved)
	return key, ok && key >= 0
}

// PageObject resolves the page associated with a page-level image.
func (im ImageObject) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !im.hasPage {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(im.page)
}

// Buffer returns a defensive copy of the encoded image stream bytes.
func (im ImageObject) Buffer() []byte {
	return cloneObjectBytes(im.data)
}

// DecodedStreamBuffer returns a defensive copy of the image stream after its
// PDF filter chain. It is the explicit counterpart to Playa's buffer property;
// DecodedBuffer remains reserved for decoded image samples.
func (im ImageObject) DecodedStreamBuffer() []byte {
	data, _ := decodeFiltersLenientLimited(im.data, im.filters, im.filterParms, decodedFilterExpansionLimit)
	return cloneObjectBytes(data)
}

// DecodedStreamBufferWithError decodes the encoded image stream without
// retaining the result in the image object.
func (im ImageObject) DecodedStreamBufferWithError() ([]byte, error) {
	decoded, err := decodeFiltersLimited(im.data, im.filters, im.filterParms, decodedFilterExpansionLimit)
	if err != nil {
		return nil, err
	}
	return cloneObjectBytes(decoded), nil
}

// DecodedStreamBufferDigest returns the length and SHA-256 digest of the
// filtered image stream without returning another byte-slice copy.
func (im ImageObject) DecodedStreamBufferDigest() (int, string) {
	length, digest, _ := im.DecodedStreamBufferDigestWithError()
	return length, digest
}

// DecodedStreamBufferDigestWithError is the bounded, allocation-conscious
// counterpart to DecodedStreamBufferWithError for callers that only need to
// compare or record the filtered stream without retaining its bytes.
func (im ImageObject) DecodedStreamBufferDigestWithError() (int, string, error) {
	decoded, err := decodeFiltersLimited(im.data, im.filters, im.filterParms, decodedFilterExpansionLimit)
	if err != nil {
		return 0, "", err
	}
	digest := sha256.Sum256(decoded)
	return len(decoded), hex.EncodeToString(digest[:]), nil
}

// Get looks up an image stream dictionary entry without decoding the image.
func (im ImageObject) Get(key Name) (Object, bool) {
	value, ok := im.dict[key]
	return cloneGraphicsObject(value), ok
}

// Has reports whether an image stream dictionary entry is present.
func (im ImageObject) Has(key Name) bool {
	_, ok := im.dict[key]
	return ok
}

// DecodedImage is the dependency-free image-data value returned by DecodeImage.
type DecodedImage = imagedata.DecodedImage

func (d *Document) DecodeImage(im ImageObject) (DecodedImage, error) {
	if len(im.decodedData) == 0 && len(im.data) == 0 {
		return DecodedImage{}, fmt.Errorf("playa: image has no sample data")
	}
	if len(im.decodedData) == 0 {
		decoded, err := im.decodedSamples()
		if err != nil {
			return DecodedImage{}, err
		}
		im.decodedData = decoded
	}
	pix, components, err := im.SamplesWithError()
	if err != nil {
		return DecodedImage{}, err
	}
	return imagedata.BorrowedDecodedImage(im.width, im.height, components, im.bpc, pix, im.colorSpace), nil
}

// Images returns the page's direct image XObjects.
//
// Errors are returned instead of being silently converted to an empty result;
// callers that need lazy traversal should use ImagesSeq.
func (d *Document) Images(p Page) ([]ImageObject, error) {
	var out []ImageObject
	for image, err := range d.ImagesSeq(p) {
		if err != nil {
			return nil, err
		}
		out = append(out, image)
	}
	return out, nil
}

// ImagesSeq lazily walks image XObjects declared directly by the page.
func (d *Document) ImagesSeq(p Page) iter.Seq2[ImageObject, error] {
	return func(yield func(ImageObject, error) bool) {
		res := Dict{}
		cachedResources, cached := Dict(nil), false
		if p.hasPageCacheKey() {
			cachedResources, cached = d.cachedPageResource(p.ref)
		}
		if cached {
			res = cachedResources
		} else if raw, present := p.dict[Name("Resources")]; present {
			resolved, resolvedOK := d.resolveIndirectChain(raw)
			if !resolvedOK {
				yield(ImageObject{}, fmt.Errorf("playa: Resources could not be resolved"))
				return
			}
			var ok bool
			res, ok = resolved.(Dict)
			if !ok {
				yield(ImageObject{}, fmt.Errorf("playa: Resources is not a dictionary"))
				return
			}
		}
		xos := Dict{}
		if raw, present := res[Name("XObject")]; present {
			resolved, resolvedOK := d.resolveIndirectChain(raw)
			if !resolvedOK {
				yield(ImageObject{}, fmt.Errorf("playa: XObject resources could not be resolved"))
				return
			}
			var ok bool
			xos, ok = resolved.(Dict)
			if !ok {
				yield(ImageObject{}, fmt.Errorf("playa: XObject resources are not a dictionary"))
				return
			}
		}
		names := make([]string, 0, len(xos))
		for name := range xos {
			names = append(names, string(name))
		}
		sort.Strings(names)
		for _, rawName := range names {
			name := Name(rawName)
			ref := xos[name]
			resolvedValue, resolved := d.resolveIndirectChain(ref)
			if ref != nil && !resolved {
				yield(ImageObject{}, fmt.Errorf("playa: XObject %q could not be resolved", rawName))
				return
			}
			s, ok := resolvedValue.(Stream)
			if !ok {
				yield(ImageObject{}, fmt.Errorf("playa: XObject %q is not a stream", rawName))
				return
			}
			sDict := s.DictBorrowed()
			typValue, subtypeResolved := d.resolveIndirectChain(sDict[Name("Subtype")])
			if sDict[Name("Subtype")] != nil && !subtypeResolved {
				yield(ImageObject{}, fmt.Errorf("playa: XObject %q subtype could not be resolved", rawName))
				return
			}
			typ, _ := typValue.(Name)
			if typ != Name("Image") {
				continue
			}
			if _, ok := d.resolveIndirectChain(imageDictValue(sDict, "Width", "W")); imageDictValue(sDict, "Width", "W") != nil && !ok {
				yield(ImageObject{}, fmt.Errorf("playa: XObject %q width could not be resolved", rawName))
				return
			}
			if _, ok := d.resolveIndirectChain(imageDictValue(sDict, "Height", "H")); imageDictValue(sDict, "Height", "H") != nil && !ok {
				yield(ImageObject{}, fmt.Errorf("playa: XObject %q height could not be resolved", rawName))
				return
			}
			if raw := imageDictValue(sDict, "BitsPerComponent", "BPC"); raw != nil {
				if _, ok := d.resolveIndirectChain(raw); !ok {
					yield(ImageObject{}, fmt.Errorf("playa: XObject %q bits-per-component could not be resolved", rawName))
					return
				}
			}
			if raw := imageDictValue(sDict, "ColorSpace", "CS"); raw != nil {
				resolvedValue, ok := d.resolveIndirectChain(raw)
				if !ok {
					yield(ImageObject{}, fmt.Errorf("playa: XObject %q color space could not be resolved", rawName))
					return
				}
				if alias, isAlias := resolvedValue.(Name); isAlias && len(res) > 0 {
					if resourceValue, present := res[Name("ColorSpace")]; present {
						resourceSpaces, resourceOK := d.resolveIndirectChain(resourceValue)
						if !resourceOK {
							yield(ImageObject{}, fmt.Errorf("playa: image color-space resources could not be resolved"))
							return
						}
						if spaces, isDict := resourceSpaces.(Dict); isDict {
							if selected, present := spaces[alias]; present {
								if _, selectedOK := d.resolveIndirectChain(selected); !selectedOK {
									yield(ImageObject{}, fmt.Errorf("playa: XObject %q color space alias could not be resolved", rawName))
									return
								}
							}
						}
					}
				}
			}
			if raw := imageDictValue(sDict, "Decode", "D"); raw != nil {
				decodeValue, ok := d.resolveIndirectChain(raw)
				if !ok {
					yield(ImageObject{}, fmt.Errorf("playa: XObject %q decode array could not be resolved", rawName))
					return
				}
				if decode, isArray := decodeValue.(Array); isArray {
					for _, item := range decode {
						if _, itemOK := d.resolveIndirectChain(item); !itemOK {
							yield(ImageObject{}, fmt.Errorf("playa: XObject %q decode value could not be resolved", rawName))
							return
						}
					}
				}
			}
			im := d.cachedImage(string(name), ref, s, res)
			im.page, im.hasPage = p.ref, p.ref != (Ref{})
			im.parentKey, im.hasParentKey = imageStructParentKey(d, s)
			if !yield(im, nil) {
				return
			}
		}
	}
}

func (d *Document) cachedImage(name string, value Object, stream Stream, resourceArgs ...Dict) ImageObject {
	withResourceMetadata := func(image ImageObject) ImageObject {
		image.name = name
		if len(resourceArgs) == 0 {
			return image
		}
		metadata := imageFromStream(d, name, stream, resourceArgs...)
		image.colorSpace = metadata.colorSpace
		image.colorSpaceInfo = metadata.colorSpaceInfo
		image.components = metadata.components
		image.indexedComponents = metadata.indexedComponents
		image.indexedHigh = metadata.indexedHigh
		image.indexedLookup = metadata.indexedLookup
		image.indexedLookupData = metadata.indexedLookupData
		image.indexedLookupFilters = metadata.indexedLookupFilters
		image.indexedLookupFilterParms = metadata.indexedLookupFilterParms
		image.indexedLookupCache = metadata.indexedLookupCache
		return image
	}
	ref, ok := d.finalIndirectRef(value)
	if ok {
		image, build, owner := d.acquireImageBuild(ref)
		if !owner {
			if build != nil {
				<-build.done
				image = build.image
			}
			return withResourceMetadata(image)
		}
		image = imageFromStream(d, name, stream, resourceArgs...)
		d.finishImageBuild(ref, image)
		return image
	}
	return imageFromStream(d, name, stream, resourceArgs...)
}

// PageImages returns both direct image XObjects and inline images in content
// order. Images() remains available for callers that only need resource-level
// XObjects.
func (d *Document) PageImages(p Page) ([]ImageObject, error) {
	var out []ImageObject
	for image, err := range d.PageImagesSeq(p) {
		if err != nil {
			return nil, err
		}
		out = append(out, image)
	}
	return out, nil
}

// PageImagesSeq is the lazy document-level image traversal used by Page.Images.
func (d *Document) PageImagesSeq(p Page) iter.Seq2[ImageObject, error] {
	return p.images(d, true, contentconfig.Options{Filter: FilterImage})
}

func interpretImagesNext(d *Document, page Ref, next func() (ContentOp, bool), restrictOps map[string]struct{}, yield func(ImageObject) bool, reportError func(error)) {
	ctm := identity()
	gstate := newGraphicsState()
	marked := []markedContentFrame{}
	markedForms := [][]markedContentFrame{}
	type saved struct {
		ctm    geometry.Matrix
		gstate graphicsState
	}
	stack := []saved{}
	stopped := false
	emitCurrent := true
	for !stopped {
		op, ok := next()
		if !ok {
			return
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
			continue
		}
		applyMarkedContent(&marked, op)
		if op.operatorValue() == "BI" || op.operatorValue() == "Do" {
			emitCurrent = len(restrictOps) == 0
			if !emitCurrent {
				_, emitCurrent = restrictOps[op.operatorValue()]
			}
		}
		if op.operatorValue() != "q" && op.operatorValue() != "Q" && op.operatorValue() != "cm" {
			applyGraphicsState(&gstate, op)
			applyExternalGraphicsState(d, &gstate, op)
			applyResourceColorSpace(d, &gstate, op)
		}
		switch op.operatorValue() {
		case "q":
			stack = append(stack, saved{ctm: ctm, gstate: gstate.Clone()})
		case "Q":
			if len(stack) > 0 {
				state := stack[len(stack)-1]
				ctm, gstate = state.ctm, state.gstate
				stack = stack[:len(stack)-1]
			}
		case "cm":
			if len(op.operandsValue()) >= 6 {
				var matrix geometry.Matrix
				valid := true
				for i := range matrix {
					matrix[i], valid = finiteNumberValue(op.operandsValue()[i])
					if !valid {
						break
					}
				}
				if valid {
					if product, ok := ctm.MulFinite(matrix); ok {
						ctm = product
						gstate.ctm = ctm
					}
				}
			}
		case "BI":
			if !emitCurrent {
				continue
			}
			if len(op.operandsValue()) == 0 {
				continue
			}
			stream, ok := op.operandsValue()[0].(Stream)
			if !ok {
				continue
			}
			image := imageFromInlineStream(d, stream, op.resources)
			image.inline = true
			image.offset = op.offsetValue()
			image.page, image.hasPage = page, page != (Ref{})
			image.parentKey, image.hasParentKey = op.parentKey, op.hasParentKey
			image.parentKeyContext = image.hasParentKey
			var valid bool
			image.bbox, valid = unitBBox(ctm)
			if !valid {
				continue
			}
			image.gstate = gstate.Clone()
			m := currentMarkedContent(marked)
			image.markedTag, image.markedProperties, image.markedStack, image.actualText, image.mcid, image.hasMCID = m.Tag, m.Properties, markedContextStack(marked), m.ActualText, m.MCID, m.HasMCID
			stopped = !yield(image)
		case "Do":
			if !emitCurrent {
				continue
			}
			if len(op.operandsValue()) == 0 || op.resources == nil {
				continue
			}
			name, ok := op.operandsValue()[0].(Name)
			if !ok {
				continue
			}
			rawXObjects, present := op.resources[Name("XObject")]
			if !present {
				continue
			}
			xobjectsValue, resolved := d.resolveIndirectChain(rawXObjects)
			if !resolved {
				reportError(fmt.Errorf("playa: XObject resources could not be resolved"))
				return
			}
			xobjects, ok := xobjectsValue.(Dict)
			if !ok {
				reportError(fmt.Errorf("playa: XObject resources are not a dictionary"))
				return
			}
			value := xobjects[name]
			if value == nil {
				continue
			}
			streamValue, resolved := d.resolveIndirectChain(value)
			if !resolved {
				reportError(fmt.Errorf("playa: XObject %q could not be resolved", name))
				return
			}
			stream, ok := streamValue.(Stream)
			if !ok {
				continue
			}
			streamDict := stream.DictBorrowed()
			rawSubtype, present := streamDict[Name("Subtype")]
			if !present {
				continue
			}
			subtypeValue, resolved := d.resolveIndirectChain(rawSubtype)
			if !resolved {
				reportError(fmt.Errorf("playa: XObject %q subtype could not be resolved", name))
				return
			}
			if subtypeValue != Name("Image") {
				continue
			}
			image := d.cachedImage(string(name), value, stream, op.resources)
			image.offset = op.offsetValue()
			image.page, image.hasPage = page, page != (Ref{})
			image.parentKey, image.hasParentKey = op.parentKey, op.hasParentKey
			image.parentKeyContext = image.hasParentKey
			if key, hasKey := imageStructParentKey(d, stream); hasKey {
				image.parentKey, image.hasParentKey, image.parentKeyContext = key, true, false
			}
			var valid bool
			image.bbox, valid = unitBBox(ctm)
			if !valid {
				continue
			}
			image.gstate = gstate.Clone()
			m := currentMarkedContent(marked)
			image.markedTag, image.markedProperties, image.markedStack, image.actualText, image.mcid, image.hasMCID = m.Tag, m.Properties, markedContextStack(marked), m.ActualText, m.MCID, m.HasMCID
			stopped = !yield(image)
		}
	}
}

func unitBBox(m geometry.Matrix) ([4]float64, bool) {
	points := [][2]float64{}
	for _, p := range [][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		x, y, ok := m.PointFinite(p[0], p[1])
		if !ok {
			return [4]float64{}, false
		}
		points = append(points, [2]float64{x, y})
	}
	minX, minY := points[0][0], points[0][1]
	maxX, maxY := minX, minY
	for _, p := range points[1:] {
		minX, maxX = min(minX, p[0]), max(maxX, p[0])
		minY, maxY = min(minY, p[1]), max(maxY, p[1])
	}
	return [4]float64{minX, minY, maxX, maxY}, true
}

func imageFromStream(d *Document, name string, s Stream, resourceArgs ...Dict) ImageObject {
	streamDict := s.DictBorrowed()
	im := ImageObject{name: name, data: s.DataBorrowed(), dict: cloneDict(streamDict)}
	return imageFromStreamDict(d, streamDict, im, resourceArgs...)
}

func imageFromInlineStream(d *Document, s Stream, resourceArgs ...Dict) ImageObject {
	streamDict := s.DictBorrowed()
	im := ImageObject{data: s.DataBorrowed(), dict: cloneDict(streamDict)}
	return imageFromStreamDict(d, effectiveInlineImageParams(streamDict), im, resourceArgs...)
}

func imageFromStreamDict(d *Document, streamDict Dict, im ImageObject, resourceArgs ...Dict) ImageObject {
	widthValue, _ := d.resolveIndirectChain(imageDictValue(streamDict, "Width", "W"))
	im.width, _ = IntValue(widthValue)
	heightValue, _ := d.resolveIndirectChain(imageDictValue(streamDict, "Height", "H"))
	im.height, _ = IntValue(heightValue)
	bpcRaw := imageDictValue(streamDict, "BitsPerComponent", "BPC")
	if bpcRaw == nil {
		im.bpc = 1
	} else {
		bpcValue, _ := d.resolveIndirectChain(bpcRaw)
		im.bpc, _ = IntValue(bpcValue)
	}
	maskValue, _ := d.resolveIndirectChain(imageDictValue(streamDict, "ImageMask", "IM"))
	if mask, ok := maskValue.(Bool); ok {
		im.imageMask = bool(mask)
	}
	if mask := imageDictValue(streamDict, "Mask"); mask != nil {
		im.mask, im.hasMask = mask, true
	}
	if mask := imageDictValue(streamDict, "SMask"); mask != nil {
		im.softMask, im.hasSoftMask = mask, true
	}
	csSpec, _ := d.resolveIndirectChain(imageDictValue(streamDict, "ColorSpace", "CS"))
	if len(resourceArgs) > 0 {
		if alias, ok := csSpec.(Name); ok {
			resourceColorSpaces, _ := d.resolveIndirectChain(resourceArgs[0][Name("ColorSpace")])
			if colorSpaces, ok := resourceColorSpaces.(Dict); ok {
				if resolved, exists := colorSpaces[alias]; exists {
					csSpec, _ = d.resolveIndirectChain(resolved)
				}
			}
		}
	}
	im.colorSpaceInfo = d.describeImageColorSpace(csSpec)
	if cs, ok := csSpec.(Name); ok {
		im.colorSpace = imageColorSpaceName(string(cs))
		im.components = colorSpaceComponents(im.colorSpace)
	} else if cs, ok := csSpec.(Array); ok && len(cs) > 0 {
		nameValue, _ := d.resolveIndirectChain(cs[0])
		if n, ok := nameValue.(Name); ok {
			im.colorSpace = imageColorSpaceName(string(n))
		}
		im.components = d.resolvedColorSpaceComponents(cs)
		if im.colorSpace == "Indexed" && len(cs) == 4 {
			im.indexedComponents = d.resolvedColorSpaceComponents(cs[1])
			var highOK bool
			im.indexedHigh, highOK = indexedHighValue(d, cs[2])
			if !highOK {
				im.colorSpace = ""
				im.components = 0
			}
			lookup, _ := d.resolveIndirectChain(cs[3])
			switch x := lookup.(type) {
			case String:
				im.indexedLookup = cloneObjectBytes(x)
			case Stream:
				xDict := x.DictBorrowed()
				filters, parms := streamFiltersWithResolver(xDict, d.resolveRaw)
				if len(filters) == 0 {
					im.indexedLookup = cloneObjectBytes(x.DataBorrowed())
				} else {
					im.indexedLookupData = cloneObjectBytes(x.DataBorrowed())
					im.indexedLookupFilters = cloneImageStrings(filters)
					im.indexedLookupFilterParms = cloneFilterParms(parms)
					if indexedLookupCacheAllowed(im.indexedHigh, im.indexedComponents, d.cacheLimits().ImageBytes) {
						im.indexedLookupCache = &imageDecodeCache{limit: d.cacheLimits().ImageBytes}
					}
				}
			}
		}
	}
	decodeValue, _ := d.resolveIndirectChain(imageDictValue(streamDict, "Decode", "D"))
	if decode, ok := decodeValue.(Array); ok {
		values := make([]float64, len(decode))
		valid := true
		for i, item := range decode {
			resolved, _ := d.resolveIndirectChain(item)
			value, itemValid := NumberValue(resolved)
			if !itemValid || math.IsNaN(value) || math.IsInf(value, 0) {
				valid = false
				break
			}
			values[i] = value
		}
		components := im.components
		if im.imageMask {
			components = 1
		}
		if !valid || components <= 0 || len(values) != components*2 {
			im.decode = nil
		} else {
			im.decode = values
		}
	}
	_, im.hasRawFilter = im.dict[Name("Filter")]
	im.rawFilters, _ = streamFiltersWithResolver(im.dict, d.resolveRaw)
	im.filters, im.filterParms = streamFiltersWithResolver(streamDict, d.resolveRaw)
	if len(im.filters) == 0 {
		// Unfiltered samples are already represented by Data; sharing the
		// backing array avoids retaining a second copy of every raw image.
		im.decodedData = im.data
	} else if imageDecodedCacheAllowed(im, d.cacheLimits().ImageBytes) {
		im.decodedCache = &imageDecodeCache{limit: d.cacheLimits().ImageBytes}
	}
	return im
}

func imageDecodedCacheAllowed(im ImageObject, limit int) bool {
	if limit <= 0 {
		return false
	}
	if im.width <= 0 || im.height <= 0 || im.components <= 0 {
		return false
	}
	components := im.components
	if im.colorSpace == "Indexed" && im.indexedComponents > components {
		components = im.indexedComponents
	}
	if components <= 0 {
		return false
	}
	maxUint64 := ^uint64(0)
	height, width, componentCount := uint64(im.height), uint64(im.width), uint64(components)
	if height > maxUint64/componentCount {
		return false
	}
	rowSamples := height * componentCount
	bytesPerSample := uint64(1)
	if im.bpc > 8 {
		bytesPerSample = uint64((im.bpc + 7) / 8)
	}
	if rowSamples > maxUint64/bytesPerSample {
		return false
	}
	rowBytes := rowSamples * bytesPerSample
	if width > maxUint64/rowBytes {
		return false
	}
	return width*rowBytes <= uint64(limit)
}

func indexedLookupCacheAllowed(high, components, limit int) bool {
	if high < 0 || components <= 0 || high == int(^uint(0)>>1) {
		return false
	}
	entries := high + 1
	return limit > 0 && entries <= limit/components
}

func (im ImageObject) indexedLookupBytes() []byte {
	data, _ := im.indexedLookupDecoded()
	return data
}

func (im ImageObject) indexedLookupDecoded() ([]byte, error) {
	if im.indexedLookup != nil {
		return im.indexedLookup, nil
	}
	if im.indexedLookupData == nil {
		return im.indexedLookup, nil
	}
	if len(im.indexedLookupFilters) == 0 {
		return im.indexedLookupData, nil
	}
	if im.indexedLookupCache == nil {
		return decodeFiltersLimited(im.indexedLookupData, im.indexedLookupFilters, im.indexedLookupFilterParms, decodedFilterExpansionLimit)
	}
	return im.indexedLookupCache.decode(im.indexedLookupData, im.indexedLookupFilters, im.indexedLookupFilterParms)
}

func cloneFilterParms(source []Dict) []Dict {
	if source == nil {
		return nil
	}
	out := make([]Dict, len(source))
	for i, parms := range source {
		if parms != nil {
			out[i] = cloneGraphicsObject(parms).(Dict)
		}
	}
	return out
}

func imageColorSpaceName(name string) string {
	switch name {
	case "G":
		return "DeviceGray"
	case "RGB":
		return "DeviceRGB"
	case "CMYK":
		return "DeviceCMYK"
	case "I":
		return "Indexed"
	default:
		return name
	}
}

func imageDictValue(d Dict, names ...string) Object {
	for _, name := range names {
		if value, ok := d[Name(name)]; ok {
			return value
		}
	}
	return nil
}

func (d *Document) resolvedColorSpaceComponents(o Object) int {
	o, _ = d.resolveIndirectChain(o)
	if name, ok := o.(Name); ok {
		return colorSpaceComponents(imageColorSpaceName(string(name)))
	}
	a, ok := o.(Array)
	if !ok || len(a) == 0 {
		return 0
	}
	rawNameValue, _ := d.resolveIndirectChain(a[0])
	rawName, _ := rawNameValue.(Name)
	name := Name(imageColorSpaceName(string(rawName)))
	if !validColorSpaceArrayLength(string(name), len(a)) {
		return 0
	}
	switch name {
	case "CalGray", "CalRGB", "Lab":
		if len(a) < 2 || !d.validCalibratedColorSpace(string(name), a[1]) {
			return 0
		}
		return colorSpaceComponents(string(name))
	case "ICCBased":
		if len(a) > 1 {
			profileValue, _ := d.resolveIndirectChain(a[1])
			if profile, ok := profileValue.(Stream); ok {
				return d.iccProfileComponents(profile)
			}
		}
		return 0
	case "DeviceN":
		if len(a) <= 1 {
			return 0
		}
		names, ok := d.colorantNames(a[1])
		if !ok {
			return 0
		}
		if len(a) == 4 && d.resolvedColorSpaceComponents(a[2]) <= 0 {
			return 0
		}
		return len(names)
	case "Separation":
		if len(a) <= 2 || !d.validSeparationColorant(a[1]) || d.resolvedColorSpaceComponents(a[2]) <= 0 {
			return 0
		}
		return 1
	case "Pattern":
		if len(a) > 1 {
			base := d.describeImageColorSpace(a[1])
			if base.Name() == "Pattern" {
				return 0
			}
			components := d.resolvedColorSpaceComponents(a[1])
			if components <= 0 {
				return 0
			}
			return components + 1
		}
		return 1
	case "Indexed":
		if len(a) < 3 {
			return 0
		}
		if _, ok := indexedHighValue(d, a[2]); !ok {
			return 0
		}
		if d.resolvedColorSpaceComponents(a[1]) <= 0 {
			return 0
		}
		return 1
	default:
		return colorSpaceComponents(string(name))
	}
}

// Samples returns decoded image samples. For low-bit-depth images it expands
// packed samples to one value per byte. Indexed images are then expanded
// through their palette and the returned component count is the palette's
// component count rather than the one-byte index count.
func (im ImageObject) Samples() ([]byte, int) {
	data, components, _ := im.SamplesWithError()
	return data, components
}

// SamplesWithError returns decoded image samples and preserves filter or
// indexed-palette errors that Samples suppresses for compatibility.
func (im ImageObject) SamplesWithError() ([]byte, int, error) {
	if im.width < 0 || im.height < 0 || im.components < 0 {
		return nil, im.components, fmt.Errorf("playa: invalid image dimensions or component count")
	}
	if im.bpc != 0 && im.bpc != 1 && im.bpc != 2 && im.bpc != 4 && im.bpc != 8 && im.bpc != 16 {
		return nil, im.components, fmt.Errorf("playa: invalid image bits per component %d", im.bpc)
	}
	decoded, err := im.decodedSamples()
	if err != nil {
		return nil, im.components, err
	}
	data := cloneObjectBytes(decoded)
	var indexedLookup []byte
	if im.colorSpace == "Indexed" {
		indexedLookup, err = im.indexedLookupDecoded()
		if err != nil {
			return nil, im.components, fmt.Errorf("playa: indexed image palette: %w", err)
		}
	}
	data, components := im.samples(data, indexedLookup)
	return data, components, nil
}

func (im ImageObject) samples(data, indexedLookup []byte) ([]byte, int) {
	components := im.components
	if components == 0 {
		components = 1
	}
	data = UnpackImageData(data, im.bpc, im.width, im.height, components)
	if im.colorSpace != "Indexed" || len(indexedLookup) == 0 {
		return data, components
	}
	paletteComponents := im.indexedComponents
	if paletteComponents <= 0 {
		paletteComponents = 1
	}
	maxInt := int(^uint(0) >> 1)
	if (len(data) > 0 && paletteComponents > maxInt/len(data)) || paletteComponents > maxInt/256 {
		return nil, paletteComponents
	}
	out := make([]byte, 0, len(data)*paletteComponents)
	for _, index := range data {
		at := int(index) * paletteComponents
		if at >= len(indexedLookup) {
			if paletteComponents > maxInt-at {
				return nil, paletteComponents
			}
			out = append(out, make([]byte, paletteComponents)...)
			continue
		}
		if paletteComponents > maxInt-at {
			return nil, paletteComponents
		}
		end := at + paletteComponents
		if end > len(indexedLookup) {
			end = len(indexedLookup)
		}
		out = append(out, indexedLookup[at:end]...)
		for end-at < paletteComponents {
			out = append(out, 0)
			end++
		}
	}
	return out, paletteComponents
}

func (im ImageObject) decodedSamples() ([]byte, error) {
	if len(im.decodedData) > 0 {
		return im.decodedData, nil
	}
	if len(im.filters) == 0 {
		return im.data, nil
	}
	if im.decodedCache == nil {
		return decodeFiltersLimited(im.data, im.filters, im.filterParms, decodedFilterExpansionLimit)
	}
	return im.decodedCache.decode(im.data, im.filters, im.filterParms)
}

func (cache *imageDecodeCache) decode(data []byte, filters []string, parms []Dict) ([]byte, error) {
	cache.mu.Lock()
	if cache.ready {
		decoded, err := cache.data, cache.err
		cache.mu.Unlock()
		return decoded, err
	}
	if flight := cache.flight; flight != nil {
		cache.mu.Unlock()
		<-flight.done
		return flight.data, flight.err
	}
	flight := &imageDecodeFlight{done: make(chan struct{})}
	cache.flight = flight
	cache.mu.Unlock()

	decoded, err := decodeFiltersLimited(data, filters, parms, decodedFilterExpansionLimit)
	cache.mu.Lock()
	limit := cache.limit
	if !cache.ready && (err != nil || len(decoded) <= limit) {
		cache.data, cache.err, cache.ready = decoded, err, true
	}
	flight.data, flight.err = decoded, err
	cache.flight = nil
	close(flight.done)
	cache.mu.Unlock()
	return decoded, err
}

// DecodeSample applies the PDF /Decode interval for one component. The raw
// byte returned by Samples remains available for callers that need exact
// encoded sample values.
func (im ImageObject) DecodeSample(component, sample int) float64 {
	if component < 0 || component+1 >= len(im.decode) {
		return float64(sample)
	}
	if im.bpc <= 0 || im.bpc > 16 {
		return float64(sample)
	}
	maxSample := (1 << uint(im.bpc)) - 1
	if maxSample <= 0 {
		maxSample = 1
	}
	normalized := float64(sample) / float64(maxSample)
	return im.decode[component] + normalized*(im.decode[component+1]-im.decode[component])
}

func colorSpaceComponents(name string) int {
	switch name {
	case "DeviceGray", "CalGray", "Indexed", "Pattern":
		return 1
	case "DeviceRGB", "CalRGB", "Lab":
		return 3
	case "DeviceCMYK":
		return 4
	default:
		return 0
	}
}

func arrayColorSpaceComponents(a Array) int {
	if len(a) == 0 {
		return 0
	}
	if n, ok := a[0].(Name); ok {
		return colorSpaceComponents(string(n))
	}
	return 0
}

// UnpackImageData expands PDF 1/2/4-bit samples to one sample per byte. It
// preserves row boundaries and is intentionally independent of colorspace.
func UnpackImageData(data []byte, bpc, width, height, components int) []byte {
	return imagedata.Unpack(data, bpc, width, height, components)
}
