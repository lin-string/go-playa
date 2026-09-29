package fontdata_test

import (
	"testing"

	"github.com/lin-string/go-playa/fontdata"
)

func TestGlyphTextUsesAGLNameSyntax(t *testing.T) {
	cases := []struct {
		name string
		want string
		ok   bool
	}{
		{name: "Aacute", want: "\u00c1", ok: true},
		{name: "Aacute.alt", want: "\u00c1", ok: true},
		{name: "f_f_i", want: "ffi", ok: true},
		{name: "uni00410042", want: "AB", ok: true},
		{name: "uniD8000041", want: "A", ok: true},
		{name: "u1F600", want: "\U0001f600", ok: true},
		{name: "uD800", ok: false},
		{name: "not-a-glyph", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := fontdata.GlyphText(tc.name)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("GlyphText(%q) = %q, %v; want %q, %v", tc.name, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestGlyphTextDoesNotTreatUnknownSingleCharacterAsUnicode(t *testing.T) {
	for _, name := range []string{"?", "."} {
		if got, ok := fontdata.GlyphText(name); ok || got != "" {
			t.Fatalf("GlyphText(%q) = %q, %v; want an unmapped glyph name", name, got, ok)
		}
	}
}

func TestGlyphTextPreservesScopedLegacyAliases(t *testing.T) {
	cases := map[string]string{
		"applelogo":   "\uf8ff",
		"f_f":         "\ufb00",
		"f_f_i":       "\ufb03",
		"f_f_l":       "\ufb04",
		"f_i":         "\ufb01",
		"f_l":         "\ufb02",
		"hyphenminus": "-",
		"nbhyphen":    "\u2011",
	}
	for name, want := range cases {
		got, ok := fontdata.GlyphTextWithLegacyAliases(name)
		if !ok || got != want {
			t.Fatalf("GlyphText(%q) = %q, %v; want %q, true", name, got, ok, want)
		}
	}
}
