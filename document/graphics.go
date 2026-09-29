package document

import (
	"encoding/json"
	"math"
	"reflect"

	"github.com/lin-string/go-playa/geometry"
)

func cloneGraphicsFloats(value []float64) []float64 {
	if value == nil {
		return nil
	}
	return append(make([]float64, 0, len(value)), value...)
}

func cloneGraphicsStrings(value []string) []string {
	if value == nil {
		return nil
	}
	return append(make([]string, 0, len(value)), value...)
}

// graphicsState contains the state that content operators carry between
// painted objects. It is deliberately value-like; Clone makes the mutable
// slices safe to retain on finalized content objects.
type graphicsState struct {
	ctm               geometry.Matrix
	fontName          string
	fontSize          float64
	lineWidth         float64
	lineCap           int
	lineJoin          int
	miterLimit        float64
	dash              []float64
	dashPhase         float64
	strokeColor       geometry.Color
	fillColor         geometry.Color
	strokeIndexed     bool
	strokeIndexedHigh int
	fillIndexed       bool
	fillIndexedHigh   int
	renderMode        int
	characterRise     float64
	characterSpacing  float64
	wordSpacing       float64
	horizontalScale   float64
	leading           float64
	alpha             float64
	strokeAlpha       float64
	fillAlpha         float64
	blendMode         string
	blendModes        []string
	intent            string
	flatness          float64
	strokeAdjustment  bool
	alphaSource       bool
	knockout          bool
	overprint         bool
	strokeOverprint   bool
	overprintMode     int
	halftone          Object
	hasHalftone       bool
	softMask          Object
	hasSoftMask       bool
	blackPointComp    string
	clipDepth         int
	clipEvenOdd       bool
}

func normalizeIndexedColor(color geometry.Color, high int) geometry.Color {
	values := color.ValuesCopy()
	if len(values) != 1 || high < 0 || math.IsNaN(values[0]) || math.IsInf(values[0], 0) {
		return color
	}
	index := math.Floor(values[0] + 0.5)
	index = max(0, min(index, float64(high)))
	return geometry.NewColor(color.Space(), []float64{index}, color.Pattern(), color.Components())
}

type markedContentFrame struct {
	Tag        string
	Properties Dict
	MCID       int
	HasMCID    bool
	ActualText string
	identity   *markedContentIdentity
}

// markedContentIdentity distinguishes separate BMC/BDC instances even when
// their visible tag and properties are identical. It is private because it
// only supports Playa-compatible grouping during one parse.
type markedContentIdentity byte

func newMarkedContentIdentity() *markedContentIdentity {
	return new(markedContentIdentity)
}

func applyMarkedContent(stack *[]markedContentFrame, op ContentOp) {
	switch op.operatorValue() {
	case "BMC":
		frame := markedContentFrame{MCID: -1, identity: newMarkedContentIdentity()}
		if len(op.operandsValue()) > 0 {
			if tag, ok := op.operandsValue()[0].(Name); ok {
				frame.Tag = string(tag)
			}
		}
		*stack = append(*stack, frame)
	case "BDC":
		if len(op.operandsValue()) < 2 {
			return
		}
		if _, ok := op.operandsValue()[1].(Dict); !ok {
			return
		}
		frame := markedContentFrame{MCID: -1, identity: newMarkedContentIdentity()}
		if len(op.operandsValue()) > 0 {
			if tag, ok := op.operandsValue()[0].(Name); ok {
				frame.Tag = string(tag)
			}
		}
		props := op.operandsValue()[1].(Dict)
		frame.Properties = props
		if id, ok := IntValue(props[Name("MCID")]); ok {
			frame.MCID, frame.HasMCID = id, true
		}
		if text, ok := props[Name("ActualText")].(String); ok {
			frame.ActualText = decodePDFText(text)
		}
		*stack = append(*stack, frame)
	case "EMC":
		if len(*stack) > 0 {
			*stack = (*stack)[:len(*stack)-1]
		}
	}
}

func currentMarkedContent(stack []markedContentFrame) markedContentFrame {
	if len(stack) == 0 {
		return markedContentFrame{MCID: -1}
	}
	return stack[len(stack)-1]
}

func markedContextStack(stack []markedContentFrame) []markedContentContext {
	if len(stack) == 0 {
		return nil
	}
	out := make([]markedContentContext, len(stack))
	for i, frame := range stack {
		out[i] = newMarkedContentContext(frame.Tag, frame.Properties, frame.ActualText, frame.MCID, frame.HasMCID, frame.identity)
	}
	return out
}

func newGraphicsState() graphicsState {
	return graphicsState{
		ctm:             identity(),
		lineWidth:       1,
		miterLimit:      10,
		strokeColor:     geometry.NewColor("DeviceGray", []float64{0}, "", 1),
		fillColor:       geometry.NewColor("DeviceGray", []float64{0}, "", 1),
		alpha:           1,
		strokeAlpha:     1,
		fillAlpha:       1,
		horizontalScale: 1,
		intent:          "RelativeColorimetric",
		flatness:        1,
		knockout:        true,
	}
}

func (g graphicsState) CTM() geometry.Matrix        { return g.ctm }
func (g graphicsState) FontName() string            { return g.fontName }
func (g graphicsState) FontSize() float64           { return g.fontSize }
func (g graphicsState) LineWidth() float64          { return g.lineWidth }
func (g graphicsState) LineCap() int                { return g.lineCap }
func (g graphicsState) LineJoin() int               { return g.lineJoin }
func (g graphicsState) MiterLimit() float64         { return g.miterLimit }
func (g graphicsState) DashPhase() float64          { return g.dashPhase }
func (g graphicsState) StrokeColor() geometry.Color { return g.strokeColor.Finalize() }
func (g graphicsState) FillColor() geometry.Color   { return g.fillColor.Finalize() }
func (g graphicsState) RenderMode() int             { return g.renderMode }
func (g graphicsState) CharacterRise() float64      { return g.characterRise }
func (g graphicsState) CharacterSpacing() float64   { return g.characterSpacing }
func (g graphicsState) WordSpacing() float64        { return g.wordSpacing }
func (g graphicsState) HorizontalScale() float64    { return g.horizontalScale }
func (g graphicsState) Leading() float64            { return g.leading }
func (g graphicsState) Alpha() float64              { return g.alpha }
func (g graphicsState) StrokeAlpha() float64        { return g.strokeAlpha }
func (g graphicsState) FillAlpha() float64          { return g.fillAlpha }
func (g graphicsState) BlendMode() string           { return g.blendMode }
func (g graphicsState) Intent() string              { return g.intent }
func (g graphicsState) Flatness() float64           { return g.flatness }
func (g graphicsState) StrokeAdjustment() bool      { return g.strokeAdjustment }
func (g graphicsState) AlphaSource() bool           { return g.alphaSource }
func (g graphicsState) Knockout() bool              { return g.knockout }
func (g graphicsState) Overprint() bool             { return g.overprint }
func (g graphicsState) StrokeOverprint() bool       { return g.strokeOverprint }
func (g graphicsState) OverprintMode() int          { return g.overprintMode }
func (g graphicsState) HasHalftone() bool           { return g.hasHalftone }
func (g graphicsState) HasSoftMask() bool           { return g.hasSoftMask }
func (g graphicsState) BlackPointComp() string      { return g.blackPointComp }
func (g graphicsState) ClipDepth() int              { return g.clipDepth }
func (g graphicsState) ClipEvenOdd() bool           { return g.clipEvenOdd }

func (g graphicsState) Clone() graphicsState {
	g.dash = cloneGraphicsFloats(g.dash)
	g.strokeColor = g.strokeColor.Finalize()
	g.fillColor = g.fillColor.Finalize()
	g.blendModes = cloneGraphicsStrings(g.blendModes)
	g.halftone = cloneGraphicsObject(g.halftone)
	g.softMask = cloneGraphicsObject(g.softMask)
	return g
}

// DashCopy returns the graphics state's dash pattern.
func (g graphicsState) DashCopy() []float64 { return cloneGraphicsFloats(g.dash) }

// BlendModesCopy returns the graphics state's blend-mode sequence.
func (g graphicsState) BlendModesCopy() []string { return cloneGraphicsStrings(g.blendModes) }

// SoftMaskCopy returns an independent copy of the soft-mask object.
func (g graphicsState) SoftMaskCopy() (Object, bool) {
	return cloneGraphicsObject(g.softMask), g.hasSoftMask
}

// HalftoneCopy returns an independent copy of the halftone object.
func (g graphicsState) HalftoneCopy() (Object, bool) {
	return cloneGraphicsObject(g.halftone), g.hasHalftone
}

func (g graphicsState) MarshalJSON() ([]byte, error) {
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
		SoftMask         Object    `json:"SoftMask"`
		Halftone         Object    `json:"Halftone"`
		Dash             []float64 `json:"Dash"`
		BlendModes       []string  `json:"BlendModes"`
	}{
		CTM: g.ctm, FontName: g.fontName, FontSize: g.fontSize, LineWidth: g.lineWidth,
		LineCap: g.lineCap, LineJoin: g.lineJoin, MiterLimit: g.miterLimit, DashPhase: g.dashPhase,
		StrokeColor: g.strokeColor, FillColor: g.fillColor, RenderMode: g.renderMode,
		CharacterRise: g.characterRise, CharacterSpacing: g.characterSpacing, WordSpacing: g.wordSpacing,
		HorizontalScale: g.horizontalScale, Leading: g.leading, Alpha: g.alpha,
		StrokeAlpha: g.strokeAlpha, FillAlpha: g.fillAlpha, BlendMode: g.blendMode,
		Intent: g.intent, Flatness: g.flatness, StrokeAdjustment: g.strokeAdjustment,
		AlphaSource: g.alphaSource, Knockout: g.knockout, Overprint: g.overprint,
		StrokeOverprint: g.strokeOverprint, OverprintMode: g.overprintMode,
		HasHalftone: g.hasHalftone, HasSoftMask: g.hasSoftMask, BlackPointComp: g.blackPointComp,
		ClipDepth: g.clipDepth, ClipEvenOdd: g.clipEvenOdd,
		SoftMask: g.softMask, Halftone: g.halftone, Dash: g.dash, BlendModes: g.blendModes,
	})
}

// Finalize returns an independent snapshot of this graphics state.
func (g graphicsState) Finalize() graphicsState {
	return g.Clone()
}

func cloneGraphicsObject(value Object) Object {
	switch value := value.(type) {
	case Array:
		if value == nil {
			return Array(nil)
		}
		out := make(Array, len(value))
		for i, item := range value {
			out[i] = cloneGraphicsObject(item)
		}
		return out
	case InvalidArray:
		if value == nil {
			return InvalidArray(nil)
		}
		out := make(InvalidArray, len(value))
		for i, item := range value {
			out[i] = cloneGraphicsObject(item)
		}
		return out
	case Dict:
		if value == nil {
			return Dict(nil)
		}
		out := make(Dict, len(value))
		for key, item := range value {
			out[key] = cloneGraphicsObject(item)
		}
		return out
	case String:
		return String(cloneObjectBytes(value))
	case Stream:
		return value.ClonePDFObject()
	default:
		return value
	}
}

// applyGraphicsState applies operators whose state is local to one graphics
// state frame. q/Q stack management remains with the content interpreter.
func applyGraphicsState(g *graphicsState, op ContentOp) {
	if g == nil {
		return
	}
	number := func(i int) (float64, bool) {
		if i < 0 || i >= len(op.operandsValue()) {
			return 0, false
		}
		return finiteNumberValue(op.operandsValue()[i])
	}
	integer := func(i int) (int, bool) {
		value, ok := number(i)
		if !ok || value != float64(int(value)) {
			return 0, false
		}
		return int(value), true
	}
	integerInRange := func(i, min, max int) (int, bool) {
		value, ok := integer(i)
		if !ok || value < min || value > max {
			return 0, false
		}
		return value, true
	}
	numberInRange := func(i int, min, max float64) (float64, bool) {
		value, ok := number(i)
		if !ok || value < min || value > max {
			return 0, false
		}
		return value, true
	}
	values := func() []float64 {
		out := make([]float64, 0, len(op.operandsValue()))
		for _, operand := range op.operandsValue() {
			if value, ok := finiteNumberValue(operand); ok {
				out = append(out, value)
			}
		}
		return out
	}
	deviceValues := func(count int) []float64 {
		out := values()
		for len(out) < count {
			out = append(out, 0)
		}
		return out[:count]
	}
	colorValues := func(space string, components int) []float64 {
		out := values()
		// Playa's ColorSpace name may be a resource key (for example R13),
		// so detect an optional pattern name from the operand shape instead of
		// relying on the public color-space name.
		if len(op.operandsValue()) > 0 {
			if _, ok := op.operandsValue()[len(op.operandsValue())-1].(Name); ok {
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
	setColor := func(stroke bool, space string, vals []float64, components int) {
		color := geometry.NewColor(space, vals, "", components)
		if stroke {
			g.strokeColor = color
		} else {
			g.fillColor = color
		}
	}
	setPattern := func(stroke bool) {
		if len(op.operandsValue()) == 0 {
			return
		}
		name, ok := op.operandsValue()[len(op.operandsValue())-1].(Name)
		if !ok {
			return
		}
		if stroke {
			g.strokeColor = g.strokeColor.WithPattern(string(name))
		} else {
			g.fillColor = g.fillColor.WithPattern(string(name))
		}
	}
	switch op.operatorValue() {
	case "cm":
		if len(op.operandsValue()) == 6 {
			var m geometry.Matrix
			valid := true
			for i := range m {
				m[i], valid = number(i)
				if !valid {
					break
				}
			}
			if valid {
				if product, ok := g.ctm.MulFinite(m); ok {
					g.ctm = product
				}
			}
		}
	case "w":
		if v, ok := number(0); ok {
			g.lineWidth = v
		}
	case "Tf":
		if len(op.operandsValue()) >= 2 {
			if name, ok := op.operandsValue()[0].(Name); ok {
				g.fontName = string(name)
			}
			if size, ok := number(1); ok {
				g.fontSize = size
			}
		}
	case "J":
		if v, ok := integerInRange(0, 0, 2); ok {
			g.lineCap = v
		}
	case "j":
		if v, ok := integerInRange(0, 0, 2); ok {
			g.lineJoin = v
		}
	case "M":
		if v, ok := number(0); ok {
			g.miterLimit = v
		}
	case "d":
		if len(op.operandsValue()) == 2 {
			phase, phaseOK := finiteNumberValue(op.operandsValue()[1])
			array, arrayOK := op.operandsValue()[0].(Array)
			if phaseOK && arrayOK {
				values := make([]float64, len(array))
				valid := true
				for i, item := range array {
					values[i], valid = finiteNumberValue(item)
					if !valid {
						break
					}
				}
				if valid {
					g.dash = append(g.dash[:0], values...)
					g.dashPhase = phase
				}
			}
		}
	case "RG":
		if !containsNonFiniteNumber(op.operandsValue()) {
			setColor(true, "DeviceRGB", deviceValues(3), 3)
			g.strokeIndexed = false
		}
	case "rg":
		if !containsNonFiniteNumber(op.operandsValue()) {
			setColor(false, "DeviceRGB", deviceValues(3), 3)
			g.fillIndexed = false
		}
	case "G":
		if !containsNonFiniteNumber(op.operandsValue()) {
			setColor(true, "DeviceGray", deviceValues(1), 1)
			g.strokeIndexed = false
		}
	case "g":
		if !containsNonFiniteNumber(op.operandsValue()) {
			setColor(false, "DeviceGray", deviceValues(1), 1)
			g.fillIndexed = false
		}
	case "K":
		if !containsNonFiniteNumber(op.operandsValue()) {
			setColor(true, "DeviceCMYK", deviceValues(4), 4)
			g.strokeIndexed = false
		}
	case "k":
		if !containsNonFiniteNumber(op.operandsValue()) {
			setColor(false, "DeviceCMYK", deviceValues(4), 4)
			g.fillIndexed = false
		}
	case "CS":
		if len(op.operandsValue()) > 0 {
			if name, ok := op.operandsValue()[0].(Name); ok {
				g.strokeColor = g.strokeColor.WithSpace(string(name), colorSpaceComponents(string(name)))
				g.strokeIndexed = false
			}
		}
	case "cs":
		if len(op.operandsValue()) > 0 {
			if name, ok := op.operandsValue()[0].(Name); ok {
				g.fillColor = g.fillColor.WithSpace(string(name), colorSpaceComponents(string(name)))
				g.fillIndexed = false
			}
		}
	case "SC", "SCN":
		if !containsNonFiniteNumber(op.operandsValue()) {
			space := g.strokeColor.Space()
			if space == "" {
				space = "DeviceN"
			}
			setColor(true, space, colorValues(space, g.strokeColor.Components()), g.strokeColor.Components())
			setPattern(true)
			if g.strokeIndexed {
				g.strokeColor = normalizeIndexedColor(g.strokeColor, g.strokeIndexedHigh)
			}
		}
	case "sc", "scn":
		if !containsNonFiniteNumber(op.operandsValue()) {
			space := g.fillColor.Space()
			if space == "" {
				space = "DeviceN"
			}
			setColor(false, space, colorValues(space, g.fillColor.Components()), g.fillColor.Components())
			setPattern(false)
			if g.fillIndexed {
				g.fillColor = normalizeIndexedColor(g.fillColor, g.fillIndexedHigh)
			}
		}
	case "Tr":
		if v, ok := integerInRange(0, 0, 7); ok {
			g.renderMode = v
		}
	case "Ts":
		if v, ok := number(0); ok {
			g.characterRise = v
		}
	case "Tc":
		if v, ok := number(0); ok {
			g.characterSpacing = v
		}
	case "Tw":
		if v, ok := number(0); ok {
			g.wordSpacing = v
		}
	case "Tz":
		if v, ok := number(0); ok {
			g.horizontalScale = v / 100
		}
	case "TL":
		if v, ok := number(0); ok {
			g.leading = v
		}
	case "TD":
		if v, ok := number(1); ok {
			g.leading = -v
		}
	case "CA":
		if v, ok := numberInRange(0, 0, 1); ok {
			g.strokeAlpha = v
		}
	case "ca":
		if v, ok := numberInRange(0, 0, 1); ok {
			g.fillAlpha = v
			g.alpha = v
		}
	case "BM":
		if len(op.operandsValue()) > 0 {
			switch mode := op.operandsValue()[0].(type) {
			case Name:
				g.blendModes = nil
				g.blendMode = string(mode)
			case String:
				g.blendModes = nil
				g.blendMode = decodePDFText(mode)
			case Array:
				if blendModes, ok := blendModeNames(mode, nil); ok {
					g.blendModes = blendModes
					g.blendMode = blendModes[0]
				}
			}
		}
	case "SA":
		if len(op.operandsValue()) > 0 {
			if value, ok := op.operandsValue()[0].(Bool); ok {
				g.strokeAdjustment = bool(value)
			}
		}
	case "AIS":
		if len(op.operandsValue()) > 0 {
			if value, ok := op.operandsValue()[0].(Bool); ok {
				g.alphaSource = bool(value)
			}
		}
	case "TK":
		if len(op.operandsValue()) > 0 {
			if value, ok := op.operandsValue()[0].(Bool); ok {
				g.knockout = bool(value)
			}
		}
	case "OP":
		if len(op.operandsValue()) > 0 {
			if value, ok := op.operandsValue()[0].(Bool); ok {
				g.strokeOverprint = bool(value)
			}
		}
	case "op":
		if len(op.operandsValue()) > 0 {
			if value, ok := op.operandsValue()[0].(Bool); ok {
				g.overprint = bool(value)
			}
		}
	case "OPM":
		if v, ok := integerInRange(0, 0, 1); ok {
			g.overprintMode = v
		}
	case "HT":
		if len(op.operandsValue()) > 0 {
			if isHalftoneObject(op.operandsValue()[0]) {
				g.halftone = cloneGraphicsObject(op.operandsValue()[0])
				g.hasHalftone = true
			}
		}
	case "SMask":
		if len(op.operandsValue()) > 0 {
			if name, ok := op.operandsValue()[0].(Name); ok && name == Name("None") {
				g.softMask, g.hasSoftMask = nil, false
			} else if mask, ok := op.operandsValue()[0].(Dict); ok {
				g.softMask, g.hasSoftMask = mask, true
			}
		}
	case "UseBlackPtComp":
		if len(op.operandsValue()) > 0 {
			if name, ok := op.operandsValue()[0].(Name); ok {
				g.blackPointComp = string(name)
			}
		}
	case "ri":
		if len(op.operandsValue()) > 0 {
			switch intent := op.operandsValue()[0].(type) {
			case Name:
				g.intent = string(intent)
			case String:
				g.intent = decodePDFText(intent)
			}
		}
	case "i":
		if v, ok := number(0); ok {
			g.flatness = v
		}
	}
}

// applyExternalGraphicsState applies the resource-backed state selected by a
// gs operator. Rendering-only soft-mask details remain represented by the raw
// resource dictionary for callers that need them.
func applyExternalGraphicsState(d *Document, g *graphicsState, op ContentOp) {
	if d == nil || g == nil || op.operatorValue() != "gs" || len(op.operandsValue()) == 0 || op.resources == nil {
		return
	}
	name, ok := op.operandsValue()[0].(Name)
	if !ok {
		return
	}
	extgValue, _ := d.resolveIndirectChain(op.resources[Name("ExtGState")])
	extg, _ := extgValue.(Dict)
	stateValue, _ := d.resolveIndirectChain(extg[name])
	state, _ := stateValue.(Dict)
	if state == nil {
		return
	}
	applyNumber := func(key, operator string) {
		if value, ok := state[Name(key)]; ok {
			resolved, _ := d.resolveIndirectChain(value)
			applyGraphicsState(g, newContentOpBorrowed(operator, []Object{resolved}, 0))
		}
	}
	applyNumber("LW", "w")
	applyNumber("LC", "J")
	applyNumber("LJ", "j")
	applyNumber("ML", "M")
	if value, ok := state[Name("D")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if dash, ok := resolved.(Array); ok && len(dash) == 2 {
			resolvedDash := Array{dash[0], dash[1]}
			valuesValue, _ := d.resolveIndirectChain(dash[0])
			values, ok := valuesValue.(Array)
			if ok {
				resolvedValues := make(Array, len(values))
				for i, item := range values {
					resolvedValues[i], _ = d.resolveIndirectChain(item)
					if _, valid := finiteNumberValue(resolvedValues[i]); !valid {
						ok = false
						break
					}
				}
				resolvedDash[0] = resolvedValues
				resolvedDash[1], _ = d.resolveIndirectChain(dash[1])
				if _, valid := finiteNumberValue(resolvedDash[1]); !valid {
					ok = false
				}
				if ok {
					applyGraphicsState(g, newContentOpBorrowed("d", []Object{resolvedDash[0], resolvedDash[1]}, 0))
				}
			}
		}
	}
	applyNumber("RI", "ri")
	applyNumber("FL", "i")
	applyNumber("CA", "CA")
	applyNumber("ca", "ca")
	if value, ok := state[Name("BM")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		switch mode := resolved.(type) {
		case Name:
			applyGraphicsState(g, newContentOpBorrowed("BM", []Object{mode}, 0))
		case Array:
			blendModes, valid := blendModeNames(mode, d)
			if valid {
				g.blendModes = blendModes
				g.blendMode = blendModes[0]
			}
		}
	}
	if value, ok := state[Name("SMask")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if name, ok := resolved.(Name); ok && name == Name("None") {
			g.softMask, g.hasSoftMask = nil, false
		} else if _, ok := resolved.(Dict); ok {
			g.softMask, g.hasSoftMask = resolved, true
		}
	}
	if value, ok := state[Name("Font")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if font, ok := resolved.(Array); ok && len(font) == 2 {
			fontSize, _ := d.resolveIndirectChain(font[1])
			if size, sizeOK := finiteNumberValue(fontSize); sizeOK {
				if fontName, nameOK := externalFontName(op.resources, font[0], d); nameOK {
					g.fontName = fontName
				}
				g.fontSize = size
			}
		}
	}
	if value, ok := state[Name("UseBlackPtComp")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if name, ok := resolved.(Name); ok {
			g.blackPointComp = string(name)
		}
	}
	if value, ok := state[Name("SA")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if enabled, ok := resolved.(Bool); ok {
			g.strokeAdjustment = bool(enabled)
		}
	}
	if value, ok := state[Name("AIS")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if enabled, ok := resolved.(Bool); ok {
			g.alphaSource = bool(enabled)
		}
	}
	if value, ok := state[Name("TK")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if enabled, ok := resolved.(Bool); ok {
			g.knockout = bool(enabled)
		}
	}
	if value, ok := state[Name("OP")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if enabled, ok := resolved.(Bool); ok {
			g.strokeOverprint = bool(enabled)
		}
	}
	if value, ok := state[Name("op")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if enabled, ok := resolved.(Bool); ok {
			g.overprint = bool(enabled)
		}
	}
	if value, ok := state[Name("OPM")]; ok {
		resolved, _ := d.resolveIndirectChain(value)
		if mode, ok := IntValue(resolved); ok {
			g.overprintMode = mode
		}
	}
	if value, ok := state[Name("HT")]; ok {
		halftone, _ := d.resolveIndirectChain(value)
		if isHalftoneObject(halftone) {
			g.halftone = cloneGraphicsObject(halftone)
			g.hasHalftone = true
		}
	}
}

func isHalftoneObject(value Object) bool {
	switch value.(type) {
	case Name, Dict, Stream:
		return true
	default:
		return false
	}
}

func blendModeNames(values Array, d *Document) ([]string, bool) {
	if len(values) == 0 {
		return nil, false
	}
	names := make([]string, len(values))
	for i, value := range values {
		if d != nil {
			value, _ = d.resolveIndirectChain(value)
		}
		name, ok := value.(Name)
		if !ok {
			return nil, false
		}
		names[i] = string(name)
	}
	return names, true
}

// applyResourceColorSpace resolves a named content color space through the
// current resource dictionary. Playa keeps array-resource keys in
// graphics-state snapshots, uses predefined names for direct aliases, and
// derives the expected component count from the resolved definition. The
// resolved definition remains available from the ColorSpaceObject projection.
func applyResourceColorSpace(d *Document, g *graphicsState, op ContentOp) {
	if d == nil || g == nil || op.resources == nil || len(op.operandsValue()) == 0 {
		return
	}
	operator := op.operatorValue()
	selection := operator == "CS" || operator == "cs"
	colorUpdate := operator == "SC" || operator == "SCN" || operator == "sc" || operator == "scn"
	if !selection && !colorUpdate {
		return
	}
	if colorUpdate {
		if (operator == "SC" || operator == "SCN") && g.strokeIndexed {
			return
		}
		if (operator == "sc" || operator == "scn") && g.fillIndexed {
			return
		}
	}
	var name Name
	if selection {
		var ok bool
		name, ok = op.operandsValue()[0].(Name)
		if !ok {
			return
		}
	} else if operator == "SC" || operator == "SCN" {
		name = Name(g.strokeColor.Space())
	} else {
		name = Name(g.fillColor.Space())
	}
	spacesValue, _ := d.resolveIndirectChain(op.resources[Name("ColorSpace")])
	spaces, _ := spacesValue.(Dict)
	value, ok := spaces[name]
	if !ok {
		return
	}
	resolved, _ := d.resolveIndirectChain(value)
	space := ""
	switch value := resolved.(type) {
	case Name:
		space = string(value)
	case Array:
		if len(value) > 0 {
			resolvedName, _ := d.resolveIndirectChain(value[0])
			if name, ok := resolvedName.(Name); ok {
				space = string(name)
			}
		}
	}
	if space == "" {
		return
	}
	info := d.describeImageColorSpace(resolved)
	_, arraySpace := resolved.(Array)
	indexed := arraySpace && info.Name() == "Indexed" && info.Components() == 1
	if colorUpdate {
		if !indexed {
			return
		}
		if operator == "SC" || operator == "SCN" {
			g.strokeColor = normalizeIndexedColor(g.strokeColor, info.High())
		} else {
			g.fillColor = normalizeIndexedColor(g.fillColor, info.High())
		}
		return
	}
	snapshotSpace := string(name)
	if _, direct := resolved.(Name); direct {
		snapshotSpace = space
	}
	components := d.resolvedColorSpaceComponents(resolved)
	if operator == "CS" {
		g.strokeColor = g.strokeColor.WithSpace(snapshotSpace, components)
		g.strokeIndexed = indexed
		g.strokeIndexedHigh = info.High()
	} else {
		g.fillColor = g.fillColor.WithSpace(snapshotSpace, components)
		g.fillIndexed = indexed
		g.fillIndexedHigh = info.High()
	}
}

func externalFontName(resources Dict, value Object, d *Document) (string, bool) {
	fontsValue, _ := d.resolveIndirectChain(resources[Name("Font")])
	fonts, _ := fontsValue.(Dict)
	for name, candidate := range fonts {
		if ref, ok := value.(Ref); ok {
			if other, ok := candidate.(Ref); ok && ref == other {
				return string(name), true
			}
			continue
		}
		candidateResolved, _ := d.resolveIndirectChain(candidate)
		valueResolved, _ := d.resolveIndirectChain(value)
		if reflect.DeepEqual(candidateResolved, valueResolved) {
			return string(name), true
		}
	}
	return "", false
}

func graphicsStateOps(g graphicsState) []ContentOp {
	ops := []ContentOp{}
	if g.ctm != (geometry.Matrix{}) && g.ctm != identity() {
		operands := make([]Object, len(g.ctm))
		for i, value := range g.ctm {
			operands[i] = Number(value)
		}
		ops = append(ops, newContentOpBorrowed("cm", operands, 0))
	}
	appendNumber := func(operator string, value float64) {
		ops = append(ops, newContentOpBorrowed(operator, []Object{Number(value)}, 0))
	}
	appendNumber("w", g.lineWidth)
	appendNumber("J", float64(g.lineCap))
	appendNumber("j", float64(g.lineJoin))
	appendNumber("M", g.miterLimit)
	if len(g.dash) > 0 {
		dash := make(Array, len(g.dash))
		for i, value := range g.dash {
			dash[i] = Number(value)
		}
		ops = append(ops, newContentOpBorrowed("d", []Object{dash, Number(g.dashPhase)}, 0))
	}
	ops = append(ops, newContentOpBorrowed("ri", []Object{Name(g.intent)}, 0))
	appendNumber("i", g.flatness)
	appendNumber("CA", g.strokeAlpha)
	appendNumber("ca", g.fillAlpha)
	if g.blendMode != "" {
		if len(g.blendModes) > 0 {
			modes := make(Array, len(g.blendModes))
			for i, mode := range g.blendModes {
				modes[i] = Name(mode)
			}
			ops = append(ops, newContentOpBorrowed("BM", []Object{modes}, 0))
		} else {
			ops = append(ops, newContentOpBorrowed("BM", []Object{Name(g.blendMode)}, 0))
		}
	}
	ops = append(ops, newContentOpBorrowed("SA", []Object{Bool(g.strokeAdjustment)}, 0))
	ops = append(ops, newContentOpBorrowed("AIS", []Object{Bool(g.alphaSource)}, 0))
	ops = append(ops, newContentOpBorrowed("TK", []Object{Bool(g.knockout)}, 0))
	ops = append(ops, newContentOpBorrowed("OP", []Object{Bool(g.strokeOverprint)}, 0))
	ops = append(ops, newContentOpBorrowed("op", []Object{Bool(g.overprint)}, 0))
	ops = append(ops, newContentOpBorrowed("OPM", []Object{Number(float64(g.overprintMode))}, 0))
	if g.hasHalftone {
		ops = append(ops, newContentOpBorrowed("HT", []Object{cloneGraphicsObject(g.halftone)}, 0))
	}
	if g.hasSoftMask {
		if mask, ok := g.softMask.(Dict); ok {
			ops = append(ops, newContentOpBorrowed("SMask", []Object{mask}, 0))
		}
	} else {
		ops = append(ops, newContentOpBorrowed("SMask", []Object{Name("None")}, 0))
	}
	if g.blackPointComp != "" {
		ops = append(ops, newContentOpBorrowed("UseBlackPtComp", []Object{Name(g.blackPointComp)}, 0))
	}
	if g.fontName != "" {
		ops = append(ops, newContentOpBorrowed("Tf", []Object{Name(g.fontName), Number(g.fontSize)}, 0))
	}
	appendColor := func(stroke bool, color geometry.Color) {
		colorValues := color.ValuesCopy()
		values := make([]Object, 0, len(colorValues)+1)
		for _, value := range colorValues {
			values = append(values, Number(value))
		}
		if color.Pattern() != "" {
			values = append(values, Name(color.Pattern()))
		}
		operator := "scn"
		spaceOperatorName := "cs"
		if stroke {
			operator, spaceOperatorName = "SCN", "CS"
		}
		switch color.Space() {
		case "DeviceGray":
			if stroke {
				operator = "G"
			} else {
				operator = "g"
			}
		case "DeviceRGB":
			if stroke {
				operator = "RG"
			} else {
				operator = "rg"
			}
		case "DeviceCMYK":
			if stroke {
				operator = "K"
			} else {
				operator = "k"
			}
		default:
			ops = append(ops, newContentOpBorrowed(spaceOperatorName, []Object{Name(color.Space())}, 0))
		}
		if operator == "G" || operator == "g" || operator == "RG" || operator == "rg" || operator == "K" || operator == "k" {
			ops = append(ops, newContentOpBorrowed(operator, values, 0))
			return
		}
		ops = append(ops, newContentOpBorrowed(operator, values, 0))
	}
	appendColor(true, g.strokeColor)
	appendColor(false, g.fillColor)
	return ops
}
