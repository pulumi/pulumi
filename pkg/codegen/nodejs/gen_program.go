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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter

	program         *model.Program
	outputDirectory string
	diagnostics     hcl.Diagnostics
}

func GenerateProgram(program *model.Program, outputDirectory string) (hcl.Diagnostics, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := model.Linearize(program)

	g := &generator{
		program:         program,
		outputDirectory: outputDirectory,
	}
	g.Formatter = format.NewFormatter(g)

	index, err := os.Create(filepath.Join(outputDirectory, "index.ts"))
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
		g.Fgenf(w, "%s//%s\n", g.Indent, l)
	}
}

func (g *generator) genPreamble(w io.Writer, program *model.Program) {
	// Print the @pulumi/pulumi import at the top.
	g.Fprintln(w, `import * as pulumi from "@pulumi/pulumi";`)

	// Accumulate other imports for the various providers. Don't emit them yet, as we need to sort them later on.
	var imports []string
	importSet := codegen.StringSet{}
	for _, n := range program.Nodes {
		// TODO: invokes
		if r, isResource := n.(*model.Resource); isResource {
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

func (g *generator) genNode(w io.Writer, n model.Node) {
	switch n := n.(type) {
	case *model.Resource:
		g.genResource(w, n)
	case *model.ConfigVariable:
	case *model.LocalVariable:
	case *model.OutputVariable:
	}
}

// resourceTypeName computes the NodeJS package, module, and type name for the given resource.
func resourceTypeName(r *model.Resource) (string, string, string, hcl.Diagnostics) {
	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := r.DecomposeToken()
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
func (g *generator) genResource(w io.Writer, r *model.Resource) {
	pkg, module, memberName, diagnostics := resourceTypeName(r)
	g.diagnostics = append(g.diagnostics, diagnostics...)

	if module != "" {
		module = "." + module
	}

	qualifiedMemberName := fmt.Sprintf("%s%s.%s", pkg, module, memberName)

	optionsBag := ""

	inputs := g.genExpression(r.Inputs)

	name := r.Name()
	resName := g.makeResourceName(name, "")

	g.genTrivia(w, r.Tokens.Type)
	for _, l := range r.Tokens.Labels {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, r.Tokens.OpenBrace)
	g.Fgenf(w, "%sconst %s = new %s(%s, %s%s);\n", g.Indent, name, qualifiedMemberName, resName, inputs, optionsBag)
	g.genTrivia(w, r.Tokens.CloseBrace)
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	g.Fgenf(w, "(() => throw new Error(%q))()", fmt.Sprintf(reason, vs...))
}
