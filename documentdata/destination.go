package documentdata

import (
	"encoding/json"

	"github.com/lin-string/go-playa/pdftypes/primitives"
)

// Destination is the document-independent form of a normalized PDF
// destination. Page and coordinate-space resolution remains document-owned.
type Destination struct {
	page         primitives.Ref
	hasPage      bool
	pageIndex    int
	hasPageIndex bool
	view         string
	params       []primitives.Object
}

// NewDestination constructs a normalized destination value.
func NewDestination(page primitives.Ref, hasPage bool, pageIndex int, hasPageIndex bool, view string, params []primitives.Object) Destination {
	return Destination{
		page: page, hasPage: hasPage, pageIndex: pageIndex, hasPageIndex: hasPageIndex,
		view: view, params: cloneObjects(params),
	}
}

func (t Destination) PageRef() (primitives.Ref, bool) { return t.page, t.hasPage }
func (t Destination) PageIndex() (int, bool)          { return t.pageIndex, t.hasPageIndex }
func (t Destination) View() string                    { return t.view }

// NCoords reports how many explicit coordinates the destination view accepts.
// Unknown and page-fitting views have no explicit coordinate tuple.
func (t Destination) NCoords() int {
	switch t.view {
	case "XYZ":
		return 3
	case "FitH", "FitBH", "FitV", "FitBV":
		return 1
	case "FitR":
		return 4
	default:
		return 0
	}
}

func (t Destination) ParamsCount() int { return len(t.params) }

// ParamCopy returns an independent parameter value by index.
func (t Destination) ParamCopy(index int) (primitives.Object, bool) {
	if index < 0 || index >= len(t.params) {
		return nil, false
	}
	return cloneObject(t.params[index]), true
}

func (t Destination) ParamsCopy() []primitives.Object { return cloneObjects(t.params) }
func (t Destination) Finalize() Destination {
	return NewDestination(t.page, t.hasPage, t.pageIndex, t.hasPageIndex, t.view, cloneObjects(t.params))
}

func (t Destination) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Page         primitives.Ref
		HasPage      bool
		PageIndex    int
		HasPageIndex bool
		View         string
		Params       []primitives.Object `json:"Params"`
	}{Page: t.page, HasPage: t.hasPage, PageIndex: t.pageIndex, HasPageIndex: t.hasPageIndex, View: t.view, Params: t.params})
}

func cloneObjects(values []primitives.Object) []primitives.Object {
	if values == nil {
		return nil
	}
	cloned := make([]primitives.Object, len(values))
	for i, value := range values {
		cloned[i] = cloneObject(value)
	}
	return cloned
}
