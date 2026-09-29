package documentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/documentdata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestAnnotationValuePublicAPI(t *testing.T) {
	value := documentdata.NewAnnotation(documentdata.AnnotationSpec{})
	_ = value.Subtype
	_ = value.Page
	_ = value.InReplyTo
	_ = value.Popup
	_ = value.Rect
	_ = value.DictCopy
	_ = value.ActionCopy
	_ = value.DestCopy
	_ = value.AppearanceCopy
	_ = value.QuadPointsCopy
	_ = value.ColorCopy
	_ = value.WithPage
	_ = value.Finalize
	var _ primitives.Object = primitives.Null{}
}

func TestFormFieldValuePublicAPI(t *testing.T) {
	value := documentdata.NewFormField(documentdata.FormFieldSpec{})
	_ = value.Name
	_ = value.FullName
	_ = value.Page
	_ = value.Parent
	_ = value.FieldType
	_ = value.ValuesCopy
	_ = value.DefaultValuesCopy
	_ = value.OptionsCopy
	_ = value.OptionValuesCopy
	_ = value.SelectedCopy
	_ = value.DictCopy
	_ = value.Finalize
}

func TestEncryptionInfoPublicAPI(t *testing.T) {
	value := documentdata.NewEncryptionInfo(nil, nil)
	_ = value.IDsCopy
	_ = value.DictCopy
	_ = value.Finalize
}

func TestDestinationPublicAPI(t *testing.T) {
	value := documentdata.NewDestination(primitives.Ref{}, false, 0, false, "Fit", nil)
	_ = value.PageRef
	_ = value.PageIndex
	_ = value.View
	_ = value.NCoords
	_ = value.ParamsCount
	_ = value.ParamCopy
	_ = value.ParamsCopy
	_ = value.Finalize
}

func TestOutlineNodePublicAPI(t *testing.T) {
	value := documentdata.NewOutlineNode(documentdata.OutlineNodeSpec{})
	_ = value.Title
	_ = value.Parent
	_ = value.HasParent
	_ = value.ActionKind
	_ = value.ElementRef
	_ = value.HasElement
	_ = value.Count
	_ = value.HasCount
	_ = value.ActionCopy
	_ = value.TargetCopy
	_ = value.ActionValueCopy
	_ = value.DestCopy
	_ = value.Finalize
}
