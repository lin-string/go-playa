package documentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestEncryptionInfoOwnsIDsAndDictionary(t *testing.T) {
	info := documentdata.NewEncryptionInfo([][]byte{{1, 2}, {3, 4}}, primitives.Dict{
		primitives.Name("Filter"): primitives.Name("Standard"),
		primitives.Name("O"):      primitives.String{5, 6},
	})
	ids := info.IDsCopy()
	ids[0][0] = 9
	dict := info.DictCopy()
	dict[primitives.Name("O")].(primitives.String)[0] = 9
	if got := info.IDsCopy()[0][0]; got != 1 {
		t.Fatalf("encryption IDs were not copied: %d", got)
	}
	if got := info.DictCopy()[primitives.Name("O")].(primitives.String)[0]; got != 5 {
		t.Fatalf("encryption dictionary was not copied: %d", got)
	}
	if snapshot := info.Finalize(); snapshot.DictCopy()[primitives.Name("Filter")] != primitives.Name("Standard") {
		t.Fatalf("encryption snapshot = %#v", snapshot)
	}
}

func TestEncryptionInfoCopiesPreserveAbsentAndEmptyIDs(t *testing.T) {
	if ids := documentdata.NewEncryptionInfo(nil, nil).IDsCopy(); ids != nil {
		t.Fatalf("absent IDs = %#v, want nil", ids)
	}
	info := documentdata.NewEncryptionInfo([][]byte{nil, {}}, nil)
	for _, snapshot := range []documentdata.EncryptionInfo{info, info.Finalize()} {
		ids := snapshot.IDsCopy()
		if len(ids) != 2 || ids[0] != nil || ids[1] != nil {
			t.Fatalf("empty IDs = %#v, want two nil entries", ids)
		}
	}
}
