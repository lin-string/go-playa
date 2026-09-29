package testfixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const pdfaFixtureDirEnv = "GO_PLAYA_PDFA_FIXTURE_DIR"
const fetchPDFAFixturesEnv = "GO_PLAYA_FETCH_PDFA_FIXTURES"

var errPDFAFixtureUnavailable = errors.New("PDF Association test fixture unavailable")

//go:embed pdf_association_fixtures.json
var embeddedPDFAManifest []byte

var pdfaIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type PDFAManifest struct {
	SchemaVersion  int                 `json:"schema_version"`
	Sources        []PDFASource        `json:"sources"`
	Fixtures       []PDFAFixture       `json:"fixtures"`
	Exclusions     []PDFAExclusion     `json:"exclusions"`
	Differences    []PDFADifference    `json:"differences"`
	UpstreamIssues []PDFAUpstreamIssue `json:"upstream_issues"`
}

type PDFASource struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	License    string `json:"license"`
	LicenseURL string `json:"license_url"`
}

type PDFAFixture struct {
	ID                      string   `json:"id"`
	Source                  string   `json:"source"`
	Path                    string   `json:"path"`
	SHA256                  string   `json:"sha256"`
	Behavior                string   `json:"behavior"`
	EvidenceURL             string   `json:"evidence_url"`
	Password                string   `json:"password,omitempty"`
	ExpectedPasswordFailure bool     `json:"expected_password_failure,omitempty"`
	ExpectedPageFailure     bool     `json:"expected_page_failure,omitempty"`
	Compatibility           bool     `json:"compatibility"`
	Sections                []string `json:"sections"`
}

type PDFAExclusion struct {
	Source      string `json:"source"`
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	Reason      string `json:"reason"`
	EvidenceURL string `json:"evidence_url"`
}

type PDFACase struct {
	Fixture PDFAFixture
	Path    string
}

type PDFADifference struct {
	ID            string   `json:"id"`
	FixtureID     string   `json:"fixture_id"`
	FixtureSHA256 string   `json:"fixture_sha256"`
	Spaces        []string `json:"spaces"`
	Sections      []string `json:"sections"`
	Lines         []string `json:"lines"`
	PlayaError    string   `json:"playa_error"`
	GoError       string   `json:"go_error,omitempty"`
	Title         string   `json:"title"`
	Expectation   string   `json:"expectation"`
	PlayaBehavior string   `json:"playa_behavior"`
	EvidenceURL   string   `json:"evidence_url"`
	Reproduction  string   `json:"reproduction"`
}

type PDFAUpstreamIssue struct {
	ID                     string   `json:"id"`
	DifferenceIDs          []string `json:"difference_ids"`
	State                  string   `json:"state"`
	Repository             string   `json:"repository"`
	CheckedCommit          string   `json:"checked_commit"`
	CheckedAt              string   `json:"checked_at"`
	URL                    string   `json:"url"`
	Title                  string   `json:"title"`
	Summary                string   `json:"summary"`
	Reproduction           string   `json:"reproduction"`
	ActualBehavior         string   `json:"actual_behavior"`
	ExpectedBehavior       string   `json:"expected_behavior"`
	RelevantImplementation string   `json:"relevant_implementation"`
}

func parsePDFAManifest(data []byte) (PDFAManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest PDFAManifest
	if err := decoder.Decode(&manifest); err != nil {
		return PDFAManifest{}, fmt.Errorf("decode PDF Association fixture manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON value")
		}
		return PDFAManifest{}, fmt.Errorf("decode PDF Association fixture manifest: %w", err)
	}
	if err := validatePDFAManifest(manifest); err != nil {
		return PDFAManifest{}, err
	}
	return manifest, nil
}

func validatePDFAManifest(manifest PDFAManifest) error {
	if manifest.SchemaVersion != 2 {
		return fmt.Errorf("PDF Association fixture manifest schema_version = %d, want 2", manifest.SchemaVersion)
	}
	sources := make(map[string]PDFASource, len(manifest.Sources))
	for _, source := range manifest.Sources {
		if !pdfaIDPattern.MatchString(source.Name) {
			return fmt.Errorf("PDF Association source name %q is not a stable id", source.Name)
		}
		if _, exists := sources[source.Name]; exists {
			return fmt.Errorf("duplicate PDF Association source name %q", source.Name)
		}
		if !strings.HasPrefix(source.Repository, "https://github.com/") || !strings.HasSuffix(source.Repository, ".git") {
			return fmt.Errorf("PDF Association source %q repository must be an HTTPS GitHub clone URL", source.Name)
		}
		if !validHex(source.Commit, 40) {
			return fmt.Errorf("PDF Association source %q commit must be a 40-character hexadecimal hash", source.Name)
		}
		if source.License == "" || !strings.HasPrefix(source.LicenseURL, "https://") {
			return fmt.Errorf("PDF Association source %q must declare its license and HTTPS license URL", source.Name)
		}
		sources[source.Name] = source
	}
	fixtures := make(map[string]PDFAFixture, len(manifest.Fixtures))
	inventoryPaths := make(map[string]string, len(manifest.Fixtures)+len(manifest.Exclusions))
	for _, fixture := range manifest.Fixtures {
		if !pdfaIDPattern.MatchString(fixture.ID) {
			return fmt.Errorf("PDF Association fixture id %q is not a stable id", fixture.ID)
		}
		if _, exists := fixtures[fixture.ID]; exists {
			return fmt.Errorf("duplicate fixture id %q", fixture.ID)
		}
		if _, exists := sources[fixture.Source]; !exists {
			return fmt.Errorf("PDF Association fixture %q references unknown source %q", fixture.ID, fixture.Source)
		}
		if !validPDFAPath(fixture.Path) {
			return fmt.Errorf("PDF Association fixture %q path %q must be a clean local relative path", fixture.ID, fixture.Path)
		}
		if !validHex(fixture.SHA256, 64) {
			return fmt.Errorf("PDF Association fixture %q SHA-256 must be a 64-character hexadecimal hash", fixture.ID)
		}
		if fixture.Behavior == "" || !strings.HasPrefix(fixture.EvidenceURL, "https://") {
			return fmt.Errorf("PDF Association fixture %q must declare behavior and an HTTPS evidence URL", fixture.ID)
		}
		if fixture.ExpectedPasswordFailure {
			if fixture.Password == "" {
				return fmt.Errorf("PDF Association fixture %q expected password failure requires a password", fixture.ID)
			}
			if fixture.Compatibility {
				return fmt.Errorf("PDF Association fixture %q expected password failure cannot be compatibility-enabled", fixture.ID)
			}
		}
		if fixture.ExpectedPasswordFailure && fixture.ExpectedPageFailure {
			return fmt.Errorf("PDF Association fixture %q cannot expect both password and page failures", fixture.ID)
		}
		if !fixture.Compatibility && len(fixture.Sections) != 0 {
			return fmt.Errorf("PDF Association fixture %q is not compatibility-enabled but declares sections", fixture.ID)
		}
		if fixture.Compatibility && len(fixture.Sections) == 0 {
			return fmt.Errorf("PDF Association fixture %q is compatibility-enabled without sections", fixture.ID)
		}
		inventoryKey := fixture.Source + "\x00" + fixture.Path
		if previous, exists := inventoryPaths[inventoryKey]; exists {
			return fmt.Errorf("PDF Association source path %q is declared by both %s and fixture %q", fixture.Path, previous, fixture.ID)
		}
		inventoryPaths[inventoryKey] = "fixture"
		fixtures[fixture.ID] = fixture
	}
	for _, exclusion := range manifest.Exclusions {
		if _, exists := sources[exclusion.Source]; !exists {
			return fmt.Errorf("PDF Association exclusion %q references unknown source %q", exclusion.Path, exclusion.Source)
		}
		if !validPDFAPath(exclusion.Path) {
			return fmt.Errorf("PDF Association exclusion path %q must be a clean local relative path", exclusion.Path)
		}
		if !validHex(exclusion.SHA256, 64) {
			return fmt.Errorf("PDF Association exclusion %q SHA-256 must be a 64-character hexadecimal hash", exclusion.Path)
		}
		if exclusion.Reason == "" || !strings.HasPrefix(exclusion.EvidenceURL, "https://") {
			return fmt.Errorf("PDF Association exclusion %q must declare a reason and an HTTPS evidence URL", exclusion.Path)
		}
		inventoryKey := exclusion.Source + "\x00" + exclusion.Path
		if previous, exists := inventoryPaths[inventoryKey]; exists {
			return fmt.Errorf("PDF Association source path %q is declared by both %s and exclusion", exclusion.Path, previous)
		}
		inventoryPaths[inventoryKey] = "exclusion"
	}
	differenceIDs := make(map[string]struct{}, len(manifest.Differences))
	for _, difference := range manifest.Differences {
		if !pdfaIDPattern.MatchString(difference.ID) {
			return fmt.Errorf("PDF Association difference id %q is not a stable id", difference.ID)
		}
		if _, exists := differenceIDs[difference.ID]; exists {
			return fmt.Errorf("duplicate PDF Association difference id %q", difference.ID)
		}
		fixture, exists := fixtures[difference.FixtureID]
		if !exists {
			return fmt.Errorf("PDF Association difference %q references unknown fixture %q", difference.ID, difference.FixtureID)
		}
		if difference.FixtureSHA256 != fixture.SHA256 {
			return fmt.Errorf("PDF Association difference %q fixture SHA-256 does not match %q", difference.ID, difference.FixtureID)
		}
		if !fixture.Compatibility {
			return fmt.Errorf("PDF Association difference %q references non-compatibility fixture %q", difference.ID, difference.FixtureID)
		}
		if len(difference.Spaces) == 0 {
			return fmt.Errorf("PDF Association difference %q has no coordinate spaces", difference.ID)
		}
		seenSpaces := map[string]bool{}
		for _, space := range difference.Spaces {
			if space != "page" && space != "screen" && space != "default" {
				return fmt.Errorf("PDF Association difference %q has unsupported coordinate space %q", difference.ID, space)
			}
			if seenSpaces[space] {
				return fmt.Errorf("PDF Association difference %q repeats coordinate space %q", difference.ID, space)
			}
			seenSpaces[space] = true
		}
		if !equalStrings(difference.Sections, fixture.Sections) {
			return fmt.Errorf("PDF Association difference %q sections do not exactly match fixture %q", difference.ID, difference.FixtureID)
		}
		outcomes := 0
		if len(difference.Lines) != 0 {
			outcomes++
		}
		if difference.PlayaError != "" {
			outcomes++
		}
		if difference.GoError != "" {
			outcomes++
		}
		if outcomes != 1 {
			return fmt.Errorf("PDF Association difference %q must declare exactly one of lines, playa_error, or go_error", difference.ID)
		}
		seenLines := map[string]bool{}
		for _, line := range difference.Lines {
			if line == "" || seenLines[line] {
				return fmt.Errorf("PDF Association difference %q has an empty or duplicate line", difference.ID)
			}
			seenLines[line] = true
		}
		if strings.Contains(difference.PlayaError, "\n") {
			return fmt.Errorf("PDF Association difference %q playa_error must be one terminal line", difference.ID)
		}
		if strings.Contains(difference.GoError, "\n") {
			return fmt.Errorf("PDF Association difference %q go_error must be one terminal line", difference.ID)
		}
		if difference.Title == "" || difference.Expectation == "" || difference.PlayaBehavior == "" || difference.Reproduction == "" {
			return fmt.Errorf("PDF Association difference %q has incomplete issue details", difference.ID)
		}
		if !strings.HasPrefix(difference.EvidenceURL, "https://") {
			return fmt.Errorf("PDF Association difference %q must declare an HTTPS evidence URL", difference.ID)
		}
		differenceIDs[difference.ID] = struct{}{}
	}
	issueIDs := make(map[string]struct{}, len(manifest.UpstreamIssues))
	assignedDifferences := make(map[string]string, len(differenceIDs))
	for _, issue := range manifest.UpstreamIssues {
		if !pdfaIDPattern.MatchString(issue.ID) {
			return fmt.Errorf("PDF Association upstream issue id %q is not a stable id", issue.ID)
		}
		if _, exists := issueIDs[issue.ID]; exists {
			return fmt.Errorf("duplicate PDF Association upstream issue id %q", issue.ID)
		}
		issueIDs[issue.ID] = struct{}{}
		if len(issue.DifferenceIDs) == 0 {
			return fmt.Errorf("PDF Association upstream issue %q has no differences", issue.ID)
		}
		for _, differenceID := range issue.DifferenceIDs {
			if _, exists := differenceIDs[differenceID]; !exists {
				return fmt.Errorf("PDF Association upstream issue %q references unknown difference %q", issue.ID, differenceID)
			}
			if previous, exists := assignedDifferences[differenceID]; exists {
				return fmt.Errorf("PDF Association difference %q is assigned to both upstream issues %q and %q", differenceID, previous, issue.ID)
			}
			assignedDifferences[differenceID] = issue.ID
		}
		if issue.State != "candidate" && issue.State != "existing" && issue.State != "submitted" {
			return fmt.Errorf("PDF Association upstream issue %q has unsupported state %q", issue.ID, issue.State)
		}
		if !validGitHubRepository(issue.Repository) {
			return fmt.Errorf("PDF Association upstream issue %q repository %q is not an owner/repository name", issue.ID, issue.Repository)
		}
		if !validHex(issue.CheckedCommit, 40) {
			return fmt.Errorf("PDF Association upstream issue %q checked_commit must be a 40-character hexadecimal hash", issue.ID)
		}
		if checkedAt, err := time.Parse("2006-01-02", issue.CheckedAt); err != nil || checkedAt.Format("2006-01-02") != issue.CheckedAt {
			return fmt.Errorf("PDF Association upstream issue %q checked_at must be an ISO 8601 date", issue.ID)
		}
		if issue.State == "candidate" && issue.URL != "" {
			return fmt.Errorf("PDF Association upstream issue %q candidate must not have an issue URL", issue.ID)
		}
		if issue.State != "candidate" && !validGitHubIssueURL(issue.Repository, issue.URL) {
			return fmt.Errorf("PDF Association upstream issue %q in state %q requires an issue URL in repository %q", issue.ID, issue.State, issue.Repository)
		}
		if issue.Title == "" || issue.Summary == "" || issue.Reproduction == "" || issue.ActualBehavior == "" || issue.ExpectedBehavior == "" {
			return fmt.Errorf("PDF Association upstream issue %q has incomplete draft details", issue.ID)
		}
	}
	for differenceID := range differenceIDs {
		if _, exists := assignedDifferences[differenceID]; !exists {
			return fmt.Errorf("PDF Association difference %q has no upstream issue record", differenceID)
		}
	}
	return nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validHex(value string, size int) bool {
	if len(value) != size || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validGitHubRepository(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
		for _, char := range part {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.", char) {
				return false
			}
		}
	}
	return true
}

func validGitHubIssueURL(repository, value string) bool {
	prefix := "https://github.com/" + repository + "/issues/"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	number := strings.TrimPrefix(value, prefix)
	parsed, err := strconv.Atoi(number)
	return err == nil && parsed > 0 && strconv.Itoa(parsed) == number
}

func validPDFAPath(value string) bool {
	return value != "" && value != "." && filepath.IsLocal(value) && filepath.ToSlash(value) == value && path.Clean(value) == value
}

func loadPDFAManifest() (PDFAManifest, error) {
	return parsePDFAManifest(embeddedPDFAManifest)
}

func PDFAFixtureByID(id string) (PDFAFixture, error) {
	manifest, err := loadPDFAManifest()
	if err != nil {
		return PDFAFixture{}, err
	}
	for _, fixture := range manifest.Fixtures {
		if fixture.ID == id {
			return fixture, nil
		}
	}
	return PDFAFixture{}, fmt.Errorf("unknown PDF Association fixture id %q", id)
}

func PDFAManifestValue() (PDFAManifest, error) {
	return loadPDFAManifest()
}

func pdfaFixtureDir(root string) string {
	if dir := os.Getenv(pdfaFixtureDirEnv); dir != "" {
		if filepath.IsAbs(dir) {
			return filepath.Clean(dir)
		}
		return filepath.Clean(filepath.Join(root, dir))
	}
	return filepath.Join(filepath.Dir(root), "pdf-association-fixtures")
}

func resolvePDFAPath(root string, fixture PDFAFixture) (string, error) {
	return resolvePDFAFile(root, fixture.Source, fixture.Path, fixture.SHA256, fixture.ID)
}

func resolvePDFAExclusion(root string, exclusion PDFAExclusion) (string, error) {
	return resolvePDFAFile(root, exclusion.Source, exclusion.Path, exclusion.SHA256, "excluded "+exclusion.Path)
}

func resolvePDFAFile(root, source, relativePath, wantSHA256, label string) (string, error) {
	filePath := filepath.Join(pdfaFixtureDir(root), source, filepath.FromSlash(relativePath))
	info, err := os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%w: %q at %q; run make pdfa-fixtures-pull", errPDFAFixtureUnavailable, label, filePath)
	}
	if err != nil {
		return "", fmt.Errorf("inspect PDF Association fixture %q: %w", filePath, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("PDF Association fixture %q is not a regular file", filePath)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open PDF Association fixture %q: %w", filePath, err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return "", fmt.Errorf("hash PDF Association fixture %q: %w", filePath, copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close PDF Association fixture %q: %w", filePath, closeErr)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != wantSHA256 {
		return "", fmt.Errorf("PDF Association fixture %q SHA-256 = %s, want %s", label, actual, wantSHA256)
	}
	return filePath, nil
}

func PDFAPath(t testTB, id string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("locate PDF Association corpus: %v", err)
		return ""
	}
	root, err := findModuleRoot(cwd)
	if err != nil {
		t.Fatalf("locate PDF Association corpus: %v", err)
		return ""
	}
	fixture, err := PDFAFixtureByID(id)
	if err != nil {
		t.Fatalf("PDF Association fixture: %v", err)
		return ""
	}
	resolved, err := resolvePDFAPath(root, fixture)
	if err == nil {
		return resolved
	}
	if errors.Is(err, errPDFAFixtureUnavailable) {
		if os.Getenv(fetchPDFAFixturesEnv) != "1" {
			t.Skipf("%v; set %s=1 to fetch it automatically", err, fetchPDFAFixturesEnv)
			return ""
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if fetchErr := syncPDFASourceFrom(ctx, root, fixture.Source); fetchErr != nil {
			t.Fatalf("fetch PDF Association fixture %q: %v", id, fetchErr)
			return ""
		}
		resolved, err = resolvePDFAPath(root, fixture)
		if err != nil {
			t.Fatalf("required PDF Association fixture after fetch: %v", err)
			return ""
		}
		return resolved
	}
	t.Fatalf("required PDF Association fixture: %v", err)
	return ""
}
