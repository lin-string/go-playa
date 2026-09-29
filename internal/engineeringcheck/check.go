// Package engineeringcheck enforces source-level engineering contracts.
package engineeringcheck

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Diagnostic identifies a source-level contract violation.
type Diagnostic struct {
	Path string
	Line int
	Rule string
}

func (d Diagnostic) String() string { return fmt.Sprintf("%s:%d:%s", d.Path, d.Line, d.Rule) }

type sourceFile struct {
	path      string
	fset      *token.FileSet
	file      *ast.File
	condition constraint.Expr
}

type typeDefinition struct {
	expr      ast.Expr
	file      *ast.File
	name      *ast.Ident
	condition constraint.Expr
}

// Scan checks production sources, including files not yet added to Git.
// Hidden directories and vendored dependencies are not project sources.
func Scan(root string) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	packages := make(map[string][]sourceFile)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !productionSource(path) {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, parseDiagnostics := parseSource(filepath.ToSlash(relative), source, fset)
		diagnostics = append(diagnostics, parseDiagnostics...)
		if file.file != nil {
			key := filepath.Dir(path) + "/" + file.file.Name.Name
			packages[key] = append(packages[key], file)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, files := range packages {
		definitions := collectTypes(files)
		bindings := resolveBindings(files)
		for _, file := range files {
			diagnostics = append(diagnostics, checkFile(file, definitions, bindings)...)
		}
	}
	sortDiagnostics(diagnostics)
	return diagnostics, nil
}

// CheckSource checks one file without requiring a checkout or buildable imports.
func CheckSource(path string, source []byte) []Diagnostic {
	file, diagnostics := parseSource(filepath.ToSlash(path), source, token.NewFileSet())
	if file.file != nil {
		files := []sourceFile{file}
		diagnostics = append(diagnostics, checkFile(file, collectTypes(files), resolveBindings(files))...)
	}
	sortDiagnostics(diagnostics)
	return diagnostics
}

func productionSource(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

func parseSource(path string, source []byte, fset *token.FileSet) (sourceFile, []Diagnostic) {
	result := sourceFile{path: path, fset: fset}
	if !productionSource(path) {
		return result, nil
	}
	file, err := parser.ParseFile(result.fset, path, source, parser.ParseComments|parser.SkipObjectResolution)
	if file != nil && ast.IsGenerated(file) {
		return result, nil
	}
	if err != nil {
		line := 1
		if errors, ok := err.(scanner.ErrorList); ok && len(errors) > 0 {
			line = errors[0].Pos.Line
		}
		return result, []Diagnostic{{Path: path, Line: line, Rule: "parse-error"}}
	}
	result.file = file
	condition, diagnostics := fileConstraint(result)
	if len(diagnostics) != 0 {
		result.file = nil
		return result, diagnostics
	}
	result.condition = condition
	return result, nil
}

func collectTypes(files []sourceFile) map[string][]typeDefinition {
	types := make(map[string][]typeDefinition)
	for _, file := range files {
		for _, declaration := range file.file.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok || group.Tok != token.TYPE {
				continue
			}
			for _, spec := range group.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				types[typeSpec.Name.Name] = append(types[typeSpec.Name.Name], typeDefinition{expr: typeSpec.Type, file: file.file, name: typeSpec.Name, condition: file.condition})
			}
		}
	}
	return types
}

func resolveBindings(files []sourceFile) *types.Info {
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	var syntax []*ast.File
	for _, file := range files {
		syntax = append(syntax, file.file)
	}
	// Only lexical declaration identities are needed. Import/type errors are
	// outside this source-policy gate and are checked by vet and the build.
	// Not importing dependencies keeps scanning independent of caches and tags.
	config := types.Config{Error: func(error) {}, DisableUnusedImportCheck: true}
	_, _ = config.Check("engineeringcheck/"+files[0].file.Name.Name, files[0].fset, syntax, info)
	return info
}

func checkFile(source sourceFile, definitions map[string][]typeDefinition, bindings *types.Info) []Diagnostic {
	var diagnostics []Diagnostic
	targets := make(map[types.Object]string)
	addParameters := func(fields *ast.FieldList, exported bool, optional map[string]bool) {
		if fields == nil {
			return
		}
		for _, field := range fields.List {
			kind := parameterKind(field.Type, source.file, definitions, bindings, make(map[token.Pos]bool), source.condition)
			for _, name := range field.Names {
				if kind == "nil-context" || (exported && kind == "nil-callback" && !optional[name.Name]) {
					if object := bindings.Defs[name]; object != nil {
						targets[object] = kind
					}
				}
			}
		}
	}
	ast.Inspect(source.file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			exported := node.Name.IsExported()
			addParameters(node.Type.Params, exported, optionalParameters(source.path, node))
			if exported && node.Recv != nil {
				for _, field := range node.Recv.List {
					if _, ok := unparen(field.Type).(*ast.StarExpr); ok {
						for _, name := range field.Names {
							if object := bindings.Defs[name]; object != nil {
								targets[object] = "nil-receiver"
							}
						}
					}
				}
			}
		case *ast.FuncLit:
			addParameters(node.Type.Params, false, nil)
		}
		return true
	})
	ast.Inspect(source.file, func(node ast.Node) bool {
		comparison, ok := node.(*ast.BinaryExpr)
		if !ok || (comparison.Op != token.EQL && comparison.Op != token.NEQ) {
			return true
		}
		left, right := unparen(comparison.X), unparen(comparison.Y)
		if isNil(left, bindings) {
			left, right = right, left
		}
		if identifier, ok := left.(*ast.Ident); ok && isNil(right, bindings) {
			if rule := targets[bindings.Uses[identifier]]; rule != "" {
				diagnostics = append(diagnostics, Diagnostic{Path: source.path, Line: source.fset.Position(comparison.Pos()).Line, Rule: rule})
			}
		}
		return true
	})
	for _, declaration := range source.file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.VAR {
			continue
		}
		for _, spec := range group.Specs {
			value := spec.(*ast.ValueSpec)
			if len(value.Names) != len(value.Values) {
				continue // Multiple-return calls are legitimate; explicit types cannot hide anchors.
			}
			for i, name := range value.Names {
				if name.Name == "_" && deadCodeReference(value.Values[i], bindings) {
					diagnostics = append(diagnostics, Diagnostic{Path: source.path, Line: source.fset.Position(name.Pos()).Line, Rule: "dead-code-anchor"})
				}
			}
		}
	}
	return diagnostics
}

func parameterKind(expr ast.Expr, file *ast.File, definitions map[string][]typeDefinition, bindings *types.Info, seen map[token.Pos]bool, condition constraint.Expr) string {
	switch expr := unparen(expr).(type) {
	case *ast.FuncType:
		return "nil-callback"
	case *ast.SelectorExpr:
		if identifier, ok := expr.X.(*ast.Ident); ok && expr.Sel.Name == "Context" && contextImport(file, identifier.Name) {
			return "nil-context"
		}
	case *ast.Ident:
		if name, ok := bindings.Uses[expr].(*types.TypeName); ok {
			if _, parameter := name.Type().(*types.TypeParam); parameter {
				return "" // A type parameter shadows a same-named package declaration.
			}
		}
		if expr.Name == "Context" && contextImport(file, ".") {
			return "nil-context"
		}
		candidates := definitions[expr.Name]
		// Resolve only declarations visible in a compatible build. Keep the
		// accumulated condition through alias chains so mutually exclusive
		// definitions cannot leak into a platform-specific API's contract.
		for _, definition := range candidates {
			if definition.file == file {
				candidates = []typeDefinition{definition}
				break
			}
		}
		for _, definition := range candidates {
			position := definition.name.Pos()
			if seen[position] || !compatibleConstraints(condition, definition.condition) {
				continue
			}
			seen[position] = true
			kind := parameterKind(definition.expr, definition.file, definitions, bindings, seen, andConstraint(condition, definition.condition))
			delete(seen, position)
			if kind != "" {
				return kind
			}
		}
	case *ast.IndexExpr:
		return parameterKind(expr.X, file, definitions, bindings, seen, condition)
	case *ast.IndexListExpr:
		return parameterKind(expr.X, file, definitions, bindings, seen, condition)
	}
	return ""
}

func contextImport(file *ast.File, name string) bool {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil && path == "context" && ((spec.Name == nil && name == "context") || (spec.Name != nil && spec.Name.Name == name)) {
			return true
		}
	}
	return false
}

type optionalCallbackSpec struct {
	path, receiver, function, parameter string
}

// Every exception is validated against its current declaration by tests, so
// removing/renaming an API or its callback cannot leave a silent stale entry.
var optionalCallbacks = []optionalCallbackSpec{
	{"pdftypes/stream.go", "Stream", "DecodedBufferWithResolver", "resolve"},
	{"pdftypes/stream.go", "Stream", "DecodedBufferWithResolverWithError", "resolve"},
	{"pdftypes/stream.go", "Stream", "DecodedBufferDigestWithResolver", "resolve"},
	{"fontdata/type1_charstring.go", "", "ParseType1CharStringWithSeac", "resolve"},
}

func optionalParameters(path string, function *ast.FuncDecl) map[string]bool {
	var optional map[string]bool
	for _, exception := range optionalCallbacks {
		if exception.path == path && optionalCallbackMatches(function, exception) {
			if optional == nil {
				optional = make(map[string]bool)
			}
			optional[exception.parameter] = true
		}
	}
	return optional
}

func optionalCallbackExists(file *ast.File, exception optionalCallbackSpec) bool {
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && optionalCallbackMatches(function, exception) {
			return true
		}
	}
	return false
}

func optionalCallbackMatches(function *ast.FuncDecl, exception optionalCallbackSpec) bool {
	if function.Name.Name != exception.function {
		return false
	}
	if function.Recv == nil {
		if exception.receiver != "" {
			return false
		}
	} else {
		receiver, ok := unparen(function.Recv.List[0].Type).(*ast.Ident)
		if !ok || receiver.Name != exception.receiver {
			return false
		}
	}
	for _, field := range function.Type.Params.List {
		if _, callback := unparen(field.Type).(*ast.FuncType); !callback {
			continue
		}
		for _, name := range field.Names {
			if name.Name == exception.parameter {
				return true
			}
		}
	}
	return false
}

func deadCodeReference(expr ast.Expr, bindings *types.Info) bool {
	switch expr := unparen(expr).(type) {
	case *ast.IndexExpr:
		return deadCodeReference(expr.X, bindings)
	case *ast.IndexListExpr:
		return deadCodeReference(expr.X, bindings)
	case *ast.Ident:
		_, ok := bindings.Uses[expr].(*types.Func)
		return ok
	case *ast.SelectorExpr:
		_, ok := bindings.Uses[expr.Sel].(*types.Func)
		return ok
	}
	return false
}

func isNil(expr ast.Expr, bindings *types.Info) bool {
	name, ok := expr.(*ast.Ident)
	return ok && name.Name == "nil" && bindings.Uses[name] == types.Universe.Lookup("nil")
}

func unparen(expr ast.Expr) ast.Expr {
	for {
		parenthesized, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = parenthesized.X
	}
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		left, right := diagnostics[i], diagnostics[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Rule < right.Rule
	})
}
