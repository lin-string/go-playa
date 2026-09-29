package geometry

import "math"

// BBoxProvider is implemented by values with device-space bounds.
// It is the Go counterpart of Playa's LTComponent relationship contract.
type BBoxProvider interface {
	BBox() Rect
}

// X0 returns the lower-left x coordinate of a rectangle.
func X0(rect Rect) float64 { return rect[0] }

// Y0 returns the lower-left y coordinate of a rectangle.
func Y0(rect Rect) float64 { return rect[1] }

// X1 returns the upper-right x coordinate of a rectangle.
func X1(rect Rect) float64 { return rect[2] }

// Y1 returns the upper-right y coordinate of a rectangle.
func Y1(rect Rect) float64 { return rect[3] }

// Width returns the rectangle width.
func Width(rect Rect) float64 { return rect[2] - rect[0] }

// Height returns the rectangle height.
func Height(rect Rect) float64 { return rect[3] - rect[1] }

// IsEmpty reports Playa's LTComponent empty-rectangle semantics.
func IsEmpty(rect Rect) bool { return Width(rect) <= 0 || Height(rect) <= 0 }

// IsHoverlap reports whether two rectangles overlap or touch horizontally.
func IsHoverlap(left Rect, right BBoxProvider) bool {
	other := right.BBox()
	return other[0] <= left[2] && left[0] <= other[2]
}

// HDistance returns the horizontal distance between rectangles, or zero when
// their horizontal projections overlap.
func HDistance(left Rect, right BBoxProvider) float64 {
	if IsHoverlap(left, right) {
		return 0
	}
	other := right.BBox()
	return minAbs(left[0]-other[2], left[2]-other[0])
}

// Hoverlap returns Playa's horizontal overlap measure.
func Hoverlap(left Rect, right BBoxProvider) float64 {
	if !IsHoverlap(left, right) {
		return 0
	}
	other := right.BBox()
	return minAbs(left[0]-other[2], left[2]-other[0])
}

// IsVOverlap reports whether two rectangles overlap or touch vertically.
func IsVOverlap(left Rect, right BBoxProvider) bool {
	other := right.BBox()
	return other[1] <= left[3] && left[1] <= other[3]
}

// VDistance returns the vertical distance between rectangles, or zero when
// their vertical projections overlap.
func VDistance(left Rect, right BBoxProvider) float64 {
	if IsVOverlap(left, right) {
		return 0
	}
	other := right.BBox()
	return minAbs(left[1]-other[3], left[3]-other[1])
}

// VOverlap returns Playa's vertical overlap measure.
func VOverlap(left Rect, right BBoxProvider) float64 {
	if !IsVOverlap(left, right) {
		return 0
	}
	other := right.BBox()
	return minAbs(left[1]-other[3], left[3]-other[1])
}

func minAbs(left, right float64) float64 {
	left, right = math.Abs(left), math.Abs(right)
	if left < right {
		return left
	}
	return right
}
