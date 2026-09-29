package document

import (
	"errors"
	"os"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestEncryptedR2AcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_encrypted_r2.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenBytes(data); !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("missing password error = %v", err)
	}
	if _, err := OpenBytes(data, WithPassword("wrong")); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("wrong password error = %v", err)
	}
	d, err := OpenBytes(data, WithPassword("secret"))
	if err != nil {
		t.Fatal(err)
	}
	info, encrypted, err := d.EncryptionWithError()
	if err != nil || !encrypted || len(info.IDsCopy()) != 2 {
		t.Fatalf("encryption metadata = %#v, %v, %v", info, encrypted, err)
	}
	if filter, ok := info.DictCopy()[Name("Filter")].(Name); !ok || filter != Name("Standard") {
		t.Fatalf("encryption filter = %#v", info.DictCopy()[Name("Filter")])
	}
	page, err := d.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	texts, err := d.PageText(page)
	if err != nil || len(texts) != 1 || texts[0].Text() != "Encrypted report" {
		t.Fatalf("decrypted text = %#v, err=%v", texts, err)
	}
}

func TestEncryptedObjectsExposePublicDecryptedView(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_encrypted_r2.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data, WithPassword("secret"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	ref := Ref{Object: 9, Generation: 0}
	raw, err := d.resolveRefRaw(ref)
	if err != nil {
		t.Fatal(err)
	}
	public, err := d.ObjectWithError(ref)
	if err != nil {
		t.Fatal(err)
	}
	rawDict, rawOK := raw.(Dict)
	publicDict, publicOK := public.(Dict)
	if !rawOK || !publicOK {
		t.Fatalf("encryption object types = %T/%T", raw, public)
	}
	rawOwner, rawOK := rawDict[Name("O")].(String)
	publicOwner, publicOK := publicDict[Name("O")].(String)
	if !rawOK || !publicOK || string(rawOwner) == string(publicOwner) {
		t.Fatalf("public encryption dictionary did not decrypt O: raw=%x public=%x", []byte(rawOwner), []byte(publicOwner))
	}
}

func TestEncryptedObjectWithErrorDoesNotDecryptCachedObjectTwice(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_encrypted_r2.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data, WithPassword("secret"))
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := d.ObjectWithError(Ref{Object: 5})
	if err != nil {
		t.Fatal(err)
	}
	stream, ok := decrypted.(Stream)
	if !ok || string(stream.DataBorrowed()) != "BT /F1 12 Tf 72 740 Td (Encrypted report) Tj ET" {
		t.Fatalf("public encrypted stream = %#v, want decrypted stream", decrypted)
	}
}
