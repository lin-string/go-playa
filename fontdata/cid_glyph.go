package fontdata

// ParseCIDToGIDMap decodes the two-byte big-endian entries of a PDF
// CIDToGIDMap stream. At most the PDF 16-bit CID domain is materialized;
// incomplete trailing bytes and entries outside that domain are ignored.
func ParseCIDToGIDMap(data []byte) map[int]int {
	if len(data) < 2 {
		return nil
	}
	count := len(data) / 2
	if count > 1<<16 {
		count = 1 << 16
	}
	mapping := make(map[int]int, count)
	for cid := 0; cid < count; cid++ {
		mapping[cid] = int(data[cid*2])<<8 | int(data[cid*2+1])
	}
	return mapping
}
