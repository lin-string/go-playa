package contentdata_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestPropertiesObjectOwnsDependencyFreeValue(t *testing.T) {
	page := primitives.Ref{Object: 7}
	source := primitives.Dict{
		primitives.Name("MCID"): primitives.Number(3),
		primitives.Name("Nested"): primitives.Dict{
			primitives.Name("Value"): primitives.String("source"),
		},
	}
	value := contentdata.NewPropertiesObject("P", "BDC", page, true, source)

	copyValue := value.DictCopy()
	copyValue[primitives.Name("Nested")].(primitives.Dict)[primitives.Name("Value")] = primitives.String("copy")
	if !bytes.Equal([]byte(source[primitives.Name("Nested")].(primitives.Dict)[primitives.Name("Value")].(primitives.String)), []byte("source")) {
		t.Fatal("PropertiesObject.DictCopy shares nested dictionary storage")
	}

	finalized := value.Finalize()
	if finalized.Name() != "P" || finalized.Operator() != "BDC" || finalized.Page() != page || !finalized.HasPage() {
		t.Fatalf("PropertiesObject metadata = %q/%q/%#v/%v", finalized.Name(), finalized.Operator(), finalized.Page(), finalized.HasPage())
	}
	if _, err := json.Marshal(finalized); err != nil {
		t.Fatalf("PropertiesObject JSON = %v", err)
	}
}
