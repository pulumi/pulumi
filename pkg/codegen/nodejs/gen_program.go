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
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter

	program     *hcl2.Program
	diagnostics hcl.Diagnostics

	asyncMain     bool
	configCreated bool
}

func GenerateProgram(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error) {
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := hcl2.Linearize(program)

	g := &generator{
		program: program,
	}
	g.Formatter = format.NewFormatter(g)

	for _, p := range program.Packages() {
		if err := p.ImportLanguages(map[string]schema.Language{"nodejs": Importer}); err != nil {
			return nil, nil, err
		}
	}

	var index bytes.Buffer
	g.genPreamble(&index, program)
	for _, n := range nodes {
		if r, ok := n.(*hcl2.Resource); ok && requiresAsyncMain(r) {
			g.asyncMain = true
			break
		}
	}

	indenter := func(f func()) { f() }
	if g.asyncMain {
		indenter = g.Indented
		g.Fgenf(&index, "export = async () => {\n")
	}

	indenter(func() {
		for _, n := range nodes {
			g.genNode(&index, n)
		}

		if g.asyncMain {
			var result *model.ObjectConsExpression
			for _, n := range nodes {
				if o, ok := n.(*hcl2.OutputVariable); ok {
					if result == nil {
						result = &model.ObjectConsExpression{}
					}
					name := makeValidIdentifier(o.Name())
					result.Items = append(result.Items, model.ObjectConsItem{
						Key: &model.LiteralValueExpression{Value: cty.StringVal(name)},
						Value: &model.ScopeTraversalExpression{
							RootName:  name,
							Traversal: hcl.Traversal{hcl.TraverseRoot{Name: name}},
							Parts: []model.Traversable{&model.Variable{
								Name:         name,
								VariableType: o.Type(),
							}},
						},
					})
				}
			}
			if result != nil {
				g.Fgenf(&index, "%sreturn %v;\n", g.Indent, result)
			}
		}

	})

	if g.asyncMain {
		g.Fgenf(&index, "}\n")
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

	// Accumulate other imports for the various providers and packages. Don't emit them yet, as we need to sort them
	// later on.
	importSet := codegen.NewStringSet("@pulumi/pulumi")
	for _, n := range program.Nodes {
		if r, isResource := n.(*hcl2.Resource); isResource {
			pkg, _, _, _ := r.DecomposeToken()
			importSet.Add("@pulumi/" + pkg)
		}
		diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
			if call, ok := n.(*model.FunctionCallExpression); ok {
				if i := g.getFunctionImports(call); i != "" {
					importSet.Add(i)
				}
			}
			return n, nil
		})
		contract.Assert(len(diags) == 0)
	}

	var imports []string
	for _, pkg := range importSet.SortedValues() {
		if pkg == "@pulumi/pulumi" {
			continue
		}
		as := makeValidIdentifier(path.Base(pkg))
		if as != pkg {
			imports = append(imports, fmt.Sprintf("import * as %v from \"%v\";", as, pkg))
		} else {
			imports = append(imports, fmt.Sprintf("import * from \"%v\";", pkg))
		}
	}
	sort.Strings(imports)

	// Now sort the imports and emit them.
	for _, i := range imports {
		g.Fprintln(w, i)
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

func requiresAsyncMain(r *hcl2.Resource) bool {
	if r.Options == nil || r.Options.Range == nil {
		return false
	}

	return model.ContainsPromises(r.Options.Range.Type())
}

// resourceTypeName computes the NodeJS package, module, and type name for the given resource.
func resourceTypeName(r *hcl2.Resource) (string, string, string, hcl.Diagnostics) {
	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := r.DecomposeToken()
	if pkg == "pulumi" && module == "providers" {
		pkg, module, member = member, "", "Provider"
	}

	// Normalize module.
	if r.Schema != nil {
		pkg := r.Schema.Package
		if lang, ok := pkg.Language["nodejs"]; ok {
			pkgInfo := lang.(NodePackageInfo)
			if m, ok := pkgInfo.ModuleToPackage[module]; ok {
				module = m
			}
		}
	}

	return makeValidIdentifier(pkg), strings.Replace(module, "/", ".", -1), title(member), diagnostics
}

// makeResourceName returns the expression that should be emitted for a resource's "name" parameter given its base name
// and the count variable name, if any.
func (g *generator) makeResourceName(baseName, count string) string {
	if count == "" {
		return fmt.Sprintf(`"%s"`, baseName)
	}
	return fmt.Sprintf("`%s-${%s}`", baseName, count)
}

func (g *generator) genResourceOptions(opts *hcl2.ResourceOptions) string {
	if opts == nil {
		return ""
	}

	// Turn the resource options into an ObjectConsExpression and generate it.
	var object *model.ObjectConsExpression
	appendOption := func(name string, value model.Expression) {
		if object == nil {
			object = &model.ObjectConsExpression{}
		}
		object.Items = append(object.Items, model.ObjectConsItem{
			Key: &model.LiteralValueExpression{
				Tokens: syntax.NewLiteralValueTokens(cty.StringVal(name)),
				Value:  cty.StringVal(name),
			},
			Value: value,
		})
	}

	if opts.Parent != nil {
		appendOption("parent", opts.Parent)
	}
	if opts.Provider != nil {
		appendOption("provider", opts.Provider)
	}
	if opts.DependsOn != nil {
		appendOption("dependsOn", opts.DependsOn)
	}
	if opts.Protect != nil {
		appendOption("protect", opts.Protect)
	}
	if opts.IgnoreChanges != nil {
		appendOption("ignoreChanges", opts.IgnoreChanges)
	}

	if object == nil {
		return ""
	}

	var buffer bytes.Buffer
	g.Fgenf(&buffer, ", %v", g.lowerExpression(object))
	return buffer.String()
}

// genResource handles the generation of instantiations of non-builtin resources.
func (g *generator) genResource(w io.Writer, r *hcl2.Resource) {
	pkg, module, memberName, diagnostics := resourceTypeName(r)
	g.diagnostics = append(g.diagnostics, diagnostics...)

	if module != "" {
		module = "." + module
	}

	qualifiedMemberName := fmt.Sprintf("%s%s.%s", pkg, module, memberName)

	optionsBag := g.genResourceOptions(r.Options)

	name := r.Name()
	variableName := makeValidIdentifier(name)

	g.genTrivia(w, r.Definition.Tokens.GetType(""))
	for _, l := range r.Definition.Tokens.GetLabels(nil) {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, r.Definition.Tokens.GetOpenBrace())

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
		rangeType := model.ResolveOutputs(r.Options.Range.Type())
		rangeExpr := g.lowerExpression(r.Options.Range)

		if model.InputType(model.BoolType).ConversionFrom(rangeType) == model.SafeConversion {
			g.Fgenf(w, "%slet %s: %s | undefined;\n", g.Indent, variableName, qualifiedMemberName)
			g.Fgenf(w, "%sif (%.v) {\n", g.Indent, rangeExpr)
			g.Indented(func() {
				g.Fgenf(w, "%s%s = ", g.Indent, variableName)
				instantiate(g.makeResourceName(name, ""))
				g.Fgenf(w, ";\n")
			})
			g.Fgenf(w, "%s}\n", g.Indent)
		} else {
			g.Fgenf(w, "%sconst %s: %s[];\n", g.Indent, variableName, qualifiedMemberName)

			resKey := "key"
			if model.InputType(model.NumberType).ConversionFrom(rangeExpr.Type()) != model.NoConversion {
				g.Fgenf(w, "%sfor (const range = {value: 0}; range.value < %.12o; range.value++) {\n", g.Indent, rangeExpr)
				resKey = "value"
			} else {
				rangeExpr := &model.FunctionCallExpression{
					Name: "entries",
					Args: []model.Expression{rangeExpr},
				}
				g.Fgenf(w, "%sfor (const range of %.v) {\n", g.Indent, rangeExpr)
			}

			resName := g.makeResourceName(name, "range."+resKey)
			g.Indented(func() {
				g.Fgenf(w, "%s%s.push(", g.Indent, variableName)
				instantiate(resName)
				g.Fgenf(w, ");\n")
			})
			g.Fgenf(w, "%s}\n", g.Indent)
		}
	} else {
		g.Fgenf(w, "%sconst %s = ", g.Indent, variableName)
		instantiate(g.makeResourceName(name, ""))
		g.Fgenf(w, ";\n")
	}

	g.genTrivia(w, r.Definition.Tokens.GetCloseBrace())
}

func (g *generator) genConfigVariable(w io.Writer, v *hcl2.ConfigVariable) {
	// TODO(pdg): trivia

	if !g.configCreated {
		g.Fprintf(w, "%sconst config = new pulumi.Config();\n", g.Indent)
		g.configCreated = true
	}

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
	g.Fgenf(w, "%sconst %s = %.3v;\n", g.Indent, v.Name(), g.lowerExpression(v.Definition.Value))
}

func (g *generator) genOutputVariable(w io.Writer, v *hcl2.OutputVariable) {
	// TODO(pdg): trivia
	export := "export "
	if g.asyncMain {
		export = ""
	}
	g.Fgenf(w, "%s%sconst %s = %.3v;\n", g.Indent, export, makeValidIdentifier(v.Name()), g.lowerExpression(v.Value))
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	message := fmt.Sprintf("not yet implemented: %s", fmt.Sprintf(reason, vs...))
	g.diagnostics = append(g.diagnostics, &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  message,
		Detail:   message,
	})
	g.Fgenf(w, "(() => throw new Error(%q))()", fmt.Sprintf(reason, vs...))
}
