package testfixture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncPDFASourceFetchesPinnedCommitWithoutResetting(t *testing.T) {
	root := t.TempDir()
	fixtureRoot := filepath.Join(t.TempDir(), "pdf-association-fixtures")
	t.Setenv(pdfaFixtureDirEnv, fixtureRoot)
	contents := []byte("%PDF-2.0\n")
	digest := sha256.Sum256(contents)
	source := PDFASource{
		Name:       "examples",
		Repository: "https://github.com/pdf-association/examples.git",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
	}
	fixture := PDFAFixture{
		ID:     "sample",
		Source: source.Name,
		Path:   "sample.pdf",
		SHA256: hex.EncodeToString(digest[:]),
	}
	var calls []string
	run := func(_ context.Context, _ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		repoDir := filepath.Join(fixtureRoot, source.Name)
		switch {
		case strings.Contains(joined, " clone "):
			if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
				return "", err
			}
		case strings.HasSuffix(joined, "remote get-url origin"):
			return source.Repository + "\n", nil
		case strings.Contains(joined, "cat-file -e"):
			return "", errors.New("commit is not local")
		case strings.Contains(joined, "checkout --detach"):
			if err := os.WriteFile(filepath.Join(repoDir, fixture.Path), contents, 0o644); err != nil {
				return "", err
			}
		case strings.HasSuffix(joined, "rev-parse HEAD"):
			return source.Commit + "\n", nil
		case strings.HasSuffix(joined, "ls-tree -rz --name-only HEAD"):
			return fixture.Path + "\x00", nil
		}
		return "", nil
	}

	if err := syncPDFASource(context.Background(), root, source, []PDFAFixture{fixture}, nil, run); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(calls, "\n")
	for _, want := range []string{"clone --filter=blob:none", "fetch --depth=1 origin " + source.Commit, "checkout --detach " + source.Commit} {
		if !strings.Contains(got, want) {
			t.Fatalf("git calls:\n%s\nwant %q", got, want)
		}
	}
	if strings.Contains(got, " reset ") || strings.Contains(got, " clean ") || strings.Contains(got, " pull ") {
		t.Fatalf("git calls contain destructive or moving-ref command:\n%s", got)
	}
}

// Exercise real Git status: a clone without a checkout reports every file deleted.
func TestSyncPDFASourceFromFreshClone(t *testing.T) {
	ctx := context.Background()
	upstream := t.TempDir()
	contents := []byte("%PDF-2.0\n")
	if err := os.WriteFile(filepath.Join(upstream, "sample.pdf"), contents, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", upstream},
		{"-C", upstream, "add", "sample.pdf"},
		{"-C", upstream, "-c", "user.name=Fixture Test", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "public fixture"},
	} {
		if _, err := runPDFAGit(ctx, "", args...); err != nil {
			t.Fatal(err)
		}
	}
	commit, err := runPDFAGit(ctx, "", "-C", upstream, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	source := PDFASource{Name: "examples", Repository: "https://github.com/pdf-association/examples.git", Commit: strings.TrimSpace(commit)}
	digest := sha256.Sum256(contents)
	fixture := PDFAFixture{ID: "sample", Source: source.Name, Path: "sample.pdf", SHA256: hex.EncodeToString(digest[:])}
	t.Setenv(pdfaFixtureDirEnv, t.TempDir())
	run := func(ctx context.Context, dir string, args ...string) (string, error) {
		args = append([]string(nil), args...)
		if strings.Contains(strings.Join(args, " "), " clone ") {
			for i, arg := range args {
				if arg == source.Repository {
					args[i] = upstream
				}
			}
			if _, err := runPDFAGit(ctx, dir, args...); err != nil {
				return "", err
			}
			return runPDFAGit(ctx, "", "-C", args[len(args)-1], "remote", "set-url", "origin", source.Repository)
		}
		return runPDFAGit(ctx, dir, args...)
	}
	if err := syncPDFASource(ctx, t.TempDir(), source, []PDFAFixture{fixture}, nil, run); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPDFASourceRejectsWrongOrigin(t *testing.T) {
	root := t.TempDir()
	fixtureRoot := t.TempDir()
	t.Setenv(pdfaFixtureDirEnv, fixtureRoot)
	source := PDFASource{
		Name:       "examples",
		Repository: "https://github.com/pdf-association/examples.git",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
	}
	repoDir := filepath.Join(fixtureRoot, source.Name)
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.HasSuffix(strings.Join(args, " "), "remote get-url origin") {
			return "https://github.com/attacker/examples.git\n", nil
		}
		return "", fmt.Errorf("unexpected git call: %v", args)
	}

	err := checkPDFASource(context.Background(), root, source, nil, nil, run)
	if err == nil || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("checkPDFASource error = %v, want origin mismatch", err)
	}
}

func TestCheckPDFASourceRejectsDirtyCheckout(t *testing.T) {
	root := t.TempDir()
	fixtureRoot := t.TempDir()
	t.Setenv(pdfaFixtureDirEnv, fixtureRoot)
	source := PDFASource{
		Name:       "examples",
		Repository: "https://github.com/pdf-association/examples.git",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
	}
	repoDir := filepath.Join(fixtureRoot, source.Name)
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(_ context.Context, _ string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.HasSuffix(joined, "remote get-url origin"):
			return source.Repository + "\n", nil
		case strings.HasSuffix(joined, "status --porcelain"):
			return " M sample.pdf\n", nil
		default:
			return "", fmt.Errorf("unexpected git call: %v", args)
		}
	}

	err := checkPDFASource(context.Background(), root, source, nil, nil, run)
	if err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("checkPDFASource error = %v, want dirty checkout", err)
	}
}

func TestParsePDFAManifestAcceptsValidFixture(t *testing.T) {
	data := []byte(`{
  "schema_version": 2,
  "sources": [{
    "name": "examples",
    "repository": "https://github.com/pdf-association/pdf20examples.git",
    "commit": "c20f2c17bfcc4baab7cfe62e70fae64caf14d5fa",
    "license": "CC-BY-SA-4.0",
    "license_url": "https://github.com/pdf-association/pdf20examples/blob/c20f2c17bfcc4baab7cfe62e70fae64caf14d5fa/LICENSE.md"
  }],
  "fixtures": [{
    "id": "simple",
    "source": "examples",
    "path": "Simple PDF 2.0 file.pdf",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "behavior": "opens as PDF 2.0",
    "evidence_url": "https://example.test/readme",
    "password": "secret",
    "expected_page_failure": true,
    "compatibility": true,
    "sections": ["document", "pages"]
  }],
  "differences": []
}`)

	manifest, err := parsePDFAManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Sources) != 1 || manifest.Sources[0].Name != "examples" {
		t.Fatalf("sources = %#v", manifest.Sources)
	}
	if len(manifest.Fixtures) != 1 || manifest.Fixtures[0].ID != "simple" {
		t.Fatalf("fixtures = %#v", manifest.Fixtures)
	}
	if manifest.Fixtures[0].Password != "secret" {
		t.Fatalf("fixture password = %q, want %q", manifest.Fixtures[0].Password, "secret")
	}
	if !manifest.Fixtures[0].ExpectedPageFailure {
		t.Fatal("fixture expected page failure was not parsed")
	}
}

func TestParsePDFAManifestAcceptsAuditedExclusion(t *testing.T) {
	data := []byte(`{
  "schema_version": 2,
  "sources": [{
    "name": "examples",
    "repository": "https://github.com/pdf-association/pdf20examples.git",
    "commit": "c20f2c17bfcc4baab7cfe62e70fae64caf14d5fa",
    "license": "CC-BY-SA-4.0",
    "license_url": "https://github.com/pdf-association/pdf20examples/blob/c20f2c17bfcc4baab7cfe62e70fae64caf14d5fa/LICENSE.md"
  }],
  "fixtures": [],
  "exclusions": [{
    "source": "examples",
    "path": "documentation.pdf",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "reason": "documentation rather than a parser test",
    "evidence_url": "https://example.test/readme"
  }],
  "differences": []
}`)

	manifest, err := parsePDFAManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Exclusions) != 1 || manifest.Exclusions[0].Path != "documentation.pdf" {
		t.Fatalf("exclusions = %#v", manifest.Exclusions)
	}
}

func TestValidatePDFAManifestRejectsFixtureExclusionPathCollision(t *testing.T) {
	manifest := validPDFAManifestWithIssue()
	fixture := manifest.Fixtures[0]
	manifest.Exclusions = []PDFAExclusion{{
		Source:      fixture.Source,
		Path:        fixture.Path,
		SHA256:      fixture.SHA256,
		Reason:      "duplicate inventory entry",
		EvidenceURL: "https://example.test/evidence",
	}}
	if err := validatePDFAManifest(manifest); err == nil || !strings.Contains(err.Error(), "both fixture and exclusion") {
		t.Fatalf("validation error = %v, want fixture/exclusion collision", err)
	}
}

func TestValidatePDFAManifestRejectsInvalidExclusions(t *testing.T) {
	valid := validPDFAManifestWithIssue()
	valid.Exclusions = []PDFAExclusion{{
		Source:      valid.Sources[0].Name,
		Path:        "documentation.pdf",
		SHA256:      strings.Repeat("a", 64),
		Reason:      "documentation rather than a test fixture",
		EvidenceURL: "https://example.test/evidence",
	}}
	tests := []struct {
		name string
		edit func(*PDFAManifest)
		want string
	}{
		{
			name: "unknown source",
			edit: func(manifest *PDFAManifest) {
				manifest.Exclusions[0].Source = "missing"
			},
			want: "unknown source",
		},
		{
			name: "unsafe path",
			edit: func(manifest *PDFAManifest) {
				manifest.Exclusions[0].Path = "../escape.pdf"
			},
			want: "clean local relative path",
		},
		{
			name: "invalid digest",
			edit: func(manifest *PDFAManifest) {
				manifest.Exclusions[0].SHA256 = "not-a-digest"
			},
			want: "64-character hexadecimal hash",
		},
		{
			name: "missing reason",
			edit: func(manifest *PDFAManifest) {
				manifest.Exclusions[0].Reason = ""
			},
			want: "reason and an HTTPS evidence URL",
		},
		{
			name: "invalid evidence URL",
			edit: func(manifest *PDFAManifest) {
				manifest.Exclusions[0].EvidenceURL = "http://example.test/evidence"
			},
			want: "reason and an HTTPS evidence URL",
		},
		{
			name: "duplicate path",
			edit: func(manifest *PDFAManifest) {
				manifest.Exclusions = append(manifest.Exclusions, manifest.Exclusions[0])
			},
			want: "declared by both exclusion and exclusion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := valid
			manifest.Exclusions = append([]PDFAExclusion(nil), valid.Exclusions...)
			test.edit(&manifest)
			if err := validatePDFAManifest(manifest); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validatePDFAManifest error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCheckPDFASourceRejectsUnlistedPDF(t *testing.T) {
	root := t.TempDir()
	fixtureRoot := t.TempDir()
	t.Setenv(pdfaFixtureDirEnv, fixtureRoot)
	source := PDFASource{
		Name:       "examples",
		Repository: "https://github.com/pdf-association/examples.git",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
	}
	repoDir := filepath.Join(fixtureRoot, source.Name)
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "unlisted.pdf"), []byte("%PDF-2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(_ context.Context, _ string, args ...string) (string, error) {
		switch {
		case strings.HasSuffix(strings.Join(args, " "), "remote get-url origin"):
			return source.Repository + "\n", nil
		case strings.HasSuffix(strings.Join(args, " "), "rev-parse HEAD"):
			return source.Commit + "\n", nil
		case strings.HasSuffix(strings.Join(args, " "), "ls-tree -rz --name-only HEAD"):
			return "unlisted.pdf\x00", nil
		default:
			return "", nil
		}
	}

	err := checkPDFASource(context.Background(), root, source, nil, nil, run)
	if err == nil || !strings.Contains(err.Error(), "unlisted PDF") {
		t.Fatalf("checkPDFASource error = %v, want unlisted PDF", err)
	}
}

func TestCheckPDFASourceAcceptsExcludedPDF(t *testing.T) {
	root := t.TempDir()
	fixtureRoot := t.TempDir()
	t.Setenv(pdfaFixtureDirEnv, fixtureRoot)
	contents := []byte("%PDF-2.0\n")
	digest := sha256.Sum256(contents)
	source := PDFASource{
		Name:       "examples",
		Repository: "https://github.com/pdf-association/examples.git",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
	}
	repoDir := filepath.Join(fixtureRoot, source.Name)
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "documentation.pdf"), contents, 0o644); err != nil {
		t.Fatal(err)
	}
	exclusions := []PDFAExclusion{{
		Source: source.Name,
		Path:   "documentation.pdf",
		SHA256: hex.EncodeToString(digest[:]),
	}}
	run := func(_ context.Context, _ string, args ...string) (string, error) {
		switch {
		case strings.HasSuffix(strings.Join(args, " "), "remote get-url origin"):
			return source.Repository + "\n", nil
		case strings.HasSuffix(strings.Join(args, " "), "rev-parse HEAD"):
			return source.Commit + "\n", nil
		case strings.HasSuffix(strings.Join(args, " "), "ls-tree -rz --name-only HEAD"):
			return "documentation.pdf\x00", nil
		default:
			return "", nil
		}
	}

	if err := checkPDFASource(context.Background(), root, source, nil, exclusions, run); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePDFAExclusionRejectsDigestMismatch(t *testing.T) {
	root := t.TempDir()
	fixtureRoot := t.TempDir()
	t.Setenv(pdfaFixtureDirEnv, fixtureRoot)
	exclusion := PDFAExclusion{
		Source: "examples",
		Path:   "documentation.pdf",
		SHA256: strings.Repeat("0", 64),
	}
	filePath := filepath.Join(fixtureRoot, exclusion.Source, exclusion.Path)
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("%PDF-2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := resolvePDFAExclusion(root, exclusion); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("resolvePDFAExclusion error = %v, want digest mismatch", err)
	}
}

func TestCheckPDFASourceInventoryRejectsManifestPathNotTracked(t *testing.T) {
	source := PDFASource{Name: "examples"}
	fixtures := []PDFAFixture{{Path: "declared.pdf"}}
	run := func(_ context.Context, _ string, _ ...string) (string, error) {
		return "notes.txt\x00", nil
	}

	err := checkPDFASourceInventory(context.Background(), "/fixtures/examples", source, fixtures, nil, run)
	if err == nil || !strings.Contains(err.Error(), "is not a tracked PDF") {
		t.Fatalf("checkPDFASourceInventory error = %v, want untracked manifest path", err)
	}
}

func TestValidatePDFAManifestConstrainsExpectedPasswordFailure(t *testing.T) {
	manifest := validPDFAManifestWithIssue()
	manifest.Fixtures[0].ExpectedPasswordFailure = true
	if err := validatePDFAManifest(manifest); err == nil || !strings.Contains(err.Error(), "requires a password") {
		t.Fatalf("validation without password error = %v", err)
	}
	manifest.Fixtures[0].Password = "wrong password"
	if err := validatePDFAManifest(manifest); err == nil || !strings.Contains(err.Error(), "cannot be compatibility-enabled") {
		t.Fatalf("validation with compatibility error = %v", err)
	}
	manifest.Fixtures[0].Compatibility = false
	manifest.Fixtures[0].Sections = nil
	manifest.Differences = nil
	manifest.UpstreamIssues = nil
	if err := validatePDFAManifest(manifest); err != nil {
		t.Fatalf("validation for negative password fixture: %v", err)
	}
}

func TestParsePDFAManifestAcceptsUpstreamIssueRegistry(t *testing.T) {
	data := []byte(`{
  "schema_version": 2,
  "sources": [{
    "name": "examples",
    "repository": "https://github.com/pdf-association/pdf20examples.git",
    "commit": "c20f2c17bfcc4baab7cfe62e70fae64caf14d5fa",
    "license": "CC-BY-SA-4.0",
    "license_url": "https://example.test/license"
  }],
  "fixtures": [{
    "id": "simple",
    "source": "examples",
    "path": "simple.pdf",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "behavior": "opens as PDF 2.0",
    "evidence_url": "https://example.test/readme",
    "compatibility": true,
    "sections": ["document"]
  }],
  "differences": [{
    "id": "known-difference",
    "fixture_id": "simple",
    "fixture_sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    "spaces": ["page"],
    "sections": ["document"],
    "lines": ["difference"],
    "playa_error": "",
    "title": "Known difference",
    "expectation": "Expected behavior",
    "playa_behavior": "Actual behavior",
    "evidence_url": "https://example.test/readme",
    "reproduction": "Open the fixture"
  }],
  "upstream_issues": [{
    "id": "known-issue",
    "difference_ids": ["known-difference"],
    "state": "candidate",
    "repository": "dhdaines/playa",
    "checked_commit": "9496dfea5150343c9dc05544d9c004bbcccf17e6",
    "checked_at": "2026-09-05",
    "url": "",
    "title": "Known issue",
    "summary": "Summary",
    "reproduction": "Reproduction",
    "actual_behavior": "Actual behavior",
    "expected_behavior": "Expected behavior",
    "relevant_implementation": "Relevant implementation"
  }]
}`)

	manifest, err := parsePDFAManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.UpstreamIssues) != 1 || manifest.UpstreamIssues[0].ID != "known-issue" {
		t.Fatalf("upstream issues = %#v", manifest.UpstreamIssues)
	}
}

func TestValidatePDFAManifestRejectsInvalidUpstreamIssueRegistry(t *testing.T) {
	valid := validPDFAManifestWithIssue()
	tests := []struct {
		name string
		edit func(*PDFAManifest)
		want string
	}{
		{
			name: "duplicate issue id",
			edit: func(manifest *PDFAManifest) {
				manifest.UpstreamIssues = append(manifest.UpstreamIssues, manifest.UpstreamIssues[0])
			},
			want: "duplicate PDF Association upstream issue id",
		},
		{
			name: "unknown difference",
			edit: func(manifest *PDFAManifest) {
				manifest.UpstreamIssues[0].DifferenceIDs = []string{"missing"}
			},
			want: "references unknown difference",
		},
		{
			name: "difference assigned twice",
			edit: func(manifest *PDFAManifest) {
				second := manifest.UpstreamIssues[0]
				second.ID = "second-issue"
				manifest.UpstreamIssues = append(manifest.UpstreamIssues, second)
			},
			want: "assigned to both",
		},
		{
			name: "difference unassigned",
			edit: func(manifest *PDFAManifest) {
				manifest.UpstreamIssues = nil
			},
			want: "has no upstream issue record",
		},
		{
			name: "invalid state",
			edit: func(manifest *PDFAManifest) {
				manifest.UpstreamIssues[0].State = "open"
			},
			want: "unsupported state",
		},
		{
			name: "candidate with url",
			edit: func(manifest *PDFAManifest) {
				manifest.UpstreamIssues[0].URL = "https://github.com/dhdaines/playa/issues/1"
			},
			want: "candidate must not have",
		},
		{
			name: "submitted without url",
			edit: func(manifest *PDFAManifest) {
				manifest.UpstreamIssues[0].State = "submitted"
			},
			want: "in state \"submitted\" requires",
		},
		{
			name: "wrong repository url",
			edit: func(manifest *PDFAManifest) {
				manifest.UpstreamIssues[0].State = "existing"
				manifest.UpstreamIssues[0].URL = "https://github.com/elsewhere/playa/issues/1"
			},
			want: "issue URL",
		},
		{
			name: "invalid checked commit",
			edit: func(manifest *PDFAManifest) {
				manifest.UpstreamIssues[0].CheckedCommit = "main"
			},
			want: "checked_commit",
		},
		{
			name: "invalid checked date",
			edit: func(manifest *PDFAManifest) {
				manifest.UpstreamIssues[0].CheckedAt = "September 5"
			},
			want: "checked_at",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := valid
			manifest.Sources = append([]PDFASource(nil), valid.Sources...)
			manifest.Fixtures = append([]PDFAFixture(nil), valid.Fixtures...)
			manifest.Differences = append([]PDFADifference(nil), valid.Differences...)
			manifest.UpstreamIssues = append([]PDFAUpstreamIssue(nil), valid.UpstreamIssues...)
			test.edit(&manifest)
			if err := validatePDFAManifest(manifest); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validatePDFAManifest error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidatePDFAManifestAcceptsOneGoProjectionError(t *testing.T) {
	manifest := validPDFAManifestWithIssue()
	manifest.Differences[0].Lines = nil
	manifest.Differences[0].GoError = "snapshot Go page 0: invalid content stream"
	if err := validatePDFAManifest(manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Differences[0].PlayaError = "SyntaxError: invalid content stream"
	if err := validatePDFAManifest(manifest); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("validation with both Go and Playa errors = %v", err)
	}
}

func validPDFAManifestWithIssue() PDFAManifest {
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	return PDFAManifest{
		SchemaVersion: 2,
		Sources: []PDFASource{{
			Name:       "examples",
			Repository: "https://github.com/pdf-association/examples.git",
			Commit:     "0123456789abcdef0123456789abcdef01234567",
			License:    "MIT",
			LicenseURL: "https://example.test/license",
		}},
		Fixtures: []PDFAFixture{{
			ID:            "simple",
			Source:        "examples",
			Path:          "simple.pdf",
			SHA256:        digest,
			Behavior:      "opens",
			EvidenceURL:   "https://example.test/evidence",
			Compatibility: true,
			Sections:      []string{"document"},
		}},
		Differences: []PDFADifference{{
			ID:            "known-difference",
			FixtureID:     "simple",
			FixtureSHA256: digest,
			Spaces:        []string{"page"},
			Sections:      []string{"document"},
			Lines:         []string{"difference"},
			Title:         "Known difference",
			Expectation:   "Expected behavior",
			PlayaBehavior: "Actual behavior",
			EvidenceURL:   "https://example.test/evidence",
			Reproduction:  "Open the fixture",
		}},
		UpstreamIssues: []PDFAUpstreamIssue{{
			ID:                     "known-issue",
			DifferenceIDs:          []string{"known-difference"},
			State:                  "candidate",
			Repository:             "dhdaines/playa",
			CheckedCommit:          "9496dfea5150343c9dc05544d9c004bbcccf17e6",
			CheckedAt:              "2026-09-05",
			Title:                  "Known issue",
			Summary:                "Summary",
			Reproduction:           "Reproduction",
			ActualBehavior:         "Actual behavior",
			ExpectedBehavior:       "Expected behavior",
			RelevantImplementation: "Relevant implementation",
		}},
	}
}

func TestParsePDFAManifestRejectsUnknownField(t *testing.T) {
	data := []byte(`{"schema_version":2,"sources":[],"fixtures":[],"exclusions":[],"differences":[],"extra":true}`)
	if _, err := parsePDFAManifest(data); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("parsePDFAManifest error = %v, want unknown field", err)
	}
}

func TestParsePDFAManifestRejectsDuplicateFixtureID(t *testing.T) {
	data := []byte(`{
  "schema_version": 2,
  "sources": [{"name":"examples","repository":"https://github.com/pdf-association/examples.git","commit":"0123456789abcdef0123456789abcdef01234567","license":"MIT","license_url":"https://example.test/license"}],
  "fixtures": [
    {"id":"duplicate","source":"examples","path":"a.pdf","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","behavior":"a","evidence_url":"https://example.test/a","compatibility":false,"sections":[]},
    {"id":"duplicate","source":"examples","path":"b.pdf","sha256":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","behavior":"b","evidence_url":"https://example.test/b","compatibility":false,"sections":[]}
  ],
  "differences": []
}`)
	if _, err := parsePDFAManifest(data); err == nil || !strings.Contains(err.Error(), "duplicate fixture id") {
		t.Fatalf("parsePDFAManifest error = %v, want duplicate fixture id", err)
	}
}

func TestParsePDFAManifestRejectsUnsafeFixturePath(t *testing.T) {
	data := []byte(`{
  "schema_version": 2,
  "sources": [{"name":"examples","repository":"https://github.com/pdf-association/examples.git","commit":"0123456789abcdef0123456789abcdef01234567","license":"MIT","license_url":"https://example.test/license"}],
  "fixtures": [{"id":"escape","source":"examples","path":"../escape.pdf","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","behavior":"escape","evidence_url":"https://example.test/a","compatibility":false,"sections":[]}],
  "differences": []
}`)
	if _, err := parsePDFAManifest(data); err == nil || !strings.Contains(err.Error(), "local relative path") {
		t.Fatalf("parsePDFAManifest error = %v, want unsafe path", err)
	}
}

func TestResolvePDFAPathVerifiesDigest(t *testing.T) {
	root := t.TempDir()
	fixtureRoot := t.TempDir()
	t.Setenv(pdfaFixtureDirEnv, fixtureRoot)
	contents := []byte("%PDF-2.0\n")
	digest := sha256.Sum256(contents)
	fixture := PDFAFixture{ID: "sample", Source: "examples", Path: "nested/sample.pdf", SHA256: hex.EncodeToString(digest[:])}
	want := filepath.Join(fixtureRoot, "examples", "nested", "sample.pdf")
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, contents, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolvePDFAPath(root, fixture)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolvePDFAPath = %q, want %q", got, want)
	}

	fixture.SHA256 = strings.Repeat("0", 64)
	if _, err := resolvePDFAPath(root, fixture); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("resolvePDFAPath error = %v, want digest mismatch", err)
	}
}

func TestResolvePDFAPathReportsPullCommandWhenMissing(t *testing.T) {
	root := t.TempDir()
	t.Setenv(pdfaFixtureDirEnv, filepath.Join(t.TempDir(), "missing"))
	fixture := PDFAFixture{ID: "sample", Source: "examples", Path: "sample.pdf", SHA256: strings.Repeat("0", 64)}

	_, err := resolvePDFAPath(root, fixture)
	if !errors.Is(err, errPDFAFixtureUnavailable) {
		t.Fatalf("resolvePDFAPath error = %v, want errPDFAFixtureUnavailable", err)
	}
	if !strings.Contains(err.Error(), "make pdfa-fixtures-pull") {
		t.Fatalf("resolvePDFAPath error = %q, want pull command", err)
	}
}
