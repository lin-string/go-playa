package fontdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/geometry"
)

// Metadata contains the immutable scalar properties of a resolved font.
//
// It deliberately excludes parsed font programs and lazy caches. Callers can
// retain this value without retaining the document's parsing state.
type Metadata struct {
	name         string
	flags        int
	ascent       float64
	descent      float64
	leading      float64
	capHeight    float64
	stemV        float64
	fontMatrix   geometry.Matrix
	hasFlags     bool
	hasBBox      bool
	italicAngle  float64
	defaultWidth float64
	vertical     bool
	bbox         [4]float64
}

// NewMetadata constructs an immutable font descriptor snapshot.
func NewMetadata(name string, flags int, ascent, descent, leading, capHeight, stemV float64, fontMatrix geometry.Matrix, hasFlags, hasBBox bool, italicAngle, defaultWidth float64, vertical bool, bbox [4]float64) Metadata {
	return Metadata{name: name, flags: flags, ascent: ascent, descent: descent, leading: leading, capHeight: capHeight, stemV: stemV, fontMatrix: fontMatrix, hasFlags: hasFlags, hasBBox: hasBBox, italicAngle: italicAngle, defaultWidth: defaultWidth, vertical: vertical, bbox: bbox}
}

func (m Metadata) Name() string                { return m.name }
func (m Metadata) Flags() int                  { return m.flags }
func (m Metadata) Ascent() float64             { return m.ascent }
func (m Metadata) Descent() float64            { return m.descent }
func (m Metadata) Leading() float64            { return m.leading }
func (m Metadata) CapHeight() float64          { return m.capHeight }
func (m Metadata) StemV() float64              { return m.stemV }
func (m Metadata) FontMatrix() geometry.Matrix { return m.fontMatrix }
func (m Metadata) HasFlags() bool              { return m.hasFlags }
func (m Metadata) HasBBox() bool               { return m.hasBBox }
func (m Metadata) ItalicAngle() float64        { return m.italicAngle }
func (m Metadata) DefaultWidth() float64       { return m.defaultWidth }
func (m Metadata) Vertical() bool              { return m.vertical }
func (m Metadata) BBox() [4]float64            { return m.bbox }

// Finalize returns an independent metadata snapshot. Metadata contains only
// value fields, so finalization is intentionally a no-op value copy.
func (m Metadata) Finalize() Metadata { return m }

func (m Metadata) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name         string
		Flags        int
		Ascent       float64
		Descent      float64
		Leading      float64
		CapHeight    float64
		StemV        float64
		FontMatrix   geometry.Matrix
		HasFlags     bool
		HasBBox      bool
		ItalicAngle  float64
		DefaultWidth float64
		Vertical     bool
		BBox         [4]float64
	}{m.name, m.flags, m.ascent, m.descent, m.leading, m.capHeight, m.stemV, m.fontMatrix, m.hasFlags, m.hasBBox, m.italicAngle, m.defaultWidth, m.vertical, m.bbox})
}
