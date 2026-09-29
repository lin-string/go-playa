package testfixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type pdfaGitRunner func(context.Context, string, ...string) (string, error)

var runPDFAGitCommand pdfaGitRunner = runPDFAGit

// SyncPDFAFrom locates the go-playa module above start and checks out every
// declared PDF Association source at its manifest-pinned commit.
func SyncPDFAFrom(ctx context.Context, start string) error {
	root, err := findModuleRoot(start)
	if err != nil {
		return fmt.Errorf("locate go-playa module: %w", err)
	}
	manifest, err := loadPDFAManifest()
	if err != nil {
		return err
	}
	for _, source := range manifest.Sources {
		if err := syncPDFASource(ctx, root, source, fixturesForSource(manifest, source.Name), exclusionsForSource(manifest, source.Name), runPDFAGitCommand); err != nil {
			return err
		}
	}
	return nil
}

// CheckPDFAFrom verifies every PDF Association checkout and fixture without
// accessing the network or changing a checkout.
func CheckPDFAFrom(ctx context.Context, start string) error {
	root, err := findModuleRoot(start)
	if err != nil {
		return fmt.Errorf("locate go-playa module: %w", err)
	}
	manifest, err := loadPDFAManifest()
	if err != nil {
		return err
	}
	for _, source := range manifest.Sources {
		if err := checkPDFASource(ctx, root, source, fixturesForSource(manifest, source.Name), exclusionsForSource(manifest, source.Name), runPDFAGitCommand); err != nil {
			return err
		}
	}
	return nil
}

// PDFAPathsFrom returns verified local paths for all fixtures, optionally
// limiting the result to fixtures supported by the compatibility runner.
func PDFAPathsFrom(ctx context.Context, start string, compatibilityOnly bool) ([]string, error) {
	cases, err := PDFACasesFrom(ctx, start, compatibilityOnly)
	if err != nil {
		return nil, err
	}
	paths := make([]string, len(cases))
	for index, testCase := range cases {
		paths[index] = testCase.Path
	}
	return paths, nil
}

// PDFACasesFrom returns verified fixture metadata and local paths, optionally
// limiting the result to fixtures supported by the compatibility runner.
func PDFACasesFrom(ctx context.Context, start string, compatibilityOnly bool) ([]PDFACase, error) {
	root, err := findModuleRoot(start)
	if err != nil {
		return nil, fmt.Errorf("locate go-playa module: %w", err)
	}
	manifest, err := loadPDFAManifest()
	if err != nil {
		return nil, err
	}
	if err := checkPDFAFromManifest(ctx, root, manifest); err != nil {
		return nil, err
	}
	cases := make([]PDFACase, 0, len(manifest.Fixtures))
	for _, fixture := range manifest.Fixtures {
		if compatibilityOnly && !fixture.Compatibility {
			continue
		}
		fixturePath, err := resolvePDFAPath(root, fixture)
		if err != nil {
			return nil, err
		}
		cases = append(cases, PDFACase{Fixture: fixture, Path: fixturePath})
	}
	return cases, nil
}

func syncPDFASourceFrom(ctx context.Context, root, sourceName string) error {
	manifest, err := loadPDFAManifest()
	if err != nil {
		return err
	}
	for _, source := range manifest.Sources {
		if source.Name == sourceName {
			return syncPDFASource(ctx, root, source, fixturesForSource(manifest, source.Name), exclusionsForSource(manifest, source.Name), runPDFAGitCommand)
		}
	}
	return fmt.Errorf("unknown PDF Association source %q", sourceName)
}

func checkPDFAFromManifest(ctx context.Context, root string, manifest PDFAManifest) error {
	for _, source := range manifest.Sources {
		if err := checkPDFASource(ctx, root, source, fixturesForSource(manifest, source.Name), exclusionsForSource(manifest, source.Name), runPDFAGitCommand); err != nil {
			return err
		}
	}
	return nil
}

func fixturesForSource(manifest PDFAManifest, sourceName string) []PDFAFixture {
	var fixtures []PDFAFixture
	for _, fixture := range manifest.Fixtures {
		if fixture.Source == sourceName {
			fixtures = append(fixtures, fixture)
		}
	}
	return fixtures
}

func exclusionsForSource(manifest PDFAManifest, sourceName string) []PDFAExclusion {
	var exclusions []PDFAExclusion
	for _, exclusion := range manifest.Exclusions {
		if exclusion.Source == sourceName {
			exclusions = append(exclusions, exclusion)
		}
	}
	return exclusions
}

func syncPDFASource(ctx context.Context, root string, source PDFASource, fixtures []PDFAFixture, exclusions []PDFAExclusion, run pdfaGitRunner) error {
	baseDir := pdfaFixtureDir(root)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return fmt.Errorf("create PDF Association fixture directory: %w", err)
	}
	repoDir := filepath.Join(baseDir, source.Name)
	lockPath := filepath.Join(baseDir, "."+source.Name+".go-playa-fetch.lock")
	return withFixtureLock(ctx, lockPath, func() error {
		gitDir := filepath.Join(repoDir, ".git")
		if _, err := os.Stat(gitDir); errors.Is(err, os.ErrNotExist) {
			if _, repoErr := os.Stat(repoDir); repoErr == nil {
				return fmt.Errorf("PDF Association source path %q exists but is not a Git checkout", repoDir)
			} else if !errors.Is(repoErr, os.ErrNotExist) {
				return fmt.Errorf("inspect PDF Association source path %q: %w", repoDir, repoErr)
			}
			if _, err := run(ctx, baseDir, "-c", "http.version=HTTP/1.1", "clone", "--filter=blob:none", source.Repository, repoDir); err != nil {
				return fmt.Errorf("clone PDF Association source %q: %w", source.Name, err)
			}
		} else if err != nil {
			return fmt.Errorf("inspect PDF Association source %q: %w", source.Name, err)
		}
		if err := checkPDFARepositoryIdentity(ctx, repoDir, source, run, false); err != nil {
			return err
		}
		if _, err := run(ctx, "", "-C", repoDir, "cat-file", "-e", source.Commit+"^{commit}"); err != nil {
			if _, fetchErr := run(ctx, "", "-C", repoDir, "fetch", "--depth=1", "origin", source.Commit); fetchErr != nil {
				return fmt.Errorf("fetch pinned commit for PDF Association source %q: %w", source.Name, fetchErr)
			}
		}
		if _, err := run(ctx, "", "-C", repoDir, "checkout", "--detach", source.Commit); err != nil {
			return fmt.Errorf("check out pinned commit for PDF Association source %q: %w", source.Name, err)
		}
		return checkPDFASource(ctx, root, source, fixtures, exclusions, run)
	})
}

func checkPDFASource(ctx context.Context, root string, source PDFASource, fixtures []PDFAFixture, exclusions []PDFAExclusion, run pdfaGitRunner) error {
	repoDir := filepath.Join(pdfaFixtureDir(root), source.Name)
	gitDir := filepath.Join(repoDir, ".git")
	if info, err := os.Stat(gitDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: source %q at %q; run make pdfa-fixtures-pull", errPDFAFixtureUnavailable, source.Name, repoDir)
		}
		return fmt.Errorf("inspect PDF Association source %q: %w", source.Name, err)
	} else if !info.IsDir() {
		return fmt.Errorf("PDF Association source %q metadata %q is not a directory", source.Name, gitDir)
	}
	if err := checkPDFARepositoryIdentity(ctx, repoDir, source, run, true); err != nil {
		return err
	}
	for _, fixture := range fixtures {
		if _, err := resolvePDFAPath(root, fixture); err != nil {
			return err
		}
	}
	for _, exclusion := range exclusions {
		if _, err := resolvePDFAExclusion(root, exclusion); err != nil {
			return err
		}
	}
	return checkPDFASourceInventory(ctx, repoDir, source, fixtures, exclusions, run)
}

func checkPDFASourceInventory(ctx context.Context, repoDir string, source PDFASource, fixtures []PDFAFixture, exclusions []PDFAExclusion, run pdfaGitRunner) error {
	output, err := run(ctx, "", "-C", repoDir, "ls-tree", "-rz", "--name-only", "HEAD")
	if err != nil {
		return fmt.Errorf("list tracked files for PDF Association source %q: %w", source.Name, err)
	}
	want := make(map[string]struct{}, len(fixtures)+len(exclusions))
	for _, fixture := range fixtures {
		want[fixture.Path] = struct{}{}
	}
	for _, exclusion := range exclusions {
		want[exclusion.Path] = struct{}{}
	}
	seen := make(map[string]struct{}, len(want))
	for _, trackedPath := range strings.Split(output, "\x00") {
		if trackedPath == "" || !strings.EqualFold(filepath.Ext(trackedPath), ".pdf") {
			continue
		}
		trackedPath = filepath.ToSlash(trackedPath)
		if _, exists := want[trackedPath]; !exists {
			return fmt.Errorf("PDF Association source %q has unlisted PDF %q; add it as a fixture or audited exclusion", source.Name, trackedPath)
		}
		seen[trackedPath] = struct{}{}
	}
	for declaredPath := range want {
		if _, exists := seen[declaredPath]; !exists {
			return fmt.Errorf("PDF Association source %q manifest path %q is not a tracked PDF", source.Name, declaredPath)
		}
	}
	return nil
}

func checkPDFARepositoryIdentity(ctx context.Context, repoDir string, source PDFASource, run pdfaGitRunner, checkHead bool) error {
	origin, err := run(ctx, "", "-C", repoDir, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("read origin for PDF Association source %q: %w", source.Name, err)
	}
	if strings.TrimSpace(origin) != source.Repository {
		return fmt.Errorf("PDF Association source %q origin = %q, want %q", source.Name, strings.TrimSpace(origin), source.Repository)
	}
	status, err := run(ctx, "", "-C", repoDir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("inspect PDF Association source %q worktree: %w", source.Name, err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("PDF Association source %q has uncommitted changes; refusing to overwrite them", source.Name)
	}
	if !checkHead {
		return nil
	}
	head, err := run(ctx, "", "-C", repoDir, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read PDF Association source %q HEAD: %w", source.Name, err)
	}
	if strings.TrimSpace(head) != source.Commit {
		return fmt.Errorf("PDF Association source %q HEAD = %q, want pinned commit %s; run make pdfa-fixtures-pull", source.Name, strings.TrimSpace(head), source.Commit)
	}
	return nil
}

func runPDFAGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_LFS_SKIP_SMUDGE=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, message)
	}
	return string(output), nil
}
