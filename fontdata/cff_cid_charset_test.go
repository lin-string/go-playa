package fontdata

import "testing"

func TestParseCFFCharsetCIDsSupportsAllFormats(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want map[int]int
	}{
		{name: "format 0", data: []byte{0, 0, 0, 0, 0, 100, 0, 101}, want: map[int]int{100: 1, 101: 2}},
		{name: "format 1", data: []byte{0, 0, 0, 1, 0, 100, 1, 0}, want: map[int]int{100: 1, 101: 2}},
		{name: "format 2", data: []byte{0, 0, 0, 2, 0, 100, 0, 1, 0}, want: map[int]int{100: 1, 101: 2}},
	}
	for _, test := range tests {
		got := ParseCFFCharsetCIDs(test.data, 3, 3)
		if len(got) != len(test.want) {
			t.Fatalf("%s CID charset = %#v, want %#v", test.name, got, test.want)
		}
		for cid, gid := range test.want {
			if got[cid] != gid {
				t.Fatalf("%s CID %d = %d, want %d", test.name, cid, got[cid], gid)
			}
		}
	}
}

func TestParseCFFCharsetCIDsRejectsOversizedRange(t *testing.T) {
	if got := ParseCFFCharsetCIDs([]byte{0, 0, 0, 1, 0, 100, 2}, 3, 3); got != nil {
		t.Fatalf("oversized CID charset range = %#v", got)
	}
}
