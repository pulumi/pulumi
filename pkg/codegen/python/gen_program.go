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
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter

	program     *hcl2.Program
	diagnostics hcl.Diagnostics

	configCreated bool
	casingTables  map[string]map[string]string
	quotes        map[model.Expression]string
}

type objectTypeInfo struct {
	isDictionary         bool
	camelCaseToSnakeCase map[string]string
}

func GenerateProgram(program *hcl2.Program) (map[string][]byte, hcl.Diagnostics, error) {
	g, err := newGenerator(program)
	if err != nil {
		return nil, nil, err
	}

	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := hcl2.Linearize(program)

	var main bytes.Buffer
	g.genPreamble(&main, program)
	for _, n := range nodes {
		g.genNode(&main, n)
	}

	files := map[string][]byte{
		"__main__.py": main.Bytes(),
	}
	return files, g.diagnostics, nil
}

var caseTableCache sync.Map

func newGenerator(program *hcl2.Program) (*generator, error) {
	// Import Python-specific schema info.
	casingTables := map[string]map[string]string{}
	for _, p := range program.Packages() {
		if err := p.ImportLanguages(map[string]schema.Language{"python": Importer}); err != nil {
			return nil, err
		}

		// Build the case mapping table.
		var camelCaseToSnakeCase map[string]string
		if table, ok := caseTableCache.Load(p); ok {
			camelCaseToSnakeCase = table.(map[string]string)
		} else {
			seenTypes := codegen.Set{}
			buildCaseMappingTables(p, nil, camelCaseToSnakeCase, seenTypes)
			caseTableCache.Store(p, camelCaseToSnakeCase)
		}
		casingTables[PyName(p.Name)] = camelCaseToSnakeCase
	}

	g := &generator{
		program:      program,
		casingTables: casingTables,
		quotes:       map[model.Expression]string{},
	}
	g.Formatter = format.NewFormatter(g)

	return g, nil
}

// genLeadingTrivia generates the list of leading trivia associated with a given token.
func (g *generator) genLeadingTrivia(w io.Writer, token syntax.Token) {
	// TODO(pdg): whitespace
	for _, t := range token.LeadingTrivia {
		if c, ok := t.(syntax.Comment); ok {
			g.genComment(w, c)
		}
	}
}

// genTrailingTrivia generates the list of trailing trivia associated with a given token.
func (g *generator) genTrailingTrivia(w io.Writer, token syntax.Token) {
	// TODO(pdg): whitespace
	for _, t := range token.TrailingTrivia {
		if c, ok := t.(syntax.Comment); ok {
			g.genComment(w, c)
		}
	}
}

// genTrivia generates the list of trivia associated with a given token.
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
	importSet := codegen.NewStringSet("pulumi")
	for _, n := range program.Nodes {
		if r, isResource := n.(*hcl2.Resource); isResource {
			pkg, _, _, _ := r.DecomposeToken()
			importSet.Add("pulumi_" + makeValidIdentifier(pkg))
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
		if pkg == "pulumi" {
			continue
		}
		if strings.HasPrefix(pkg, "pulumi_") {
			imports = append(imports, fmt.Sprintf("import %s as %s", pkg, pkg[len("pulumi_"):]))
		} else {
			imports = append(imports, fmt.Sprintf("import %s", pkg))
		}
	}

	// Now sort the imports and emit them.
	sort.Strings(imports)
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

// resourceTypeName computes the Python package, module, and type name for the given resource.
func resourceTypeName(r *hcl2.Resource) (string, string, string, hcl.Diagnostics) {
	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diagnostics := r.DecomposeToken()

	// Normalize module.
	if r.Schema != nil {
		pkg := r.Schema.Package
		if lang, ok := pkg.Language["python"]; ok {
			pkgInfo := lang.(PackageInfo)
			if m, ok := pkgInfo.ModuleNameOverrides[module]; ok {
				module = m
			}
		}
	}

	components := strings.Split(module, "/")
	for i, component := range components {
		components[i] = PyName(component)
	}
	return PyName(pkg), strings.Join(components, "."), title(member), diagnostics
}

// argumentTypeName computes the Python argument class name for the given expression and model type.
func (g *generator) argumentTypeName(expr model.Expression, destType model.Type) string {
	schemaType, ok := hcl2.GetSchemaForType(destType.(model.Type))
	if !ok {
		return ""
	}

	schemaType = codegen.UnwrapType(schemaType)

	objType, ok := schemaType.(*schema.ObjectType)
	if !ok {
		return ""
	}

	if objType.Language != nil {
		pyTypeInfo, ok := objType.Language["python"].(objectTypeInfo)
		if ok {
			if pyTypeInfo.isDictionary {
				return ""
			}
		}
	}

	token := objType.Token
	tokenRange := expr.SyntaxNode().Range()

	// Example: aws, s3/BucketLogging, BucketLogging, []Diagnostics
	pkgName, module, member, diagnostics := hcl2.DecomposeToken(token, tokenRange)
	contract.Assert(len(diagnostics) == 0)

	modName := objType.Package.TokenToModule(token)

	// Normalize module.
	pkg := objType.Package
	if lang, ok := pkg.Language["python"]; ok {
		pkgInfo := lang.(PackageInfo)
		if m, ok := pkgInfo.ModuleNameOverrides[module]; ok {
			modName = m
		}
	}
	if modName != "" {
		modName = "." + PyName(modName)
	}
	modName = strings.Replace(modName, "_", ".", -1)
	member = member + "Args"

	// Example: aws.s3.BucketLoggingArgs
	return fmt.Sprintf("%s%s.%s", PyName(pkgName), modName, title(member))
}

// makeResourceName returns the expression that should be emitted for a resource's "name" parameter given its base name
// and the count variable name, if any.
func (g *generator) makeResourceName(baseName, count string) string {
	if count == "" {
		return fmt.Sprintf(`"%s"`, baseName)
	}
	return fmt.Sprintf(`f"%s-{%s}"`, baseName, count)
}

func (g *generator) lowerResourceOptions(opts *hcl2.ResourceOptions) (*model.Block, []*quoteTemp) {
	if opts == nil {
		return nil, nil
	}

	var block *model.Block
	var temps []*quoteTemp
	appendOption := func(name string, value model.Expression) {
		if block == nil {
			block = &model.Block{
				Type: "options",
				Body: &model.Body{},
			}
		}

		value, valueTemps := g.lowerExpression(value, value.Type())
		temps = append(temps, valueTemps...)

		block.Body.Items = append(block.Body.Items, &model.Attribute{
			Tokens: syntax.NewAttributeTokens(name),
			Name:   name,
			Value:  value,
		})
	}

	if opts.Parent != nil {
		appendOption("parent", opts.Parent)
	}
	if opts.Provider != nil {
		appendOption("provider", opts.Provider)
	}
	if opts.DependsOn != nil {
		appendOption("depends_on", opts.DependsOn)
	}
	if opts.Protect != nil {
		appendOption("protect", opts.Protect)
	}
	if opts.IgnoreChanges != nil {
		appendOption("ignore_changes", opts.IgnoreChanges)
	}

	return block, temps
}

func (g *generator) genResourceOptions(w io.Writer, block *model.Block, hasInputs bool) {
	if block == nil {
		return
	}

	prefix := " "
	if hasInputs {
		prefix = "\n" + g.Indent
	}
	g.Fprintf(w, ",%sopts=pulumi.ResourceOptions(", prefix)
	g.Indented(func() {
		for i, item := range block.Body.Items {
			if i > 0 {
				g.Fprintf(w, ",\n%s", g.Indent)
			}
			attr := item.(*model.Attribute)
			g.Fgenf(w, "%s=%v", attr.Name, attr.Value)
		}
	})
	g.Fprint(w, ")")
}

// genResource handles the generation of instantiations of non-builtin resources.
func (g *generator) genResource(w io.Writer, r *hcl2.Resource) {
	pkg, module, memberName, diagnostics := resourceTypeName(r)
	g.diagnostics = append(g.diagnostics, diagnostics...)
	if module != "" {
		module = "." + module
	}
	qualifiedMemberName := fmt.Sprintf("%s%s.%s", pkg, module, memberName)

	optionsBag, temps := g.lowerResourceOptions(r.Options)

	name := PyName(r.Name())

	g.genTrivia(w, r.Definition.Tokens.GetType(""))
	for _, l := range r.Definition.Tokens.Labels {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, r.Definition.Tokens.GetOpenBrace())

	casingTable := g.casingTables[pkg]
	for _, input := range r.Inputs {
		g.lowerObjectKeys(input.Value, casingTable)

		destType, diagnostics := r.InputType.Traverse(hcl.TraverseAttr{Name: input.Name})
		g.diagnostics = append(g.diagnostics, diagnostics...)
		value, valueTemps := g.lowerExpression(input.Value, destType.(model.Type))
		temps = append(temps, valueTemps...)
		input.Value = value
	}
	g.genTemps(w, temps)

	instantiate := func(resName string) {
		g.Fgenf(w, "%s(%s", qualifiedMemberName, resName)
		indenter := func(f func()) { f() }
		if len(r.Inputs) > 1 {
			indenter = g.Indented
		}
		indenter(func() {
			for _, attr := range r.Inputs {
				propertyName := PyName(attr.Name)
				if len(r.Inputs) == 1 {
					g.Fgenf(w, ", %s=%.v", propertyName, attr.Value)
				} else {
					g.Fgenf(w, ",\n%s%s=%.v", g.Indent, propertyName, attr.Value)
				}
			}
			g.genResourceOptions(w, optionsBag, len(r.Inputs) != 0)
		})
		g.Fprint(w, ")")
	}

	if r.Options != nil && r.Options.Range != nil {
		rangeExpr := r.Options.Range
		if model.InputType(model.BoolType).ConversionFrom(r.Options.Range.Type()) == model.SafeConversion {
			g.Fgenf(w, "%s%s = None\n", g.Indent, name)
			g.Fgenf(w, "%sif %.v:\n", g.Indent, rangeExpr)
			g.Indented(func() {
				g.Fprintf(w, "%s%s = ", g.Indent, name)
				instantiate(g.makeResourceName(r.Name(), ""))
				g.Fprint(w, "\n")
			})
		} else {
			g.Fgenf(w, "%s%s = []\n", g.Indent, name)

			resKey := "key"
			if model.InputType(model.NumberType).ConversionFrom(rangeExpr.Type()) != model.NoConversion {
				g.Fgenf(w, "%sfor range in [{\"value\": i} for i in range(0, %.v)]:\n", g.Indent, rangeExpr)
				resKey = "value"
			} else {
				g.Fgenf(w, "%sfor range in [{\"key\": k, \"value\": v} for [k, v] in enumerate(%.v)]:\n", g.Indent, rangeExpr)
			}

			resName := g.makeResourceName(r.Name(), fmt.Sprintf("range['%s']", resKey))
			g.Indented(func() {
				g.Fgenf(w, "%s%s.append(", g.Indent, name)
				instantiate(resName)
				g.Fprint(w, ")\n")
			})
		}
	} else {
		g.Fgenf(w, "%s%s = ", g.Indent, name)
		instantiate(g.makeResourceName(r.Name(), ""))
		g.Fprint(w, "\n")
	}

	g.genTrivia(w, r.Definition.Tokens.GetCloseBrace())
}

func (g *generator) genTemps(w io.Writer, temps []*quoteTemp) {
	for _, t := range temps {
		// TODO(pdg): trivia
		g.Fgenf(w, "%s%s = %.v\n", g.Indent, t.Name, t.Value)
	}
}

func (g *generator) genConfigVariable(w io.Writer, v *hcl2.ConfigVariable) {
	// TODO(pdg): trivia

	if !g.configCreated {
		g.Fprintf(w, "%sconfig = pulumi.Config()\n", g.Indent)
		g.configCreated = true
	}

	getType := "_object"
	switch v.Type() {
	case model.StringType:
		getType = ""
	case model.NumberType:
		getType = "_float"
	case model.IntType:
		getType = "_int"
	case model.BoolType:
		getType = "_bool"
	}

	getOrRequire := "get"
	if v.DefaultValue == nil {
		getOrRequire = "require"
	}

	var defaultValue model.Expression
	var temps []*quoteTemp
	if v.DefaultValue != nil {
		defaultValue, temps = g.lowerExpression(v.DefaultValue, v.DefaultValue.Type())
	}
	g.genTemps(w, temps)

	name := PyName(v.Name())
	g.Fgenf(w, "%s%s = config.%s%s(\"%s\")\n", g.Indent, name, getOrRequire, getType, v.Name())
	if defaultValue != nil {
		g.Fgenf(w, "%sif %s is None:\n", g.Indent, name)
		g.Indented(func() {
			g.Fgenf(w, "%s%s = %.v\n", g.Indent, name, defaultValue)
		})
	}
}

func (g *generator) genLocalVariable(w io.Writer, v *hcl2.LocalVariable) {
	value, temps := g.lowerExpression(v.Definition.Value, v.Type())
	g.genTemps(w, temps)

	// TODO(pdg): trivia
	g.Fgenf(w, "%s%s = %.v\n", g.Indent, PyName(v.Name()), value)
}

func (g *generator) genOutputVariable(w io.Writer, v *hcl2.OutputVariable) {
	value, temps := g.lowerExpression(v.Value, v.Type())
	g.genTemps(w, temps)

	// TODO(pdg): trivia
	g.Fgenf(w, "%spulumi.export(\"%s\", %.v)\n", g.Indent, v.Name(), value)
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	message := fmt.Sprintf("not yet implemented: %s", fmt.Sprintf(reason, vs...))
	g.diagnostics = append(g.diagnostics, &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  message,
		Detail:   message,
	})
	g.Fgenf(w, "(lambda: raise Exception(%q))()", fmt.Sprintf(reason, vs...))
}
