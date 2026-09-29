package fontdata

import "math"

// ParseCFFCharStringWidth reads the optional Type 2 CharString width prefix.
// It follows local and global Subr calls when the first drawing operator is
// reached before a width is established. CFF2 variation operators are not
// accepted by this CFF1 helper; variation-aware width extraction remains part
// of the document font model.
func ParseCFFCharStringWidth(data []byte, nominal, defaultWidth float64, local, global [][]byte) (float64, bool) {
	return parseCFFCharStringWidth(data, nominal, defaultWidth, local, global, 0)
}

// CFF2VariationStoreReader provides the immutable variation data needed by
// ParseCFF2CharStringWidth. The interface keeps the parser independent from
// document-owned variation-store implementations.
type CFF2VariationStoreReader interface {
	DataCount() int
	Item(index int) (regions []int, rows [][]float64, ok bool)
	RegionScalars(coords []float64) []float64
}

// ParseCFF2CharStringWidth reads a CFF2 Type 2 CharString width prefix,
// including VSIndex and blend operators. It follows local and global Subr
// calls before a width is established and returns the default width when the
// first drawing operator does not contain an explicit width.
func ParseCFF2CharStringWidth(data []byte, nominal, defaultWidth float64, local, global [][]byte, variation CFF2VariationStoreReader, coords []float64, variationIndex int) (float64, bool) {
	return parseCFF2CharStringWidth(data, nominal, defaultWidth, local, global, variation, coords, variationIndex, 0)
}

func parseCFF2CharStringWidth(data []byte, nominal, defaultWidth float64, local, global [][]byte, variation CFF2VariationStoreReader, coords []float64, variationIndex, depth int) (float64, bool) {
	if depth > 10 {
		return defaultWidth, true
	}
	currentVariationIndex := variationIndex
	variationIndexSeen := false
	variationBlendSeen := false
	operands := []float64{}
	for at := 0; at < len(data); {
		value, next, ok := ParseCFFNumber(data, at)
		if ok {
			operands = append(operands, value)
			if len(operands) > 513 {
				return 0, false
			}
			at = next
			continue
		}
		op, opNext, ok := ParseCFFCharStringOperator(data, at, true)
		if !ok {
			return 0, false
		}
		argCount := func(base int) (float64, bool) {
			if len(operands) == base+1 {
				return finiteCFFAdd(nominal, operands[0])
			}
			if len(operands) == base {
				return defaultWidth, true
			}
			return 0, false
		}
		switch op {
		case 15:
			if variation == nil || len(operands) != 1 || variationIndexSeen || variationBlendSeen {
				return 0, false
			}
			index, ok := ParseCFFNonNegativeInt(operands[0])
			if !ok || index >= variation.DataCount() {
				return 0, false
			}
			variationIndexSeen = true
			currentVariationIndex = index
			operands = nil
			at = opNext
			continue
		case 16:
			if variation == nil || len(operands) == 0 || currentVariationIndex < 0 || currentVariationIndex >= variation.DataCount() {
				return 0, false
			}
			nValue := operands[len(operands)-1]
			if nValue < 1 || nValue != math.Trunc(nValue) || nValue > float64(len(operands)-1) {
				return 0, false
			}
			n := int(nValue)
			regions, rows, ok := variation.Item(currentVariationIndex)
			if !ok || n > len(rows) || len(regions) > (len(operands)-1)/n || len(operands) != n*(len(regions)+1)+1 {
				return 0, false
			}
			scalars := variation.RegionScalars(coords)
			blended := append([]float64(nil), operands[:n]...)
			for valueIndex := 0; valueIndex < n; valueIndex++ {
				row := rows[valueIndex]
				if len(row) < len(regions) {
					return 0, false
				}
				for regionIndex, region := range regions {
					if region < 0 || region >= len(scalars) {
						return 0, false
					}
					delta := operands[n+valueIndex*len(regions)+regionIndex]
					value := blended[valueIndex] + delta*scalars[region]
					if math.IsNaN(value) || math.IsInf(value, 0) {
						return 0, false
					}
					blended[valueIndex] = value
				}
			}
			variationBlendSeen = true
			operands = blended
			at = opNext
			continue
		case 10:
			if len(operands) == 0 {
				return defaultWidth, true
			}
			hasWidth := len(operands) > 1
			width := defaultWidth
			if hasWidth {
				var ok bool
				width, ok = finiteCFFAdd(nominal, operands[0])
				if !ok {
					return 0, false
				}
			}
			if len(local) == 0 {
				return width, true
			}
			index, ok := ParseCFFSubroutineIndex(operands[len(operands)-1], len(local))
			if !ok || hasWidth {
				return width, true
			}
			return parseCFF2CharStringWidth(local[index], nominal, defaultWidth, local, global, variation, coords, currentVariationIndex, depth+1)
		case 29:
			if len(operands) == 0 {
				return defaultWidth, true
			}
			hasWidth := len(operands) > 1
			width := defaultWidth
			if hasWidth {
				var ok bool
				width, ok = finiteCFFAdd(nominal, operands[0])
				if !ok {
					return 0, false
				}
			}
			if len(global) == 0 {
				return width, true
			}
			index, ok := ParseCFFSubroutineIndex(operands[len(operands)-1], len(global))
			if !ok || hasWidth {
				return width, true
			}
			return parseCFF2CharStringWidth(global[index], nominal, defaultWidth, local, global, variation, coords, currentVariationIndex, depth+1)
		case 1, 3, 18, 19, 20, 23:
			if len(operands)%2 == 1 {
				return finiteCFFAdd(nominal, operands[0])
			}
			return defaultWidth, true
		case 4, 21, 22:
			base := 1
			if op == 21 {
				base = 2
			}
			if width, ok := argCount(base); ok {
				return width, true
			}
			return 0, false
		case 14:
			if len(operands) == 1 || len(operands) == 5 {
				return finiteCFFAdd(nominal, operands[0])
			}
			return defaultWidth, true
		default:
			return defaultWidth, true
		}
	}
	return defaultWidth, true
}

func parseCFFCharStringWidth(data []byte, nominal, defaultWidth float64, local, global [][]byte, depth int) (float64, bool) {
	if depth > 10 {
		return defaultWidth, true
	}
	operands := []float64{}
	for at := 0; at < len(data); {
		if value, next, ok := ParseCFFNumber(data, at); ok {
			operands = append(operands, value)
			if len(operands) > 48 {
				return 0, false
			}
			at = next
			continue
		}
		op, _, ok := ParseCFFCharStringOperator(data, at, false)
		if !ok {
			return 0, false
		}
		widthForArgs := func(base int) (float64, bool) {
			if len(operands) == base+1 {
				return finiteCFFAdd(nominal, operands[0])
			}
			if len(operands) == base {
				return defaultWidth, true
			}
			return 0, false
		}
		switch op {
		case 10:
			if len(operands) == 0 {
				return defaultWidth, true
			}
			hasWidth := len(operands) > 1
			width := defaultWidth
			if hasWidth {
				var widthOK bool
				width, widthOK = finiteCFFAdd(nominal, operands[0])
				if !widthOK {
					return 0, false
				}
			}
			if len(local) == 0 {
				return width, true
			}
			index, indexOK := ParseCFFSubroutineIndex(operands[len(operands)-1], len(local))
			if !indexOK || hasWidth {
				return width, true
			}
			return parseCFFCharStringWidth(local[index], nominal, defaultWidth, local, global, depth+1)
		case 29:
			if len(operands) == 0 {
				return defaultWidth, true
			}
			hasWidth := len(operands) > 1
			width := defaultWidth
			if hasWidth {
				var widthOK bool
				width, widthOK = finiteCFFAdd(nominal, operands[0])
				if !widthOK {
					return 0, false
				}
			}
			if len(global) == 0 {
				return width, true
			}
			index, indexOK := ParseCFFSubroutineIndex(operands[len(operands)-1], len(global))
			if !indexOK || hasWidth {
				return width, true
			}
			return parseCFFCharStringWidth(global[index], nominal, defaultWidth, local, global, depth+1)
		case 1, 3, 18, 19, 20, 23:
			if len(operands)%2 == 1 {
				return finiteCFFAdd(nominal, operands[0])
			}
			return defaultWidth, true
		case 4, 21, 22:
			base := 1
			if op == 21 {
				base = 2
			}
			return widthForArgs(base)
		case 14:
			if len(operands) == 1 || len(operands) == 5 {
				return finiteCFFAdd(nominal, operands[0])
			}
			return defaultWidth, true
		default:
			return defaultWidth, true
		}
	}
	return defaultWidth, true
}

func finiteCFFAdd(left, right float64) (float64, bool) {
	value := left + right
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}
