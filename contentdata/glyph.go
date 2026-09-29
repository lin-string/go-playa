package contentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// Glyph is the dependency-free value portion of one decoded glyph. Font
// resolution, lazy outline generation, and Type3 content traversal remain
// owned by document.GlyphObject.
type Glyph struct {
	text         string
	chars        string
	code         []byte
	cid          int
	gid          int
	matrix       geometry.Matrix
	origin       [2]float64
	displacement [2]float64
	bbox         [4]float64
	unmapped     bool
	invisible    bool
	context      *GlyphContext
}

// GlyphContext is immutable graphics and marked-content state shared by
// glyphs from one text object. Its fields remain private so sharing does not
// expose mutable document state.
type GlyphContext struct {
	page        primitives.Ref
	hasPage     bool
	mcid        int
	hasMCID     bool
	fontName    string
	fontSize    float64
	size        float64
	fontBase    string
	textFont    string
	vertical    bool
	gstate      GraphicsState
	markedStack []MarkedContentContext
}

// GlyphContextSpec supplies immutable state shared by glyphs from one text
// object.
type GlyphContextSpec struct {
	Page        primitives.Ref
	HasPage     bool
	MCID        int
	HasMCID     bool
	FontName    string
	FontSize    float64
	Size        float64
	FontBase    string
	TextFont    string
	Vertical    bool
	GState      GraphicsState
	MarkedStack []MarkedContentContext
}

// NewGlyphContextBorrowed constructs a shared context over caller-owned
// backing values. Finalize a glyph before retaining it past the ownership
// boundary of the graphics state or marked-content stack in spec.
func NewGlyphContextBorrowed(spec GlyphContextSpec) *GlyphContext {
	return &GlyphContext{
		page: spec.Page, hasPage: spec.HasPage, mcid: spec.MCID, hasMCID: spec.HasMCID,
		fontName: spec.FontName, fontSize: spec.FontSize, size: spec.Size,
		fontBase: spec.FontBase, textFont: spec.TextFont, vertical: spec.Vertical,
		gstate: spec.GState, markedStack: spec.MarkedStack,
	}
}

// SpecBorrowed returns the complete shared construction view without copying
// mutable backing values.
func (c *GlyphContext) SpecBorrowed() GlyphContextSpec {
	return GlyphContextSpec{
		Page: c.page, HasPage: c.hasPage, MCID: c.mcid, HasMCID: c.hasMCID,
		FontName: c.fontName, FontSize: c.fontSize, Size: c.size,
		FontBase: c.fontBase, TextFont: c.textFont, Vertical: c.vertical,
		GState: c.gstate, MarkedStack: c.markedStack,
	}
}

// GlyphSpec supplies the dependency-free fields used to construct a Glyph.
type GlyphSpec struct {
	Text         string
	Chars        string
	Page         primitives.Ref
	HasPage      bool
	MCID         int
	HasMCID      bool
	Code         []byte
	CID          int
	GID          int
	FontName     string
	FontSize     float64
	Size         float64
	FontBase     string
	TextFont     string
	Matrix       geometry.Matrix
	Origin       [2]float64
	Displacement [2]float64
	BBox         [4]float64
	Vertical     bool
	Unmapped     bool
	Invisible    bool
	GState       GraphicsState
	MarkedStack  []MarkedContentContext
}

// NewGlyph constructs an owned glyph value.
func NewGlyph(spec GlyphSpec) Glyph {
	owned := spec
	owned.Code = primitives.CloneBytes(spec.Code)
	context := glyphContextSpec(spec)
	context.GState = spec.GState.Finalize()
	context.MarkedStack = cloneMarkedStack(spec.MarkedStack)
	return newGlyphWithContext(owned, NewGlyphContextBorrowed(context))
}

func newGlyphWithContext(spec GlyphSpec, context *GlyphContext) Glyph {
	return Glyph{
		text: spec.Text, chars: spec.Chars, code: spec.Code,
		cid: spec.CID, gid: spec.GID,
		matrix: spec.Matrix, origin: spec.Origin, displacement: spec.Displacement,
		bbox: spec.BBox, unmapped: spec.Unmapped,
		invisible: spec.Invisible, context: context,
	}
}

func glyphContextSpec(spec GlyphSpec) GlyphContextSpec {
	return GlyphContextSpec{
		Page: spec.Page, HasPage: spec.HasPage, MCID: spec.MCID, HasMCID: spec.HasMCID,
		FontName: spec.FontName, FontSize: spec.FontSize, Size: spec.Size,
		FontBase: spec.FontBase, TextFont: spec.TextFont, Vertical: spec.Vertical,
		GState: spec.GState, MarkedStack: spec.MarkedStack,
	}
}

// NewGlyphBorrowed constructs a low-allocation value over slices owned by the
// caller. Use CodeCopy, MarkedStackCopy, or Finalize before retaining it.
func NewGlyphBorrowed(spec GlyphSpec) Glyph {
	return NewGlyphBorrowedWithContext(spec, NewGlyphContextBorrowed(glyphContextSpec(spec)))
}

// NewGlyphBorrowedWithContext constructs a low-allocation glyph that shares
// an immutable borrowed context. A nil context represents zero graphics state
// and an empty marked-content stack.
func NewGlyphBorrowedWithContext(spec GlyphSpec, context *GlyphContext) Glyph {
	return newGlyphWithContext(spec, context)
}

func (g Glyph) Text() string  { return g.text }
func (g Glyph) Chars() string { return g.chars }
func (g Glyph) Page() primitives.Ref {
	if g.context == nil {
		return primitives.Ref{}
	}
	return g.context.page
}
func (g Glyph) HasPage() bool {
	return g.context != nil && g.context.hasPage
}
func (g Glyph) MCID() int {
	if g.context == nil {
		return 0
	}
	return g.context.mcid
}
func (g Glyph) HasMCID() bool {
	return g.context != nil && g.context.hasMCID
}
func (g Glyph) CodeCopy() []byte { return primitives.CloneBytes(g.code) }
func (g Glyph) CID() int         { return g.cid }
func (g Glyph) GID() int         { return g.gid }
func (g Glyph) FontName() string {
	if g.context == nil {
		return ""
	}
	return g.context.fontName
}
func (g Glyph) FontSize() float64 {
	if g.context == nil {
		return 0
	}
	return g.context.fontSize
}
func (g Glyph) Size() float64 {
	if g.context == nil {
		return 0
	}
	return g.context.size
}
func (g Glyph) FontBase() string {
	if g.context == nil {
		return ""
	}
	return g.context.fontBase
}
func (g Glyph) TextFont() string {
	if g.context == nil {
		return ""
	}
	return g.context.textFont
}
func (g Glyph) Matrix() geometry.Matrix  { return g.matrix }
func (g Glyph) Origin() [2]float64       { return g.origin }
func (g Glyph) Displacement() [2]float64 { return g.displacement }
func (g Glyph) BBox() [4]float64         { return g.bbox }
func (g Glyph) Vertical() bool           { return g.context != nil && g.context.vertical }
func (g Glyph) Unmapped() bool           { return g.unmapped }
func (g Glyph) Invisible() bool          { return g.invisible }
func (g Glyph) GState() GraphicsState {
	if g.context == nil {
		return GraphicsState{}
	}
	return g.context.gstate.Finalize()
}
func (g Glyph) MarkedStackCopy() []MarkedContentContext {
	if g.context == nil {
		return nil
	}
	return cloneMarkedStack(g.context.markedStack)
}

// ContextBorrowed returns the immutable shared construction context. It is
// intended only for low-allocation adapters rebuilding the same glyph value.
func (g Glyph) ContextBorrowed() *GlyphContext { return g.context }

// SpecBorrowed returns the complete construction view without copying mutable
// backing values. It is intended for low-allocation adapters that immediately
// build another borrowed Glyph. Use Finalize or the Copy accessors before
// retaining or modifying Code, GState, or MarkedStack.
func (g Glyph) SpecBorrowed() GlyphSpec {
	var context GlyphContextSpec
	if g.context != nil {
		context = g.context.SpecBorrowed()
	}
	return GlyphSpec{
		Text: g.text, Chars: g.chars, Page: context.Page, HasPage: context.HasPage,
		MCID: context.MCID, HasMCID: context.HasMCID, Code: g.code, CID: g.cid, GID: g.gid,
		FontName: context.FontName, FontSize: context.FontSize, Size: context.Size, FontBase: context.FontBase,
		TextFont: context.TextFont, Matrix: g.matrix, Origin: g.origin, Displacement: g.displacement,
		BBox: g.bbox, Vertical: context.Vertical, Unmapped: g.unmapped, Invisible: g.invisible,
		GState: context.GState, MarkedStack: context.MarkedStack,
	}
}

// Finalize returns an independent glyph snapshot.
func (g Glyph) Finalize() Glyph {
	return NewGlyph(g.SpecBorrowed())
}

func (g Glyph) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Text         string
		Chars        string
		Page         primitives.Ref
		HasPage      bool
		MCID         int
		HasMCID      bool
		Code         []byte
		CID          int
		GID          int
		FontName     string
		FontSize     float64
		Size         float64
		FontBase     string
		TextFont     string
		Matrix       geometry.Matrix
		Origin       [2]float64
		Displacement [2]float64
		BBox         [4]float64
		Vertical     bool
		Unmapped     bool
		Invisible    bool
		GState       GraphicsState
		MarkedStack  []MarkedContentContext
	}{
		Text: g.text, Chars: g.chars, Page: g.Page(), HasPage: g.HasPage(), MCID: g.MCID(),
		HasMCID: g.HasMCID(), Code: g.CodeCopy(), CID: g.cid, GID: g.gid,
		FontName: g.FontName(), FontSize: g.FontSize(), Size: g.Size(), FontBase: g.FontBase(),
		TextFont: g.TextFont(), Matrix: g.matrix, Origin: g.origin, Displacement: g.displacement,
		BBox: g.bbox, Vertical: g.Vertical(), Unmapped: g.unmapped, Invisible: g.invisible,
		GState: g.GState(), MarkedStack: g.MarkedStackCopy(),
	})
}
