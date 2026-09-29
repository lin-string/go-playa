package documentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// EncryptionInfo is the public, document-independent view of a PDF Standard
// security dictionary. Password keys and the active decipher callback remain
// private to document parsing.
type EncryptionInfo struct {
	ids  [][]byte
	dict primitives.Dict
}

// NewEncryptionInfo constructs an owned encryption metadata snapshot.
func NewEncryptionInfo(ids [][]byte, dict primitives.Dict) EncryptionInfo {
	return EncryptionInfo{ids: cloneByteSlices(ids), dict: cloneDict(dict)}
}

// IDsCopy returns the trailer file identifiers as independent byte slices.
func (e EncryptionInfo) IDsCopy() [][]byte { return cloneByteSlices(e.ids) }

// DictCopy returns an independent copy of the encryption dictionary.
func (e EncryptionInfo) DictCopy() primitives.Dict { return cloneDict(e.dict) }

// Finalize returns an independent encryption metadata snapshot.
func (e EncryptionInfo) Finalize() EncryptionInfo {
	return NewEncryptionInfo(e.ids, e.dict)
}

func (e EncryptionInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		IDs  [][]byte        `json:"IDs"`
		Dict primitives.Dict `json:"Dict"`
	}{IDs: e.IDsCopy(), Dict: e.DictCopy()})
}

func cloneByteSlices(value [][]byte) [][]byte {
	if value == nil {
		return nil
	}
	out := make([][]byte, len(value))
	for i, item := range value {
		if len(item) > 0 {
			out[i] = primitives.CloneBytes(item)
		}
	}
	return out
}
