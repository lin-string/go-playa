package contentdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/contentdata"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/pdftypes/primitives"
)

func TestTextOwnsDependencyFreeState(t *testing.T) {
	args := []primitives.Object{primitives.String("A")}
	stroke := []float64{0.1, 0.2, 0.3}
	text := contentdata.NewText(contentdata.TextSpec{
		Text: "A", Chars: "A", Page: primitives.Ref{Object: 7}, HasPage: true,
		Args: args, Matrix: geometry.Matrix{1, 0, 0, 1, 2, 3},
		BBox: [4]float64{1, 2, 3, 4}, StrokeColor: stroke,
		VisualBBox: &[4]float64{1, 2, 3, 4},
	})
	args[0] = primitives.String("B")
	stroke[0] = 9
	if text.Text() != "A" || !text.HasPage() || text.Page() != (primitives.Ref{Object: 7}) {
		t.Fatalf("text metadata = %#v", text)
	}
	if got := text.ArgsCopy()[0].(primitives.String); string(got) != "A" {
		t.Fatalf("text retained caller args = %q", got)
	}
	if got := text.StrokeColorCopy(); got[0] != 0.1 {
		t.Fatalf("text retained caller color = %v", got)
	}
	if got, ok := text.VisualBBoxCopy(); !ok || got != [4]float64{1, 2, 3, 4} {
		t.Fatalf("text visual bbox = %v, %v", got, ok)
	}
}

func TestTextBorrowedSequenceAndFinalize(t *testing.T) {
	args := []primitives.Object{primitives.String("A"), primitives.Number(2)}
	text := contentdata.NewTextBorrowed(contentdata.TextSpec{Args: args})
	args[0] = primitives.String("B")
	if got := text.ArgsBorrowed()[0].(primitives.String); string(got) != "B" {
		t.Fatalf("borrowed text args = %q", got)
	}
	final := text.Finalize()
	args[0] = primitives.String("C")
	if got := final.ArgsCopy()[0].(primitives.String); string(got) != "B" {
		t.Fatalf("finalized text args = %q", got)
	}
}
