package testcompat_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/lin-string/go-playa/internal/testcompat"
)

func TestXObjectResourceGraphPreservesSharedCycles(t *testing.T) {
	project := func(payload string) map[string]interface{} {
		t.Helper()
		pdf := testPDF(
			"<< /Type /Catalog /Pages 2 0 R >>",
			"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << /XObject << /Fm 4 0 R >> >> /Contents 5 0 R >>",
			"<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] /Resources 6 0 R /Length 3 >>\nstream\nq Q\nendstream",
			"<< /Length 6 >>\nstream\n/Fm Do\nendstream",
			"<< /Properties << /Payload 7 0 R /Shared 8 0 R >> /Aliases [8 0 R 8 0 R] /Cycle 9 0 R /Empty [] >>",
			fmt.Sprintf("<< /Back 6 0 R /Next 8 0 R /Length %d >>\nstream\n%s\nendstream", len(payload), payload),
			"<< /Self 8 0 R /Next 7 0 R /Bytes <ff00> >>",
			"10 0 R",
			"9 0 R",
		)
		doc, err := openPagePDF(pdf)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = doc.Close() }()
		page, err := doc.PageAt(0)
		if err != nil {
			t.Fatal(err)
		}
		var first map[string]interface{}
		for run := 0; run < 2; run++ {
			snapshot, err := testcompat.PageSnapshotSections(doc, page, 0, []string{"content.xobjects"})
			if err != nil {
				t.Fatal(err)
			}
			if len(snapshot.XObjects) != 1 {
				t.Fatalf("xobjects=%d", len(snapshot.XObjects))
			}
			got := snapshot.XObjects[0].Resources
			if run == 0 {
				first = got
			} else if !reflect.DeepEqual(first, got) {
				t.Fatal("graph changed between repeated projections")
			}
		}
		return first
	}
	graph := project("payload")
	nodes, ok := graph["objects"].([]interface{})
	if !ok || len(nodes) != 5 {
		t.Fatalf("graph=%#v, want five unique reachable nodes", graph)
	}
	for i, id := range []int{6, 7, 8, 9, 10} {
		if nodes[i].(map[string]interface{})["object"] != id {
			t.Fatalf("nodes not sorted: %#v", nodes)
		}
	}
	root := graph["root"].(map[string]interface{})
	aliases := root["Aliases"].([]interface{})
	if len(aliases) != 2 || !reflect.DeepEqual(aliases[0], map[string]interface{}{"ref": 8}) || !reflect.DeepEqual(aliases[0], aliases[1]) {
		t.Fatalf("aliases=%#v", aliases)
	}
	if empty := root["Empty"].([]interface{}); empty == nil || len(empty) != 0 {
		t.Fatal("empty array must be nonnil")
	}
	stream := nodes[1].(map[string]interface{})["value"].(map[string]interface{})
	if stream["length"] != 7 || stream["sha256"] != fmt.Sprintf("%x", sha256.Sum256([]byte("payload"))) {
		t.Fatalf("stream=%#v", stream)
	}
	attrs := stream["dict"].(map[string]interface{})
	if !reflect.DeepEqual(attrs["Back"], map[string]interface{}{"ref": 6}) {
		t.Fatalf("stream back ref=%#v", attrs)
	}
	shared := nodes[2].(map[string]interface{})["value"].(map[string]interface{})
	if shared["Bytes"] != "ff00" || !reflect.DeepEqual(shared["Self"], map[string]interface{}{"ref": 8}) {
		t.Fatalf("shared=%#v", shared)
	}
	for i, target := range []int{10, 9} {
		if got := nodes[i+3].(map[string]interface{})["value"]; !reflect.DeepEqual(got, map[string]interface{}{"ref": target}) {
			t.Fatalf("alias node %d = %#v, want direct reference %d", i+9, got, target)
		}
	}
	encoded, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 1000 {
		t.Fatalf("graph unexpectedly expanded: %d bytes", len(encoded))
	}
	changed := project("changed")["objects"].([]interface{})[1].(map[string]interface{})["value"].(map[string]interface{})
	if changed["sha256"] == stream["sha256"] {
		t.Fatal("changed stream bytes did not change digest")
	}
}
