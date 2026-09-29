package fontdata

import "encoding/binary"

// ParseCFFIndex parses a CFF1 INDEX at byte offset at. Returned item slices
// borrow data and remain valid while data remains unchanged. The returned end
// offset points immediately after the INDEX.
func ParseCFFIndex(data []byte, at int) (items [][]byte, end int, ok bool) {
	return parseCFFIndex(data, at, 2)
}

// ParseCFF2Index parses a CFF2 INDEX at byte offset at. Returned item slices
// borrow data and remain valid while data remains unchanged. The returned end
// offset points immediately after the INDEX.
func ParseCFF2Index(data []byte, at int) (items [][]byte, end int, ok bool) {
	return parseCFFIndex(data, at, 4)
}

func parseCFFIndex(data []byte, at, countBytes int) (items [][]byte, end int, ok bool) {
	if at < 0 || at > len(data) || len(data)-at < countBytes {
		return nil, 0, false
	}
	var count uint64
	if countBytes == 2 {
		count = uint64(binary.BigEndian.Uint16(data[at:]))
	} else {
		count = uint64(binary.BigEndian.Uint32(data[at:]))
	}
	at += countBytes
	if count == 0 {
		return nil, at, true
	}
	if at >= len(data) {
		return nil, 0, false
	}
	offSize := int(data[at])
	at++
	if offSize < 1 || offSize > 4 {
		return nil, 0, false
	}
	available := len(data) - at
	maxOffsets := available / offSize
	if maxOffsets == 0 || count > uint64(maxOffsets-1) {
		return nil, 0, false
	}
	countInt := int(count)
	offsetBytes := (countInt + 1) * offSize
	offsets := make([]uint64, countInt+1)
	for i := range offsets {
		var value uint64
		for j := 0; j < offSize; j++ {
			value = value<<8 | uint64(data[at+i*offSize+j])
		}
		offsets[i] = value
	}
	payloadStart := at + offsetBytes
	if offsets[0] != 1 || offsets[countInt] < 1 {
		return nil, 0, false
	}
	payloadLength := uint64(len(data) - payloadStart)
	if offsets[countInt]-1 > payloadLength {
		return nil, 0, false
	}
	end = payloadStart + int(offsets[countInt]-1)
	items = make([][]byte, countInt)
	for i := range items {
		if offsets[i] < 1 || offsets[i+1] < offsets[i] || offsets[i+1]-1 > payloadLength {
			return nil, 0, false
		}
		start := payloadStart + int(offsets[i]-1)
		finish := payloadStart + int(offsets[i+1]-1)
		items[i] = data[start:finish]
	}
	return items, end, true
}
