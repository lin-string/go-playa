package contentdata

import (
	"encoding/json"
	"iter"

	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// Text is the dependency-free value portion of one interpreted text object.
// Font resolution and glyph outline/content traversal remain owned by
// document.TextObject.
type Text struct {
	text             string
	page             primitives.Ref
	hasPage          bool
	chars            string
	fontName         string
	fontSize         float64
	size             float64
	fontBase         string
	textFont         string
	args             []primitives.Object
	matrix           geometry.Matrix
	textMatrix       geometry.Matrix
	lineMatrix       geometry.Matrix
	scalingMatrix    geometry.Matrix
	origin           [2]float64
	displacement     [2]float64
	rotation         float64
	bbox             [4]float64
	visualBBox       *[4]float64
	lineWidth        float64
	strokeColor      []float64
	nonStrokeColor   []float64
	invisible        bool
	vertical         bool
	unmapped         bool
	gstate           GraphicsState
	markedTag        string
	markedProperties primitives.Dict
	markedStack      []MarkedContentContext
	actualText       string
	mcid             int
	hasMCID          bool
}

// TextSpec supplies the dependency-free fields used to construct a Text.
type TextSpec struct {
	Text             string
	Page             primitives.Ref
	HasPage          bool
	Chars            string
	FontName         string
	FontSize         float64
	Size             float64
	FontBase         string
	TextFont         string
	Args             []primitives.Object
	Matrix           geometry.Matrix
	TextMatrix       geometry.Matrix
	LineMatrix       geometry.Matrix
	ScalingMatrix    geometry.Matrix
	Origin           [2]float64
	Displacement     [2]float64
	Rotation         float64
	BBox             [4]float64
	VisualBBox       *[4]float64
	LineWidth        float64
	StrokeColor      []float64
	NonStrokeColor   []float64
	Invisible        bool
	Vertical         bool
	Unmapped         bool
	GState           GraphicsState
	MarkedTag        string
	MarkedProperties primitives.Dict
	MarkedStack      []MarkedContentContext
	ActualText       string
	MCID             int
	HasMCID          bool
}

// NewText constructs an owned text value.
func NewText(spec TextSpec) Text {
	return Text{
		text: spec.Text, page: spec.Page, hasPage: spec.HasPage, chars: spec.Chars,
		fontName: spec.FontName, fontSize: spec.FontSize, size: spec.Size,
		fontBase: spec.FontBase, textFont: spec.TextFont, args: cloneObjects(spec.Args),
		matrix: spec.Matrix, textMatrix: spec.TextMatrix, lineMatrix: spec.LineMatrix,
		scalingMatrix: spec.ScalingMatrix, origin: spec.Origin, displacement: spec.Displacement,
		rotation: spec.Rotation, bbox: spec.BBox, visualBBox: cloneVisualBBox(spec.VisualBBox),
		lineWidth: spec.LineWidth, strokeColor: cloneFloats(spec.StrokeColor),
		nonStrokeColor: cloneFloats(spec.NonStrokeColor), invisible: spec.Invisible,
		vertical: spec.Vertical, unmapped: spec.Unmapped, gstate: spec.GState.Finalize(),
		markedTag: spec.MarkedTag, markedProperties: cloneDict(spec.MarkedProperties),
		markedStack: cloneMarkedStack(spec.MarkedStack), actualText: spec.ActualText,
		mcid: spec.MCID, hasMCID: spec.HasMCID,
	}
}

// NewTextBorrowed constructs a low-allocation value over slices owned by the
// caller. Use the Copy accessors or Finalize before retaining it.
func NewTextBorrowed(spec TextSpec) Text {
	return Text{
		text: spec.Text, page: spec.Page, hasPage: spec.HasPage, chars: spec.Chars,
		fontName: spec.FontName, fontSize: spec.FontSize, size: spec.Size,
		fontBase: spec.FontBase, textFont: spec.TextFont, args: spec.Args,
		matrix: spec.Matrix, textMatrix: spec.TextMatrix, lineMatrix: spec.LineMatrix,
		scalingMatrix: spec.ScalingMatrix, origin: spec.Origin, displacement: spec.Displacement,
		rotation: spec.Rotation, bbox: spec.BBox, visualBBox: spec.VisualBBox,
		lineWidth: spec.LineWidth, strokeColor: spec.StrokeColor,
		nonStrokeColor: spec.NonStrokeColor, invisible: spec.Invisible,
		vertical: spec.Vertical, unmapped: spec.Unmapped, gstate: spec.GState,
		markedTag: spec.MarkedTag, markedProperties: spec.MarkedProperties,
		markedStack: spec.MarkedStack, actualText: spec.ActualText,
		mcid: spec.MCID, hasMCID: spec.HasMCID,
	}
}

func (t Text) Text() string                      { return t.text }
func (t Text) Page() primitives.Ref              { return t.page }
func (t Text) HasPage() bool                     { return t.hasPage }
func (t Text) Chars() string                     { return t.chars }
func (t Text) FontName() string                  { return t.fontName }
func (t Text) FontSize() float64                 { return t.fontSize }
func (t Text) Size() float64                     { return t.size }
func (t Text) FontBase() string                  { return t.fontBase }
func (t Text) TextFont() string                  { return t.textFont }
func (t Text) ArgsBorrowed() []primitives.Object { return t.args }
func (t Text) Matrix() geometry.Matrix           { return t.matrix }
func (t Text) TextMatrix() geometry.Matrix       { return t.textMatrix }
func (t Text) LineMatrix() geometry.Matrix       { return t.lineMatrix }
func (t Text) ScalingMatrix() geometry.Matrix    { return t.scalingMatrix }
func (t Text) Origin() [2]float64                { return t.origin }
func (t Text) Displacement() [2]float64          { return t.displacement }
func (t Text) Rotation() float64                 { return t.rotation }
func (t Text) BBox() [4]float64                  { return t.bbox }
func (t Text) LineWidth() float64                { return t.lineWidth }
func (t Text) Invisible() bool                   { return t.invisible }
func (t Text) Vertical() bool                    { return t.vertical }
func (t Text) Unmapped() bool                    { return t.unmapped }
func (t Text) GState() GraphicsState             { return t.gstate.Finalize() }
func (t Text) MarkedTag() string                 { return t.markedTag }
func (t Text) ActualText() string                { return t.actualText }
func (t Text) MCID() int                         { return t.mcid }
func (t Text) HasMCID() bool                     { return t.hasMCID }

// ArgsCopy returns independent text-show arguments.
func (t Text) ArgsCopy() []primitives.Object { return cloneObjects(t.args) }

// ArgsSeq lazily yields borrowed text-show arguments in source order.
func (t Text) ArgsSeq() iter.Seq[primitives.Object] {
	return func(yield func(primitives.Object) bool) {
		for _, arg := range t.args {
			if !yield(arg) {
				return
			}
		}
	}
}

// VisualBBoxCopy returns the optional visual bounding box.
func (t Text) VisualBBoxCopy() ([4]float64, bool) {
	if t.visualBBox == nil {
		return [4]float64{}, false
	}
	return *t.visualBBox, true
}

// StrokeColorCopy returns independent stroke color components.
func (t Text) StrokeColorCopy() []float64 { return cloneFloats(t.strokeColor) }

// NonStrokeColorCopy returns independent non-stroke color components.
func (t Text) NonStrokeColorCopy() []float64 { return cloneFloats(t.nonStrokeColor) }

// MarkedPropertiesCopy returns independent marked-content properties.
func (t Text) MarkedPropertiesCopy() primitives.Dict { return cloneDict(t.markedProperties) }

// MarkedStackCopy returns independent enclosing marked-content contexts.
func (t Text) MarkedStackCopy() []MarkedContentContext { return cloneMarkedStack(t.markedStack) }

// Finalize returns an independent text snapshot.
func (t Text) Finalize() Text {
	return NewText(TextSpec{
		Text: t.text, Page: t.page, HasPage: t.hasPage, Chars: t.chars,
		FontName: t.fontName, FontSize: t.fontSize, Size: t.size, FontBase: t.fontBase,
		TextFont: t.textFont, Args: t.args, Matrix: t.matrix, TextMatrix: t.textMatrix,
		LineMatrix: t.lineMatrix, ScalingMatrix: t.scalingMatrix, Origin: t.origin,
		Displacement: t.displacement, Rotation: t.rotation, BBox: t.bbox,
		VisualBBox: t.visualBBox, LineWidth: t.lineWidth, StrokeColor: t.strokeColor,
		NonStrokeColor: t.nonStrokeColor, Invisible: t.invisible, Vertical: t.vertical,
		Unmapped: t.unmapped, GState: t.gstate, MarkedTag: t.markedTag,
		MarkedProperties: t.markedProperties, MarkedStack: t.markedStack,
		ActualText: t.actualText, MCID: t.mcid, HasMCID: t.hasMCID,
	})
}

func (t Text) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Text             string                 `json:"text"`
		Page             primitives.Ref         `json:"page"`
		HasPage          bool                   `json:"has_page"`
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
		Args             []primitives.Object    `json:"args,omitempty"`
		StrokeColor      []float64              `json:"scolor"`
		NonStrokeColor   []float64              `json:"ncolor"`
		MarkedProperties primitives.Dict        `json:"marked_properties,omitempty"`
		MarkedStack      []MarkedContentContext `json:"marked_stack,omitempty"`
	}{
		Text: t.text, Page: t.page, HasPage: t.hasPage, Chars: t.chars,
		FontName: t.fontName, FontSize: t.fontSize, Size: t.size, FontBase: t.fontBase,
		TextFont: t.textFont, Matrix: t.matrix, TextMatrix: t.textMatrix,
		LineMatrix: t.lineMatrix, ScalingMatrix: t.scalingMatrix, Origin: t.origin,
		Displacement: t.displacement, Rotation: t.rotation, BBox: t.bbox,
		VisualBBox: cloneVisualBBox(t.visualBBox), LineWidth: t.lineWidth,
		Invisible: t.invisible, Vertical: t.vertical, Unmapped: t.unmapped,
		GState: t.gstate, MarkedTag: t.markedTag, ActualText: t.actualText,
		MCID: t.mcid, HasMCID: t.hasMCID, Args: t.ArgsCopy(),
		StrokeColor: t.StrokeColorCopy(), NonStrokeColor: t.NonStrokeColorCopy(),
		MarkedProperties: t.MarkedPropertiesCopy(), MarkedStack: t.MarkedStackCopy(),
	})
}

func cloneVisualBBox(value *[4]float64) *[4]float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
