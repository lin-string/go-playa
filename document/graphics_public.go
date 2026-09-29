package document

import "github.com/lin-string/go-playa/contentdata"

// GraphicsState is the immutable public value model for content graphics
// state. The mutable interpreter state remains private to document.
type GraphicsState = contentdata.GraphicsState
type DashPattern = contentdata.DashPattern

// DefaultGraphicsState returns the PDF default graphics-state value.
func DefaultGraphicsState() GraphicsState { return contentdata.DefaultGraphicsState() }

func (g graphicsState) publicValue() GraphicsState {
	return contentdata.NewGraphicsState(contentdata.GraphicsStateValues{
		CTM: g.ctm, FontName: g.fontName, FontSize: g.fontSize, LineWidth: g.lineWidth,
		LineCap: g.lineCap, LineJoin: g.lineJoin, MiterLimit: g.miterLimit,
		Dash: g.dash, DashPhase: g.dashPhase, StrokeColor: g.strokeColor,
		FillColor: g.fillColor, RenderMode: g.renderMode, CharacterRise: g.characterRise,
		CharacterSpacing: g.characterSpacing, WordSpacing: g.wordSpacing,
		HorizontalScale: g.horizontalScale, Leading: g.leading, Alpha: g.alpha,
		StrokeAlpha: g.strokeAlpha, FillAlpha: g.fillAlpha, BlendMode: g.blendMode,
		BlendModes: g.blendModes, Intent: g.intent, Flatness: g.flatness,
		StrokeAdjustment: g.strokeAdjustment, AlphaSource: g.alphaSource,
		Knockout: g.knockout, Overprint: g.overprint, StrokeOverprint: g.strokeOverprint,
		OverprintMode: g.overprintMode, Halftone: g.halftone, HasHalftone: g.hasHalftone,
		SoftMask: g.softMask, HasSoftMask: g.hasSoftMask, BlackPointComp: g.blackPointComp,
		ClipDepth: g.clipDepth, ClipEvenOdd: g.clipEvenOdd,
	})
}

func graphicsStateFromPublic(g GraphicsState) graphicsState {
	halftone, hasHalftone := g.HalftoneCopy()
	softMask, hasSoftMask := g.SoftMaskCopy()
	return graphicsState{
		ctm: g.CTM(), fontName: g.FontName(), fontSize: g.FontSize(), lineWidth: g.LineWidth(),
		lineCap: g.LineCap(), lineJoin: g.LineJoin(), miterLimit: g.MiterLimit(),
		dash: g.DashCopy(), dashPhase: g.DashPhase(), strokeColor: g.StrokeColor(),
		fillColor: g.FillColor(), renderMode: g.RenderMode(), characterRise: g.CharacterRise(),
		characterSpacing: g.CharacterSpacing(), wordSpacing: g.WordSpacing(),
		horizontalScale: g.HorizontalScale(), leading: g.Leading(), alpha: g.Alpha(),
		strokeAlpha: g.StrokeAlpha(), fillAlpha: g.FillAlpha(), blendMode: g.BlendMode(),
		blendModes: g.BlendModesCopy(), intent: g.Intent(), flatness: g.Flatness(),
		strokeAdjustment: g.StrokeAdjustment(), alphaSource: g.AlphaSource(),
		knockout: g.Knockout(), overprint: g.Overprint(), strokeOverprint: g.StrokeOverprint(),
		overprintMode: g.OverprintMode(), halftone: halftone, hasHalftone: hasHalftone,
		softMask: softMask, hasSoftMask: hasSoftMask, blackPointComp: g.BlackPointComp(),
		clipDepth: g.ClipDepth(), clipEvenOdd: g.ClipEvenOdd(),
	}
}

// ApplyGraphicsState applies direct content operators to a public state
// value. The interpreter uses its private allocation-free path instead.
func ApplyGraphicsState(g *GraphicsState, op ContentOp) {
	if g == nil {
		return
	}
	state := graphicsStateFromPublic(*g)
	applyGraphicsState(&state, op)
	*g = state.publicValue()
}

// ApplyExternalGraphicsState applies a resource-backed gs selection to a
// public state value.
func ApplyExternalGraphicsState(d *Document, g *GraphicsState, op ContentOp) {
	if g == nil {
		return
	}
	state := graphicsStateFromPublic(*g)
	applyExternalGraphicsState(d, &state, op)
	*g = state.publicValue()
}

// ApplyResourceColorSpace resolves a resource-backed color-space selection
// into a public state value.
func ApplyResourceColorSpace(d *Document, g *GraphicsState, op ContentOp) {
	if g == nil {
		return
	}
	state := graphicsStateFromPublic(*g)
	applyResourceColorSpace(d, &state, op)
	*g = state.publicValue()
}
