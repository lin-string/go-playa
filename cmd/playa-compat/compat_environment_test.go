package main

import (
	"os"
	"strings"
	"testing"
)

func TestCompatEnvironmentInstallsPlayaCryptoExtra(t *testing.T) {
	project, err := os.ReadFile("../../compat/pyproject.toml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(project), `"playa-pdf[crypto]==`) {
		t.Fatal("compat environment does not install the Playa crypto extra required by encrypted corpus fixtures")
	}
}
