package document

import "github.com/lin-string/go-playa/geometry"

// BBoxProvider is implemented by public objects with device-space bounds.
// It is the Go counterpart of Playa's LTComponent relationship contract.
type BBoxProvider = geometry.BBoxProvider

func bboxX0(bbox [4]float64) float64 { return geometry.X0(bbox) }
func bboxY0(bbox [4]float64) float64 { return geometry.Y0(bbox) }
func bboxX1(bbox [4]float64) float64 { return geometry.X1(bbox) }
func bboxY1(bbox [4]float64) float64 { return geometry.Y1(bbox) }

func bboxWidth(bbox [4]float64) float64  { return geometry.Width(bbox) }
func bboxHeight(bbox [4]float64) float64 { return geometry.Height(bbox) }
func bboxIsEmpty(bbox [4]float64) bool   { return geometry.IsEmpty(bbox) }

func bboxIsHoverlap(left [4]float64, right BBoxProvider) bool {
	return geometry.IsHoverlap(left, right)
}

func bboxHDistance(left [4]float64, right BBoxProvider) float64 {
	return geometry.HDistance(left, right)
}

func bboxHoverlap(left [4]float64, right BBoxProvider) float64 {
	return geometry.Hoverlap(left, right)
}

func bboxIsVOverlap(left [4]float64, right BBoxProvider) bool {
	return geometry.IsVOverlap(left, right)
}

func bboxVDistance(left [4]float64, right BBoxProvider) float64 {
	return geometry.VDistance(left, right)
}

func bboxVOverlap(left [4]float64, right BBoxProvider) float64 {
	return geometry.VOverlap(left, right)
}
