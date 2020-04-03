// Copyright 2016-2020, Pulumi Corporation.
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

package python

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/go/common/util/contract"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter

	program         *hcl2.Program
	outputDirectory string
	diagnostics     hcl.Diagnostics
}

func GenerateProgram(program *hcl2.Program, outputDirectory string) (hcl.Diagnostics, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := hcl2.Linearize(program)

	g := &generator{
		program:         program,
		outputDirectory: outputDirectory,
	}
	g.Formatter = format.NewFormatter(g)

	index, err := os.Create(filepath.Join(outputDirectory, "__main__.py"))
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(index)

	g.genPreamble(index, program)

	for _, n := range nodes {
		g.genNode(index, n)
	}

	return g.diagnostics, nil
}

func pyName(pulumiName string, isObjectKey bool) string {
	if isObjectKey {
		return fmt.Sprintf("%q", pulumiName)
	}
	return PyName(cleanName(pulumiName))
}

// genLeadingTrivia generates the list of leading trivia assicated with a given token.
func (g *generator) genLeadingTrivia(w io.Writer, token syntax.Token) {
	// TODO(pdg): whitespace
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
		g.Fgenf(w, "%s#%s\n", g.Indent, l)
	}
}

func (g *generator) genPreamble(w io.Writer, program *hcl2.Program) {
	// Print the pulumi import at the top.
	g.Fprintln(w, "import pulumi")

	// Accumulate other imports for the various providers. Don't emit them yet, as we need to sort them later on.
	var imports []string
	importSet := codegen.StringSet{}
	for _, n := range program.Nodes {
		// TODO: invokes
		if r, isResource := n.(*hcl2.Resource); isResource {
			pkg, _, _, _ := r.DecomposeToken()

			if !importSet.Has(pkg) {
				imports = append(imports, fmt.Sprintf("import pulumi_%[1]s as %[1]s", pkg))
				importSet.Add(pkg)
			}
		}
	}

	// TODO(pdg): do this optionally
	g.Fprintln(w, "import json")

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
	return pyName(pkg, false), strings.Replace(module, "/", ".", -1), title(member), diagnostics
}

// makeResourceName returns the expression that should be emitted for a resource's "name" parameter given its base name
// and the count variable name, if any.
func (g *generator) makeResourceName(baseName, count string) string {
	if count == "" {
		return fmt.Sprintf(`"%s"`, baseName)
	}
	return fmt.Sprintf(`f"%s-${%s}"`, baseName, count)
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

	name := pyName(r.Name(), false)
	resName := g.makeResourceName(name, "")

	g.genTrivia(w, r.Tokens.Type)
	for _, l := range r.Tokens.Labels {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, r.Tokens.OpenBrace)

	g.Fgenf(w, "%s%s = %s(%s", g.Indent, name, qualifiedMemberName, resName)
	indenter := func(f func()) { f() }
	if len(r.Inputs) > 1 {
		indenter = g.Indented
	}
	indenter(func() {
		for _, attr := range r.Inputs {
			propertyName := pyName(attr.Name, false)
			if len(r.Inputs) == 1 {
				g.Fgenf(w, ", %s=%v", propertyName, g.genExpression(attr.Value))
			} else {
				g.Fgenf(w, ",\n%s%s=%v", g.Indent, propertyName, g.genExpression(attr.Value))
			}
		}
	})
	g.Fgenf(w, "%s)\n", optionsBag)
	g.genTrivia(w, r.Tokens.CloseBrace)
}

func (g *generator) genConfigVariable(w io.Writer, v *hcl2.ConfigVariable) {
	// TODO(pdg): trivia

	getType := "object"
	switch v.Type() {
	case model.StringType:
		getType = ""
	case model.NumberType:
		getType = "float"
	case model.IntType:
		getType = "int"
	case model.BoolType:
		getType = "bool"
	}

	getOrRequire := "get"
	if v.DefaultValue == nil {
		getOrRequire = "require"
	}

	g.Fgenf(w, "%[1]s%[2]s = config.%[3]s_%[4]s(\"%[2]s\")\n", g.Indent, v.Name(), getOrRequire, getType)
	if v.DefaultValue != nil {
		g.Fgenf(w, "%sif %s is None:\n", g.Indent, v.Name())
		g.Indented(func() {
			g.Fgenf(w, "%s%s = %s\n", g.Indent, v.Name(), g.genExpression(v.DefaultValue))
		})
	}
}

func (g *generator) genLocalVariable(w io.Writer, v *hcl2.LocalVariable) {
	// TODO(pdg): trivia
	g.Fgenf(w, "%s%s = %s\n", g.Indent, v.Name(), g.genExpression(v.Value))
}

func (g *generator) genOutputVariable(w io.Writer, v *hcl2.OutputVariable) {
	// TODO(pdg): trivia
	g.Fgenf(w, "%spulumi.export(%s, %s)\n", g.Indent, v.Name(), g.genExpression(v.Value))
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	g.Fgenf(w, "(lambda: raise Exception(%q))()", fmt.Sprintf(reason, vs...))
}
