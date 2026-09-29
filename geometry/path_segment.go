package geometry

import "encoding/json"

// PathSegment is a PDF path segment. Operator is one of m, l, c, v, y, or h.
type PathSegment struct {
	operator string
	points   [][2]float64
}

// NewPathSegment constructs an owned path segment.
func NewPathSegment(operator string, points ...[2]float64) PathSegment {
	copy := make([][2]float64, len(points))
	copy = append(copy[:0], points...)
	return PathSegment{operator: operator, points: copy}
}

// Operator returns the PDF path operator name.
func (segment PathSegment) Operator() string { return segment.operator }

// ClonePathSegment returns an independent path segment.
func ClonePathSegment(segment PathSegment) PathSegment {
	if segment.points != nil {
		copy := make([][2]float64, len(segment.points))
		segment.points = append(copy[:0], segment.points...)
	}
	return segment
}

// PointsCopy returns an independent copy of the segment's control points.
func (segment PathSegment) PointsCopy() [][2]float64 {
	if segment.points == nil {
		return nil
	}
	copy := make([][2]float64, len(segment.points))
	return append(copy[:0], segment.points...)
}

// Finalize returns an independent path snapshot.
func (segment PathSegment) Finalize() PathSegment { return ClonePathSegment(segment) }

func (segment PathSegment) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Operator string
		Points   [][2]float64 `json:"Points"`
	}{Operator: segment.operator, Points: segment.points})
}
