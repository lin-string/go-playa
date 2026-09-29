package playa_test

import (
	"context"
	"testing"

	playa "github.com/lin-string/go-playa"
	"github.com/lin-string/go-playa/content"
	"github.com/lin-string/go-playa/internal/testfixture"
	"github.com/lin-string/go-playa/page"
	"github.com/lin-string/go-playa/pdftypes"
)

func TestRootFacadeAndDomainPackagesSharePageTypes(t *testing.T) {
	document, err := playa.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := document.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	pages, err := document.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	acceptPage(pages[0])
	var _ content.TextObject
	var _ playa.TextWord
	var _ playa.TextLine
	var _ playa.TextParagraph
	var _ playa.TextBox
	var _ playa.TextGroup
	var _ playa.TextGroupChild
	var _ playa.LayoutOptions
	var _ playa.LayoutResult
	var _ playa.LayoutItem
	var _ playa.LayoutItemKind
	_ = playa.LayoutTextBox
	_ = playa.LayoutImage
	_ = playa.LayoutPath
	_ = playa.LayoutXObject
	var _ playa.Matrix
	var _ playa.Token
	var _ playa.TokenKind
	var _ playa.Stream
	var _ playa.Null
	var _ playa.Bool
	var _ playa.Number
	var _ playa.Name
	var _ playa.String
	var _ playa.Keyword
	var _ playa.ContentKind
	var _ playa.ContentFilter
	var _ playa.ContentOptions
	_ = playa.ContentText
	_ = playa.ContentXObject
	_ = playa.TokenEOF
	_ = playa.TokenLiteral
	_ = playa.TokenKeyword
	_ = playa.StructureMarkedContent
	_ = playa.StructureObject
	_ = playa.StructureItemElement
	_ = playa.StructureItemMarkedContent
	_ = playa.StructureItemObject
	var _ playa.ContentOp
	var _ playa.ContentSection
	var _ playa.ContentSequence
	var _ playa.PageList
	_ = (playa.ContentObject{}).Len
	_ = (playa.TextObject{}).Len
	_ = (playa.GlyphObject{}).ContentSeq
	_ = (playa.GlyphObject{}).Len
	_ = (playa.XObjectObject{}).Len
	var _ playa.GraphicsState
	var _ playa.DashPattern
	var _ playa.Color
	var _ playa.ColorSpace
	var _ playa.DeviceSpace
	var _ playa.Point
	var _ playa.Rect
	var _ playa.BBoxProvider
	var _ playa.ObjRef
	var _ playa.ContentStream
	var _ playa.PathSegment
	var _ playa.XObjectObject
	var _ playa.ShadingObject
	var _ playa.PatternObject
	var _ playa.ExtGStateObject
	var _ playa.ColorSpaceObject
	var _ playa.PropertiesObject
	var _ playa.FontResource
	var _ playa.FontMetadata
	var _ playa.Annotation
	var _ playa.FormField
	var _ playa.PageStructureEntry
	var _ playa.PageLabelSpec
	var _ playa.IndirectObject
	var _ playa.StructElement
	var _ playa.StructureContent
	var _ playa.StructureContentKind
	var _ playa.StructureItem
	var _ playa.StructureItemKind
	var _ playa.StructureIndex
	var _ playa.OutlineNode
	var _ playa.DecodedImage
	var _ playa.ImageColorSpace
	_ = playa.DefaultGraphicsState
	_ = playa.Resolve
	_ = playa.ResolveAll
	_ = playa.AsObject
	_ = playa.NewColorSpace
}

func TestRootResolverFacadeMatchesPDFObjectSemantics(t *testing.T) {
	objects := map[playa.ObjRef]playa.Object{
		{Object: 1}: playa.Ref{Object: 2},
		{Object: 2}: playa.Number(7),
	}
	resolve := func(ref playa.ObjRef) (playa.Object, bool) {
		value, ok := objects[ref]
		return value, ok
	}
	if value := playa.Resolve(playa.Ref{Object: 1}, resolve); value != (playa.Number(7)) {
		t.Fatalf("Resolve() = %#v, want 7", value)
	}
	if value := playa.Resolve(playa.Ref{Object: 99}, resolve, playa.Name("missing")); value != (playa.Name("missing")) {
		t.Fatalf("Resolve() missing default = %#v, want missing", value)
	}
	cycle := map[playa.ObjRef]playa.Object{{Object: 3}: playa.Ref{Object: 3}}
	if value := playa.Resolve(playa.Ref{Object: 3}, func(ref playa.ObjRef) (playa.Object, bool) {
		value, ok := cycle[ref]
		return value, ok
	}, playa.Null{}); value != (playa.Null{}) {
		t.Fatalf("Resolve() cycle = %#v, want null default", value)
	}
	resolved := playa.ResolveAll(playa.Array{playa.Ref{Object: 2}}, resolve)
	array, ok := resolved.(playa.Array)
	if !ok || len(array) != 1 || array[0] != (playa.Number(7)) {
		t.Fatalf("ResolveAll() = %#v, want [7]", resolved)
	}
}

func TestRootColorSpaceMakesOwnedColors(t *testing.T) {
	space := playa.NewColorSpace("DeviceRGB", 3)
	color := space.MakeColor(playa.Number(0.25), playa.Number(0.5))
	if color.Space() != "DeviceRGB" || color.Components() != 3 {
		t.Fatalf("color space metadata = %#v", color)
	}
	values := color.ValuesCopy()
	if len(values) != 3 || values[0] != 0.25 || values[1] != 0.5 || values[2] != 0 {
		t.Fatalf("padded color components = %v", values)
	}
	pattern := playa.NewColorSpace("Pattern", 1).MakeColor(playa.Name("P1"))
	if pattern.Pattern() != "P1" || pattern.Components() != 0 {
		t.Fatalf("pattern color = %#v", pattern)
	}
	indexed := playa.NewColorSpace("Indexed", 1).WithHigh(7).MakeColor(playa.Number(6.5))
	if got := indexed.ValuesCopy(); len(got) != 1 || got[0] != 7 {
		t.Fatalf("Indexed color = %#v, want [7]", got)
	}
}

func TestRootAsObjectProjectsPDFPrimitives(t *testing.T) {
	value := playa.Dict{
		playa.Name("Name"):  playa.String("text"),
		playa.Name("Ref"):   playa.Ref{Object: 7},
		playa.Name("Items"): playa.Array{playa.Number(2), playa.String([]byte{0xff})},
	}
	projected, ok := playa.AsObject(value).(map[string]any)
	if !ok || projected["Name"] != "text" || projected["Ref"].(map[string]any)["ref"] != 7 {
		t.Fatalf("AsObject() = %#v", projected)
	}
	items, ok := projected["Items"].([]any)
	if !ok || items[0] != float64(2) || items[1] != "base64:/w==" {
		t.Fatalf("AsObject() items = %#v", projected["Items"])
	}
}

func TestRootPrimitiveFacadeUsesPDFTypes(t *testing.T) {
	acceptRootPrimitives(pdftypes.Number(1), pdftypes.Ref{Object: 1}, pdftypes.Dict{}, pdftypes.Array{pdftypes.Number(1)}, pdftypes.Stream{})
	value, ok := pdftypes.NumberValue(pdftypes.Number(1))
	if !ok || value != 1 {
		t.Fatalf("root primitive facade is not aligned with pdftypes: %v, %v", value, ok)
	}
}

func TestRootFacadeExportsObjectLookupError(t *testing.T) {
	if playa.ErrObjectNotFound == nil {
		t.Fatal("root ErrObjectNotFound is nil")
	}
	if playa.ErrNilDocument == nil {
		t.Fatal("root ErrNilDocument is nil")
	}
	if playa.ErrPageNotFound == nil {
		t.Fatal("root ErrPageNotFound is nil")
	}
}

func acceptRootPrimitives(object playa.Object, ref playa.Ref, dict playa.Dict, array playa.Array, stream playa.Stream) {
	if object == nil || ref.Object != 1 || dict == nil || len(array) != 1 || stream.Buffer() != nil {
		panic("root primitive facade is not aligned with pdftypes")
	}
}

func TestDocumentCloseIsIdempotent(t *testing.T) {
	document, err := playa.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := document.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := document.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestOpenOptionsAreAvailableFromRootFacade(t *testing.T) {
	defaults := playa.DefaultDocumentOptions()
	if defaults.Space != playa.CoordinateSpaceScreen || !defaults.CacheSet || defaults.Cache.ObjectBytes <= 0 {
		t.Fatalf("default document options = %#v", defaults)
	}
	customDefaults := playa.WithDocumentOptions(defaults)
	if customDefaults == nil {
		t.Fatal("WithDocumentOptions returned nil")
	}
	var custom playa.OpenOption = func(options *playa.DocumentOptions) {
		options.Space = playa.CoordinateSpacePage
	}
	_ = custom
	cache := playa.DefaultCacheOptions()
	cache.PageBytes = 0
	document, err := playa.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"),
		playa.WithCoordinateSpace(playa.CoordinateSpacePage),
		playa.WithCacheOptions(cache),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := document.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if document.CoordinateSpace() != playa.CoordinateSpacePage {
		t.Fatalf("coordinate space = %q, want %q", document.CoordinateSpace(), playa.CoordinateSpacePage)
	}
}

func TestRootFacadeExposesRecoveryDiagnostics(t *testing.T) {
	document, err := playa.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ngarbage\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := document.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if !document.WasRecovered() || document.RecoveryError() == nil {
		t.Fatalf("recovery diagnostics = recovered:%v error:%v", document.WasRecovered(), document.RecoveryError())
	}
}

func TestPageLevelIteratorFacade(t *testing.T) {
	textOptions := playa.DefaultTextExtractionOptions()
	if textOptions.BBox != nil {
		t.Fatalf("default text extraction options = %#v", textOptions)
	}
	layout := playa.DefaultLayoutOptions()
	if layout.WordMargin != 0.1 || layout.LineMargin != 0.5 || layout.LineOverlap != 0.5 || layout.CharMargin != 2 || layout.BoxesFlow == nil || *layout.BoxesFlow != 0.5 {
		t.Fatalf("default layout options = %#v", layout)
	}
	concurrencyOptions := []playa.PageConcurrencyOption{playa.WithMaxPageWorkers(2), playa.WithPagesPerWorker(1)}
	document, err := playa.Open(testfixture.Path(t, "acceptance_page_labels.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = document.Close() }()
	if err := document.ForEachPageConcurrent(context.Background(), func(playa.Page) error { return nil }, concurrencyOptions...); err != nil {
		t.Fatalf("root page concurrency error = %v", err)
	}
	defaultOptions := playa.DefaultContentOptions()
	if defaultOptions.Filter != playa.FilterAll || defaultOptions.RestrictOps != nil {
		t.Fatalf("default content options = %#v", defaultOptions)
	}
	var p page.Page
	var options = page.ContentOptions{Filter: page.FilterText}
	var _ = p.Interp(nil, options)
	var _ = p.Flatten(nil, options)
	var _ = p.Texts(nil)
	var _ = p.Glyphs(nil)
	var _ = p.Paths(nil)
	var _ = p.Images(nil)
	var _ = p.XObjects(nil)
	_, _ = p.Fonts(nil)
	var _ content.ContentObject
	var _ content.ContentSection
	var _ content.ContentSequence
}

func acceptPage(page.Page) {}
