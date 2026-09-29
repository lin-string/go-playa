package imagedata

import (
	"encoding/json"
	"math"

	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// ColorSpace is the dependency-free scalar description of a PDF image color
// space. Filtered Indexed lookup bytes and their lazy decode cache remain
// document-owned because they depend on the document's parser and cache
// policy.
type ColorSpace struct {
	name       string
	components int
	spec       primitives.Object
	base       *ColorSpace
	colorants  []string
	high       int
	profileN   int
}

// NewColorSpace constructs a scalar color-space description.
func NewColorSpace(name string, components int) ColorSpace {
	return ColorSpace{name: name, components: components}
}

func (c ColorSpace) Name() string            { return c.name }
func (c ColorSpace) Components() int         { return c.components }
func (c ColorSpace) High() int               { return c.high }
func (c ColorSpace) ProfileN() int           { return c.profileN }
func (c ColorSpace) ColorantsCopy() []string { return cloneColorSpaceStrings(c.colorants) }

// SpecCopy returns the raw PDF color-space specification when retained.
func (c ColorSpace) SpecCopy() primitives.Object { return cloneColorSpaceObject(c.spec) }

// MakeColor converts PDF component objects into an owned color value. Missing
// or non-numeric components are padded with zero, matching Playa's
// ColorSpace.make_color behavior. A final name is treated as a pattern.
func (c ColorSpace) MakeColor(values ...primitives.Object) geometry.Color {
	componentCount := c.components
	pattern := ""
	if len(values) != 0 {
		if name, ok := primitives.NameValue(values[len(values)-1]); ok {
			pattern = name
			componentCount--
		}
	}
	if componentCount < 0 {
		componentCount = 0
	}
	components := make([]float64, componentCount)
	for index := range components {
		if index >= len(values) {
			break
		}
		if number, ok := primitives.NumberValue(values[index]); ok {
			components[index] = number
		}
	}
	if c.name == "Indexed" && componentCount == 1 && pattern == "" && c.high >= 0 &&
		!math.IsNaN(components[0]) && !math.IsInf(components[0], 0) {
		components[0] = max(0, min(math.Floor(components[0]+0.5), float64(c.high)))
	}
	return geometry.NewColor(c.name, components, pattern, componentCount)
}

// String returns the PDF color-space name.
func (c ColorSpace) String() string { return c.name }

// BaseCopy returns an independent nested base color-space description.
func (c ColorSpace) BaseCopy() (ColorSpace, bool) {
	if c.base == nil {
		return ColorSpace{}, false
	}
	return c.base.Finalize(), true
}

// WithComponents returns a copy with a normalized component count.
func (c ColorSpace) WithComponents(components int) ColorSpace {
	c.components = components
	return c
}

// WithBase returns a copy with an owned nested base color space.
func (c ColorSpace) WithBase(base ColorSpace) ColorSpace {
	base = base.Finalize()
	c.base = &base
	return c
}

// WithColorants returns a copy with an owned colorant-name list.
func (c ColorSpace) WithColorants(colorants []string) ColorSpace {
	c.colorants = cloneColorSpaceStrings(colorants)
	return c
}

// WithHigh returns a copy with the Indexed high value.
func (c ColorSpace) WithHigh(high int) ColorSpace {
	c.high = high
	return c
}

// WithProfileN returns a copy with the ICC profile component count.
func (c ColorSpace) WithProfileN(profileN int) ColorSpace {
	c.profileN = profileN
	return c
}

// WithSpec returns a copy with an owned raw PDF color-space specification.
func (c ColorSpace) WithSpec(spec primitives.Object) ColorSpace {
	c.spec = cloneColorSpaceObject(spec)
	return c
}

// Finalize returns an independent scalar color-space snapshot.
func (c ColorSpace) Finalize() ColorSpace {
	clone := c
	clone.spec = cloneColorSpaceObject(c.spec)
	clone.colorants = cloneColorSpaceStrings(c.colorants)
	if c.base != nil {
		base := c.base.Finalize()
		clone.base = &base
	}
	return clone
}

func (c ColorSpace) MarshalJSON() ([]byte, error) {
	var base *ColorSpace
	if c.base != nil {
		value := c.base.Finalize()
		base = &value
	}
	return json.Marshal(struct {
		Name       string            `json:"Name"`
		Components int               `json:"Components"`
		Spec       primitives.Object `json:"Spec"`
		Base       *ColorSpace       `json:"Base"`
		Colorants  []string          `json:"Colorants"`
		High       int               `json:"High"`
		ProfileN   int               `json:"ProfileN"`
	}{Name: c.name, Components: c.components, Spec: c.SpecCopy(), Base: base, Colorants: c.colorants, High: c.high, ProfileN: c.profileN})
}

func cloneColorSpaceObject(value primitives.Object) primitives.Object {
	switch value := value.(type) {
	case nil:
		return nil
	case primitives.Array:
		if value == nil {
			return primitives.Array(nil)
		}
		out := make(primitives.Array, len(value))
		for i, item := range value {
			out[i] = cloneColorSpaceObject(item)
		}
		return out
	case primitives.InvalidArray:
		if value == nil {
			return primitives.InvalidArray(nil)
		}
		out := make(primitives.InvalidArray, len(value))
		for i, item := range value {
			out[i] = cloneColorSpaceObject(item)
		}
		return out
	case primitives.Dict:
		if value == nil {
			return primitives.Dict(nil)
		}
		out := make(primitives.Dict, len(value))
		for key, item := range value {
			out[key] = cloneColorSpaceObject(item)
		}
		return out
	case primitives.String:
		return primitives.String(primitives.CloneBytes(value))
	default:
		return value
	}
}

func cloneColorSpaceStrings(value []string) []string {
	if value == nil {
		return nil
	}
	return append(make([]string, 0, len(value)), value...)
}
