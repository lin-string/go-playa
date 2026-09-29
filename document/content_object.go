package document

import (
	"encoding/json"
	"fmt"

	"github.com/lin-string/go-playa/contentconfig"
	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
)

const (
	ContentText       = contentconfig.KindText
	ContentPath       = contentconfig.KindPath
	ContentImage      = contentconfig.KindImage
	ContentTag        = contentconfig.KindTag
	ContentXObject    = contentconfig.KindXObject
	ContentExtGState  = contentconfig.KindExtGState
	ContentColorSpace = contentconfig.KindColorSpace
	ContentPattern    = contentconfig.KindPattern
	ContentShading    = contentconfig.KindShading
	ContentProperties = contentconfig.KindProperties
)

// XObjectObject represents a Form XObject invocation in page content.
type XObjectObject struct {
	data        contentdata.XObject
	gstate      graphicsState
	markedStack []markedContentContext
}

func newXObjectObject(spec contentdata.XObjectSpec) XObjectObject {
	return XObjectObject{
		data:   contentdata.NewXObjectBorrowed(spec),
		gstate: graphicsStateFromPublic(spec.GState),
	}
}

func (x XObjectObject) dataSpec() contentdata.XObjectSpec {
	return contentdata.XObjectSpec{
		Name: x.data.Name(), Ref: x.data.Ref(), Page: x.data.Page(), HasPage: x.data.HasPage(),
		Stream: x.data.StreamBorrowed(), Matrix: x.data.Matrix(), BBox: x.data.BBox(),
		Group: x.data.GroupBorrowed(), ParentKey: x.data.ParentKey(), HasParentKey: x.data.HasParentKey(),
		Resources: x.data.ResourcesBorrowed(), DeclaredResources: x.data.DeclaredResourcesBorrowed(),
		HasDeclaredResources: x.data.HasDeclaredResources(), ResourceContext: x.data.ResourceContextBorrowed(),
		Path: x.data.Path(), MarkedTag: x.data.MarkedTag(), MarkedProperties: x.data.MarkedPropertiesBorrowed(),
		MarkedStack: x.data.MarkedStackBorrowed(), MCID: x.data.MCID(), HasMCID: x.data.HasMCID(),
		GState: x.data.GStateBorrowed(),
	}
}

func (x *XObjectObject) updateData(update func(*contentdata.XObjectSpec)) {
	spec := x.dataSpec()
	update(&spec)
	x.data = contentdata.NewXObjectBorrowed(spec)
	x.gstate = graphicsStateFromPublic(spec.GState)
}

func (x *XObjectObject) setMarkedContent(context markedContentFrame, marked []markedContentFrame) {
	x.markedStack = markedContextStack(marked)
	x.updateData(func(spec *contentdata.XObjectSpec) {
		spec.MarkedTag = context.Tag
		spec.MarkedProperties = context.Properties
		spec.MarkedStack = markedStackCopy(x.markedStack)
	})
}

// Name returns the resource name used to invoke this Form XObject.
func (x XObjectObject) Name() string { return x.data.Name() }

// Ref returns the indirect reference of this Form XObject, when present.
func (x XObjectObject) Ref() Ref { return x.data.Ref() }

// Page returns the owning page reference, when present.
func (x XObjectObject) Page() Ref { return x.data.Page() }

// HasPage reports whether Page identifies an owning page.
func (x XObjectObject) HasPage() bool { return x.data.HasPage() }

// StreamCopy returns an independent copy of the Form XObject stream.
func (x XObjectObject) StreamCopy() Stream {
	stream := x.data.StreamBorrowed()
	if len(stream.DataBorrowed()) == 0 && stream.DictBorrowed() == nil {
		return Stream{}
	}
	return stream.Finalize()
}

// Matrix returns the Form XObject transformation matrix.
func (x XObjectObject) Matrix() geometry.Matrix { return x.data.Matrix() }

// BBox returns the Form XObject bounding box.
func (x XObjectObject) BBox() [4]float64 { return x.data.BBox() }

func (x XObjectObject) X0() float64     { return bboxX0(x.BBox()) }
func (x XObjectObject) Y0() float64     { return bboxY0(x.BBox()) }
func (x XObjectObject) X1() float64     { return bboxX1(x.BBox()) }
func (x XObjectObject) Y1() float64     { return bboxY1(x.BBox()) }
func (x XObjectObject) Width() float64  { return bboxWidth(x.BBox()) }
func (x XObjectObject) Height() float64 { return bboxHeight(x.BBox()) }
func (x XObjectObject) IsEmpty() bool   { return bboxIsEmpty(x.BBox()) }
func (x XObjectObject) IsHoverlap(other BBoxProvider) bool {
	return bboxIsHoverlap(x.BBox(), other)
}
func (x XObjectObject) HDistance(other BBoxProvider) float64 {
	return bboxHDistance(x.BBox(), other)
}
func (x XObjectObject) Hoverlap(other BBoxProvider) float64 {
	return bboxHoverlap(x.BBox(), other)
}
func (x XObjectObject) IsVOverlap(other BBoxProvider) bool {
	return bboxIsVOverlap(x.BBox(), other)
}
func (x XObjectObject) VDistance(other BBoxProvider) float64 {
	return bboxVDistance(x.BBox(), other)
}
func (x XObjectObject) VOverlap(other BBoxProvider) float64 {
	return bboxVOverlap(x.BBox(), other)
}

// ParentKey returns the ParentTree key, when present.
func (x XObjectObject) ParentKey() int { return x.data.ParentKey() }

// HasParentKey reports whether ParentKey identifies a ParentTree entry.
func (x XObjectObject) HasParentKey() bool { return x.data.HasParentKey() }

// Path returns the nested Form XObject invocation path.
func (x XObjectObject) Path() string { return x.data.Path() }

// MarkedTag returns the current marked-content tag.
func (x XObjectObject) MarkedTag() string { return x.data.MarkedTag() }

// GState returns an independent graphics-state snapshot.
func (x XObjectObject) GState() GraphicsState { return x.data.GState() }

func cloneMarkedStack(stack []markedContentContext) []markedContentContext {
	if stack == nil {
		return nil
	}
	clone := make([]markedContentContext, len(stack))
	for i := range clone {
		clone[i] = stack[i]
		clone[i].data = clone[i].data.Finalize()
	}
	return clone
}

func markedStackCopy(stack []markedContentContext) []MarkedContentContext {
	if stack == nil {
		return nil
	}
	copy := make([]MarkedContentContext, len(stack))
	for i := range copy {
		copy[i] = stack[i].data.Finalize()
	}
	return copy
}

func cloneXObjectObject(object XObjectObject) XObjectObject {
	object.data = object.data.Finalize()
	object.gstate = object.gstate.Clone()
	object.markedStack = cloneMarkedStack(object.markedStack)
	return object
}

// MarkedPropertiesCopy returns independent Form XObject marked-content properties.
func (x XObjectObject) MarkedPropertiesCopy() Dict { return x.data.MarkedPropertiesCopy() }

// GroupCopy returns an independent copy of the Form XObject transparency group.
func (x XObjectObject) GroupCopy() Dict { return x.data.GroupCopy() }

func (x XObjectObject) ownResources() Dict {
	if x.data.HasDeclaredResources() {
		return x.data.DeclaredResourcesBorrowed()
	}
	return x.data.ResourcesBorrowed()
}

// ResourcesCopy returns an independent copy of Form XObject resources that
// are declared by the Form itself. It does not expose inherited page
// resources.
func (x XObjectObject) ResourcesCopy() Dict { return cloneDict(x.ownResources()) }

// ResourcesWithError returns an independent Form XObject resource dictionary
// and reports malformed or unresolved indirect resources.
func (x XObjectObject) ResourcesWithError(d *Document) (Dict, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if x.Ref() != (Ref{}) {
		if err, ok := d.cachedXObjectResourceError(x.Ref()); ok {
			return nil, err
		}
		if resources, ok := d.cachedXObjectResource(x.Ref()); ok {
			return cloneDict(resources), nil
		}
	}
	cacheError := func(err error) (Dict, error) {
		if x.Ref() != (Ref{}) {
			err = d.storeXObjectResourceError(x.Ref(), err)
		}
		return nil, err
	}
	resources := x.ownResources()
	if resources != nil {
		for _, key := range []Name{"Font", "XObject", "ExtGState", "ColorSpace", "Pattern", "Shading", "Properties"} {
			raw, present := resources[key]
			if !present {
				continue
			}
			resolved, ok := d.resolveIndirectChain(raw)
			if !ok {
				return cacheError(fmt.Errorf("playa: %s resources could not be resolved", key))
			}
			if _, ok := resolved.(Dict); !ok {
				return cacheError(fmt.Errorf("playa: %s resources are not a dictionary", key))
			}
		}
		if err := d.validateResourceEntries(resources); err != nil {
			return cacheError(err)
		}
		if x.Ref() != (Ref{}) {
			d.storeXObjectResource(x.Ref(), resources)
		}
		return cloneDict(resources), nil
	}
	streamDict := x.data.StreamBorrowed().DictBorrowed()
	rawResources, present := streamDict[Name("Resources")]
	if !present {
		return nil, nil
	}
	value, ok := d.resolveIndirectChain(rawResources)
	if !ok {
		return cacheError(fmt.Errorf("playa: XObject Resources could not be resolved"))
	}
	resources, ok = value.(Dict)
	if !ok {
		return cacheError(fmt.Errorf("playa: XObject Resources is not a dictionary"))
	}
	if err := d.validateResourceEntries(resources); err != nil {
		return cacheError(err)
	}
	if x.Ref() != (Ref{}) {
		d.storeXObjectResource(x.Ref(), resources)
	}
	return cloneDict(resources), nil
}

// MarkedStackCopy returns an independent Form XObject marked-content stack.
func (x XObjectObject) MarkedStackCopy() []MarkedContentContext {
	return x.data.MarkedStackCopy()
}

func (x XObjectObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Name             string                 `json:"Name"`
		Ref              Ref                    `json:"Ref"`
		Page             Ref                    `json:"Page"`
		HasPage          bool                   `json:"HasPage"`
		Stream           Stream                 `json:"Stream"`
		Matrix           geometry.Matrix        `json:"Matrix"`
		BBox             [4]float64             `json:"BBox"`
		Group            Dict                   `json:"Group"`
		ParentKey        int                    `json:"ParentKey"`
		HasParentKey     bool                   `json:"HasParentKey"`
		Resources        Dict                   `json:"Resources"`
		Path             string                 `json:"Path"`
		MarkedTag        string                 `json:"MarkedTag"`
		MarkedProperties Dict                   `json:"MarkedProperties"`
		MarkedStack      []MarkedContentContext `json:"MarkedStack"`
		GState           GraphicsState          `json:"gstate"`
	}{Name: x.Name(), Ref: x.Ref(), Page: x.Page(), HasPage: x.HasPage(), Stream: x.StreamCopy(), Matrix: x.Matrix(), BBox: x.BBox(), Group: x.GroupCopy(), ParentKey: x.ParentKey(), HasParentKey: x.HasParentKey(), Resources: x.ResourcesCopy(), Path: x.Path(), MarkedTag: x.MarkedTag(), MarkedProperties: x.MarkedPropertiesCopy(), MarkedStack: x.MarkedStackCopy(), GState: x.GState()})
}

func cloneTextObject(object TextObject) TextObject {
	object.data = object.data.Finalize()
	if object.glyphs != nil {
		glyphs := object.glyphs
		object.glyphs = make([]GlyphObject, len(glyphs))
		for i, glyph := range glyphs {
			object.glyphs[i] = cloneGlyphObject(glyph)
		}
	}
	object.markedStack = cloneMarkedStack(object.markedStack)
	return object
}

func cloneContentOps(ops []ContentOp) []ContentOp {
	if ops == nil {
		return nil
	}
	clone := make([]ContentOp, len(ops))
	for i, op := range ops {
		clone[i] = op.Finalize()
	}
	return clone
}

func cloneGlyphObject(object GlyphObject) GlyphObject {
	object.data = object.data.Finalize()
	return object
}

func cloneBorrowedGlyphObjects(values []GlyphObject) []GlyphObject {
	if values == nil {
		return nil
	}
	out := make([]GlyphObject, len(values))
	for i, value := range values {
		out[i] = cloneGlyphObject(value)
	}
	return out
}

func finalizeGlyphObject(object GlyphObject) GlyphObject {
	object = cloneGlyphObject(object)
	if object.font != nil {
		object.font = object.font.Finalize()
	}
	if object.outline != nil {
		outline := *object.outline
		outline.ops = cloneContentOps(outline.ops)
		object.outline = &outline
	}
	return object
}

func finalizeGlyphObjectWithError(object GlyphObject) (GlyphObject, error) {
	object = cloneGlyphObject(object)
	if object.font != nil {
		font, err := object.font.FinalizeWithError()
		if err != nil {
			return GlyphObject{}, err
		}
		object.font = font
	}
	if object.outline != nil {
		outline := *object.outline
		outline.ops = cloneContentOps(outline.ops)
		object.outline = &outline
	}
	return object, nil
}

func finalizeTextObject(object TextObject) TextObject {
	object = cloneTextObject(object)
	if object.font != nil {
		object.font = object.font.Finalize()
	}
	for i := range object.glyphs {
		object.glyphs[i] = finalizeGlyphObject(object.glyphs[i])
	}
	return object
}

func finalizeTextObjectWithError(object TextObject) (TextObject, error) {
	object = cloneTextObject(object)
	if object.font != nil {
		font, err := object.font.FinalizeWithError()
		if err != nil {
			return TextObject{}, err
		}
		object.font = font
	}
	for i := range object.glyphs {
		glyph, err := finalizeGlyphObjectWithError(object.glyphs[i])
		if err != nil {
			return TextObject{}, err
		}
		object.glyphs[i] = glyph
	}
	return object, nil
}

func clonePathObject(object PathObject) PathObject {
	object.data = object.data.Finalize()
	return object
}

// Finalize returns an independent snapshot of this text object.
func (object TextObject) Finalize() TextObject { return finalizeTextObject(object) }

// FinalizeWithError returns an independent text snapshot and reports deferred
// font-program or character-procedure failures.
func (object TextObject) FinalizeWithError() (TextObject, error) {
	return finalizeTextObjectWithError(object)
}

// Finalize returns an independent snapshot of this glyph object.
func (object GlyphObject) Finalize() GlyphObject { return finalizeGlyphObject(object) }

// FinalizeWithError returns an independent glyph snapshot and reports deferred
// font-program or character-procedure failures.
func (object GlyphObject) FinalizeWithError() (GlyphObject, error) {
	return finalizeGlyphObjectWithError(object)
}

// Finalize returns an independent snapshot of this path object.
func (object PathObject) Finalize() PathObject { return clonePathObject(object) }

// Parent resolves the structure element associated with this Form XObject.
func (x XObjectObject) Parent(d *Document) *StructElement {
	parent, _ := x.ParentWithError(d)
	return parent
}

// ParentWithError resolves this Form XObject's ParentTree element and reports
// malformed ParentTree data.
func (x XObjectObject) ParentWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if !x.HasParentKey() {
		// Form XObjects do not inherit the page's StructParents key. Playa's
		// XObjectObject.from_stream leaves _parentkey unset when the Form has
		// neither StructParent nor StructParents, even inside marked content.
		return nil, nil
	}
	value, present, err := d.parentTreeValueChecked(x.ParentKey())
	if err != nil || !present {
		return nil, err
	}
	resolved, ok := d.resolveIndirectChain(value)
	if !ok {
		return nil, fmt.Errorf("playa: ParentTree value could not be resolved")
	}
	if _, direct := resolved.(Dict); direct {
		return d.parentTreeElementWithError(x.ParentKey())
	}
	// If an explicit Form key names an array, Playa falls back to generic
	// marked-content lookup using the same Form-local ParentTree key.
	stack := x.data.MarkedStackBorrowed()
	for index := len(stack) - 1; index >= 0; index-- {
		if stack[index].HasMCID() {
			return d.parentTreeContentElementWithError(x.ParentKey(), stack[index].MCID())
		}
	}
	if x.data.HasMCID() {
		return d.parentTreeContentElementWithError(x.ParentKey(), x.data.MCID())
	}
	return nil, nil
}

// PageObject resolves the page associated with a page-level Form XObject.
func (x XObjectObject) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !x.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(x.Page())
}

// Buffer returns a defensive copy of the raw Form XObject stream bytes.
func (x XObjectObject) Buffer() []byte {
	return x.data.StreamBorrowed().Buffer()
}

// DecodedBuffer returns a defensive copy of the Form XObject stream after its
// PDF filter chain, matching Playa's XObject buffer property.
func (x XObjectObject) DecodedBuffer() []byte {
	return x.data.StreamBorrowed().DecodedBuffer()
}

// DecodedBufferWithError returns decoded Form XObject stream bytes and any
// filter error.
func (x XObjectObject) DecodedBufferWithError() ([]byte, error) {
	return x.data.StreamBorrowed().DecodedBufferWithError()
}

// DecodedBufferWithDocument resolves indirect stream filter metadata through
// d before decoding the Form XObject stream.
func (x XObjectObject) DecodedBufferWithDocument(d *Document) []byte {
	return x.data.StreamBorrowed().DecodedBufferWithDocument(d)
}

// DecodedBufferWithDocumentWithError is the error-aware document-resolved
// Form XObject stream decoder.
func (x XObjectObject) DecodedBufferWithDocumentWithError(d *Document) ([]byte, error) {
	return x.data.StreamBorrowed().DecodedBufferWithDocumentWithError(d)
}

// Get looks up a Form XObject stream dictionary entry without decoding it.
func (x XObjectObject) Get(key Name) (Object, bool) {
	return x.data.StreamBorrowed().Get(key)
}

// Has reports whether a Form XObject stream dictionary entry is present.
func (x XObjectObject) Has(key Name) bool {
	return x.data.StreamBorrowed().Has(key)
}

// TagObject represents a marked-content point (MP or DP).
type TagObject struct {
	data         contentdata.Tag
	parentKey    int
	hasParentKey bool
}

func newTagObject(spec contentdata.TagSpec) TagObject {
	return TagObject{data: contentdata.NewTagBorrowed(spec)}
}

// Name returns the resource name associated with this marked-content point.
func (t TagObject) Name() string { return t.data.Name() }

// Page returns the owning page reference, when present.
func (t TagObject) Page() Ref { return t.data.Page() }

// HasPage reports whether Page identifies an owning page.
func (t TagObject) HasPage() bool { return t.data.HasPage() }

// ActualText returns the decoded replacement text, when present.
func (t TagObject) ActualText() string { return t.data.ActualText() }

// MCID returns the marked-content identifier, when present.
func (t TagObject) MCID() int { return t.data.MCID() }

// HasMCID reports whether MCID is present.
func (t TagObject) HasMCID() bool { return t.data.HasMCID() }

// Len returns the number of children yielded by a marked-content point. Tag
// points are leaf content objects in Playa's interpreter model.
func (t TagObject) Len() int { return 0 }

// MarkedTag returns the enclosing marked-content tag name, when present.
func (t TagObject) MarkedTag() string { return t.data.MarkedTag() }

// GState returns an independent graphics-state value.
func (t TagObject) GState() GraphicsState { return t.data.GState() }

func cloneTagObject(tag TagObject) TagObject {
	tag.data = tag.data.Finalize()
	return tag
}

// MarkedPropertiesCopy returns independent tag marked-content properties.
func (t TagObject) MarkedPropertiesCopy() Dict { return t.data.MarkedPropertiesCopy() }

// PropertiesCopy returns an independent copy of tag properties.
func (t TagObject) PropertiesCopy() Dict { return t.data.PropertiesCopy() }

// MarkedStackCopy returns an independent tag marked-content stack.
func (t TagObject) MarkedStackCopy() []MarkedContentContext {
	return t.data.MarkedStackCopy()
}

func (t TagObject) MarshalJSON() ([]byte, error) { return json.Marshal(t.data) }

// Finalize returns an independent snapshot of this image object.
func (im ImageObject) Finalize() ImageObject { return cloneImageObject(im) }

// FinalizeWithError returns an independent image snapshot and reports deferred
// indexed-palette or color-space lookup failures without decoding samples.
func (im ImageObject) FinalizeWithError() (ImageObject, error) {
	if im.colorSpace == "Indexed" {
		if _, err := im.indexedLookupDecoded(); err != nil {
			return ImageObject{}, err
		}
	}
	colorSpace, err := im.colorSpaceInfo.FinalizeWithError()
	if err != nil {
		return ImageObject{}, err
	}
	clone := cloneImageObject(im)
	clone.colorSpaceInfo = colorSpace
	return clone, nil
}

// Finalize returns an independent snapshot of this Form XObject invocation.
func (x XObjectObject) Finalize() XObjectObject { return cloneXObjectObject(x) }

// Finalize returns an independent snapshot of this marked-content tag.
func (t TagObject) Finalize() TagObject { return cloneTagObject(t) }

// PageObject resolves the page associated with a page-level tag point.
func (t TagObject) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !t.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(t.Page())
}

// Parent resolves the structure element associated with this tag's MCID.
func (t TagObject) Parent(d *Document) *StructElement {
	parent, _ := t.ParentWithError(d)
	return parent
}

// ParentWithError resolves this tag's ParentTree element and reports
// malformed page structure data.
func (t TagObject) ParentWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	stack := t.data.MarkedStackBorrowed()
	for index := len(stack) - 1; index >= 0; index-- {
		if stack[index].HasMCID() {
			return d.contentParentWithContext(t.Page(), stack[index].MCID(), true, t.parentKey, t.hasParentKey)
		}
	}
	return d.contentParentWithContext(t.Page(), t.MCID(), t.HasMCID(), t.parentKey, t.hasParentKey)
}

// MarkedContentContext is the dependency-free value for one enclosing
// marked-content section. MarkedStack is ordered from outermost to innermost.
type MarkedContentContext = contentdata.MarkedContentContext

// markedContentContext keeps parser-only identity alongside the public value.
// The identity is used only while grouping adjacent text objects and never
// escapes through the public content model.
type markedContentContext struct {
	data     contentdata.MarkedContentContext
	identity *markedContentIdentity
}

func newMarkedContentContext(tag string, properties Dict, actualText string, mcid int, hasMCID bool, identity *markedContentIdentity) markedContentContext {
	return markedContentContext{
		data:     contentdata.NewMarkedContentContextBorrowed(tag, properties, actualText, mcid, hasMCID),
		identity: identity,
	}
}

func (m markedContentContext) Tag() string          { return m.data.Tag() }
func (m markedContentContext) ActualText() string   { return m.data.ActualText() }
func (m markedContentContext) MCID() int            { return m.data.MCID() }
func (m markedContentContext) HasMCID() bool        { return m.data.HasMCID() }
func (m markedContentContext) PropertiesCopy() Dict { return m.data.PropertiesCopy() }

// ContentObject is a page-content object. Exactly one payload is non-nil,
// according to Kind.
type ContentObject struct {
	kind       contentconfig.Kind
	text       *TextObject
	path       *PathObject
	image      *ImageObject
	tag        *TagObject
	xobject    *XObjectObject
	extgstate  *ExtGStateObject
	colorspace *ColorSpaceObject
	pattern    *PatternObject
	shading    *ShadingObject
	properties *PropertiesObject
}

// Kind returns the Playa-style concrete content kind.
func (o ContentObject) Kind() contentconfig.Kind { return o.kind }

// MarshalJSON preserves the concrete payload projection without exposing the
// borrowed payload pointers held by the iterator result.
func (o ContentObject) MarshalJSON() ([]byte, error) {
	type projection struct {
		Kind       contentconfig.Kind
		Text       *TextObject
		Path       *PathObject
		Image      *ImageObject
		Tag        *TagObject
		XObject    *XObjectObject
		ExtGState  *ExtGStateObject
		ColorSpace *ColorSpaceObject
		Pattern    *PatternObject
		Shading    *ShadingObject
		Properties *PropertiesObject
	}
	return json.Marshal(projection{Kind: o.kind, Text: o.text, Path: o.path, Image: o.image, Tag: o.tag, XObject: o.xobject, ExtGState: o.extgstate, ColorSpace: o.colorspace, Pattern: o.pattern, Shading: o.shading, Properties: o.properties})
}

// TextBorrowed returns the text payload without copying it. Use TextCopy or
// Finalize when the payload must outlive the current borrowed sequence view.
func (o ContentObject) TextBorrowed() *TextObject { return o.text }

// PathBorrowed returns the path payload without copying it. Use PathCopy or
// Finalize for an independent snapshot.
func (o ContentObject) PathBorrowed() *PathObject { return o.path }

// ImageBorrowed returns the image payload without copying it. Use ImageCopy
// or Finalize for an independent snapshot.
func (o ContentObject) ImageBorrowed() *ImageObject { return o.image }

// TagBorrowed returns the marked-content point without copying it. Use
// TagCopy or Finalize for an independent snapshot.
func (o ContentObject) TagBorrowed() *TagObject { return o.tag }

// XObjectBorrowed returns the Form XObject payload without copying it. Use
// XObjectCopy or Finalize for an independent snapshot.
func (o ContentObject) XObjectBorrowed() *XObjectObject { return o.xobject }

// ExtGStateBorrowed returns the graphics-state resource without copying it.
// Use ExtGStateCopy or Finalize for an independent snapshot.
func (o ContentObject) ExtGStateBorrowed() *ExtGStateObject { return o.extgstate }

// ColorSpaceBorrowed returns the color-space resource without copying it.
// Use ColorSpaceCopy or Finalize for an independent snapshot.
func (o ContentObject) ColorSpaceBorrowed() *ColorSpaceObject { return o.colorspace }

// PatternBorrowed returns the pattern resource without copying it. Use
// PatternCopy or Finalize for an independent snapshot.
func (o ContentObject) PatternBorrowed() *PatternObject { return o.pattern }

// ShadingBorrowed returns the shading resource without copying it. Use
// ShadingCopy or Finalize for an independent snapshot.
func (o ContentObject) ShadingBorrowed() *ShadingObject { return o.shading }

// PropertiesBorrowed returns the properties resource without copying it. Use
// PropertiesCopy or Finalize for an independent snapshot.
func (o ContentObject) PropertiesBorrowed() *PropertiesObject { return o.properties }

// TextCopy returns an independent text payload, when this is a text object.
func (o ContentObject) TextCopy() *TextObject {
	if o.text == nil {
		return nil
	}
	clone := finalizeTextObject(*o.text)
	return &clone
}

// TextCopyWithError returns an independent text payload and reports deferred
// font-program or character-procedure failures.
func (o ContentObject) TextCopyWithError() (*TextObject, error) {
	if o.text == nil {
		return nil, nil
	}
	clone, err := finalizeTextObjectWithError(*o.text)
	if err != nil {
		return nil, err
	}
	return &clone, nil
}

// PathCopy returns an independent path payload, when this is a path object.
func (o ContentObject) PathCopy() *PathObject {
	if o.path == nil {
		return nil
	}
	clone := clonePathObject(*o.path)
	return &clone
}

// ImageCopy returns an independent image payload, when this is an image object.
func (o ContentObject) ImageCopy() *ImageObject {
	if o.image == nil {
		return nil
	}
	clone := cloneImageObject(*o.image)
	return &clone
}

// TagCopy returns an independent tag payload, when this is a tag object.
func (o ContentObject) TagCopy() *TagObject {
	if o.tag == nil {
		return nil
	}
	clone := cloneTagObject(*o.tag)
	return &clone
}

// XObjectCopy returns an independent form XObject payload, when present.
func (o ContentObject) XObjectCopy() *XObjectObject {
	if o.xobject == nil {
		return nil
	}
	clone := cloneXObjectObject(*o.xobject)
	return &clone
}

func (o ContentObject) ExtGStateCopy() *ExtGStateObject {
	if o.extgstate == nil {
		return nil
	}
	clone := o.extgstate.Finalize()
	return &clone
}

func (o ContentObject) ColorSpaceCopy() *ColorSpaceObject {
	if o.colorspace == nil {
		return nil
	}
	clone := o.colorspace.Finalize()
	return &clone
}

func (o ContentObject) PatternCopy() *PatternObject {
	if o.pattern == nil {
		return nil
	}
	clone := o.pattern.Finalize()
	return &clone
}

func (o ContentObject) ShadingCopy() *ShadingObject {
	if o.shading == nil {
		return nil
	}
	clone := o.shading.Finalize()
	return &clone
}

func (o ContentObject) PropertiesCopy() *PropertiesObject {
	if o.properties == nil {
		return nil
	}
	clone := o.properties.Finalize()
	return &clone
}

func cloneContentObject(object ContentObject) ContentObject {
	clone := object
	if object.text != nil {
		text := finalizeTextObject(*object.text)
		clone.text = &text
	}
	if object.path != nil {
		path := clonePathObject(*object.path)
		clone.path = &path
	}
	if object.image != nil {
		image := cloneImageObject(*object.image)
		clone.image = &image
	}
	if object.xobject != nil {
		xobject := cloneXObjectObject(*object.xobject)
		clone.xobject = &xobject
	}
	if object.tag != nil {
		tag := cloneTagObject(*object.tag)
		clone.tag = &tag
	}
	if object.extgstate != nil {
		value := object.extgstate.Finalize()
		clone.extgstate = &value
	}
	if object.colorspace != nil {
		value := object.colorspace.Finalize()
		clone.colorspace = &value
	}
	if object.pattern != nil {
		value := object.pattern.Finalize()
		clone.pattern = &value
	}
	if object.shading != nil {
		value := object.shading.Finalize()
		clone.shading = &value
	}
	if object.properties != nil {
		value := object.properties.Finalize()
		clone.properties = &value
	}
	return clone
}

// Finalize returns an independent snapshot of the concrete content payload.
func (o ContentObject) Finalize() ContentObject { return cloneContentObject(o) }

// FinalizeWithError returns an independent content snapshot and reports
// deferred errors from a text or glyph font.
func (o ContentObject) FinalizeWithError() (ContentObject, error) {
	clone := o
	if o.text != nil {
		text, err := finalizeTextObjectWithError(*o.text)
		if err != nil {
			return ContentObject{}, err
		}
		clone.text = &text
	}
	if o.path != nil {
		path := clonePathObject(*o.path)
		clone.path = &path
	}
	if o.image != nil {
		image, err := o.image.FinalizeWithError()
		if err != nil {
			return ContentObject{}, err
		}
		clone.image = &image
	}
	if o.xobject != nil {
		xobject := cloneXObjectObject(*o.xobject)
		clone.xobject = &xobject
	}
	if o.tag != nil {
		tag := cloneTagObject(*o.tag)
		clone.tag = &tag
	}
	if o.extgstate != nil {
		value := o.extgstate.Finalize()
		clone.extgstate = &value
	}
	if o.colorspace != nil {
		value, err := o.colorspace.FinalizeWithError()
		if err != nil {
			return ContentObject{}, err
		}
		clone.colorspace = &value
	}
	if o.pattern != nil {
		value := o.pattern.Finalize()
		clone.pattern = &value
	}
	if o.shading != nil {
		value := o.shading.Finalize()
		clone.shading = &value
	}
	if o.properties != nil {
		value := o.properties.Finalize()
		clone.properties = &value
	}
	return clone, nil
}

// ObjectType returns the Playa-style concrete content object name.
func (o ContentObject) ObjectType() string {
	return string(o.kind)
}

// Len returns the number of children yielded by the concrete content object.
// Form XObjects are interpreted lazily; other non-text content kinds are
// leaves, matching Playa's generic ContentObject.__len__ behavior.
func (o ContentObject) Len(d *Document) (int, error) {
	switch o.kind {
	case ContentText:
		if o.text != nil {
			return o.text.Len(), nil
		}
	case ContentXObject:
		if o.xobject != nil {
			return o.xobject.Len(d)
		}
	case ContentPath:
		if o.path != nil {
			return o.path.Len(), nil
		}
	case ContentImage:
		if o.image != nil {
			return o.image.Len(), nil
		}
	case ContentTag:
		if o.tag != nil {
			return o.tag.Len(), nil
		}
	}
	return 0, nil
}

// GraphicsStateCopy returns an independent copy of the graphics state carried
// by a drawable content object. Resource-selection objects do not carry a
// painted graphics state and return the zero value.
func (o ContentObject) GraphicsStateCopy() GraphicsState {
	switch o.kind {
	case ContentText:
		if o.text != nil {
			return o.text.GState()
		}
	case ContentPath:
		if o.path != nil {
			return o.path.GState()
		}
	case ContentImage:
		if o.image != nil {
			return o.image.gstate.publicValue()
		}
	case ContentTag:
		if o.tag != nil {
			return o.tag.GState()
		}
	case ContentXObject:
		if o.xobject != nil {
			return o.xobject.gstate.publicValue()
		}
	}
	return GraphicsState{}
}

// MatrixValue returns the device-space CTM captured by a drawable content
// object. Resource-selection objects do not carry a drawable matrix.
func (o ContentObject) MatrixValue() (geometry.Matrix, bool) {
	switch o.kind {
	case ContentText:
		if o.text != nil {
			return o.text.GState().CTM(), true
		}
	case ContentPath:
		if o.path != nil {
			return o.path.GState().CTM(), true
		}
	case ContentImage:
		if o.image != nil {
			return o.image.gstate.ctm, true
		}
	case ContentTag:
		if o.tag != nil {
			return o.tag.GState().CTM(), true
		}
	case ContentXObject:
		if o.xobject != nil {
			// Playa exposes a Form XObject's ctm after applying the Form's
			// optional Matrix to the invocation CTM. Keep GState as the
			// inherited invocation state so interpreting the Form applies its
			// Matrix exactly once.
			return o.xobject.gstate.ctm.Mul(o.xobject.Matrix()), true
		}
	}
	return geometry.Matrix{}, false
}

// MarkedStackCopy returns an independent outermost-to-innermost marked
// content stack carried by the content object.
func (o ContentObject) MarkedStackCopy() []MarkedContentContext {
	switch o.kind {
	case ContentText:
		if o.text != nil {
			return o.text.MarkedStackCopy()
		}
	case ContentPath:
		if o.path != nil {
			return o.path.MarkedStackCopy()
		}
	case ContentImage:
		if o.image != nil {
			return o.image.MarkedStackCopy()
		}
	case ContentTag:
		if o.tag != nil {
			return o.tag.MarkedStackCopy()
		}
	case ContentXObject:
		if o.xobject != nil {
			return o.xobject.MarkedStackCopy()
		}
	}
	return nil
}

// BBoxValue returns the concrete object's device-space bounding box when it
// has one. Marked-content points do not have a geometry of their own.
func (o ContentObject) BBoxValue() ([4]float64, bool) {
	switch o.kind {
	case ContentText:
		if o.text != nil {
			return o.text.BBox(), true
		}
	case ContentPath:
		if o.path != nil {
			return o.path.BBox(), true
		}
	case ContentImage:
		if o.image != nil {
			return o.image.bbox, true
		}
	case ContentXObject:
		if o.xobject != nil {
			return o.xobject.BBox(), true
		}
	}
	return [4]float64{}, false
}

// MarkedContext returns the nearest enclosing marked-content section.
func (o ContentObject) MarkedContext() *MarkedContentContext {
	var stack []markedContentContext
	var publicStack []MarkedContentContext
	switch o.kind {
	case ContentText:
		if o.text != nil {
			publicStack = o.text.MarkedStackCopy()
		}
	case ContentPath:
		if o.path != nil {
			publicStack = o.path.MarkedStackCopy()
		}
	case ContentImage:
		if o.image != nil {
			stack = o.image.markedStack
		}
	case ContentTag:
		if o.tag != nil {
			publicStack = o.tag.data.MarkedStackCopy()
		}
	case ContentXObject:
		if o.xobject != nil {
			stack = o.xobject.markedStack
		}
	}
	if len(publicStack) > 0 {
		context := publicStack[len(publicStack)-1].Finalize()
		return &context
	}
	if len(stack) == 0 {
		return nil
	}
	context := stack[len(stack)-1].data.Finalize()
	return &context
}

// MCIDValue returns the nearest marked-content ID, if one exists.
func (o ContentObject) MCIDValue() (int, bool) {
	var stack []markedContentContext
	var publicStack []MarkedContentContext
	var fallback int
	var hasFallback bool
	switch o.kind {
	case ContentText:
		if o.text != nil {
			publicStack, fallback, hasFallback = o.text.MarkedStackCopy(), o.text.MCID(), o.text.HasMCID()
		}
	case ContentPath:
		if o.path != nil {
			publicStack, fallback, hasFallback = o.path.MarkedStackCopy(), o.path.MCID(), o.path.HasMCID()
		}
	case ContentImage:
		if o.image != nil {
			stack, fallback, hasFallback = o.image.markedStack, o.image.mcid, o.image.hasMCID
		}
	case ContentTag:
		if o.tag != nil {
			publicStack, fallback, hasFallback = o.tag.data.MarkedStackCopy(), o.tag.MCID(), o.tag.HasMCID()
		}
	case ContentXObject:
		if o.xobject != nil {
			stack = o.xobject.markedStack
		}
	}
	for index := len(publicStack) - 1; index >= 0; index-- {
		if publicStack[index].HasMCID() {
			return publicStack[index].MCID(), true
		}
	}
	for index := len(stack) - 1; index >= 0; index-- {
		if stack[index].HasMCID() {
			return stack[index].MCID(), true
		}
	}
	return fallback, hasFallback
}

// PageObject resolves the page associated with the concrete content payload.
func (o ContentObject) PageObject(d *Document) (Page, error) {
	switch o.kind {
	case ContentText:
		if o.text != nil {
			return o.text.PageObject(d)
		}
	case ContentPath:
		if o.path != nil {
			return o.path.PageObject(d)
		}
	case ContentImage:
		if o.image != nil {
			return o.image.PageObject(d)
		}
	case ContentTag:
		if o.tag != nil {
			return o.tag.PageObject(d)
		}
	case ContentXObject:
		if o.xobject != nil {
			return o.xobject.PageObject(d)
		}
	}
	if d == nil {
		return Page{}, errNilDocument
	}
	return Page{}, ErrPageNotFound
}

// Parent resolves the structure element associated with the concrete
// content payload, when that payload has a page/MCID or ParentTree key.
func (o ContentObject) Parent(d *Document) *StructElement {
	parent, _ := o.ParentWithError(d)
	return parent
}

// ParentWithError resolves the concrete content payload's ParentTree element
// and reports malformed page structure data.
func (o ContentObject) ParentWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	switch o.kind {
	case ContentText:
		if o.text != nil {
			return o.text.ParentWithError(d)
		}
	case ContentPath:
		if o.path != nil {
			return o.path.ParentWithError(d)
		}
	case ContentImage:
		if o.image != nil {
			return o.image.ParentWithError(d)
		}
	case ContentTag:
		if o.tag != nil {
			return o.tag.ParentWithError(d)
		}
	case ContentXObject:
		if o.xobject != nil {
			return o.xobject.ParentWithError(d)
		}
	}
	return nil, nil
}
