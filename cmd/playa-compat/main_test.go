package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	core "github.com/lin-string/go-playa/document"
)

func TestRunHelp(t *testing.T) {
	for _, option := range []string{"-h", "-help", "--help"} {
		t.Run(option, func(t *testing.T) {
			var output strings.Builder
			if err := run([]string{option}, &output); err != nil {
				t.Fatalf("help failed: %v", err)
			}
			for _, want := range []string{"Usage of playa-compat:", "-compare", "-pdf"} {
				if !strings.Contains(output.String(), want) {
					t.Errorf("help %q does not contain %q", output.String(), want)
				}
			}
		})
	}
}

func TestRunUnknownFlagRemainsUsageError(t *testing.T) {
	var output strings.Builder
	err := run([]string{"--unknown-flag"}, &output)
	if !isUsageError(err) || output.Len() != 0 {
		t.Fatalf("error = %v, output = %q; want usage error and no stdout", err, output.String())
	}
}

func TestCompareJSONDecoderAvoidsPerTokenScannerErrors(t *testing.T) {
	const numberCount = 10_000
	input := "[" + strings.Repeat("0,", numberCount-1) + "0]"
	decoder := newCompareJSONDecoder(strings.NewReader(input))
	decoder.UseNumber()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start, err := decoder.Token()
	if err != nil || start != json.Delim('[') {
		t.Fatalf("array start = %v, err=%v", start, err)
	}
	count := 0
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			t.Fatal(err)
		}
		if token != json.Number("0") {
			t.Fatalf("token %d = %#v", count, token)
		}
		count++
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim(']') || count != numberCount {
		t.Fatalf("array end = %v, count=%d, err=%v", end, count, err)
	}
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 2<<20 {
		t.Fatalf("reading %d JSON tokens allocated %d bytes", numberCount, allocated)
	}
}

func TestCompareJSONDecoderMatchesStandardTokens(t *testing.T) {
	input := `{"emoji":"\uD83D\uDE00","invalid_pair":"\uD800\u0041","lone_low":"\uDE00","escaped":"a\\b\n","nested":[true,null,-1.2e3,{"text":"é"}]}`
	collect := func(decoder jsonTokenDecoder) []json.Token {
		decoder.UseNumber()
		var tokens []json.Token
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				return tokens
			}
			if err != nil {
				t.Fatal(err)
			}
			tokens = append(tokens, token)
		}
	}
	standard := json.NewDecoder(strings.NewReader(input))
	if got, want := collect(newCompareJSONDecoder(strings.NewReader(input))), collect(standard); !reflect.DeepEqual(got, want) {
		t.Fatalf("stream tokens = %#v, want %#v", got, want)
	}
}

func TestCompareJSONDecoderDisallowsUnknownFields(t *testing.T) {
	decoder := newCompareJSONDecoder(strings.NewReader(`{"known":1,"extra":2}`))
	decoder.DisallowUnknownFields()
	var target struct {
		Known int `json:"known"`
	}
	if err := decoder.Decode(&target); err == nil {
		t.Fatal("Decode accepted an unknown typed projection field")
	}
}

func TestRunRequiresPDF(t *testing.T) {
	var output bytes.Buffer
	if err := run(nil, &output); err == nil {
		t.Fatal("run() error = nil, want missing --pdf error")
	}
}

func TestReflectObjectMetadataIsCachedByType(t *testing.T) {
	type projection struct {
		Name   string `json:"name"`
		Empty  string `json:"empty,omitempty"`
		Zero   int    `json:"zero,omitempty"`
		Skip   string `json:"-"`
		hidden string
	}
	sample := projection{hidden: "not exported"}
	if sample.hidden == "" {
		t.Fatal("test fixture did not initialize its hidden field")
	}

	first := reflectObjectMetadata(reflect.TypeOf(projection{}))
	second := reflectObjectMetadata(reflect.TypeOf(projection{}))
	if first == nil || second == nil {
		t.Fatal("reflectObjectMetadata returned nil metadata")
	}
	if first != second {
		t.Fatal("reflectObjectMetadata rebuilt metadata for the same type")
	}
	if got := first.byName["name"]; len(got) != 1 || got[0] != 0 {
		t.Fatalf("name field indexes = %v, want [0]", got)
	}
	if got := first.byName["empty"]; len(got) != 1 || got[0] != 1 {
		t.Fatalf("empty field indexes = %v, want [1]", got)
	}
	if _, ok := first.byName["-"]; ok {
		t.Fatal("ignored JSON field was indexed")
	}
	if len(first.fields) != 3 {
		t.Fatalf("metadata fields = %d, want 3", len(first.fields))
	}
	if keys := objectKeys(reflect.ValueOf(projection{})); len(keys) != 1 || keys[0] != "name" {
		t.Fatalf("zero-valued omitempty fields = %v, want [name]", keys)
	}
}

func TestRunPrunesConfirmedOldCompatCacheEntries(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.jsonl")
	if err := os.WriteFile(old, []byte(`{"kind":"header","metadata":{"cache_version":18}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"--prune-cache", "--cache-dir", dir}, &output); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old cache exists after prune: %v", err)
	}
	if !strings.Contains(output.String(), "removed 1") {
		t.Fatalf("prune output = %q", output.String())
	}
}

func TestParseCoordinateSpace(t *testing.T) {
	for _, value := range []string{"page", "screen", "default", "user"} {
		if _, err := parseCoordinateSpace(value); err != nil {
			t.Fatalf("parseCoordinateSpace(%q): %v", value, err)
		}
	}
	if _, err := parseCoordinateSpace("invalid"); err == nil {
		t.Fatal("parseCoordinateSpace accepted invalid space")
	}
}

func TestSequentialPageIndices(t *testing.T) {
	for _, test := range []struct {
		name    string
		indices []int
		want    bool
	}{
		{name: "all pages", want: true},
		{name: "ascending", indices: []int{0, 2, 2, 5}, want: true},
		{name: "descending", indices: []int{0, 3, 2}, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := sequentialPageIndices(test.indices); got != test.want {
				t.Fatalf("sequentialPageIndices(%v) = %v, want %v", test.indices, got, test.want)
			}
		})
	}
}

func TestCompareTokenRecordUsesDirectTokenProjection(t *testing.T) {
	var record expectedTokenRecord
	record.Kind = "token"
	record.Token.Kind = "name"
	record.Token.Value = json.RawMessage(`"FontA"`)

	differences := []string{}
	compareTokenRecord(record, core.NewToken(core.TokenName, "FontA"), 3, 0, &differences)
	if len(differences) != 0 {
		t.Fatalf("matching token record produced differences: %v", differences)
	}

	record.Token.Value = json.RawMessage(`"Other"`)
	compareTokenRecord(record, core.NewToken(core.TokenName, "FontA"), 3, 0, &differences)
	if len(differences) == 0 {
		t.Fatal("mismatched token record produced no differences")
	}
}

func TestRunRejectsUserSpaceForCompare(t *testing.T) {
	var output bytes.Buffer
	err := run([]string{"--compare", "--space", "user", "--pdf", "missing.pdf"}, &output)
	if err == nil || !strings.Contains(err.Error(), "does not support coordinate space") {
		t.Fatalf("compare user space error = %v", err)
	}
	if !isUsageError(err) {
		t.Fatalf("compare user space error type = %T, want usageError", err)
	}
}

func TestRunRejectsMultipleSpacesWithoutCompare(t *testing.T) {
	var output bytes.Buffer
	err := run([]string{"--space", "page", "--space", "screen", "--pdf", "missing.pdf"}, &output)
	if err == nil || !isUsageError(err) || !strings.Contains(err.Error(), "may be repeated only with --compare") {
		t.Fatalf("multiple snapshot spaces error = %v, want comparison-only usage error", err)
	}
}

func TestSelectSections(t *testing.T) {
	sections, err := selectSections(nil)
	if err != nil || len(sections) != len(requiredSections) {
		t.Fatalf("selectSections(nil) = %v, %v", sections, err)
	}
	if _, err := selectSections([]string{"missing"}); err == nil {
		t.Fatal("selectSections accepted an unknown section")
	}
}

func TestPageFieldSelectionCoversEveryProjectedPageDomain(t *testing.T) {
	sections := map[string]bool{
		"pages":                         true,
		"content.text":                  true,
		"content.extract_text":          true,
		"content.extract_text.tagged":   true,
		"content.extract_text.untagged": true,
		"content.glyphs":                true,
		"content.flatten":               true,
		"content.interp":                true,
		"content.streams":               true,
		"content.tokens":                true,
		"content.xobjects":              true,
		"content.contents":              true,
		"layout":                        true,
		"content.paths":                 true,
		"content.images":                true,
		"fonts":                         true,
		"content.tags":                  true,
		"content.marked":                true,
		"annotations":                   true,
	}
	fields := []string{
		"index", "label", "width", "height", "rotation", "parent_key",
		"text", "extract_text", "extract_text_tagged", "extract_text_untagged",
		"glyphs", "flatten", "interp", "streams", "tokens", "xobjects",
		"contents", "layout", "paths", "images", "fonts", "tags", "marked",
		"annotations",
	}
	for _, field := range fields {
		if !pageFieldSelected(field, sections) {
			t.Errorf("pageFieldSelected(%q) = false, want true", field)
		}
		if field != "index" && pageSectionForField(field) == "pages" && field != "label" && field != "width" && field != "height" && field != "rotation" && field != "parent_key" {
			t.Errorf("pageSectionForField(%q) = pages, want a page domain section", field)
		}
	}
	if pageFieldSelected("unknown", sections) {
		t.Fatal("pageFieldSelected accepted an unknown page field")
	}
}

func TestFontsSectionSelectsDocumentFontMetadata(t *testing.T) {
	fields := headerFieldsForSections([]string{"fonts"})
	if !fields["document_fonts"] {
		t.Fatal("fonts section does not retain document_fonts from the Playa snapshot header")
	}
	metadata := comparisonMetadataSections([]string{"fonts", "pages", "content.contents"})
	if len(metadata) != 1 || metadata[0] != "fonts" {
		t.Fatalf("metadata sections = %v, want [fonts]", metadata)
	}
}

func TestCompatibilitySectionGroupsPartitionLargeDefaultJobs(t *testing.T) {
	pdf := filepath.Join(t.TempDir(), "large.pdf")
	if err := os.WriteFile(pdf, make([]byte, 9<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	groups := compatibilitySectionGroups(pdf, requiredSections)
	if len(groups) < 2 {
		t.Fatalf("large default compatibility job groups = %#v, want partitioned groups", groups)
	}
	seen := make(map[string]int, len(requiredSections))
	for _, group := range groups {
		if len(group) == 0 {
			t.Fatal("compatibility section group is empty")
		}
		for _, section := range group {
			seen[section]++
		}
	}
	if len(seen) != len(requiredSections) {
		t.Fatalf("partitioned sections = %#v, want all required sections", seen)
	}
	for _, section := range requiredSections {
		if seen[section] != 1 {
			t.Fatalf("section %q appears %d times", section, seen[section])
		}
	}
}

func TestCompatibilitySectionGroupsIsolateLargePageProjections(t *testing.T) {
	pdf := filepath.Join(t.TempDir(), "large.pdf")
	if err := os.WriteFile(pdf, make([]byte, 9<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, group := range compatibilitySectionGroups(pdf, requiredSections) {
		if containsSection(group, "layout") && (containsSection(group, "content.text") || containsSection(group, "content.glyphs")) {
			t.Fatalf("large compatibility layout group retains text/glyph projections: %#v", group)
		}
		if containsSection(group, "content.glyphs") && containsSection(group, "content.text") {
			t.Fatalf("large compatibility glyph group retains text projection: %#v", group)
		}
	}
	for _, section := range []string{"content.flatten", "content.interp", "content.xobjects", "content.paths", "content.images"} {
		for _, group := range compatibilitySectionGroups(pdf, requiredSections) {
			if containsSection(group, section) && len(group) != 1 {
				t.Fatalf("large compatibility section %q shares a page projection group: %#v", section, group)
			}
		}
	}
}

func containsSection(sections []string, wanted string) bool {
	for _, section := range sections {
		if section == wanted {
			return true
		}
	}
	return false
}

func TestCompatibilitySectionGroupsKeepSmallJobsSinglePassAndPartitionLargeFocusedJobs(t *testing.T) {
	directory := t.TempDir()
	smallPDF := filepath.Join(directory, "small.pdf")
	if err := os.WriteFile(smallPDF, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if groups := compatibilitySectionGroups(smallPDF, requiredSections); len(groups) != 1 {
		t.Fatalf("small compatibility job groups = %#v, want one group", groups)
	}
	if groups := compatibilitySectionGroups(smallPDF, []string{"content.flatten", "content.interp", "content.streams"}); len(groups) != 1 {
		t.Fatalf("small focused compatibility job groups = %#v, want one group", groups)
	}

	largePDF := filepath.Join(directory, "large.pdf")
	if err := os.WriteFile(largePDF, make([]byte, 9<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	groups := compatibilitySectionGroups(largePDF, []string{"content.flatten", "content.interp", "content.streams"})
	if len(groups) != 3 {
		t.Fatalf("large focused compatibility job groups = %#v, want three isolated groups", groups)
	}
}

func TestCompareJSONUsesFloatToleranceAndReportsPaths(t *testing.T) {
	expected, err := decodeJSON([]byte(`{"value":1.0,"items":[{"text":"A"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := decodeJSON([]byte(`{"value":1.0000005,"items":[{"text":"B"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	differences := []string{}
	compareJSON(expected, actual, "", 1e-6, &differences)
	if len(differences) != 1 || !strings.Contains(differences[0], "items[0].text") {
		t.Fatalf("differences = %v", differences)
	}
}

func TestCompareJSONDoesNotHideDifferencesAfterLegacyLimit(t *testing.T) {
	expected := make([]any, 40)
	actual := make([]int, 40)
	for index := range expected {
		expected[index] = float64(index)
		actual[index] = index + 1
	}
	var differences []string
	compareJSON(expected, actual, "values", 1e-6, &differences)
	if len(differences) != 40 {
		t.Fatalf("differences = %d, want all 40", len(differences))
	}
}

func TestCompareJSONReportsObjectDifferencesInStableKeyOrder(t *testing.T) {
	expected := map[string]any{
		"zeta":  1.0,
		"alpha": 2.0,
		"gamma": 3.0,
		"beta":  4.0,
	}
	actual := map[string]any{
		"zeta":  0.0,
		"alpha": 0.0,
		"gamma": 0.0,
		"beta":  0.0,
	}
	want := []string{
		"object.alpha: playa=2, go=0",
		"object.beta: playa=4, go=0",
		"object.gamma: playa=3, go=0",
		"object.zeta: playa=1, go=0",
	}
	for pass := 0; pass < 20; pass++ {
		var differences []string
		compareJSON(expected, actual, "object", 0, &differences)
		if !reflect.DeepEqual(differences, want) {
			t.Fatalf("pass %d differences = %v, want %v", pass, differences, want)
		}
	}
}

func TestCompareJSONStreamReportsUnorderedObjectDifferencesInStableKeyOrder(t *testing.T) {
	want := []string{
		"object.properties.alpha: playa=2, go=0",
		"object.properties.beta: playa=4, go=0",
		"object.properties.gamma: playa=3, go=0",
		"object.properties.zeta: playa=1, go=0",
	}
	for pass := 0; pass < 20; pass++ {
		expected := json.NewDecoder(strings.NewReader(`{"zeta":1,"alpha":2,"gamma":3,"beta":4}`))
		actual := json.NewDecoder(strings.NewReader(`{"zeta":0,"alpha":0,"gamma":0,"beta":0}`))
		expected.UseNumber()
		actual.UseNumber()
		var differences []string
		if err := compareJSONStreamValue(expected, actual, "object.properties", 0, &differences); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(differences, want) {
			t.Fatalf("pass %d differences = %v, want %v", pass, differences, want)
		}
	}
}

func TestCompareJSONStreamTreatsNestedPDFDictionariesAsUnordered(t *testing.T) {
	expected := json.NewDecoder(strings.NewReader(`{"stream":{"dict":{"Resources":{"ExtGState":{"GS11":{"ref":11}},"Font":{"F10":{"ref":1288}}},"Type":"XObject"}}}`))
	actual := json.NewDecoder(strings.NewReader(`{"stream":{"dict":{"Type":"XObject","Resources":{"Font":{"F10":{"ref":1288}},"ExtGState":{"GS11":{"ref":11}}}}}}`))
	expected.UseNumber()
	actual.UseNumber()
	differences := []string{}
	if err := compareJSONStreamValue(expected, actual, "objects[2056].stream.dict", 1e-6, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("nested PDF dictionary differences = %v", differences)
	}
}

func TestCompareObjectRecordSetsIgnoresEnumerationOrder(t *testing.T) {
	expected := map[objectRecordKey]json.RawMessage{
		{Object: 1}: json.RawMessage(`{"kind":"object","object":{"object":1,"generation":0,"value":{"A":1,"B":2}}}`),
		{Object: 2}: json.RawMessage(`{"kind":"object","object":{"object":2,"generation":0,"value":{"C":3}}}`),
	}
	actual := map[objectRecordKey]json.RawMessage{
		{Object: 2}: json.RawMessage(`{"kind":"object","object":{"object":2,"generation":0,"value":{"C":3}}}`),
		{Object: 1}: json.RawMessage(`{"kind":"object","object":{"object":1,"generation":0,"value":{"B":2,"A":1}}}`),
	}
	differences := []string{}
	if err := compareObjectRecordSets(expected, actual, 1e-6, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("object record differences = %v", differences)
	}
}

func TestCompareJSONAcceptsTypedProjectionWithoutMarshalling(t *testing.T) {
	expected, err := decodeJSON([]byte(`{"name":"A","width":10.0000005,"items":[{"value":2}]}`))
	if err != nil {
		t.Fatal(err)
	}
	type item struct {
		Value int `json:"value"`
	}
	type projection struct {
		Name  string  `json:"name"`
		Width float64 `json:"width"`
		Items []item  `json:"items"`
	}
	differences := []string{}
	compareJSON(expected, projection{Name: "A", Width: 10, Items: []item{{Value: 2}}}, "", 1e-6, &differences)
	if len(differences) != 0 {
		t.Fatalf("typed projection differences = %v", differences)
	}
}

func TestCompareJSONTreatsTypedNilMapAsNull(t *testing.T) {
	expected, err := decodeJSON([]byte(`{"resources":null}`))
	if err != nil {
		t.Fatal(err)
	}
	type projection struct {
		Resources map[string]interface{} `json:"resources"`
	}
	differences := []string{}
	compareJSON(expected, projection{}, "", 1e-6, &differences)
	if len(differences) != 0 {
		t.Fatalf("typed nil map differences = %v", differences)
	}
}

func TestReadSnapshotHeaderStreamsStructure(t *testing.T) {
	input := `{"kind":"header","snapshot":{"schema_version":"go-playa.compat/v1","structure":[{"type":"P"},{"type":"Span"}],"page_count":2}}
{"kind":"page","page":{"index":0}}
	`
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.UseNumber()
	snapshot, structure, err := readSnapshotHeader(decoder, true, headerFieldsForSections([]string{"structure"}))
	if err != nil {
		t.Fatal(err)
	}
	if structure == nil {
		t.Fatal("structure file = nil")
	}
	streamedStructure := structure
	defer func() {
		_ = streamedStructure.Close()
		_ = os.Remove(streamedStructure.Name())
	}()
	if _, ok := snapshot["structure"]; ok {
		t.Fatal("snapshot retained streamed structure")
	}
	if _, err := streamedStructure.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	differences := []string{}
	if err := compareStructureStream(streamedStructure, strings.NewReader(`[{"type":"P"},{"type":"Span"}]`), 0, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("differences = %v", differences)
	}
	decoder = json.NewDecoder(strings.NewReader(input))
	decoder.UseNumber()
	snapshot, structure, err = readSnapshotHeader(decoder, false, headerFieldsForSections([]string{"document"}))
	if err != nil {
		t.Fatal(err)
	}
	if structure != nil {
		_ = structure.Close()
		t.Fatal("unselected structure field allocated a file")
	}
	if _, ok := snapshot["structure"]; ok {
		t.Fatal("unselected structure field retained in snapshot")
	}
}

func TestCompareStructureStreamReportsNestedDifferences(t *testing.T) {
	differences := []string{}
	err := compareStructureStream(
		strings.NewReader(`[{"children":[{"type":"P"}]}]`),
		strings.NewReader(`[{"children":[{"type":"Span"}]}]`),
		0,
		&differences,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(differences) != 1 || !strings.Contains(differences[0], "structure[0].children[0].type") {
		t.Fatalf("differences = %v", differences)
	}
}

func TestCompareTextStreamAcceptsEmptyPageText(t *testing.T) {
	document, err := core.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := document.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = document.Close() }()
	differences := []string{}
	decoder := json.NewDecoder(strings.NewReader(`[]`))
	if err := compareTextStream(decoder, document, page, "pages[0].text", 0, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("text differences = %v", differences)
	}
}

func TestCompareTypedJSONStreamReflectUsesFloatTolerance(t *testing.T) {
	type value struct {
		Number     float64        `json:"number"`
		Properties map[string]any `json:"properties"`
	}
	decoder := json.NewDecoder(strings.NewReader(`[{"number":1.0000005,"properties":{"MCID":1.0}}]`))
	decoder.UseNumber()
	differences := []string{}
	if err := compareTypedJSONStreamReflect(decoder, reflect.ValueOf([]value{{Number: 1, Properties: map[string]any{"MCID": 1}}}), "values", 1e-6, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("differences = %v", differences)
	}
}

func TestCompareTypedJSONStreamReflectDistinguishesNullAndEmpty(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   any
	}{
		{name: "slice", expected: `null`, actual: []string{}},
		{name: "map", expected: `null`, actual: map[string]string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decoder := json.NewDecoder(strings.NewReader(test.expected))
			differences := []string{}
			if err := compareTypedJSONStreamReflect(decoder, reflect.ValueOf(test.actual), "value", 0, &differences); err != nil {
				t.Fatal(err)
			}
			if len(differences) != 1 {
				t.Fatalf("differences = %v, want one null/empty difference", differences)
			}
		})
	}
}

func TestCompareTypedJSONArrayStreamReflectClearsReusedDecodeValue(t *testing.T) {
	type nested struct {
		Values []int          `json:"values,omitempty"`
		Labels map[string]int `json:"labels,omitempty"`
	}
	type value struct {
		Number int      `json:"number,omitempty"`
		Nested []nested `json:"nested,omitempty"`
	}
	actual := []value{
		{Number: 1, Nested: []nested{{Values: []int{2}, Labels: map[string]int{"first": 3}}}},
		{},
	}
	decoder := json.NewDecoder(strings.NewReader(`[
		{"number":1,"nested":[{"values":[2],"labels":{"first":3}}]},
		{}
	]`))
	var differences []string
	if err := compareTypedJSONStreamReflect(decoder, reflect.ValueOf(actual), "values", 0, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("differences = %v, want none", differences)
	}
}

func TestCompareRawJSONReflectPreservesTypedDecodeSemantics(t *testing.T) {
	type value struct {
		Count int            `json:"count"`
		Text  string         `json:"text"`
		Items []int          `json:"items"`
		Meta  map[string]any `json:"meta"`
	}
	tests := []struct {
		name      string
		expected  string
		actual    value
		tolerance float64
		different bool
		wantErr   bool
	}{
		{
			name:      "escaped string and numeric tolerance",
			expected:  `{"COUNT":2,"text":"line\n\u4e2d","items":[1],"meta":{"value":1.0000005}}`,
			actual:    value{Count: 2, Text: "line\n中", Items: []int{1}, Meta: map[string]any{"value": 1.0}},
			tolerance: 1e-6,
		},
		{name: "null decodes to zero struct", expected: `null`},
		{name: "null differs from nonzero struct", expected: `null`, actual: value{Count: 1}, different: true},
		{name: "nil differs from empty", expected: `{"count":0,"text":"","items":[],"meta":null}`, actual: value{}, different: true},
		{name: "fraction cannot decode into integer", expected: `{"count":1.5}`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, different, err := compareRawJSONReflect([]byte(test.expected), reflect.ValueOf(test.actual), test.tolerance)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, test.wantErr)
			}
			if err == nil && different != test.different {
				t.Fatalf("different = %v, want %v", different, test.different)
			}
		})
	}
}

func TestCompareTypedJSONStreamReflectMatchesDynamicJSONObjectToStruct(t *testing.T) {
	type token struct {
		Kind  string `json:"kind"`
		Value any    `json:"value,omitempty"`
	}
	decoder := json.NewDecoder(strings.NewReader(`[{"kind":"number","value":1.0}]`))
	decoder.UseNumber()
	differences := []string{}
	actual := []any{token{Kind: "number", Value: 1}}
	if err := compareTypedJSONStreamReflect(decoder, reflect.ValueOf(actual), "values", 0, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("differences = %v", differences)
	}
}

func TestCompareImageStreamAcceptsEmptyPageImages(t *testing.T) {
	document, err := core.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := document.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = document.Close() }()
	differences := []string{}
	decoder := json.NewDecoder(strings.NewReader(`[]`))
	if err := compareImageStream(decoder, document, page, "pages[0].images", 0, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("image differences = %v", differences)
	}
}

func TestCompareFontStreamAcceptsEmptyPageFonts(t *testing.T) {
	document, err := core.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := document.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = document.Close() }()
	differences := []string{}
	decoder := json.NewDecoder(strings.NewReader(`[]`))
	if err := compareFontStream(decoder, document, page, "pages[0].fonts", 0, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("font differences = %v", differences)
	}
}

func TestCompareTagStreamAcceptsEmptyPageTags(t *testing.T) {
	document, err := core.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := document.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = document.Close() }()
	differences := []string{}
	decoder := json.NewDecoder(strings.NewReader(`[]`))
	if err := compareTagStream(decoder, document, page, "pages[0].tags", 0, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("tag differences = %v", differences)
	}
}

func TestCompareAnnotationStreamAcceptsEmptyPageAnnotations(t *testing.T) {
	document, err := core.OpenBytes([]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := document.PageAt(0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = document.Close() }()
	differences := []string{}
	decoder := json.NewDecoder(strings.NewReader(`[]`))
	if err := compareAnnotationStream(decoder, document, page, "pages[0].annotations", 0, &differences); err != nil {
		t.Fatal(err)
	}
	if len(differences) != 0 {
		t.Fatalf("annotation differences = %v", differences)
	}
}

func TestRunJSONLWritesHeapProfile(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "minimal.pdf")
	data := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n")
	if err := os.WriteFile(pdf, data, 0o600); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(dir, "heap.pprof")
	var output bytes.Buffer
	if err := run([]string{"--pdf", pdf, "--jsonl", "--memprofile", profile}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"kind":"header"`) || !strings.Contains(output.String(), `"kind":"page"`) {
		t.Fatalf("JSONL output = %s", output.String())
	}
	if info, err := os.Stat(profile); err != nil || info.Size() == 0 {
		t.Fatalf("heap profile = %v, err = %v", info, err)
	}
}
