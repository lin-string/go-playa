package testcompat_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/internal/testcompat"
)

func TestMarkedPropertiesPreserveStreamReference(t *testing.T) {
	for _, bad := range []bool{false, true} {
		data := []byte("FEFF00410042>")
		if bad {
			data = []byte("not hex>")
		}
		content := "/PlacedPDF /Props BDC 0 0 2 3 re f EMC /PlacedPDF /Props DP"
		doc, err := document.OpenBytes(testPDF(
			"<< /Type /Catalog /Pages 2 0 R >>",
			"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Resources << /Properties << /Props << /Metadata 5 0 R /Custom 6 0 R /Nested [5 0 R << /Item 6 0 R >>] >> >> >> /Contents 4 0 R >>",
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
			fmt.Sprintf("<< /Filter /ASCIIHexDecode /Link 9 0 R /Length %d >>\nstream\n%s\nendstream", len(data), data),
			"<< /Value 23 >>",
		))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = doc.Close() }()
		page, err := doc.PageAt(0)
		if err != nil {
			t.Fatal(err)
		}
		tagCount := 0
		for tag, err := range page.Tags(doc) {
			tagCount++
			if err != nil {
				t.Fatal(err)
			}
			if got := tag.PropertiesCopy()["Metadata"]; got != (document.Ref{Object: 5}) {
				t.Errorf("named DP metadata=%T", got)
			}
		}
		if tagCount != 1 {
			t.Fatalf("tags=%d", tagCount)
		}
		markedCount := 0
		for marked, err := range page.MarkedContent(doc) {
			markedCount++
			if err != nil {
				t.Fatal(err)
			}
			if got := marked.PropertiesCopy()["Metadata"]; got != (document.Ref{Object: 5}) {
				t.Errorf("named BDC metadata=%T", got)
			}
		}
		if markedCount != 1 {
			t.Fatalf("marked sections=%d", markedCount)
		}
		var paths []document.PathObject
		for path, err := range page.Paths(doc) {
			if err != nil {
				t.Fatal(err)
			}
			props := path.MarkedPropertiesCopy()
			if props["Metadata"] != (document.Ref{Object: 5}) || props["Custom"] != (document.Ref{Object: 6}) {
				t.Fatalf("raw property references lost: metadata=%T custom=%T", props["Metadata"], props["Custom"])
			}
			nested := props["Nested"].(document.Array)
			if nested[0] != (document.Ref{Object: 5}) || nested[1].(document.Dict)["Item"] != (document.Ref{Object: 6}) {
				t.Fatal("nested raw references lost")
			}
			paths = append(paths, path.Finalize())
		}
		if len(paths) != 1 {
			t.Fatalf("paths=%d", len(paths))
		}
		for repeat := 0; repeat < 2; repeat++ {
			encoded, err := json.Marshal(testcompat.PathSnapshot(paths[0], 0))
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]interface{}
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			for _, props := range []interface{}{got["marked_properties"], got["marked_stack"].([]interface{})[0].(map[string]interface{})["properties"]} {
				stream, ok := props.(map[string]interface{})["Metadata"].(map[string]interface{})
				if !ok {
					t.Fatalf("metadata=%T, want stream object", props.(map[string]interface{})["Metadata"])
				}
				if len(stream) != 1 || stream["ref"] != float64(5) {
					t.Fatalf("metadata reference=%v", stream)
				}
			}
		}
	}
}
