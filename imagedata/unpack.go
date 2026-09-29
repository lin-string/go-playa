// Package imagedata owns dependency-free PDF image sample-data helpers.
package imagedata

// Unpack expands PDF 1/2/4-bit samples to one sample per byte. It preserves
// row boundaries and is independent of colorspace interpretation.
func Unpack(data []byte, bpc, width, height, components int) []byte {
	if bpc != 1 && bpc != 2 && bpc != 4 {
		return data
	}
	if width <= 0 || height <= 0 || components <= 0 {
		return nil
	}
	maxUint := ^uint64(0)
	width64, height64, components64 := uint64(width), uint64(height), uint64(components)
	if width64 > maxUint/components64 {
		return nil
	}
	rowSamples64 := width64 * components64
	if rowSamples64 > maxUint/uint64(bpc) {
		return nil
	}
	maxInt := uint64(^uint(0) >> 1)
	rowBits := rowSamples64 * uint64(bpc)
	if rowBits > maxUint-7 {
		return nil
	}
	rowSize64 := (rowBits + 7) / 8
	if rowSamples64 > maxUint/height64 {
		return nil
	}
	totalSamples64 := rowSamples64 * height64
	if rowSize64 == 0 || rowSize64 > maxInt || rowSamples64 > maxInt || totalSamples64 > maxInt {
		return nil
	}
	rowSize := int(rowSize64)
	rowSamples := int(rowSamples64)
	capacity := totalSamples64
	dataBytes := uint64(len(data)) / uint64(bpc)
	if dataBytes > maxUint/8 {
		dataBytes = maxUint / 8
	}
	dataSamples := dataBytes * 8
	if dataSamples < capacity {
		capacity = dataSamples
	}
	out := make([]byte, 0, int(capacity))
	rows := height
	if len(data) == 0 {
		rows = 0
	} else if available := (len(data)-1)/rowSize + 1; available < rows {
		rows = available
	}
	for row := 0; row < rows; row++ {
		start := row * rowSize
		end := start + rowSize
		if end > len(data) {
			end = len(data)
		}
		samples := 0
		for _, b := range data[start:end] {
			for shift := 8 - bpc; shift >= 0; shift -= bpc {
				out = append(out, (b>>uint(shift))&byte((1<<bpc)-1))
				samples++
				if samples == rowSamples {
					break
				}
			}
		}
	}
	return out
}
