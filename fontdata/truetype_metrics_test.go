package fontdata

import (
	"encoding/binary"
	"testing"
)

func TestParseTrueTypeHorizontalMetricsScalesAndRepeats(t *testing.T) {
	metrics := ParseTrueTypeHorizontalMetrics(testTrueTypeWithHorizontalMetrics())
	if len(metrics) != 3 || metrics[0] != 500 || metrics[1] != 750 || metrics[2] != 750 {
		t.Fatalf("TrueType metrics = %#v, want 500, 750, 750", metrics)
	}
}

func TestParseTrueTypeHorizontalMetricsRejectsIncompleteTables(t *testing.T) {
	data := testTrueTypeWithHorizontalMetrics()
	data = data[:len(data)-1]
	if metrics := ParseTrueTypeHorizontalMetrics(data); metrics != nil {
		t.Fatalf("truncated hmtx returned partial metrics: %#v", metrics)
	}

	data = testTrueTypeWithHorizontalMetrics()
	hheaOffset := binary.BigEndian.Uint32(data[12+16+8:])
	binary.BigEndian.PutUint16(data[hheaOffset+34:], 4)
	if metrics := ParseTrueTypeHorizontalMetrics(data); metrics != nil {
		t.Fatalf("invalid hhea metric count returned metrics: %#v", metrics)
	}
}

func testTrueTypeWithHorizontalMetrics() []byte {
	tables := map[string][]byte{
		"head": func() []byte {
			data := make([]byte, 20)
			binary.BigEndian.PutUint16(data[18:20], 1000)
			return data
		}(),
		"hhea": func() []byte {
			data := make([]byte, 36)
			binary.BigEndian.PutUint16(data[34:36], 2)
			return data
		}(),
		"maxp": func() []byte {
			data := make([]byte, 6)
			binary.BigEndian.PutUint16(data[4:6], 3)
			return data
		}(),
		"hmtx": {0x01, 0xf4, 0, 0, 0x02, 0xee, 0, 0, 0, 0, 0, 0},
	}
	order := []string{"head", "hhea", "maxp", "hmtx"}
	data := make([]byte, 12+16*len(order))
	copy(data[:4], []byte{0, 1, 0, 0})
	binary.BigEndian.PutUint16(data[4:6], uint16(len(order)))
	for i, name := range order {
		table := tables[name]
		offset := len(data)
		data = append(data, table...)
		entry := 12 + i*16
		copy(data[entry:entry+4], name)
		binary.BigEndian.PutUint32(data[entry+8:entry+12], uint32(offset))
		binary.BigEndian.PutUint32(data[entry+12:entry+16], uint32(len(table)))
	}
	return data
}
