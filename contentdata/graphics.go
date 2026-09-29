package contentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// GraphicsStateValues is the construction view for GraphicsState. The
// returned GraphicsState owns every mutable value and exposes only read-only
// accessors thereafter.
type GraphicsStateValues struct {
	CTM              geometry.Matrix
	FontName         string
	FontSize         float64
	LineWidth        float64
	LineCap          int
	LineJoin         int
	MiterLimit       float64
	Dash             []float64
	DashPhase        float64
	StrokeColor      geometry.Color
	FillColor        geometry.Color
	RenderMode       int
	CharacterRise    float64
	CharacterSpacing float64
	WordSpacing      float64
	HorizontalScale  float64
	Leading          float64
	Alpha            float64
	StrokeAlpha      float64
	FillAlpha        float64
	BlendMode        string
	BlendModes       []string
	Intent           string
	Flatness         float64
	StrokeAdjustment bool
	AlphaSource      bool
	Knockout         bool
	Overprint        bool
	StrokeOverprint  bool
	OverprintMode    int
	Halftone         primitives.Object
	HasHalftone      bool
	SoftMask         primitives.Object
	HasSoftMask      bool
	BlackPointComp   string
	ClipDepth        int
	ClipEvenOdd      bool
}

// GraphicsState contains the value portion of the PDF graphics state.
// Resource resolution and interpreter mutation remain document-owned.
type GraphicsState struct {
	ctm  geometry.Matrix
	data *graphicsStateData
}

type graphicsStateData struct {
	fontName         string
	fontSize         float64
	lineWidth        float64
	lineCap          int
	lineJoin         int
	miterLimit       float64
	dash             []float64
	dashPhase        float64
	strokeColor      geometry.Color
	fillColor        geometry.Color
	renderMode       int
	characterRise    float64
	characterSpacing float64
	wordSpacing      float64
	horizontalScale  float64
	leading          float64
	alpha            float64
	strokeAlpha      float64
	fillAlpha        float64
	blendMode        string
	blendModes       []string
	intent           string
	flatness         float64
	strokeAdjustment bool
	alphaSource      bool
	knockout         bool
	overprint        bool
	strokeOverprint  bool
	overprintMode    int
	halftone         primitives.Object
	hasHalftone      bool
	softMask         primitives.Object
	hasSoftMask      bool
	blackPointComp   string
	clipDepth        int
	clipEvenOdd      bool
}

var zeroGraphicsStateData graphicsStateData

func (g GraphicsState) shared() *graphicsStateData {
	if g.data == nil {
		return &zeroGraphicsStateData
	}
	return g.data
}

// NewGraphicsState constructs an owned value snapshot.
func NewGraphicsState(values GraphicsStateValues) GraphicsState {
	return GraphicsState{
		ctm: values.CTM,
		data: &graphicsStateData{fontName: values.FontName, fontSize: values.FontSize,
			lineWidth: values.LineWidth, lineCap: values.LineCap, lineJoin: values.LineJoin,
			miterLimit: values.MiterLimit, dash: cloneFloats(values.Dash), dashPhase: values.DashPhase,
			strokeColor: values.StrokeColor.Finalize(), fillColor: values.FillColor.Finalize(),
			renderMode: values.RenderMode, characterRise: values.CharacterRise,
			characterSpacing: values.CharacterSpacing, wordSpacing: values.WordSpacing,
			horizontalScale: values.HorizontalScale, leading: values.Leading,
			alpha: values.Alpha, strokeAlpha: values.StrokeAlpha, fillAlpha: values.FillAlpha,
			blendMode: values.BlendMode, blendModes: cloneStrings(values.BlendModes), intent: values.Intent,
			flatness: values.Flatness, strokeAdjustment: values.StrokeAdjustment,
			alphaSource: values.AlphaSource, knockout: values.Knockout, overprint: values.Overprint,
			strokeOverprint: values.StrokeOverprint, overprintMode: values.OverprintMode,
			halftone: cloneObject(values.Halftone), hasHalftone: values.HasHalftone,
			softMask: cloneObject(values.SoftMask), hasSoftMask: values.HasSoftMask,
			blackPointComp: values.BlackPointComp, clipDepth: values.ClipDepth,
			clipEvenOdd: values.ClipEvenOdd},
	}
}

// WithCTM returns a graphics state with a different transform while sharing
// all other immutable state.
func (g GraphicsState) WithCTM(ctm geometry.Matrix) GraphicsState {
	return GraphicsState{ctm: ctm, data: g.data}
}

// DefaultGraphicsState returns the PDF default graphics-state value.
func DefaultGraphicsState() GraphicsState {
	return NewGraphicsState(GraphicsStateValues{
		CTM: geometry.Matrix{1, 0, 0, 1, 0, 0}, LineWidth: 1, MiterLimit: 10,
		StrokeColor: geometry.NewColor("DeviceGray", []float64{0}, "", 1),
		FillColor:   geometry.NewColor("DeviceGray", []float64{0}, "", 1),
		Alpha:       1, StrokeAlpha: 1, FillAlpha: 1, HorizontalScale: 1,
		Intent: "RelativeColorimetric", Flatness: 1, Knockout: true,
	})
}

func (g GraphicsState) CTM() geometry.Matrix { return g.ctm }
func (g GraphicsState) FontName() string     { return g.shared().fontName }
func (g GraphicsState) FontSize() float64    { return g.shared().fontSize }
func (g GraphicsState) LineWidth() float64   { return g.shared().lineWidth }
func (g GraphicsState) LineCap() int         { return g.shared().lineCap }
func (g GraphicsState) LineJoin() int        { return g.shared().lineJoin }
func (g GraphicsState) MiterLimit() float64  { return g.shared().miterLimit }
func (g GraphicsState) DashPhase() float64   { return g.shared().dashPhase }

// Dash returns the complete stroke dash pattern.
func (g GraphicsState) Dash() DashPattern {
	state := g.shared()
	return NewDashPattern(state.dash, state.dashPhase)
}
func (g GraphicsState) StrokeColor() geometry.Color { return g.shared().strokeColor.Finalize() }
func (g GraphicsState) FillColor() geometry.Color   { return g.shared().fillColor.Finalize() }
func (g GraphicsState) RenderMode() int             { return g.shared().renderMode }
func (g GraphicsState) CharacterRise() float64      { return g.shared().characterRise }
func (g GraphicsState) CharacterSpacing() float64   { return g.shared().characterSpacing }
func (g GraphicsState) WordSpacing() float64        { return g.shared().wordSpacing }
func (g GraphicsState) HorizontalScale() float64    { return g.shared().horizontalScale }
func (g GraphicsState) Leading() float64            { return g.shared().leading }
func (g GraphicsState) Alpha() float64              { return g.shared().alpha }
func (g GraphicsState) StrokeAlpha() float64        { return g.shared().strokeAlpha }
func (g GraphicsState) FillAlpha() float64          { return g.shared().fillAlpha }
func (g GraphicsState) BlendMode() string           { return g.shared().blendMode }
func (g GraphicsState) Intent() string              { return g.shared().intent }
func (g GraphicsState) Flatness() float64           { return g.shared().flatness }
func (g GraphicsState) StrokeAdjustment() bool      { return g.shared().strokeAdjustment }
func (g GraphicsState) AlphaSource() bool           { return g.shared().alphaSource }
func (g GraphicsState) Knockout() bool              { return g.shared().knockout }
func (g GraphicsState) Overprint() bool             { return g.shared().overprint }
func (g GraphicsState) StrokeOverprint() bool       { return g.shared().strokeOverprint }
func (g GraphicsState) OverprintMode() int          { return g.shared().overprintMode }
func (g GraphicsState) HasHalftone() bool           { return g.shared().hasHalftone }
func (g GraphicsState) HasSoftMask() bool           { return g.shared().hasSoftMask }
func (g GraphicsState) BlackPointComp() string      { return g.shared().blackPointComp }
func (g GraphicsState) ClipDepth() int              { return g.shared().clipDepth }
func (g GraphicsState) ClipEvenOdd() bool           { return g.shared().clipEvenOdd }

// DashCopy returns independent dash components.
func (g GraphicsState) DashCopy() []float64 { return cloneFloats(g.shared().dash) }

// BlendModesCopy returns independent blend-mode names.
func (g GraphicsState) BlendModesCopy() []string { return cloneStrings(g.shared().blendModes) }

// SoftMaskCopy returns an independent soft-mask object.
func (g GraphicsState) SoftMaskCopy() (primitives.Object, bool) {
	state := g.shared()
	return cloneObject(state.softMask), state.hasSoftMask
}

// HalftoneCopy returns an independent halftone object.
func (g GraphicsState) HalftoneCopy() (primitives.Object, bool) {
	state := g.shared()
	return cloneObject(state.halftone), state.hasHalftone
}

// Finalize returns an independent snapshot.
func (g GraphicsState) Finalize() GraphicsState {
	state := g.shared()
	return NewGraphicsState(GraphicsStateValues{
		CTM: g.ctm, FontName: state.fontName, FontSize: state.fontSize, LineWidth: state.lineWidth,
		LineCap: state.lineCap, LineJoin: state.lineJoin, MiterLimit: state.miterLimit, Dash: state.dash,
		DashPhase: state.dashPhase, StrokeColor: state.strokeColor, FillColor: state.fillColor,
		RenderMode: state.renderMode, CharacterRise: state.characterRise,
		CharacterSpacing: state.characterSpacing, WordSpacing: state.wordSpacing,
		HorizontalScale: state.horizontalScale, Leading: state.leading, Alpha: state.alpha,
		StrokeAlpha: state.strokeAlpha, FillAlpha: state.fillAlpha, BlendMode: state.blendMode,
		BlendModes: state.blendModes, Intent: state.intent, Flatness: state.flatness,
		StrokeAdjustment: state.strokeAdjustment, AlphaSource: state.alphaSource,
		Knockout: state.knockout, Overprint: state.overprint, StrokeOverprint: state.strokeOverprint,
		OverprintMode: state.overprintMode, Halftone: state.halftone, HasHalftone: state.hasHalftone,
		SoftMask: state.softMask, HasSoftMask: state.hasSoftMask, BlackPointComp: state.blackPointComp,
		ClipDepth: state.clipDepth, ClipEvenOdd: state.clipEvenOdd,
	})
}

func (g GraphicsState) MarshalJSON() ([]byte, error) {
	state := g.shared()
	return json.Marshal(struct {
		CTM              geometry.Matrix
		FontName         string
		FontSize         float64
		LineWidth        float64
		LineCap          int
		LineJoin         int
		MiterLimit       float64
		DashPhase        float64
		StrokeColor      geometry.Color
		FillColor        geometry.Color
		RenderMode       int
		CharacterRise    float64
		CharacterSpacing float64
		WordSpacing      float64
		HorizontalScale  float64
		Leading          float64
		Alpha            float64
		StrokeAlpha      float64
		FillAlpha        float64
		BlendMode        string
		Intent           string
		Flatness         float64
		StrokeAdjustment bool
		AlphaSource      bool
		Knockout         bool
		Overprint        bool
		StrokeOverprint  bool
		OverprintMode    int
		HasHalftone      bool
		HasSoftMask      bool
		BlackPointComp   string
		ClipDepth        int
		ClipEvenOdd      bool
		SoftMask         primitives.Object `json:"SoftMask"`
		Halftone         primitives.Object `json:"Halftone"`
		Dash             []float64         `json:"Dash"`
		BlendModes       []string          `json:"BlendModes"`
	}{
		CTM: g.ctm, FontName: state.fontName, FontSize: state.fontSize, LineWidth: state.lineWidth,
		LineCap: state.lineCap, LineJoin: state.lineJoin, MiterLimit: state.miterLimit, DashPhase: state.dashPhase,
		StrokeColor: state.strokeColor, FillColor: state.fillColor, RenderMode: state.renderMode,
		CharacterRise: state.characterRise, CharacterSpacing: state.characterSpacing, WordSpacing: state.wordSpacing,
		HorizontalScale: state.horizontalScale, Leading: state.leading, Alpha: state.alpha,
		StrokeAlpha: state.strokeAlpha, FillAlpha: state.fillAlpha, BlendMode: state.blendMode,
		Intent: state.intent, Flatness: state.flatness, StrokeAdjustment: state.strokeAdjustment,
		AlphaSource: state.alphaSource, Knockout: state.knockout, Overprint: state.overprint,
		StrokeOverprint: state.strokeOverprint, OverprintMode: state.overprintMode,
		HasHalftone: state.hasHalftone, HasSoftMask: state.hasSoftMask, BlackPointComp: state.blackPointComp,
		ClipDepth: state.clipDepth, ClipEvenOdd: state.clipEvenOdd,
		SoftMask: state.softMask, Halftone: state.halftone, Dash: state.dash, BlendModes: state.blendModes,
	})
}

func cloneFloats(value []float64) []float64 {
	if value == nil {
		return nil
	}
	return append(make([]float64, 0, len(value)), value...)
}

func cloneStrings(value []string) []string {
	if value == nil {
		return nil
	}
	return append(make([]string, 0, len(value)), value...)
}
