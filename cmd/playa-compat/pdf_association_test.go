package main

import (
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestBuildPDFACompareTasksCarriesFixturePassword(t *testing.T) {
	cases := []testfixture.PDFACase{{
		Fixture: testfixture.PDFAFixture{
			ID:       "unicode-password",
			SHA256:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Password: "secret",
			Sections: []string{"document", "content.text"},
		},
		Path: "/fixtures/unicode-password.pdf",
	}}
	tasks := buildPDFACompareTasks(cases, []string{"page", "screen"})
	if len(tasks) != 2 {
		t.Fatalf("task count = %d, want 2", len(tasks))
	}
	for _, task := range tasks {
		if task.password != "secret" || !task.passwordSet {
			t.Fatalf("task password = %q, set=%v", task.password, task.passwordSet)
		}
		if task.fixtureID != "unicode-password" || task.pdf != "/fixtures/unicode-password.pdf" {
			t.Fatalf("task identity = fixture %q path %q", task.fixtureID, task.pdf)
		}
	}
}
