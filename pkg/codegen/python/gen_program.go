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

package python

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/format"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const stackRefQualifiedName = "pulumi.StackReference"

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter

	program     *pcl.Program
	diagnostics hcl.Diagnostics

	configCreated           bool
	quotes                  map[model.Expression]string
	isComponent             bool
	deferredOutputVariables []*pcl.DeferredOutputVariable
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

	for componentDir, component := range program.CollectComponents() {
		componentFilename := strings.ReplaceAll(filepath.Base(componentDir), "-", "_")
		componentName := component.DeclarationName()
		componentGenerator, err := newGenerator(component.Program)
		if err != nil {
			return files, componentGenerator.diagnostics, err
		}

		// mark the generator to target components
		componentGenerator.isComponent = true

		componentPreambleMethods := codegen.NewStringSet()
		var componentBuffer bytes.Buffer
		// generate imports for the component
		componentGenerator.genPreamble(&componentBuffer, component.Program, componentPreambleMethods)
		componentGenerator.genComponentDefinition(&componentBuffer, component, componentName)
		files[componentFilename+".py"] = componentBuffer.Bytes()
	}
	return files, g.diagnostics, nil
}

func componentInputElementType(pclType model.Type) string {
	switch pclType {
	case model.BoolType:
		return "bool"
	case model.IntType:
		return "int"
	case model.NumberType:
		return "float"
	case model.StringType:
		return "str"
	default:
		switch pclType := pclType.(type) {
		case *model.ListType:
			elementType := componentInputElementType(pclType.ElementType)
			return fmt.Sprintf("list[%s]", elementType)
		case *model.MapType:
			elementType := componentInputElementType(pclType.ElementType)
			return fmt.Sprintf("Dict[str, %s]", elementType)
		// reduce option(T) to just T
		// the TypedDict has total=False which means all properties are optional by default
		case *model.UnionType:
			if len(pclType.ElementTypes) == 2 && pclType.ElementTypes[0] == model.NoneType {
				return componentInputElementType(pclType.ElementTypes[1])
			} else if len(pclType.ElementTypes) == 2 && pclType.ElementTypes[1] == model.NoneType {
				return componentInputElementType(pclType.ElementTypes[0])
			}
			return "Any"
		default:
			return "Any"
		}
	}
}

// collectObjectTypedConfigVariables returns the object types in config variables need to be emitted
// as classes.
func collectObjectTypedConfigVariables(component *pcl.Component) map[string]*model.ObjectType {
	objectTypes := map[string]*model.ObjectType{}
	for _, config := range component.Program.ConfigVariables() {
		switch configType := config.Type().(type) {
		case *model.ObjectType:
			objectTypes[config.Name()] = configType
		case *model.ListType:
			switch elementType := configType.ElementType.(type) {
			case *model.ObjectType:
				objectTypes[config.Name()] = elementType
			}
		case *model.MapType:
			switch elementType := configType.ElementType.(type) {
			case *model.ObjectType:
				objectTypes[config.Name()] = elementType
			}
		}
	}

	return objectTypes
}

func (g *generator) genComponentDefinition(w io.Writer, component *pcl.Component, componentName string) {
	configVars := component.Program.ConfigVariables()
	hasAnyInputVariables := len(configVars) > 0
	if hasAnyInputVariables {
		objectTypedConfigs := collectObjectTypedConfigVariables(component)
		variableNames := pcl.SortedStringKeys(objectTypedConfigs)
		// generate resource args for this component
		for _, variableName := range variableNames {
			objectType := objectTypedConfigs[variableName]
			objectTypeName := title(variableName)
			g.Fprintf(w, "class %s(TypedDict, total=False):\n", objectTypeName)
			g.Indented(func() {
				propertyNames := pcl.SortedStringKeys(objectType.Properties)
				for _, propertyName := range propertyNames {
					propertyType := objectType.Properties[propertyName]
					inputType := componentInputElementType(propertyType)
					g.Fprintf(w, "%s%s: Input[%s]\n",
						g.Indent,
						propertyName,
						inputType)
				}
			})
			g.Fgen(w, "\n")
		}

		// emit args class
		g.Fgenf(w, "class %sArgs(TypedDict, total=False):\n", componentName)
		g.Indented(func() {
			// define constructor args
			for _, configVar := range configVars {
				argName := configVar.Name()
				argType := componentInputElementType(configVar.Type())
				switch configType := configVar.Type().(type) {
				case *model.ObjectType:
					// for objects of type T, generate T as is
					argType = title(configVar.Name())
				case *model.ListType:
					// for list(T) where T is an object type, generate List[T]
					switch configType.ElementType.(type) {
					case *model.ObjectType:
						objectTypeName := title(configVar.Name())
						argType = fmt.Sprintf("list(%s)", objectTypeName)
					}
				case *model.MapType:
					// for map(T) where T is an object type, generate Dict[str, T]
					switch configType.ElementType.(type) {
					case *model.ObjectType:
						objectTypeName := title(configVar.Name())
						argType = fmt.Sprintf("Dict[str, %s]", objectTypeName)
					}
				}

				argType = fmt.Sprintf("Input[%s]", argType)
				g.Fgenf(w, "%s%s: %s", g.Indent, argName, argType)
				g.Fgen(w, "\n")
			}
		})

		g.Fgen(w, "\n")
	}

	componentToken := "components:index:" + componentName
	g.Fgenf(w, "class %s(pulumi.ComponentResource):\n", componentName)
	g.Indented(func() {
		if hasAnyInputVariables {
			g.Fgenf(w, "%sdef __init__(self, name: str, args: %s, opts:Optional[pulumi.ResourceOptions] = None):\n",
				g.Indent,
				componentName+"Args")

			g.Fgenf(w, "%s%ssuper().__init__(\"%s\", name, args, opts)\n",
				g.Indent,
				g.Indent,
				componentToken)
		} else {
			g.Fgenf(w, "%sdef __init__(self, name: str, opts: Optional[pulumi.ResourceOptions] = None):\n", g.Indent)
			g.Fgenf(w, "%s%ssuper().__init__(\"%s\", name, {}, opts)\n",
				g.Indent,
				g.Indent,
				componentToken)
		}

		g.Fgen(w, "\n")

		g.Indented(func() {
			for _, node := range pcl.Linearize(component.Program) {
				switch node := node.(type) {
				case *pcl.LocalVariable:
					g.genLocalVariable(w, node)
					g.Fgen(w, "\n")
				case *pcl.Component:
					// set options { parent = self } for the component resource
					// where "self" is a reference to the component resource itself
					if node.Options == nil {
						node.Options = &pcl.ResourceOptions{}
					}

					if node.Options.Parent == nil {
						node.Options.Parent = model.ConstantReference(&model.Constant{
							Name: "self",
						})
					}
					g.genComponent(w, node)
					g.Fgen(w, "\n")
				case *pcl.Resource:
					// set options { parent = self } for the component resource
					// where "self" is a reference to the component resource itself
					if node.Options == nil {
						node.Options = &pcl.ResourceOptions{}
					}

					if node.Options.Parent == nil {
						node.Options.Parent = model.ConstantReference(&model.Constant{
							Name: "self",
						})
					}
					g.genResource(w, node)
					g.Fgen(w, "\n")
				}
			}

			outputVars := component.Program.OutputVariables()
			for _, output := range outputVars {
				g.Fgenf(w, "%sself.%s = %v\n", g.Indent, output.Name(), output.Value)
			}

			if len(outputVars) == 0 {
				g.Fgenf(w, "%sself.register_outputs()\n", g.Indent)
			} else {
				g.Fgenf(w, "%sself.register_outputs({\n", g.Indent)
				g.Indented(func() {
					for index, output := range outputVars {
						g.Fgenf(w, "%s'%s': %v", g.Indent, output.Name(), output.Value)
						if index != len(outputVars)-1 {
							g.Fgen(w, ", ")
						}
						g.Fgen(w, "\n")
					}
				})
				g.Fgenf(w, "%s})", g.Indent)
			}
		})
	})
}

func GenerateProject(
	directory string, project workspace.Project,
	program *pcl.Program, localDependencies map[string]string,
	typechecker, toolchain string,
) error {
	files, diagnostics, err := GenerateProgram(program)
	if err != nil {
		return err
	}
	if diagnostics.HasErrors() {
		return diagnostics
	}

	// Check the project for "main" as that changes where we write out files and some relative paths.
	rootDirectory := directory
	if project.Main != "" {
		directory = filepath.Join(rootDirectory, project.Main)
		// mkdir -p the subdirectory
		err = os.MkdirAll(directory, 0o700)
		if err != nil {
			return fmt.Errorf("create main directory: %w", err)
		}
	}

	var options map[string]interface{}
	if _, ok := localDependencies["pulumi"]; ok {
		options = map[string]interface{}{
			"virtualenv": "venv",
		}
	}

	if typechecker != "" {
		if options == nil {
			options = map[string]interface{}{}
		}
		options["typechecker"] = typechecker
	}

	if toolchain != "" {
		if options == nil {
			options = map[string]interface{}{}
		}
		options["toolchain"] = toolchain
	}

	// Set the runtime to "python" then marshal to Pulumi.yaml
	project.Runtime = workspace.NewProjectRuntimeInfo("python", options)
	projectBytes, err := encoding.YAML.Marshal(project)
	if err != nil {
		return err
	}

	// Build a requirements.txt based on the packages used by program
	requirementsTxtLines := []string{}
	if path, ok := localDependencies["pulumi"]; ok {
		requirementsTxtLines = append(requirementsTxtLines, path)
	} else {
		requirementsTxtLines = append(requirementsTxtLines, "pulumi>=3.0.0,<4.0.0")
	}

	// For each package add a PackageReference line
	// find references from the main/entry program and programs of components
	packages, err := program.CollectNestedPackageSnapshots()
	if err != nil {
		return err
	}

	for _, p := range packages {
		if p.Name == "pulumi" {
			continue
		}
		if path, ok := localDependencies[p.Name]; ok {
			requirementsTxtLines = append(requirementsTxtLines, path)
		} else {
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
				requirementsTxtLines = append(requirementsTxtLines, fmt.Sprintf("%s==%s", packageName, p.Version.String()))
			} else {
				requirementsTxtLines = append(requirementsTxtLines, packageName)
			}
		}
	}

	// If a typechecker is given we need to list that in the requirements.txt as well
	if typechecker != "" {
		requirementsTxtLines = append(requirementsTxtLines, typechecker)
	}

	// We want the requirements.txt files we generate to be stable, so we sort the
	// lines before obtaining the bytes.
	slices.Sort(requirementsTxtLines)
	files["requirements.txt"] = []byte(strings.Join(requirementsTxtLines, "\n") + "\n")

	// Add the language specific .gitignore
	files[".gitignore"] = []byte(`*.pyc
venv/`)

	for filename, data := range files {
		outPath := path.Join(directory, filename)
		err := os.WriteFile(outPath, data, 0o600)
		if err != nil {
			return fmt.Errorf("could not write output program: %w", err)
		}
	}

	// Write out the Pulumi.yaml
	err = os.WriteFile(path.Join(rootDirectory, "Pulumi.yaml"), projectBytes, 0o600)
	if err != nil {
		return fmt.Errorf("write Pulumi.yaml: %w", err)
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

// rewriteApplyLambdaBody rewrites the body of a lambda where it rewrites the usage of lambda variables
// into an index expression of a dictionary. for example lambda arg `value` will become <argsParamName>["value"]
func rewriteApplyLambdaBody(applyLambda *model.AnonymousFunctionExpression, argsParamName string) model.Expression {
	rewriter := func(expr model.Expression) (model.Expression, hcl.Diagnostics) {
		switch expr := expr.(type) {
		case *model.ScopeTraversalExpression:
			if len(expr.Parts) == 1 {
				// check whether this expression is traversing a lambda arg
				// rewrite arg into argsParamName["argName"]
				for _, param := range applyLambda.Signature.Parameters {
					if param.Name == expr.RootName {
						return &model.IndexExpression{
							Collection: model.VariableReference(&model.Variable{
								Name: argsParamName,
							}),
							Key: &model.LiteralValueExpression{
								Value: cty.StringVal(fmt.Sprintf("'%s'", param.Name)),
							},
						}, nil
					}
				}
			}
		}

		return expr, nil
	}

	rewrittenBody, _ := model.VisitExpression(applyLambda.Body, model.IdentityVisitor, rewriter)

	return rewrittenBody
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
			pcl.FixupPulumiPackageTokens(r)
			pkg, _, _, _ := r.DecomposeToken()
			if pkg == "pulumi" {
				continue
			}
			var packageName string
			if r.Schema != nil && r.Schema.PackageReference != nil {
				pkg, err := r.Schema.PackageReference.Definition()
				if err == nil {
					if pkgInfo, ok := pkg.Language["python"].(PackageInfo); ok && pkgInfo.PackageName != "" {
						packageName = pkgInfo.PackageName
					}
				}
				if packageName == "" {
					packageName = PyPack(pkg.Namespace, pkg.Name)
				}
			} else {
				packageName = "pulumi_" + makeValidIdentifier(pkg)
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
							Pkg:      maybePkg,
						}
					}
				}
				if helperMethodBody, ok := getHelperMethodIfNeeded(call.Name, g.Indent); ok {
					preambleHelperMethods.Add(helperMethodBody)
				}
			}
			return n, nil
		})
		contract.Assertf(len(diags) == 0, "unexpected diagnostics reported: %v", diags)
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
			imports = append(imports, fmt.Sprintf("import %s as %s", pkg, EnsureKeywordSafe(control.Pkg)))
		} else {
			imports = append(imports, "import "+pkg)
		}
	}

	if g.isComponent {
		// add typing information
		imports = append(imports, "from typing import Optional, Dict, TypedDict, Any")
		imports = append(imports, "from pulumi import Input")
	}

	seenComponentImports := map[string]bool{}
	for _, node := range program.Nodes {
		if component, ok := node.(*pcl.Component); ok {
			componentPath := strings.ReplaceAll(filepath.Base(component.DirPath()), "-", "_")
			componentName := component.DeclarationName()
			pathAndName := componentPath + "-" + componentName
			if _, ok := seenComponentImports[pathAndName]; !ok {
				imports = append(imports, fmt.Sprintf("from %s import %s", componentPath, componentName))
				seenComponentImports[pathAndName] = true
			}
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
	case *pcl.Component:
		g.genComponent(w, n)
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
	pcl.FixupPulumiPackageTokens(r)

	// Normalize module.
	if r.Schema != nil {
		pkg, err := r.Schema.PackageReference.Definition()
		if err != nil {
			diagnostics = append(diagnostics, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "unable to bind schema for resource",
				Detail:   err.Error(),
				Subject:  r.Definition.Syntax.DefRange().Ptr(),
			})
		} else {
			err = pkg.ImportLanguages(map[string]schema.Language{"python": Importer})
			contract.AssertNoErrorf(err, "failed to import python language plugin for package %s", pkg.Name)
			if lang, ok := pkg.Language["python"]; ok {
				if pkgInfo, ok := lang.(PackageInfo); ok {
					if m, ok := pkgInfo.ModuleNameOverrides[module]; ok {
						module = m
					}
				}
			}
		}
	}

	return tokenToQualifiedName(pkg, module, member), diagnostics
}

func (g *generator) typedDictEnabled(expr model.Expression, typ model.Type) bool {
	schemaType, ok := pcl.GetSchemaForType(typ)
	if !ok {
		return false
	}

	schemaType = codegen.UnwrapType(schemaType)

	objType, ok := schemaType.(*schema.ObjectType)
	if !ok {
		return false
	}

	pkg, err := objType.PackageReference.Definition()
	contract.AssertNoErrorf(err, "error loading definition for package %q", objType.PackageReference.Name())
	if lang, ok := pkg.Language["python"]; ok {
		if pkgInfo, ok := lang.(PackageInfo); ok {
			return typedDictEnabled(pkgInfo.InputTypes)
		}
	}

	return true
}

// argumentTypeName computes the Python argument class name for the given expression and model type.
func (g *generator) argumentTypeName(expr model.Expression, destType model.Type) string {
	schemaType, ok := pcl.GetSchemaForType(destType)
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
	contract.Assertf(len(diagnostics) == 0, "unexpected diagnostics reported: %v", diagnostics)

	modName := objType.PackageReference.TokenToModule(token)

	// Normalize module.
	pkg, err := objType.PackageReference.Definition()
	contract.AssertNoErrorf(err, "error loading definition for package %q", objType.PackageReference.Name())
	if lang, ok := pkg.Language["python"]; ok {
		if pkgInfo, ok := lang.(PackageInfo); ok {
			if m, ok := pkgInfo.ModuleNameOverrides[module]; ok {
				modName = m
			}
		}
	}
	return tokenToQualifiedName(pkgName, modName, member) + "Args"
}

// makeResourceName returns the expression that should be emitted for a resource's "name" parameter given its base name
// and the count variable name, if any.
func (g *generator) makeResourceName(baseName, count string) string {
	if count == "" {
		if g.isComponent {
			return fmt.Sprintf(`f"{name}-%s"`, baseName)
		}
		return fmt.Sprintf(`"%s"`, baseName)
	}

	if g.isComponent {
		return fmt.Sprintf(`f"{name}-%s-{%s}"`, baseName, count)
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

	// Reference: https://www.pulumi.com/docs/iac/concepts/options/
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
	if opts.RetainOnDelete != nil {
		appendOption("retain_on_delete", opts.RetainOnDelete)
	}
	if opts.IgnoreChanges != nil {
		appendOption("ignore_changes", opts.IgnoreChanges)
	}
	if opts.DeletedWith != nil {
		appendOption("deleted_with", opts.DeletedWith)
	}
	if opts.ImportID != nil {
		appendOption("import_", opts.ImportID)
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
	g.Fprintf(w, ",%sopts = pulumi.ResourceOptions(", prefix)
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

// genResourceDeclaration handles the generation of instantiations resources.
func (g *generator) genResourceDeclaration(w io.Writer, r *pcl.Resource, needsDefinition bool) {
	qualifiedMemberName, diagnostics := resourceTypeName(r)
	g.diagnostics = append(g.diagnostics, diagnostics...)
	optionsBag, temps := g.lowerResourceOptions(r.Options)
	name := r.LogicalName()
	nameVar := PyName(r.Name())

	if needsDefinition {
		g.genTrivia(w, r.Definition.Tokens.GetType(""))
		for _, l := range r.Definition.Tokens.Labels {
			g.genTrivia(w, l)
		}
		g.genTrivia(w, r.Definition.Tokens.GetOpenBrace())
	}

	if r.Schema != nil {
		for _, input := range r.Inputs {
			destType, diagnostics := r.InputType.Traverse(hcl.TraverseAttr{Name: input.Name})
			g.diagnostics = append(g.diagnostics, diagnostics...)
			value, valueTemps := g.lowerExpression(input.Value, destType.(model.Type))
			temps = append(temps, valueTemps...)
			input.Value = value
		}
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
				// special case: pulumi.StackReference requires `stack_name` instead of `name`
				if qualifiedMemberName == stackRefQualifiedName && propertyName == "name" {
					propertyName = "stack_name"
				}
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
		rangeType := r.Options.Range.Type()

		if model.ContainsOutputs(rangeType) {
			loweredRangeExpr, rangeExprTemps := g.lowerExpression(rangeExpr, rangeType)
			if model.InputType(model.BoolType).ConversionFrom(r.Options.Range.Type()) == model.SafeConversion {
				g.Fgenf(w, "%s%s = None\n", g.Indent, nameVar)
			} else {
				g.Fgenf(w, "%s%s = []\n", g.Indent, nameVar)
			}
			localFuncName := "create_" + PyName(r.LogicalName())

			// Generate a local definition which actually creates the resources
			g.Fgenf(w, "def %s(range_body):\n", localFuncName)
			g.Indented(func() {
				r.Options.Range = model.VariableReference(&model.Variable{
					Name:         "range_body",
					VariableType: model.ResolveOutputs(rangeExpr.Type()),
				})
				g.genResourceDeclaration(w, r, false)
				g.Fgen(w, "\n")
			})

			g.genTemps(w, rangeExprTemps)

			switch expr := loweredRangeExpr.(type) {
			case *model.FunctionCallExpression:
				if expr.Name == pcl.IntrinsicApply {
					applyArgs, applyLambda := pcl.ParseApplyCall(expr)

					// Step 1: generate the apply function call:
					if len(applyArgs) == 1 {
						// If we only have a single output, just generate a normal `.apply`
						g.Fgenf(w, "%v.apply(", applyArgs[0])
					} else {
						// Otherwise, generate a call to `pulumi.Output.all([]).apply()`.
						g.Fgen(w, "pulumi.Output.all(\n")
						g.Indented(func() {
							for i, arg := range applyArgs {
								argName := applyLambda.Signature.Parameters[i].Name
								g.Fgenf(w, "%s%s=%v", g.Indent, argName, arg)
								if i < len(applyArgs)-1 {
									g.Fgen(w, ",")
								}
								g.Fgen(w, "\n")
							}
						})
						g.Fgen(w, ").apply(")
					}

					// Step 2: apply lambda function arguments
					g.Fgen(w, "lambda resolved_outputs:")
					// Step 3: The function body is where the resources are generated:
					// The function body is also a non-output value so we rewrite the range of
					// the resource declaration to this non-output value
					rewrittenLambdaBody := rewriteApplyLambdaBody(applyLambda, "resolved_outputs")
					g.Fgenf(w, " %s(%.v))\n", localFuncName, rewrittenLambdaBody)
					return
				}

				// If we have anything else that returns output, just generate a normal `.apply`
				g.Fgenf(w, "%.20v.apply(%s)\n", loweredRangeExpr, localFuncName)
				return
			case *model.ForExpression:
				// A list generator that contains outputs looks like list(output(T))
				// when we pass that list into `Output.all` it returns a list with a single element,
				// that element is another list of all resolved items
				// that is why we index the resolved outputs at 0
				g.Fgenf(w, "pulumi.Output.all(%v).apply(lambda resolved_outputs: %s(resolved_outputs[0]))\n",
					rangeExpr,
					localFuncName)
				return
			case *model.TupleConsExpression:
				// A list that contains outputs looks like list(output(T))
				// ideally we want this to be output(list(T)) and then call apply:
				// so we call pulumi.all to lift the elements of the list, then call apply
				g.Fgen(w, "pulumi.Output.all(\n")
				g.Indented(func() {
					for i, item := range expr.Expressions {
						g.Fgenf(w, "%s%v", g.Indent, item)
						if i < len(expr.Expressions)-1 {
							g.Fgenf(w, ",")
						}

						g.Fgen(w, "\n")
					}
				})

				g.Fgenf(w, ").apply(%s)\n", localFuncName)
				return

			default:
				// If we have anything else that returns output, just generate a normal `.apply`
				g.Fgenf(w, "%v.apply(%s)\n", rangeExpr, localFuncName)
				return
			}
		}

		if model.InputType(model.BoolType).ConversionFrom(r.Options.Range.Type()) == model.SafeConversion {
			if needsDefinition {
				g.Fgenf(w, "%s%s = None\n", g.Indent, nameVar)
			}

			g.Fgenf(w, "%sif %.v:\n", g.Indent, rangeExpr)
			g.Indented(func() {
				g.Fprintf(w, "%s%s = ", g.Indent, nameVar)
				instantiate(g.makeResourceName(name, ""))
				g.Fprint(w, "\n")
			})
		} else {
			if needsDefinition {
				g.Fgenf(w, "%s%s = []\n", g.Indent, nameVar)
			}

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

// genResource handles the generation of instantiations of resources.
func (g *generator) genResource(w io.Writer, r *pcl.Resource) {
	g.genResourceDeclaration(w, r, true)
}

// genComponent handles the generation of instantiations of non-builtin resources.
func (g *generator) genComponent(w io.Writer, r *pcl.Component) {
	componentName := r.DeclarationName()
	optionsBag, temps := g.lowerResourceOptions(r.Options)
	name := r.LogicalName()
	nameVar := PyName(r.Name())

	g.genTrivia(w, r.Definition.Tokens.GetType(""))
	for _, l := range r.Definition.Tokens.Labels {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, r.Definition.Tokens.GetOpenBrace())

	// collect here all the deferred output variables
	// these must be declared before the component instantiation
	componentInputs := slice.Prealloc[*model.Attribute](len(r.Inputs))
	var componentDeferredOutputVariables []*pcl.DeferredOutputVariable
	for _, attr := range r.Inputs {
		expr, deferredOutputs := pcl.ExtractDeferredOutputVariables(g.program, r, attr.Value)
		componentInputs = append(componentInputs, &model.Attribute{
			Name:  attr.Name,
			Value: expr,
		})

		// add the deferred outputs local to this component
		componentDeferredOutputVariables = append(componentDeferredOutputVariables, deferredOutputs...)
		// add the deferred outputs to the global list of the program
		// such that we can emit the resolution statement at the end
		// of the component declaration (from which the output is resolved)
		g.deferredOutputVariables = append(g.deferredOutputVariables, deferredOutputs...)
	}

	for _, input := range componentInputs {
		value, valueTemps := g.lowerExpression(input.Value, input.Value.Type())
		temps = append(temps, valueTemps...)
		input.Value = value
	}
	g.genTemps(w, temps)

	declareDeferredOutputVariables := func() {
		for _, output := range componentDeferredOutputVariables {
			g.Fgenf(w, "%s", g.Indent)
			g.Fgenf(w, "%s, resolve_%s = pulumi.deferred_output()\n",
				PyName(output.Name),
				PyName(output.Name))
		}
	}

	hasInputVariables := len(r.Program.ConfigVariables()) > 0
	instantiate := func(resName string) {
		if hasInputVariables {
			g.Fgenf(w, "%s(%s, {\n", componentName, resName)
		} else {
			g.Fgenf(w, "%s(%s", componentName, resName)
		}
		g.Indented(func() {
			for index, attr := range componentInputs {
				propertyName := attr.Name
				g.Fgenf(w, "%s'%s': %.v", g.Indent, propertyName, attr.Value)

				if index != len(r.Inputs)-1 {
					// add comma after each input when that property is not the last
					g.Fgen(w, ", ")
					if len(r.Inputs) > 1 {
						g.Fgen(w, "\n")
					}
				}
			}
			g.genResourceOptions(w, optionsBag, len(r.Inputs) != 0)
		})

		if hasInputVariables {
			g.Fgenf(w, "%s})", g.Indent)
		} else {
			g.Fgen(w, ")")
		}
	}

	if r.Options != nil && r.Options.Range != nil {
		rangeExpr := r.Options.Range
		if model.InputType(model.BoolType).ConversionFrom(r.Options.Range.Type()) == model.SafeConversion {
			g.Fgenf(w, "%s%s = None\n", g.Indent, nameVar)
			g.Fgenf(w, "%sif %.v:\n", g.Indent, rangeExpr)
			g.Indented(func() {
				declareDeferredOutputVariables()
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
				declareDeferredOutputVariables()
				g.Fgenf(w, "%s%s.append(", g.Indent, nameVar)
				instantiate(resName)
				g.Fprint(w, ")\n")
			})
		}
	} else {
		declareDeferredOutputVariables()
		g.Fgenf(w, "%s%s = ", g.Indent, nameVar)
		instantiate(g.makeResourceName(name, ""))
		g.Fprint(w, "\n")
	}

	// resolve the deferred output variables from this component
	for _, output := range g.deferredOutputVariables {
		if output.SourceComponent.Name() == r.Name() {
			g.Fgenf(w, "%s", g.Indent)
			expr, temps := g.lowerExpression(output.Expr, output.Expr.Type())
			g.genTemps(w, temps)
			if _, ok := output.Expr.(*model.ScopeTraversalExpression); ok {
				g.Fgenf(w, "resolve_%s(%v)\n", PyName(output.Name), expr)
			} else {
				g.Fgenf(w, "resolve_%s(pulumi.Output.from_input(%v))\n", PyName(output.Name), expr)
			}
		}
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

	if v.Description != "" {
		for _, line := range strings.Split(v.Description, "\n") {
			g.Fgenf(w, "%s# %s\n", g.Indent, line)
		}
	}
	name := PyName(v.Name())
	g.Fgenf(w, "%s%s = config.%s%s(\"%s\")\n", g.Indent, name, getOrRequire, getType, v.LogicalName())
	if defaultValue != nil {
		g.Fgenf(w, "%sif %s is None:\n", g.Indent, name)
		g.Indented(func() {
			g.Fgenf(w, "%s%s = %.v\n", g.Indent, name, defaultValue)
		})
	}
}

func (g *generator) genLocalVariable(w io.Writer, v *pcl.LocalVariable) {
	value, temps := g.lowerExpression(v.Definition.Value, v.Type())
	g.genTemps(w, temps)

	g.genTrivia(w, v.Definition.Tokens.Name)
	g.Fgenf(w, "%s%s = %.v\n", g.Indent, PyName(v.Name()), value)
}

func (g *generator) genOutputVariable(w io.Writer, v *pcl.OutputVariable) {
	value, temps := g.lowerExpression(v.Value, v.Type())
	g.genTemps(w, temps)

	// TODO(pdg): trivia
	g.Fgenf(w, "%spulumi.export(\"%s\", %.v)\n", g.Indent, v.LogicalName(), value)
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	message := "not yet implemented: " + fmt.Sprintf(reason, vs...)
	g.diagnostics = append(g.diagnostics, &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  message,
		Detail:   message,
	})
	g.Fgenf(w, "(lambda: raise Exception(%q))()", fmt.Sprintf(reason, vs...))
}
