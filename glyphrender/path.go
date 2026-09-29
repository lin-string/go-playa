package glyphrender

import "github.com/lin-string/go-playa/geometry"

// expandShorthandCurves adapts preserved PDF v/y segments to the cubic
// primitives used by both renderers. The input is an owned path snapshot.
func expandShorthandCurves(segments []geometry.PathSegment) error {
	var current, start [2]float64
	haveCurrent := false
	for index, segment := range segments {
		points := segment.PointsCopy()
		switch segment.Operator() {
		case "m":
			if len(points) == 1 {
				current, start = points[0], points[0]
				haveCurrent = true
			}
		case "l", "c":
			if len(points) > 0 {
				current = points[len(points)-1]
				haveCurrent = true
			}
		case "v":
			if !haveCurrent || len(points) != 2 {
				return ErrInvalidOptions
			}
			segments[index] = geometry.NewPathSegment("c", current, points[0], points[1])
			current = points[1]
		case "y":
			if !haveCurrent || len(points) != 2 {
				return ErrInvalidOptions
			}
			segments[index] = geometry.NewPathSegment("c", points[0], points[1], points[1])
			current = points[1]
		case "h":
			current = start
		}
	}
	return nil
}
