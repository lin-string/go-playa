package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestRunHelp(t *testing.T) {
	for _, option := range []string{"-h", "-help", "--help"} {
		t.Run(option, func(t *testing.T) {
			var output strings.Builder
			if err := run([]string{option}, &output); err != nil {
				t.Fatalf("help failed: %v", err)
			}
			for _, want := range []string{"Usage of playa:", "-text", "-pages"} {
				if !strings.Contains(output.String(), want) {
					t.Errorf("help %q does not contain %q", output.String(), want)
				}
			}
		})
	}
}

func TestRunUnknownFlagRemainsUsageError(t *testing.T) {
	var output strings.Builder
	err := run([]string{"--unknown-flag"}, &output)
	if !errors.Is(err, errUsage) || output.Len() != 0 {
		t.Fatalf("error = %v, output = %q; want usage error and no stdout", err, output.String())
	}
}

func TestRunTextMode(t *testing.T) {
	path := testfixture.Path(t, "acceptance_cjk_cid.pdf")
	var output strings.Builder
	if err := run([]string{"--text", "--pages", "0", path}, &output); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output.String()) == "" {
		t.Fatal("text output is empty")
	}
}

func TestRunRequiresExactlyOneMode(t *testing.T) {
	err := run([]string{"document.pdf"}, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "select exactly one mode") {
		t.Fatalf("error = %v", err)
	}
}
