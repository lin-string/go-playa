// Package geometry contains immutable value operations used by PDF geometry.
package geometry

import "math"

// Point is a two-dimensional PDF coordinate.
type Point = [2]float64

// Rect is a PDF rectangle in [llx, lly, urx, ury] order.
type Rect = [4]float64

// Matrix is an affine PDF transformation matrix [a b c d e f].
type Matrix [6]float64

// Mul composes m with n using PDF's affine matrix convention.
func (m Matrix) Mul(n Matrix) Matrix {
	return Matrix{m[0]*n[0] + m[2]*n[1], m[1]*n[0] + m[3]*n[1], m[0]*n[2] + m[2]*n[3], m[1]*n[2] + m[3]*n[3], m[0]*n[4] + m[2]*n[5] + m[4], m[1]*n[4] + m[3]*n[5] + m[5]}
}

// MulFinite composes two matrices and rejects a non-finite result.
func (m Matrix) MulFinite(n Matrix) (Matrix, bool) {
	value := m.Mul(n)
	for _, item := range value {
		if !finite(item) {
			return Matrix{}, false
		}
	}
	return value, true
}

// OffsetFinite applies a translation and rejects non-finite input or output.
func (m Matrix) OffsetFinite(dx, dy float64) (Matrix, bool) {
	if !finite(dx) || !finite(dy) {
		return Matrix{}, false
	}
	value := m
	value[4] += value[0]*dx + value[2]*dy
	value[5] += value[1]*dx + value[3]*dy
	for _, item := range value {
		if !finite(item) {
			return Matrix{}, false
		}
	}
	return value, true
}

// Point transforms a point by m.
func (m Matrix) Point(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

// PointFinite transforms a finite point and rejects a non-finite result.
func (m Matrix) PointFinite(x, y float64) (float64, float64, bool) {
	if !finite(x) || !finite(y) {
		return 0, 0, false
	}
	pointX, pointY := m.Point(x, y)
	if !finite(pointX) || !finite(pointY) {
		return 0, 0, false
	}
	return pointX, pointY, true
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
