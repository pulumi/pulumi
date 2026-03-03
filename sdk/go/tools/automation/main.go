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

	outputDir := filepath.Join(".", "output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output directory: %v\n", err)
		return 1
	}

	decls, err := generateOptionsTypes(spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate options types: %v\n", err)
		return 1
	}

	var buf bytes.Buffer
	if err := astToBuffer(&buf, decls); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write output: %v\n", err)
		return 1
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: gofmt failed, writing unformatted code: %v\n", err)
		formatted = buf.Bytes()
	}

	outputPath := filepath.Join(outputDir, "main.go")
	if err := os.WriteFile(outputPath, formatted, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write output: %v\n", err)
		return 1
	}

	return 0
}

func astToBuffer(w *bytes.Buffer, decls []ast.Decl) error {
	fset := token.NewFileSet()
	fixCommentPositions(fset, decls)

	file := &ast.File{
		Name:  ast.NewIdent("auto"),
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

// generateOptionsTypes walks the CLI specification tree and returns AST
// declarations for a flattened "Options" struct for every command and menu.
func generateOptionsTypes(root Structure) ([]ast.Decl, error) {
	var decls []ast.Decl
	err := walkStructure(&decls, root, nil, nil)
	return decls, err
}

// walkStructure recursively descends the CLI tree, aggregating flags and
// appending a Go struct per command/menu.
func walkStructure(
	decls *[]ast.Decl,
	node Structure,
	breadcrumbs []string,
	inherited map[string]Flag,
) error {
	command := "pulumi"
	if len(breadcrumbs) > 0 {
		command = command + " " + strings.Join(breadcrumbs, " ")
	}
	typeName := toPascal(command) + "Options"

	// Merge inherited and local flags, with local flags winning on conflicts.
	flags := make(map[string]Flag, len(inherited)+len(node.Flags))
	for k, v := range inherited {
		flags[k] = v
	}
	for k, v := range node.Flags {
		flags[k] = v
	}

	spec, err := buildOptionsSpec(typeName, flags)
	if err != nil {
		return err
	}
	*decls = append(*decls, &ast.GenDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{{Text: fmt.Sprintf("// %s are options for the `%s` command.", typeName, command)}},
		},
		Tok:   token.TYPE,
		Specs: []ast.Spec{spec},
	})

	if node.Type == "menu" && len(node.Commands) > 0 {
		names := make([]string, 0, len(node.Commands))
		for name := range node.Commands {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			child := node.Commands[name]
			if err := walkStructure(decls, child, append(breadcrumbs, name), flags); err != nil {
				return err
			}
		}
	}

	return nil
}

func buildOptionsSpec(typeName string, flags map[string]Flag) (*ast.TypeSpec, error) {
	names := make([]string, 0, len(flags))
	for name := range flags {
		names = append(names, name)
	}
	sort.Strings(names)

	fields := make([]*ast.Field, 0, len(names))
	for _, name := range names {
		flag := flags[name]

		goType, err := astTypeFor(flag.Type, flag.Repeatable)
		if err != nil {
			return nil, err
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
	}

	return &ast.TypeSpec{
		Name: ast.NewIdent(typeName),
		Type: &ast.StructType{
			Fields: &ast.FieldList{List: fields},
		},
	}, nil
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

func toPascal(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return strcase.ToCamel(s)
}
