package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/lin-string/go-playa/internal/testfixture"
)

type compatDifferenceManifest struct {
	SchemaVersion  int                             `json:"schema_version"`
	Records        []compatDifferenceRecord        `json:"records"`
	UpstreamIssues []testfixture.PDFAUpstreamIssue `json:"upstream_issues,omitempty"`
}

type compatDifferenceRecord struct {
	ID                string     `json:"id"`
	Fixture           string     `json:"fixture"`
	FixtureSHA256     string     `json:"fixture_sha256"`
	Spaces            []string   `json:"spaces"`
	Sections          []string   `json:"sections"`
	AlternateSections [][]string `json:"alternate_sections,omitempty"`
	PathPattern       string     `json:"path_pattern"`
	DifferenceCount   int        `json:"difference_count"`
	DifferencesSHA256 string     `json:"differences_sha256"`
	UpstreamIssue     string     `json:"upstream_issue"`
	Reason            string     `json:"reason"`
}

var compatDifferenceIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func loadCompatDifferenceManifest() (compatDifferenceManifest, error) {
	path, err := repositoryFile("compat", "known_differences.json")
	if err != nil {
		return compatDifferenceManifest{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return compatDifferenceManifest{}, fmt.Errorf("read compatibility difference manifest: %w", err)
	}
	return parseCompatDifferenceManifest(data)
}

func parseCompatDifferenceManifest(data []byte) (compatDifferenceManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest compatDifferenceManifest
	if err := decoder.Decode(&manifest); err != nil {
		return compatDifferenceManifest{}, fmt.Errorf("decode compatibility difference manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON value")
		}
		return compatDifferenceManifest{}, fmt.Errorf("decode compatibility difference manifest: %w", err)
	}
	if err := validateCompatDifferenceManifest(manifest); err != nil {
		return compatDifferenceManifest{}, err
	}
	return manifest, nil
}

func validateCompatDifferenceManifest(manifest compatDifferenceManifest) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("compatibility difference manifest schema_version = %d, want 1", manifest.SchemaVersion)
	}
	pdfa, err := testfixture.PDFAManifestValue()
	if err != nil {
		return err
	}
	issues := make(map[string]testfixture.PDFAUpstreamIssue, len(pdfa.UpstreamIssues))
	for _, issue := range pdfa.UpstreamIssues {
		issues[issue.ID] = issue
	}
	recordIssues := make(map[string]string, len(manifest.Records))
	for _, record := range manifest.Records {
		recordIssues[record.ID] = record.UpstreamIssue
	}
	for _, issue := range manifest.UpstreamIssues {
		if !compatDifferenceIDPattern.MatchString(issue.ID) {
			return fmt.Errorf("invalid corpus upstream issue id %q", issue.ID)
		}
		if _, exists := issues[issue.ID]; exists {
			return fmt.Errorf("duplicate corpus upstream issue %q", issue.ID)
		}
		urlPattern := `^https://github\.com/` + regexp.QuoteMeta(issue.Repository) + `/issues/[1-9][0-9]*$`
		if (issue.State != "existing" && issue.State != "submitted") ||
			!regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(issue.Repository) ||
			!regexp.MustCompile(urlPattern).MatchString(issue.URL) {
			return fmt.Errorf("corpus upstream issue %q requires a confirmed issue URL in its repository", issue.ID)
		}
		if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(issue.CheckedCommit) {
			return fmt.Errorf("corpus upstream issue %q requires a source commit", issue.ID)
		}
		checkedAt, err := time.Parse("2006-01-02", issue.CheckedAt)
		if err != nil || checkedAt.Format("2006-01-02") != issue.CheckedAt {
			return fmt.Errorf("corpus upstream issue %q requires a checked date", issue.ID)
		}
		if issue.Title == "" || issue.Summary == "" || issue.Reproduction == "" || issue.ActualBehavior == "" || issue.ExpectedBehavior == "" {
			return fmt.Errorf("corpus upstream issue %q has incomplete report details", issue.ID)
		}
		if len(issue.DifferenceIDs) == 0 {
			return fmt.Errorf("corpus upstream issue %q has no differences", issue.ID)
		}
		assigned := make(map[string]bool, len(issue.DifferenceIDs))
		for _, id := range issue.DifferenceIDs {
			if assigned[id] || recordIssues[id] != issue.ID {
				return fmt.Errorf("corpus upstream issue %q references an unknown, duplicate or differently owned difference %q", issue.ID, id)
			}
			assigned[id] = true
		}
		for id, owner := range recordIssues {
			if owner == issue.ID && !assigned[id] {
				return fmt.Errorf("corpus upstream issue %q does not list difference %q", issue.ID, id)
			}
		}
		issues[issue.ID] = issue
	}
	ids := make(map[string]bool, len(manifest.Records))
	for _, record := range manifest.Records {
		if !compatDifferenceIDPattern.MatchString(record.ID) {
			return fmt.Errorf("compatibility difference id %q is not a stable id", record.ID)
		}
		if ids[record.ID] {
			return fmt.Errorf("duplicate compatibility difference id %q", record.ID)
		}
		ids[record.ID] = true
		issue, ok := issues[record.UpstreamIssue]
		if !ok {
			return fmt.Errorf("compatibility difference %q references unknown upstream issue %q", record.ID, record.UpstreamIssue)
		}
		if (issue.State != "existing" && issue.State != "submitted") || issue.URL == "" {
			return fmt.Errorf("compatibility difference %q references upstream issue %q in state %q without a confirmed URL", record.ID, record.UpstreamIssue, issue.State)
		}
		if record.Fixture == "" || !validDigest(record.FixtureSHA256) {
			return fmt.Errorf("compatibility difference %q must declare a fixture name and SHA-256", record.ID)
		}
		if len(record.Spaces) == 0 || record.DifferenceCount <= 0 || !validDigest(record.DifferencesSHA256) {
			return fmt.Errorf("compatibility difference %q must declare spaces, sections, a positive difference count, and an aggregate SHA-256", record.ID)
		}
		sectionGroups := make([][]string, 0, 1+len(record.AlternateSections))
		sectionGroups = append(sectionGroups, record.Sections)
		sectionGroups = append(sectionGroups, record.AlternateSections...)
		seenSectionGroups := make(map[string]bool, len(sectionGroups))
		for _, group := range sectionGroups {
			if len(group) == 0 {
				return fmt.Errorf("compatibility difference %q has an empty section group", record.ID)
			}
			for _, section := range group {
				if strings.TrimSpace(section) == "" {
					return fmt.Errorf("compatibility difference %q has an empty section name", record.ID)
				}
			}
			key := strings.Join(group, "\x00")
			if seenSectionGroups[key] {
				return fmt.Errorf("compatibility difference %q repeats section group %q", record.ID, strings.Join(group, ","))
			}
			seenSectionGroups[key] = true
		}
		if _, err := regexp.Compile(record.PathPattern); err != nil || record.PathPattern == "" {
			return fmt.Errorf("compatibility difference %q has invalid path_pattern %q: %v", record.ID, record.PathPattern, err)
		}
		if strings.TrimSpace(record.Reason) == "" {
			return fmt.Errorf("compatibility difference %q must declare a reason", record.ID)
		}
		seenSpaces := map[string]bool{}
		for _, space := range record.Spaces {
			if space != "page" && space != "screen" && space != "default" {
				return fmt.Errorf("compatibility difference %q has unsupported coordinate space %q", record.ID, space)
			}
			if seenSpaces[space] {
				return fmt.Errorf("compatibility difference %q repeats coordinate space %q", record.ID, space)
			}
			seenSpaces[space] = true
		}
	}
	return nil
}

func reconcileCompatKnownDifferences(task compareTask, differences []string, records []compatDifferenceRecord) ([]string, error) {
	type scopedRecord struct {
		record  compatDifferenceRecord
		pattern *regexp.Regexp
		actual  []string
	}
	var scoped []*scopedRecord
	for _, record := range records {
		if record.FixtureSHA256 != task.fixtureSHA256 || !slices.Contains(record.Spaces, task.space) || !compatDifferenceSectionsMatch(record, task.sections) {
			continue
		}
		pattern, err := regexp.Compile(record.PathPattern)
		if err != nil {
			return nil, fmt.Errorf("compile compatibility difference %q path pattern: %w", record.ID, err)
		}
		scoped = append(scoped, &scopedRecord{record: record, pattern: pattern})
	}
	unknown := make([]string, 0)
	for _, difference := range differences {
		path := differencePath(difference)
		var owner *scopedRecord
		for _, candidate := range scoped {
			if candidate.pattern.MatchString(path) {
				if owner != nil {
					return nil, fmt.Errorf("compatibility difference path %q matches both %q and %q", path, owner.record.ID, candidate.record.ID)
				}
				owner = candidate
			}
		}
		if owner == nil {
			unknown = append(unknown, fmt.Sprintf("difference_sha256=%s path=%s", differenceDigest(difference), path))
			continue
		}
		owner.actual = append(owner.actual, difference)
	}
	if len(unknown) != 0 {
		shown := unknown
		if len(shown) > 32 {
			shown = shown[:32]
		}
		return nil, fmt.Errorf("unrecorded compatibility differences for %q SHA-256=%s [%s] sections=%s count=%d differences_sha256=%s (showing %d):\n%s", task.pdf, task.fixtureSHA256, task.space, strings.Join(task.sections, ","), len(differences), differenceSetDigest(differences), len(shown), strings.Join(shown, "\n"))
	}
	matched := make([]string, 0, len(scoped))
	for _, candidate := range scoped {
		if len(candidate.actual) == 0 {
			return nil, fmt.Errorf("known compatibility difference %q was not observed for %q SHA-256=%s [%s] sections=%s", candidate.record.ID, task.pdf, task.fixtureSHA256, task.space, strings.Join(task.sections, ","))
		}
		actualDigest := differenceSetDigest(candidate.actual)
		if len(candidate.actual) != candidate.record.DifferenceCount || actualDigest != candidate.record.DifferencesSHA256 {
			return nil, fmt.Errorf("known compatibility difference %q does not match for %q SHA-256=%s [%s] sections=%s: count=%d sha256=%s, want count=%d sha256=%s", candidate.record.ID, task.pdf, task.fixtureSHA256, task.space, strings.Join(task.sections, ","), len(candidate.actual), actualDigest, candidate.record.DifferenceCount, candidate.record.DifferencesSHA256)
		}
		matched = append(matched, candidate.record.ID)
	}
	slices.Sort(matched)
	return matched, nil
}

func compatDifferenceSectionsMatch(record compatDifferenceRecord, sections []string) bool {
	if slices.Equal(record.Sections, sections) {
		return true
	}
	for _, alternate := range record.AlternateSections {
		if slices.Equal(alternate, sections) {
			return true
		}
	}
	return false
}

func differenceDigest(difference string) string {
	digest := sha256.Sum256([]byte(difference))
	return hex.EncodeToString(digest[:])
}

func differenceSetDigest(differences []string) string {
	hash := sha256.New()
	var length [8]byte
	for _, difference := range differences {
		binary.BigEndian.PutUint64(length[:], uint64(len(difference)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(difference))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func differencePath(difference string) string {
	if index := strings.Index(difference, ": playa="); index >= 0 {
		return difference[:index]
	}
	if index := strings.IndexByte(difference, ':'); index >= 0 {
		return difference[:index]
	}
	return difference
}

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}
