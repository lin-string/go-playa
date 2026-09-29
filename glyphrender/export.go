package glyphrender

import (
	"encoding/json"
	"io"

	"github.com/lin-string/go-playa/page"
)

// ExportOutputs selects the output streams for one glyph. Nil streams are
// skipped, allowing callers to request only SVG, only PNG, or only metadata.
type ExportOutputs struct {
	SVG      io.Writer
	PNG      io.Writer
	Manifest io.Writer
}

// ExportOptions contains the options for the selected output streams.
type ExportOptions struct {
	SVG SVGOptions
	PNG PNGOptions
}

// ExportGlyph writes the selected per-glyph outputs without retaining the
// glyph or its rendered bytes after each writer returns.
func ExportGlyph(glyph page.GlyphObject, index int, outputs ExportOutputs, options ExportOptions) error {
	if outputs.SVG == nil && outputs.PNG == nil && outputs.Manifest == nil {
		return ErrInvalidOutput
	}
	if outputs.SVG != nil {
		if err := RenderSVG(outputs.SVG, glyph, options.SVG); err != nil {
			return err
		}
	}
	if outputs.PNG != nil {
		if err := RenderPNG(outputs.PNG, glyph, options.PNG); err != nil {
			return err
		}
	}
	if outputs.Manifest != nil {
		if err := encodeManifestEntry(json.NewEncoder(outputs.Manifest), index, glyph); err != nil {
			return err
		}
	}
	return nil
}
