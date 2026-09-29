// Package testfixture locates checked-in and manifest-pinned public PDF fixtures for tests.
package testfixture

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	publicFixtureDirEnv = "GO_PLAYA_PUBLIC_FIXTURE_DIR"
	lfsPointerPrefix    = "version https://git-lfs.github.com/spec/v1\n"
)

var errPublicFixtureUnavailable = errors.New("public PDF test fixture unavailable")

type testTB interface {
	Helper()
	Fatalf(format string, args ...any)
	Skipf(format string, args ...any)
}

// Path returns the path to a required PDF fixture. Small fixtures are stored in
// the repository and are always required. Large public fixtures are resolved
// from the pinned public corpus and skipped until explicitly downloaded.
func Path(t testTB, name string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("locate PDF test corpus: %v", err)
		return ""
	}
	root, err := findModuleRoot(cwd)
	if err != nil {
		t.Fatalf("locate PDF test corpus: %v", err)
		return ""
	}
	return pathFromRoot(t, root, name)
}

// Dir returns the directory containing the small, checked-in PDF fixtures.
func Dir(t testTB) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("locate PDF test fixtures: %v", err)
		return ""
	}
	root, err := findModuleRoot(cwd)
	if err != nil {
		t.Fatalf("locate PDF test fixtures: %v", err)
		return ""
	}
	dir := filepath.Join(root, "testdata", "files")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("required PDF test fixture directory %q: %v", dir, err)
		return ""
	}
	return dir
}

func pathFromRoot(t testTB, root, name string) string {
	t.Helper()
	path, err := resolvePath(root, name)
	if err == nil {
		return path
	}
	if !errors.Is(err, errPublicFixtureUnavailable) {
		t.Fatalf("required PDF test fixture: %v", err)
		return ""
	}
	t.Skipf("%v; run make public-fixtures-pull", err)
	return ""
}

func resolvePath(root, name string) (string, error) {
	if name == "" || name == "." || filepath.Base(name) != name {
		return "", fmt.Errorf("fixture name %q must be a basename", name)
	}
	dir := filepath.Join(root, "testdata", "files")
	isPublic, err := isPublicFixture(root, name)
	if err != nil {
		return "", err
	}
	if isPublic {
		dir = publicFixtureDir(root)
	}
	path := filepath.Join(dir, name)
	ready, err := fixtureMaterialized(path)
	if err != nil {
		return "", fmt.Errorf("fixture %q: %w", path, err)
	}
	if ready {
		return path, nil
	}
	if isPublic {
		return "", fmt.Errorf("%w: %q at %q; run make public-fixtures-pull", errPublicFixtureUnavailable, name, path)
	}
	return "", fmt.Errorf("fixture %q: %w", path, os.ErrNotExist)
}

func publicFixtureDir(root string) string {
	if dir := os.Getenv(publicFixtureDirEnv); dir != "" {
		if filepath.IsAbs(dir) {
			return filepath.Clean(dir)
		}
		return filepath.Clean(filepath.Join(root, dir))
	}
	return filepath.Join(root, ".compat-cache", "public-corpus")
}

func isPublicFixture(root, name string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(root, "compat", "public_corpus.json"))
	if err != nil {
		return false, fmt.Errorf("read public PDF manifest: %w", err)
	}
	var manifest struct {
		Fixtures []struct {
			ID string `json:"id"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false, fmt.Errorf("decode public PDF manifest: %w", err)
	}
	for _, fixture := range manifest.Fixtures {
		if fixture.ID+".pdf" == name {
			return true, nil
		}
	}
	return false, nil
}

func fixtureMaterialized(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	prefix := make([]byte, len(lfsPointerPrefix))
	n, readErr := io.ReadFull(file, prefix)
	closeErr := file.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return false, readErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	return !strings.HasPrefix(string(prefix[:n]), lfsPointerPrefix), nil
}

func findModuleRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil && info.Mode().IsRegular() {
			return dir, nil
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above %q", start)
		}
		dir = parent
	}
}
