package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestReconcilePDFAKnownDifferencesRequiresExactObservedLines(t *testing.T) {
	task := compareTask{
		fixtureID:     "fixture",
		fixtureSHA256: strings.Repeat("a", 64),
		space:         "page",
		sections:      []string{"document", "pages"},
	}
	record := testfixture.PDFADifference{
		ID:            "known",
		FixtureID:     task.fixtureID,
		FixtureSHA256: task.fixtureSHA256,
		Spaces:        []string{"page"},
		Sections:      append([]string(nil), task.sections...),
		Lines:         []string{"pages[0].label: playa=x, go=y"},
	}

	matched, err := reconcilePDFAKnownDifferences(task, record.Lines, nil, []testfixture.PDFADifference{record})
	if err != nil || len(matched) != 1 || matched[0] != record.ID {
		t.Fatalf("exact difference = %v, %v, want [%s], nil", matched, err, record.ID)
	}
	if _, err := reconcilePDFAKnownDifferences(task, []string{"unexpected"}, nil, []testfixture.PDFADifference{record}); err == nil || !strings.Contains(err.Error(), "unrecorded") {
		t.Fatalf("unrecorded difference error = %v", err)
	}
	if _, err := reconcilePDFAKnownDifferences(task, nil, nil, []testfixture.PDFADifference{record}); err == nil || !strings.Contains(err.Error(), "not observed") {
		t.Fatalf("stale difference error = %v", err)
	}
}

func TestReconcilePDFAKnownDifferencesUsesExactScope(t *testing.T) {
	base := compareTask{
		fixtureID:     "fixture",
		fixtureSHA256: strings.Repeat("a", 64),
		space:         "page",
		sections:      []string{"document", "pages"},
	}
	record := testfixture.PDFADifference{
		ID:            "known",
		FixtureID:     base.fixtureID,
		FixtureSHA256: base.fixtureSHA256,
		Spaces:        []string{"page"},
		Sections:      append([]string(nil), base.sections...),
		Lines:         []string{"difference"},
	}
	for _, mutate := range []func(*compareTask){
		func(task *compareTask) { task.fixtureSHA256 = strings.Repeat("b", 64) },
		func(task *compareTask) { task.space = "screen" },
		func(task *compareTask) { task.sections = []string{"pages", "document"} },
	} {
		task := base
		mutate(&task)
		if _, err := reconcilePDFAKnownDifferences(task, []string{"difference"}, nil, []testfixture.PDFADifference{record}); err == nil || !strings.Contains(err.Error(), "unrecorded") {
			t.Fatalf("wrong-scope task %#v error = %v", task, err)
		}
	}
}

func TestReconcilePDFAKnownDifferencesNormalizesPlayaProcessError(t *testing.T) {
	task := compareTask{
		fixtureID:     "fixture",
		fixtureSHA256: strings.Repeat("a", 64),
		space:         "default",
		sections:      []string{"document"},
	}
	record := testfixture.PDFADifference{
		ID:            "known-error",
		FixtureID:     task.fixtureID,
		FixtureSHA256: task.fixtureSHA256,
		Spaces:        []string{"default"},
		Sections:      append([]string(nil), task.sections...),
		PlayaError:    "NotImplementedError: unsupported filter",
	}
	err := &playaSnapshotError{message: "Traceback (most recent call last):\n  details\nNotImplementedError: unsupported filter"}
	matched, reconcileErr := reconcilePDFAKnownDifferences(task, nil, err, []testfixture.PDFADifference{record})
	if reconcileErr != nil || len(matched) != 1 || matched[0] != record.ID {
		t.Fatalf("known Playa error = %v, %v", matched, reconcileErr)
	}
}

func TestReconcilePDFAKnownDifferencesMatchesGoProjectionError(t *testing.T) {
	task := compareTask{
		fixtureID:     "fixture",
		fixtureSHA256: strings.Repeat("a", 64),
		space:         "default",
		sections:      []string{"content.contents"},
	}
	record := testfixture.PDFADifference{
		ID:            "known-go-error",
		FixtureID:     task.fixtureID,
		FixtureSHA256: task.fixtureSHA256,
		Spaces:        []string{"default"},
		Sections:      append([]string(nil), task.sections...),
		GoError:       "snapshot Go page 0: invalid content stream",
	}
	err := errors.New(record.GoError)
	matched, reconcileErr := reconcilePDFAKnownDifferences(task, nil, err, []testfixture.PDFADifference{record})
	if reconcileErr != nil || len(matched) != 1 || matched[0] != record.ID {
		t.Fatalf("known Go error = %v, %v", matched, reconcileErr)
	}
}

func TestReconcilePDFAKnownDifferencesAllowsTerminalErrorToPrecedeLineDifference(t *testing.T) {
	task := compareTask{
		fixtureID:     "fixture",
		fixtureSHA256: strings.Repeat("a", 64),
		space:         "default",
		sections:      []string{"structure", "content.extract_text.tagged"},
	}
	errorRecord := testfixture.PDFADifference{
		ID:            "known-go-error",
		FixtureID:     task.fixtureID,
		FixtureSHA256: task.fixtureSHA256,
		Spaces:        []string{"default"},
		Sections:      append([]string(nil), task.sections...),
		GoError:       "compare snapshot: go structure attributes differ",
	}
	lineRecord := testfixture.PDFADifference{
		ID:            "known-line-difference",
		FixtureID:     task.fixtureID,
		FixtureSHA256: task.fixtureSHA256,
		Spaces:        []string{"default"},
		Sections:      append([]string(nil), task.sections...),
		Lines:         []string{"pages[0].extract_text_tagged differs"},
	}
	err := errors.New(errorRecord.GoError)
	matched, reconcileErr := reconcilePDFAKnownDifferences(task, nil, err, []testfixture.PDFADifference{errorRecord, lineRecord})
	if reconcileErr != nil || len(matched) != 1 || matched[0] != errorRecord.ID {
		t.Fatalf("known terminal error before line difference = %v, %v", matched, reconcileErr)
	}
}

func TestReconcilePDFAKnownDifferencesRejectsUnobservedSecondTerminalError(t *testing.T) {
	task := compareTask{
		fixtureID:     "fixture",
		fixtureSHA256: strings.Repeat("a", 64),
		space:         "default",
		sections:      []string{"structure", "content.extract_text.tagged"},
	}
	observed := testfixture.PDFADifference{
		ID:            "observed-go-error",
		FixtureID:     task.fixtureID,
		FixtureSHA256: task.fixtureSHA256,
		Spaces:        []string{"default"},
		Sections:      append([]string(nil), task.sections...),
		GoError:       "compare snapshot: observed structure error",
	}
	unobserved := testfixture.PDFADifference{
		ID:            "unobserved-playa-error",
		FixtureID:     task.fixtureID,
		FixtureSHA256: task.fixtureSHA256,
		Spaces:        []string{"default"},
		Sections:      append([]string(nil), task.sections...),
		PlayaError:    "unobserved oracle error",
	}
	_, err := reconcilePDFAKnownDifferences(task, nil, errors.New(observed.GoError), []testfixture.PDFADifference{observed, unobserved})
	if err == nil || !strings.Contains(err.Error(), unobserved.ID) {
		t.Fatalf("unobserved second terminal error = %v, want error naming %q", err, unobserved.ID)
	}
}

func TestRenderPDFAIssueDraftIsNeutralAndCitesFixture(t *testing.T) {
	manifest, upstream := candidatePDFAIssueManifest(t)
	draft, err := renderPDFAIssueDraftFrom(manifest, upstream, "unknown-filter-linearization")
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(draft)
	if strings.Contains(lower, "go-playa") {
		t.Fatalf("issue draft names this implementation:\n%s", draft)
	}
	if strings.Contains(lower, "prepared and submitted") {
		t.Fatalf("candidate issue draft claims it was already submitted:\n%s", draft)
	}
	for _, want := range []string{
		"UnknownFilter-Linearized.pdf",
		"playa-pdf",
		"NotImplementedError",
		"pdf-association/pdf-differences/blob/",
		"AI coding agent",
		"9496dfea5150343c9dc05544d9c004bbcccf17e6",
	} {
		if !strings.Contains(draft, want) {
			t.Errorf("issue draft missing %q:\n%s", want, draft)
		}
	}
}

func TestPDFAUTF8DifferencesShareCombinedIssueDraft(t *testing.T) {
	manifest, upstream := candidatePDFAIssueManifest(t)
	want, err := renderPDFAIssueDraftFrom(manifest, upstream, "pdf20-utf8-text-strings")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"utf8-outline-decoded-as-legacy-text", "utf8-annotation-decoded-as-legacy-text"} {
		got, err := renderPDFAIssueDraftFrom(manifest, upstream, id)
		if err != nil {
			t.Fatalf("render %q: %v", id, err)
		}
		if got != want {
			t.Fatalf("draft for %q differs from its shared issue group", id)
		}
	}
	for _, wantText := range []string{"pdf20-utf8-test.pdf", "PDF 2.0 UTF-8 string and annotation.pdf", "ไฮไลต์ข้อความ"} {
		if !strings.Contains(want, wantText) {
			t.Errorf("combined UTF-8 draft missing %q:\n%s", wantText, want)
		}
	}
}

func TestEveryPDFAIssueDraftIsNeutral(t *testing.T) {
	manifest, upstream := candidatePDFAIssueManifest(t)
	for _, issue := range manifest.UpstreamIssues {
		draft, err := renderPDFAIssueDraftFrom(manifest, upstream, issue.ID)
		if err != nil {
			t.Fatalf("render %q: %v", issue.ID, err)
		}
		if strings.Contains(strings.ToLower(draft), "go-playa") {
			t.Fatalf("issue draft %q names this implementation:\n%s", issue.ID, draft)
		}
		if !strings.Contains(draft, issue.ExpectedBehavior) || !strings.Contains(draft, issue.ActualBehavior) {
			t.Fatalf("issue draft %q omits behavior details:\n%s", issue.ID, draft)
		}
	}
}

func TestRenderPDFAIssueDraftRejectsExistingOrSubmittedIssue(t *testing.T) {
	for _, state := range []string{"existing", "submitted"} {
		t.Run(state, func(t *testing.T) {
			manifest, upstream := candidatePDFAIssueManifest(t)
			manifest.UpstreamIssues[0].State = state
			manifest.UpstreamIssues[0].URL = "https://github.com/dhdaines/playa/issues/230"
			_, err := renderPDFAIssueDraftFrom(manifest, upstream, manifest.UpstreamIssues[0].ID)
			if err == nil || !strings.Contains(err.Error(), state) || !strings.Contains(err.Error(), manifest.UpstreamIssues[0].URL) {
				t.Fatalf("render state %q error = %v, want state and recorded URL", state, err)
			}
		})
	}
}

func TestRenderPDFAUpstreamIssueStatusListsEveryGroup(t *testing.T) {
	manifest, _ := candidatePDFAIssueManifest(t)
	manifest.UpstreamIssues[1].State = "existing"
	manifest.UpstreamIssues[1].URL = "https://github.com/dhdaines/playa/issues/231"
	status := renderPDFAUpstreamIssueStatus(manifest)
	for _, want := range []string{
		"pdf20-utf8-text-strings\tcandidate\tdhdaines/playa\t2026-09-05\t9496dfea5150343c9dc05544d9c004bbcccf17e6\t-",
		"unknown-filter-linearization\texisting\tdhdaines/playa\t2026-09-05\t9496dfea5150343c9dc05544d9c004bbcccf17e6\thttps://github.com/dhdaines/playa/issues/231",
	} {
		if !strings.Contains(status, want) {
			t.Errorf("status missing %q:\n%s", want, status)
		}
	}
}

func TestPDFAUpstreamIssueStatusCommand(t *testing.T) {
	var output strings.Builder
	if err := run([]string{"--issue-status"}, &output); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"pdf20-utf8-text-strings", "unknown-filter-linearization"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("status output missing %q:\n%s", want, output.String())
		}
	}
}

func candidatePDFAIssueManifest(t *testing.T) (testfixture.PDFAManifest, upstreamConfig) {
	t.Helper()
	manifest, err := testfixture.PDFAManifestValue()
	if err != nil {
		t.Fatal(err)
	}
	for index := range manifest.UpstreamIssues {
		manifest.UpstreamIssues[index].State = "candidate"
		manifest.UpstreamIssues[index].URL = ""
	}
	return manifest, upstreamConfig{
		Package: "playa-pdf",
		Version: "1.1.0",
		Tag:     "v1.1.0",
		Commit:  "85a9c1e22e327bf873e4282d66ed1dba89fa7575",
	}
}
