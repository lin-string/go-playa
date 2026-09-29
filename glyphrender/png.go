package glyphrender

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"

	"github.com/lin-string/go-playa/page"
	"golang.org/x/image/vector"
)

var ErrRasterTooLarge = errors.New("glyphrender: raster exceeds maximum dimensions")

// PNGOptions controls rasterized glyph export. Scale controls pixels per
// glyph coordinate unit after padding. MaxDimension bounds both output axes
// to prevent malformed outlines from causing unbounded allocations.
type PNGOptions struct {
	Padding      float64
	Scale        float64
	Background   color.Color
	Fill         color.Color
	MaxDimension int
}

func defaultPNGOptions(options PNGOptions) (PNGOptions, error) {
	if options.Scale == 0 {
		options.Scale = 1
	}
	if options.Padding == 0 {
		options.Padding = 8
	}
	if options.Fill == nil {
		options.Fill = color.Black
	}
	if options.MaxDimension == 0 {
		options.MaxDimension = 4096
	}
	if !finiteNonNegative(options.Padding) || !finitePositive(options.Scale) || options.MaxDimension <= 0 {
		return PNGOptions{}, ErrInvalidOptions
	}
	return options, nil
}

// PNG renders a finalized glyph outline into PNG bytes.
func PNG(glyph page.GlyphObject, options PNGOptions) ([]byte, error) {
	var output bytes.Buffer
	if err := RenderPNG(&output, glyph, options); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// RenderPNG writes a finalized glyph outline as a bounded-memory PNG image.
func RenderPNG(output io.Writer, glyph page.GlyphObject, options PNGOptions) error {
	if output == nil {
		return ErrInvalidOptions
	}
	options, err := defaultPNGOptions(options)
	if err != nil {
		return err
	}
	paths, bounds, err := collectRasterPaths(glyph)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return ErrNoOutline
	}
	width := int(math.Ceil((bounds[2] - bounds[0] + 2*options.Padding) * options.Scale))
	height := int(math.Ceil((bounds[3] - bounds[1] + 2*options.Padding) * options.Scale))
	if width <= 0 || height <= 0 {
		return ErrNoOutline
	}
	if width > options.MaxDimension || height > options.MaxDimension {
		return ErrRasterTooLarge
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	if options.Background != nil {
		draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: options.Background}, image.Point{}, draw.Src)
	}
	fill := image.NewUniform(options.Fill)
	for _, path := range paths {
		raster := vector.NewRasterizer(width, height)
		for _, segment := range path {
			points := segment.PointsCopy()
			switch segment.Operator() {
			case "m":
				if len(points) != 1 {
					return ErrInvalidOptions
				}
				raster.MoveTo(float32((points[0][0]-bounds[0]+options.Padding)*options.Scale), float32((points[0][1]-bounds[1]+options.Padding)*options.Scale))
			case "l":
				if len(points) != 1 {
					return ErrInvalidOptions
				}
				raster.LineTo(float32((points[0][0]-bounds[0]+options.Padding)*options.Scale), float32((points[0][1]-bounds[1]+options.Padding)*options.Scale))
			case "c":
				if len(points) != 3 {
					return ErrInvalidOptions
				}
				raster.CubeTo(float32((points[0][0]-bounds[0]+options.Padding)*options.Scale), float32((points[0][1]-bounds[1]+options.Padding)*options.Scale), float32((points[1][0]-bounds[0]+options.Padding)*options.Scale), float32((points[1][1]-bounds[1]+options.Padding)*options.Scale), float32((points[2][0]-bounds[0]+options.Padding)*options.Scale), float32((points[2][1]-bounds[1]+options.Padding)*options.Scale))
			case "h":
				raster.ClosePath()
			default:
				return ErrInvalidOptions
			}
		}
		raster.Draw(canvas, canvas.Bounds(), fill, image.Point{})
	}
	return png.Encode(output, canvas)
}

func collectRasterPaths(glyph page.GlyphObject) ([][]page.PathSegment, [4]float64, error) {
	paths := [][]page.PathSegment{}
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
		for _, segment := range segments {
			for _, point := range segment.PointsCopy() {
				if !finite(point[0]) || !finite(point[1]) {
					return nil, [4]float64{}, ErrInvalidOptions
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
		}
		paths = append(paths, segments)
	}
	return paths, bounds, nil
}
