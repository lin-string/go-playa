package structuredata_test

import (
	"testing"

	"github.com/lin-string/go-playa/structuredata"
)

func TestPublicContentAPI(t *testing.T) {
	value := structuredata.NewContent(structuredata.ContentSpec{})
	_ = value.Kind
	_ = value.MCID
	_ = value.HasMCID
	_ = value.Page
	_ = value.HasPage
	_ = value.HasStream
	_ = value.StreamCopy
	_ = value.ObjectRef
	_ = value.HasObject
	_ = value.DictCopy
	_ = value.Finalize
	_ = testing.Short
}

func TestPublicItemAPI(t *testing.T) {
	value := structuredata.NewItem(structuredata.ItemSpec{})
	_ = value.Kind
	_ = value.ContentBorrowed
	_ = value.ContentCopy
	_ = value.Finalize
}

func TestPublicElementAPI(t *testing.T) {
	value := structuredata.NewElement(structuredata.ElementSpec{})
	_ = value.Type
	_ = value.StructureType
	_ = value.Role
	_ = value.RawRole
	_ = value.Title
	_ = value.Language
	_ = value.AlternateDescription
	_ = value.ActualText
	_ = value.AbbreviationExpansion
	_ = value.ClassName
	_ = value.Page
	_ = value.HasPage
	_ = value.Parent
	_ = value.HasParent
	_ = value.MCID
	_ = value.HasMCID
	_ = value.IsMCR
	_ = value.ObjectRef
	_ = value.HasObject
	_ = value.BBox
	_ = value.HasBBox
	_ = value.AttributesCopy
	_ = value.DictCopy
	_ = value.ObjectCopy
	_ = value.Finalize
}
