// Package layout exposes configuration for Playa-compatible text layout
// analysis.
package layout

import "math"

// Options controls Playa-compatible text layout analysis.
type Options struct {
	// AllTexts recursively includes text inside Form XObjects, matching
	// Playa's LAParams.all_texts. The default only analyzes page-level text.
	AllTexts bool
	// WordMargin is Playa's normalized gap threshold for inserting spaces.
	WordMargin float64
	// LineMargin is Playa's normalized distance threshold for grouping lines.
	LineMargin  float64
	LineOverlap float64
	CharMargin  float64
	// DetectVertical enables Playa's vertical text-line analysis. It is false
	// by default; vertical writing-mode glyphs are otherwise analyzed by the
	// horizontal pass, matching LAParams.
	DetectVertical bool
	// DisableBoxesFlow corresponds to Playa's boxes_flow=None. It keeps the
	// text boxes but skips hierarchical text-group analysis and uses Playa's
	// positional box order. A nil BoxesFlow retains the Go zero-value default
	// of 0.5 for compatibility with existing callers.
	DisableBoxesFlow bool
	// BoxesFlow controls reading order. A nil value uses Playa's default of
	// 0.5. Non-nil values are clamped to Playa's [-1, 1] range.
	BoxesFlow *float64
}

// DefaultOptions returns the Playa-compatible default mining thresholds.
func DefaultOptions() Options {
	boxesFlow := 0.5
	return Options{WordMargin: 0.1, LineMargin: 0.5, LineOverlap: 0.5, CharMargin: 2, BoxesFlow: &boxesFlow}
}

// FiniteThreshold returns a configured finite non-zero value or the default.
// Zero retains Go's default configuration convention; finite negative values
// are preserved.
func FiniteThreshold(value, defaultValue float64) float64 {
	if value != 0 && !math.IsNaN(value) && !math.IsInf(value, 0) {
		return value
	}
	return defaultValue
}

// BoxesFlow returns the normalized reading-order weight. A nil or non-finite
// value uses Playa's default; finite values are clamped to [-1, 1].
func BoxesFlow(value *float64) float64 {
	flow := 0.5
	if value != nil {
		flow = *value
	}
	if math.IsNaN(flow) || math.IsInf(flow, 0) {
		return 0.5
	}
	if flow < -1 {
		return -1
	}
	if flow > 1 {
		return 1
	}
	return flow
}

// WritingMode returns Playa's canonical layout writing-mode name.
func WritingMode(vertical bool) string {
	if vertical {
		return "tb-rl"
	}
	return "lr-tb"
}
