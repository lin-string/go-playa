package documentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// Action is the document-independent value portion of a normalized PDF
// action. Document-bound destination lookup, lazy /Next traversal, and
// deferred parser errors remain owned by document.Action.
type Action struct {
	kind   string
	uri    string
	file   string
	name   string
	script string
	raw    primitives.Dict
}

// NewAction constructs an action value and takes ownership of a recursive
// snapshot of raw.
func NewAction(kind, uri, file, name, script string, raw primitives.Dict) Action {
	return Action{kind: kind, uri: uri, file: file, name: name, script: script, raw: cloneObject(raw).(primitives.Dict)}
}

func (a Action) Kind() string   { return a.kind }
func (a Action) URI() string    { return a.uri }
func (a Action) File() string   { return a.file }
func (a Action) Name() string   { return a.name }
func (a Action) Script() string { return a.script }

// RawCopy returns an independent copy of the source action dictionary.
func (a Action) RawCopy() primitives.Dict {
	if a.raw == nil {
		return nil
	}
	return cloneObject(a.raw).(primitives.Dict)
}

// Finalize returns an independent action value snapshot.
func (a Action) Finalize() Action {
	return NewAction(a.kind, a.uri, a.file, a.name, a.script, a.raw)
}

func (a Action) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string
		URI    string
		File   string
		Name   string
		Script string
		Raw    primitives.Dict `json:"Raw"`
	}{Kind: a.kind, URI: a.uri, File: a.file, Name: a.name, Script: a.script, Raw: a.raw})
}
