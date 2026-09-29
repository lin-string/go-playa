package engineeringcheck

import (
	"go/build"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheckSource(t *testing.T) {
	tests := []struct {
		name, path, source string
		want               []string
	}{
		{"context", "sample.go", `package sample; import "context"; func run(ctx context.Context) { if ctx == nil {} }`, []string{"nil-context"}},
		{"context alias reverse compound", "sample.go", `package sample; import c "context"; func Run(ctx c.Context) { if true && nil != (ctx) {} }`, []string{"nil-context"}},
		{"dot import context", "sample.go", `package sample; import . "context"; func run(ctx Context) { if ctx != nil {} }`, []string{"nil-context"}},
		{"context type alias", "sample.go", `package sample; import "context"; type Ctx = context.Context; func run(ctx Ctx) { if ctx == nil {} }`, []string{"nil-context"}},
		{"context anonymous parameter", "sample.go", `package sample; import "context"; var run = func(ctx context.Context) { if ctx == nil {} }`, []string{"nil-context"}},
		{"required callback", "sample.go", `package sample; func Run(fn func()) { if nil == fn {} }`, []string{"nil-callback"}},
		{"named callback", "sample.go", `package sample; type Callback func(); func Run(fn Callback) { if fn != nil {} }`, []string{"nil-callback"}},
		{"generic callback", "sample.go", `package sample; type Callback[T any] func(T); func Run(fn Callback[int]) { if fn == nil {} }`, []string{"nil-callback"}},
		{"generic callback two arguments", "sample.go", `package sample; type Callback[A, B any] func(A, B); func Run(fn Callback[int, string]) { if fn == nil {} }`, []string{"nil-callback"}},
		{"type parameter shadows callback", "sample.go", `package sample; type Callback func(); func Run[Callback ~*int](value Callback) { if value == nil {} }`, nil},
		{"receiver type parameter shadows callback", "sample.go", `package sample; type Callback func(); type T[Callback ~*int] struct{}; func (r T[Callback]) Run(value Callback) { if value == nil {} }`, nil},
		{"receiver in closure", "sample.go", `package sample; type T struct{}; func (r *T) Run() func() { return func() { if r == nil {} } }`, []string{"nil-receiver"}},
		{"receiver fields optional", "sample.go", `package sample; type T struct { other *T }; func (r *T) Run() { if r.other == nil {} }`, nil},
		{"optional helper callback", "sample.go", `package sample; func run(resolve func()) { if resolve != nil {} }`, nil},
		{"optional functional options", "sample.go", `package sample; type Option func(); func Run(options ...Option) { for _, option := range options { if option != nil {} } }`, nil},
		{"documented stream resolver", "pdftypes/stream.go", `package pdftypes; type Stream struct{}; func (s Stream) DecodedBufferWithResolver(resolve func()) { if resolve == nil {} }`, nil},
		{"resolver name not an exemption", "sample.go", `package sample; func Run(resolve func()) { if resolve == nil {} }`, []string{"nil-callback"}},
		{"receiver shadow", "sample.go", `package sample; type T struct{}; func (r *T) Run() { { r := (*T)(nil); if r == nil {} } }`, nil},
		{"context closure shadow", "sample.go", `package sample; import "context"; func run(ctx context.Context) { f := func(ctx *int) { if ctx == nil {} }; _ = f }`, nil},
		{"callback shadow then outer", "sample.go", `package sample; func Run(fn func()) { { fn := (*int)(nil); if fn == nil {} }; if fn == nil {} }`, []string{"nil-callback"}},
		{"nil identifier shadow", "sample.go", `package sample; import "context"; func run(ctx context.Context) { nil := ctx; if ctx == nil {} }`, nil},
		{"unexported receiver", "sample.go", `package sample; type T struct{}; func (r *T) run() { if r == nil {} }`, nil},
		{"blank function anchor", "sample.go", `package sample; func unused() {}; var _ = unused`, []string{"dead-code-anchor"}},
		{"typed blank function anchor", "sample.go", `package sample; func unused() {}; var _ func() = unused`, []string{"dead-code-anchor"}},
		{"typed generic function anchor", "sample.go", `package sample; func unused[T any](value T) {}; var _ func(int) = unused[int]`, []string{"dead-code-anchor"}},
		{"typed generic function two arguments", "sample.go", `package sample; func unused[A, B any](a A, b B) {}; var _ func(int, string) = unused[int, string]`, []string{"dead-code-anchor"}},
		{"named typed blank function anchor", "sample.go", `package sample; type Callback func(); func unused() {}; var _ Callback = unused`, []string{"dead-code-anchor"}},
		{"typed blank method anchor", "sample.go", `package sample; type T struct{}; func (*T) unused() {}; var _ func(*T) = (*T).unused`, []string{"dead-code-anchor"}},
		{"any typed function anchor", "sample.go", `package sample; func unused() {}; var _ any = unused`, []string{"dead-code-anchor"}},
		{"genuine function interface assertion", "sample.go", `package sample; type F func(); func (F) Run() {}; type I interface{ Run() }; func unused() {}; var _ I = F(unused)`, nil},
		{"blank method anchor", "sample.go", `package sample; type T struct{}; func (r *T) unused() {}; var _ = (*T).unused`, []string{"dead-code-anchor"}},
		{"blank method value anchor", "sample.go", `package sample; type T struct{}; func (r *T) unused() {}; var _ = (*T)(nil).unused`, []string{"dead-code-anchor"}},
		{"blank field reference allowed", "sample.go", `package sample; type T struct{ field int }; var _ = (&T{}).field`, nil},
		{"interface assertion allowed", "sample.go", `package sample; type I interface{ Run() }; type T struct{}; func (*T) Run() {}; var _ I = (*T)(nil)`, nil},
		{"blank call allowed", "sample.go", `package sample; func run() int { return 0 }; var _ = run()`, nil},
		{"local discard allowed", "sample.go", `package sample; func unused() {}; func run() { _ = unused }`, nil},
		{"generated skipped", "sample.go", "// Code generated by test. DO NOT EDIT.\npackage sample\nfunc Run(fn func()) { if fn == nil {} }", nil},
		{"tests skipped", "sample_test.go", `this is not valid Go`, nil},
		{"parse fails closed", "sample.go", `package sample; func broken(`, []string{"parse-error"}},
		{"invalid build constraint fails closed", "sample.go", "//go:build linux &&\n\npackage sample\n", []string{"parse-error"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostics := CheckSource(tt.path, []byte(tt.source))
			var rules []string
			for _, diagnostic := range diagnostics {
				rules = append(rules, diagnostic.Rule)
				if diagnostic.Path != tt.path || diagnostic.Line < 1 {
					t.Fatalf("invalid diagnostic location: %+v", diagnostic)
				}
			}
			if !reflect.DeepEqual(rules, tt.want) {
				t.Fatalf("rules = %v, want %v (diagnostics: %+v)", rules, tt.want, diagnostics)
			}
		})
	}
}

func TestScanTypeVariantsDoNotOverwriteOneAnother(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"type_linux.go":   "package sample\ntype Callback func()\nfunc Run(fn Callback) { if fn == nil {} }\n",
		"type_windows.go": "package sample\ntype Callback *int\nfunc Run(fn Callback) { if fn == nil {} }\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	diagnostics, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []Diagnostic{{Path: "type_linux.go", Line: 3, Rule: "nil-callback"}}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics = %+v, want %+v", diagnostics, want)
	}
}

func TestScanResolvesOnlyCompatibleBuildVariants(t *testing.T) {
	cases := []struct {
		name, callbackFile, pointerFile, runFile string
		callbackTags, pointerTags, runTags       string
		wantRule                                 bool
	}{
		{"windows pointer", "type_linux.go", "type_windows.go", "run_windows.go", "", "", "", false},
		{"windows pointer dotted linux filename", "type_linux.extra.go", "type_windows.go", "run_windows.go", "", "", "", false},
		{"linux pointer dotted windows filename", "type_windows.extra.go", "type_linux.go", "run_linux.go", "", "", "", false},
		{"arm64 pointer dotted amd64 filename", "type_amd64.extra.go", "type_arm64.go", "run_arm64.go", "", "", "", false},
		{"linux callback", "type_linux.go", "type_windows.go", "run_linux.go", "", "", "", true},
		{"linux pointer reverse", "type_windows.go", "type_linux.go", "run_linux.go", "", "", "", false},
		{"windows callback reverse", "type_windows.go", "type_linux.go", "run_windows.go", "", "", "", true},
		{"openbsd pointer", "type_freebsd.go", "type_openbsd.go", "run_openbsd.go", "", "", "", false},
		{"arm64 pointer", "type_amd64.go", "type_arm64.go", "run_arm64.go", "", "", "", false},
		{"os arch pointer", "type_linux_amd64.go", "type_windows_arm64.go", "run_windows_arm64.go", "", "", "", false},
		{"go build pointer", "callback.go", "pointer.go", "run.go", "//go:build linux\n\n", "//go:build windows\n\n", "//go:build windows\n\n", false},
		{"go build callback", "callback.go", "pointer.go", "run.go", "//go:build linux\n\n", "//go:build windows\n\n", "//go:build linux\n\n", true},
		{"custom tag pointer", "callback.go", "pointer.go", "run.go", "//go:build enterprise\n\n", "//go:build !enterprise\n\n", "//go:build !enterprise\n\n", false},
		{"compound tag pointer", "callback.go", "pointer.go", "run.go", "//go:build (linux && arm64) || (windows && amd64)\n\n", "//go:build windows && arm64\n\n", "//go:build windows && arm64\n\n", false},
		{"android includes linux", "type_linux.go", "type_windows.go", "run_android.go", "", "", "", true},
		{"unix excludes windows", "callback.go", "type_windows.go", "run_windows.go", "//go:build unix\n\n", "", "", false},
		{"legacy tags pointer", "callback.go", "pointer.go", "run.go", "// +build linux\n\n", "// +build windows\n\n", "// +build windows\n\n", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				test.callbackFile: test.callbackTags + "package sample\ntype Callback func()\n",
				test.pointerFile:  test.pointerTags + "package sample\ntype Callback *int\n",
				test.runFile:      test.runTags + "package sample\nfunc Run(fn Callback) { if fn == nil {} }\n",
			}
			for path, source := range files {
				if err := os.WriteFile(filepath.Join(root, path), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			diagnostics, err := Scan(root)
			if err != nil {
				t.Fatal(err)
			}
			if test.wantRule {
				if len(diagnostics) != 1 || diagnostics[0].Rule != "nil-callback" || diagnostics[0].Path != test.runFile {
					t.Fatalf("wanted run callback diagnostic, got %+v", diagnostics)
				}
			} else if len(diagnostics) != 0 {
				t.Fatalf("incompatible callback declaration caused false diagnostic: %+v", diagnostics)
			}
		})
	}
}

func TestFilenameConstraintMatchesGoBuild(t *testing.T) {
	root := t.TempDir()
	names := []string{
		"type_linux.extra.go", "type_windows.extra.go", "type_amd64.extra.go",
		"type_linux_amd64.extra.go", "type.extra_linux.go", "type.extra_windows.go",
		"type.extra_linux_amd64.go", "linux.go", "type_linux.go",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package sample\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, platform := range []struct{ os, arch string }{{"linux", "amd64"}, {"windows", "amd64"}, {"linux", "arm64"}, {"android", "arm64"}} {
			t.Run(name+"/"+platform.os+"/"+platform.arch, func(t *testing.T) {
				context := build.Default
				context.GOOS, context.GOARCH = platform.os, platform.arch
				want, err := context.MatchFile(root, name)
				if err != nil {
					t.Fatal(err)
				}
				expr := filenameConstraint(name)
				got := expr == nil || expr.Eval(func(tag string) bool { return platformTag(tag, platform.os, platform.arch, "gc") })
				if got != want {
					t.Fatalf("filenameConstraint = %v, Go build.MatchFile = %v", got, want)
				}
			})
		}
	}
}

func TestScanDotBeforePlatformSuffixIsNotPlatformConstrained(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"type.extra_linux.go": "package sample\ntype Callback func()\n",
		"run_windows.go":      "package sample\nfunc Run(fn Callback) { if fn == nil {} }\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	diagnostics, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []Diagnostic{{Path: "run_windows.go", Line: 2, Rule: "nil-callback"}}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics = %+v, want %+v", diagnostics, want)
	}
}

func TestOptionalCallbackExceptionsAreCurrent(t *testing.T) {
	for _, exception := range optionalCallbacks {
		path := filepath.Join("..", "..", filepath.FromSlash(exception.path))
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, diagnostics := parseSource(exception.path, source, token.NewFileSet())
		if len(diagnostics) != 0 || file.file == nil || !optionalCallbackExists(file.file, exception) {
			t.Fatalf("optional callback exception is stale: %+v", exception)
		}
	}
}

func TestOptionalCallbackRegistryValidation(t *testing.T) {
	exception := optionalCallbackSpec{path: "sample.go", receiver: "T", function: "Run", parameter: "resolve"}
	cases := []struct {
		source string
		want   bool
	}{
		{`package sample; type T struct{}; func (T) Run(resolve func()) {}`, true},
		{`package sample; type T struct{}; func (T) Renamed(resolve func()) {}`, false},
		{`package sample; type T struct{}; func (T) Run(renamed func()) {}`, false},
		{`package sample; type T struct{}; func (T) Run(resolve int) {}`, false},
		{`package sample; type T struct{}; func Run(resolve func()) {}`, false},
		{`package sample; type Renamed struct{}; func (Renamed) Run(resolve func()) {}`, false},
	}
	for _, test := range cases {
		file, diagnostics := parseSource(exception.path, []byte(test.source), token.NewFileSet())
		if len(diagnostics) != 0 {
			t.Fatal(diagnostics)
		}
		if got := optionalCallbackExists(file.file, exception); got != test.want {
			t.Fatalf("optionalCallbackExists(%q) = %v, want %v", test.source, got, test.want)
		}
	}
}

func TestScanChecksAllBuildVariants(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"run_linux.go", "run_darwin.go"} {
		source := "package sample\nfunc Run(fn func()) { if fn == nil {} }\n"
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	diagnostics, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("all build variants must be checked, got %+v", diagnostics)
	}
}

func TestScanIncludesUntrackedSourcesAndSortsDiagnostics(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"z.go":                 `package sample; func Run(fn func()) { if fn == nil {} }`,
		"a.go":                 "package sample\nimport \"context\"\nfunc run(ctx context.Context) { if ctx == nil {} }\n",
		"types.go":             `package sample; type T struct{}; type Callback func(); func unused() {}; func (*T) unused() {}`,
		"anchors.go":           `package sample; var _ = (*T).unused; var _ = unused; func Exported(fn Callback) { if fn == nil {} }`,
		"sample_test.go":       "invalid test source",
		".compat-cache/bad.go": "invalid cache source",
		"vendor/bad.go":        "invalid dependency source",
	}
	for path, source := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	diagnostics, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, diagnostic := range diagnostics {
		got = append(got, diagnostic.String())
	}
	want := []string{"a.go:3:nil-context", "anchors.go:1:dead-code-anchor", "anchors.go:1:dead-code-anchor", "anchors.go:1:nil-callback", "z.go:1:nil-callback"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostics = %v, want %v", got, want)
	}
}
