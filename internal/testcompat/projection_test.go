package testcompat_test

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	core "github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/geometry"
	"github.com/lin-string/go-playa/internal/testcompat"
	"github.com/lin-string/go-playa/internal/testfixture"
)

func openPagePDF(data []byte) (*core.Document, error) {
	return core.OpenBytes(data, core.WithCoordinateSpace(core.CoordinateSpacePage))
}

func TestTextSnapshotKeepsGoOnlyRotationOutOfCompatibilitySchema(t *testing.T) {
	ops, err := core.ParseContent([]byte("BT 0 1 -1 0 10 20 Tm /F1 10 Tf (A) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	texts := core.InterpretText(ops)
	if len(texts) != 1 || texts[0].Rotation() < 89 || texts[0].Rotation() > 91 {
		t.Fatalf("source text rotation = %#v", texts)
	}
	if got := testcompat.TextSnapshot(texts[0], 0).Rotation; got != 0 {
		t.Fatalf("compatibility rotation = %v, want 0", got)
	}
}

func TestMappingSnapshotPreservesEncryptedSourceView(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_encrypted_r2.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := core.OpenBytes(data, core.WithPassword("secret"))
	if err != nil {
		t.Fatal(err)
	}
	var streamRecord, encryptionRecord *testcompat.MappingRecord
	for item, itemErr := range doc.Items() {
		if itemErr != nil {
			t.Fatal(itemErr)
		}
		record, recordErr := testcompat.MappingSnapshot(doc, item)
		if recordErr != nil {
			t.Fatal(recordErr)
		}
		switch item.Ref().Object {
		case 5:
			streamRecord = &record
		case 9:
			encryptionRecord = &record
		}
	}
	if streamRecord == nil || streamRecord.Stream == nil || streamRecord.Stream.SHA256 != "628d2303f2d16fe2108d3a6e79265eef4d08a8141acfe6d7865ccf2b5770cf06" {
		t.Fatalf("encrypted mapping stream = %#v, want decrypted stream digest", streamRecord)
	}
	value, ok := encryptionRecord.Value.(map[string]interface{})
	if !ok || value["O"] != "92fe0f4454ad4c9644693f33c07cb54f587dce1e2682fe9ecea6107a1ef630dd" {
		t.Fatalf("encrypted mapping dictionary = %#v, want source O value", encryptionRecord)
	}
}

func TestSnapshotProjectsStablePageFields(t *testing.T) {
	doc, err := core.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"), core.WithCoordinateSpace(core.CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := doc.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != testcompat.SchemaVersion {
		t.Fatalf("schema version = %q, want %q", snapshot.SchemaVersion, testcompat.SchemaVersion)
	}
	if snapshot.PDFVersion != "1.4" || snapshot.IsTagged || !snapshot.IsPrintable || !snapshot.IsModifiable || !snapshot.IsExtractable {
		t.Fatalf("document projection = version=%q tagged=%v printable=%v modifiable=%v extractable=%v", snapshot.PDFVersion, snapshot.IsTagged, snapshot.IsPrintable, snapshot.IsModifiable, snapshot.IsExtractable)
	}
	if snapshot.Catalog["Type"] != "Catalog" || len(snapshot.Info) != 0 || len(snapshot.Names) != 0 {
		t.Fatalf("document dictionaries = info=%#v catalog=%#v names=%#v", snapshot.Info, snapshot.Catalog, snapshot.Names)
	}
	if root, ok := snapshot.Trailer["Root"].(map[string]interface{}); !ok || root["ref"] != 1 {
		t.Fatalf("trailer root projection = %#v", snapshot.Trailer["Root"])
	}
	if len(snapshot.Pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(snapshot.Pages))
	}
}

func TestFontSnapshotProjectsPlayaIdentityMetadata(t *testing.T) {
	doc, err := core.OpenBytes([]byte(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100]
/Resources << /Font << /F1 4 0 R >> >> >>
endobj
4 0 obj
<< /Type /Font /Subtype /Type0 /BaseFont /RootFont-Identity-H
/Encoding /Identity-H /DescendantFonts [5 0 R] >>
endobj
5 0 obj
<< /Type /Font /Subtype /CIDFontType2 /BaseFont /DescendantFont
/CIDSystemInfo << /Registry (Adobe) /Ordering (Japan1) /Supplement 7 >>
/FontDescriptor << /FontName /DescendantFont >> >>
endobj
trailer << /Root 1 0 R >>
%%EOF
`), core.WithCoordinateSpace(core.CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	page, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	for resource, resourceErr := range page.FontsSeq(doc) {
		if resourceErr != nil {
			t.Fatal(resourceErr)
		}
		font, ok := testcompat.FontSnapshot(resource)
		if !ok {
			t.Fatal("font snapshot was unavailable")
		}
		if font.BaseFont != "DescendantFont" || font.CIDCoding != "Adobe-Japan1" {
			t.Fatalf("font identity projection = %#v", font)
		}
		return
	}
	t.Fatal("page had no font resources")
}

func TestSnapshotProjectsDocumentFontsWhenFontsSectionIsSelected(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openPagePDF(data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{
		Pages: []int{0}, Sections: []string{"fonts"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.DocumentFonts) == 0 {
		t.Fatal("document font mapping was omitted from the fonts section")
	}
}

func TestSnapshotMetadataEncodesEmptyDocumentFontsAsEmptySequence(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "form_simple.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openPagePDF(data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	report, err := testcompat.SnapshotMetadata(doc, []string{"fonts"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"document_fonts":[]`)) {
		t.Fatalf("empty document font projection = %s, want []", encoded)
	}
}

func TestSnapshotProjectsEmptyDashAsAnEmptySequence(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_inline_image.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openPagePDF(data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{
		Pages: []int{0}, Sections: []string{"content.flatten"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages) == 0 || len(snapshot.Pages[0].Flatten) == 0 {
		t.Fatal("flatten projection was empty")
	}
	for index, object := range snapshot.Pages[0].Flatten {
		if object.State.Dash == nil {
			t.Fatalf("flatten[%d] dash is nil, want an empty sequence", index)
		}
	}
}

func TestSnapshotProjectsExplicitTextExtractionModes(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_tagged_text.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := core.OpenBytes(data, core.WithCoordinateSpace(core.CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := document.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()

	snapshot, err := testcompat.Snapshot(document, testcompat.Options{
		Pages:    []int{0},
		Sections: []string{"content.extract_text.tagged", "content.extract_text.untagged"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(snapshot.Pages))
	}
	page, err := document.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	wantTagged, err := page.ExtractTextTagged(document, core.DefaultTextExtractionOptions())
	if err != nil {
		t.Fatal(err)
	}
	wantUntagged, err := page.ExtractTextUntagged(document, core.DefaultTextExtractionOptions())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Pages[0].ExtractTextTagged != wantTagged || snapshot.Pages[0].ExtractTextUntagged != wantUntagged {
		t.Fatalf("explicit extraction projections = tagged %q/untagged %q, want %q/%q", snapshot.Pages[0].ExtractTextTagged, snapshot.Pages[0].ExtractTextUntagged, wantTagged, wantUntagged)
	}
}

func TestSnapshotProjectsFallbackTrailerLikePlaya(t *testing.T) {
	doc, err := openPagePDF([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >>\nendobj\ntrailing damaged data\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Sections: []string{"document"}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"Type": "Page", "Parent": map[string]interface{}{"ref": 2}, "MediaBox": []interface{}{float64(0), float64(0), float64(595), float64(842)}, "Size": 3}
	if !reflect.DeepEqual(snapshot.Trailer, want) {
		t.Fatalf("fallback trailer = %#v, want %#v", snapshot.Trailer, want)
	}
}

func TestSnapshotPreservesHybridTrailerXRefStm(t *testing.T) {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 5)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>")
	xrefOffset := len(data)
	data = append(data, []byte("xref\n0 3\n0000000000 65535 f \n")...)
	for number := 1; number <= 2; number++ {
		data = append(data, []byte(fmt.Sprintf("%010d 00000 n \n", offsets[number]))...)
	}
	xrefStreamOffset := len(data)
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	raw := append(record(1, offsets[3], 0), record(1, xrefStreamOffset, 0)...)
	data = append(data, fmt.Sprintf("4 0 obj\n<< /Type /XRef /Size 5 /Root 1 0 R /W [1 4 2] /Index [3 2] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\ntrailer\n<< /Size 5 /Root 1 0 R /XRefStm %d >>\nstartxref\n%d\n%%%%EOF\n", xrefStreamOffset, xrefOffset))...)

	doc, err := openPagePDF(data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Sections: []string{"document"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := snapshot.Trailer["XRefStm"].(float64); !ok || got != float64(xrefStreamOffset) {
		t.Fatalf("hybrid trailer XRefStm = %#v, want %d", snapshot.Trailer["XRefStm"], xrefStreamOffset)
	}
}

func TestSnapshotProjectsIndirectObjectsWithoutRetainingStreamBytes(t *testing.T) {
	data := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>",
	)
	doc, err := openPagePDF(data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"document.objects"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Objects) != 3 || snapshot.Objects[0].Object != 1 || snapshot.Objects[0].Generation != 0 {
		t.Fatalf("objects = %#v", snapshot.Objects)
	}
	value, ok := snapshot.Objects[0].Value.(map[string]interface{})
	if !ok || value["Type"] != "Catalog" {
		t.Fatalf("catalog object = %#v", snapshot.Objects[0])
	}
}

func TestSnapshotProjectsDecodedResourceStreamDigest(t *testing.T) {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write([]byte("decoded-resource")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /XObject << /Fm 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Resources << /Properties << /Payload 6 0 R >> >> /Length 3 >> stream\nq Q\nendstream",
		"<< /Length 9 >> stream\nq /Fm Do Q\nendstream",
		fmt.Sprintf("<< /Length %d /Filter /FlateDecode >> stream\n%s\nendstream", compressed.Len(), compressed.Bytes()),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.xobjects"}})
	if err != nil {
		t.Fatal(err)
	}
	objects, ok := snapshot.Pages[0].XObjects[0].Resources["objects"].([]interface{})
	if !ok || len(objects) != 1 {
		t.Fatalf("resource objects = %#v, want one stream", snapshot.Pages[0].XObjects[0].Resources)
	}
	payload := objects[0].(map[string]interface{})["value"].(map[string]interface{})
	if payload["length"] != len("decoded-resource") || payload["sha256"] != fmt.Sprintf("%x", sha256.Sum256([]byte("decoded-resource"))) {
		t.Fatalf("resource stream projection = %#v, want decoded digest", payload)
	}
	if _, exists := payload["data"]; exists {
		t.Fatal("resource stream includes raw decoded data")
	}
	if len(snapshot.Pages[0].XObjects[0].Contents) == 0 {
		t.Fatal("xobject contents are missing when content.xobjects is selected")
	}
}

func TestSnapshotProjectsPlayaXObjectCTMAfterFormMatrix(t *testing.T) {
	contents := "q 1 2 3 4 5 6 cm /Fm Do"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /XObject << /Fm 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Matrix [7 8 9 10 11 12] /Length 3 >> stream\nq Q\nendstream",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.xobjects"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshot.Pages[0].XObjects[0].CTM; got != [6]float64{31, 46, 39, 58, 52, 76} {
		t.Fatalf("xobject ctm = %v, want Playa invocation CTM times Form Matrix", got)
	}
}

func TestSnapshotProjectsNestedPlayaXObjectCTMWithAncestorMatrices(t *testing.T) {
	pageContents := "1 2 0 1 10 20 cm /Parent Do"
	parentContents := "4 0 0 5 11 13 cm /Child Do"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /XObject << /Parent 4 0 R >> >> /Contents 6 0 R >>",
		fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Matrix [2 0 0 3 5 7] /Resources << /XObject << /Child 5 0 R >> >> /Length %d >> stream\n%s\nendstream", len(parentContents), parentContents),
		"<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Matrix [1 0 0 1 17 19] /Length 3 >> stream\nq Q\nendstream",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(pageContents), pageContents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.xobjects"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages[0].XObjects) != 2 {
		t.Fatalf("xobjects = %#v, want parent and child", snapshot.Pages[0].XObjects)
	}
	pageMatrix := geometry.Matrix{1, 2, 0, 1, 10, 20}
	parentMatrix := geometry.Matrix{2, 0, 0, 3, 5, 7}
	insideMatrix := geometry.Matrix{4, 0, 0, 5, 11, 13}
	childMatrix := geometry.Matrix{1, 0, 0, 1, 17, 19}
	want := pageMatrix.Mul(parentMatrix).Mul(insideMatrix).Mul(childMatrix)
	if got := snapshot.Pages[0].XObjects[1].CTM; got != want {
		t.Fatalf("nested xobject ctm = %v, want ancestor-aware Playa ctm %v", got, want)
	}
}

func TestSnapshotPreservesPlayaContentsContainerRecoveryAcrossStreams(t *testing.T) {
	first := "/Span << /Lang (en-GB) /MCID "
	second := "40 >> BDC"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{
		testcompat.TokenRecord{Kind: "name", Value: "Span"},
		testcompat.TokenRecord{Kind: "name", Value: "MCID"},
		testcompat.TokenRecord{Kind: "string", Value: "656e2d4742"},
		testcompat.TokenRecord{Kind: "name", Value: "Lang"},
		testcompat.TokenRecord{Kind: "dict_start"},
		testcompat.TokenRecord{Kind: "number", Value: float64(40)},
		"None",
		testcompat.TokenRecord{Kind: "keyword", Value: "BDC"},
	}
	if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
		t.Fatalf("cross-stream contents = %#v, want Playa recovery %#v", got, want)
	}
}

func TestSnapshotDropsUnclosedContainerAtFinalStreamEOFLikePlaya(t *testing.T) {
	contents := "/Span << /MCID 3"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{testcompat.TokenRecord{Kind: "name", Value: "Span"}}
	if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
		t.Fatalf("final unclosed contents = %#v, want Playa prefix %#v", got, want)
	}
}

func TestSnapshotRejectsContentStreamIndirectReferences(t *testing.T) {
	contents := "/Span << /Self 7 0 R >> BDC"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	if _, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}}); err == nil {
		t.Fatal("compatibility snapshot accepted an indirect reference rejected by Page.Contents")
	}
}

func TestSnapshotPreservesBoundaryRecoveryBeforeInlineImage(t *testing.T) {
	first := "/Span << /MCID "
	second := "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G ID x EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	if len(got) != 8 || got[0] != (testcompat.TokenRecord{Kind: "name", Value: "Span"}) || got[1] != (testcompat.TokenRecord{Kind: "name", Value: "MCID"}) || got[2] != (testcompat.TokenRecord{Kind: "dict_start"}) || got[3] != (testcompat.TokenRecord{Kind: "number", Value: float64(3)}) || got[4] != "None" || got[5] != (testcompat.TokenRecord{Kind: "keyword", Value: "BDC"}) || got[7] != (testcompat.TokenRecord{Kind: "keyword", Value: "Q"}) {
		t.Fatalf("recovered inline contents = %#v", got)
	}
	image, ok := got[6].(map[string]interface{})
	if !ok || image["kind"] != "stream" || image["length"] != 1 {
		t.Fatalf("inline image projection = %#v", got[6])
	}
}

func TestSnapshotPreservesInlineImageBeforeBoundaryRecovery(t *testing.T) {
	first := "BI /W 1 /H 1 /BPC 8 /CS /G ID x EI /Span << /MCID "
	second := "3 >> BDC"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	if len(got) != 7 || got[1] != (testcompat.TokenRecord{Kind: "name", Value: "Span"}) || got[2] != (testcompat.TokenRecord{Kind: "name", Value: "MCID"}) || got[3] != (testcompat.TokenRecord{Kind: "dict_start"}) || got[4] != (testcompat.TokenRecord{Kind: "number", Value: float64(3)}) || got[5] != "None" || got[6] != (testcompat.TokenRecord{Kind: "keyword", Value: "BDC"}) {
		t.Fatalf("inline then recovered contents = %#v", got)
	}
	image, ok := got[0].(map[string]interface{})
	if !ok || image["kind"] != "stream" || image["length"] != 1 {
		t.Fatalf("inline image projection = %#v", got[0])
	}
}

func TestSnapshotRejectsInlineImageInsideContainerLikePlaya(t *testing.T) {
	contents := "[ BI /W 1 /H 1 /BPC 8 /CS /G ID x EI ]"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	if _, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}}); err == nil {
		t.Fatal("inline image inside array succeeded, want pinned Playa syntax error")
	}
}

func TestSnapshotPreservesPlayaInlineImageRecoveryAfterBoundary(t *testing.T) {
	tests := []struct {
		name         string
		second       string
		third        string
		wantImageLen int
		wantTail     []interface{}
	}{
		{
			name:         "default skips one whitespace",
			second:       "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G ID  x EI Q",
			wantImageLen: 2,
			wantTail:     []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}},
		},
		{
			name:         "length is authoritative without EI",
			second:       "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G /Length 1 ID x ZZ Q",
			wantImageLen: 1,
			wantTail:     []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}},
		},
		{
			name:         "furthest EI salvage",
			second:       "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G ID xEI Q",
			wantImageLen: 1,
			wantTail:     []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}},
		},
		{
			name:     "missing EI leaves compound pending until final EOF",
			second:   "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G ID x Q",
			wantTail: []interface{}{},
		},
		{
			name:     "missing EI spills compound into next stream",
			second:   "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G ID x Q",
			third:    "R",
			wantTail: []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}, testcompat.TokenRecord{Kind: "keyword", Value: "x"}, testcompat.TokenRecord{Kind: "keyword", Value: "R"}},
		},
		{
			name:     "length beyond stream ends current stream",
			second:   "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G /Length 99 ID x",
			third:    "Q",
			wantTail: []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}},
		},
		{
			name:     "length marker unterminated string ends current stream",
			second:   "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G /Length 1 ID x (",
			third:    "Q",
			wantTail: []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first := "/Span << /MCID "
			contents := "[4 0 R 5 0 R]"
			objects := []string{
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
				"",
				fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
				fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(test.second), test.second),
			}
			if test.third != "" {
				contents = "[4 0 R 5 0 R 6 0 R]"
				objects = append(objects, fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(test.third), test.third))
			}
			objects[2] = fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents %s >>", contents)
			doc, err := openPagePDF(testPDF(objects...))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = doc.Close() }()
			snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
			if err != nil {
				t.Fatal(err)
			}
			got := snapshot.Pages[0].Contents
			prefixLen := 6
			if len(got) < prefixLen {
				t.Fatalf("contents = %#v, missing recovered prefix", got)
			}
			if test.wantImageLen > 0 {
				image, ok := got[prefixLen].(map[string]interface{})
				if !ok || image["kind"] != "stream" || image["length"] != test.wantImageLen {
					t.Fatalf("inline image = %#v, want length %d in %#v", got[prefixLen], test.wantImageLen, got)
				}
				prefixLen++
			}
			if tail := got[prefixLen:]; !reflect.DeepEqual(tail, test.wantTail) {
				t.Fatalf("inline recovery tail = %#v, want %#v; contents=%#v", tail, test.wantTail, got)
			}
		})
	}
}

func TestSnapshotPreservesPlayaInlineImageFilterAliasPriority(t *testing.T) {
	first := "/Span << /MCID "
	second := "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G /Filter /FlateDecode /F /AHx ID 00> EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	if len(got) != 8 {
		t.Fatalf("contents = %#v, want recovered prefix, image, Q", got)
	}
	image, ok := got[6].(map[string]interface{})
	if !ok || image["length"] != 1 || got[7] != (testcompat.TokenRecord{Kind: "keyword", Value: "Q"}) {
		t.Fatalf("filter alias contents = %#v, want /F default terminator and trailing Q", got)
	}
}

func TestSnapshotPreservesPlayaNegativeInlineImageLengthSeek(t *testing.T) {
	first := "/Span << /MCID "
	second := "3 >> BDC"
	third := "BI /W 1 /H 1 /BPC 8 /CS /G /Length -100 ID x EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R 6 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(third), third),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	if _, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}}); err == nil {
		t.Fatal("negative Length re-scan reached isolated ID without pinned Playa unmatched-BI error")
	}
}

func TestSnapshotPreservesPlayaInlineImageStopIterationWithoutPriorBoundary(t *testing.T) {
	tests := []struct {
		name  string
		first string
	}{
		{name: "Length beyond stream", first: "BI /W 1 /H 1 /BPC 8 /CS /G /Length 99 ID x"},
		{name: "unterminated marker token", first: "BI /W 1 /H 1 /BPC 8 /CS /G /Length 1 ID x ("},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			second := "Q"
			pdf := testPDF(
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
				"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
				fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(test.first), test.first),
				fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
			)
			doc, err := openPagePDF(pdf)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = doc.Close() }()
			snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
			if err != nil {
				t.Fatal(err)
			}
			want := []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}}
			if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
				t.Fatalf("StopIteration contents = %#v, want next stream only %#v", got, want)
			}
		})
	}
}

func TestSnapshotPreservesPlayaASCII85WordBoundary(t *testing.T) {
	first := "/Span << /MCID "
	second := "3 >> BDC BI /W 1 /H 1 /BPC 8 /CS /G /F /A85 ID z~>EIx EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	if len(got) != 8 || got[7] != (testcompat.TokenRecord{Kind: "keyword", Value: "Q"}) {
		t.Fatalf("ASCII85 EI word boundary contents = %#v", got)
	}
}

func TestSnapshotPreservesPlayaProcedureContainerRecoveryAcrossStreams(t *testing.T) {
	first := "/Tag { 1"
	second := "2 } BDC"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{
		testcompat.TokenRecord{Kind: "name", Value: "Tag"},
		testcompat.TokenRecord{Kind: "number", Value: float64(1)},
		testcompat.TokenRecord{Kind: "keyword", Value: "{"},
		testcompat.TokenRecord{Kind: "number", Value: float64(2)},
		"None",
		testcompat.TokenRecord{Kind: "keyword", Value: "BDC"},
	}
	if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
		t.Fatalf("procedure boundary contents = %#v, want %#v", got, want)
	}
}

func TestSnapshotPreservesPlayaBoundaryReferenceRecovery(t *testing.T) {
	tests := []struct {
		name   string
		first  string
		middle interface{}
	}{
		{name: "generation type is ignored", first: "/Prefix [1 /x R", middle: "<ObjRef:1>"},
		{name: "zero object becomes None", first: "/Prefix [0 /x R", middle: "None"},
		{name: "floating object becomes None", first: "/Prefix [1.0 /x R", middle: "None"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			second := "] Q"
			pdf := testPDF(
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
				"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
				fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(test.first), test.first),
				fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
			)
			doc, err := openPagePDF(pdf)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = doc.Close() }()
			snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
			if err != nil {
				t.Fatal(err)
			}
			want := []interface{}{
				testcompat.TokenRecord{Kind: "name", Value: "Prefix"},
				test.middle,
				testcompat.TokenRecord{Kind: "array_start"},
				"None",
				testcompat.TokenRecord{Kind: "keyword", Value: "Q"},
			}
			if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
				t.Fatalf("boundary reference contents = %#v, want %#v", got, want)
			}
		})
	}
}

func TestSnapshotRejectsPlayaBoundaryReferenceWithTooFewOperands(t *testing.T) {
	first := "/Prefix [R"
	second := "] Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	if _, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}}); err == nil {
		t.Fatal("boundary reference with too few operands succeeded, want pinned Playa stack error")
	}
}

func TestSnapshotPreservesNestedNoneDuringBoundaryRecovery(t *testing.T) {
	first := "[null] /T << /A {null} >> /Span << /MCID "
	second := "3 >> BDC"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	if len(got) < 3 || !reflect.DeepEqual(got[0], []interface{}{"None"}) || !reflect.DeepEqual(got[2], map[string]interface{}{"A": []interface{}{"None"}}) {
		t.Fatalf("nested None contents = %#v", got)
	}
}

func TestSnapshotPreservesPlayaInlineParameterObjectsAfterBoundary(t *testing.T) {
	tests := []struct {
		name       string
		second     string
		wantAttrs  map[string]interface{}
		wantLength int
		wantTail   []interface{}
	}{
		{
			name:       "boolean Length is integer one",
			second:     "3 >> BDC BI /L true ID X EI Q",
			wantAttrs:  map[string]interface{}{"L": testcompat.TokenRecord{Kind: "number", Value: 1}},
			wantLength: 1,
			wantTail:   []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}},
		},
		{
			name:       "odd parameter tail is discarded",
			second:     "3 >> BDC BI /A ID X EI Q",
			wantAttrs:  map[string]interface{}{},
			wantLength: 1,
			wantTail:   []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}},
		},
		{
			name:       "procedure parameter preserves None",
			second:     "3 >> BDC BI /A {1 null} /L 1 ID X EI Q",
			wantAttrs:  map[string]interface{}{"A": []interface{}{testcompat.TokenRecord{Kind: "number", Value: float64(1)}, "None"}, "L": testcompat.TokenRecord{Kind: "number", Value: float64(1)}},
			wantLength: 1,
			wantTail:   []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}},
		},
		{
			name:       "unterminated comment is marker token",
			second:     "3 >> BDC BI /L 1 ID X%tail",
			wantAttrs:  map[string]interface{}{"L": testcompat.TokenRecord{Kind: "number", Value: float64(1)}},
			wantLength: 1,
			wantTail:   []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "tail"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first := "/Span << /MCID "
			pdf := testPDF(
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
				"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
				fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
				fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(test.second), test.second),
			)
			doc, err := openPagePDF(pdf)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = doc.Close() }()
			snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
			if err != nil {
				t.Fatal(err)
			}
			got := snapshot.Pages[0].Contents
			if len(got) < 7 {
				t.Fatalf("contents = %#v, missing inline image", got)
			}
			image, ok := got[6].(map[string]interface{})
			if !ok || image["length"] != test.wantLength || !reflect.DeepEqual(image["attrs"], test.wantAttrs) {
				t.Fatalf("inline image = %#v, want length %d attrs %#v", got[6], test.wantLength, test.wantAttrs)
			}
			if tail := got[7:]; !reflect.DeepEqual(tail, test.wantTail) {
				t.Fatalf("inline tail = %#v, want %#v", tail, test.wantTail)
			}
		})
	}
}

func TestSnapshotRejectsFloatingPlayaInlineLengthAfterBoundary(t *testing.T) {
	first := "/Span << /MCID "
	second := "3 >> BDC BI /L 1.0 ID X EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	if _, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}}); err == nil {
		t.Fatal("floating Playa inline Length succeeded, want TypeError-compatible failure")
	}
}

func TestSnapshotUsesPlayaObjectProjectionWithoutBoundaryRecovery(t *testing.T) {
	contents := "[null] [1 0 R] << 1 null >> BI 1 null ID X EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	if len(got) != 5 || !reflect.DeepEqual(got[0], []interface{}{"None"}) || !reflect.DeepEqual(got[1], []interface{}{"<ObjRef:1>"}) || !reflect.DeepEqual(got[2], map[string]interface{}{}) {
		t.Fatalf("ordinary Playa contents = %#v", got)
	}
	image, ok := got[3].(map[string]interface{})
	if !ok || image["length"] != 1 || !reflect.DeepEqual(image["attrs"], map[string]interface{}{}) || got[4] != (testcompat.TokenRecord{Kind: "keyword", Value: "Q"}) {
		t.Fatalf("ordinary inline contents = %#v", got)
	}
}

func TestSnapshotPreservesPlayaInlineStackAcrossStreams(t *testing.T) {
	first := "BI /W 1"
	second := "Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{
		testcompat.TokenRecord{Kind: "number", Value: float64(1)},
		testcompat.TokenRecord{Kind: "name", Value: "W"},
		testcompat.TokenRecord{Kind: "keyword", Value: "BI"},
		testcompat.TokenRecord{Kind: "keyword", Value: "Q"},
	}
	if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
		t.Fatalf("inline stack contents = %#v, want %#v", got, want)
	}
}

func TestSnapshotRejectsPlayaInlineUnmatchedClosers(t *testing.T) {
	for _, closer := range []string{"]", ">>", "}"} {
		t.Run(closer, func(t *testing.T) {
			contents := "BI " + closer + " ID X EI"
			pdf := testPDF(
				"<< /Type /Catalog /Pages 2 0 R >>",
				"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
				"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
				fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
			)
			doc, err := openPagePDF(pdf)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = doc.Close() }()
			if _, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}}); err == nil {
				t.Fatalf("inline image with unmatched %q succeeded, want pinned Playa stack error", closer)
			}
		})
	}
}

func TestSnapshotPreservesPlayaBoundaryLexerTokens(t *testing.T) {
	contents := "1e3 12abc #foo \x00bar [999999999999999999999 0 R]"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{
		testcompat.TokenRecord{Kind: "number", Value: float64(1)},
		testcompat.TokenRecord{Kind: "keyword", Value: "e3"},
		testcompat.TokenRecord{Kind: "number", Value: float64(12)},
		testcompat.TokenRecord{Kind: "keyword", Value: "abc"},
		testcompat.TokenRecord{Kind: "keyword", Value: "#"},
		testcompat.TokenRecord{Kind: "keyword", Value: "foo"},
		testcompat.TokenRecord{Kind: "keyword", Value: "\x00"},
		testcompat.TokenRecord{Kind: "keyword", Value: "bar"},
		[]interface{}{"<ObjRef:999999999999999999999>"},
	}
	if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
		t.Fatalf("Playa lexer contents = %#v, want %#v", got, want)
	}
}

func TestSnapshotTreatsUnterminatedPlayaStringAsStreamEOF(t *testing.T) {
	first := "("
	second := "Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R 5 0 R] >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(first), first),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(second), second),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "Q"}}
	if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
		t.Fatalf("unterminated string contents = %#v, want %#v", got, want)
	}
}

func TestSnapshotRejectsOverflowingPlayaInlineLength(t *testing.T) {
	contents := "BI /L 999999999999999999999999999 ID X EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	if _, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}}); err == nil {
		t.Fatal("overflowing inline Length succeeded, want pinned Playa overflow-compatible failure")
	}
}

func TestSnapshotProjectsDecodedPlayaInlineImageLength(t *testing.T) {
	contents := "BI /F /AHx ID FF>EI BI /F /A85 ID z~>EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	if len(got) != 3 {
		t.Fatalf("inline contents = %#v, want two images and Q", got)
	}
	first, firstOK := got[0].(map[string]interface{})
	second, secondOK := got[1].(map[string]interface{})
	if !firstOK || first["length"] != 1 || !secondOK || second["length"] != 4 {
		t.Fatalf("decoded inline lengths = %#v/%#v, want 1/4", got[0], got[1])
	}
}

func TestSnapshotResolvesIndirectInlineImageFilterForDecodedLength(t *testing.T) {
	contents := "BI /F 7 0 R ID 41> EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
		"null",
		"null",
		"/ASCIIHexDecode",
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	if len(got) != 2 {
		t.Fatalf("indirect-filter contents = %#v", got)
	}
	image, ok := got[0].(map[string]interface{})
	if !ok || image["length"] != 1 || !reflect.DeepEqual(image["attrs"], map[string]interface{}{"F": "<ObjRef:7>"}) {
		t.Fatalf("indirect-filter inline image = %#v", got[0])
	}
}

func TestSnapshotResolvesLatestGenerationInlineImageFilter(t *testing.T) {
	contents := "BI /F 7 0 R ID 41> EI Q"
	pdf := testPDFWithGenerationObject(contents, 7, 2, "/ASCIIHexDecode")
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	image, ok := got[0].(map[string]interface{})
	if len(got) != 2 || !ok || image["length"] != 1 {
		t.Fatalf("latest-generation inline filter = %#v", got)
	}
}

func TestSnapshotIgnoresTopLevelReferenceMarkerInInlineFilterObject(t *testing.T) {
	contents := "BI /F 7 0 R ID 41> EI Q"
	pdf := testPDFWithGenerationObjects(contents, map[int]generationObject{
		7: {generation: 2, body: "8 1 R"},
		8: {generation: 1, body: "/ASCIIHexDecode"},
	})
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	image, ok := got[0].(map[string]interface{})
	if len(got) != 2 || !ok || image["length"] != 3 {
		t.Fatalf("top-level reference marker inline filter = %#v", got)
	}
}

func TestSnapshotNormalizesGenerationInsideIndirectInlineFilterArray(t *testing.T) {
	contents := "BI /F 7 0 R ID 41> EI Q"
	pdf := testPDFWithGenerationObjects(contents, map[int]generationObject{
		7: {generation: 2, body: "[8 99 R]"},
		8: {generation: 1, body: "/ASCIIHexDecode"},
	})
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	image, ok := got[0].(map[string]interface{})
	if len(got) != 2 || !ok || image["length"] != 1 {
		t.Fatalf("indirect inline filter array = %#v", got)
	}
}

func TestSnapshotFallsBackToRawInlineImageLengthOnDecodeError(t *testing.T) {
	contents := "BI /F /UnknownFilter ID raw EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	image, ok := got[0].(map[string]interface{})
	if len(got) != 2 || !ok || image["length"] != 3 {
		t.Fatalf("raw fallback inline length = %#v", got)
	}
}

func TestSnapshotResolvesIndirectInlineImageDecodeParmsForDecodedLength(t *testing.T) {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write([]byte{0, 'A', 'B'}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	contents := append([]byte("BI /F /FlateDecode /DP 7 0 R ID "), compressed.Bytes()...)
	contents = append(contents, []byte(" EI Q")...)
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
		"null",
		"null",
		"<< /Predictor 12 /Columns 2 /Colors 1 /BitsPerComponent 8 >>",
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	if len(got) != 2 {
		t.Fatalf("indirect DecodeParms contents = %#v", got)
	}
	image, ok := got[0].(map[string]interface{})
	wantAttrs := map[string]interface{}{
		"F":  testcompat.TokenRecord{Kind: "name", Value: "FlateDecode"},
		"DP": "<ObjRef:7>",
	}
	if !ok || image["length"] != 2 || !reflect.DeepEqual(image["attrs"], wantAttrs) {
		t.Fatalf("indirect DecodeParms inline image = %#v", got[0])
	}
}

func TestSnapshotPreservesPlayaNameStringHexAndKeywordBytes(t *testing.T) {
	contents := append([]byte("/#80 (a\rb) <41\f42> (a\\400b) a"), 0xff)
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{
		testcompat.TokenRecord{Kind: "name", Value: "\u0080"},
		testcompat.TokenRecord{Kind: "string", Value: "610a62"},
		testcompat.TokenRecord{Kind: "string", Value: "4142"},
		testcompat.TokenRecord{Kind: "string", Value: "6162"},
		testcompat.TokenRecord{Kind: "keyword", Value: "aÿ"},
	}
	if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
		t.Fatalf("Playa byte tokens = %#v, want %#v", got, want)
	}
}

func TestSnapshotUsesPlayaObjectProjectionForFormContents(t *testing.T) {
	formContents := "[null] BI 1 null ID X EI BI /A {1 null} /L 1 ID Y EI Q"
	pageContents := "/Fm Do"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /XObject << /Fm 4 0 R >> >> /Contents 5 0 R >>",
		fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Length %d >> stream\n%s\nendstream", len(formContents), formContents),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(pageContents), pageContents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.xobjects"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages[0].XObjects) != 1 {
		t.Fatalf("xobjects = %#v", snapshot.Pages[0].XObjects)
	}
	got := snapshot.Pages[0].XObjects[0].Contents
	if len(got) != 4 || !reflect.DeepEqual(got[0], []interface{}{"None"}) {
		t.Fatalf("Form Playa contents = %#v", got)
	}
	firstImage, firstOK := got[1].(map[string]interface{})
	secondImage, secondOK := got[2].(map[string]interface{})
	wantSecondAttrs := map[string]interface{}{
		"A": []interface{}{testcompat.TokenRecord{Kind: "number", Value: float64(1)}, "None"},
		"L": testcompat.TokenRecord{Kind: "number", Value: float64(1)},
	}
	if !firstOK || !reflect.DeepEqual(firstImage["attrs"], map[string]interface{}{}) || !secondOK || !reflect.DeepEqual(secondImage["attrs"], wantSecondAttrs) {
		t.Fatalf("Form inline contents = %#v", got)
	}
	if got[3] != (testcompat.TokenRecord{Kind: "keyword", Value: "Q"}) {
		t.Fatalf("Form trailing contents = %#v", got)
	}
}

func TestSnapshotRejectsIndirectReferencesInFormContentStreams(t *testing.T) {
	formContents := "/Span << /Self 7 0 R >> BDC"
	pageContents := "/Fm Do"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /XObject << /Fm 4 0 R >> >> /Contents 5 0 R >>",
		fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Length %d >> stream\n%s\nendstream", len(formContents), formContents),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(pageContents), pageContents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	if _, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.xobjects"}}); err == nil {
		t.Fatal("compatibility snapshot accepted an indirect reference in a Form XObject content stream")
	}
}

func TestSnapshotTreatsUnterminatedFormStringAsObjectParserEOF(t *testing.T) {
	formContents := "q ("
	pageContents := "/Fm Do"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /XObject << /Fm 4 0 R >> >> /Contents 5 0 R >>",
		fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Length %d >> stream\n%s\nendstream", len(formContents), formContents),
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(pageContents), pageContents),
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.xobjects"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages[0].XObjects) != 1 {
		t.Fatalf("xobjects = %#v", snapshot.Pages[0].XObjects)
	}
	want := []testcompat.TokenRecord{{Kind: "keyword", Value: "q"}}
	if got := snapshot.Pages[0].XObjects[0].Tokens; !reflect.DeepEqual(got, want) {
		t.Fatalf("Form Playa tokens = %#v, want %#v", got, want)
	}
	wantContents := []interface{}{testcompat.TokenRecord{Kind: "keyword", Value: "q"}}
	if got := snapshot.Pages[0].XObjects[0].Contents; !reflect.DeepEqual(got, wantContents) {
		t.Fatalf("Form Playa contents = %#v, want %#v", got, wantContents)
	}
}

func TestSnapshotResolvesIndirectPlayaInlineLength(t *testing.T) {
	tests := []struct {
		name       string
		objects    []string
		compressed string
		imageData  string
		wantError  bool
		wantLength int
	}{
		{name: "one level integer", objects: []string{"1"}, imageData: "X", wantLength: 1},
		{name: "top-level reference marker is ignored", objects: []string{"8 0 R", "1"}, imageData: "12345678", wantLength: 8},
		{name: "integral float remains float", objects: []string{"1.0"}, wantError: true},
		{name: "compressed integer", compressed: "1", imageData: "X", wantLength: 1},
		{name: "compressed integral float remains float", compressed: "1.0", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			imageData := test.imageData
			if imageData == "" {
				imageData = "X"
			}
			contents := "BI /L 7 0 R ID " + imageData + " EI Q"
			var pdf []byte
			if test.compressed != "" {
				pdf = testPDFWithCompressedLength(contents, test.compressed)
			} else {
				objects := []string{
					"<< /Type /Catalog /Pages 2 0 R >>",
					"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
					"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
					fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
					"null",
					"null",
				}
				objects = append(objects, test.objects...)
				pdf = testPDF(objects...)
			}
			doc, err := openPagePDF(pdf)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = doc.Close() }()
			snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
			if test.wantError {
				if err == nil {
					t.Fatalf("Snapshot() succeeded with floating indirect Length: %#v", snapshot.Pages[0].Contents)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := snapshot.Pages[0].Contents
			if len(got) != 2 {
				t.Fatalf("indirect Length contents = %#v", got)
			}
			image, ok := got[0].(map[string]interface{})
			if !ok || image["length"] != test.wantLength || !reflect.DeepEqual(image["attrs"], map[string]interface{}{"L": "<ObjRef:7>"}) || got[1] != (testcompat.TokenRecord{Kind: "keyword", Value: "Q"}) {
				t.Fatalf("indirect Length inline image = %#v", got)
			}
		})
	}
}

func TestSnapshotBoundsCompressedLengthAtNextObject(t *testing.T) {
	contents := "BI /L 7 0 R ID 12345678 EI Q"
	pdf := testPDFWithCompressedObjects(contents, []string{"8 0 R", "1"})
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	image, ok := got[0].(map[string]interface{})
	if len(got) != 2 || !ok || image["length"] != 8 {
		t.Fatalf("bounded compressed Length = %#v", got)
	}
}

func TestSnapshotIgnoresMalformedCompressedObjectOffsets(t *testing.T) {
	contents := "BI /L 7 0 R ID 12345678 EI Q"
	pdf := testPDFWithCompressedObjects(contents, []string{"8", "1"})
	pdf = bytes.Replace(pdf, []byte("7 0 8 2 8 1 "), []byte("7 9 8 0 8 1 "), 1)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	image, ok := got[0].(map[string]interface{})
	if len(got) != 2 || !ok || image["length"] != 8 {
		t.Fatalf("malformed-offset compressed Length = %#v", got)
	}
}

func TestSnapshotIgnoresCompressedObjectFirstAndHeaderIDs(t *testing.T) {
	contents := "BI /L 7 0 R ID 12345678 EI Q"
	pdf := testPDFWithCompressedObjectsAndHeader(contents, []string{"8", "1"}, "2", "0", nil)
	pdf = bytes.Replace(pdf, []byte("7 0 8 2 8 1 "), []byte("99 0 42 2 8 1 "), 1)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	image, ok := got[0].(map[string]interface{})
	if len(got) != 2 || !ok || image["length"] != 8 {
		t.Fatalf("malformed compressed header = %#v", got)
	}
}

func TestSnapshotUsesPinnedMalformedObjectStreamN(t *testing.T) {
	tests := []struct {
		name       string
		contents   string
		n          string
		wantLength int
	}{
		{name: "integral float becomes zero", contents: "BI /L 7 0 R ID 12345678 EI Q", n: "2.0", wantLength: 7},
		{name: "xref index may exceed N", contents: "BI /L 8 0 R ID 12345678 EI Q", n: "1", wantLength: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pdf := testPDFWithCompressedObjectsAndHeader(test.contents, []string{"8", "1"}, test.n, "0", nil)
			doc, err := openPagePDF(pdf)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = doc.Close() }()
			snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
			if err != nil {
				t.Fatal(err)
			}
			got := snapshot.Pages[0].Contents
			image, ok := got[0].(map[string]interface{})
			if len(got) == 0 || !ok || image["length"] != test.wantLength {
				t.Fatalf("malformed N contents = %#v, want length %d", got, test.wantLength)
			}
		})
	}
}

func TestPageProjectorInvalidatesSourceCacheAfterXRefFallback(t *testing.T) {
	pdf := testPDFWithStaleProjectorRefs()
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	projector := testcompat.NewPageProjector(doc)
	page0, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projector.PageSnapshotSections(page0, 0, []string{"content.contents"}); err == nil {
		t.Fatal("pre-fallback stale inline Length unexpectedly resolved")
	}
	page1, err := doc.PageAt(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projector.PageSnapshotSections(page1, 1, []string{"content.contents"}); err != nil {
		t.Fatal(err)
	}
	page2, err := doc.PageAt(2)
	if err != nil {
		t.Fatal(err)
	}
	third, err := projector.PageSnapshotSections(page2, 2, []string{"content.contents"})
	if err != nil {
		t.Fatal(err)
	}
	thirdImage, ok := third.Contents[0].(map[string]interface{})
	if !ok || thirdImage["length"] != 3 {
		t.Fatalf("post-fallback inline length = %#v, want recovered authoritative 3", third.Contents)
	}
}

func TestSnapshotIgnoresTopLevelReferenceMarkersInObjectStreamHeaderValues(t *testing.T) {
	contents := "BI /L 7 0 R ID 12345678 EI Q"
	pdf := testPDFWithCompressedObjectsAndHeader(contents, []string{"8", "1"}, "5 0 R", "6 0 R", map[int]string{
		5: "2 0 R",
		6: "8 0 R",
	})
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	image, ok := got[0].(map[string]interface{})
	if len(got) != 2 || !ok || image["length"] != 8 {
		t.Fatalf("indirect object-stream header values = %#v", got)
	}
}

func TestSnapshotSeeksDirectlyToIndirectLengthXRefOffset(t *testing.T) {
	contents := "BI /L 7 0 R ID X EI Q"
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents),
		"<< /Length 1 >> stream\n(\nendstream",
		"null",
		"1",
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	got := snapshot.Pages[0].Contents
	image, ok := got[0].(map[string]interface{})
	if len(got) != 2 || !ok || image["length"] != 1 {
		t.Fatalf("xref-seek indirect Length = %#v", got)
	}
}

func TestSnapshotSkipsNonStreamPageContentsEntries(t *testing.T) {
	pdf := testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents [4 0 R null 42 99 0 R 5 0 R] >>",
		"<< /Length 1 >> stream\nq\nendstream",
		"<< /Length 1 >> stream\nQ\nendstream",
	)
	doc, err := openPagePDF(pdf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.contents"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{
		testcompat.TokenRecord{Kind: "keyword", Value: "q"},
		testcompat.TokenRecord{Kind: "keyword", Value: "Q"},
	}
	if got := snapshot.Pages[0].Contents; !reflect.DeepEqual(got, want) {
		t.Fatalf("contents with non-stream item = %#v, want %#v", got, want)
	}
}

func TestTokenSnapshotNormalizesPDFBooleanKeywords(t *testing.T) {
	trueToken := testcompat.TokenSnapshot(core.NewToken(core.TokenKeyword, "true"))
	falseToken := testcompat.TokenSnapshot(core.NewToken(core.TokenKeyword, "false"))
	if trueToken.Kind != "number" || trueToken.Value != 1 || falseToken.Kind != "number" || falseToken.Value != 0 {
		t.Fatalf("boolean token projections = %#v/%#v", trueToken, falseToken)
	}
}

func TestContentOpSnapshotPreservesLexicalObjectOrder(t *testing.T) {
	ops, err := core.ParseContent([]byte("1 0 m /F1 12 Tf (A) Tj"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 3 {
		t.Fatalf("operations = %#v", ops)
	}
	values := make([]interface{}, 0)
	for _, op := range ops {
		values = append(values, testcompat.ContentOpSnapshot(op)...)
	}
	want := []interface{}{
		testcompat.TokenRecord{Kind: "number", Value: float64(1)},
		testcompat.TokenRecord{Kind: "number", Value: float64(0)},
		testcompat.TokenRecord{Kind: "keyword", Value: "m"},
		testcompat.TokenRecord{Kind: "name", Value: "F1"},
		testcompat.TokenRecord{Kind: "number", Value: float64(12)},
		testcompat.TokenRecord{Kind: "keyword", Value: "Tf"},
		testcompat.TokenRecord{Kind: "string", Value: "41"},
		testcompat.TokenRecord{Kind: "keyword", Value: "Tj"},
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("content projection = %#v, want %#v", values, want)
	}
}

func TestContentOpSnapshotProjectsInlineImagesAsBareStreams(t *testing.T) {
	ops, err := core.ParseContent([]byte("BI /W 1 /H 1 /BPC 8 /CS /G /F /AHx ID 4142> EI Q"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("operations = %#v", ops)
	}
	values := make([]interface{}, 0)
	for _, op := range ops {
		values = append(values, testcompat.ContentOpSnapshot(op)...)
	}
	want := []interface{}{
		map[string]interface{}{
			"kind":   "stream",
			"length": 2,
			"attrs": map[string]interface{}{
				"W":   testcompat.TokenRecord{Kind: "number", Value: float64(1)},
				"H":   testcompat.TokenRecord{Kind: "number", Value: float64(1)},
				"BPC": testcompat.TokenRecord{Kind: "number", Value: float64(8)},
				"CS":  testcompat.TokenRecord{Kind: "name", Value: "G"},
				"F":   testcompat.TokenRecord{Kind: "name", Value: "AHx"},
			},
		},
		testcompat.TokenRecord{Kind: "keyword", Value: "Q"},
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("inline-image projection = %#v, want %#v", values, want)
	}
}

func TestSnapshotProjectsTextAndGlyphGeometry(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openPagePDF(data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages) != 1 || len(snapshot.Pages[0].Text) == 0 || len(snapshot.Pages[0].Text[0].Glyphs) == 0 {
		t.Fatalf("text projection = %#v", snapshot.Pages)
	}
	text := snapshot.Pages[0].Text[0]
	glyph := text.Glyphs[0]
	if text.FontName != "ResearchCJK" || text.FontBase != "ResearchCJK" || text.TextFont != "ResearchCJK 18" || text.Size != 18 {
		t.Fatalf("text font projection = %#v", text)
	}
	if text.Matrix != [6]float64{18, 0, 0, 18, 72, 740} || text.TextMatrix != [6]float64{1, 0, 0, 1, 72, 740} || text.ScalingMatrix != [6]float64{18, 0, 0, 18, 0, 0} {
		t.Fatalf("text matrices = %#v", text)
	}
	if len(text.Args) != 1 || text.Args[0] != "00010002" {
		t.Fatalf("text args = %#v", text.Args)
	}
	if glyph.CID != 1 || glyph.FontName != "ResearchCJK" || glyph.Size != 18 || glyph.Matrix != [6]float64{18, 0, 0, 18, 72, 740} {
		t.Fatalf("glyph projection = %#v", glyph)
	}
	if len(snapshot.Pages[0].Fonts) != 1 || snapshot.Pages[0].Fonts[0].Leading != 0 || snapshot.Pages[0].Fonts[0].Matrix != [6]float64{0.001, 0, 0, 0.001, 0, 0} {
		t.Fatalf("font geometry projection = %#v", snapshot.Pages[0].Fonts)
	}
}

func TestSnapshotProjectsMixedLayoutItems(t *testing.T) {
	data, err := os.ReadFile(testfixture.Path(t, "acceptance_inline_image.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openPagePDF(data)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"layout"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages) != 1 || snapshot.Pages[0].Layout == nil {
		t.Fatalf("layout projection = %#v", snapshot.Pages)
	}
	items := snapshot.Pages[0].Layout.Items
	if len(items) != 3 {
		t.Fatalf("layout items = %#v, want one text box and two images", items)
	}
	if items[0].Kind != "text_box" || items[0].TextBox == nil {
		t.Fatalf("first layout item = %#v, want text box", items[0])
	}
	for index, item := range items[1:] {
		if item.Kind != "image" || !item.HasBBox {
			t.Fatalf("layout item %d = %#v, want image with bbox", index+1, item)
		}
	}
}

func TestSnapshotJSONLStreamsHeaderAndPages(t *testing.T) {
	doc, err := openPagePDF([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	var output bytes.Buffer
	if err := testcompat.SnapshotJSONL(doc, testcompat.Options{Pages: []int{0}}, &output); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var header struct {
		Kind string `json:"kind"`
	}
	if err := decoder.Decode(&header); err != nil || header.Kind != "header" {
		t.Fatalf("header = %#v, err = %v", header, err)
	}
	var page struct {
		Kind string `json:"kind"`
		Page struct {
			Index int `json:"index"`
		} `json:"page"`
	}
	if err := decoder.Decode(&page); err != nil || page.Kind != "page" || page.Page.Index != 0 {
		t.Fatalf("page = %#v, err = %v", page, err)
	}
}

func TestSnapshotProjectsPageAnnotations(t *testing.T) {
	doc, err := core.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Annots [4 0 R] >>\nendobj\n4 0 obj\n<< /Type /Annot /Subtype /Text /Rect [1 2 3 4] /Contents (Note) /NM (annotation-1) /M (D:20240102030405Z) >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"), core.WithCoordinateSpace(core.CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := doc.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages[0].Annotations) != 1 {
		t.Fatalf("annotations = %#v", snapshot.Pages[0].Annotations)
	}
	annotation := snapshot.Pages[0].Annotations[0]
	if annotation.Type != "Text" || annotation.Rect != [4]float64{1, 2, 3, 4} || annotation.BBox != [4]float64{1, 2, 3, 4} || annotation.PageIndex != 0 || annotation.Contents != "Note" || annotation.Name != "annotation-1" || annotation.Modified != "D:20240102030405Z" {
		t.Fatalf("annotation = %#v", annotation)
	}
	if annotation.Parent != nil {
		t.Fatalf("annotation parent = %#v, want nil", annotation.Parent)
	}
	if annotation.Properties["Subtype"] != "Text" || annotation.Properties["Contents"] != "Note" {
		t.Fatalf("annotation properties = %#v", annotation.Properties)
	}
}

func TestSnapshotAnnotationPropertiesPreserveIndirectReferences(t *testing.T) {
	doc, err := core.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Annots [4 0 R] >>\nendobj\n4 0 obj\n<< /Type /Annot /Subtype /Link /Rect [1 2 3 4] /Dest 3 0 R >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"), core.WithCoordinateSpace(core.CoordinateSpacePage))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"annotations"}})
	if err != nil {
		t.Fatal(err)
	}
	reference, ok := snapshot.Pages[0].Annotations[0].Properties["Dest"].(map[string]interface{})
	if !ok || reference["ref"] != 3 {
		t.Fatalf("destination reference = %#v, want ref 3", snapshot.Pages[0].Annotations[0].Properties["Dest"])
	}
}

func TestSnapshotAnnotationActionsResolveTargetPageIndicesAcrossPageTree(t *testing.T) {
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Pages /Parent 2 0 R /Kids [5 0 R] /Count 1 >>",
		"<< /Type /Pages /Parent 2 0 R /Kids [6 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 3 0 R /MediaBox [0 0 200 100] /Annots [7 0 R 8 0 R] >>",
		"<< /Type /Page /Parent 4 0 R /MediaBox [0 0 200 100] >>",
		"<< /Type /Annot /Subtype /Link /Rect [0 0 10 10] /A << /S /GoTo /D [6 0 R /Fit] >> >>",
		"<< /Type /Annot /Subtype /Text /Rect [20 20 30 30] /Contents (note) >>",
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	page, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := testcompat.PageSnapshotSections(doc, page, 0, []string{"annotations"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Annotations) != 2 {
		t.Fatalf("annotation count = %d, want 2", len(snapshot.Annotations))
	}
	action := snapshot.Annotations[0].Action
	if action == nil || action.Destination == nil || action.Destination.PageIndex != 1 {
		t.Fatalf("cross-branch destination = %#v, want page index 1", action)
	}
	if snapshot.Annotations[1].Action != nil {
		t.Fatalf("text annotation action = %#v, want nil", snapshot.Annotations[1].Action)
	}
}

func TestSnapshotResolvesNestedXObjectStructurePagesOutsideSelectedPages(t *testing.T) {
	content := []byte("/Fm0 Do")
	form := []byte("q Q")
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R /StructTreeRoot 8 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << /XObject << /Fm0 6 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] /StructParents 7 /Length %d >>\nstream\n%s\nendstream", len(form), form),
		"null",
		"<< /Type /StructTreeRoot /ParentTree 9 0 R >>",
		"<< /Nums [7 [10 0 R]] >>",
		"<< /Type /StructElem /S /Figure /Pg 3 0 R /K [11 0 R] >>",
		"<< /Type /StructElem /S /Span /P 10 0 R /Pg 4 0 R >>",
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	page, err := doc.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := testcompat.PageSnapshotSections(doc, page, 0, []string{"content.xobjects"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.XObjects) != 1 || len(snapshot.XObjects[0].Structure) != 1 || len(snapshot.XObjects[0].Structure[0].Children) != 1 {
		t.Fatalf("xobject structure = %#v, want one nested element", snapshot.XObjects)
	}
	if got := snapshot.XObjects[0].Structure[0].PageIndex; got != 0 {
		t.Fatalf("xobject structure root page = %d, want 0", got)
	}
	if got := snapshot.XObjects[0].Structure[0].Children[0].PageIndex; got != 1 {
		t.Fatalf("xobject structure child page = %d, want 1", got)
	}
}

func TestSnapshotProjectsContentTags(t *testing.T) {
	content := []byte("/P BMC /Span << /MCID 9 >> BDC /Point << /MCID 6 /ActualText (point) >> DP EMC EMC")
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := doc.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages[0].Tags) != 1 {
		t.Fatalf("tags = %#v", snapshot.Pages[0].Tags)
	}
	tag := snapshot.Pages[0].Tags[0]
	if tag.Tag != "Point" || !tag.HasMCID || tag.MCID != 6 || tag.ActualText != "point" || tag.MarkedTag != "Span" || len(tag.MarkedStack) != 2 {
		t.Fatalf("tag = %#v", tag)
	}
	if tag.Properties["MCID"] != float64(6) && tag.Properties["MCID"] != int64(6) && tag.Properties["MCID"] != int(6) {
		t.Fatalf("tag properties = %#v", tag.Properties)
	}
}

func TestSnapshotMarkedProjectionOmitsEmptyMCIDSections(t *testing.T) {
	content := []byte("/P << /MCID 1 >> BDC EMC /P << /MCID 3 >> BDC BT (x) Tj ET EMC")
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{
		Pages: []int{0}, Sections: []string{"content.marked"},
	})
	if err != nil {
		t.Fatal(err)
	}
	marked := snapshot.Pages[0].Marked
	if len(marked) != 1 || marked[0].MCID != 3 {
		t.Fatalf("marked projection = %#v, want only MCID 3", marked)
	}
}

func TestProjectMarkedStackNormalizesMissingMCID(t *testing.T) {
	content := []byte("/Artifact BMC BT /F1 12 Tf 10 50 Td (A) Tj ET EMC")
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"content.text"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages[0].Text) != 1 || len(snapshot.Pages[0].Text[0].MarkedStack) != 1 {
		t.Fatalf("text marked stack = %#v", snapshot.Pages[0].Text)
	}
	context := snapshot.Pages[0].Text[0].MarkedStack[0]
	if context.MCID != 0 || context.HasMCID {
		t.Fatalf("missing MCID projection = %#v, want mcid=0 and has_mcid=false", context)
	}
	text := snapshot.Pages[0].Text[0]
	if text.MCID != 0 || text.HasMCID {
		t.Fatalf("text missing MCID projection = %#v, want mcid=0 and has_mcid=false", text)
	}
	if text.FontSize != text.Size {
		t.Fatalf("text font size = %v, size = %v; Playa projects the device-space size", text.FontSize, text.Size)
	}
	if len(snapshot.Pages[0].Text[0].Glyphs) != 1 {
		t.Fatalf("glyphs = %#v", snapshot.Pages[0].Text[0].Glyphs)
	}
	glyph := snapshot.Pages[0].Text[0].Glyphs[0]
	if glyph.MCID != 0 || glyph.HasMCID {
		t.Fatalf("glyph missing MCID projection = %#v, want mcid=0 and has_mcid=false", glyph)
	}
	glyphContext := glyph.MarkedStack[0]
	if glyphContext.MCID != 0 || glyphContext.HasMCID {
		t.Fatalf("glyph missing MCID projection = %#v, want mcid=0 and has_mcid=false", glyphContext)
	}
}

func TestSnapshotProjectsContentPaths(t *testing.T) {
	content := []byte("0 0 m 10 0 l 10 10 l h f")
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := doc.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages[0].Paths) != 1 {
		t.Fatalf("paths = %#v", snapshot.Pages[0].Paths)
	}
	path := snapshot.Pages[0].Paths[0]
	if path.Stroke || !path.Fill || path.EvenOdd || path.BBox != [4]float64{0, 0, 10, 10} {
		t.Fatalf("path = %#v", path)
	}
	if len(path.RawSegments) != 4 || path.RawSegments[0].Operator != "m" || path.RawSegments[3].Operator != "h" {
		t.Fatalf("raw segments = %#v", path.RawSegments)
	}
	if path.RawSegments[3].Points == nil || path.Segments[3].Points == nil {
		t.Fatalf("empty path points must serialize as arrays: %#v", path)
	}
}

func TestSnapshotProjectsNavigationAndForms(t *testing.T) {
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R /Dests << /Start [3 0 R /Fit] >> /Outlines 4 0 R /AcroForm 7 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>",
		"<< /Type /Outlines /First 5 0 R /Last 5 0 R /Count 1 >>",
		"<< /Title (Chapter) /Dest [3 0 R /Fit] /Count 0 >>",
		"<< >>",
		"<< /Fields [8 0 R] /NeedAppearances true >>",
		"<< /T (person) /FT /Tx /V (Ada) /Kids [9 0 R] >>",
		"<< /Subtype /Widget /Rect [10 20 30 40] >>",
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := doc.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Destinations) != 1 || snapshot.Destinations[0].Name != "Start" || snapshot.Destinations[0].Destination.PageIndex != 0 || snapshot.Destinations[0].Destination.View != "Fit" {
		t.Fatalf("destinations = %#v", snapshot.Destinations)
	}
	if len(snapshot.Outline) != 1 || snapshot.Outline[0].Title != "Chapter" || snapshot.Outline[0].Destination.PageIndex != 0 || snapshot.Outline[0].HasCount != true {
		t.Fatalf("outline = %#v", snapshot.Outline)
	}
	if !snapshot.NeedAppearances || len(snapshot.Forms) != 1 || snapshot.Forms[0].FullName != "person" || snapshot.Forms[0].Value != "Ada" || len(snapshot.Forms[0].Kids) != 1 || !snapshot.Forms[0].Kids[0].HasRect {
		t.Fatalf("forms = need=%v fields=%#v", snapshot.NeedAppearances, snapshot.Forms)
	}
	if snapshot.Forms[0].Options == nil || snapshot.Forms[0].Selected == nil || snapshot.Forms[0].Kids[0].Options == nil || snapshot.Forms[0].Kids[0].Selected == nil {
		t.Fatalf("empty form collections must serialize as arrays: %#v", snapshot.Forms)
	}
}

func TestSnapshotProjectsOutlineActionModel(t *testing.T) {
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R /Outlines 4 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>",
		"<< /Type /Outlines /First 5 0 R /Last 5 0 R /Count 1 >>",
		"<< /Title (Chapter) /A << /S /URI /URI (https://outline.example) /Next << /S /Named /N /NextPage >> >> >>",
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"outline"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Outline) != 1 || snapshot.Outline[0].Action == nil {
		t.Fatalf("outline action = %#v, want an action projection", snapshot.Outline)
	}
	action := snapshot.Outline[0].Action
	if action.Kind != "URI" || action.URI != "https://outline.example" || len(action.Next) != 1 || action.Next[0].Kind != "Named" || action.Next[0].Name != "NextPage" {
		t.Fatalf("outline action = %#v", action)
	}
}

func TestSnapshotProjectsAnnotationActionModel(t *testing.T) {
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Annots [4 0 R] >>",
		"<< /Type /Annot /Subtype /Link /Rect [10 20 30 40] /A << /S /URI /URI (https://annotation.example) >> >>",
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: []int{0}, Sections: []string{"annotations"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Pages) != 1 || len(snapshot.Pages[0].Annotations) != 1 || snapshot.Pages[0].Annotations[0].Action == nil {
		t.Fatalf("annotation action = %#v, want an action projection", snapshot.Pages)
	}
	action := snapshot.Pages[0].Annotations[0].Action
	if action.Kind != "URI" || action.URI != "https://annotation.example" {
		t.Fatalf("annotation action = %#v", action)
	}
}

func TestSnapshotProjectsOpenActionModel(t *testing.T) {
	doc, err := openPagePDF(testPDF(
		"<< /Type /Catalog /Pages 2 0 R /OpenAction [3 0 R /Fit] >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>",
	))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Sections: []string{"document"}})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.OpenAction == nil || snapshot.OpenAction.Kind != "GoTo" || snapshot.OpenAction.Destination == nil || snapshot.OpenAction.Destination.PageIndex != 0 {
		t.Fatalf("open action = %#v, want a GoTo action targeting page 0", snapshot.OpenAction)
	}
	if len(snapshot.OpenAction.Raw) != 0 {
		t.Fatalf("direct open action raw = %#v, want an empty source dictionary", snapshot.OpenAction.Raw)
	}
}

func testPDF(objects ...string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(objects)+1)
	b.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

type generationObject struct {
	generation int
	body       string
}

func testPDFWithGenerationObject(contents string, objectNumber, generation int, body string) []byte {
	return testPDFWithGenerationObjects(contents, map[int]generationObject{objectNumber: {generation: generation, body: body}})
}

func testPDFWithGenerationObjects(contents string, extra map[int]generationObject) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	objectNumber := 4
	for number := range extra {
		if number > objectNumber {
			objectNumber = number
		}
	}
	offsets := make([]int, objectNumber+1)
	objects := map[int]generationObject{
		1: {body: "<< /Type /Catalog /Pages 2 0 R >>"},
		2: {body: "<< /Type /Pages /Kids [3 0 R] /Count 1 >>"},
		3: {body: "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>"},
		4: {body: fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents)},
	}
	for number, object := range extra {
		objects[number] = object
	}
	for number := 1; number <= objectNumber; number++ {
		object, ok := objects[number]
		if !ok {
			continue
		}
		offsets[number] = b.Len()
		fmt.Fprintf(&b, "%d %d obj\n%s\nendobj\n", number, object.generation, object.body)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", objectNumber+1)
	b.WriteString("0000000000 65535 f \n")
	for number := 1; number <= objectNumber; number++ {
		object, ok := objects[number]
		if !ok {
			b.WriteString("0000000000 00000 f \n")
			continue
		}
		fmt.Fprintf(&b, "%010d %05d n \n", offsets[number], object.generation)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%EOF\n", objectNumber+1, xref)
	return b.Bytes()
}

func testPDFWithCompressedLength(contents, lengthValue string) []byte {
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, 10)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>")
	appendObject(4, fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents))
	objectStreamData := "7 0 " + lengthValue
	appendObject(8, fmt.Sprintf("<< /Type /ObjStm /N 1 /First 4 /Length %d >> stream\n%s\nendstream", len(objectStreamData), objectStreamData))
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	xrefOffset := len(data)
	raw := record(0, 0, 65535)
	for number := 1; number <= 9; number++ {
		switch number {
		case 1, 2, 3, 4, 8:
			raw = append(raw, record(1, offsets[number], 0)...)
		case 7:
			raw = append(raw, record(2, 8, 0)...)
		case 9:
			raw = append(raw, record(1, xrefOffset, 0)...)
		default:
			raw = append(raw, record(0, 0, 0)...)
		}
	}
	data = append(data, fmt.Sprintf("9 0 obj\n<< /Type /XRef /Size 10 /Root 1 0 R /W [1 4 2] /Length %d >>\nstream\n", len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefOffset))...)
	return data
}

func testPDFWithStaleProjectorRefs() []byte {
	var data bytes.Buffer
	data.WriteString("%PDF-1.4\n")
	offsets := make([]int, 11)
	appendObject := func(number int, body string) {
		offsets[number] = data.Len()
		fmt.Fprintf(&data, "%d 0 obj\n%s\nendobj\n", number, body)
	}
	inline := "BI /L 7 0 R ID X EI Q"
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R 4 0 R 5 0 R] /Count 3 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Contents 6 0 R >>")
	appendObject(4, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Contents 9 0 R >>")
	appendObject(5, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Contents 10 0 R >>")
	appendObject(6, fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(inline), inline))
	appendObject(7, "3")
	appendObject(8, "null")
	appendObject(9, "<< /Length 1 >> stream\nq\nendstream")
	appendObject(10, fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(inline), inline))
	xref := data.Len()
	data.WriteString("xref\n0 11\n0000000000 65535 f \n")
	for number := 1; number < len(offsets); number++ {
		offset := offsets[number]
		if number == 7 || number == 9 {
			offset = offsets[8]
		}
		fmt.Fprintf(&data, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&data, "trailer\n<< /Size 11 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	return data.Bytes()
}

func testPDFWithCompressedObjects(contents string, values []string) []byte {
	return testPDFWithCompressedObjectsAndHeader(contents, values, strconv.Itoa(len(values)), "", nil)
}

func testPDFWithCompressedObjectsAndHeader(contents string, values []string, nValue, firstValue string, direct map[int]string) []byte {
	const objectStreamNumber = 10
	const xrefObjectNumber = 11
	data := []byte("%PDF-1.5\n")
	offsets := make([]int, xrefObjectNumber+1)
	appendObject := func(number int, body string) {
		offsets[number] = len(data)
		data = append(data, fmt.Sprintf("%d 0 obj\n%s\nendobj\n", number, body)...)
	}
	appendObject(1, "<< /Type /Catalog /Pages 2 0 R >>")
	appendObject(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	appendObject(3, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Contents 4 0 R >>")
	appendObject(4, fmt.Sprintf("<< /Length %d >> stream\n%s\nendstream", len(contents), contents))
	for number, value := range direct {
		appendObject(number, value)
	}
	var header strings.Builder
	var body strings.Builder
	for index, value := range values {
		fmt.Fprintf(&header, "%d %d ", 7+index, body.Len())
		body.WriteString(value)
		body.WriteByte(' ')
	}
	objectStreamData := header.String() + body.String()
	if firstValue == "" {
		firstValue = strconv.Itoa(header.Len())
	}
	appendObject(objectStreamNumber, fmt.Sprintf("<< /Type /ObjStm /N %s /First %s /Length %d >> stream\n%s\nendstream", nValue, firstValue, len(objectStreamData), objectStreamData))
	record := func(kind, value, generation int) []byte {
		return []byte{byte(kind), byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(generation >> 8), byte(generation)}
	}
	xrefOffset := len(data)
	raw := record(0, 0, 65535)
	for number := 1; number <= xrefObjectNumber; number++ {
		switch {
		case number == 1 || number == 2 || number == 3 || number == 4 || number == objectStreamNumber || direct[number] != "":
			raw = append(raw, record(1, offsets[number], 0)...)
		case number >= 7 && number < 7+len(values):
			raw = append(raw, record(2, objectStreamNumber, number-7)...)
		case number == xrefObjectNumber:
			raw = append(raw, record(1, xrefOffset, 0)...)
		default:
			raw = append(raw, record(0, 0, 0)...)
		}
	}
	data = append(data, fmt.Sprintf("%d 0 obj\n<< /Type /XRef /Size %d /Root 1 0 R /W [1 4 2] /Length %d >>\nstream\n", xrefObjectNumber, xrefObjectNumber+1, len(raw))...)
	data = append(data, raw...)
	data = append(data, []byte(fmt.Sprintf("\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", xrefOffset))...)
	return data
}
