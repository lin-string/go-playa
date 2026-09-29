package document

import (
	"os"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestFormChoiceAcceptanceFixture(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_form_choice.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := d.CollectFormFields()
	if err != nil || len(fields) != 1 {
		t.Fatalf("form fields = %#v, err=%v", fields, err)
	}
	field := fields[0]
	if field.Name() != "report_type" || field.FieldType() != "Ch" || field.Value() != "Annual" {
		t.Fatalf("form field = %#v", field)
	}
	if got := field.OptionsCopy(); len(got) != 2 || got[0] != "Annual" || got[1] != "Quarterly" {
		t.Fatalf("form options = %#v", got)
	}
	kids, err := field.KidsCopy()
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) != 1 || !kids[0].IsWidget() || kids[0].Rect() != [4]float64{72, 680, 220, 710} {
		t.Fatalf("form widget = %#v", kids)
	}
	page, err := kids[0].PageObject(d)
	if err != nil || page.ref.Object != 3 {
		t.Fatalf("form widget page = %#v, err=%v", page, err)
	}
}
