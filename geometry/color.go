package geometry

import "encoding/json"

// Color is a PDF color-space value with owned component storage.
type Color struct {
	space      string
	values     []float64
	pattern    string
	components int
}

// NewColor constructs an owned color value.
func NewColor(space string, values []float64, pattern string, components int) Color {
	var owned []float64
	if values != nil {
		owned = append(make([]float64, 0, len(values)), values...)
	}
	return Color{space: space, values: owned, pattern: pattern, components: components}
}

// Space returns the PDF color-space name.
func (c Color) Space() string { return c.space }

// Pattern returns the selected pattern name, when the color is pattern-based.
func (c Color) Pattern() string { return c.pattern }

// ValuesCopy returns independent color components.
func (c Color) ValuesCopy() []float64 {
	if c.values == nil {
		return nil
	}
	return append(make([]float64, 0, len(c.values)), c.values...)
}

// Components reports the number of components expected by the color space.
func (c Color) Components() int {
	if c.components > 0 {
		return c.components
	}
	if c.components < 0 {
		return 0
	}
	return len(c.values)
}

// WithPattern returns a copy with the pattern name replaced.
func (c Color) WithPattern(pattern string) Color {
	c.pattern = pattern
	return c
}

// WithSpace returns a copy with the color-space metadata replaced.
func (c Color) WithSpace(space string, components int) Color {
	c.space = space
	c.pattern = ""
	c.components = components
	if components == 0 {
		c.components = -1
	}
	return c
}

// Finalize returns an independent snapshot of the color value.
func (c Color) Finalize() Color {
	c.values = c.ValuesCopy()
	return c
}

func (c Color) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Space   string    `json:"Space"`
		Values  []float64 `json:"Values"`
		Pattern string    `json:"Pattern"`
	}{Space: c.space, Values: c.values, Pattern: c.pattern})
}
