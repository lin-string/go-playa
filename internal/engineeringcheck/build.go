package engineeringcheck

import (
	"go/build"
	"go/build/constraint"
	"path/filepath"
	"sort"
	"strings"
)

// These are Go's recognized filename suffixes, including historical targets
// still recognized by go/build. OS aliases are applied to suffixes and tags.
var goOperatingSystems = strings.Fields("aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos")
var goArchitectures = strings.Fields("386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm")
var unixOperatingSystems = strings.Fields("aix android darwin dragonfly freebsd hurd illumos ios linux netbsd openbsd solaris")

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func andConstraint(left, right constraint.Expr) constraint.Expr {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}
	return &constraint.AndExpr{X: left, Y: right}
}

func filenameConstraint(path string) constraint.Expr {
	// go/build.goodOSArchFile considers only the part before the first dot,
	// so type_linux.extra.go is Linux-only but type.extra_linux.go is not.
	name, _, _ := strings.Cut(filepath.Base(path), ".")
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return nil // linux.go is not an OS-constrained filename.
	}
	last := parts[len(parts)-1]
	if contains(goOperatingSystems, last) {
		return &constraint.TagExpr{Tag: last}
	}
	if contains(goArchitectures, last) {
		expr := constraint.Expr(&constraint.TagExpr{Tag: last})
		if len(parts) > 2 && contains(goOperatingSystems, parts[len(parts)-2]) {
			expr = andConstraint(expr, &constraint.TagExpr{Tag: parts[len(parts)-2]})
		}
		return expr
	}
	return nil
}

func fileConstraint(source sourceFile) (constraint.Expr, []Diagnostic) {
	var modern, legacy constraint.Expr
	for _, group := range source.file.Comments {
		if group.Pos() > source.file.Package {
			break
		}
		for _, comment := range group.List {
			isModern := constraint.IsGoBuild(comment.Text)
			if !isModern && !constraint.IsPlusBuild(comment.Text) {
				continue
			}
			expr, err := constraint.Parse(comment.Text)
			if err != nil || (isModern && modern != nil) {
				return nil, []Diagnostic{{Path: source.path, Line: source.fset.Position(comment.Pos()).Line, Rule: "parse-error"}}
			}
			if isModern {
				modern = expr
			} else {
				legacy = andConstraint(legacy, expr)
			}
		}
	}
	if modern != nil {
		legacy = modern // As in Go builds, go:build takes precedence over +build.
	}
	return andConstraint(filenameConstraint(source.path), legacy), nil
}

// compatibleConstraints asks whether the declarations can coexist in at least
// one Go build. Unknown user tags are boolean variables, not host preferences.
func compatibleConstraints(left, right constraint.Expr) bool {
	expr := andConstraint(left, right)
	if expr == nil {
		return true
	}
	tags := make(map[string]bool)
	collectConstraintTags(expr, tags)
	var unknown []string
	oses, arches, compilers := []string{"linux"}, []string{"amd64"}, []string{"gc"}
	for tag := range tags {
		switch {
		case contains(goOperatingSystems, tag) || tag == "unix":
			oses = goOperatingSystems
		case contains(goArchitectures, tag):
			arches = goArchitectures
		case tag == "gc" || tag == "gccgo":
			compilers = []string{"gc", "gccgo"}
		case strings.HasPrefix(tag, "go1."):
			// Release tags are fixed by the scanner's installed Go toolchain.
		default:
			unknown = append(unknown, tag)
		}
	}
	sort.Strings(unknown)
	for _, operatingSystem := range oses {
		for _, architecture := range arches {
			for _, compiler := range compilers {
				values := make(map[string]bool)
				for tag := range tags {
					if !contains(unknown, tag) {
						values[tag] = platformTag(tag, operatingSystem, architecture, compiler)
					}
				}
				if satisfyConstraint(expr, values, unknown, 0) {
					return true
				}
			}
		}
	}
	return false
}

func platformTag(tag, operatingSystem, architecture, compiler string) bool {
	return tag == operatingSystem || tag == architecture || tag == compiler ||
		(tag == "linux" && operatingSystem == "android") ||
		(tag == "darwin" && operatingSystem == "ios") ||
		(tag == "solaris" && operatingSystem == "illumos") ||
		(tag == "unix" && contains(unixOperatingSystems, operatingSystem)) ||
		contains(build.Default.ReleaseTags, tag)
}

func collectConstraintTags(expr constraint.Expr, tags map[string]bool) {
	switch expr := expr.(type) {
	case *constraint.TagExpr:
		tags[expr.Tag] = true
	case *constraint.NotExpr:
		collectConstraintTags(expr.X, tags)
	case *constraint.AndExpr:
		collectConstraintTags(expr.X, tags)
		collectConstraintTags(expr.Y, tags)
	case *constraint.OrExpr:
		collectConstraintTags(expr.X, tags)
		collectConstraintTags(expr.Y, tags)
	}
}

// Partial evaluation prunes assignments as soon as a clause is decided.
// Results are -1=false, 0=unknown, 1=true.
func evaluateConstraint(expr constraint.Expr, values map[string]bool) int {
	switch expr := expr.(type) {
	case *constraint.TagExpr:
		value, known := values[expr.Tag]
		if !known {
			return 0
		}
		if value {
			return 1
		}
		return -1
	case *constraint.NotExpr:
		return -evaluateConstraint(expr.X, values)
	case *constraint.AndExpr:
		left, right := evaluateConstraint(expr.X, values), evaluateConstraint(expr.Y, values)
		if left < right {
			return left
		}
		return right
	case *constraint.OrExpr:
		left, right := evaluateConstraint(expr.X, values), evaluateConstraint(expr.Y, values)
		if left > right {
			return left
		}
		return right
	}
	return 0
}

func satisfyConstraint(expr constraint.Expr, values map[string]bool, unknown []string, index int) bool {
	if result := evaluateConstraint(expr, values); result != 0 {
		return result > 0
	}
	if index == len(unknown) {
		return false
	}
	tag := unknown[index]
	defer delete(values, tag)
	values[tag] = false
	if satisfyConstraint(expr, values, unknown, index+1) {
		return true
	}
	values[tag] = true
	return satisfyConstraint(expr, values, unknown, index+1)
}
