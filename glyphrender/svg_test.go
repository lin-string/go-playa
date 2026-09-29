package glyphrender

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"iter"
	"math"
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/page"
)

func TestSVGRejectsGlyphWithoutOutline(t *testing.T) {
	_, err := SVG(page.GlyphObject{}, SVGOptions{})
	if !errors.Is(err, ErrNoOutline) {
		t.Fatalf("SVG error = %v, want ErrNoOutline", err)
	}
}

func TestSVGExportsGlyphOutlineFromPDF(t *testing.T) {
	doc, err := document.OpenBytes(type3GlyphPDF())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var glyph page.GlyphObject
	found := false
	for candidate, glyphErr := range p.Glyphs(doc) {
		if glyphErr != nil {
			t.Fatal(glyphErr)
		}
		candidate = candidate.Finalize()
		if _, renderErr := SVG(candidate, SVGOptions{}); renderErr == nil {
			glyph = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatal("fixture did not expose a glyph outline")
	}
	var output bytes.Buffer
	if err := RenderSVG(&output, glyph, SVGOptions{}); err != nil {
		t.Fatalf("RenderSVG error = %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte("<svg ")) || !bytes.Contains(output.Bytes(), []byte("<path ")) {
		t.Fatalf("SVG output = %q", output.String())
	}
	pngData, err := PNG(glyph, PNGOptions{})
	if err != nil {
		t.Fatalf("PNG error = %v", err)
	}
	image, err := png.Decode(bytes.NewReader(pngData))
	if err != nil || image.Bounds().Dx() == 0 || image.Bounds().Dy() == 0 {
		t.Fatalf("PNG decode = %v, bounds=%v", err, image.Bounds())
	}
	if _, err := PNG(glyph, PNGOptions{MaxDimension: 1}); !errors.Is(err, ErrRasterTooLarge) {
		t.Fatalf("bounded PNG error = %v, want ErrRasterTooLarge", err)
	}
}

func type3GlyphPDF() []byte {
	charProc := "100 0 0 100 0 0 d1 0 0 m 1000 0 l 1000 1000 l 0 1000 l h f"
	return type3GlyphPDFWithProcedure(charProc)
}

func type3GlyphPDFWithProcedure(charProc string) []byte {
	content := "BT /F1 10 Tf (A) Tj ET"
	return []byte(fmt.Sprintf(`%%PDF-1.4
1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj
2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj
3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << /Font << /F1 4 0 R >> >> /Contents 6 0 R >> endobj
4 0 obj << /Type /Font /Subtype /Type3 /Name /F1 /FontBBox [0 0 1000 1000] /FontMatrix [0.001 0 0 0.001 0 0] /CharProcs << /A 5 0 R >> /Encoding << /Type /Encoding /Differences [65 /A] >> >> endobj
5 0 obj << /Length %d >> stream
%s
endstream endobj
6 0 obj << /Length %d >> stream
%s
endstream endobj
trailer << /Root 1 0 R >>
%%%%EOF
`, len(charProc), charProc, len(content), content))
}

func TestSVGRejectsNonFiniteOptions(t *testing.T) {
	for _, options := range []SVGOptions{{Scale: -1}, {Scale: math.Inf(1)}} {
		if _, err := SVG(page.GlyphObject{}, options); !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("SVG(%+v) error = %v, want ErrInvalidOptions", options, err)
		}
	}
}

func TestPNGRejectsGlyphWithoutOutline(t *testing.T) {
	_, err := PNG(page.GlyphObject{}, PNGOptions{})
	if !errors.Is(err, ErrNoOutline) {
		t.Fatalf("PNG error = %v, want ErrNoOutline", err)
	}
}

func TestExportGlyphWritesSelectedOutputs(t *testing.T) {
	doc, err := document.OpenBytes(type3GlyphPDF())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var glyph page.GlyphObject
	for candidate, glyphErr := range p.Glyphs(doc) {
		if glyphErr != nil {
			t.Fatal(glyphErr)
		}
		glyph = candidate.Finalize()
		break
	}
	var svg, raster, manifest bytes.Buffer
	if err := ExportGlyph(glyph, 7, ExportOutputs{SVG: &svg, PNG: &raster, Manifest: &manifest}, ExportOptions{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(svg.Bytes(), []byte("<path ")) || len(raster.Bytes()) == 0 || !bytes.HasSuffix(manifest.Bytes(), []byte("\n")) {
		t.Fatalf("export sizes = svg:%d png:%d manifest:%d", svg.Len(), raster.Len(), manifest.Len())
	}
	var entry ManifestEntry
	if err := json.Unmarshal(bytes.TrimSpace(manifest.Bytes()), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Index != 7 {
		t.Fatalf("manifest index = %d, want 7", entry.Index)
	}
}

func TestWriteJSONLStreamsManifestEntries(t *testing.T) {
	doc, err := document.OpenBytes(type3GlyphPDF())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	p, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	var glyph page.GlyphObject
	for candidate, glyphErr := range p.Glyphs(doc) {
		if glyphErr != nil {
			t.Fatal(glyphErr)
		}
		glyph = candidate.Finalize()
		break
	}
	var output bytes.Buffer
	sequence := func(yield func(page.GlyphObject, error) bool) {
		if !yield(glyph, nil) {
			return
		}
		yield(glyph, nil)
	}
	var _ iter.Seq2[page.GlyphObject, error] = sequence
	if err := WriteJSONL(&output, sequence); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var first, second ManifestEntry
	if err := decoder.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&second); err != nil {
		t.Fatal(err)
	}
	if first.Index != 0 || first.FontName != glyph.FontName() || second.Index != 1 || second.CID != glyph.CID() {
		t.Fatalf("manifest entries = %#v, %#v", first, second)
	}
}

func TestRenderShorthandCurvesLikeExpandedCubics(t *testing.T) {
	render := func(proc string) (string, []byte) {
		t.Helper()
		doc, err := document.OpenBytes(type3GlyphPDFWithProcedure("100 0 0 100 0 0 d1 " + proc))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = doc.Close() }()
		p, err := doc.PageAt(0)
		if err != nil {
			t.Fatal(err)
		}
		for glyph, err := range p.Glyphs(doc) {
			if err != nil {
				t.Fatal(err)
			}
			svg, err := SVG(glyph, SVGOptions{})
			if err != nil {
				t.Fatal(err)
			}
			png, err := PNG(glyph, PNGOptions{})
			if err != nil {
				t.Fatal(err)
			}
			return string(svg), png
		}
		t.Fatal("missing glyph")
		return "", nil
	}
	wantSVG, wantPNG := render("0 0 m 0 0 100 200 300 400 c 500 600 700 800 700 800 c h f")
	gotSVG, gotPNG := render("0 0 m 100 200 300 400 v 500 600 700 800 y h f")
	if gotSVG != wantSVG || !bytes.Equal(gotPNG, wantPNG) {
		t.Fatal("shorthand curve render differs from equivalent cubic")
	}
}
