package document

import (
	"fmt"

	"github.com/lin-string/go-playa/documentdata"
)

// EncryptionInfo is the owned public view of a PDF encryption dictionary.
type EncryptionInfo = documentdata.EncryptionInfo

// Encryption reports whether the document has a valid encryption metadata
// snapshot and returns that snapshot. It suppresses malformed metadata errors
// for parity with the other lenient document accessors.
func (d *Document) Encryption() (EncryptionInfo, bool) {
	info, present, err := d.EncryptionWithError()
	return info, present && err == nil
}

// EncryptionWithError returns the document encryption metadata without
// exposing password keys or the internal decipher implementation. The bool is
// false when the document has no /Encrypt entry.
func (d *Document) EncryptionWithError() (EncryptionInfo, bool, error) {
	raw, present := d.trailer[Name("Encrypt")]
	if !present {
		return EncryptionInfo{}, false, nil
	}
	value, ok := d.resolveIndirectChain(raw)
	if !ok {
		return EncryptionInfo{}, true, fmt.Errorf("playa: Encrypt could not be resolved")
	}
	dict, ok := value.(Dict)
	if !ok {
		return EncryptionInfo{}, true, fmt.Errorf("playa: Encrypt is not a dictionary")
	}
	ids, err := d.encryptionFileIDs()
	if err != nil {
		return EncryptionInfo{}, true, err
	}
	return documentdata.NewEncryptionInfo(ids, dict), true, nil
}

func (d *Document) encryptionFileIDs() ([][]byte, error) {
	value, present := d.trailer[Name("ID")]
	if !present {
		return nil, fmt.Errorf("playa: trailer ID is missing")
	}
	value, ok := d.resolveIndirectChain(value)
	if !ok {
		return nil, fmt.Errorf("playa: trailer ID could not be resolved")
	}
	ids, ok := value.(Array)
	if !ok {
		return nil, fmt.Errorf("playa: trailer ID is not an array")
	}
	out := make([][]byte, len(ids))
	for i, item := range ids {
		item, ok := d.resolveIndirectChain(item)
		if !ok {
			return nil, fmt.Errorf("playa: trailer ID entry %d could not be resolved", i)
		}
		id, ok := item.(String)
		if !ok {
			return nil, fmt.Errorf("playa: trailer ID entry %d is not a string", i)
		}
		if len(id) > 0 {
			out[i] = cloneObjectBytes(id)
		}
	}
	return out, nil
}
