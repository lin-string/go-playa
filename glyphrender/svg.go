// Package glyphrender exports finalized glyph outlines without coupling the
// renderer to PDF document ownership or OCR.
package glyphrender

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/lin-string/go-playa/page"
)

var (
	ErrNoOutline      = errors.New("glyphrender: glyph has no outline")
	ErrInvalidOptions = errors.New("glyphrender: invalid SVG options")
)

// SVGOptions controls the standalone SVG export. Coordinates remain in the
// glyph's device space; Scale only controls the output dimensions.
type SVGOptions struct {
	Padding     float64
	Scale       float64
	Background  string
	Fill        string
	Stroke      string
	StrokeWidth float64
}

func defaultSVGOptions(options SVGOptions) (SVGOptions, error) {
	if options.Scale == 0 {
		options.Scale = 1
	}
	if options.Padding == 0 {
		options.Padding = 8
	}
	if options.Fill == "" {
		options.Fill = "#000000"
	}
	if options.StrokeWidth == 0 {
		options.StrokeWidth = 1
	}
	if !finiteNonNegative(options.Padding) || !finitePositive(options.Scale) || !finitePositive(options.StrokeWidth) {
		return SVGOptions{}, ErrInvalidOptions
	}
	return options, nil
}

// SVG renders a finalized glyph outline into an SVG document.
func SVG(glyph page.GlyphObject, options SVGOptions) ([]byte, error) {
	var output bytes.Buffer
	if err := RenderSVG(&output, glyph, options); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// RenderSVG writes a finalized glyph outline as SVG.
func RenderSVG(output io.Writer, glyph page.GlyphObject, options SVGOptions) error {
	if output == nil {
		return ErrInvalidOptions
	}
	options, err := defaultSVGOptions(options)
	if err != nil {
		return err
	}
	paths, bounds, err := collectPaths(glyph)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return ErrNoOutline
	}
	minX, minY := bounds[0]-options.Padding, bounds[1]-options.Padding
	width := bounds[2] - bounds[0] + options.Padding*2
	height := bounds[3] - bounds[1] + options.Padding*2
	if width <= 0 || height <= 0 || !finitePositive(width) || !finitePositive(height) {
		return ErrNoOutline
	}
	var document strings.Builder
	document.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="`)
	document.WriteString(number(minX))
	document.WriteByte(' ')
	document.WriteString(number(minY))
	document.WriteByte(' ')
	document.WriteString(number(width))
	document.WriteByte(' ')
	document.WriteString(number(height))
	document.WriteString(`" width="`)
	document.WriteString(number(width * options.Scale))
	document.WriteString(`" height="`)
	document.WriteString(number(height * options.Scale))
	document.WriteString(`">`)
	if options.Background != "" {
		document.WriteString(`<rect x="` + number(minX) + `" y="` + number(minY) + `" width="` + number(width) + `" height="` + number(height) + `" fill="` + html.EscapeString(options.Background) + `"/>`)
	}
	for _, path := range paths {
		document.WriteString(`<path d="`)
		document.WriteString(path)
		document.WriteString(`" fill="`)
		document.WriteString(html.EscapeString(options.Fill))
		document.WriteString(`"`)
		if options.Stroke != "" {
			document.WriteString(` stroke="` + html.EscapeString(options.Stroke) + `" stroke-width="` + number(options.StrokeWidth) + `"`)
		}
		document.WriteString(` fill-rule="nonzero"/>`)
	}
	document.WriteString(`</svg>`)
	_, err = io.WriteString(output, document.String())
	return err
}

func collectPaths(glyph page.GlyphObject) ([]string, [4]float64, error) {
	paths := []string{}
	var bounds [4]float64
	haveBounds := false
	for path, err := range glyph.PathsSeq() {
		if err != nil {
			return nil, [4]float64{}, err
		}
		segments := path.SegmentsCopy()
		if err := expandShorthandCurves(segments); err != nil {
			return nil, [4]float64{}, err
		}
		if len(segments) == 0 {
			continue
		}
		var data strings.Builder
		for _, segment := range segments {
			points := segment.PointsCopy()
			for _, point := range points {
				if !finite(point[0]) || !finite(point[1]) {
					return nil, [4]float64{}, fmt.Errorf("glyphrender: non-finite outline point")
				}
				if !haveBounds {
					bounds = [4]float64{point[0], point[1], point[0], point[1]}
					haveBounds = true
				} else {
					bounds[0] = math.Min(bounds[0], point[0])
					bounds[1] = math.Min(bounds[1], point[1])
					bounds[2] = math.Max(bounds[2], point[0])
					bounds[3] = math.Max(bounds[3], point[1])
				}
			}
			switch segment.Operator() {
			case "m", "l":
				if len(points) != 1 {
					return nil, [4]float64{}, fmt.Errorf("glyphrender: invalid %s segment", segment.Operator())
				}
				data.WriteString(segment.Operator())
				data.WriteByte(' ')
				writePoint(&data, points[0])
			case "c":
				if len(points) != 3 {
					return nil, [4]float64{}, fmt.Errorf("glyphrender: invalid cubic segment")
				}
				data.WriteString("c ")
				for _, point := range points {
					writePoint(&data, point)
					data.WriteByte(' ')
				}
			case "h":
				data.WriteString("Z ")
			default:
				return nil, [4]float64{}, fmt.Errorf("glyphrender: unsupported path operator %q", segment.Operator())
			}
		}
		paths = append(paths, strings.TrimSpace(data.String()))
	}
	if !haveBounds {
		return nil, [4]float64{}, nil
	}
	return paths, bounds, nil
}

func writePoint(output *strings.Builder, point [2]float64) {
	output.WriteString(number(point[0]))
	output.WriteByte(' ')
	output.WriteString(number(point[1]))
}

func number(value float64) string          { return strconv.FormatFloat(value, 'g', -1, 64) }
func finite(value float64) bool            { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func finitePositive(value float64) bool    { return finite(value) && value > 0 }
func finiteNonNegative(value float64) bool { return finite(value) && value >= 0 }
