package fontdata

import (
	"encoding/binary"
	"math"
)

type cff2VariationRegion struct {
	start []float64
	peak  []float64
	end   []float64
}

type cff2VariationData struct {
	regions []int
	rows    [][]float64
}

// CFF2VariationStore is an immutable decoded CFF2 Item Variation Store.
// Region and item data are private; accessors return copies so callers cannot
// mutate the store shared by font resources.
type CFF2VariationStore struct {
	regions []cff2VariationRegion
	data    []cff2VariationData
}

// RegionCount reports the number of variation regions.
func (store *CFF2VariationStore) RegionCount() int {
	return len(store.regions)
}

// DataCount reports the number of ItemVariationData entries.
func (store *CFF2VariationStore) DataCount() int {
	return len(store.data)
}

// Region returns copies of one region's start, peak, and end coordinates.
func (store *CFF2VariationStore) Region(index int) (start, peak, end []float64, ok bool) {
	if index < 0 || index >= len(store.regions) {
		return nil, nil, nil, false
	}
	region := store.regions[index]
	return cloneCFF2Floats(region.start), cloneCFF2Floats(region.peak), cloneCFF2Floats(region.end), true
}

// Item returns copies of one ItemVariationData entry's region indexes and
// delta rows.
func (store *CFF2VariationStore) Item(index int) (regions []int, rows [][]float64, ok bool) {
	if index < 0 || index >= len(store.data) {
		return nil, nil, false
	}
	item := store.data[index]
	regions = cloneCFF2Ints(item.regions)
	if item.rows != nil {
		rows = make([][]float64, len(item.rows))
		for rowIndex, row := range item.rows {
			rows[rowIndex] = cloneCFF2Floats(row)
		}
	}
	return regions, rows, true
}

// RegionScalars evaluates all variation-region scalars at coords.
func (store *CFF2VariationStore) RegionScalars(coords []float64) []float64 {
	out := make([]float64, len(store.regions))
	for index, region := range store.regions {
		if len(region.start) != len(region.peak) || len(region.end) != len(region.peak) {
			continue
		}
		scalar := 1.0
		valid := true
		for axis := 0; axis < len(region.peak); axis++ {
			coord := 0.0
			if axis < len(coords) {
				coord = coords[axis]
			}
			start, peak, end := region.start[axis], region.peak[axis], region.end[axis]
			if math.IsNaN(coord) || math.IsInf(coord, 0) || math.IsNaN(start) || math.IsInf(start, 0) || math.IsNaN(peak) || math.IsInf(peak, 0) || math.IsNaN(end) || math.IsInf(end, 0) {
				valid = false
				break
			}
			if coord < start || coord > end {
				scalar = 0
				break
			}
			if coord < peak {
				if peak == start {
					continue
				}
				scalar *= (coord - start) / (peak - start)
			} else if coord > peak {
				if end == peak {
					continue
				}
				scalar *= (end - coord) / (end - peak)
			}
		}
		if valid {
			out[index] = scalar
		}
	}
	return out
}

// ParseCFF2VariationStore decodes a CFF2 Item Variation Store at offset.
// Malformed, overlapping, or out-of-bounds stores return nil.
func ParseCFF2VariationStore(data []byte, offset int) *CFF2VariationStore {
	if offset < 0 || offset > len(data) || len(data)-offset < 8 {
		return nil
	}
	readU16 := func(at int) (int, bool) {
		if at < 0 || at > len(data) || len(data)-at < 2 {
			return 0, false
		}
		return int(binary.BigEndian.Uint16(data[at:])), true
	}
	readU32 := func(at int) (int, bool) {
		if at < 0 || at > len(data) || len(data)-at < 4 {
			return 0, false
		}
		value := binary.BigEndian.Uint32(data[at:])
		if uint64(value) > uint64(len(data)) {
			return 0, false
		}
		return int(value), true
	}
	format, ok := readU16(offset)
	if !ok || format != 1 {
		return nil
	}
	regionOffset, ok := readU32(offset + 2)
	if !ok {
		return nil
	}
	dataCount, ok := readU16(offset + 6)
	if !ok || dataCount > (len(data)-offset-8)/4 {
		return nil
	}
	if regionOffset < 8+dataCount*4 {
		return nil
	}
	store := &CFF2VariationStore{data: make([]cff2VariationData, dataCount)}
	dataOffsets := make([]int, dataCount)
	for index := range dataOffsets {
		value, valid := readU32(offset + 8 + index*4)
		if !valid || value < 8+dataCount*4 || value >= regionOffset || value > len(data)-offset {
			return nil
		}
		dataOffsets[index] = offset + value
	}
	regionStart := offset + regionOffset
	regionAt := regionStart
	axisCount, ok := readU16(regionAt)
	if !ok {
		return nil
	}
	regionCount, ok := readU16(regionAt + 2)
	if !ok || axisCount <= 0 || regionCount > (len(data)-regionAt-4)/(axisCount*6) {
		return nil
	}
	regionAt += 4
	store.regions = make([]cff2VariationRegion, regionCount)
	for regionIndex := range store.regions {
		region := cff2VariationRegion{start: make([]float64, axisCount), peak: make([]float64, axisCount), end: make([]float64, axisCount)}
		for axis := 0; axis < axisCount; axis++ {
			if len(data)-regionAt < 6 {
				return nil
			}
			region.start[axis] = float64(int16(binary.BigEndian.Uint16(data[regionAt:]))) / 16384
			region.peak[axis] = float64(int16(binary.BigEndian.Uint16(data[regionAt+2:]))) / 16384
			region.end[axis] = float64(int16(binary.BigEndian.Uint16(data[regionAt+4:]))) / 16384
			if region.start[axis] > region.peak[axis] || region.peak[axis] > region.end[axis] {
				return nil
			}
			regionAt += 6
		}
		store.regions[regionIndex] = region
	}
	itemRanges := make([][2]int, 0, dataCount)
	for index, itemAt := range dataOffsets {
		itemCount, valid := readU16(itemAt)
		shortCount, validShort := readU16(itemAt + 2)
		regionCount, validRegions := readU16(itemAt + 4)
		if !valid || !validShort || !validRegions || shortCount > regionCount || itemAt > len(data) || len(data)-itemAt < 6 || regionCount > (len(data)-itemAt-6)/2 {
			return nil
		}
		at := itemAt + 6
		rowSize := shortCount*2 + (regionCount - shortCount)
		if rowSize < 0 || (rowSize > 0 && itemCount > (len(data)-at-regionCount*2)/rowSize) {
			return nil
		}
		item := cff2VariationData{regions: make([]int, regionCount), rows: make([][]float64, itemCount)}
		for regionIndex := range item.regions {
			value, valid := readU16(at)
			if !valid || value >= len(store.regions) {
				return nil
			}
			item.regions[regionIndex], at = value, at+2
		}
		for rowIndex := range item.rows {
			row := make([]float64, regionCount)
			for deltaIndex := 0; deltaIndex < regionCount; deltaIndex++ {
				if deltaIndex < shortCount {
					if len(data)-at < 2 {
						return nil
					}
					row[deltaIndex] = float64(int16(binary.BigEndian.Uint16(data[at:])))
					at += 2
				} else {
					if at < 0 || at >= len(data) {
						return nil
					}
					row[deltaIndex] = float64(int8(data[at]))
					at++
				}
			}
			item.rows[rowIndex] = row
		}
		if at > regionStart {
			return nil
		}
		for _, itemRange := range itemRanges {
			if itemAt < itemRange[1] && itemRange[0] < at {
				return nil
			}
		}
		itemRanges = append(itemRanges, [2]int{itemAt, at})
		store.data[index] = item
	}
	return store
}

func cloneCFF2Floats(value []float64) []float64 {
	if value == nil {
		return nil
	}
	return append([]float64(nil), value...)
}

func cloneCFF2Ints(value []int) []int {
	if value == nil {
		return nil
	}
	return append([]int(nil), value...)
}
