package document

import "testing"

func TestOwnershipCopiesPreserveEmptyBytesAndIsolateStorage(t *testing.T) {
	for _, data := range [][]byte{nil, {}, {1, 2}} {
		color := ImageColorSpace{lookup: data}
		lookup := color.LookupCopy()
		if (lookup == nil) != (data == nil) {
			t.Fatalf("lookup nil = %v, want %v", lookup == nil, data == nil)
		}
		strings := cloneStringByteMap(map[string][]byte{"glyph": data})
		if (strings["glyph"] == nil) != (len(data) == 0) {
			t.Fatalf("charstring = %#v, want empty inputs collapsed to nil", strings["glyph"])
		}
		state := &textObjectState{}
		appendTextArg(state, String(data), true)
		text := state.args[0].(String)
		if (text == nil) != (len(data) == 0) {
			t.Fatalf("text = %#v, want empty inputs collapsed to nil", text)
		}
		if len(data) > 0 {
			lookup[0], strings["glyph"][0], text[0] = 9, 8, 7
			if data[0] != 1 {
				t.Fatal("owned copy mutated original storage")
			}
		}
	}
}
