package main

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func reconcilePDFAKnownDifferences(task compareTask, differences []string, compareErr error, records []testfixture.PDFADifference) ([]string, error) {
	inScope := make([]testfixture.PDFADifference, 0)
	for _, record := range records {
		if record.FixtureID == task.fixtureID &&
			record.FixtureSHA256 == task.fixtureSHA256 &&
			slices.Contains(record.Spaces, task.space) &&
			slices.Equal(record.Sections, task.sections) {
			inScope = append(inScope, record)
		}
	}

	if compareErr != nil {
		actual, playaError := normalizedPlayaSnapshotError(compareErr)
		if !playaError {
			actual = compareErr.Error()
		}
		matched := make([]string, 0, 1)
		matchedIndex := -1
		for index, record := range inScope {
			expected := record.GoError
			if playaError {
				expected = record.PlayaError
			}
			if expected == actual {
				matched = append(matched, record.ID)
				matchedIndex = index
			}
		}
		if len(matched) != 1 {
			side := "Go"
			if playaError {
				side = "Playa"
			}
			return nil, fmt.Errorf("unrecorded %s error for PDF Association fixture %q [%s] sections=%s: %s", side, task.fixtureID, task.space, strings.Join(task.sections, ","), actual)
		}
		for index, record := range inScope {
			if index != matchedIndex && (record.PlayaError != "" || record.GoError != "") {
				return nil, fmt.Errorf("known PDF Association difference %q was not observed for fixture %q [%s] sections=%s", record.ID, task.fixtureID, task.space, strings.Join(task.sections, ","))
			}
		}
		// A terminal projection or oracle error prevents later line
		// differences in the same scope from being observed. Keep only those
		// records active: once the error disappears, the normal comparison
		// path below requires every exact line difference again.
		return matched, nil
	}

	expected := make(map[string]string)
	for _, record := range inScope {
		if record.PlayaError != "" || record.GoError != "" {
			return nil, fmt.Errorf("known PDF Association difference %q was not observed for fixture %q [%s] sections=%s", record.ID, task.fixtureID, task.space, strings.Join(task.sections, ","))
		}
		for _, line := range record.Lines {
			expected[line] = record.ID
		}
	}
	matchedIDs := make(map[string]bool)
	for _, difference := range differences {
		id, ok := expected[difference]
		if !ok {
			return nil, fmt.Errorf("unrecorded difference for PDF Association fixture %q [%s] sections=%s: %s", task.fixtureID, task.space, strings.Join(task.sections, ","), difference)
		}
		delete(expected, difference)
		matchedIDs[id] = true
	}
	if len(expected) != 0 {
		missing := make([]string, 0, len(expected))
		for line := range expected {
			missing = append(missing, line)
		}
		slices.Sort(missing)
		return nil, fmt.Errorf("known PDF Association differences not observed for fixture %q [%s] sections=%s: %s", task.fixtureID, task.space, strings.Join(task.sections, ","), strings.Join(missing, "; "))
	}
	matched := make([]string, 0, len(matchedIDs))
	for id := range matchedIDs {
		matched = append(matched, id)
	}
	slices.Sort(matched)
	return matched, nil
}

func normalizedPlayaSnapshotError(err error) (string, bool) {
	var snapshotErr *playaSnapshotError
	if !errors.As(err, &snapshotErr) {
		return "", false
	}
	lines := strings.Split(strings.TrimSpace(snapshotErr.message), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if line := strings.TrimSpace(lines[index]); line != "" {
			return line, true
		}
	}
	return "", true
}

func renderPDFAIssueDraft(id string) (string, error) {
	manifest, err := testfixture.PDFAManifestValue()
	if err != nil {
		return "", err
	}
	upstreamPath, err := repositoryFile("compat", "upstream.toml")
	if err != nil {
		return "", err
	}
	upstream, err := loadUpstreamConfig(upstreamPath)
	if err != nil {
		return "", err
	}
	return renderPDFAIssueDraftFrom(manifest, upstream, id)
}

func renderPDFAIssueDraftFrom(manifest testfixture.PDFAManifest, upstream upstreamConfig, id string) (string, error) {
	issue, err := resolvePDFAUpstreamIssue(manifest, id)
	if err != nil {
		return "", err
	}
	if issue.State != "candidate" {
		return "", fmt.Errorf("PDF Association upstream issue %q is already %s at %s", issue.ID, issue.State, issue.URL)
	}

	differences := make(map[string]testfixture.PDFADifference, len(manifest.Differences))
	for _, difference := range manifest.Differences {
		differences[difference.ID] = difference
	}
	fixtures := make(map[string]testfixture.PDFAFixture, len(manifest.Fixtures))
	for _, fixture := range manifest.Fixtures {
		fixtures[fixture.ID] = fixture
	}
	sources := make(map[string]testfixture.PDFASource, len(manifest.Sources))
	for _, source := range manifest.Sources {
		sources[source.Name] = source
	}

	var testFiles strings.Builder
	seenFixtures := make(map[string]bool)
	for _, differenceID := range issue.DifferenceIDs {
		difference, exists := differences[differenceID]
		if !exists {
			return "", fmt.Errorf("PDF Association upstream issue %q has unresolved difference %q", issue.ID, differenceID)
		}
		if seenFixtures[difference.FixtureID] {
			continue
		}
		fixture, exists := fixtures[difference.FixtureID]
		if !exists {
			return "", fmt.Errorf("PDF Association difference %q has unresolved fixture provenance", differenceID)
		}
		source, exists := sources[fixture.Source]
		if !exists {
			return "", fmt.Errorf("PDF Association fixture %q has unresolved source provenance", fixture.ID)
		}
		fileURL := strings.TrimSuffix(source.Repository, ".git") + "/blob/" + source.Commit + "/" + escapeRepositoryPath(fixture.Path)
		fmt.Fprintf(&testFiles, "- [%s](%s), with expected behavior described in the [PDF Association test documentation](%s).\n", filepath.Base(fixture.Path), fileURL, difference.EvidenceURL)
		seenFixtures[difference.FixtureID] = true
	}

	relevant := ""
	if issue.RelevantImplementation != "" {
		relevant = "\n## Relevant implementation\n\n" + issue.RelevantImplementation + "\n"
	}
	return fmt.Sprintf("# %s\n\n## Summary\n\n%s\n\n## Versions tested\n\n- `%s` %s (`%s`, commit `%s`)\n- Current `main` at commit [`%s`](https://github.com/%s/commit/%s), checked on %s\n\n## Test files\n\n%s\n## Reproduction\n\n%s\n\n## Actual behavior\n\n%s\n\n## Expected behavior\n\n%s\n%s\n## Disclosure\n\nThis issue was prepared by an AI coding agent on behalf of @lin-string. The reproduction and results were verified against the current default branch.\n",
		issue.Title,
		issue.Summary,
		upstream.Package,
		upstream.Version,
		upstream.Tag,
		upstream.Commit,
		issue.CheckedCommit,
		issue.Repository,
		issue.CheckedCommit,
		issue.CheckedAt,
		testFiles.String(),
		issue.Reproduction,
		issue.ActualBehavior,
		issue.ExpectedBehavior,
		relevant,
	), nil
}

func resolvePDFAUpstreamIssue(manifest testfixture.PDFAManifest, id string) (testfixture.PDFAUpstreamIssue, error) {
	for _, issue := range manifest.UpstreamIssues {
		if issue.ID == id || slices.Contains(issue.DifferenceIDs, id) {
			return issue, nil
		}
	}
	return testfixture.PDFAUpstreamIssue{}, fmt.Errorf("unknown PDF Association upstream issue or difference id %q", id)
}

func renderPDFAUpstreamIssueStatus(manifest testfixture.PDFAManifest) string {
	var output strings.Builder
	for _, issue := range manifest.UpstreamIssues {
		issueURL := issue.URL
		if issueURL == "" {
			issueURL = "-"
		}
		fmt.Fprintf(&output, "%s\t%s\t%s\t%s\t%s\t%s\n", issue.ID, issue.State, issue.Repository, issue.CheckedAt, issue.CheckedCommit, issueURL)
	}
	return output.String()
}

func escapeRepositoryPath(value string) string {
	parts := strings.Split(filepath.ToSlash(value), "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
