// Package contentconfig owns dependency-free page-content configuration.
package contentconfig

// Kind identifies the concrete Playa page-content model.
type Kind string

const (
	KindText       Kind = "text"
	KindPath       Kind = "path"
	KindImage      Kind = "image"
	KindTag        Kind = "tag"
	KindXObject    Kind = "xobject"
	KindExtGState  Kind = "extgstate"
	KindColorSpace Kind = "colorspace"
	KindPattern    Kind = "pattern"
	KindShading    Kind = "shading"
	KindProperties Kind = "properties"
)

// Filter selects the content object kinds emitted by a page iterator.
type Filter uint8

const (
	FilterAll Filter = iota
	FilterText
	FilterPath
	FilterImage
	FilterXObject
	FilterTag
)

// Options controls page content interpretation.
type Options struct {
	Filter      Filter
	RestrictOps []string
}

// DefaultOptions returns the default interpretation policy.
func DefaultOptions() Options { return Options{Filter: FilterAll} }
