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
	"io/ioutil"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter

	program     *pcl.Program
	diagnostics hcl.Diagnostics

	configCreated bool
	quotes        map[model.Expression]string
}

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	pcl.MapProvidersAsResources(program)
	g, err := newGenerator(program)
	if err != nil {
		return nil, nil, err
	}

	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := pcl.Linearize(program)

	// Creating a list to store and later print helper methods if they turn out to be needed
	preambleHelperMethods := codegen.NewStringSet()

	var main bytes.Buffer
	g.genPreamble(&main, program, preambleHelperMethods)
	for _, n := range nodes {
		g.genNode(&main, n)
	}

	files := map[string][]byte{
		"__main__.py": main.Bytes(),
	}
	return files, g.diagnostics, nil
}

func GenerateProject(directory string, project workspace.Project, program *pcl.Program) error {
	files, diagnostics, err := GenerateProgram(program)
	if err != nil {
		return err
	}
	if diagnostics.HasErrors() {
		return diagnostics
	}

	// Set the runtime to "python" then marshal to Pulumi.yaml
	project.Runtime = workspace.NewProjectRuntimeInfo("python", nil)
	projectBytes, err := encoding.YAML.Marshal(project)
	if err != nil {
		return err
	}
	files["Pulumi.yaml"] = projectBytes

	// Build a requirements.txt based on the packages used by program
	var requirementsTxt bytes.Buffer
	requirementsTxt.WriteString("pulumi>=3.0.0,<4.0.0\n")

	// For each package add a PackageReference line
	packages, err := program.PackageSnapshots()
	if err != nil {
		return err
	}
	for _, p := range packages {
		if err := p.ImportLanguages(map[string]schema.Language{"python": Importer}); err != nil {
			return err
		}

		packageName := "pulumi-" + p.Name
		if langInfo, found := p.Language["python"]; found {
			pyInfo, ok := langInfo.(PackageInfo)
			if ok && pyInfo.PackageName != "" {
				packageName = pyInfo.PackageName
			}
		}
		if p.Version != nil {
			requirementsTxt.WriteString(fmt.Sprintf("%s==%s\n", packageName, p.Version.String()))
		} else {
			requirementsTxt.WriteString(fmt.Sprintf("%s\n", packageName))
		}
	}

	files["requirements.txt"] = requirementsTxt.Bytes()

	// Add the language specific .gitignore
	files[".gitignore"] = []byte(`*.pyc
venv/`)

	for filename, data := range files {
		outPath := path.Join(directory, filename)
		err := ioutil.WriteFile(outPath, data, 0600)
		if err != nil {
			return fmt.Errorf("could not write output program: %w", err)
		}
	}

	return nil
}

func newGenerator(program *pcl.Program) (*generator, error) {
	// Import Python-specific schema info.
	packages, err := program.PackageSnapshots()
	if err != nil {
		return nil, err
	}
	for _, p := range packages {
		if err := p.ImportLanguages(map[string]schema.Language{"python": Importer}); err != nil {
			return nil, err
		}
	}

	g := &generator{
		program: program,
		quotes:  map[model.Expression]string{},
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

func (g *generator) genPreamble(w io.Writer, program *pcl.Program, preambleHelperMethods codegen.StringSet) {
	// Print the pulumi import at the top.
	g.Fprintln(w, "import pulumi")

	// Accumulate other imports for the various providers. Don't emit them yet, as we need to sort them later on.
	type Import struct {
		// Use an "import ${KEY} as ${.Pkg}"
		ImportAs bool
		// Only relevant for when ImportAs=true
		Pkg string
	}
	importSet := map[string]Import{}
	for _, n := range program.Nodes {
		if r, isResource := n.(*pcl.Resource); isResource {
			pkg, _, _, _ := r.DecomposeToken()
			packageName := "pulumi_" + makeValidIdentifier(pkg)
			if r.Schema != nil && r.Schema.Package != nil {
				if info, ok := r.Schema.Package.Language["python"].(PackageInfo); ok && info.PackageName != "" {
					packageName = info.PackageName
				}
			}
			importSet[packageName] = Import{ImportAs: true, Pkg: makeValidIdentifier(pkg)}
		}
		diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
			if call, ok := n.(*model.FunctionCallExpression); ok {
				if i := g.getFunctionImports(call); len(i) > 0 && i[0] != "" {
					for _, importPackage := range i {
						importAs := strings.HasPrefix(importPackage, "pulumi_")
						var maybePkg string
						if importAs {
							maybePkg = importPackage[len("pulumi_"):]
						}
						importSet[importPackage] = Import{
							ImportAs: importAs,
							Pkg:      maybePkg}
					}
				}
				if helperMethodBody, ok := getHelperMethodIfNeeded(call.Name); ok {
					preambleHelperMethods.Add(helperMethodBody)
				}
			}
			return n, nil
		})
		contract.Assert(len(diags) == 0)
	}

	var imports []string
	importSetNames := codegen.NewStringSet()
	for k := range importSet {
		importSetNames.Add(k)
	}
	for _, pkg := range importSetNames.SortedValues() {
		if pkg == "pulumi" {
			continue
		}
		control := importSet[pkg]
		if control.ImportAs {
			imports = append(imports, fmt.Sprintf("import %s as %s", pkg, control.Pkg))
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

	// If we collected any helper methods that should be added, write them just before the main func
	for _, preambleHelperMethodBody := range preambleHelperMethods.SortedValues() {
		g.Fprintf(w, "%s\n\n", preambleHelperMethodBody)
	}
}

func (g *generator) genNode(w io.Writer, n pcl.Node) {
	switch n := n.(type) {
	case *pcl.Resource:
		g.genResource(w, n)
	case *pcl.ConfigVariable:
		g.genConfigVariable(w, n)
	case *pcl.LocalVariable:
		g.genLocalVariable(w, n)
	case *pcl.OutputVariable:
		g.genOutputVariable(w, n)
	}
}

func tokenToQualifiedName(pkg, module, member string) string {
	components := strings.Split(module, "/")
	for i, component := range components {
		components[i] = PyName(component)
	}
	module = strings.Join(components, ".")
	if module != "" {
		module = "." + module
	}

	return fmt.Sprintf("%s%s.%s", PyName(pkg), module, title(member))

}

// resourceTypeName computes the qualified name of a python resource.
func resourceTypeName(r *pcl.Resource) (string, hcl.Diagnostics) {
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

	return tokenToQualifiedName(pkg, module, member), diagnostics
}

// argumentTypeName computes the Python argument class name for the given expression and model type.
func (g *generator) argumentTypeName(expr model.Expression, destType model.Type) string {
	schemaType, ok := pcl.GetSchemaForType(destType.(model.Type))
	if !ok {
		return ""
	}

	schemaType = codegen.UnwrapType(schemaType)

	objType, ok := schemaType.(*schema.ObjectType)
	if !ok {
		return ""
	}

	token := objType.Token
	tokenRange := expr.SyntaxNode().Range()

	// Example: aws, s3/BucketLogging, BucketLogging, []Diagnostics
	pkgName, module, member, diagnostics := pcl.DecomposeToken(token, tokenRange)
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
	return tokenToQualifiedName(pkgName, modName, member) + "Args"
}

// makeResourceName returns the expression that should be emitted for a resource's "name" parameter given its base name
// and the count variable name, if any.
func (g *generator) makeResourceName(baseName, count string) string {
	if count == "" {
		return fmt.Sprintf(`"%s"`, baseName)
	}
	return fmt.Sprintf(`f"%s-{%s}"`, baseName, count)
}

func (g *generator) lowerResourceOptions(opts *pcl.ResourceOptions) (*model.Block, []*quoteTemp) {
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
func (g *generator) genResource(w io.Writer, r *pcl.Resource) {
	qualifiedMemberName, diagnostics := resourceTypeName(r)
	g.diagnostics = append(g.diagnostics, diagnostics...)

	optionsBag, temps := g.lowerResourceOptions(r.Options)

	name := r.LogicalName()
	nameVar := PyName(r.Name())

	g.genTrivia(w, r.Definition.Tokens.GetType(""))
	for _, l := range r.Definition.Tokens.Labels {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, r.Definition.Tokens.GetOpenBrace())

	for _, input := range r.Inputs {
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
				propertyName := InitParamName(attr.Name)
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
			g.Fgenf(w, "%s%s = None\n", g.Indent, nameVar)
			g.Fgenf(w, "%sif %.v:\n", g.Indent, rangeExpr)
			g.Indented(func() {
				g.Fprintf(w, "%s%s = ", g.Indent, nameVar)
				instantiate(g.makeResourceName(name, ""))
				g.Fprint(w, "\n")
			})
		} else {
			g.Fgenf(w, "%s%s = []\n", g.Indent, nameVar)

			resKey := "key"
			if model.InputType(model.NumberType).ConversionFrom(rangeExpr.Type()) != model.NoConversion {
				g.Fgenf(w, "%sfor range in [{\"value\": i} for i in range(0, %.v)]:\n", g.Indent, rangeExpr)
				resKey = "value"
			} else {
				g.Fgenf(w, "%sfor range in [{\"key\": k, \"value\": v} for [k, v] in enumerate(%.v)]:\n", g.Indent, rangeExpr)
			}

			resName := g.makeResourceName(name, fmt.Sprintf("range['%s']", resKey))
			g.Indented(func() {
				g.Fgenf(w, "%s%s.append(", g.Indent, nameVar)
				instantiate(resName)
				g.Fprint(w, ")\n")
			})
		}
	} else {
		g.Fgenf(w, "%s%s = ", g.Indent, nameVar)
		instantiate(g.makeResourceName(name, ""))
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

func (g *generator) genConfigVariable(w io.Writer, v *pcl.ConfigVariable) {
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
	if defaultValue != nil {
		g.Fgenf(w, "%s%s = config.%s%s(\"%s\", %.v)\n", g.Indent, name, getOrRequire, getType, v.Name(), defaultValue)
	} else {
		g.Fgenf(w, "%s%s = config.%s%s(\"%s\")\n", g.Indent, name, getOrRequire, getType, v.Name())
	}
}

func (g *generator) genLocalVariable(w io.Writer, v *pcl.LocalVariable) {
	value, temps := g.lowerExpression(v.Definition.Value, v.Type())
	g.genTemps(w, temps)

	// TODO(pdg): trivia
	g.Fgenf(w, "%s%s = %.v\n", g.Indent, PyName(v.Name()), value)
}

func (g *generator) genOutputVariable(w io.Writer, v *pcl.OutputVariable) {
	value, temps := g.lowerExpression(v.Value, v.Type())
	g.genTemps(w, temps)

	// TODO(pdg): trivia
	g.Fgenf(w, "%spulumi.export(\"%s\", %.v)\n", g.Indent, v.LogicalName(), value)
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
