package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	for _, option := range []string{"-h", "-help", "--help"} {
		t.Run(option, func(t *testing.T) {
			var output strings.Builder
			if err := run(context.Background(), t.TempDir(), []string{option}, &output); err != nil {
				t.Fatalf("help failed: %v", err)
			}
			for _, want := range []string{"Usage of playa-fixtures:", "-pdf-association", "-check"} {
				if !strings.Contains(output.String(), want) {
					t.Errorf("help %q does not contain %q", output.String(), want)
				}
			}
		})
	}
}

func TestRunUnknownFlagRemainsError(t *testing.T) {
	var output strings.Builder
	err := run(context.Background(), t.TempDir(), []string{"--unknown-flag"}, &output)
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") || output.Len() != 0 {
		t.Fatalf("error = %v, output = %q; want unknown flag error and no stdout", err, output.String())
	}
}

func TestRunRequiresPDFAssociationMode(t *testing.T) {
	err := run(context.Background(), "/workspace/go-playa", nil, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run without PDF Association mode succeeded")
	}
}

func TestRunSynchronizesPDFAssociationFixtures(t *testing.T) {
	oldSync := syncPDFAFixtures
	var gotStart string
	syncPDFAFixtures = func(_ context.Context, start string) error {
		gotStart = start
		return nil
	}
	t.Cleanup(func() { syncPDFAFixtures = oldSync })

	if err := run(context.Background(), "/workspace/go-playa", []string{"--pdf-association"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if gotStart != "/workspace/go-playa" {
		t.Fatalf("sync start = %q", gotStart)
	}
}

func TestRunChecksPDFAssociationFixturesWithoutSync(t *testing.T) {
	oldCheck := checkPDFAFixtures
	wantErr := errors.New("bad checkout")
	checkPDFAFixtures = func(context.Context, string) error { return wantErr }
	t.Cleanup(func() { checkPDFAFixtures = oldCheck })

	err := run(context.Background(), "/workspace/go-playa", []string{"--pdf-association", "--check"}, &bytes.Buffer{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("run error = %v, want %v", err, wantErr)
	}
}

func TestRunListsVerifiedCompatibilityPaths(t *testing.T) {
	oldPaths := pdfaPaths
	want := []string{"/fixtures/one.pdf", "/fixtures/two.pdf"}
	pdfaPaths = func(context.Context, string, bool) ([]string, error) { return want, nil }
	t.Cleanup(func() { pdfaPaths = oldPaths })
	var output bytes.Buffer

	if err := run(context.Background(), "/workspace/go-playa", []string{"--pdf-association", "--list-compat"}, &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "/fixtures/one.pdf\n/fixtures/two.pdf\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRunRejectsPDFAssociationOnlyFlagWithoutMode(t *testing.T) {
	err := run(context.Background(), "/workspace/go-playa", []string{"--list-compat"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run succeeded, want flag validation error")
	}
}
