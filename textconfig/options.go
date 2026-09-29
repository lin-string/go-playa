// Package textconfig owns dependency-free text extraction configuration.
package textconfig

// Options controls high-level page text extraction. BBox is applied to
// complete text objects, matching Playa's extraction contract.
type Options struct {
	BBox *[4]float64
}

// DefaultOptions returns the full-page extraction policy.
func DefaultOptions() Options { return Options{} }

// Snapshot returns an independent copy of options and its optional bounding box.
func Snapshot(options Options) Options {
	if options.BBox != nil {
		bbox := *options.BBox
		options.BBox = &bbox
	}
	return options
}
