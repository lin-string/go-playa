package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestReconcileCompatKnownDifferencesRequiresExactHashes(t *testing.T) {
	task := compareTask{
		pdf:           "fixture.pdf",
		fixtureSHA256: strings.Repeat("a", 64),
		space:         "page",
		sections:      []string{"content.paths"},
	}
	difference := "pages[0].paths[0].state.has_clip: playa=false, go=true"
	record := compatDifferenceRecord{
		ID:                "clipping-path",
		Fixture:           "fixture.pdf",
		FixtureSHA256:     task.fixtureSHA256,
		Spaces:            []string{"page"},
		Sections:          append([]string(nil), task.sections...),
		PathPattern:       `^pages\[[0-9]+\]\.paths\[[0-9]+\]\.state\.has_clip$`,
		DifferenceCount:   1,
		DifferencesSHA256: differenceSetDigest([]string{difference}),
		UpstreamIssue:     "clipping-path-state",
		Reason:            "The oracle does not expose clipping state.",
	}

	matched, err := reconcileCompatKnownDifferences(task, []string{difference}, []compatDifferenceRecord{record})
	if err != nil || len(matched) != 1 || matched[0] != record.ID {
		t.Fatalf("exact difference = %v, %v, want [%s], nil", matched, err, record.ID)
	}
	if _, err := reconcileCompatKnownDifferences(task, []string{difference + " changed"}, []compatDifferenceRecord{record}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("changed difference error = %v", err)
	}
	if _, err := reconcileCompatKnownDifferences(task, nil, []compatDifferenceRecord{record}); err == nil || !strings.Contains(err.Error(), "not observed") {
		t.Fatalf("stale difference error = %v", err)
	}
}

func TestReconcileCompatKnownDifferencesUsesExactScope(t *testing.T) {
	difference := "difference"
	base := compareTask{
		pdf:           "fixture.pdf",
		fixtureSHA256: strings.Repeat("a", 64),
		space:         "page",
		sections:      []string{"content.paths"},
	}
	record := compatDifferenceRecord{
		ID:                "known",
		Fixture:           "fixture.pdf",
		FixtureSHA256:     base.fixtureSHA256,
		Spaces:            []string{"page"},
		Sections:          append([]string(nil), base.sections...),
		PathPattern:       `^difference$`,
		DifferenceCount:   1,
		DifferencesSHA256: differenceSetDigest([]string{difference}),
		UpstreamIssue:     "clipping-path-state",
		Reason:            "The oracle does not expose clipping state.",
	}
	for _, mutate := range []func(*compareTask){
		func(task *compareTask) { task.fixtureSHA256 = strings.Repeat("b", 64) },
		func(task *compareTask) { task.space = "screen" },
		func(task *compareTask) { task.sections = []string{"content.interp"} },
	} {
		task := base
		mutate(&task)
		if _, err := reconcileCompatKnownDifferences(task, []string{difference}, []compatDifferenceRecord{record}); err == nil || !strings.Contains(err.Error(), "unrecorded") {
			t.Fatalf("wrong-scope task %#v error = %v", task, err)
		}
	}
}

func TestReconcileCompatKnownDifferencesAcceptsDeclaredAlternateSectionGroup(t *testing.T) {
	difference := "difference"
	record := compatDifferenceRecord{
		ID:                "known",
		Fixture:           "fixture.pdf",
		FixtureSHA256:     strings.Repeat("a", 64),
		Spaces:            []string{"page"},
		Sections:          []string{"document", "fonts"},
		AlternateSections: [][]string{{"fonts"}},
		PathPattern:       `^difference$`,
		DifferenceCount:   1,
		DifferencesSHA256: differenceSetDigest([]string{difference}),
		UpstreamIssue:     "document-fonts-nested-resources",
		Reason:            "The oracle omits nested fonts.",
	}
	task := compareTask{pdf: "fixture.pdf", fixtureSHA256: record.FixtureSHA256, space: "page", sections: []string{"fonts"}}
	matched, err := reconcileCompatKnownDifferences(task, []string{difference}, []compatDifferenceRecord{record})
	if err != nil || !reflect.DeepEqual(matched, []string{"known"}) {
		t.Fatalf("alternate section group = %v, %v, want [known], nil", matched, err)
	}
}

func TestParseCompatDifferenceManifestRejectsUnknownIssue(t *testing.T) {
	data := []byte(`{
  "schema_version": 1,
  "records": [{
    "id": "known",
    "fixture": "fixture.pdf",
    "fixture_sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "spaces": ["page"],
    "sections": ["content.paths"],
    "path_pattern": "^difference$",
    "difference_count": 1,
    "differences_sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
    "upstream_issue": "missing"
  }]
}`)
	if _, err := parseCompatDifferenceManifest(data); err == nil || !strings.Contains(err.Error(), "unknown upstream issue") {
		t.Fatalf("unknown issue error = %v", err)
	}
}

func TestLoadCompatDifferenceManifestValidatesRepositoryRegistry(t *testing.T) {
	manifest, err := loadCompatDifferenceManifest()
	if err != nil {
		t.Fatal(err)
	}
	path, err := repositoryFile("compat", "public_corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var public struct {
		Fixtures []struct {
			ID     string `json:"id"`
			SHA256 string `json:"sha256"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &public); err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]string, len(public.Fixtures)+2)
	for _, fixture := range public.Fixtures {
		sources[fixture.ID+".pdf"] = fixture.SHA256
	}
	for _, name := range []string{"acceptance_form_xobject.pdf", "acceptance_visual_semantics.pdf"} {
		path, err := repositoryFile("testdata", "files", name)
		if err != nil {
			t.Fatal(err)
		}
		digest, err := fileDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = digest
	}
	required := map[string]bool{
		"acceptance-form-xobject-nested-fonts": false,
		"acceptance-visual-clipping-state":     false,
	}
	for _, record := range manifest.Records {
		if _, ok := required[record.ID]; ok {
			required[record.ID] = true
		}
		digest, ok := sources[record.Fixture]
		if !ok {
			t.Fatalf("difference record %q uses nonpublic fixture %q", record.ID, record.Fixture)
		}
		if record.FixtureSHA256 != digest {
			t.Fatalf("difference record %q fixture digest = %s, want public source %s", record.ID, record.FixtureSHA256, digest)
		}
	}
	for id, found := range required {
		if !found {
			t.Fatalf("required public acceptance difference %q is missing", id)
		}
	}
}

func TestCompatDifferenceManifestAcceptsConfirmedCorpusIssues(t *testing.T) {
	data, err := os.ReadFile("../../compat/known_differences.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	record := raw["records"].([]any)[0].(map[string]any)
	record["upstream_issue"] = "public-corpus-issue"
	raw["records"] = []any{record}
	issue := map[string]any{
		"id": "public-corpus-issue", "difference_ids": []string{record["id"].(string)},
		"state": "submitted", "repository": "dhdaines/playa",
		"checked_commit": strings.Repeat("a", 40), "checked_at": "2026-09-28",
		"url": "https://github.com/dhdaines/playa/issues/245", "title": "Public corpus issue",
		"summary": "Independent source evidence", "reproduction": "A minimal in-memory reproduction",
		"actual_behavior": "The oracle differs", "expected_behavior": "The source-defined result",
	}
	raw["upstream_issues"] = []any{issue}
	encode := func() []byte {
		b, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if _, err := parseCompatDifferenceManifest(encode()); err != nil {
		t.Fatalf("confirmed corpus issue: %v", err)
	}
	for _, tt := range []struct {
		key   string
		value any
	}{
		{"state", "candidate"}, {"url", "https://github.com/other/project/issues/245"},
		{"difference_ids", []string{"missing-difference"}}, {"checked_at", "2026-99-99"},
		{"checked_commit", "not-a-commit"}, {"reproduction", ""},
	} {
		t.Run(tt.key, func(t *testing.T) {
			old := issue[tt.key]
			issue[tt.key] = tt.value
			defer func() { issue[tt.key] = old }()
			if _, err := parseCompatDifferenceManifest(encode()); err == nil {
				t.Fatalf("invalid %s accepted", tt.key)
			}
		})
	}
}
