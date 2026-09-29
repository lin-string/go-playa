package fontdata

import "math"

// ParseCFFNonNegativeInt converts a finite integral CFF operand to int when
// it is within the platform's representable non-negative range.
func ParseCFFNonNegativeInt(value float64) (int, bool) {
	maxInt := int(^uint(0) >> 1)
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value != math.Trunc(value) || value > float64(maxInt) {
		return 0, false
	}
	return int(value), true
}

// ParseCFFInteger converts a finite integral CFF operand to int when it is
// within the platform's representable range.
func ParseCFFInteger(value float64) (int, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return 0, false
	}
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	if value < float64(minInt) || value > float64(maxInt) {
		return 0, false
	}
	return int(value), true
}

// CFFSubroutineBias returns the Type 2 subroutine bias for count entries.
func CFFSubroutineBias(count int) int {
	switch {
	case count < 1240:
		return 107
	case count < 33900:
		return 1131
	default:
		return 32768
	}
}

// ParseCFFSubroutineIndex applies the Type 2 subroutine bias and validates the
// resulting index against count.
func ParseCFFSubroutineIndex(value float64, count int) (int, bool) {
	index, ok := ParseCFFInteger(value + float64(CFFSubroutineBias(count)))
	if !ok || index < 0 || index >= count {
		return 0, false
	}
	return index, true
}
