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
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./sdk/go/tools/automation <path-to-specification.json>")
		return 1
	}
	specPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve specification path: %v\n", err)
		return 1
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

	// Always write generated code under tools/automation/output so it lives
	// alongside this generator, regardless of the current working directory.
	outputDir := filepath.Join("tools", "automation", "output")
	// Best-effort removal of the legacy single-file output, if present.
	_ = os.Remove(filepath.Join(outputDir, "main.go"))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output directory: %v\n", err)
		return 1
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

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: gofmt failed for package %s, writing unformatted code: %v\n", file.Package, err)
			formatted = buf.Bytes()
		}

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

	// We always emit a single struct declaration followed by the helpers.
	decls := make([]ast.Decl, 0, 1+len(optionDecls))

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

		for _, name := range names {
			child := node.Commands[name]
			childFiles, err := walkStructure(child, append(breadcrumbs, name), flags)
			if err != nil {
				return nil, err
			}
			files = append(files, childFiles...)
		}
	}

	return files, nil
}

type optionField struct {
	name string
	typ  ast.Expr
}

func buildOptionsSpec(typeName string, flags map[string]Flag) (*ast.TypeSpec, []optionField, error) {
	names := make([]string, 0, len(flags))
	for name := range flags {
		names = append(names, name)
	}
	sort.Strings(names)

	fields := make([]*ast.Field, 0, len(names))
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

	// All helpers in a package share the simple names Option / With<Field>.
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

	// One With* helper per field.
	for _, f := range fields {
		funcName := "With" + f.name

		// func With<Field>(v <T>) Option
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
