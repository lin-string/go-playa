package document

import (
	"errors"
	"fmt"
	"iter"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
)

var ErrNilDocument = errors.New("playa: nil document")

var errNilDocument = ErrNilDocument

const (
	FilterAll     = contentconfig.FilterAll
	FilterText    = contentconfig.FilterText
	FilterPath    = contentconfig.FilterPath
	FilterImage   = contentconfig.FilterImage
	FilterXObject = contentconfig.FilterXObject
	FilterTag     = contentconfig.FilterTag
)

// DefaultContentOptions returns the default interpretation policy: emit all
// content kinds and do not restrict operators.
func DefaultContentOptions() contentconfig.Options { return contentconfig.DefaultOptions() }

func (p Page) Interp(d *Document, opts contentconfig.Options) iter.Seq2[ContentObject, error] {
	opts.RestrictOps = append([]string(nil), opts.RestrictOps...)
	return p.contentObjects(d, false, opts)
}

func (p Page) Flatten(d *Document, opts contentconfig.Options) iter.Seq2[ContentObject, error] {
	opts.RestrictOps = append([]string(nil), opts.RestrictOps...)
	return p.contentObjects(d, true, opts)
}

func (p Page) Texts(d *Document) iter.Seq2[TextObject, error] {
	return p.texts(d, true, contentconfig.Options{Filter: FilterText})
}

func (p Page) texts(d *Document, flatten bool, opts contentconfig.Options) iter.Seq2[TextObject, error] {
	return func(yield func(TextObject, error) bool) {
		if d == nil {
			yield(TextObject{}, errNilDocument)
			return
		}
		iterator := newContentOpIterator(d, p, flatten)
		allowed := make(map[string]struct{}, len(opts.RestrictOps))
		for _, name := range opts.RestrictOps {
			allowed[name] = struct{}{}
		}
		var iteratorErr error
		fonts := map[string]*Font{}
		fontLookup := func(name string) *Font {
			if font, ok := fonts[name]; ok {
				return font
			}
			font := d.pageFontByName(p, name)
			fonts[name] = font
			return font
		}
		var interpretErr error
		interpretTextWithFontResolverNextErrorPage(d, p.ref, func() (ContentOp, bool) {
			op, err, ok := iterator.next()
			if err != nil {
				iteratorErr = err
				return ContentOp{}, false
			}
			if !ok {
				return ContentOp{}, false
			}
			return op, true
		}, fontLookup, false, false, allowed, func(text TextObject) bool {
			return yield(text, nil)
		}, func(err error) bool {
			interpretErr = err
			return false
		})
		if interpretErr != nil {
			yield(TextObject{}, interpretErr)
			return
		}
		if iteratorErr != nil {
			yield(TextObject{}, iteratorErr)
		}
	}
}

func (p Page) Glyphs(d *Document) iter.Seq2[GlyphObject, error] {
	return func(yield func(GlyphObject, error) bool) {
		for text, err := range p.Texts(d) {
			if err != nil {
				yield(GlyphObject{}, err)
				return
			}
			for glyph, glyphErr := range text.GlyphsSeq() {
				if glyphErr != nil {
					yield(GlyphObject{}, glyphErr)
					return
				}
				if !yield(glyph, nil) {
					return
				}
			}
		}
	}
}

func (p Page) Paths(d *Document) iter.Seq2[PathObject, error] {
	return p.paths(d, true, contentconfig.Options{Filter: FilterPath})
}

func (p Page) paths(d *Document, flatten bool, opts contentconfig.Options) iter.Seq2[PathObject, error] {
	return func(yield func(PathObject, error) bool) {
		if d == nil {
			yield(PathObject{}, errNilDocument)
			return
		}
		iterator := newContentOpIterator(d, p, flatten)
		allowed := make(map[string]struct{}, len(opts.RestrictOps))
		for _, name := range opts.RestrictOps {
			allowed[name] = struct{}{}
		}
		var iteratorErr error
		interpretPathsNext(d, func() (ContentOp, bool) {
			op, err, ok := iterator.next()
			if err != nil {
				iteratorErr = err
				return ContentOp{}, false
			}
			if !ok {
				return ContentOp{}, false
			}
			return op, true
		}, allowed, func(path PathObject) bool {
			path.setPage(p.ref)
			return yield(path, nil)
		})
		if iteratorErr != nil {
			yield(PathObject{}, iteratorErr)
		}
	}
}

func (p Page) Images(d *Document) iter.Seq2[ImageObject, error] {
	return p.images(d, true, contentconfig.Options{Filter: FilterImage})
}

func (p Page) images(d *Document, flatten bool, opts contentconfig.Options) iter.Seq2[ImageObject, error] {
	return func(yield func(ImageObject, error) bool) {
		if d == nil {
			yield(ImageObject{}, errNilDocument)
			return
		}
		iterator := newContentOpIterator(d, p, flatten)
		allowed := make(map[string]struct{}, len(opts.RestrictOps))
		for _, name := range opts.RestrictOps {
			allowed[name] = struct{}{}
		}
		var iteratorErr error
		var interpretErr error
		interpretImagesNext(d, p.ref, func() (ContentOp, bool) {
			op, err, ok := iterator.next()
			if err != nil {
				iteratorErr = err
				return ContentOp{}, false
			}
			if !ok {
				return ContentOp{}, false
			}
			return op, true
		}, allowed, func(image ImageObject) bool { return yield(image, nil) }, func(err error) {
			interpretErr = err
		})
		if iteratorErr != nil {
			yield(ImageObject{}, iteratorErr)
		} else if interpretErr != nil {
			yield(ImageObject{}, interpretErr)
		}
	}
}

func (p Page) XObjects(d *Document) iter.Seq2[XObjectObject, error] {
	return func(yield func(XObjectObject, error) bool) {
		if d == nil {
			yield(XObjectObject{}, errNilDocument)
			return
		}
		for object, err := range p.xobjects(d) {
			if err != nil {
				yield(XObjectObject{}, err)
				return
			}
			if !yield(object, nil) {
				return
			}
		}
	}
}

// Contents exposes the decoded operators belonging to a Form XObject.
// Unlike Interp and Flatten, it does not interpret or expand those operators.
func (x XObjectObject) Contents(d *Document) iter.Seq2[ContentOp, error] {
	return func(yield func(ContentOp, error) bool) {
		if d == nil {
			yield(ContentOp{}, errNilDocument)
			return
		}
		stream := x.data.StreamBorrowed()
		value := Object(stream)
		if x.Ref() != (Ref{}) && d.objects != nil {
			resolved, ok := d.resolveIndirectChain(x.Ref())
			if !ok {
				yield(ContentOp{}, fmt.Errorf("playa: XObject reference could not be resolved"))
				return
			}
			stream, ok = resolved.(Stream)
			if !ok {
				yield(ContentOp{}, fmt.Errorf("playa: XObject reference is not a stream"))
				return
			}
			value = x.Ref()
		}
		resources := x.data.ResourcesBorrowed()
		if resources == nil {
			if x.Ref() != (Ref{}) {
				if err, ok := d.cachedXObjectResourceError(x.Ref()); ok {
					yield(ContentOp{}, err)
					return
				}
				resources, _ = d.cachedXObjectResource(x.Ref())
			}
			if resources == nil {
				rawResources, present := stream.DictBorrowed()[Name("Resources")]
				if present {
					resourcesValue, ok := d.resolveIndirectChain(rawResources)
					if !ok {
						yield(ContentOp{}, fmt.Errorf("playa: XObject Resources could not be resolved"))
						return
					}
					resources, ok = resourcesValue.(Dict)
					if !ok {
						yield(ContentOp{}, fmt.Errorf("playa: XObject Resources is not a dictionary"))
						return
					}
				}
			}
		}
		iterator := newXObjectOpIterator(d, Page{}, value, stream, resources)
		iterator.raw = true
		for {
			op, err, ok := iterator.next()
			if err != nil {
				yield(ContentOp{}, err)
				return
			}
			if !ok || !yield(op, nil) {
				return
			}
		}
	}
}

// Tokens exposes lexical tokens from the decoded Form XObject stream.
func (x XObjectObject) Tokens(d *Document) iter.Seq2[Token, error] {
	return func(yield func(Token, error) bool) {
		if err := x.validate(d); err != nil {
			yield(Token{}, err)
			return
		}
		for token, err := range x.pageView(d).Tokens(d) {
			if !yield(token, err) {
				return
			}
		}
	}
}

// CollectTokens materializes Tokens for callers that need random access.
func (x XObjectObject) CollectTokens(d *Document) ([]Token, error) {
	var out []Token
	for token, err := range x.Tokens(d) {
		if err != nil {
			return nil, err
		}
		out = append(out, token)
	}
	return out, nil
}

// Interp interprets a Form XObject in its own resource context without
// recursively expanding nested Form XObjects.
func (x XObjectObject) Interp(d *Document, opts contentconfig.Options) iter.Seq2[ContentObject, error] {
	return x.contentObjects(d, func(p Page) iter.Seq2[ContentObject, error] { return p.Interp(d, opts) })
}

// Flatten interprets a Form XObject and recursively expands nested forms.
func (x XObjectObject) Flatten(d *Document, opts contentconfig.Options) iter.Seq2[ContentObject, error] {
	return x.contentObjects(d, func(p Page) iter.Seq2[ContentObject, error] { return p.Flatten(d, opts) })
}

// Len counts the direct interpreted children of this Form XObject without
// retaining them after iteration.
func (x XObjectObject) Len(d *Document) (int, error) {
	count := 0
	for _, err := range x.Interp(d, DefaultContentOptions()) {
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

// Texts yields text objects directly contained in this Form XObject.
func (x XObjectObject) Texts(d *Document) iter.Seq2[TextObject, error] {
	return x.textObjects(d, func(p Page) iter.Seq2[TextObject, error] {
		return p.texts(d, false, contentconfig.Options{Filter: FilterText})
	})
}

// Glyphs yields glyphs from text objects directly contained in this Form
// XObject without recursively expanding nested Form XObjects.
func (x XObjectObject) Glyphs(d *Document) iter.Seq2[GlyphObject, error] {
	return func(yield func(GlyphObject, error) bool) {
		for text, err := range x.Texts(d) {
			if err != nil {
				yield(GlyphObject{}, err)
				return
			}
			for glyph, glyphErr := range text.GlyphsSeq() {
				if glyphErr != nil {
					yield(GlyphObject{}, glyphErr)
					return
				}
				if !yield(glyph, nil) {
					return
				}
			}
		}
	}
}

// Paths yields paths directly contained in this Form XObject.
func (x XObjectObject) Paths(d *Document) iter.Seq2[PathObject, error] {
	return x.pathObjects(d, func(p Page) iter.Seq2[PathObject, error] {
		return p.paths(d, false, contentconfig.Options{Filter: FilterPath})
	})
}

// Images yields image XObjects directly referenced by this Form XObject.
func (x XObjectObject) Images(d *Document) iter.Seq2[ImageObject, error] {
	return x.imageObjects(d, func(p Page) iter.Seq2[ImageObject, error] {
		return p.images(d, false, contentconfig.Options{Filter: FilterImage})
	})
}

// Tags yields marked-content points directly contained in this Form XObject.
func (x XObjectObject) Tags(d *Document) iter.Seq2[TagObject, error] {
	return x.tagObjects(d, func(p Page) iter.Seq2[TagObject, error] {
		return p.tags(d, false, contentconfig.Options{Filter: FilterTag})
	})
}

// Fonts returns fonts declared by this Form XObject's resource dictionary.
func (x XObjectObject) Fonts(d *Document) (map[string]*Font, error) {
	if err := x.validate(d); err != nil {
		return nil, err
	}
	resources, err := x.ResourcesWithError(d)
	if err != nil {
		return nil, err
	}
	return d.pageFontsDirect(Page{dict: Dict{Name("Resources"): resources}})
}

// FontsSeq lazily yields fonts declared by this Form XObject.
func (x XObjectObject) FontsSeq(d *Document) iter.Seq2[FontResource, error] {
	return func(yield func(FontResource, error) bool) {
		if err := x.validate(d); err != nil {
			yield(FontResource{}, err)
			return
		}
		resources, err := x.ResourcesWithError(d)
		if err != nil {
			yield(FontResource{}, err)
			return
		}
		page := Page{dict: Dict{Name("Resources"): resources}}
		for resource, resourceErr := range page.FontsSeq(d) {
			if !yield(resource, resourceErr) || resourceErr != nil {
				return
			}
		}
	}
}

// MarkedContent yields marked-content sections contained in this Form XObject.
func (x XObjectObject) MarkedContent(d *Document) iter.Seq2[MarkedContent, error] {
	return x.markedObjects(d, func(p Page) iter.Seq2[MarkedContent, error] { return p.MarkedContent(d) })
}

// MarkedContentByMCIDSeq yields marked-content sections with one MCID from
// this Form XObject without materializing unrelated sections.
func (x XObjectObject) MarkedContentByMCIDSeq(d *Document, mcid int) iter.Seq2[MarkedContent, error] {
	return x.markedObjects(d, func(p Page) iter.Seq2[MarkedContent, error] { return p.MarkedContentByMCIDSeq(d, mcid) })
}

// MarkedContentIndex returns this Form XObject's marked-content tree and MCID
// lookup index.
func (x XObjectObject) MarkedContentIndex(d *Document) (MarkedContentIndex, error) {
	if err := x.validate(d); err != nil {
		return MarkedContentIndex{}, err
	}
	return x.pageView(d).MarkedContentIndex(d)
}

// MarkedContentSequence returns the MCID-indexed content projection for this
// Form XObject. Its stream is interpreted lazily on first use.
func (x XObjectObject) MarkedContentSequence(d *Document) ContentSequence {
	return newXObjectContentSequence(d, x)
}

// StructureSeq maps this Form XObject's StructParent entry through the
// document ParentTree without materializing unrelated structure entries.
func (x XObjectObject) StructureSeq(d *Document) iter.Seq2[PageStructureEntry, error] {
	return x.structureObjects(d, func(p Page) iter.Seq2[PageStructureEntry, error] { return p.StructureSeq(d) })
}

// Structure returns the ParentTree structure view associated with this Form
// XObject, using its StructParent entry and resource-local content context.
func (x XObjectObject) Structure(d *Document) (PageStructure, error) {
	if err := x.validate(d); err != nil {
		return PageStructure{}, err
	}
	// A singular StructParent identifies the Form itself as a content item;
	// Playa exposes no internal marked-content sequence for it. Only the
	// plural StructParents entry can describe the XObject's internal slots.
	if _, present := x.data.StreamBorrowed().DictBorrowed()[Name("StructParent")]; present {
		return PageStructure{byMCID: map[int][]StructElement{}}, nil
	}
	return x.structurePageView(d).Structure(d)
}

func (x XObjectObject) validate(d *Document) error {
	if d == nil {
		return errNilDocument
	}
	if x.Ref() != (Ref{}) && d.objects != nil {
		resolved, ok := d.resolveIndirectChain(x.Ref())
		if !ok {
			return fmt.Errorf("playa: XObject reference could not be resolved")
		}
		if _, ok := resolved.(Stream); !ok {
			return fmt.Errorf("playa: XObject reference is not a stream")
		}
	}
	if x.data.ResourcesBorrowed() == nil {
		if x.Ref() != (Ref{}) {
			if err, ok := d.cachedXObjectResourceError(x.Ref()); ok {
				return err
			}
			if _, ok := d.cachedXObjectResource(x.Ref()); ok {
				return nil
			}
		}
		raw, present := x.data.StreamBorrowed().DictBorrowed()[Name("Resources")]
		if present {
			resolved, ok := d.resolveIndirectChain(raw)
			if !ok {
				return fmt.Errorf("playa: XObject Resources could not be resolved")
			}
			if _, ok := resolved.(Dict); !ok {
				return fmt.Errorf("playa: XObject Resources is not a dictionary")
			}
		}
	}
	return nil
}

func (x XObjectObject) contentObjects(d *Document, selectPage func(Page) iter.Seq2[ContentObject, error]) iter.Seq2[ContentObject, error] {
	return func(yield func(ContentObject, error) bool) {
		if err := x.validate(d); err != nil {
			yield(ContentObject{}, err)
			return
		}
		for object, err := range selectPage(x.pageView(d)) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (x XObjectObject) textObjects(d *Document, selectPage func(Page) iter.Seq2[TextObject, error]) iter.Seq2[TextObject, error] {
	return func(yield func(TextObject, error) bool) {
		if err := x.validate(d); err != nil {
			yield(TextObject{}, err)
			return
		}
		for object, err := range selectPage(x.pageView(d)) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (x XObjectObject) pathObjects(d *Document, selectPage func(Page) iter.Seq2[PathObject, error]) iter.Seq2[PathObject, error] {
	return func(yield func(PathObject, error) bool) {
		if err := x.validate(d); err != nil {
			yield(PathObject{}, err)
			return
		}
		for object, err := range selectPage(x.pageView(d)) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (x XObjectObject) imageObjects(d *Document, selectPage func(Page) iter.Seq2[ImageObject, error]) iter.Seq2[ImageObject, error] {
	return func(yield func(ImageObject, error) bool) {
		if err := x.validate(d); err != nil {
			yield(ImageObject{}, err)
			return
		}
		for object, err := range selectPage(x.pageView(d)) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (x XObjectObject) tagObjects(d *Document, selectPage func(Page) iter.Seq2[TagObject, error]) iter.Seq2[TagObject, error] {
	return func(yield func(TagObject, error) bool) {
		if err := x.validate(d); err != nil {
			yield(TagObject{}, err)
			return
		}
		for object, err := range selectPage(x.pageView(d)) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (x XObjectObject) markedObjects(d *Document, selectPage func(Page) iter.Seq2[MarkedContent, error]) iter.Seq2[MarkedContent, error] {
	return func(yield func(MarkedContent, error) bool) {
		if err := x.validate(d); err != nil {
			yield(MarkedContent{}, err)
			return
		}
		for object, err := range selectPage(x.pageView(d)) {
			if !yield(object, err) {
				return
			}
		}
	}
}

func (x XObjectObject) structureObjects(d *Document, selectPage func(Page) iter.Seq2[PageStructureEntry, error]) iter.Seq2[PageStructureEntry, error] {
	return func(yield func(PageStructureEntry, error) bool) {
		if err := x.validate(d); err != nil {
			yield(PageStructureEntry{}, err)
			return
		}
		if _, present := x.data.StreamBorrowed().DictBorrowed()[Name("StructParent")]; present {
			return
		}
		for object, err := range selectPage(x.structurePageView(d)) {
			if !yield(object, err) {
				return
			}
		}
	}
}

// structurePageView keeps the XObject's content context while preventing its
// synthetic page from colliding with the owning page's structure cache.
func (x XObjectObject) structurePageView(d *Document) Page {
	page := x.pageView(d)
	page.ref = Ref{}
	return page
}

func (x XObjectObject) pageView(d *Document) Page {
	resources := x.data.ResourcesBorrowed()
	if x.data.ResourceContextBorrowed() != nil {
		resources = x.data.ResourceContextBorrowed()
	}
	stream := x.data.StreamBorrowed()
	contents := Object(stream)
	if x.Ref() != (Ref{}) && d != nil {
		if resolved, ok := d.resolveIndirectChain(x.Ref()); ok {
			if resolvedStream, streamOK := resolved.(Stream); streamOK {
				stream = resolvedStream
			}
			contents = x.Ref()
		}
	}
	if resources == nil && d != nil && !x.data.HasDeclaredResources() {
		if x.Ref() != (Ref{}) {
			resources, _ = d.cachedXObjectResource(x.Ref())
		}
		if resources == nil {
			resourcesValue, _ := d.resolveIndirectChain(stream.DictBorrowed()[Name("Resources")])
			resources, _ = resourcesValue.(Dict)
		}
	}
	dict := Dict{Name("Contents"): contents, Name("Resources"): resources}
	if x.HasParentKey() {
		dict[Name("StructParents")] = Number(x.ParentKey())
	}
	return Page{
		ref:              x.Page(),
		isFormView:       true,
		dict:             dict,
		contentMatrix:    x.Matrix(),
		hasContentMatrix: x.Matrix() != (geometry.Matrix{}),
		initialGState:    x.gstate,
		hasInitialGState: x.gstate.lineWidth != 0 || x.gstate.intent != "" || x.gstate.fontName != "" || x.gstate.ctm != (geometry.Matrix{}),
	}
}

// Contents returns parsed direct page content operations in source order.
func (p Page) Contents(d *Document) iter.Seq2[ContentOp, error] {
	return func(yield func(ContentOp, error) bool) {
		if d == nil {
			yield(ContentOp{}, errNilDocument)
			return
		}
		for op, err := range p.contentOpsSeq(d) {
			if err != nil {
				yield(ContentOp{}, err)
				return
			}
			if !yield(op, nil) {
				return
			}
		}
	}
}

func (p Page) contentOpsSeq(d *Document) iter.Seq2[ContentOp, error] {
	return func(yield func(ContentOp, error) bool) {
		if d == nil {
			yield(ContentOp{}, errNilDocument)
			return
		}
		iterator := newRawContentOpIterator(d, p)
		for {
			op, err, ok := iterator.next()
			if err != nil {
				yield(ContentOp{}, err)
				return
			}
			if !ok || !yield(op, nil) {
				return
			}
		}
	}
}

// Annotations returns page annotations in source order.
func (p Page) Annotations(d *Document) iter.Seq2[Annotation, error] {
	if d == nil {
		return func(yield func(Annotation, error) bool) { yield(Annotation{}, errNilDocument) }
	}
	return d.Annotations(p)
}

// CollectAnnotations materializes the page annotation sequence for callers
// that explicitly need a complete snapshot.
func (p Page) CollectAnnotations(d *Document) ([]Annotation, error) {
	if d == nil {
		return nil, errNilDocument
	}
	return d.CollectAnnotations(p)
}

// MarkedContent returns the page's marked-content sections in source order.
func (p Page) MarkedContent(d *Document) iter.Seq2[MarkedContent, error] {
	if d == nil {
		return func(yield func(MarkedContent, error) bool) { yield(MarkedContent{}, errNilDocument) }
	}
	return d.markedContentSeq(p)
}

// MarkedContentByMCIDSeq returns marked-content sections for one MCID without
// materializing unrelated sections on the page.
func (p Page) MarkedContentByMCIDSeq(d *Document, mcid int) iter.Seq2[MarkedContent, error] {
	if d == nil {
		return func(yield func(MarkedContent, error) bool) { yield(MarkedContent{}, errNilDocument) }
	}
	return d.PageMarkedContentByMCIDSeq(p, mcid)
}

// MarkedContentIndex returns the page's nested marked-content sections and
// their MCID lookup index.
func (p Page) MarkedContentIndex(d *Document) (MarkedContentIndex, error) {
	if d == nil {
		return MarkedContentIndex{}, errNilDocument
	}
	return d.PageMarkedContentIndex(p)
}

// MarkedContentSequence returns Playa's MCID-indexed content projection for
// this page. The content stream is interpreted lazily on the first sequence
// accessor; use ContentSection.Finalize when a stable snapshot is required.
func (p Page) MarkedContentSequence(d *Document) ContentSequence {
	return newPageContentSequence(d, p)
}

// Tags returns marked-content points (MP and DP) in page content order.
func (p Page) Tags(d *Document) iter.Seq2[TagObject, error] {
	if d == nil {
		return func(yield func(TagObject, error) bool) { yield(TagObject{}, errNilDocument) }
	}
	return d.PageTagsSeq(p)
}

func (p Page) contentObjects(d *Document, flatten bool, opts contentconfig.Options) iter.Seq2[ContentObject, error] {
	return func(yield func(ContentObject, error) bool) {
		if d == nil {
			yield(ContentObject{}, errNilDocument)
			return
		}
		if opts.Filter == FilterText && len(opts.RestrictOps) == 0 {
			for text, err := range p.texts(d, flatten, opts) {
				if err != nil {
					yield(ContentObject{}, err)
					return
				}
				value := text
				if !yield(ContentObject{kind: ContentText, text: &value}, nil) {
					return
				}
			}
			return
		}
		if opts.Filter == FilterAll {
			p.orderedContentObjects(d, flatten, opts, yield)
			return
		}
		emitText := opts.Filter == FilterAll || opts.Filter == FilterText
		emitPath := opts.Filter == FilterAll || opts.Filter == FilterPath
		emitImage := opts.Filter == FilterAll || opts.Filter == FilterImage
		emitXObject := !flatten && (opts.Filter == FilterAll || opts.Filter == FilterXObject)
		emitTag := !flatten && (opts.Filter == FilterAll || opts.Filter == FilterTag)
		if emitText {
			for text, err := range p.texts(d, flatten, opts) {
				if err != nil {
					yield(ContentObject{}, err)
					return
				}
				value := text
				if !yield(ContentObject{kind: ContentText, text: &value}, nil) {
					return
				}
			}
		}
		if emitPath {
			for path, err := range p.paths(d, flatten, opts) {
				if err != nil {
					yield(ContentObject{}, err)
					return
				}
				value := path
				value.setPage(p.ref)
				if !yield(ContentObject{kind: ContentPath, path: &value}, nil) {
					return
				}
			}
		}
		if emitImage {
			for image, err := range p.images(d, flatten, opts) {
				if err != nil {
					yield(ContentObject{}, err)
					return
				}
				value := image
				if !yield(ContentObject{kind: ContentImage, image: &value}, nil) {
					return
				}
			}
		}
		if emitXObject {
			for object, err := range p.directXObjects(d) {
				if err != nil {
					yield(ContentObject{}, err)
					return
				}
				value := object
				if !yield(ContentObject{kind: ContentXObject, xobject: &value}, nil) {
					return
				}
			}
		}
		if emitTag {
			for item, err := range p.tags(d, flatten, opts) {
				if err != nil {
					yield(ContentObject{}, err)
					return
				}
				value := item
				if !yield(ContentObject{kind: ContentTag, tag: &value}, nil) {
					return
				}
			}
		}
	}
}

func (p Page) orderedContentObjects(d *Document, flatten bool, opts contentconfig.Options, yield func(ContentObject, error) bool) {
	p.streamingAllContentObjects(d, flatten, opts, yield)
}

func isPathPaintOperator(operator string) bool {
	switch operator {
	case "S", "s", "f", "F", "f*", "B", "B*", "b", "b*", "n":
		return true
	default:
		return false
	}
}

func (p Page) xobjects(d *Document) iter.Seq2[XObjectObject, error] {
	return func(yield func(XObjectObject, error) bool) {
		// XObject discovery needs the page resource context so external
		// graphics-state selections on the path to Do are applied. The raw
		// iterator is reserved for the public Contents sequence, whose
		// operands intentionally remain unresolved like Playa's parser.
		iterator := newContentOpIterator(d, p)
		var resources Dict
		var err error
		if p.hasPageCacheKey() {
			if cached, ok := d.cachedPageResource(p.ref); ok {
				resources, err = d.validateXObjectResourceDict(cached)
			} else {
				resources, err = d.xobjectResourceDict(p.dict)
			}
		} else {
			resources, err = d.xobjectResourceDict(p.dict)
		}
		if err != nil {
			yield(XObjectObject{}, err)
			return
		}
		d.walkXObjectsIterator(resources, iterator, "", map[formCycleKey]bool{}, nil, newGraphicsState(), nil, yield)
	}
}

func (d *Document) xobjectResourceDict(ctx Dict) (Dict, error) {
	if raw, present := ctx[Name("Resources")]; present {
		resolved, _ := d.resolveIndirectChain(raw)
		resources, ok := resolved.(Dict)
		if !ok {
			return nil, fmt.Errorf("playa: Resources is not a dictionary")
		}
		return d.validateXObjectResourceDict(resources)
	}
	return nil, nil
}

func (d *Document) validateXObjectResourceDict(resources Dict) (Dict, error) {
	raw, present := resources[Name("XObject")]
	if !present {
		return resources, nil
	}
	resolved, _ := d.resolveIndirectChain(raw)
	xobjects, ok := resolved.(Dict)
	if !ok {
		return nil, fmt.Errorf("playa: XObject resources are not a dictionary")
	}
	for name, value := range xobjects {
		resolved, ok := d.resolveIndirectChain(value)
		if value != nil && !ok {
			return nil, fmt.Errorf("playa: XObject resource %q could not be resolved", name)
		}
		if _, ok := resolved.(Stream); !ok {
			return nil, fmt.Errorf("playa: XObject resource %q is not a stream", name)
		}
	}
	return resources, nil
}

// directXObjects yields only Form XObjects invoked by this content stream.
// Page.XObjects is intentionally recursive, while Playa's filtered interp
// iterator exposes only objects created by the current interpreter.
func (p Page) directXObjects(d *Document) iter.Seq2[XObjectObject, error] {
	return func(yield func(XObjectObject, error) bool) {
		if d == nil {
			yield(XObjectObject{}, errNilDocument)
			return
		}
		var err error
		if p.hasPageCacheKey() {
			if cached, ok := d.cachedPageResource(p.ref); ok {
				_, err = d.validateXObjectResourceDict(cached)
			} else {
				_, err = d.xobjectResourceDict(p.dict)
			}
		} else {
			_, err = d.xobjectResourceDict(p.dict)
		}
		if err != nil {
			yield(XObjectObject{}, err)
			return
		}
		iterator := newContentOpIterator(d, p, false)
		marked := []markedContentFrame{}
		gstate := newGraphicsState()
		saved := []graphicsState{}
		for {
			op, err, ok := iterator.next()
			if err != nil {
				yield(XObjectObject{}, err)
				return
			}
			if !ok {
				return
			}
			applyMarkedContent(&marked, op)
			switch op.operatorValue() {
			case "q":
				saved = append(saved, gstate.Clone())
			case "Q":
				if len(saved) > 0 {
					gstate = saved[len(saved)-1]
					saved = saved[:len(saved)-1]
				}
			default:
				applyGraphicsState(&gstate, op)
				applyExternalGraphicsState(d, &gstate, op)
				applyResourceColorSpace(d, &gstate, op)
			}
			if op.operatorValue() != "Do" {
				continue
			}
			object, ok, err := directFormObject(d, p, op, gstate)
			if err != nil {
				yield(XObjectObject{}, err)
				return
			}
			if !ok {
				continue
			}
			context := currentMarkedContent(marked)
			object.setMarkedContent(context, marked)
			if !yield(object, nil) {
				return
			}
		}
	}
}

func (d *Document) walkXObjectsIterator(resources Dict, iterator *contentOpIterator, path string, seen map[formCycleKey]bool, marked []markedContentFrame, gstate graphicsState, saved []graphicsState, yield func(XObjectObject, error) bool) bool {
	resources, err := d.validateXObjectResourceDict(resources)
	if err != nil {
		yield(XObjectObject{}, err)
		return false
	}
	xobjectsValue, _ := d.resolveIndirectChain(resources[Name("XObject")])
	xobjects, _ := xobjectsValue.(Dict)
	for {
		op, err, ok := iterator.next()
		if err != nil {
			yield(XObjectObject{}, err)
			return false
		}
		if !ok {
			return true
		}
		switch op.operatorValue() {
		case "q":
			saved = append(saved, gstate.Clone())
		case "Q":
			if len(saved) > 0 {
				gstate = saved[len(saved)-1]
				saved = saved[:len(saved)-1]
			}
		default:
			applyGraphicsState(&gstate, op)
			applyExternalGraphicsState(d, &gstate, op)
			applyResourceColorSpace(d, &gstate, op)
		}
		applyMarkedContent(&marked, op)
		if op.operatorValue() != "Do" || len(op.operandsValue()) == 0 {
			continue
		}
		name, ok := op.operandsValue()[0].(Name)
		if !ok {
			continue
		}
		value := xobjects[name]
		streamValue, resolved := d.resolveIndirectChain(value)
		if value != nil && !resolved {
			yield(XObjectObject{}, fmt.Errorf("playa: XObject %q could not be resolved", name))
			return false
		}
		stream, ok := streamValue.(Stream)
		if !ok {
			continue
		}
		streamDict := stream.DictBorrowed()
		typValue, resolved := d.resolveIndirectChain(streamDict[Name("Subtype")])
		if streamDict[Name("Subtype")] != nil && !resolved {
			yield(XObjectObject{}, fmt.Errorf("playa: XObject %q subtype could not be resolved", name))
			return false
		}
		typ, _ := typValue.(Name)
		if typ != Name("Form") {
			continue
		}
		ref, _ := value.(Ref)
		key := d.formCycleKey(value)
		matrix := resolveFormMatrix(d, streamDict[Name("Matrix")])
		if seen[key] {
			continue
		}
		seen[key] = true
		var childResources Dict
		var declaredResources Dict
		hasCachedResources := false
		hasDeclaredResources := true
		if ref != (Ref{}) {
			if cachedErr, ok := d.cachedXObjectResourceError(ref); ok {
				yield(XObjectObject{}, cachedErr)
				return false
			}
			if cached, ok := d.cachedXObjectResource(ref); ok {
				childResources = cached
				declaredResources = cached
				hasCachedResources = true
			}
		}
		if !hasCachedResources {
			childResources, err = d.xobjectResourceDict(streamDict)
			if err != nil {
				yield(XObjectObject{}, err)
				return false
			}
			declaredResources = childResources
		}
		if childResources == nil {
			childResources = resources
		}
		childState := formGraphicsState(d, stream, gstate.Clone())
		context := currentMarkedContent(marked)
		parentKey, hasParentKey := formParentKey(d, stream)
		object := newXObjectObject(contentdata.XObjectSpec{
			Name: string(name), Ref: ref, Page: iterator.page.ref, HasPage: iterator.page.ref != (Ref{}),
			Stream: stream, Matrix: matrix, BBox: formBBoxForPage(d, iterator.page, stream, gstate.ctm.Mul(matrix)),
			Group: formGroup(d, stream), ParentKey: parentKey, HasParentKey: hasParentKey,
			Resources: childResources, DeclaredResources: declaredResources, HasDeclaredResources: hasDeclaredResources,
			ResourceContext: childResources, Path: path + string(name), MarkedTag: context.Tag,
			MarkedProperties: context.Properties, MarkedStack: markedStackCopy(markedContextStack(marked)),
			GState: childState.publicValue(),
		})
		object.markedStack = markedContextStack(marked)
		if !yield(object, nil) {
			delete(seen, key)
			return false
		}
		child := newXObjectOpIterator(d, iterator.page, value, stream, childResources)
		childWalkState := childState.Clone()
		childWalkState.ctm = childWalkState.ctm.Mul(matrix)
		// Playa creates a fresh content interpreter for each Form. The Form's
		// own BMC/BDC operators contribute to nested invocations, but marked
		// content surrounding the parent Do does not cross the Form boundary.
		if !d.walkXObjectsIterator(childResources, child, path+string(name)+"/", seen, nil, childWalkState, nil, yield) {
			return false
		}
		delete(seen, key)
	}
}
