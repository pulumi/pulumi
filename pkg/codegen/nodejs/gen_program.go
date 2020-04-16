// Copyright 2016-2020, Pulumi Corporation.
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

package nodejs

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter

	program     *hcl2.Program
	diagnostics hcl.Diagnostics
}

func GenerateProgram(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := hcl2.Linearize(program)

	g := &generator{
		program: program,
	}
	g.Formatter = format.NewFormatter(g)

	var index bytes.Buffer
	g.genPreamble(&index, program)
	for _, n := range nodes {
		g.genNode(&index, n)
	}

	files := map[string][]byte{
		"index.ts": index.Bytes(),
	}
	return files, g.diagnostics, nil
}

// genLeadingTrivia generates the list of leading trivia assicated with a given token.
func (g *generator) genLeadingTrivia(w io.Writer, token syntax.Token) {
	// TODO(pdg): whitespace?
	for _, t := range token.LeadingTrivia {
		if c, ok := t.(syntax.Comment); ok {
			g.genComment(w, c)
		}
	}
}

// genTrailingTrivia generates the list of trailing trivia assicated with a given token.
func (g *generator) genTrailingTrivia(w io.Writer, token syntax.Token) {
	// TODO(pdg): whitespace
	for _, t := range token.TrailingTrivia {
		if c, ok := t.(syntax.Comment); ok {
			g.genComment(w, c)
		}
	}
}

// genTrivia generates the list of trivia assicated with a given token.
func (g *generator) genTrivia(w io.Writer, token syntax.Token) {
	g.genLeadingTrivia(w, token)
	g.genTrailingTrivia(w, token)
}

// genComment generates a comment into the output.
func (g *generator) genComment(w io.Writer, comment syntax.Comment) {
	for _, l := range comment.Lines {
		g.Fgenf(w, "%s//%s\n", g.Indent, l)
	}
}

func (g *generator) genPreamble(w io.Writer, program *hcl2.Program) {
	// Print the @pulumi/pulumi import at the top.
	g.Fprintln(w, `import * as pulumi from "@pulumi/pulumi";`)

	// Accumulate other imports for the various providers. Don't emit them yet, as we need to sort them later on.
	var imports []string
	importSet := codegen.NewStringSet("pulumi")
	for _, n := range program.Nodes {
		// TODO: invokes
		if r, isResource := n.(*hcl2.Resource); isResource {
			pkg, _, _, _ := r.DecomposeToken()

			importName := cleanName(pkg)
			if !importSet.Has(importName) {
				imports = append(imports, fmt.Sprintf(`import * as %s from "@pulumi/%s";`, importName, pkg))
				importSet.Add(importName)
			}
		}
	}

	// Now sort the imports, so we emit them deterministically, and emit them.
	sort.Strings(imports)
	for _, line := range imports {
		g.Fprintln(w, line)
	}
	g.Fprint(w, "\n")
}

func (g *generator) genNode(w io.Writer, n hcl2.Node) {
	switch n := n.(type) {
	case *hcl2.Resource:
		g.genResource(w, n)
	case *hcl2.ConfigVariable:
		g.genConfigVariable(w, n)
	case *hcl2.LocalVariable:
		g.genLocalVariable(w, n)
	case *hcl2.OutputVariable:
		g.genOutputVariable(w, n)
	}
}

// resourceTypeName computes the NodeJS package, module, and type name for the given resource.
func resourceTypeName(r *hcl2.Resource) (string, string, string, hcl.Diagnostics) {
	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := r.DecomposeToken()
	if pkg == "pulumi" && module == "providers" {
		pkg, module, member = member, "", "Provider"
	}
	return cleanName(pkg), strings.Replace(module, "/", ".", -1), title(member), diagnostics
}

// makeResourceName returns the expression that should be emitted for a resource's "name" parameter given its base name
// and the count variable name, if any.
func (g *generator) makeResourceName(baseName, count string) string {
	if count == "" {
		return fmt.Sprintf(`"%s"`, baseName)
	}
	return fmt.Sprintf("`%s-${%s}`", baseName, count)
}

// genResource handles the generation of instantiations of non-builtin resources.
func (g *generator) genResource(w io.Writer, r *hcl2.Resource) {
	pkg, module, memberName, diagnostics := resourceTypeName(r)
	g.diagnostics = append(g.diagnostics, diagnostics...)

	if module != "" {
		module = "." + module
	}

	qualifiedMemberName := fmt.Sprintf("%s%s.%s", pkg, module, memberName)

	optionsBag := ""

	name := r.Name()

	g.genTrivia(w, r.Tokens.GetType(""))
	for _, l := range r.Tokens.GetLabels(nil) {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, r.Tokens.GetOpenBrace())

	instantiate := func(resName string) {
		g.Fgenf(w, "new %s(%s, {", qualifiedMemberName, resName)
		indenter := func(f func()) { f() }
		if len(r.Inputs) > 1 {
			indenter = g.Indented
		}
		indenter(func() {
			for _, attr := range r.Inputs {
				propertyName := attr.Name
				if !isLegalIdentifier(propertyName) {
					propertyName = fmt.Sprintf("%q", propertyName)
				}

				if len(r.Inputs) == 1 {
					g.Fgenf(w, "%s: %.v", propertyName, g.lowerExpression(attr.Value))
				} else {
					g.Fgenf(w, "\n%s%s: %.v,", g.Indent, propertyName, g.lowerExpression(attr.Value))
				}
			}
		})
		if len(r.Inputs) > 1 {
			g.Fgenf(w, "\n%s", g.Indent)
		}
		g.Fgenf(w, "}%s)", optionsBag)
	}

	if r.Options != nil && r.Options.Range != nil {
		if model.InputType(model.BoolType).ConversionFrom(r.Options.Range.Type()) == model.SafeConversion {
			rangeExpr := newAwaitCall(r.Options.Range)

			g.Fgenf(w, "%slet %s: %s | undefined;\n", g.Indent, name, qualifiedMemberName)
			g.Fgenf(w, "%sif (%.v) {\n", g.Indent, rangeExpr)
			g.Indented(func() {
				g.Fgenf(w, "%s%s = ", g.Indent, name)
				instantiate(g.makeResourceName(name, ""))
				g.Fgenf(w, ";\n")
			})
			g.Fgenf(w, "%s}\n", g.Indent)
		} else {
			rangeExpr := newAwaitCall(r.Options.Range)
			g.Fgenf(w, "%sconst %s: %s[];\n", g.Indent, name, qualifiedMemberName)

			resKey, isIterable := "key", false
			if model.InputType(model.NumberType).ConversionFrom(rangeExpr.Type()) != model.NoConversion {
				g.Fgenf(w, "%sfor (const range = {value: 0}; range.value < %.12o; range.value++) {\n", g.Indent, rangeExpr)
				resKey = "value"
			} else {
				g.Fgenf(w, "%sfor (const [__key, __value] of %v) {\n", g.Indent, rangeExpr)
				isIterable = true
			}

			resName := g.makeResourceName(name, "range."+resKey)
			g.Indented(func() {
				if isIterable {
					g.Fgenf(w, "%sconst range = {key: __key, value: __value};\n", g.Indent)
				}
				g.Fgenf(w, "%s%s.push(", g.Indent, name)
				instantiate(resName)
				g.Fgenf(w, ");\n")
			})
			g.Fgenf(w, "%s}\n", g.Indent)
		}
	} else {
		g.Fgenf(w, "%sconst %s = ", g.Indent, name)
		instantiate(g.makeResourceName(name, ""))
		g.Fgenf(w, ";\n")
	}

	g.genTrivia(w, r.Tokens.GetCloseBrace())
}

func (g *generator) genConfigVariable(w io.Writer, v *hcl2.ConfigVariable) {
	// TODO(pdg): trivia

	getType := "Object"
	switch v.Type() {
	case model.StringType:
		getType = ""
	case model.NumberType, model.IntType:
		getType = "Number"
	case model.BoolType:
		getType = "Boolean"
	}

	getOrRequire := "get"
	if v.DefaultValue == nil {
		getOrRequire = "require"
	}

	g.Fgenf(w, "%[1]sconst %[2]s = config.%[3]s%[4]s(\"%[2]s\")", g.Indent, v.Name(), getOrRequire, getType)
	if v.DefaultValue != nil {
		g.Fgenf(w, " || %.v", g.lowerExpression(v.DefaultValue))
	}
	g.Fgenf(w, ";\n")
}

func (g *generator) genLocalVariable(w io.Writer, v *hcl2.LocalVariable) {
	// TODO(pdg): trivia
	g.Fgenf(w, "%sconst %s = %.3v;\n", g.Indent, v.Name(), g.lowerExpression(v.Value))
}

func (g *generator) genOutputVariable(w io.Writer, v *hcl2.OutputVariable) {
	// TODO(pdg): trivia
	g.Fgenf(w, "%sexport const %s = %.3v;\n", g.Indent, v.Name(), g.lowerExpression(v.Value))
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	g.Fgenf(w, "(() => throw new Error(%q))()", fmt.Sprintf(reason, vs...))
}
