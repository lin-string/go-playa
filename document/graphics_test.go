package document

import (
	"math"
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/geometry"
)

func TestGraphicsStateAppliesLineAndColorOperators(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("w", []Object{Number(2.5)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("J", []Object{Number(1)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("j", []Object{Number(2)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("M", []Object{Number(7)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("d", []Object{Array{Number(3), Number(4)}, Number(1)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("RG", []Object{Number(0.1), Number(0.2), Number(0.3)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("k", []Object{Number(0.1), Number(0.2), Number(0.3), Number(0.4)}, 0))
	if g.lineWidth != 2.5 || g.lineCap != 1 || g.lineJoin != 2 || g.miterLimit != 7 || g.dashPhase != 1 || len(g.dash) != 2 {
		t.Fatalf("line state = %#v", g)
	}
	if g.strokeColor.Space() != "DeviceRGB" || len(g.strokeColor.ValuesCopy()) != 3 || g.fillColor.Space() != "DeviceCMYK" || len(g.fillColor.ValuesCopy()) != 4 {
		t.Fatalf("color state = %#v", g)
	}
}

func TestGraphicsStateDoesNotTruncateFractionalIntegerOperators(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("J", []Object{Number(1)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("j", []Object{Number(1)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("Tr", []Object{Number(3)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("OPM", []Object{Number(1)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("J", []Object{Number(1.5)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("j", []Object{Number(0.5)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("Tr", []Object{Number(3.5)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("OPM", []Object{Number(1.5)}, 0))
	if g.lineCap != 1 || g.lineJoin != 1 || g.renderMode != 3 || g.overprintMode != 1 {
		t.Fatalf("fractional integer operators changed state: %#v", g)
	}
}

func TestGraphicsStateIgnoresOutOfRangeIntegerOperators(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("J", []Object{Number(1)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("j", []Object{Number(2)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("Tr", []Object{Number(3)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("OPM", []Object{Number(1)}, 0))

	for _, op := range []ContentOp{
		newContentOpBorrowed("J", []Object{Number(-1)}, 0),
		newContentOpBorrowed("J", []Object{Number(3)}, 0),
		newContentOpBorrowed("j", []Object{Number(-1)}, 0),
		newContentOpBorrowed("j", []Object{Number(3)}, 0),
		newContentOpBorrowed("Tr", []Object{Number(-1)}, 0),
		newContentOpBorrowed("Tr", []Object{Number(8)}, 0),
		newContentOpBorrowed("OPM", []Object{Number(-1)}, 0),
		newContentOpBorrowed("OPM", []Object{Number(2)}, 0),
	} {
		applyGraphicsState(&g, op)
	}
	if g.lineCap != 1 || g.lineJoin != 2 || g.renderMode != 3 || g.overprintMode != 1 {
		t.Fatalf("out-of-range integer operator changed state: %#v", g)
	}
}

func TestGraphicsStatePadsMissingDeviceColorComponents(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("rg", []Object{Number(0.25)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("K", []Object{Number(0.1), Number(0.2)}, 0))
	cmyk := g.strokeColor.ValuesCopy()
	applyGraphicsState(&g, newContentOpBorrowed("RG", []Object{Number(0.1), Number(0.2), Number(0.3), Number(0.4)}, 0))
	if got := g.fillColor.ValuesCopy(); !reflect.DeepEqual(got, []float64{0.25, 0, 0}) {
		t.Fatalf("padded RGB color = %#v", got)
	}
	if !reflect.DeepEqual(cmyk, []float64{0.1, 0.2, 0, 0}) {
		t.Fatalf("padded CMYK color = %#v", cmyk)
	}
	if got := g.strokeColor.ValuesCopy(); !reflect.DeepEqual(got, []float64{0.1, 0.2, 0.3}) {
		t.Fatalf("truncated RGB color = %#v", got)
	}
}

func TestColorComponentsPreservesResourceArity(t *testing.T) {
	color := geometry.NewColor("DeviceRGB", []float64{0.25}, "", 3)
	if got := color.Components(); got != 3 {
		t.Fatalf("color components = %d, want 3", got)
	}
	if got := geometry.NewColor("", []float64{0.25, 0.5}, "", 0).Components(); got != 2 {
		t.Fatalf("fallback color components = %d, want 2", got)
	}
}

func TestGraphicsStateCloneCopiesMutableValues(t *testing.T) {
	g := newGraphicsState()
	g.dash = []float64{1, 2}
	g.strokeColor = geometry.NewColor("DeviceGray", []float64{0.5}, "", 1)
	c := g.Clone()
	c.dash[0] = 9
	values := c.strokeColor.ValuesCopy()
	values[0] = 8
	if g.dash[0] != 1 || g.strokeColor.ValuesCopy()[0] != 0.5 {
		t.Fatalf("clone shares mutable state: original=%#v clone=%#v", g, c)
	}
}

func TestGraphicsStateClonePreservesNilGraphicsObjects(t *testing.T) {
	var nilArray Array
	var nilInvalid InvalidArray
	var nilDict Dict

	g := newGraphicsState()
	g.halftone = Array{nilArray, nilInvalid, nilDict}
	g.softMask = nilDict
	clone := g.Clone()

	objects, ok := clone.halftone.(Array)
	if !ok || len(objects) != 3 {
		t.Fatalf("cloned graphics objects = %#v", clone.halftone)
	}
	if got, ok := objects[0].(Array); !ok || got != nil {
		t.Fatalf("nil array was not preserved: %#v", objects[0])
	}
	if got, ok := objects[1].(InvalidArray); !ok || got != nil {
		t.Fatalf("nil invalid array was not preserved: %#v", objects[1])
	}
	if got, ok := objects[2].(Dict); !ok || got != nil {
		t.Fatalf("nil dictionary was not preserved: %#v", objects[2])
	}
	if got, ok := clone.softMask.(Dict); !ok || got != nil {
		t.Fatalf("nil soft mask was not preserved: %#v", clone.softMask)
	}
}

func TestColorFinalizeCopiesComponents(t *testing.T) {
	original := geometry.NewColor("DeviceRGB", []float64{0.1, 0.2, 0.3}, "", 3)
	final := original.Finalize()
	values := final.ValuesCopy()
	values[0] = 1
	if original.ValuesCopy()[0] != 0.1 {
		t.Fatalf("finalized color shares components: original=%#v final=%#v", original, final)
	}
}

func TestGraphicsSnapshotsPreserveEmptySlices(t *testing.T) {
	color := geometry.NewColor("", make([]float64, 0), "", 0)
	if color.ValuesCopy() == nil || color.Finalize().ValuesCopy() == nil {
		t.Fatal("empty color components became nil")
	}
	state := graphicsState{
		dash:        make([]float64, 0),
		blendModes:  make([]string, 0),
		strokeColor: color,
		fillColor:   color,
	}
	snapshot := state.Clone()
	if snapshot.dash == nil || snapshot.blendModes == nil || snapshot.strokeColor.ValuesCopy() == nil || snapshot.fillColor.ValuesCopy() == nil {
		t.Fatalf("empty graphics slices were not preserved: %#v", snapshot)
	}
	if state.DashCopy() == nil || state.BlendModesCopy() == nil {
		t.Fatal("empty graphics copies became nil")
	}
}

func TestGraphicsStateTracksTransparencyAndBlendMode(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("CA", []Object{Number(0.25)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("ca", []Object{Number(0.5)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("BM", []Object{Name("Multiply")}, 0))
	if g.strokeAlpha != 0.25 || g.fillAlpha != 0.5 || g.alpha != 0.5 || g.blendMode != "Multiply" {
		t.Fatalf("transparency state = %#v", g)
	}
}

func TestGraphicsStatePreservesNamedColorSpaces(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("CS", []Object{Name("CS1")}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("SCN", []Object{Number(0.2), Number(0.4)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("cs", []Object{Name("Pattern")}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("scn", []Object{Number(1)}, 0))
	if g.strokeColor.Space() != "CS1" || g.fillColor.Space() != "Pattern" || len(g.strokeColor.ValuesCopy()) != 2 || len(g.fillColor.ValuesCopy()) != 1 {
		t.Fatalf("named color state = %#v", g)
	}
}

func TestGraphicsStatePreservesPatternColorNames(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("CS", []Object{Name("Pattern")}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("SCN", []Object{Name("P1")}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("cs", []Object{Name("Pattern")}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("scn", []Object{Number(0.5), Name("P2")}, 0))
	if g.strokeColor.Pattern() != "P1" || len(g.strokeColor.ValuesCopy()) != 0 || g.fillColor.Pattern() != "P2" || len(g.fillColor.ValuesCopy()) != 1 {
		t.Fatalf("pattern color state = %#v", g)
	}
}

func TestGraphicsStateTracksRenderingQualityDefaultsAndOperators(t *testing.T) {
	g := newGraphicsState()
	if g.intent != "RelativeColorimetric" || g.flatness != 1 || !g.knockout {
		t.Fatalf("graphics defaults = %#v", g)
	}
	applyGraphicsState(&g, newContentOpBorrowed("ri", []Object{Name("Perceptual")}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("i", []Object{Number(0.5)}, 0))
	if g.intent != "Perceptual" || g.flatness != 0.5 {
		t.Fatalf("rendering quality state = %#v", g)
	}
}

func TestGraphicsStateIgnoresMalformedDashOperator(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("d", []Object{Array{Number(1), String("invalid")}, Number(2)}, 0))
	if len(g.dash) != 0 || g.dashPhase != 0 {
		t.Fatalf("malformed dash operator was partially applied: %#v", g)
	}
}

func TestGraphicsStateIgnoresExtraMatrixOperands(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("cm", []Object{
		Number(1), Number(0), Number(0), Number(1), Number(10), Number(20), Number(99),
	}, 0))
	if g.ctm != (geometry.Matrix{1, 0, 0, 1, 0, 0}) {
		t.Fatalf("matrix with extra operands was applied: %v", g.ctm)
	}
}

func TestGraphicsStateIgnoresOverflowingMatrixProduct(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("cm", []Object{
		Number(math.MaxFloat64), Number(0), Number(0), Number(1), Number(0), Number(0),
	}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("cm", []Object{
		Number(math.MaxFloat64), Number(0), Number(0), Number(1), Number(0), Number(0),
	}, 0))
	if g.ctm != (geometry.Matrix{math.MaxFloat64, 0, 0, 1, 0, 0}) {
		t.Fatalf("overflowing matrix product changed CTM: %v", g.ctm)
	}
}

func TestGraphicsStateIgnoresNonFiniteNumbers(t *testing.T) {
	for _, value := range []Number{Number(math.NaN()), Number(math.Inf(1))} {
		g := newGraphicsState()
		applyGraphicsState(&g, newContentOpBorrowed("cm", []Object{
			value, Number(0), Number(0), Number(1), Number(0), Number(0),
		}, 0))
		applyGraphicsState(&g, newContentOpBorrowed("d", []Object{
			Array{value}, Number(1),
		}, 0))
		if g.ctm != identity() || len(g.dash) != 0 || g.dashPhase != 0 {
			t.Fatalf("non-finite graphics number %v was applied: %#v", value, g)
		}
	}
}

func TestGraphicsStateIgnoresNonFiniteDeviceColors(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("RG", []Object{Number(0.1), Number(0.2), Number(0.3)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("RG", []Object{Number(math.NaN()), Number(0.8), Number(0.9)}, 0))
	if !reflect.DeepEqual(g.strokeColor.ValuesCopy(), []float64{0.1, 0.2, 0.3}) {
		t.Fatalf("non-finite device color changed state = %#v", g.strokeColor)
	}
}

func TestGraphicsStateIgnoresOutOfRangeAlpha(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("CA", []Object{Number(0.25)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("ca", []Object{Number(0.5)}, 0))
	for _, op := range []ContentOp{
		newContentOpBorrowed("CA", []Object{Number(-0.1)}, 0),
		newContentOpBorrowed("CA", []Object{Number(1.1)}, 0),
		newContentOpBorrowed("ca", []Object{Number(-0.1)}, 0),
		newContentOpBorrowed("ca", []Object{Number(1.1)}, 0),
	} {
		applyGraphicsState(&g, op)
	}
	if g.strokeAlpha != 0.25 || g.fillAlpha != 0.5 || g.alpha != 0.5 {
		t.Fatalf("out-of-range alpha changed state: %#v", g)
	}
}

func TestGraphicsStateTracksOverprintAndHalftone(t *testing.T) {
	g := newGraphicsState()
	applyGraphicsState(&g, newContentOpBorrowed("OP", []Object{Bool(true)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("op", []Object{Bool(false)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("OPM", []Object{Number(1)}, 0))
	applyGraphicsState(&g, newContentOpBorrowed("HT", []Object{Dict{Name("Type"): Number(halftoneType)}}, 0))
	if !g.strokeOverprint || g.overprint || g.overprintMode != 1 || !g.hasHalftone {
		t.Fatalf("overprint state = %#v", g)
	}
	clone := g.Clone()
	halftone, ok := clone.HalftoneCopy()
	if !clone.hasHalftone || !ok || halftone == nil {
		t.Fatalf("halftone not retained by clone: %#v", clone)
	}
}

func TestGraphicsStateIgnoresMalformedHalftone(t *testing.T) {
	g := newGraphicsState()
	g.halftone = Dict{Name("Type"): Number(5)}
	g.hasHalftone = true
	applyGraphicsState(&g, newContentOpBorrowed("HT", []Object{Number(1)}, 0))
	if halftone, ok := g.halftone.(Dict); !g.hasHalftone || !ok || halftone[Name("Type")] != Number(5) {
		t.Fatalf("malformed halftone changed graphics state: %#v", g)
	}
}

const halftoneType = 5

func TestExternalGraphicsStateAppliesResourceValues(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{Name("ExtGState"): Dict{
		Name("GS1"): Dict{
			Name("LW"): Number(2), Name("RI"): Name("Perceptual"), Name("FL"): Number(0.25),
			Name("SA"): Bool(true), Name("AIS"): Bool(true), Name("TK"): Bool(false),
		},
	}})
	applyExternalGraphicsState(d, &g, op)
	if g.lineWidth != 2 || g.intent != "Perceptual" || g.flatness != 0.25 || !g.strokeAdjustment || !g.alphaSource || g.knockout {
		t.Fatalf("external graphics state = %#v", g)
	}
}

func TestExternalGraphicsStateAppliesOverprintAndHalftone(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{Name("ExtGState"): Dict{
		Name("GS1"): Dict{Name("OP"): Bool(true), Name("op"): Bool(true), Name("OPM"): Number(1), Name("HT"): Dict{Name("Type"): Number(5)}},
	}})
	applyExternalGraphicsState(d, &g, op)
	if !g.strokeOverprint || !g.overprint || g.overprintMode != 1 || !g.hasHalftone {
		t.Fatalf("external overprint state = %#v", g)
	}
}

func TestExternalGraphicsStateIgnoresMalformedHalftone(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	g.halftone = Dict{Name("Type"): Number(5)}
	g.hasHalftone = true
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{Name("ExtGState"): Dict{
		Name("GS1"): Dict{Name("HT"): Number(1)},
	}})
	applyExternalGraphicsState(d, &g, op)
	if halftone, ok := g.halftone.(Dict); !g.hasHalftone || !ok || halftone[Name("Type")] != Number(5) {
		t.Fatalf("malformed external halftone changed graphics state: %#v", g)
	}
}

func TestExternalGraphicsStateResolvesIndirectDashValues(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Array{Number(2), Number(3)},
		{Object: 2}: Number(4),
	}}
	g := newGraphicsState()
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{Name("ExtGState"): Dict{
		Name("GS1"): Dict{Name("D"): Array{Ref{Object: 1}, Ref{Object: 2}}},
	}})
	applyExternalGraphicsState(d, &g, op)
	if len(g.dash) != 2 || g.dash[0] != 2 || g.dash[1] != 3 || g.dashPhase != 4 {
		t.Fatalf("indirect dash state = %#v", g)
	}
}

func TestExternalGraphicsStateResolvesMultiLevelParameterValues(t *testing.T) {
	font := Ref{Object: 20}
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{
			Name("D"):              Ref{Object: 3},
			Name("BM"):             Ref{Object: 7},
			Name("SMask"):          Ref{Object: 11},
			Name("Font"):           Ref{Object: 15},
			Name("SA"):             Ref{Object: 18},
			Name("OPM"):            Ref{Object: 19},
			Name("UseBlackPtComp"): Ref{Object: 21},
		},
		{Object: 3}:  Ref{Object: 4},
		{Object: 4}:  Array{Ref{Object: 5}, Ref{Object: 6}},
		{Object: 5}:  Ref{Object: 22},
		{Object: 6}:  Ref{Object: 23},
		{Object: 7}:  Ref{Object: 8},
		{Object: 8}:  Array{Ref{Object: 9}, Ref{Object: 10}},
		{Object: 9}:  Ref{Object: 24},
		{Object: 10}: Name("Screen"),
		{Object: 11}: Ref{Object: 12},
		{Object: 12}: Dict{Name("Type"): Name("Mask")},
		{Object: 15}: Ref{Object: 16},
		{Object: 16}: Array{font, Ref{Object: 17}},
		{Object: 17}: Ref{Object: 25},
		{Object: 18}: Ref{Object: 26},
		{Object: 19}: Ref{Object: 27},
		{Object: 21}: Ref{Object: 28},
		{Object: 22}: Array{Number(2), Number(3)},
		{Object: 23}: Number(4),
		{Object: 24}: Name("Multiply"),
		{Object: 25}: Number(14),
		{Object: 26}: Bool(true),
		{Object: 27}: Number(1),
		{Object: 28}: Name("Default"),
	}}
	g := newGraphicsState()
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{
		Name("Font"):      Dict{Name("F1"): font},
		Name("ExtGState"): Dict{Name("GS1"): Ref{Object: 1}},
	})
	applyExternalGraphicsState(d, &g, op)
	if len(g.dash) != 2 || g.dash[0] != 2 || g.dash[1] != 3 || g.dashPhase != 4 {
		t.Fatalf("multi-level dash state = %#v", g)
	}
	if g.blendMode != "Multiply" || len(g.blendModes) != 2 || g.blendModes[1] != "Screen" {
		t.Fatalf("multi-level blend state = %#v", g)
	}
	if !g.hasSoftMask || g.fontName != "F1" || g.fontSize != 14 || !g.strokeAdjustment || g.overprintMode != 1 || g.blackPointComp != "Default" {
		t.Fatalf("multi-level composite state = %#v", g)
	}
}

func TestExternalGraphicsStateIgnoresMalformedDashValues(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{Name("ExtGState"): Dict{
		Name("GS1"): Dict{Name("D"): Array{Array{Number(1), String("invalid")}, Number(2)}, Name("LW"): Number(2), Name("RI"): Name("Perceptual")},
	}})
	applyExternalGraphicsState(d, &g, op)
	if len(g.dash) != 0 || g.dashPhase != 0 || g.lineWidth != 2 || g.intent != "Perceptual" {
		t.Fatalf("malformed external dash state was applied: %#v", g)
	}
}

func TestExternalGraphicsStatePreservesCompositeValues(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{Name("ExtGState"): Dict{
		Name("GS1"): Dict{
			Name("BM"):             Array{Name("Multiply"), Name("Screen")},
			Name("SMask"):          Dict{Name("G"): Name("DeviceGray"), Name("BC"): Array{Number(0)}},
			Name("UseBlackPtComp"): Name("Default"),
		},
	}})
	applyExternalGraphicsState(d, &g, op)
	if g.blendMode != "Multiply" || len(g.blendModes) != 2 || g.blendModes[1] != "Screen" || !g.hasSoftMask || g.blackPointComp != "Default" {
		t.Fatalf("composite external graphics state = %#v", g)
	}
	clone := g.Clone()
	clone.blendModes[0] = "Normal"
	if g.blendModes[0] != "Multiply" {
		t.Fatal("graphics-state clone shares blend-mode slice")
	}
	clone.softMask.(Dict)[Name("G")] = Name("DeviceRGB")
	if g.softMask.(Dict)[Name("G")] != Name("DeviceGray") {
		t.Fatal("graphics-state clone shares soft-mask dictionary")
	}
	clone.softMask.(Dict)[Name("BC")].(Array)[0] = Number(0.5)
	if g.softMask.(Dict)[Name("BC")].(Array)[0] != Number(0) {
		t.Fatal("graphics-state clone shares soft-mask array")
	}
}

func TestExternalGraphicsStateIgnoresMalformedBlendModeArray(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	g.blendMode = "Multiply"
	g.blendModes = []string{"Multiply", "Screen"}
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{Name("ExtGState"): Dict{
		Name("GS1"): Dict{Name("BM"): Array{Name("Screen"), Number(1)}},
	}})
	applyExternalGraphicsState(d, &g, op)
	if g.blendMode != "Multiply" || !reflect.DeepEqual(g.blendModes, []string{"Multiply", "Screen"}) {
		t.Fatalf("malformed blend mode array was applied: %#v", g)
	}
}

func TestGraphicsStateIgnoresMalformedBlendModeArrayAtomically(t *testing.T) {
	g := newGraphicsState()
	g.blendMode = "Multiply"
	g.blendModes = []string{"Multiply", "Screen"}
	applyGraphicsState(&g, newContentOpBorrowed("BM", []Object{
		Array{Name("Screen"), String("invalid")},
	}, 0))
	if g.blendMode != "Multiply" || !reflect.DeepEqual(g.blendModes, []string{"Multiply", "Screen"}) {
		t.Fatalf("malformed direct blend mode array changed state: %#v", g)
	}
}

func TestGraphicsStateSingleBlendModeReplacesModeArray(t *testing.T) {
	g := newGraphicsState()
	g.blendMode = "Multiply"
	g.blendModes = []string{"Multiply", "Screen"}
	applyGraphicsState(&g, newContentOpBorrowed("BM", []Object{Name("Darken")}, 0))
	if g.blendMode != "Darken" || len(g.blendModes) != 0 {
		t.Fatalf("single blend mode retained stale array: %#v", g)
	}
}

func TestExternalGraphicsStateAppliesFontResource(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	font := Ref{Object: 11, Generation: 0}
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{
		Name("Font"):      Dict{Name("F1"): font},
		Name("ExtGState"): Dict{Name("GS1"): Dict{Name("Font"): Array{font, Number(14)}}},
	})
	applyExternalGraphicsState(d, &g, op)
	if g.fontName != "F1" || g.fontSize != 14 {
		t.Fatalf("external font state = %#v", g)
	}
}

func TestExternalGraphicsStateIgnoresMalformedFontArray(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	font := Ref{Object: 11, Generation: 0}
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{
		Name("Font"):      Dict{Name("F1"): font},
		Name("ExtGState"): Dict{Name("GS1"): Dict{Name("Font"): Array{font, Number(14), Number(2)}}},
	})
	applyExternalGraphicsState(d, &g, op)
	if g.fontName != "" || g.fontSize != 0 {
		t.Fatalf("malformed external font state was applied: %#v", g)
	}
}

func TestExternalGraphicsStateKeepsFontStateForNonFiniteFontSize(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	g.fontName, g.fontSize = "Previous", 9
	font := Ref{Object: 11, Generation: 0}
	op := newContentOpWithResources("gs", []Object{Name("GS1")}, Dict{
		Name("Font"):      Dict{Name("F1"): font},
		Name("ExtGState"): Dict{Name("GS1"): Dict{Name("Font"): Array{font, Number(math.NaN())}}},
	})
	applyExternalGraphicsState(d, &g, op)
	if g.fontName != "Previous" || g.fontSize != 9 {
		t.Fatalf("non-finite external font state changed state: %#v", g)
	}
}

func TestGraphicsStateOpsRoundTripExtendedState(t *testing.T) {
	want := newGraphicsState()
	want.strokeAdjustment = true
	want.alphaSource = true
	want.knockout = false
	want.blendMode = "Multiply"
	want.blendModes = []string{"Multiply", "Screen"}
	want.softMask = Dict{Name("G"): Name("DeviceGray")}
	want.hasSoftMask = true
	want.blackPointComp = "Default"
	got := newGraphicsState()
	for _, op := range graphicsStateOps(want) {
		applyGraphicsState(&got, op)
	}
	if !got.strokeAdjustment || !got.alphaSource || got.knockout || got.blendMode != "Multiply" || len(got.blendModes) != 2 || !got.hasSoftMask || got.blackPointComp != "Default" {
		t.Fatalf("extended state round trip = %#v", got)
	}
}

func TestResourceColorSpaceResolution(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	op := newContentOpWithResources("CS", []Object{Name("CS1")}, Dict{Name("ColorSpace"): Dict{
		Name("CS1"): Array{Name("ICCBased"), newStream(Dict{Name("N"): Number(3)}, nil)},
	}})
	applyGraphicsState(&g, op)
	applyResourceColorSpace(d, &g, op)
	if g.strokeColor.Space() != "CS1" || g.strokeColor.Components() != 3 {
		t.Fatalf("resource color space = %#v, want resource key CS1 with three components", g.strokeColor)
	}
}

func TestResourceColorSpaceKeepsResourceKeyForSeparationLikePlaya(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	op := newContentOpWithResources("cs", []Object{Name("R13")}, Dict{Name("ColorSpace"): Dict{
		Name("R13"): Array{Name("Separation"), Name("SpotBlue"), Name("DeviceCMYK"), Null{}},
	}})
	applyGraphicsState(&g, op)
	applyResourceColorSpace(d, &g, op)
	if g.fillColor.Space() != "R13" || g.fillColor.Components() != 1 {
		t.Fatalf("Separation resource color space = %#v, want resource key R13 with one component", g.fillColor)
	}
}

func TestResourceColorSpaceUsesPredefinedNameForDirectAlias(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	op := newContentOpWithResources("cs", []Object{Name("CS1")}, Dict{Name("ColorSpace"): Dict{
		Name("CS1"): Name("DeviceRGB"),
	}})
	applyGraphicsState(&g, op)
	applyResourceColorSpace(d, &g, op)
	if g.fillColor.Space() != "DeviceRGB" || g.fillColor.Components() != 3 {
		t.Fatalf("direct color-space alias = %#v, want predefined DeviceRGB with three components", g.fillColor)
	}
}

func TestResourceColorSpaceFollowsMultiLevelIndirectArrayName(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Ref{Object: 3},
		{Object: 3}: Name("ICCBased"),
	}}
	g := newGraphicsState()
	op := newContentOpWithResources("CS", []Object{Name("CS1")}, Dict{Name("ColorSpace"): Dict{
		Name("CS1"): Array{Ref{Object: 1}, newStream(Dict{Name("N"): Number(3)}, nil)},
	}})
	applyResourceColorSpace(d, &g, op)
	if g.strokeColor.Space() != "CS1" || g.strokeColor.Components() != 3 {
		t.Fatalf("indirect resource color space = %#v, want resource key CS1 with three components", g.strokeColor)
	}
}

func TestResourceColorSpacePadsSpecializedComponents(t *testing.T) {
	d := &Document{}
	g := newGraphicsState()
	resources := Dict{Name("ColorSpace"): Dict{
		Name("DN"): Array{Name("DeviceN"), Array{Name("C1"), Name("C2")}, Name("DeviceRGB"), Null{}},
		Name("PT"): Array{Name("Pattern"), Name("DeviceRGB")},
	}}
	apply := func(space string, operands ...Object) {
		cs := newContentOpWithResources("cs", []Object{Name(space)}, resources)
		applyGraphicsState(&g, cs)
		applyResourceColorSpace(d, &g, cs)
		applyGraphicsState(&g, newContentOpBorrowed("scn", operands, 0))
	}
	apply("DN", Number(0.5))
	if got := g.fillColor.ValuesCopy(); !reflect.DeepEqual(got, []float64{0.5, 0}) {
		t.Fatalf("DeviceN color = %#v", got)
	}
	apply("PT", Number(0.25), Name("P1"))
	if got := g.fillColor.ValuesCopy(); !reflect.DeepEqual(got, []float64{0.25, 0, 0}) || g.fillColor.Pattern() != "P1" {
		t.Fatalf("Pattern color = %#v", g.fillColor)
	}
}

func TestResourceIndexedColorSpaceNormalizesIndices(t *testing.T) {
	resources := Dict{Name("ColorSpace"): Dict{
		Name("CS1"): Array{Name("Indexed"), Name("DeviceRGB"), Number(7), String([]byte{
			0x00, 0x80, 0x00, 0xff, 0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0xff,
			0x00, 0xff, 0xff, 0xff, 0x00, 0xff, 0xff, 0xff, 0x00, 0xf3, 0x80, 0xff,
		})},
	}}
	for _, stroke := range []bool{false, true} {
		g := newGraphicsState()
		selectionOperator, colorOperator := "cs", "sc"
		if stroke {
			selectionOperator, colorOperator = "CS", "SC"
		}
		selection := newContentOpWithResources(selectionOperator, []Object{Name("CS1")}, resources)
		applyGraphicsState(&g, selection)
		applyResourceColorSpace(&Document{}, &g, selection)
		for _, test := range []struct {
			value float64
			want  float64
		}{
			{value: -17, want: 0},
			{value: 0.49, want: 0},
			{value: 0.5, want: 1},
			{value: 6.49, want: 6},
			{value: 6.5, want: 7},
			{value: 17, want: 7},
		} {
			color := newContentOpWithResources(colorOperator, []Object{Number(test.value)}, resources)
			applyGraphicsState(&g, color)
			applyResourceColorSpace(&Document{}, &g, color)
			got := g.fillColor.ValuesCopy()
			if stroke {
				got = g.strokeColor.ValuesCopy()
			}
			if !reflect.DeepEqual(got, []float64{test.want}) {
				t.Errorf("%s %v = %#v, want [%v]", colorOperator, test.value, got, test.want)
			}
			if value, ok := finiteNumberValue(color.operandsValue()[0]); !ok || value != test.value {
				t.Errorf("%s raw operand = %#v, want %v", colorOperator, color.operandsValue()[0], test.value)
			}
		}
		deviceSelection := newContentOpBorrowed(selectionOperator, []Object{Name("DeviceGray")}, 0)
		applyGraphicsState(&g, deviceSelection)
		deviceColor := newContentOpBorrowed(colorOperator, []Object{Number(0.25)}, 0)
		applyGraphicsState(&g, deviceColor)
		got := g.fillColor.ValuesCopy()
		if stroke {
			got = g.strokeColor.ValuesCopy()
		}
		if !reflect.DeepEqual(got, []float64{0.25}) {
			t.Errorf("%s retained stale Indexed metadata: %#v", colorOperator, got)
		}
	}
}

func TestPublicResourceIndexedColorSpacePreservesNormalizationMetadata(t *testing.T) {
	resources := Dict{Name("ColorSpace"): Dict{
		Name("CS1"): Array{Name("Indexed"), Name("DeviceRGB"), Number(7), String(make([]byte, 24))},
	}}
	state := DefaultGraphicsState()
	selection := newContentOpWithResources("cs", []Object{Name("CS1")}, resources)
	ApplyGraphicsState(&state, selection)
	ApplyResourceColorSpace(&Document{}, &state, selection)
	color := newContentOpWithResources("sc", []Object{Number(6.5)}, resources)
	ApplyGraphicsState(&state, color)
	ApplyResourceColorSpace(&Document{}, &state, color)
	if got := state.FillColor().ValuesCopy(); !reflect.DeepEqual(got, []float64{7}) {
		t.Fatalf("public Indexed fill color = %#v, want [7]", got)
	}
}
