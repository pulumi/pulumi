// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./sdk/go/tools/automation <path-to-specification.json> [boilerplate-dir]")
		return 1
	}
	specPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve specification path: %v\n", err)
		return 1
	}

	// The boilerplate directory must contain a single .go file that defines
	// the `automation` package's API struct and its run method. Defaults to
	// the testing variant so tests don't need to pass the argument.
	boilerplateDir := filepath.Join("boilerplate", "testing")
	if len(os.Args) >= 3 {
		boilerplateDir = os.Args[2]
	}

	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read specification: %v\n", err)
		return 1
	}

	var spec Structure
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse specification: %v\n", err)
		return 1
	}

	// Output goes next to the generator source, regardless of CWD. The test
	// suite, `go generate`, and ad-hoc manual runs all end up in the same
	// place. `runtime.Caller(0)` gives us this file's absolute path.
	outputDir, err := defaultOutputDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve output directory: %v\n", err)
		return 1
	}
	// Best-effort removal of the legacy single-file output, if present.
	_ = os.Remove(filepath.Join(outputDir, "main.go"))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output directory: %v\n", err)
		return 1
	}

	// Resolve boilerplate dir to the same anchor so relative paths from a
	// test invocation still find the right files.
	if !filepath.IsAbs(boilerplateDir) {
		anchor, err := generatorDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to resolve generator directory: %v\n", err)
			return 1
		}
		boilerplateDir = filepath.Join(anchor, boilerplateDir)
	}

	files, err := generateOptionsFiles(spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate options types: %v\n", err)
		return 1
	}

	for _, file := range files {
		var buf bytes.Buffer
		if err := astToBuffer(&buf, file.Package, file.Decls); err != nil {
			fmt.Fprintf(os.Stderr, "failed to build AST for package %s: %v\n", file.Package, err)
			return 1
		}

		body, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: gofmt failed for package %s, writing unformatted code: %v\n", file.Package, err)
			body = buf.Bytes()
		}

		formatted := append([]byte(copyrightHeader+generatedMarker), body...)

		pkgDir := filepath.Join(outputDir, file.Package)
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create package directory %s: %v\n", pkgDir, err)
			return 1
		}

		outputPath := filepath.Join(pkgDir, "options.go")
		if err := os.WriteFile(outputPath, formatted, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write output %s: %v\n", outputPath, err)
			return 1
		}
	}

	// The API host package is a verbatim copy of the chosen boilerplate
	// plus a generated commands.go that appends one method per executable
	// CLI node.
	automationDir := filepath.Join(outputDir, "automation")
	if err := os.MkdirAll(automationDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create automation directory: %v\n", err)
		return 1
	}

	apiSource, err := readBoilerplateFile(boilerplateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read boilerplate: %v\n", err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(automationDir, "api.go"), apiSource, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write api.go: %v\n", err)
		return 1
	}

	commandsSource, err := generateCommandsFile(spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate commands: %v\n", err)
		return 1
	}
	commandsSource = append([]byte(copyrightHeader+generatedMarker), commandsSource...)
	if err := os.WriteFile(filepath.Join(automationDir, "commands.go"), commandsSource, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write commands.go: %v\n", err)
		return 1
	}

	return 0
}

type optionsFile struct {
	Package string
	Decls   []ast.Decl
}

func astToBuffer(w *bytes.Buffer, pkg string, decls []ast.Decl) error {
	fset := token.NewFileSet()
	fixCommentPositions(fset, decls)

	file := &ast.File{
		Name:  ast.NewIdent(pkg),
		Decls: decls,
	}

	return format.Node(w, fset, file)
}

func fixCommentPositions(fset *token.FileSet, decls []ast.Decl) {
	// The built-in AST package has some fun ideas. One is that a newline will
	// only be inserted if the token before the cursor is on a different line to
	// the token after. In our case, there are no newlines - we built the AST
	// ourselves. So, in order to dictate where newlines are inserted, we need to
	// create a big file with a lot of lines, and make sure that every new line we
	// want is on a different "line". These lines can be one character each, and
	// no one will mind how many tokens are supposedly on each line.
	magicLineNumber := 100 * 1000

	file := fset.AddFile("main.go", 1, magicLineNumber)
	lines := make([]int, magicLineNumber)

	for i := range lines {
		lines[i] = i
	}
	file.SetLines(lines)

	line := 1
	for _, d := range decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, s := range gen.Specs {
			spec, ok := s.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := spec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, f := range st.Fields.List {
				if f.Doc != nil {
					for _, c := range f.Doc.List {
						c.Slash = file.LineStart(line)
						line++
					}
				}
				if len(f.Names) > 0 {
					f.Names[0].NamePos = file.LineStart(line)
				}
				line++
			}
		}
	}
}

// generateOptionsFiles walks the CLI specification tree and returns one
// optionsFile per command/menu, each containing a single Options struct and its
// ergonomic helpers. Each file will become its own Go package.
func generateOptionsFiles(root Structure) ([]optionsFile, error) {
	return walkStructure(root, nil, nil)
}

// walkStructure recursively descends the CLI tree, aggregating flags and
// returning a Go file (package + declarations) per command/menu.
func walkStructure(node Structure, breadcrumbs []string, inherited map[string]Flag) ([]optionsFile, error) {
	command := "pulumi"
	if len(breadcrumbs) > 0 {
		command = command + " " + strings.Join(breadcrumbs, " ")
	}
	// Each package has a single generic Options type for its command.
	typeName := "Options"

	// Merge inherited and local flags, with local flags winning on conflicts.
	flags := make(map[string]Flag, len(inherited)+len(node.Flags))
	for k, v := range inherited {
		flags[k] = v
	}
	for k, v := range node.Flags {
		flags[k] = v
	}

	spec, fields, err := buildOptionsSpec(typeName, flags)
	if err != nil {
		return nil, err
	}

	// Ergonomic functional-style helpers for configuring this command's options.
	optionDecls := buildOptionHelpers(typeName, command, fields)

	// Package-level declarations: imports first, then the struct, then helpers.
	importDecl := buildOptionsImports()
	decls := make([]ast.Decl, 0, 2+len(optionDecls))
	decls = append(decls, importDecl)

	// The primary options struct for this command.
	decls = append(decls, &ast.GenDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{{Text: fmt.Sprintf("// %s are options for the `%s` command.", typeName, command)}},
		},
		Tok:   token.TYPE,
		Specs: []ast.Spec{spec},
	})

	decls = append(decls, optionDecls...)

	files := []optionsFile{{
		Package: packageNameFor(breadcrumbs),
		Decls:   decls,
	}}

	if node.Type == "menu" && len(node.Commands) > 0 {
		names := make([]string, 0, len(node.Commands))
		for name := range node.Commands {
			names = append(names, name)
		}
		sort.Strings(names)

		// Strip Omit/Preset from inherited flags so children can re-declare
		// their own overrides without the parent's leaking in. Mirrors
		// `baseFlag` in sdk/nodejs/tools/automation/src/index.ts.
		childInherited := make(map[string]Flag, len(flags))
		for k, v := range flags {
			v.Omit = false
			v.Preset = nil
			childInherited[k] = v
		}

		for _, name := range names {
			child := node.Commands[name]
			childFiles, err := walkStructure(child, append(breadcrumbs, name), childInherited)
			if err != nil {
				return nil, err
			}
			files = append(files, childFiles...)
		}
	}

	return files, nil
}

// buildOptionsImports returns the `import ( ... )` declaration that every
// generated options file needs so it can embed base.BaseOptions.
func buildOptionsImports() *ast.GenDecl {
	return &ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			&ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: `"` + basePackageImportPath + `"`,
				},
			},
		},
	}
}

// basePackageImportPath is the canonical location of the `base` package
// that generated code consumes. The integration PR (#4) is where this flips
// over to the real sdk/go/auto address.
const basePackageImportPath = "github.com/pulumi/pulumi/sdk/v3/go/tools/automation/boilerplate/base"

// copyrightHeader is prepended to every file written by the generator. The
// repository's lint setup checks for it via `goheader`.
const copyrightHeader = `// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

`

// generatedMarker is the standard `DO NOT EDIT` banner recognised by gopls
// and other tools. It follows the copyright header.
const generatedMarker = `// Code generated by sdk/go/tools/automation; DO NOT EDIT.

`

type optionField struct {
	name string
	typ  ast.Expr
}

func buildOptionsSpec(typeName string, flags map[string]Flag) (*ast.TypeSpec, []optionField, error) {
	names := make([]string, 0, len(flags))
	for name := range flags {
		// Omitted flags are not exposed on the Options struct; they exist
		// only to anchor preset injections in generated command bodies.
		if flags[name].Omit {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	// Every generated Options struct embeds base.BaseOptions so that the
	// API's `run` method has somewhere to read ambient invocation config
	// (cwd, env, stdout/stderr, stdin) from.
	fields := make([]*ast.Field, 0, len(names)+1)
	fields = append(fields, &ast.Field{
		Type: &ast.SelectorExpr{
			X:   ast.NewIdent("base"),
			Sel: ast.NewIdent("BaseOptions"),
		},
	})

	meta := make([]optionField, 0, len(names))
	for _, name := range names {
		flag := flags[name]

		goType, err := astTypeFor(flag.Type, flag.Repeatable)
		if err != nil {
			return nil, nil, err
		}

		fieldName := strcase.ToCamel(flag.Name)
		field := &ast.Field{
			Names: []*ast.Ident{{Name: fieldName}},
			Type:  goType,
		}
		if flag.Description != "" {
			field.Doc = toComment(flag.Description)
		}
		fields = append(fields, field)
		meta = append(meta, optionField{name: fieldName, typ: goType})
	}

	return &ast.TypeSpec{
		Name: ast.NewIdent(typeName),
		Type: &ast.StructType{
			Fields: &ast.FieldList{List: fields},
		},
	}, meta, nil
}

func buildOptionHelpers(typeName, command string, fields []optionField) []ast.Decl {
	if len(fields) == 0 {
		return nil
	}

	// All helpers in a package share the simple names Option / <Field>. The
	// helpers intentionally carry no prefix so they match the naming used by
	// the existing hand-written optXxx packages (for example optup.Stack,
	// optup.ContinueOnError). The integration PR can then replace those
	// packages with the generated ones without churning call sites.
	optionTypeName := "Option"

	decls := make([]ast.Decl, 0, 1+len(fields))

	// type Option func(*Options)
	optionType := &ast.TypeSpec{
		Name: ast.NewIdent(optionTypeName),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.StarExpr{X: ast.NewIdent(typeName)},
					},
				},
			},
		},
	}

	decls = append(decls, &ast.GenDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{{
				Text: fmt.Sprintf("// %s configures %s when building CLI commands.", optionTypeName, command),
			}},
		},
		Tok:   token.TYPE,
		Specs: []ast.Spec{optionType},
	})

	// One helper per field, named after the field itself.
	for _, f := range fields {
		funcName := f.name

		// func <Field>(v <T>) Option
		fn := &ast.FuncDecl{
			Doc: &ast.CommentGroup{
				List: []*ast.Comment{{
					Text: fmt.Sprintf("// %s returns an %s that sets %s.", funcName, optionTypeName, f.name),
				}},
			},
			Name: ast.NewIdent(funcName),
			Type: &ast.FuncType{
				Params: &ast.FieldList{
					List: []*ast.Field{
						{
							Names: []*ast.Ident{ast.NewIdent("v")},
							Type:  f.typ,
						},
					},
				},
				Results: &ast.FieldList{
					List: []*ast.Field{
						{
							Type: ast.NewIdent(optionTypeName),
						},
					},
				},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.ReturnStmt{
						Results: []ast.Expr{
							&ast.FuncLit{
								Type: &ast.FuncType{
									Params: &ast.FieldList{
										List: []*ast.Field{
											{
												Names: []*ast.Ident{ast.NewIdent("o")},
												Type:  &ast.StarExpr{X: ast.NewIdent(typeName)},
											},
										},
									},
								},
								Body: &ast.BlockStmt{
									List: []ast.Stmt{
										&ast.AssignStmt{
											Lhs: []ast.Expr{
												&ast.SelectorExpr{
													X:   ast.NewIdent("o"),
													Sel: ast.NewIdent(f.name),
												},
											},
											Tok: token.ASSIGN,
											Rhs: []ast.Expr{
												ast.NewIdent("v"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		decls = append(decls, fn)
	}

	return decls
}

func toComment(desc string) *ast.CommentGroup {
	lines := strings.Split(strings.TrimSpace(desc), "\n")
	list := make([]*ast.Comment, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		list = append(list, &ast.Comment{Text: "// " + line})
	}
	if len(list) == 0 {
		return nil
	}
	return &ast.CommentGroup{List: list}
}

func astTypeFor(typ string, repeatable bool) (ast.Expr, error) {
	var base *ast.Ident
	switch typ {
	case "string":
		base = ast.NewIdent("string")
	case "boolean":
		base = ast.NewIdent("bool")
	case "int":
		base = ast.NewIdent("int")
	default:
		return nil, fmt.Errorf("unknown flag type: %s", typ)
	}

	if repeatable {
		return &ast.ArrayType{Elt: base}, nil
	}
	return base, nil
}

// packageNameFor converts a list of CLI subcommand breadcrumbs into the Go
// package name where that command's options will live. This follows the
// existing Automation API convention of packages like "optup", "optpreview",
// "optremoteup", etc.
func packageNameFor(breadcrumbs []string) string {
	if len(breadcrumbs) == 0 {
		return "opt"
	}

	var b strings.Builder
	b.WriteString("opt")

	for _, part := range breadcrumbs {
		for _, r := range part {
			switch {
			case r >= 'a' && r <= 'z',
				r >= 'A' && r <= 'Z',
				r >= '0' && r <= '9':
				b.WriteRune(r)
			}
		}
	}

	name := strings.ToLower(b.String())
	if name == "" {
		return "opt"
	}
	return name
}

// generatorDir returns the absolute path of the directory that contains this
// source file. Stable across CWDs because it is resolved from
// `runtime.Caller`.
func generatorDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("runtime.Caller failed; cannot anchor generator paths")
	}
	return filepath.Dir(thisFile), nil
}

// defaultOutputDir returns `<generatorDir>/output`.
func defaultOutputDir() (string, error) {
	dir, err := generatorDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "output"), nil
}

// readBoilerplateFile returns the contents of the single .go file inside dir.
// Boilerplate subdirectories (e.g. boilerplate/standard, boilerplate/testing)
// hold one source file each; anything more would be ambiguous.
func readBoilerplateFile(dir string) ([]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading boilerplate directory %s: %w", dir, err)
	}
	var found string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		if found != "" {
			return nil, fmt.Errorf("boilerplate directory %s contains multiple .go files (%s and %s)", dir, found, e.Name())
		}
		found = e.Name()
	}
	if found == "" {
		return nil, fmt.Errorf("boilerplate directory %s contains no .go files", dir)
	}
	return os.ReadFile(filepath.Join(dir, found))
}

// commandMethod is the template payload for one generated method.
type commandMethod struct {
	Name         string
	OptPkg       string
	Breadcrumbs  []string
	DocComment   string
	RequiredArgs []methodArg
	OptionalArgs []methodArg
	VariadicArg  *methodArg
	// Presets are pre-rendered Go statements appending preset flag values.
	Presets []string
	// Flags are pre-rendered Go statements appending user-supplied flag values.
	Flags   []string
	HasArgs bool
}

type methodArg struct {
	GoName string
	GoType string
}

// generateCommandsFile walks the spec and produces the contents of
// output/automation/commands.go — one method per executable command/menu.
func generateCommandsFile(root Structure) ([]byte, error) {
	var methods []commandMethod
	optPkgs := map[string]struct{}{}
	if err := walkCommands(root, nil, nil, &methods, optPkgs); err != nil {
		return nil, err
	}

	// Sort opt-package imports for deterministic output.
	imports := make([]string, 0, len(optPkgs))
	for p := range optPkgs {
		imports = append(imports, p)
	}
	sort.Strings(imports)

	data := struct {
		BasePkg    string
		OptImports []string
		Methods    []commandMethod
	}{
		BasePkg:    basePackageImportPath,
		OptImports: imports,
		Methods:    methods,
	}

	var buf bytes.Buffer
	if err := commandsFileTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendering commands.go template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return the unformatted source so the caller can inspect what went
		// wrong, but surface the error.
		return buf.Bytes(), fmt.Errorf("gofmt failed on commands.go: %w", err)
	}
	return formatted, nil
}

// walkCommands is the command-side analogue of walkStructure. It descends
// the spec tree accumulating method descriptors for every executable node.
// The optPkgs set records which opt packages are referenced so the caller
// can emit the correct import list.
func walkCommands(
	node Structure,
	breadcrumbs []string,
	inherited map[string]Flag,
	out *[]commandMethod,
	optPkgs map[string]struct{},
) error {
	flags := make(map[string]Flag, len(inherited)+len(node.Flags))
	for k, v := range inherited {
		flags[k] = v
	}
	for k, v := range node.Flags {
		flags[k] = v
	}

	isExecutable := node.Type == "command" || (node.Type == "menu" && node.Executable)
	if isExecutable {
		method, err := buildCommandMethod(node, breadcrumbs, flags)
		if err != nil {
			return err
		}
		*out = append(*out, method)
		optPkgs[method.OptPkg] = struct{}{}
	}

	if node.Type == "menu" && len(node.Commands) > 0 {
		names := make([]string, 0, len(node.Commands))
		for name := range node.Commands {
			names = append(names, name)
		}
		sort.Strings(names)

		childInherited := make(map[string]Flag, len(flags))
		for k, v := range flags {
			v.Omit = false
			v.Preset = nil
			childInherited[k] = v
		}

		for _, name := range names {
			child := node.Commands[name]
			if err := walkCommands(child, append(breadcrumbs, name), childInherited, out, optPkgs); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildCommandMethod assembles the template payload for a single executable
// node: method name, positional arguments, pre-rendered preset/flag bodies.
func buildCommandMethod(node Structure, breadcrumbs []string, flags map[string]Flag) (commandMethod, error) {
	m := commandMethod{
		Name:        methodNameFor(breadcrumbs),
		OptPkg:      packageNameFor(breadcrumbs),
		Breadcrumbs: append([]string(nil), breadcrumbs...),
		DocComment:  strings.TrimSpace(node.Description),
	}

	// Positional arguments. When `requiredArguments` is absent from the spec
	// all positional args are treated as optional (per NodeJS/Python
	// convention); otherwise the first N are required and the rest optional.
	if node.Arguments != nil {
		allArgs := node.Arguments.Arguments
		// When requiredArguments is omitted from the spec, all positional
		// arguments are optional (matches sdk/nodejs/tools/automation and
		// sdk/python/tools/automation behaviour).
		required := 0
		if node.Arguments.RequiredArguments != nil {
			required = *node.Arguments.RequiredArguments
		}
		if node.Arguments.Variadic && len(allArgs) > 0 {
			// The variadic arg is the last one; preceding entries split
			// between required and optional according to `required`.
			trailing := allArgs[len(allArgs)-1]
			head := allArgs[:len(allArgs)-1]
			for i, a := range head {
				arg, err := argToMethodArg(a)
				if err != nil {
					return m, err
				}
				if i < required {
					m.RequiredArgs = append(m.RequiredArgs, arg)
				} else {
					m.OptionalArgs = append(m.OptionalArgs, arg)
				}
			}
			v, err := argToMethodArg(trailing)
			if err != nil {
				return m, err
			}
			m.VariadicArg = &v
		} else {
			for i, a := range allArgs {
				arg, err := argToMethodArg(a)
				if err != nil {
					return m, err
				}
				if i < required {
					m.RequiredArgs = append(m.RequiredArgs, arg)
				} else {
					m.OptionalArgs = append(m.OptionalArgs, arg)
				}
			}
		}
	}
	m.HasArgs = len(m.RequiredArgs)+len(m.OptionalArgs) > 0 || m.VariadicArg != nil

	// Presets first, then regular flag emissions. Both sorted alphabetically
	// by canonical flag name for determinism.
	names := make([]string, 0, len(flags))
	for name := range flags {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		flag := flags[name]
		if flag.Preset == nil {
			continue
		}
		snippet, err := renderPreset(flag)
		if err != nil {
			return m, err
		}
		m.Presets = append(m.Presets, snippet)
	}

	for _, name := range names {
		flag := flags[name]
		if flag.Omit {
			continue
		}
		snippet, err := renderFlag(flag)
		if err != nil {
			return m, err
		}
		m.Flags = append(m.Flags, snippet)
	}

	return m, nil
}

// methodNameFor converts CLI breadcrumbs to a Go method name. For example:
// [] → "Pulumi" (unused — we never emit a root method), ["cancel"] →
// "Cancel", ["stack", "rm"] → "StackRm", ["org", "search", "ai"] →
// "OrgSearchAi".
func methodNameFor(breadcrumbs []string) string {
	if len(breadcrumbs) == 0 {
		return "Pulumi"
	}
	joined := strings.Join(breadcrumbs, "_")
	// Normalise separators strcase.ToCamel doesn't touch on its own.
	joined = strings.ReplaceAll(joined, "-", "_")
	joined = strings.ReplaceAll(joined, "/", "_")
	return strcase.ToCamel(joined)
}

// argToMethodArg turns a spec Argument into a template-ready methodArg.
func argToMethodArg(a Argument) (methodArg, error) {
	var goType string
	switch a.Type {
	case "", "string":
		goType = "string"
	case "int":
		goType = "int"
	case "boolean":
		goType = "bool"
	default:
		return methodArg{}, fmt.Errorf("unsupported positional argument type: %s", a.Type)
	}
	return methodArg{
		GoName: strcase.ToLowerCamel(a.Name),
		GoType: goType,
	}, nil
}

// renderFlag emits a Go snippet that appends this flag's value to `final`
// when the user has set it. For boolean flags, the value itself is the
// gating condition. For other primitive flags without `required`, we use
// the zero value as a "not set" sentinel — callers who need to
// distinguish the zero value from "unset" should mark the flag required
// or extend the spec.
func renderFlag(flag Flag) (string, error) {
	fieldName := strcase.ToCamel(flag.Name)
	cliName := "--" + flag.Name
	switch {
	case flag.Repeatable:
		return fmt.Sprintf(
			"for _, v := range o.%s {\n\tfinal = append(final, %q, fmt.Sprint(v))\n}",
			fieldName, cliName,
		), nil
	case flag.Type == "boolean":
		return fmt.Sprintf("if o.%s {\n\tfinal = append(final, %q)\n}", fieldName, cliName), nil
	case flag.Required:
		return fmt.Sprintf("final = append(final, %q, fmt.Sprint(o.%s))", cliName, fieldName), nil
	case flag.Type == "string":
		return fmt.Sprintf(
			"if o.%s != \"\" {\n\tfinal = append(final, %q, fmt.Sprint(o.%s))\n}",
			fieldName, cliName, fieldName,
		), nil
	case flag.Type == "int":
		return fmt.Sprintf(
			"if o.%s != 0 {\n\tfinal = append(final, %q, fmt.Sprint(o.%s))\n}",
			fieldName, cliName, fieldName,
		), nil
	default:
		return "", fmt.Errorf("unsupported flag type for --%s: %s", flag.Name, flag.Type)
	}
}

// renderPreset emits a Go snippet that appends a fixed flag value. When the
// flag is Omit'd the snippet is unconditional; otherwise it is wrapped in a
// "not set by the user" guard on the generated Options field. The current
// fixture only exercises the Omit path.
func renderPreset(flag Flag) (string, error) {
	valueSnippet, err := renderPresetAppend(flag)
	if err != nil {
		return "", err
	}
	if flag.Omit {
		return valueSnippet, nil
	}
	// Guard on zero value of the exposed field.
	fieldName := strcase.ToCamel(flag.Name)
	var cond string
	switch {
	case flag.Repeatable:
		cond = "len(o." + fieldName + ") == 0"
	case flag.Type == "boolean":
		cond = "!o." + fieldName
	case flag.Type == "string":
		cond = "o." + fieldName + ` == ""`
	case flag.Type == "int":
		cond = "o." + fieldName + " == 0"
	default:
		return "", fmt.Errorf("unsupported preset guard for --%s: %s", flag.Name, flag.Type)
	}
	return fmt.Sprintf("if %s {\n\t%s\n}", cond, strings.ReplaceAll(valueSnippet, "\n", "\n\t")), nil
}

// renderPresetAppend renders the unconditional append for a preset value.
func renderPresetAppend(flag Flag) (string, error) {
	pv := flag.Preset
	switch {
	case pv.Bool != nil:
		if *pv.Bool {
			return fmt.Sprintf("final = append(final, %q)", "--"+flag.Name), nil
		}
		// `preset: false` means "never emit this flag" — a no-op in Go.
		return "", nil
	case pv.String != nil:
		return fmt.Sprintf("final = append(final, %q, %q)", "--"+flag.Name, *pv.String), nil
	case pv.Int != nil:
		return fmt.Sprintf("final = append(final, %q, fmt.Sprint(%d))", "--"+flag.Name, *pv.Int), nil
	case pv.Strings != nil:
		lines := make([]string, 0, 3)
		lines = append(lines, fmt.Sprintf("for _, v := range %s {", goStringSliceLiteral(pv.Strings)))
		lines = append(lines, fmt.Sprintf("\tfinal = append(final, %q, v)", "--"+flag.Name))
		lines = append(lines, "}")
		return strings.Join(lines, "\n"), nil
	}
	return "", fmt.Errorf("preset value for --%s is empty", flag.Name)
}

// goStringSliceLiteral renders a []string literal for use inside generated
// Go source code.
func goStringSliceLiteral(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, fmt.Sprintf("%q", x))
	}
	return "[]string{" + strings.Join(parts, ", ") + "}"
}

var commandsFileTemplate = template.Must(template.New("commands").Funcs(template.FuncMap{
	"quote": func(s string) string { return fmt.Sprintf("%q", s) },
}).Parse(`package automation

import (
	"context"
	"fmt"

	"{{.BasePkg}}"
{{range .OptImports}}	"github.com/pulumi/pulumi/sdk/v3/go/tools/automation/output/{{.}}"
{{end}})

// Silence unused-import warnings when a spec has no commands with args.
var _ = fmt.Sprint
var _ = context.Background

{{range .Methods}}
{{if .DocComment}}// {{.Name}} corresponds to ` + "`pulumi {{range $i, $c := .Breadcrumbs}}{{if $i}} {{end}}{{$c}}{{end}}`" + `.
//
// {{.DocComment}}
{{else}}// {{.Name}} corresponds to ` + "`pulumi {{range $i, $c := .Breadcrumbs}}{{if $i}} {{end}}{{$c}}{{end}}`" + `.
{{end -}}
func (a *API) {{.Name}}(
	ctx context.Context,
{{- range .RequiredArgs}}
	{{.GoName}} {{.GoType}},
{{- end}}
{{- range .OptionalArgs}}
	{{.GoName}} *{{.GoType}},
{{- end}}
{{- if .VariadicArg}}
	{{.VariadicArg.GoName}} []{{.VariadicArg.GoType}},
{{- end}}
	opts ...{{.OptPkg}}.Option,
) (base.CommandResult, error) {
	o := {{.OptPkg}}.Options{}
	for _, opt := range opts {
		opt(&o)
	}

	final := []string{ {{range .Breadcrumbs}}{{quote .}}, {{end}} }
{{range .Presets}}
	{{.}}
{{end}}
{{- range .Flags}}
	{{.}}
{{end}}
{{- if .HasArgs}}
	args := []string{}
{{- range .RequiredArgs}}
	args = append(args, fmt.Sprint({{.GoName}}))
{{- end}}
{{- range .OptionalArgs}}
	if {{.GoName}} != nil {
		args = append(args, fmt.Sprint(*{{.GoName}}))
	}
{{- end}}
{{- if .VariadicArg}}
	for _, v := range {{.VariadicArg.GoName}} {
		args = append(args, fmt.Sprint(v))
	}
{{- end}}
	if len(args) > 0 {
		final = append(final, "--")
		final = append(final, args...)
	}
{{end}}
	return a.run(ctx, o.BaseOptions, final)
}
{{end}}`))
