package document

import "testing"

func TestDocumentEncryptionSnapshot(t *testing.T) {
	d := &Document{trailer: Dict{
		Name("ID"):      Array{String{1, 2}, String{3, 4}},
		Name("Encrypt"): Dict{Name("Filter"): Name("Standard"), Name("V"): Number(1)},
	}}
	info, ok, err := d.EncryptionWithError()
	if err != nil || !ok {
		t.Fatalf("EncryptionWithError() = %#v, %v, %v", info, ok, err)
	}
	ids := info.IDsCopy()
	ids[0][0] = 9
	if _, got := d.Encryption(); !got {
		t.Fatal("Encryption() did not report encrypted document")
	}
}

func TestDocumentEncryptionSnapshotAbsent(t *testing.T) {
	d := &Document{trailer: Dict{}}
	if info, ok, err := d.EncryptionWithError(); err != nil || ok || info.DictCopy() != nil {
		t.Fatalf("unencrypted EncryptionWithError() = %#v, %v, %v", info, ok, err)
	}
}
