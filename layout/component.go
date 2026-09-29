package layout

import (
	"strings"

	"github.com/lin-string/go-playa/geometry"
)

// ComponentSpec describes the dependency-free scalar state shared by Playa
// layout components. Document-bound glyphs and nested children stay outside
// this value and are owned by the document facade.
type ComponentSpec struct {
	Text     string
	BBox     geometry.Rect
	Vertical bool
	Index    int
}

// Component is the immutable scalar portion of a Playa LTComponent-like
// layout value. Use With* methods to derive another value instead of mutating
// a borrowed layout projection.
type Component struct {
	text     string
	bbox     geometry.Rect
	vertical bool
	index    int
}

// NewComponent constructs an owned layout component value.
func NewComponent(spec ComponentSpec) Component {
	return Component{text: spec.Text, bbox: spec.BBox, vertical: spec.Vertical, index: spec.Index}
}

// Text returns the component's derived text.
func (c Component) Text() string { return c.text }

// BBox returns the component's device-space bounds.
func (c Component) BBox() geometry.Rect { return c.bbox }

// Vertical reports whether the component uses vertical writing mode.
func (c Component) Vertical() bool { return c.vertical }

// Index returns the component's reading-order index. Components without an
// assigned order use Playa's -1 sentinel.
func (c Component) Index() int { return c.index }

// X0 returns the lower-left x coordinate.
func (c Component) X0() float64 { return geometry.X0(c.bbox) }

// Y0 returns the lower-left y coordinate.
func (c Component) Y0() float64 { return geometry.Y0(c.bbox) }

// X1 returns the upper-right x coordinate.
func (c Component) X1() float64 { return geometry.X1(c.bbox) }

// Y1 returns the upper-right y coordinate.
func (c Component) Y1() float64 { return geometry.Y1(c.bbox) }

// Width returns the component width.
func (c Component) Width() float64 { return geometry.Width(c.bbox) }

// Height returns the component height.
func (c Component) Height() float64 { return geometry.Height(c.bbox) }

// IsEmpty reports Playa's LTComponent empty-rectangle semantics.
func (c Component) IsEmpty() bool { return geometry.IsEmpty(c.bbox) }

// IsHoverlap reports horizontal overlap with another component.
func (c Component) IsHoverlap(other geometry.BBoxProvider) bool {
	return geometry.IsHoverlap(c.bbox, other)
}

// HDistance returns horizontal distance to another component.
func (c Component) HDistance(other geometry.BBoxProvider) float64 {
	return geometry.HDistance(c.bbox, other)
}

// Hoverlap returns Playa's horizontal overlap measure.
func (c Component) Hoverlap(other geometry.BBoxProvider) float64 {
	return geometry.Hoverlap(c.bbox, other)
}

// IsVOverlap reports vertical overlap with another component.
func (c Component) IsVOverlap(other geometry.BBoxProvider) bool {
	return geometry.IsVOverlap(c.bbox, other)
}

// VDistance returns vertical distance to another component.
func (c Component) VDistance(other geometry.BBoxProvider) float64 {
	return geometry.VDistance(c.bbox, other)
}

// VOverlap returns Playa's vertical overlap measure.
func (c Component) VOverlap(other geometry.BBoxProvider) float64 {
	return geometry.VOverlap(c.bbox, other)
}

// WritingMode returns Playa's canonical writing-mode name.
func (c Component) WritingMode() string { return WritingMode(c.vertical) }

// WithText derives a component with a different text value.
func (c Component) WithText(text string) Component {
	c.text = text
	return c
}

// WithBBox derives a component with different bounds.
func (c Component) WithBBox(bbox geometry.Rect) Component {
	c.bbox = bbox
	return c
}

// WithVertical derives a component with a different writing direction.
func (c Component) WithVertical(vertical bool) Component {
	c.vertical = vertical
	return c
}

// WithIndex derives a component with a different reading-order index.
func (c Component) WithIndex(index int) Component {
	c.index = index
	return c
}

// TextLineIsEmpty applies Playa's whitespace-only line semantics in addition
// to the base rectangle emptiness rule.
func TextLineIsEmpty(text string, bbox geometry.Rect) bool {
	return geometry.IsEmpty(bbox) || (text != "" && strings.TrimSpace(text) == "")
}
