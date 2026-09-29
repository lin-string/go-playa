package document

import (
	"encoding/json"
	"fmt"
	"iter"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/fontdata"
	"github.com/lin-string/go-playa/geometry"
)

func identity() geometry.Matrix { return geometry.Matrix{1, 0, 0, 1, 0, 0} }

func resolveFormMatrix(d *Document, value Object) geometry.Matrix {
	resolved, _ := d.resolveIndirectChain(value)
	values, ok := resolved.(Array)
	if !ok || len(values) != 6 {
		return identity()
	}
	var matrix geometry.Matrix
	for i := range matrix {
		item, _ := d.resolveIndirectChain(values[i])
		number, valid := finiteNumberValue(item)
		if !valid || math.IsNaN(number) || math.IsInf(number, 0) {
			return identity()
		}
		matrix[i] = number
	}
	return matrix
}

func validFormMatrix(d *Document, value Object) bool {
	resolved, _ := d.resolveIndirectChain(value)
	values, ok := resolved.(Array)
	if !ok || len(values) != 6 {
		return false
	}
	for _, item := range values {
		resolvedItem, _ := d.resolveIndirectChain(item)
		number, ok := finiteNumberValue(resolvedItem)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
			return false
		}
	}
	return true
}

type GlyphObject struct {
	data    contentdata.Glyph
	font    *Font
	outline *glyphOutlineContext
	parent  *glyphParentContext
}

// glyphParentContext is immutable and shared by the glyphs in one text
// object. A nil context means that the page has no structure parent key.
type glyphParentContext struct {
	key int
}

type glyphOutlineContext struct {
	ops    []ContentOp
	matrix geometry.Matrix
}

func newGlyphOutlineContext(ops []ContentOp, matrix geometry.Matrix) *glyphOutlineContext {
	if len(ops) == 0 && matrix == (geometry.Matrix{}) {
		return nil
	}
	return &glyphOutlineContext{ops: ops, matrix: matrix}
}

func newGlyphObject(spec contentdata.GlyphSpec, font *Font, type3Ops []ContentOp, type3Matrix geometry.Matrix) GlyphObject {
	return GlyphObject{data: contentdata.NewGlyphBorrowed(spec), font: font, outline: newGlyphOutlineContext(type3Ops, type3Matrix)}
}

func newGlyphObjectWithContext(spec contentdata.GlyphSpec, context *contentdata.GlyphContext, font *Font, type3Ops []ContentOp, type3Matrix geometry.Matrix) GlyphObject {
	return GlyphObject{data: contentdata.NewGlyphBorrowedWithContext(spec, context), font: font, outline: newGlyphOutlineContext(type3Ops, type3Matrix)}
}

func (g GlyphObject) outlineOpsBorrowed() []ContentOp {
	if g.outline == nil {
		return nil
	}
	return g.outline.ops
}

func (g GlyphObject) outlineMatrix() geometry.Matrix {
	if g.outline == nil {
		return geometry.Matrix{}
	}
	return g.outline.matrix
}

func (g *GlyphObject) setOutline(ops []ContentOp, matrix geometry.Matrix) {
	if g.outline == nil {
		g.outline = &glyphOutlineContext{}
	}
	g.outline.ops = ops
	g.outline.matrix = matrix
}

func (g *GlyphObject) setGlyphText(value string) {
	spec := g.data.SpecBorrowed()
	spec.Text = value
	g.data = contentdata.NewGlyphBorrowedWithContext(spec, g.data.ContextBorrowed())
}

func (g *GlyphObject) setGlyphBBox(value [4]float64) {
	spec := g.data.SpecBorrowed()
	spec.BBox = value
	g.data = contentdata.NewGlyphBorrowedWithContext(spec, g.data.ContextBorrowed())
}

func (g *GlyphObject) setGlyphGraphicsContext(context *contentdata.GlyphContext) {
	spec := g.data.SpecBorrowed()
	g.data = contentdata.NewGlyphBorrowedWithContext(spec, context)
}

func (g GlyphObject) Text() string             { return g.data.Text() }
func (g GlyphObject) Chars() string            { return g.data.Chars() }
func (g GlyphObject) Page() Ref                { return g.data.Page() }
func (g GlyphObject) HasPage() bool            { return g.data.HasPage() }
func (g GlyphObject) MCID() int                { return g.data.MCID() }
func (g GlyphObject) HasMCID() bool            { return g.data.HasMCID() }
func (g GlyphObject) CID() int                 { return g.data.CID() }
func (g GlyphObject) GID() int                 { return g.data.GID() }
func (g GlyphObject) FontName() string         { return g.data.FontName() }
func (g GlyphObject) FontSize() float64        { return g.data.FontSize() }
func (g GlyphObject) Size() float64            { return g.data.Size() }
func (g GlyphObject) FontBase() string         { return g.data.FontBase() }
func (g GlyphObject) TextFont() string         { return g.data.TextFont() }
func (g GlyphObject) Matrix() geometry.Matrix  { return g.data.Matrix() }
func (g GlyphObject) Origin() [2]float64       { return g.data.Origin() }
func (g GlyphObject) Displacement() [2]float64 { return g.data.Displacement() }
func (g GlyphObject) BBox() [4]float64         { return g.data.BBox() }
func (g GlyphObject) X0() float64              { return bboxX0(g.BBox()) }
func (g GlyphObject) Y0() float64              { return bboxY0(g.BBox()) }
func (g GlyphObject) X1() float64              { return bboxX1(g.BBox()) }
func (g GlyphObject) Y1() float64              { return bboxY1(g.BBox()) }
func (g GlyphObject) Width() float64           { return bboxWidth(g.BBox()) }
func (g GlyphObject) Height() float64          { return bboxHeight(g.BBox()) }
func (g GlyphObject) IsEmpty() bool            { return bboxIsEmpty(g.BBox()) }
func (g GlyphObject) IsHoverlap(other BBoxProvider) bool {
	return bboxIsHoverlap(g.BBox(), other)
}
func (g GlyphObject) HDistance(other BBoxProvider) float64 {
	return bboxHDistance(g.BBox(), other)
}
func (g GlyphObject) Hoverlap(other BBoxProvider) float64 {
	return bboxHoverlap(g.BBox(), other)
}
func (g GlyphObject) IsVOverlap(other BBoxProvider) bool {
	return bboxIsVOverlap(g.BBox(), other)
}
func (g GlyphObject) VDistance(other BBoxProvider) float64 {
	return bboxVDistance(g.BBox(), other)
}
func (g GlyphObject) VOverlap(other BBoxProvider) float64 {
	return bboxVOverlap(g.BBox(), other)
}
func (g GlyphObject) Vertical() bool        { return g.data.Vertical() }
func (g GlyphObject) Unmapped() bool        { return g.data.Unmapped() }
func (g GlyphObject) Invisible() bool       { return g.data.Invisible() }
func (g GlyphObject) GState() GraphicsState { return g.data.GState() }

// FontCopy returns an independent snapshot of the glyph font.
func (g GlyphObject) FontCopy() *Font {
	if g.font == nil {
		return nil
	}
	return g.font.Finalize()
}

// FontCopyWithError returns an independent glyph font snapshot and reports
// deferred font-program or character-procedure failures.
func (g GlyphObject) FontCopyWithError() (*Font, error) {
	if g.font == nil {
		return nil, nil
	}
	return g.font.FinalizeWithError()
}

// ValidateWithError checks deferred glyph-font resources without copying the
// borrowed glyph or its font.
func (g GlyphObject) ValidateWithError() error {
	if g.font == nil {
		return nil
	}
	return g.font.ValidateWithError()
}

// Codes returns an independent copy of the source code bytes.
func (g GlyphObject) Codes() []byte {
	return g.data.CodeCopy()
}

// MarkedStackCopy returns an independent copy of the marked-content stack.
func (g GlyphObject) MarkedStackCopy() []MarkedContentContext {
	return g.data.MarkedStackCopy()
}

// MarshalJSON preserves the public projection without exposing the backing
// code slice to callers.
func (g GlyphObject) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Text         string          `json:"text"`
		Chars        string          `json:"chars"`
		MCID         int             `json:"mcid,omitempty"`
		HasMCID      bool            `json:"has_mcid"`
		Code         []byte          `json:"code,omitempty"`
		CID          int             `json:"cid"`
		GID          int             `json:"-"`
		FontName     string          `json:"font_name"`
		FontSize     float64         `json:"font_size"`
		Size         float64         `json:"size"`
		FontBase     string          `json:"font_base"`
		TextFont     string          `json:"text_font"`
		Matrix       geometry.Matrix `json:"matrix"`
		Origin       [2]float64      `json:"origin"`
		Displacement [2]float64      `json:"displacement"`
		BBox         [4]float64      `json:"bbox"`
		Vertical     bool            `json:"vertical"`
		Unmapped     bool            `json:"unmapped"`
		Invisible    bool            `json:"invisible"`
		GState       GraphicsState   `json:"gstate"`
	}{Text: g.Text(), Chars: g.Chars(), MCID: g.MCID(), HasMCID: g.HasMCID(), Code: g.Codes(), CID: g.CID(), GID: g.GID(), FontName: g.FontName(), FontSize: g.FontSize(), Size: g.Size(), FontBase: g.FontBase(), TextFont: g.TextFont(), Matrix: g.Matrix(), Origin: g.Origin(), Displacement: g.Displacement(), BBox: g.BBox(), Vertical: g.Vertical(), Unmapped: g.Unmapped(), Invisible: g.Invisible(), GState: g.GState()})
}

// PageObject resolves the page associated with a page-level glyph.
func (g GlyphObject) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !g.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(g.Page())
}

// Parent resolves the structure element associated with this glyph's MCID.
func (g GlyphObject) Parent(d *Document) *StructElement {
	parent, _ := g.ParentWithError(d)
	return parent
}

// ParentWithError resolves this glyph's ParentTree element and reports
// malformed page structure data.
func (g GlyphObject) ParentWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	if g.parent == nil {
		return d.contentParentWithContext(g.Page(), g.MCID(), g.HasMCID(), 0, false)
	}
	return d.contentParentWithContext(g.Page(), g.MCID(), g.HasMCID(), g.parent.key, true)
}

// TextObject retains document-owned font resolution and glyph traversal while
// dependency-free text state lives in contentdata.Text.
type TextObject struct {
	data         contentdata.Text
	font         *Font
	glyphs       []GlyphObject
	markedStack  []markedContentContext
	parentKey    int
	hasParentKey bool
	insideForm   bool
}

// textObjectState is the mutable interpreter builder. It is never exposed to
// callers; a single contentdata.Text snapshot is published at flush time.
type textObjectState struct {
	text             string                 `json:"-"`
	page             Ref                    `json:"-"`
	hasPage          bool                   `json:"-"`
	chars            string                 `json:"-"`
	fontName         string                 `json:"-"`
	fontSize         float64                `json:"-"`
	font             *Font                  `json:"-"`
	size             float64                `json:"-"`
	fontBase         string                 `json:"-"`
	textFont         string                 `json:"-"`
	glyphFontName    string                 `json:"-"`
	glyphFontSize    float64                `json:"-"`
	glyphFontBase    string                 `json:"-"`
	glyphTextFont    string                 `json:"-"`
	glyphVertical    bool                   `json:"-"`
	args             []Object               `json:"-"`
	matrix           geometry.Matrix        `json:"-"`
	textMatrix       geometry.Matrix        `json:"-"`
	lineMatrix       geometry.Matrix        `json:"-"`
	scalingMatrix    geometry.Matrix        `json:"-"`
	origin           [2]float64             `json:"-"`
	displacement     [2]float64             `json:"-"`
	rotation         float64                `json:"-"`
	glyphs           []GlyphObject          `json:"-"`
	bbox             [4]float64             `json:"-"`
	visualBBox       *[4]float64            `json:"-"`
	lineWidth        float64                `json:"-"`
	strokeColor      []float64              `json:"-"`
	nonStrokeColor   []float64              `json:"-"`
	invisible        bool                   `json:"-"`
	vertical         bool                   `json:"-"`
	unmapped         bool                   `json:"-"`
	gstate           graphicsState          `json:"-"`
	markedTag        string                 `json:"-"`
	markedProperties Dict                   `json:"-"`
	markedStack      []markedContentContext `json:"-"`
	actualText       string                 `json:"-"`
	mcid             int                    `json:"-"`
	hasMCID          bool                   `json:"-"`
	parentKey        int                    `json:"-"`
	hasParentKey     bool                   `json:"-"`
	insideForm       bool                   `json:"-"`
}

func (s textObjectState) publicValue() TextObject {
	stack := make([]contentdata.MarkedContentContext, len(s.markedStack))
	for i := range s.markedStack {
		stack[i] = s.markedStack[i].data
	}
	object := TextObject{
		data: contentdata.NewTextBorrowed(contentdata.TextSpec{
			Text: s.text, Page: s.page, HasPage: s.hasPage, Chars: s.chars,
			FontName: s.fontName, FontSize: s.fontSize, Size: s.size,
			FontBase: s.fontBase, TextFont: s.textFont, Args: s.args,
			Matrix: s.matrix, TextMatrix: s.textMatrix, LineMatrix: s.lineMatrix,
			ScalingMatrix: s.scalingMatrix, Origin: s.origin, Displacement: s.displacement,
			Rotation: s.rotation, BBox: s.bbox, VisualBBox: s.visualBBox,
			LineWidth: s.lineWidth, StrokeColor: s.strokeColor, NonStrokeColor: s.nonStrokeColor,
			Invisible: s.invisible, Vertical: s.vertical, Unmapped: s.unmapped,
			GState: s.gstate.publicValue(), MarkedTag: s.markedTag,
			MarkedProperties: s.markedProperties, MarkedStack: stack,
			ActualText: s.actualText, MCID: s.mcid, HasMCID: s.hasMCID,
		}),
		font: s.font, glyphs: s.glyphs, markedStack: s.markedStack,
		parentKey: s.parentKey, hasParentKey: s.hasParentKey,
		insideForm: s.insideForm,
	}
	s.attachGlyphParent(&object)
	return object
}

func (s textObjectState) layoutValue() TextObject {
	object := TextObject{
		font: s.font, glyphs: s.glyphs,
		parentKey: s.parentKey, hasParentKey: s.hasParentKey,
		insideForm: s.insideForm,
	}
	s.attachGlyphParent(&object)
	return object
}

func (s textObjectState) attachGlyphParent(object *TextObject) {
	var parent *glyphParentContext
	if s.hasParentKey {
		parent = &glyphParentContext{key: s.parentKey}
	}
	for index := range object.glyphs {
		object.glyphs[index].parent = parent
	}
}

func (t TextObject) Text() string                   { return t.data.Text() }
func (t TextObject) Page() Ref                      { return t.data.Page() }
func (t TextObject) HasPage() bool                  { return t.data.HasPage() }
func (t TextObject) Chars() string                  { return t.data.Chars() }
func (t TextObject) FontName() string               { return t.data.FontName() }
func (t TextObject) FontSize() float64              { return t.data.FontSize() }
func (t TextObject) Size() float64                  { return t.data.Size() }
func (t TextObject) FontBase() string               { return t.data.FontBase() }
func (t TextObject) TextFont() string               { return t.data.TextFont() }
func (t TextObject) Matrix() geometry.Matrix        { return t.data.Matrix() }
func (t TextObject) TextMatrix() geometry.Matrix    { return t.data.TextMatrix() }
func (t TextObject) LineMatrix() geometry.Matrix    { return t.data.LineMatrix() }
func (t TextObject) ScalingMatrix() geometry.Matrix { return t.data.ScalingMatrix() }
func (t TextObject) Origin() [2]float64             { return t.data.Origin() }
func (t TextObject) Displacement() [2]float64       { return t.data.Displacement() }
func (t TextObject) Rotation() float64              { return t.data.Rotation() }
func (t TextObject) BBox() [4]float64               { return t.data.BBox() }
func (t TextObject) X0() float64                    { return bboxX0(t.BBox()) }
func (t TextObject) Y0() float64                    { return bboxY0(t.BBox()) }
func (t TextObject) X1() float64                    { return bboxX1(t.BBox()) }
func (t TextObject) Y1() float64                    { return bboxY1(t.BBox()) }
func (t TextObject) Width() float64                 { return bboxWidth(t.BBox()) }
func (t TextObject) Height() float64                { return bboxHeight(t.BBox()) }
func (t TextObject) IsEmpty() bool                  { return bboxIsEmpty(t.BBox()) }
func (t TextObject) IsHoverlap(other BBoxProvider) bool {
	return bboxIsHoverlap(t.BBox(), other)
}
func (t TextObject) HDistance(other BBoxProvider) float64 {
	return bboxHDistance(t.BBox(), other)
}
func (t TextObject) Hoverlap(other BBoxProvider) float64 {
	return bboxHoverlap(t.BBox(), other)
}
func (t TextObject) IsVOverlap(other BBoxProvider) bool {
	return bboxIsVOverlap(t.BBox(), other)
}
func (t TextObject) VDistance(other BBoxProvider) float64 {
	return bboxVDistance(t.BBox(), other)
}
func (t TextObject) VOverlap(other BBoxProvider) float64 {
	return bboxVOverlap(t.BBox(), other)
}
func (t TextObject) LineWidth() float64    { return t.data.LineWidth() }
func (t TextObject) Invisible() bool       { return t.data.Invisible() }
func (t TextObject) Vertical() bool        { return t.data.Vertical() }
func (t TextObject) Unmapped() bool        { return t.data.Unmapped() }
func (t TextObject) GState() GraphicsState { return t.data.GState() }
func (t TextObject) MarkedTag() string     { return t.data.MarkedTag() }
func (t TextObject) ActualText() string    { return t.data.ActualText() }
func (t TextObject) MCID() int             { return t.data.MCID() }
func (t TextObject) HasMCID() bool         { return t.data.HasMCID() }

// Len returns the number of glyphs yielded by GlyphsSeq.
func (t TextObject) Len() int { return len(t.glyphs) }

// FontCopy returns an independent snapshot of the text font.
func (t TextObject) FontCopy() *Font {
	if t.font == nil {
		return nil
	}
	return t.font.Finalize()
}

// FontCopyWithError returns an independent text font snapshot and reports
// deferred font-program or character-procedure failures.
func (t TextObject) FontCopyWithError() (*Font, error) {
	if t.font == nil {
		return nil, nil
	}
	return t.font.FinalizeWithError()
}

// ValidateWithError checks deferred text and glyph-font resources without
// copying the borrowed text object. Shared fonts are validated only once.
func (t TextObject) ValidateWithError() error {
	if err := t.font.ValidateWithError(); err != nil {
		return err
	}
	for _, glyph := range t.glyphs {
		if glyph.font == t.font {
			continue
		}
		if err := glyph.ValidateWithError(); err != nil {
			return err
		}
	}
	return nil
}

// ArgsCopy returns an independent copy of the raw text arguments.
func (t TextObject) ArgsCopy() []Object {
	return t.data.ArgsCopy()
}

// ArgsSeq lazily yields borrowed text-show arguments in source order. Use
// ArgsCopy or Finalize when an owned snapshot is required.
func (t TextObject) ArgsSeq() iter.Seq[Object] {
	return t.data.ArgsSeq()
}

// GlyphsCopy returns independent glyph snapshots in source order.
func (t TextObject) GlyphsCopy() []GlyphObject {
	if t.glyphs == nil {
		return nil
	}
	glyphs := make([]GlyphObject, len(t.glyphs))
	for i, glyph := range t.glyphs {
		glyphs[i] = finalizeGlyphObject(glyph)
	}
	return glyphs
}

// GlyphsCopyWithError returns independent glyph snapshots and reports a
// deferred font-program or character-procedure failure.
func (t TextObject) GlyphsCopyWithError() ([]GlyphObject, error) {
	if t.glyphs == nil {
		return nil, nil
	}
	glyphs := make([]GlyphObject, len(t.glyphs))
	for i, glyph := range t.glyphs {
		copy, err := finalizeGlyphObjectWithError(glyph)
		if err != nil {
			return nil, err
		}
		glyphs[i] = copy
	}
	return glyphs, nil
}

// StrokeColorCopy returns the text object's stroke color components.
func (t TextObject) StrokeColorCopy() []float64 {
	return t.data.StrokeColorCopy()
}

// NonStrokeColorCopy returns the text object's non-stroke color components.
func (t TextObject) NonStrokeColorCopy() []float64 {
	return t.data.NonStrokeColorCopy()
}

// VisualBBoxCopy returns the optional visual bounding box.
func (t TextObject) VisualBBoxCopy() ([4]float64, bool) {
	return t.data.VisualBBoxCopy()
}

// MarkedPropertiesCopy returns an independent copy of the marked-content
// properties attached to the text object.
func (t TextObject) MarkedPropertiesCopy() Dict {
	return t.data.MarkedPropertiesCopy()
}

// MarkedStackCopy returns an independent copy of the marked-content stack.
func (t TextObject) MarkedStackCopy() []MarkedContentContext {
	return t.data.MarkedStackCopy()
}

// MarshalJSON preserves the public projection without exposing backing
// content slices.
func (t TextObject) MarshalJSON() ([]byte, error) {
	visualBBox, hasVisualBBox := t.VisualBBoxCopy()
	var visualBBoxValue *[4]float64
	if hasVisualBBox {
		visualBBoxValue = &visualBBox
	}
	return json.Marshal(&struct {
		Text             string                 `json:"text"`
		Chars            string                 `json:"chars"`
		FontName         string                 `json:"font_name"`
		FontSize         float64                `json:"font_size"`
		Size             float64                `json:"size"`
		FontBase         string                 `json:"font_base"`
		TextFont         string                 `json:"text_font"`
		Matrix           geometry.Matrix        `json:"matrix"`
		TextMatrix       geometry.Matrix        `json:"text_matrix"`
		LineMatrix       geometry.Matrix        `json:"line_matrix"`
		ScalingMatrix    geometry.Matrix        `json:"scaling_matrix"`
		Origin           [2]float64             `json:"origin"`
		Displacement     [2]float64             `json:"displacement"`
		Rotation         float64                `json:"rotation"`
		BBox             [4]float64             `json:"bbox"`
		VisualBBox       *[4]float64            `json:"visual_bbox"`
		LineWidth        float64                `json:"linewidth"`
		Invisible        bool                   `json:"invisible"`
		Vertical         bool                   `json:"vertical"`
		Unmapped         bool                   `json:"unmapped"`
		GState           GraphicsState          `json:"gstate"`
		MarkedTag        string                 `json:"marked_tag,omitempty"`
		ActualText       string                 `json:"actual_text,omitempty"`
		MCID             int                    `json:"mcid,omitempty"`
		HasMCID          bool                   `json:"has_mcid"`
		Args             []Object               `json:"args,omitempty"`
		Glyphs           []GlyphObject          `json:"glyphs"`
		StrokeColor      []float64              `json:"scolor"`
		NonStrokeColor   []float64              `json:"ncolor"`
		MarkedProperties Dict                   `json:"marked_properties,omitempty"`
		MarkedStack      []MarkedContentContext `json:"marked_stack,omitempty"`
	}{Text: t.Text(), Chars: t.Chars(), FontName: t.FontName(), FontSize: t.FontSize(), Size: t.Size(), FontBase: t.FontBase(), TextFont: t.TextFont(), Matrix: t.Matrix(), TextMatrix: t.TextMatrix(), LineMatrix: t.LineMatrix(), ScalingMatrix: t.ScalingMatrix(), Origin: t.Origin(), Displacement: t.Displacement(), Rotation: t.Rotation(), BBox: t.BBox(), VisualBBox: visualBBoxValue, LineWidth: t.LineWidth(), Invisible: t.Invisible(), Vertical: t.Vertical(), Unmapped: t.Unmapped(), GState: t.GState(), MarkedTag: t.MarkedTag(), ActualText: t.ActualText(), MCID: t.MCID(), HasMCID: t.HasMCID(), Args: t.ArgsCopy(), Glyphs: cloneBorrowedGlyphObjects(t.glyphs), StrokeColor: t.StrokeColorCopy(), NonStrokeColor: t.NonStrokeColorCopy(), MarkedProperties: t.MarkedPropertiesCopy(), MarkedStack: t.MarkedStackCopy()})
}

// PageObject resolves the page associated with a page-level text object.
func (t TextObject) PageObject(d *Document) (Page, error) {
	if d == nil {
		return Page{}, errNilDocument
	}
	if !t.HasPage() {
		return Page{}, ErrPageNotFound
	}
	return d.PageByRef(t.Page())
}

// Parent resolves the structure element associated with this text object's MCID.
func (t TextObject) Parent(d *Document) *StructElement {
	parent, _ := t.ParentWithError(d)
	return parent
}

// ParentWithError resolves this text object's ParentTree element and reports
// malformed page structure data.
func (t TextObject) ParentWithError(d *Document) (*StructElement, error) {
	if d == nil {
		return nil, errNilDocument
	}
	return d.contentParentWithContext(t.Page(), t.MCID(), t.HasMCID(), t.parentKey, t.hasParentKey)
}

// InterpretText implements the common text operators and intentionally keeps
// the output model independent of native PDFium handles. Font decoding and
// full graphics-state semantics are layered in the font package.
func InterpretText(ops []ContentOp) []TextObject {
	return InterpretTextWithFonts(ops, nil)
}

// InterpretTextSeq lazily interprets text objects in content-stream order.
// The sequence is repeatable and stops interpreting when the consumer stops
// yielding.
func InterpretTextSeq(ops []ContentOp) iter.Seq[TextObject] {
	return InterpretTextWithFontsSeq(ops, nil)
}

func InterpretTextWithFonts(ops []ContentOp, fonts map[string]*Font) []TextObject {
	var lookup func(string) *Font
	if fonts != nil {
		lookup = func(name string) *Font { return fonts[name] }
	}
	return interpretTextWithFontResolver(ops, lookup)
}

// InterpretTextWithFontsSeq is the lazy counterpart of
// InterpretTextWithFonts. The supplied font map is read during iteration and
// is not retained by the returned sequence.
func InterpretTextWithFontsSeq(ops []ContentOp, fonts map[string]*Font) iter.Seq[TextObject] {
	var lookup func(string) *Font
	if fonts != nil {
		lookup = func(name string) *Font { return fonts[name] }
	}
	return func(yield func(TextObject) bool) {
		index := 0
		interpretTextWithFontResolverNext(nil, func() (ContentOp, bool) {
			if index >= len(ops) {
				return ContentOp{}, false
			}
			op := ops[index]
			index++
			return op, true
		}, lookup, true, nil, yield)
	}
}

func interpretTextWithFontResolver(ops []ContentOp, lookup func(string) *Font) []TextObject {
	index := 0
	out := []TextObject{}
	interpretTextWithFontResolverNext(nil, func() (ContentOp, bool) {
		if index >= len(ops) {
			return ContentOp{}, false
		}
		op := ops[index]
		index++
		return op, true
	}, lookup, true, nil, func(text TextObject) bool {
		out = append(out, text)
		return true
	})
	return out
}

func interpretTextWithFontResolverNext(d *Document, next func() (ContentOp, bool), lookup func(string) *Font, copyTextArgs bool, restrictOps map[string]struct{}, yield func(TextObject) bool) {
	interpretTextWithFontResolverNextPage(d, Ref{}, next, lookup, copyTextArgs, restrictOps, yield)
}

func interpretTextWithFontResolverNextPage(d *Document, page Ref, next func() (ContentOp, bool), lookup func(string) *Font, copyTextArgs bool, restrictOps map[string]struct{}, yield func(TextObject) bool) {
	interpretTextWithFontResolverNextErrorPage(d, page, next, lookup, copyTextArgs, false, restrictOps, yield, nil)
}

func interpretTextWithFontResolverNextErrorPage(d *Document, page Ref, next func() (ContentOp, bool), lookup func(string) *Font, copyTextArgs, layoutValueOnly bool, restrictOps map[string]struct{}, yield func(TextObject) bool, reportError func(error) bool) {
	text := identity()
	textLine := identity()
	ctm := identity()
	font := ""
	size := 0.0
	lineWidth := 1.0
	charSpace := 0.0
	wordSpace := 0.0
	leading := 0.0
	type state struct {
		ctm                                                     geometry.Matrix
		font                                                    string
		size                                                    float64
		lineWidth, scaling, rise, charSpace, wordSpace, leading float64
		renderMode                                              int
		strokeAlpha, fillAlpha                                  float64
		blendMode                                               string
		stroke, nonStroke                                       []float64
		strokeSpace, nonStrokeSpace                             string
		external                                                graphicsState
	}
	stack := []state{}
	external := newGraphicsState()
	renderMode := 0
	scaling := 1.0
	var glyphScratch []DecodedGlyph
	var textObjectStart geometry.Matrix
	var hasTextObjectStart bool
	glyphOffsetX, glyphOffsetY := 0.0, 0.0
	rise := 0.0
	strokeColor := []float64{0}
	nonStrokeColor := []float64{0}
	strokeSpace := "DeviceGray"
	nonStrokeSpace := "DeviceGray"
	strokeComponents := 1
	nonStrokeComponents := 1
	strokeAlpha := 1.0
	fillAlpha := 1.0
	blendMode := ""
	fontTextNames := make(map[textFontNameKey]string)
	type markedState struct {
		tag        string
		properties Dict
		mcid       int
		hasMCID    bool
		actualText string
		identity   *markedContentIdentity
	}
	marked := []markedState{}
	markedForms := [][]markedState{}
	objectParentKey := 0
	objectHasParentKey := false
	currentMarked := func() markedState {
		if len(marked) == 0 {
			return markedState{}
		}
		return marked[len(marked)-1]
	}
	newTextObject := func() (textObjectState, bool) {
		m := currentMarked()
		mcid, hasMCID := m.mcid, m.hasMCID
		for index := len(marked) - 1; index >= 0; index-- {
			if marked[index].hasMCID {
				mcid, hasMCID = marked[index].mcid, true
				break
			}
		}
		stack := make([]markedContentContext, len(marked))
		for i, state := range marked {
			stack[i] = newMarkedContentContext(state.tag, state.properties, state.actualText, state.mcid, state.hasMCID, state.identity)
		}
		objectText := text
		if hasTextObjectStart {
			objectText = textObjectStart
		}
		originX, originY, ok := ctm.PointFinite(objectText[4], objectText[5]+rise)
		if !ok {
			return textObjectState{}, false
		}
		scalingMatrix := geometry.Matrix{size * scaling, 0, 0, size, 0, rise}
		textMatrix := objectText
		renderingMatrix, valid := textRenderingMatrix(ctm, textMatrix, scalingMatrix)
		if !valid {
			return textObjectState{}, false
		}
		hasTextObjectStart = false
		return textObjectState{page: page, hasPage: page != (Ref{}), fontName: font, fontSize: size, matrix: renderingMatrix, textMatrix: textMatrix, lineMatrix: textLine, scalingMatrix: scalingMatrix, origin: [2]float64{originX, originY}, args: make([]Object, 0, 2), markedTag: m.tag, markedProperties: m.properties, markedStack: stack, actualText: m.actualText, mcid: mcid, hasMCID: hasMCID, parentKey: objectParentKey, hasParentKey: objectHasParentKey, insideForm: len(markedForms) > 0}, true
	}
	cur := textObjectState{}
	active := false
	emitCurrent := true
	textBBoxPoints := [][2]float64{}
	stopped := false
	moveTextLine := func(dx, dy float64) {
		if next, ok := textLine.OffsetFinite(dx, dy); ok {
			textLine = next
			text = textLine
			glyphOffsetX, glyphOffsetY = 0, 0
		}
	}
	updateTextFromGlyphOffset := func() bool {
		next, ok := textLine.OffsetFinite(glyphOffsetX, glyphOffsetY)
		if ok {
			text = next
		}
		return ok
	}
	flush := func() {
		if active {
			cur.text = ""
			endX, endY, ok := ctm.PointFinite(text[4], text[5]+rise)
			if !ok {
				if reportError != nil {
					reportError(fmt.Errorf("playa: non-finite text displacement origin"))
				}
				stopped = true
				return
			}
			cur.displacement = [2]float64{endX - cur.origin[0], endY - cur.origin[1]}
			cur.rotation = math.Atan2(text[1], text[0]) * 180 / math.Pi
			if math.Abs(cur.rotation) < 1e-2 {
				// Avoid publishing insignificant matrix noise as a visible
				// rotation while retaining real rotations such as 90 degrees.
				cur.rotation = 0
			}
			cur.lineWidth = lineWidth
			cur.strokeColor = append([]float64(nil), strokeColor...)
			cur.nonStrokeColor = append([]float64(nil), nonStrokeColor...)
			cur.gstate = external
			cur.gstate.dash = append([]float64(nil), external.dash...)
			cur.gstate.blendModes = append([]string(nil), external.blendModes...)
			cur.gstate.softMask = cloneGraphicsObject(external.softMask)
			cur.gstate.ctm = ctm
			cur.gstate.fontName = cur.fontName
			cur.gstate.fontSize = cur.fontSize
			cur.gstate.lineWidth = lineWidth
			cur.gstate.strokeColor = geometry.NewColor(strokeSpace, strokeColor, external.strokeColor.Pattern(), external.strokeColor.Components())
			cur.gstate.fillColor = geometry.NewColor(nonStrokeSpace, nonStrokeColor, external.fillColor.Pattern(), external.fillColor.Components())
			cur.gstate.renderMode = renderMode
			cur.gstate.characterRise = rise
			cur.gstate.characterSpacing = charSpace
			cur.gstate.wordSpacing = wordSpace
			cur.gstate.horizontalScale = scaling
			cur.gstate.leading = leading
			cur.gstate.alpha = fillAlpha
			cur.gstate.strokeAlpha = strokeAlpha
			cur.gstate.fillAlpha = fillAlpha
			cur.gstate.blendMode = blendMode
			cur.invisible = renderMode == 3 || scaling <= .02
			cur.vertical = false
			cur.unmapped = false
			if lookup != nil {
				if f := lookup(cur.fontName); f != nil {
					cur.vertical = f.vertical
					cur.unmapped = false
					cur.font = f
					cur.fontName = f.name
				}
			}
			if len(cur.glyphs) > 0 {
				cur.size = cur.glyphFontSize
				cur.fontBase = cur.glyphFontBase
				cur.textFont = cur.glyphTextFont
				stack := markedStackCopy(cur.markedStack)
				context := contentdata.NewGlyphContextBorrowed(contentdata.GlyphContextSpec{
					Page: cur.page, HasPage: cur.hasPage, MCID: cur.mcid, HasMCID: cur.hasMCID,
					FontName: cur.glyphFontName, FontSize: cur.glyphFontSize, Size: cur.glyphFontSize,
					FontBase: cur.glyphFontBase, TextFont: cur.glyphTextFont, Vertical: cur.glyphVertical,
					GState: cur.gstate.publicValue(), MarkedStack: stack,
				})
				for i := range cur.glyphs {
					cur.glyphs[i].setGlyphGraphicsContext(context)
				}
			}
			var textBuilder strings.Builder
			textBuilder.Grow(len(cur.glyphs))
			for _, g := range cur.glyphs {
				_, _ = textBuilder.WriteString(g.Text())
				if g.Unmapped() {
					cur.unmapped = true
				}
			}
			cur.text = textBuilder.String()
			cur.chars = cur.text
			if len(cur.glyphs) > 0 {
				cur.bbox = cur.glyphs[0].BBox()
				var visual *[4]float64
				for _, g := range cur.glyphs[1:] {
					bbox := g.BBox()
					cur.bbox[0] = math.Min(cur.bbox[0], bbox[0])
					cur.bbox[1] = math.Min(cur.bbox[1], bbox[1])
					cur.bbox[2] = math.Max(cur.bbox[2], bbox[2])
					cur.bbox[3] = math.Max(cur.bbox[3], bbox[3])
				}
				for _, point := range textBBoxPoints {
					cur.bbox[0] = math.Min(cur.bbox[0], point[0])
					cur.bbox[1] = math.Min(cur.bbox[1], point[1])
					cur.bbox[2] = math.Max(cur.bbox[2], point[0])
					cur.bbox[3] = math.Max(cur.bbox[3], point[1])
				}
				if cur.vertical {
					for _, glyph := range cur.glyphs {
						origin := glyph.Origin()
						cur.bbox[0] = math.Min(cur.bbox[0], origin[0])
						cur.bbox[1] = math.Min(cur.bbox[1], origin[1])
						cur.bbox[2] = math.Max(cur.bbox[2], origin[0])
						cur.bbox[3] = math.Max(cur.bbox[3], origin[1])
					}
				}
				if len(textBBoxPoints) > 0 && !cur.vertical && math.Abs(cur.rotation) < 1e-9 {
					right := cur.glyphs[len(cur.glyphs)-1].BBox()[2]
					for _, point := range textBBoxPoints {
						right = math.Max(right, point[0])
					}
					cur.bbox[2] = math.Min(cur.bbox[2], right)
				}
				for _, g := range cur.glyphs {
					if g.Unmapped() || !g.Invisible() {
						if visual == nil {
							v := g.BBox()
							visual = &v
						} else {
							bbox := g.BBox()
							visual[0] = math.Min(visual[0], bbox[0])
							visual[1] = math.Min(visual[1], bbox[1])
							visual[2] = math.Max(visual[2], bbox[2])
							visual[3] = math.Max(visual[3], bbox[3])
						}
					}
				}
				cur.visualBBox = visual
				cur.invisible = visual == nil
			}
			if err := validateTextGeometry(cur); err != nil {
				if reportError != nil {
					reportError(err)
				}
				stopped = true
			} else if len(cur.glyphs) > 0 && emitCurrent {
				var value TextObject
				if layoutValueOnly {
					value = cur.layoutValue()
				} else {
					value = cur.publicValue()
				}
				if !yield(value) {
					stopped = true
				}
			}
		}
		cur = textObjectState{}
		textBBoxPoints = nil
		active = false
	}
	for !stopped {
		op, ok := next()
		if !ok {
			break
		}
		if op.formBoundary != 0 {
			if active {
				flush()
			}
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
		// A pending text object belongs to the state before the next
		// graphics/text-state operator. Flush it before applying that
		// operator so fields such as line join and font size are not borrowed
		// from the following object.
		if active {
			flush()
			if stopped {
				break
			}
		}
		applyGraphicsState(&external, op)
		applyExternalGraphicsState(d, &external, op)
		applyResourceColorSpace(d, &external, op)
		switch op.operatorValue() {
		case "CS":
			strokeSpace = external.strokeColor.Space()
		case "cs":
			nonStrokeSpace = external.fillColor.Space()
		}
		if op.operatorValue() == "gs" {
			lineWidth = external.lineWidth
			strokeAlpha = external.strokeAlpha
			fillAlpha = external.fillAlpha
			blendMode = external.blendMode
			strokeColor = append(strokeColor[:0], external.strokeColor.ValuesCopy()...)
			nonStrokeColor = append(nonStrokeColor[:0], external.fillColor.ValuesCopy()...)
			strokeSpace = external.strokeColor.Space()
			nonStrokeSpace = external.fillColor.Space()
			if external.fontName != "" {
				font = external.fontName
				size = external.fontSize
			}
		}
		if op.operatorValue() == "Tj" || op.operatorValue() == "TJ" || op.operatorValue() == "'" || op.operatorValue() == "\"" {
			emitCurrent = len(restrictOps) == 0
			if !emitCurrent {
				_, emitCurrent = restrictOps[op.operatorValue()]
			}
		}
		switch op.operatorValue() {
		case "BT":
			text = identity()
			textLine = identity()
			glyphOffsetX, glyphOffsetY = 0, 0
		case "BMC":
			if len(op.operandsValue()) > 0 {
				if tag, ok := op.operandsValue()[0].(Name); ok {
					marked = append(marked, markedState{tag: string(tag), mcid: -1, identity: newMarkedContentIdentity()})
				}
			}
		case "BDC":
			if len(op.operandsValue()) < 2 {
				continue
			}
			props, ok := op.operandsValue()[1].(Dict)
			if !ok {
				continue
			}
			m := markedState{mcid: -1, identity: newMarkedContentIdentity()}
			if len(op.operandsValue()) > 0 {
				if tag, ok := op.operandsValue()[0].(Name); ok {
					m.tag = string(tag)
				}
			}
			m.properties = props
			if id, ok := IntValue(props[Name("MCID")]); ok {
				m.mcid, m.hasMCID = id, true
			}
			if text, ok := props[Name("ActualText")].(String); ok {
				m.actualText = decodePDFText(text)
			}
			marked = append(marked, m)
		case "EMC":
			if len(marked) > 0 {
				marked = marked[:len(marked)-1]
			}
		case "q":
			stack = append(stack, state{ctm, font, size, lineWidth, scaling, rise, charSpace, wordSpace, leading, renderMode, strokeAlpha, fillAlpha, blendMode, strokeColor, nonStrokeColor, strokeSpace, nonStrokeSpace, external.Clone()})
		case "Q":
			if len(stack) > 0 {
				s := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				ctm = s.ctm
				font = s.font
				size = s.size
				lineWidth = s.lineWidth
				scaling = s.scaling
				rise = s.rise
				charSpace = s.charSpace
				wordSpace = s.wordSpace
				leading = s.leading
				renderMode = s.renderMode
				strokeAlpha = s.strokeAlpha
				fillAlpha = s.fillAlpha
				blendMode = s.blendMode
				strokeColor = s.stroke
				nonStrokeColor = s.nonStroke
				strokeSpace = s.strokeSpace
				nonStrokeSpace = s.nonStrokeSpace
				external = s.external
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
					}
				}
			}
		case "ET":
			flush()
		case "Tf":
			flush()
			if len(op.operandsValue()) >= 2 {
				if n, ok := op.operandsValue()[0].(Name); ok {
					font = string(n)
				}
				if value, ok := finiteNumberValue(op.operandsValue()[1]); ok {
					size = value
				}
			}
		case "gs":
			if external.fontName != "" {
				font = external.fontName
				size = external.fontSize
			}
		case "Tm":
			flush()
			if len(op.operandsValue()) >= 6 {
				var v [6]float64
				valid := true
				for i := range v {
					v[i], valid = finiteNumberValue(op.operandsValue()[i])
					if !valid {
						break
					}
				}
				if valid {
					text = geometry.Matrix(v)
					textLine = text
					glyphOffsetX, glyphOffsetY = 0, 0
				}
			}
		case "Td":
			flush()
			if len(op.operandsValue()) >= 2 {
				x, xOK := finiteNumberValue(op.operandsValue()[0])
				y, yOK := finiteNumberValue(op.operandsValue()[1])
				if xOK && yOK {
					moveTextLine(x, y)
				}
			}
		case "TD":
			flush()
			if len(op.operandsValue()) >= 2 {
				x, xOK := finiteNumberValue(op.operandsValue()[0])
				y, yOK := finiteNumberValue(op.operandsValue()[1])
				if xOK && yOK {
					leading = -y
					moveTextLine(x, y)
				}
			}
		case "Tc":
			flush()
			if len(op.operandsValue()) > 0 {
				if value, ok := finiteNumberValue(op.operandsValue()[0]); ok {
					charSpace = value
				}
			}
		case "Tw":
			flush()
			if len(op.operandsValue()) > 0 {
				if value, ok := finiteNumberValue(op.operandsValue()[0]); ok {
					wordSpace = value
				}
			}
		case "TL":
			flush()
			if len(op.operandsValue()) > 0 {
				if value, ok := finiteNumberValue(op.operandsValue()[0]); ok {
					leading = value
				}
			}
		case "Tz":
			flush()
			if len(op.operandsValue()) > 0 {
				if value, ok := finiteNumberValue(op.operandsValue()[0]); ok {
					scaling = value / 100
				}
			}
		case "Tr":
			flush()
			renderMode = external.renderMode
		case "Ts":
			flush()
			if len(op.operandsValue()) > 0 {
				if value, ok := finiteNumberValue(op.operandsValue()[0]); ok {
					rise = value
				}
			}
		case "CA":
			strokeAlpha = external.strokeAlpha
		case "ca":
			fillAlpha = external.fillAlpha
		case "BM":
			// applyGraphicsState performs the atomic validation for names and
			// blend-mode arrays. Reuse that result instead of partially
			// interpreting a malformed array here.
			blendMode = external.blendMode
		case "w":
			if len(op.operandsValue()) > 0 {
				if value, ok := finiteNumberValue(op.operandsValue()[0]); ok {
					lineWidth = value
				}
			}
		case "G":
			if !containsNonFiniteNumber(op.operandsValue()) {
				strokeColor = colorNumbers(op.operandsValue(), 1)
				strokeSpace = "DeviceGray"
			}
		case "g":
			if !containsNonFiniteNumber(op.operandsValue()) {
				nonStrokeColor = colorNumbers(op.operandsValue(), 1)
				nonStrokeSpace = "DeviceGray"
			}
		case "RG":
			if !containsNonFiniteNumber(op.operandsValue()) {
				strokeColor = colorNumbers(op.operandsValue(), 3)
				strokeSpace = "DeviceRGB"
			}
		case "rg":
			if !containsNonFiniteNumber(op.operandsValue()) {
				nonStrokeColor = colorNumbers(op.operandsValue(), 3)
				nonStrokeSpace = "DeviceRGB"
			}
		case "K":
			if !containsNonFiniteNumber(op.operandsValue()) {
				strokeColor = colorNumbers(op.operandsValue(), 4)
				strokeSpace = "DeviceCMYK"
			}
		case "k":
			if !containsNonFiniteNumber(op.operandsValue()) {
				nonStrokeColor = colorNumbers(op.operandsValue(), 4)
				nonStrokeSpace = "DeviceCMYK"
			}
		case "CS":
			if len(op.operandsValue()) > 0 {
				if name, ok := op.operandsValue()[0].(Name); ok {
					strokeSpace = string(name)
					applyGraphicsState(&external, op)
					applyResourceColorSpace(d, &external, op)
					strokeSpace = external.strokeColor.Space()
					strokeComponents = external.strokeColor.Components()
				}
			}
		case "cs":
			if len(op.operandsValue()) > 0 {
				if name, ok := op.operandsValue()[0].(Name); ok {
					nonStrokeSpace = string(name)
					applyGraphicsState(&external, op)
					applyResourceColorSpace(d, &external, op)
					nonStrokeSpace = external.fillColor.Space()
					nonStrokeComponents = external.fillColor.Components()
				}
			}
		case "SC", "SCN":
			if !containsNonFiniteNumber(op.operandsValue()) {
				strokeColor = colorNumbersForSpace(op.operandsValue(), strokeSpace, strokeComponents)
			}
		case "sc", "scn":
			if !containsNonFiniteNumber(op.operandsValue()) {
				nonStrokeColor = colorNumbersForSpace(op.operandsValue(), nonStrokeSpace, nonStrokeComponents)
			}
		case "Tj", "'", "\"":
			if op.operatorValue() == "'" || op.operatorValue() == "\"" {
				flush()
				moveTextLine(0, -leading)
			}
			if op.operatorValue() == "\"" && len(op.operandsValue()) >= 3 {
				wordValue, wordOK := finiteNumberValue(op.operandsValue()[0])
				charValue, charOK := finiteNumberValue(op.operandsValue()[1])
				if wordOK && charOK {
					wordSpace = wordValue
					charSpace = charValue
				}
			}
			if len(op.operandsValue()) > 0 {
				if s, ok := op.operandsValue()[len(op.operandsValue())-1].(String); ok {
					if !active {
						var valid bool
						cur, valid = newTextObject()
						if !valid {
							if reportError != nil {
								reportError(fmt.Errorf("playa: non-finite text origin"))
							}
							stopped = true
							continue
						}
						active = true
					}
					appendTextArg(&cur, s, copyTextArgs)
					endOffset := [2]float64{}
					_, err := appendFontStringWithError(&cur, textLine, glyphOffsetX, glyphOffsetY, s, font, size, lookup, rise, scaling, charSpace, wordSpace, ctm, renderMode == 3 || scaling <= .02, fontTextNames, &glyphScratch, &endOffset)
					if err != nil {
						if reportError != nil {
							reportError(err)
						}
						stopped = true
						continue
					}
					glyphOffsetX, glyphOffsetY = endOffset[0], endOffset[1]
					if !updateTextFromGlyphOffset() {
						if reportError != nil {
							reportError(fmt.Errorf("playa: non-finite text offset"))
						}
						stopped = true
					}
				}
			}
		case "TJ":
			tjVertical := isVerticalFont(font, lookup)
			currentTJText := func() (geometry.Matrix, bool) {
				return textLine.OffsetFinite(glyphOffsetX, glyphOffsetY)
			}
			addTJTextBBoxPoint := func() bool {
				current, ok := currentTJText()
				if !ok {
					return false
				}
				x, y, ok := ctm.PointFinite(current[4], current[5]+rise)
				if !ok {
					return false
				}
				textBBoxPoints = append(textBBoxPoints, [2]float64{x, y})
				return true
			}
			if len(op.operandsValue()) > 0 {
				if a, ok := op.operandsValue()[0].(Array); ok {
					// A TJ array can append many short strings to one text object.
					// Reserve their combined glyph storage once to avoid copying
					// the growing GlyphObject slice for each fragment.
					glyphHint := 0
					if len(a) > 4 {
						for _, item := range a {
							if value, ok := item.(String); ok && glyphHint < 1<<14 {
								remaining := 1<<14 - glyphHint
								if len(value) > remaining {
									glyphHint = 1 << 14
								} else {
									glyphHint += len(value)
								}
							}
						}
					}
					reservedGlyphs := false
					var leadingArgs []Object
					for itemIndex, item := range a {
						switch x := item.(type) {
						case String:
							if !active {
								var valid bool
								cur, valid = newTextObject()
								if !valid {
									if reportError != nil {
										reportError(fmt.Errorf("playa: non-finite text origin"))
									}
									stopped = true
									continue
								}
								for _, arg := range leadingArgs {
									appendTextArg(&cur, arg, copyTextArgs)
								}
								leadingArgs = nil
								active = true
							}
							appendTextArg(&cur, x, copyTextArgs)
							if _, textOK := currentTJText(); !textOK {
								if reportError != nil {
									reportError(fmt.Errorf("playa: non-finite TJ text offset"))
								}
								stopped = true
								continue
							}
							if !reservedGlyphs && glyphHint > 0 {
								reserve := glyphHint
								if lookup != nil {
									if resolved := lookup(font); resolved != nil && resolved.cid {
										reserve = (reserve + 1) / 2
									}
								}
								cur.glyphs = slices.Grow(cur.glyphs, reserve)
								reservedGlyphs = true
							}
							endOffset := [2]float64{}
							_, err := appendFontStringWithError(&cur, textLine, glyphOffsetX, glyphOffsetY, x, font, size, lookup, rise, scaling, charSpace, wordSpace, ctm, renderMode == 3 || scaling <= .02, fontTextNames, &glyphScratch, &endOffset)
							if err != nil {
								if reportError != nil {
									reportError(err)
								}
								stopped = true
								continue
							}
							glyphOffsetX, glyphOffsetY = endOffset[0], endOffset[1]
						case Number:
							if !active && !hasTextObjectStart {
								textObjectStart = text
								hasTextObjectStart = true
							}
							if !active || len(cur.glyphs) == 0 {
								if !addTJTextBBoxPoint() {
									if reportError != nil {
										reportError(fmt.Errorf("playa: non-finite text adjustment point"))
									}
									stopped = true
									continue
								}
							}
							if active {
								appendTextArg(&cur, x, copyTextArgs)
							} else {
								leadingArgs = append(leadingArgs, x)
							}
							adjust := math.FMA(math.FMA(float64(x), 0.001, 0), size, 0)
							if tjVertical {
								glyphOffsetY -= adjust
							} else {
								glyphOffsetX -= math.FMA(adjust, scaling, 0)
							}
							if tjHasFollowingString(a, itemIndex+1) {
								if !addTJTextBBoxPoint() {
									if reportError != nil {
										reportError(fmt.Errorf("playa: non-finite text adjustment point"))
									}
									stopped = true
									continue
								}
							}
							if !stopped {
								var textOK bool
								text, textOK = currentTJText()
								if !textOK {
									if reportError != nil {
										reportError(fmt.Errorf("playa: non-finite TJ text offset"))
									}
									stopped = true
								}
							}
						}
					}
					if active && !stopped {
						var textOK bool
						text, textOK = currentTJText()
						if !textOK {
							if reportError != nil {
								reportError(fmt.Errorf("playa: non-finite TJ text offset"))
							}
							stopped = true
						}
					}
					if !active {
						textBBoxPoints = nil
						hasTextObjectStart = false
					}
				}
			}
		case "T*":
			flush()
			moveTextLine(0, -leading)
		}
	}
	flush()
}

func tjHasFollowingString(a Array, start int) bool {
	for _, item := range a[start:] {
		if _, ok := item.(String); ok {
			return true
		}
	}
	return false
}

func isVerticalFont(name string, lookup func(string) *Font) bool {
	font := (*Font)(nil)
	if lookup != nil {
		font = lookup(name)
	}
	return font != nil && font.vertical
}

func numbers(a []Object) []float64 {
	out := make([]float64, 0, len(a))
	for _, x := range a {
		if n, ok := finiteNumberValue(x); ok {
			out = append(out, n)
		}
	}
	return out
}

func containsNonFiniteNumber(a []Object) bool {
	for _, x := range a {
		if value, ok := NumberValue(x); ok && (math.IsNaN(value) || math.IsInf(value, 0)) {
			return true
		}
	}
	return false
}

func colorNumbers(a []Object, components int) []float64 {
	out := numbers(a)
	for len(out) < components {
		out = append(out, 0)
	}
	return out[:components]
}

func colorNumbersForSpace(a []Object, space string, components int) []float64 {
	out := numbers(a)
	if space == "Pattern" && len(a) > 0 {
		if _, ok := a[len(a)-1].(Name); ok {
			components--
		}
	}
	if components <= 0 {
		return out
	}
	for len(out) < components {
		out = append(out, 0)
	}
	return out[:components]
}

func appendTextArg(cur *textObjectState, value Object, copyValue bool) {
	if text, ok := value.(String); ok && copyValue {
		var owned String
		if len(text) > 0 {
			owned = String(cloneObjectBytes(text))
		}
		value = owned
	}
	cur.args = append(cur.args, value)
}

func textRenderingMatrix(ctm, text, scaling geometry.Matrix) (geometry.Matrix, bool) {
	intermediate, ok := ctm.MulFinite(text)
	if !ok {
		return geometry.Matrix{}, false
	}
	return intermediate.MulFinite(scaling)
}

func validateTextGeometry(text textObjectState) error {
	for _, value := range text.matrix {
		if !cffFiniteValues(value) {
			return fmt.Errorf("playa: non-finite text matrix")
		}
	}
	if !cffFiniteValues(text.origin[0], text.origin[1], text.displacement[0], text.displacement[1], text.rotation) {
		return fmt.Errorf("playa: non-finite text geometry")
	}
	for _, value := range text.bbox {
		if !cffFiniteValues(value) {
			return fmt.Errorf("playa: non-finite text bbox")
		}
	}
	for _, glyph := range text.glyphs {
		origin := glyph.Origin()
		displacement := glyph.Displacement()
		if !cffFiniteValues(origin[0], origin[1], displacement[0], displacement[1]) {
			return fmt.Errorf("playa: non-finite glyph geometry")
		}
		for _, value := range glyph.Matrix() {
			if !cffFiniteValues(value) {
				return fmt.Errorf("playa: non-finite glyph matrix")
			}
		}
		for _, value := range glyph.BBox() {
			if !cffFiniteValues(value) {
				return fmt.Errorf("playa: non-finite glyph bbox")
			}
		}
		for _, value := range glyph.outlineMatrix() {
			if !cffFiniteValues(value) {
				return fmt.Errorf("playa: non-finite derived glyph matrix")
			}
		}
	}
	return nil
}

type textFontNameKey struct {
	base string
	size float64
}

func appendFontStringWithError(cur *textObjectState, text geometry.Matrix, offsetX, offsetY float64, raw []byte, fontName string, size float64, lookup func(string) *Font, rise, scaling, charSpace, wordSpace float64, ctm geometry.Matrix, forceInvisible bool, fontTextNames map[textFontNameKey]string, glyphScratch *[]DecodedGlyph, endOffset *[2]float64) (float64, error) {
	font := (*Font)(nil)
	if lookup != nil {
		font = lookup(fontName)
	}
	_, y, ok := text.PointFinite(0, rise)
	if !ok {
		return 0, fmt.Errorf("playa: non-finite text baseline")
	}
	advanced := 0.0
	glyphs := (*glyphScratch)[:0]
	if font != nil {
		for glyph, err := range font.DecodeGlyphsSeqWithError(raw) {
			if err != nil {
				return 0, err
			}
			glyphs = append(glyphs, glyph)
		}
	} else {
		for _, r := range raw {
			glyphs = append(glyphs, fontdata.BorrowedDecodedGlyph(string(r), []byte{r}, int(r)))
		}
	}
	var fontState *fontWidthState
	if font != nil {
		fontState = font.interpreterWidthState()
	}
	render, ok := ctm.MulFinite(text)
	if !ok {
		return 0, fmt.Errorf("playa: non-finite text rendering matrix")
	}
	glyphSize := math.Abs(size)
	if size != 0 {
		axisX, axisY := render[2], render[3]
		if font != nil && font.vertical {
			// Playa measures vertical text along the first rendering-matrix
			// axis; horizontal text uses the second axis.
			axisX, axisY = render[0], render[1]
		}
		glyphSize *= math.Hypot(axisX, axisY)
	}
	if !cffFiniteValues(glyphSize) {
		return 0, fmt.Errorf("playa: non-finite glyph size")
	}
	fontOut := fontName
	if font != nil && font.name != "" {
		fontOut = font.name
	}
	fontBase := fontBaseName(fontOut)
	glyphTextFont := cachedTextFontName(fontTextNames, fontBase, glyphSize)
	cur.glyphFontName = fontOut
	cur.glyphFontSize = glyphSize
	cur.glyphFontBase = fontBase
	cur.glyphTextFont = glyphTextFont
	cur.glyphVertical = font != nil && font.vertical
	if len(glyphs) > 1 {
		cur.glyphs = slices.Grow(cur.glyphs, len(glyphs))
	}
	for _, glyph := range glyphs {
		code := glyph.BytesBorrowed()
		glyphText := glyph.Text()
		cid := glyph.CID()
		if font != nil && font.type3 && glyphText == "" && len(code) == 1 && code[0] == 32 {
			// Type 3 fonts may leave code 32 unmapped while using it as the
			// explicit word-separator code in text-showing operations.
			glyphText = " "
		}
		if font != nil && font.type3 {
			if err := font.ensureType3GlyphWithError(code); err != nil {
				return 0, err
			}
		}
		r, _ := utf8.DecodeRuneInString(glyphText)
		displacement := 0.5
		if fontState != nil {
			displacement = fontState.hDisp(cid)
		} else if font != nil {
			var err error
			displacement, err = font.HDispWithError(cid)
			if err != nil {
				return 0, err
			}
		}
		// Playa scales the font displacement only after resolving the
		// font-space width. Preserve that operation order; multiplying the
		// raw thousandths width by the font size first can change a later
		// layout-group distance by one ULP.
		w := math.FMA(math.FMA(displacement, size, 0), scaling, 0)
		originX, originY, ok := text.PointFinite(offsetX, offsetY+rise)
		if !ok {
			return 0, fmt.Errorf("playa: non-finite text origin")
		}
		glyphX, glyphY := originX, originY
		vertical := font != nil && font.vertical
		if vertical {
			position := font.Position(cid)
			glyphX, glyphY, ok = text.PointFinite(offsetX-size*position[0]*scaling, offsetY+rise-size*position[1])
			if !ok {
				return 0, fmt.Errorf("playa: non-finite vertical text origin")
			}
		}
		reportX, reportY, ok := playaGlyphMatrixOrigin(render, offsetX, offsetY, rise)
		if !ok {
			return 0, fmt.Errorf("playa: non-finite reported text origin")
		}
		x2, y2 := reportX, reportY
		if vertical {
			x2, y2, ok = ctm.PointFinite(glyphX, glyphY)
			if !ok {
				return 0, fmt.Errorf("playa: non-finite transformed glyph origin")
			}
		}
		left := 0.0
		bottom, top := y, y+size
		if font != nil && (font.ascent != 0 || font.descent != 0) {
			fontScaleY := 0.001
			if font.type3 {
				fontScaleY = font.fontMatrix[3]
			}
			bottom = y + size*font.descent*fontScaleY
			top = y + size*font.ascent*fontScaleY
		}
		if font != nil && font.type3 && len(code) > 0 {
			if bbox, ok := font.GlyphBBox(code); ok {
				fontScaleX, fontScaleY := font.fontMatrix[0], font.fontMatrix[3]
				left = size * bbox[0] * fontScaleX
				bottom = y + size*bbox[1]*fontScaleY
				top = y + size*bbox[3]*fontScaleY
			}
		}
		// Playa computes GlyphObject.bbox from all four corners of the
		// transformed standard glyph rectangle when the text is rotated or
		// skewed.  Using only the diagonal corners loses the overhang on
		// oblique CTMs (notably on the sparse invisible-space glyphs in some
		// Office PDFs).
		// `text` carries the text-space rotation/skew/scale.  The current
		// glyph origin above already includes rise, so apply the linear part
		// of the complete rendering matrix to the standard font rectangle
		// relative to that origin.
		fontBottom := bottom - y
		fontTop := top - y
		if vertical {
			fontBottom = size * font.fontMatrix[3] * font.descent
			fontTop = size * font.fontMatrix[3] * font.ascent
		}
		fontRight := w
		if font != nil && font.type3 {
			if bbox, ok := font.GlyphBBox(code); ok {
				left = size * bbox[0] * font.fontMatrix[0]
			}
		}
		transformGlyphPoint := func(dx, dy float64) (float64, float64) {
			return x2 + render[0]*dx + render[2]*dy, y2 + render[1]*dx + render[3]*dy
		}
		corners := [4][2]float64{}
		corners[0][0], corners[0][1] = transformGlyphPoint(left, fontBottom)
		corners[1][0], corners[1][1] = transformGlyphPoint(left, fontTop)
		corners[2][0], corners[2][1] = transformGlyphPoint(fontRight, fontTop)
		corners[3][0], corners[3][1] = transformGlyphPoint(fontRight, fontBottom)
		minX, minY := corners[0][0], corners[0][1]
		maxX, maxY := minX, minY
		for _, corner := range corners[1:] {
			if !cffFiniteValues(corner[0], corner[1]) {
				return 0, fmt.Errorf("playa: non-finite glyph bounds")
			}
			minX, maxX = min(minX, corner[0]), max(maxX, corner[0])
			minY, maxY = min(minY, corner[1]), max(maxY, corner[1])
		}
		unmapped := false
		if font != nil && font.cid {
			if fontState != nil {
				unmapped = !fontState.hasUnicodeMapping(code, cid)
			} else {
				unmapped = !font.hasUnicodeMapping(code, cid)
			}
		}
		invisible := cur.invisible || forceInvisible
		if !unmapped && (unicode.IsSpace(r) || unicode.IsControl(r) || unicode.In(r, unicode.Cf)) {
			invisible = true
		}
		stepX, stepY := w+charSpace*scaling, 0.0
		if font != nil {
			if size == 0 {
				stepX = 0
			} else {
				stepDisplacement := displacement + charSpace/size
				if cid == 32 {
					stepDisplacement += wordSpace / size
				}
				stepX = math.FMA(math.FMA(stepDisplacement, size, 0), scaling, 0)
			}
		} else if wordSpacingApplies(code) {
			stepX += wordSpace * scaling
		}
		if vertical {
			stepX = 0
			stepY = 0
			if size != 0 {
				verticalDisplacement, err := font.VDispWithError(cid)
				if err != nil {
					return 0, err
				}
				stepDisplacement := verticalDisplacement + charSpace/size
				if cid == 32 {
					stepDisplacement += wordSpace / size
				}
				stepY = math.FMA(stepDisplacement, size, 0)
			}
		}
		dx := render[0]*stepX + render[2]*stepY
		dy := render[1]*stepX + render[3]*stepY
		matrixX, matrixY := x2, y2
		if vertical {
			// Playa anchors a vertical glyph matrix at the text origin. The
			// vertical origin adjustment belongs to the glyph bbox.
			matrixX, matrixY = reportX, reportY
		}
		glyphMatrix := geometry.Matrix{render[0] * size * scaling, render[1] * size * scaling, render[2] * size, render[3] * size, matrixX, matrixY}
		if !cffFiniteValues(dx, dy, reportX, reportY) {
			return 0, fmt.Errorf("playa: non-finite glyph displacement")
		}
		for _, value := range glyphMatrix {
			if !cffFiniteValues(value) {
				return 0, fmt.Errorf("playa: non-finite glyph matrix")
			}
		}
		if !vertical {
			reportX, reportY = x2, y2
		}
		gid := 0
		if fontState != nil {
			gid = fontState.glyphID(code, cid)
		} else if font != nil {
			gid = font.glyphIDForCode(code, cid)
		}
		glyphObject := newGlyphObjectWithContext(contentdata.GlyphSpec{
			Text: glyphText, Chars: glyphText, Page: cur.page, HasPage: cur.hasPage,
			MCID: cur.mcid, HasMCID: cur.hasMCID, Code: code, CID: cid, GID: gid,
			FontName: fontOut, FontSize: glyphSize, Size: glyphSize, FontBase: fontBase,
			TextFont: glyphTextFont, Matrix: glyphMatrix, Origin: [2]float64{reportX, reportY},
			Displacement: [2]float64{dx, dy}, BBox: [4]float64{minX, minY, maxX, maxY},
			Vertical: vertical, Unmapped: unmapped, Invisible: invisible,
		}, nil, font, nil, geometry.Matrix{})
		if font != nil && font.type3 {
			if ops, ok := font.type3GlyphOpsSnapshot(code); ok {
				matrix, ok := glyphMatrix.MulFinite(font.fontMatrix)
				if !ok {
					return 0, fmt.Errorf("playa: non-finite derived glyph matrix")
				}
				glyphObject.setOutline(ops, matrix)
			}
		}
		if font != nil && len(font.cffCharstrings) > 0 {
			matrix, ok := glyphMatrix.MulFinite(font.fontMatrix)
			if !ok {
				return 0, fmt.Errorf("playa: non-finite derived glyph matrix")
			}
			glyphObject.setOutline(nil, matrix)
		}
		cur.glyphs = append(cur.glyphs, glyphObject)
		offsetX += stepX
		offsetY += stepY
		if !cffFiniteValues(offsetX, offsetY) {
			return 0, fmt.Errorf("playa: non-finite glyph text offset")
		}
		advanced += stepX
		if vertical {
			advanced += stepY
		}
	}
	*glyphScratch = glyphs[:0]
	if endOffset != nil {
		*endOffset = [2]float64{offsetX, offsetY}
	}
	return advanced, nil
}

// playaGlyphMatrixOrigin preserves the two affine stages used by Playa's
// TextObject iterator: first translate the already-composed text-line/CTM
// matrix by the glyph offset, then apply text rise. Collapsing these into a
// text-space Point followed by the CTM is mathematically equivalent but can
// move a layout boundary by one binary64 ULP.
func playaGlyphMatrixOrigin(render geometry.Matrix, offsetX, offsetY, rise float64) (float64, float64, bool) {
	xOffset := math.FMA(offsetX, render[0], 0) + math.FMA(offsetY, render[2], 0)
	yOffset := math.FMA(offsetX, render[1], 0) + math.FMA(offsetY, render[3], 0)
	x := xOffset + render[4]
	y := yOffset + render[5]
	x = math.FMA(rise, render[2], 0) + x
	y = math.FMA(rise, render[3], 0) + y
	return x, y, cffFiniteValues(x, y)
}

func wordSpacingApplies(code []byte) bool {
	return len(code) == 1 && code[0] == ' '
}

func fontBaseName(name string) string {
	if separator := strings.LastIndexByte(name, '+'); separator >= 0 {
		return name[separator+1:]
	}
	return name
}

func textFontName(base string, size float64) string {
	return base + " " + strconv.FormatFloat(math.RoundToEven(size), 'f', 0, 64)
}

func cachedTextFontName(cache map[textFontNameKey]string, base string, size float64) string {
	key := textFontNameKey{base: base, size: math.RoundToEven(size)}
	if value, ok := cache[key]; ok {
		return value
	}
	value := textFontName(base, size)
	cache[key] = value
	return value
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
