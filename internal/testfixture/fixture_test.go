package testfixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func writePublicManifest(t *testing.T, root string) {
	t.Helper()
	t.Setenv(publicFixtureDirEnv, "")
	writeFixtureFile(t, filepath.Join(root, "compat", "public_corpus.json"), `{"fixtures":[{"id":"riscv-unprivileged"}]}`)
}

func TestResolvePathReturnsLocalFixture(t *testing.T) {
	root := t.TempDir()
	writePublicManifest(t, root)
	want := writeFixtureFile(t, filepath.Join(root, "testdata", "files", "form_simple.pdf"), "%PDF-1.7\n")
	got, err := resolvePath(root, "form_simple.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolvePath = %q, want %q", got, want)
	}
}

func TestResolvePathUsesPublicCorpusForLargeFixture(t *testing.T) {
	root := t.TempDir()
	writePublicManifest(t, root)
	want := writeFixtureFile(t, filepath.Join(root, ".compat-cache", "public-corpus", "riscv-unprivileged.pdf"), "%PDF-1.7\n")
	got, err := resolvePath(root, "riscv-unprivileged.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolvePath = %q, want %q", got, want)
	}
}

func TestResolvePathUsesPublicDirectoryOverride(t *testing.T) {
	root := t.TempDir()
	writePublicManifest(t, root)
	dir := t.TempDir()
	t.Setenv(publicFixtureDirEnv, dir)
	want := writeFixtureFile(t, filepath.Join(dir, "riscv-unprivileged.pdf"), "%PDF-1.7\n")
	got, err := resolvePath(root, "riscv-unprivileged.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolvePath = %q, want %q", got, want)
	}
}

func TestResolvePathReportsMissingPublicFixture(t *testing.T) {
	root := t.TempDir()
	writePublicManifest(t, root)
	_, err := resolvePath(root, "riscv-unprivileged.pdf")
	if !errors.Is(err, errPublicFixtureUnavailable) || !strings.Contains(err.Error(), "make public-fixtures-pull") {
		t.Fatalf("resolvePath error = %v, want public corpus setup instruction", err)
	}
}

func TestResolvePathRejectsUnknownAndTraversalNames(t *testing.T) {
	root := t.TempDir()
	writePublicManifest(t, root)
	if _, err := resolvePath(root, "unknown.pdf"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unknown fixture error = %v", err)
	}
	if _, err := resolvePath(root, "../riscv-unprivileged.pdf"); err == nil || !strings.Contains(err.Error(), "basename") {
		t.Fatalf("traversal error = %v", err)
	}
}

func TestPathFromRootSkipsMissingPublicFixture(t *testing.T) {
	root := t.TempDir()
	writePublicManifest(t, root)
	tb := &recordingTB{}
	if got := pathFromRoot(tb, root, "riscv-unprivileged.pdf"); got != "" || !tb.skipped || tb.failed {
		t.Fatalf("pathFromRoot = %q, skipped=%v failed=%v", got, tb.skipped, tb.failed)
	}
	if !strings.Contains(tb.message, "make public-fixtures-pull") {
		t.Fatalf("skip message = %q", tb.message)
	}
}

func TestWithFixtureLockSerializesCallers(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "fixture.lock")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	entered := make(chan struct{})
	release := make(chan struct{})
	var active atomic.Int32
	var peak atomic.Int32
	done := make(chan error, 2)
	run := func(block bool) {
		done <- withFixtureLock(ctx, lockPath, func() error {
			current := active.Add(1)
			for {
				old := peak.Load()
				if current <= old || peak.CompareAndSwap(old, current) {
					break
				}
			}
			if block {
				close(entered)
				<-release
			}
			active.Add(-1)
			return nil
		})
	}
	go run(true)
	<-entered
	go run(false)
	time.Sleep(75 * time.Millisecond)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := peak.Load(); got != 1 {
		t.Fatalf("peak concurrent lock holders = %d, want 1", got)
	}
}

func TestWithFixtureLockReclaimsStaleLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "fixture.lock")
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-fixtureLockStaleAfter - time.Minute)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}
	called := false
	if err := withFixtureLock(context.Background(), lockPath, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("lock callback was not called")
	}
}

func TestFindModuleRootUsesNearestGoMod(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "go.mod"), "module example.com/root\n")
	child := filepath.Join(root, "internal", "testfixture")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := findModuleRoot(child)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("findModuleRoot = %q, want %q", got, root)
	}
}

type recordingTB struct {
	skipped bool
	failed  bool
	message string
}

func (*recordingTB) Helper() {}

func (t *recordingTB) Fatalf(format string, args ...any) {
	t.failed = true
	t.message = fmt.Sprintf(format, args...)
}

func (t *recordingTB) Skipf(format string, args ...any) {
	t.skipped = true
	t.message = fmt.Sprintf(format, args...)
}

func writeFixtureFile(t *testing.T, path, contents string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
