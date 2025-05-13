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
	"os"
	"path"
	"path/filepath"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/zclconf/go-cty/cty"
)

const PulumiToken = "pulumi"

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter

	program     *pcl.Program
	diagnostics hcl.Diagnostics

	asyncMain               bool
	configCreated           bool
	isComponent             bool
	deferredOutputVariables []*pcl.DeferredOutputVariable
}

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	pcl.MapProvidersAsResources(program)
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := pcl.Linearize(program)

	g := &generator{
		program: program,
	}
	g.Formatter = format.NewFormatter(g)

	packages, err := program.PackageSnapshots()
	if err != nil {
		return nil, nil, err
	}
	for _, p := range packages {
		if err := p.ImportLanguages(map[string]schema.Language{"nodejs": Importer}); err != nil {
			return nil, nil, err
		}
	}

	var index bytes.Buffer
	err = g.genPreamble(&index, program)
	if err != nil {
		return nil, nil, err
	}
	// used to track declared variables in the main program
	// since outputs have identifiers which can conflict with other program nodes' identifiers
	// we switch the entry point to async which allows for declaring arbitrary output names
	declaredNodeIdentifiers := map[string]bool{}
	for _, n := range nodes {
		if g.asyncMain {
			break
		}
		switch x := n.(type) {
		case *pcl.Resource:
			if resourceRequiresAsyncMain(x) {
				g.asyncMain = true
			}
			declaredNodeIdentifiers[makeValidIdentifier(x.Name())] = true
		case *pcl.ConfigVariable:
			declaredNodeIdentifiers[makeValidIdentifier(x.Name())] = true
		case *pcl.LocalVariable:
			declaredNodeIdentifiers[makeValidIdentifier(x.Name())] = true
		case *pcl.Component:
			declaredNodeIdentifiers[makeValidIdentifier(x.Name())] = true
		case *pcl.OutputVariable:
			if outputRequiresAsyncMain(x) {
				g.asyncMain = true
			}

			outputIdentifier := makeValidIdentifier(x.Name())
			if _, alreadyDeclared := declaredNodeIdentifiers[outputIdentifier]; alreadyDeclared {
				g.asyncMain = true
			}
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
				if o, ok := n.(*pcl.OutputVariable); ok {
					if result == nil {
						result = &model.ObjectConsExpression{}
					}
					name := o.LogicalName()
					result.Items = append(result.Items, model.ObjectConsItem{
						Key:   &model.LiteralValueExpression{Value: cty.StringVal(name)},
						Value: g.lowerExpression(o.Value, o.Type()),
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

	for componentDir, component := range program.CollectComponents() {
		componentFilename := filepath.Base(componentDir)
		componentName := component.DeclarationName()
		componentGenerator := &generator{
			program:     component.Program,
			isComponent: true,
		}

		componentGenerator.Formatter = format.NewFormatter(componentGenerator)

		var componentBuffer bytes.Buffer
		componentGenerator.genComponentResourceDefinition(&componentBuffer, componentName, component)
		files[componentFilename+".ts"] = componentBuffer.Bytes()
	}

	return files, g.diagnostics, nil
}

func GenerateProject(
	directory string, project workspace.Project,
	program *pcl.Program, localDependencies map[string]string,
	forceTsc bool,
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

	// Set the runtime to "nodejs" then marshal to Pulumi.yaml
	runtime := workspace.NewProjectRuntimeInfo("nodejs", nil)
	if forceTsc {
		runtime.SetOption("typescript", false)
	}
	project.Runtime = runtime

	projectBytes, err := encoding.YAML.Marshal(project)
	if err != nil {
		return err
	}
	err = os.WriteFile(path.Join(rootDirectory, "Pulumi.yaml"), projectBytes, 0o600)
	if err != nil {
		return fmt.Errorf("write Pulumi.yaml: %w", err)
	}

	// Build the package.json
	var packageJSON bytes.Buffer
	fmt.Fprintf(&packageJSON, `{
	"name": "%s",
	"devDependencies": {
		"@types/node": "%s"
	},
	"dependencies": {
		"typescript": "^4.0.0",
		`, project.Name.String(), MinimumNodeTypesVersion)

	// Check if pulumi is a local dependency, else add it as a normal range dependency
	if pulumiArtifact, has := localDependencies[PulumiToken]; has {
		fmt.Fprintf(&packageJSON, `"@pulumi/pulumi": "%s"`, pulumiArtifact)
	} else {
		fmt.Fprintf(&packageJSON, `"@pulumi/pulumi": "^3.0.0"`)
	}

	// For each package add a dependency line
	packages, err := program.CollectNestedPackageSnapshots()
	if err != nil {
		return err
	}
	// Sort the dependencies to ensure a deterministic package.json. Note that the typescript and
	// @pulumi/pulumi dependencies are already added above and not sorted.
	sortedPackageNames := make([]string, 0, len(packages))
	for k := range packages {
		sortedPackageNames = append(sortedPackageNames, k)
	}
	sort.Strings(sortedPackageNames)
	for _, k := range sortedPackageNames {
		p := packages[k]
		if p.Name == PulumiToken {
			continue
		}
		if err := p.ImportLanguages(map[string]schema.Language{"nodejs": Importer}); err != nil {
			return err
		}

		namespace := "@pulumi"
		if p.Namespace != "" {
			namespace = "@" + p.Namespace
		}
		packageName := namespace + "/" + p.Name
		err := p.ImportLanguages(map[string]schema.Language{"nodejs": Importer})
		if err != nil {
			return err
		}
		if langInfo, found := p.Language["nodejs"]; found {
			nodeInfo, ok := langInfo.(NodePackageInfo)
			if ok && nodeInfo.PackageName != "" {
				packageName = nodeInfo.PackageName
			}
		}

		dependencyTemplate := ",\n		\"%s\": \"%s\""
		if path, has := localDependencies[p.Name]; has {
			fmt.Fprintf(&packageJSON, dependencyTemplate, packageName, path)
		} else {
			if p.Version != nil {
				fmt.Fprintf(&packageJSON, dependencyTemplate, packageName, p.Version.String())
			} else {
				fmt.Fprintf(&packageJSON, dependencyTemplate, packageName, "*")
			}
		}
	}
	packageJSON.WriteString(`
	}
}`)

	files["package.json"] = packageJSON.Bytes()

	// Add the language specific .gitignore
	files[".gitignore"] = []byte(`/bin/
/node_modules/`)

	// Add the basic tsconfig
	var tsConfig bytes.Buffer
	tsConfig.WriteString(`{
	"compilerOptions": {
		"strict": true,
		"outDir": "bin",
		"target": "es2016",
		"module": "commonjs",
		"moduleResolution": "node",
		"sourceMap": true,
		"experimentalDecorators": true,
		"pretty": true,
		"noFallthroughCasesInSwitch": true,
		"noImplicitReturns": true,
		"forceConsistentCasingInFileNames": true
	},
	"files": [
`)

	fileNames := make([]string, 0, len(files))
	for file := range files {
		fileNames = append(fileNames, file)
	}
	sort.Strings(fileNames)

	for i, file := range fileNames {
		if strings.HasSuffix(file, ".ts") {
			tsConfig.WriteString("		\"" + file + "\"")
			lastFile := i == len(files)-1
			if !lastFile {
				tsConfig.WriteString(",\n")
			} else {
				tsConfig.WriteString("\n")
			}
		}
	}

	tsConfig.WriteString(`	]
}`)
	files["tsconfig.json"] = tsConfig.Bytes()

	for filename, data := range files {
		outPath := path.Join(directory, filename)
		err := os.WriteFile(outPath, data, 0o600)
		if err != nil {
			return fmt.Errorf("could not write output program: %w", err)
		}
	}

	return nil
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

type programImports struct {
	importStatements      []string
	preambleHelperMethods codegen.StringSet
}

func (g *generator) collectProgramImports(program *pcl.Program) programImports {
	importSet := codegen.NewStringSet("@pulumi/pulumi")
	preambleHelperMethods := codegen.NewStringSet()

	// This map tracks the package tokens by the associated import.
	//
	// It will have entries similar to the following:
	//
	// npmToPuPkgName["@pulumiverse/scaleway"] = "scaleway"
	npmToPuPkgName := make(map[string]string)

	var componentImports []string
	seenComponentImports := map[string]bool{}

	// Index known PackageReference by corresponding package name token.
	knownPackageRefsByPkg := make(map[string]schema.PackageReference)
	for _, pkgRef := range program.PackageReferences() {
		// Can more than one package be bound to the same name? It seems possible in case of components.
		knownPackageRefsByPkg[pkgRef.Name()] = pkgRef
	}

	// Collect imports for a package; optionally supply packageRef if handily available, otherwise pass nil.
	visitPkg := func(pkg string, packageRef schema.PackageReference) {
		if packageRef == nil {
			packageRef = knownPackageRefsByPkg[pkg]
		}
		if pkg == PulumiToken {
			return
		}
		namespace := "@pulumi"
		if packageRef != nil && packageRef.Namespace() != "" {
			namespace = "@" + packageRef.Namespace()
		}
		pkgName := namespace + "/" + pkg
		if packageRef != nil {
			def, err := packageRef.Definition()
			contract.AssertNoErrorf(err, "Should be able to retrieve definition for %q", pkg)

			if info, ok := def.Language["nodejs"].(NodePackageInfo); ok && info.PackageName != "" {
				pkgName = info.PackageName
			}
			npmToPuPkgName[pkgName] = pkg
		}
		importSet.Add(pkgName)
	}

	// Like visitPkg but PakckageReference is not handily available.
	visitPkgWithoutRef := func(pkg string) {
		visitPkg(pkg, nil)
	}

	// Notes direct npm imports.
	visitDirectImport := func(nodeImportString string) {
		if nodeImportString == "" {
			return
		}
		importSet.Add(nodeImportString)
	}

	for _, n := range program.Nodes {
		switch n := n.(type) {
		case *pcl.Resource:
			pkg, _, _, _ := n.DecomposeToken()
			var packageRef schema.PackageReference
			if n.Schema != nil && n.Schema.PackageReference != nil {
				packageRef = n.Schema.PackageReference
			}
			visitPkg(pkg, packageRef)
		case *pcl.Component:
			componentDir := filepath.Base(n.DirPath())
			componentName := n.DeclarationName()
			dirAndName := componentDir + "-" + componentName
			if _, ok := seenComponentImports[dirAndName]; !ok {
				importStatement := fmt.Sprintf("import { %s } from \"./%s\";", componentName, componentDir)
				componentImports = append(componentImports, importStatement)
				seenComponentImports[dirAndName] = true
			}
		}
		diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
			if call, ok := n.(*model.FunctionCallExpression); ok {
				g.visitFunctionImports(call, visitDirectImport, visitPkgWithoutRef)
				if helperMethodBody, ok := getHelperMethodIfNeeded(call, g.Indent); ok {
					preambleHelperMethods.Add(helperMethodBody)
				}
			}
			return n, nil
		})
		contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)
	}

	sortedValues := importSet.SortedValues()
	imports := slice.Prealloc[string](len(sortedValues))
	for _, pkg := range sortedValues {
		if pkg == "@pulumi/pulumi" {
			continue
		}
		var as string
		if puPkg, ok := npmToPuPkgName[pkg]; ok {
			as = makeValidIdentifier(puPkg)
		} else {
			as = makeValidIdentifier(path.Base(pkg))
		}
		imports = append(imports, fmt.Sprintf("import * as %v from \"%v\";", as, pkg))
	}

	imports = append(imports, componentImports...)
	sort.Strings(imports)

	return programImports{
		importStatements:      imports,
		preambleHelperMethods: preambleHelperMethods,
	}
}

func (g *generator) genPreamble(w io.Writer, program *pcl.Program) error {
	// Print the @pulumi/pulumi import at the top.
	g.Fprintln(w, `import * as pulumi from "@pulumi/pulumi";`)

	programImports := g.collectProgramImports(program)

	// Now sort the imports and emit them.
	for _, i := range programImports.importStatements {
		g.Fprintln(w, i)
	}
	g.Fprint(w, "\n")

	// If we collected any helper methods that should be added, write them just before the main func
	for _, preambleHelperMethodBody := range programImports.preambleHelperMethods.SortedValues() {
		g.Fprintf(w, "%s\n\n", preambleHelperMethodBody)
	}
	return nil
}

func componentElementType(pclType model.Type) string {
	switch pclType {
	case model.BoolType:
		return "boolean"
	case model.IntType, model.NumberType:
		return "number"
	case model.StringType:
		return "string"
	default:
		switch pclType := pclType.(type) {
		case *model.ListType:
			elementType := componentElementType(pclType.ElementType)
			return elementType + "[]"
		case *model.MapType:
			elementType := componentElementType(pclType.ElementType)
			return fmt.Sprintf("Record<string, pulumi.Input<%s>>", elementType)
		case *model.OutputType:
			// something is already an output
			// get only the element type because we are wrapping these in Output<T> anyway
			return componentElementType(pclType.ElementType)
		case *model.UnionType:
			if len(pclType.ElementTypes) == 2 && pclType.ElementTypes[0] == model.NoneType {
				return componentElementType(pclType.ElementTypes[1])
			} else if len(pclType.ElementTypes) == 2 && pclType.ElementTypes[1] == model.NoneType {
				return componentElementType(pclType.ElementTypes[0])
			}
			return "any"
		default:
			return "any"
		}
	}
}

func componentInputType(pclType model.Type) string {
	elementType := componentElementType(pclType)
	return fmt.Sprintf("pulumi.Input<%s>", elementType)
}

func componentOutputType(pclType model.Type) string {
	elementType := componentElementType(pclType)
	return fmt.Sprintf("pulumi.Output<%s>", elementType)
}

func (g *generator) genObjectTypedConfig(w io.Writer, objectType *model.ObjectType) {
	attributeKeys := []string{}
	for attributeKey := range objectType.Properties {
		attributeKeys = append(attributeKeys, attributeKey)
	}

	// get deterministically sorted keys
	sort.Strings(attributeKeys)

	g.Fgenf(w, "{\n")
	g.Indented(func() {
		for _, attributeKey := range attributeKeys {
			attributeType := objectType.Properties[attributeKey]
			optional := "?"
			g.Fgenf(w, "%s", g.Indent)
			typeName := componentInputType(attributeType)
			g.Fgenf(w, "%s%s: %s,\n", attributeKey, optional, typeName)
		}
	})
	g.Fgenf(w, "%s}", g.Indent)
}

func (g *generator) genComponentResourceDefinition(w io.Writer, componentName string, component *pcl.Component) {
	// Print the @pulumi/pulumi import at the top.
	g.Fprintln(w, `import * as pulumi from "@pulumi/pulumi";`)

	programImports := g.collectProgramImports(component.Program)

	// Now sort the imports and emit them.
	for _, i := range programImports.importStatements {
		g.Fprintln(w, i)
	}
	g.Fprint(w, "\n")

	// If we collected any helper methods that should be added, write them just before the main func
	for _, preambleHelperMethodBody := range programImports.preambleHelperMethods.SortedValues() {
		g.Fprintf(w, "%s\n\n", preambleHelperMethodBody)
	}

	configVars := component.Program.ConfigVariables()

	if len(configVars) > 0 {
		g.Fgenf(w, "interface %sArgs {\n", componentName)
		g.Indented(func() {
			for _, configVar := range configVars {
				optional := "?"
				if configVar.DefaultValue == nil {
					optional = ""
				}
				if configVar.Description != "" {
					g.Fgenf(w, "%s/**\n", g.Indent)
					for _, line := range strings.Split(configVar.Description, "\n") {
						g.Fgenf(w, "%s * %s\n", g.Indent, line)
					}
					g.Fgenf(w, "%s */\n", g.Indent)
				}

				g.Fgenf(w, "%s", g.Indent)
				configVarName := makeValidIdentifier(configVar.Name())
				switch configVarType := configVar.Type().(type) {
				case *model.ObjectType:
					// generate {...}
					g.Fgenf(w, "%s%s: ", configVarName, optional)
					g.genObjectTypedConfig(w, configVarType)
					g.Fgen(w, ",\n")
				case *model.ListType:
					switch elementType := configVarType.ElementType.(type) {
					case *model.ObjectType:
						// generate {...}[]
						g.Fgenf(w, "%s%s: ", configVarName, optional)
						g.genObjectTypedConfig(w, elementType)
						g.Fgen(w, "[],\n")
					default:
						typeName := componentInputType(configVar.Type())
						g.Fgenf(w, "%s%s: %s,\n", configVarName, optional, typeName)
					}
				case *model.MapType:
					switch elementType := configVarType.ElementType.(type) {
					case *model.ObjectType:
						// generate Record<string, {...}>
						g.Fgenf(w, "%s%s: Record<string, ", configVarName, optional)
						g.genObjectTypedConfig(w, elementType)
						g.Fgen(w, ">,\n")
					default:
						typeName := componentInputType(configVar.Type())
						g.Fgenf(w, "%s%s: %s,\n", configVarName, optional, typeName)
					}
				default:
					typeName := componentInputType(configVar.Type())
					g.Fgenf(w, "%s%s: %s,\n", configVarName, optional, typeName)
				}
			}
		})
		g.Fgenf(w, "}\n\n")
	}

	outputs := component.Program.OutputVariables()

	g.Fgenf(w, "export class %s extends pulumi.ComponentResource {\n", componentName)
	g.Indented(func() {
		for _, output := range outputs {
			var outputType string
			switch expr := output.Value.(type) {
			case *model.ScopeTraversalExpression:
				resource, ok := expr.Parts[0].(*pcl.Resource)
				if ok && len(expr.Parts) == 1 {
					pkg, module, memberName, diagnostics := resourceTypeName(resource)
					g.diagnostics = append(g.diagnostics, diagnostics...)

					if module != "" {
						module = "." + module
					}

					qualifiedMemberName := fmt.Sprintf("%s%s.%s", pkg, module, memberName)
					// special case: the output is a Resource type
					outputType = fmt.Sprintf("pulumi.Output<%s>", qualifiedMemberName)
				} else {
					outputType = componentOutputType(expr.Type())
				}
			default:
				outputType = componentOutputType(expr.Type())
			}
			g.Fgenf(w, "%s", g.Indent)
			g.Fgenf(w, "public %s: %s;\n", makeValidIdentifier(output.Name()), outputType)
		}

		token := "components:index:" + componentName

		if len(configVars) == 0 {
			g.Fgenf(w, "%s", g.Indent)
			g.Fgen(w, "constructor(name: string, opts?: pulumi.ComponentResourceOptions) {\n")
			g.Indented(func() {
				g.Fgenf(w, "%s", g.Indent)
				g.Fgenf(w, "super(\"%s\", name, {}, opts);\n", token)
			})
		} else {
			g.Fgenf(w, "%s", g.Indent)
			argsTypeName := componentName + "Args"
			g.Fgenf(w, "constructor(name: string, args: %s, opts?: pulumi.ComponentResourceOptions) {\n",
				argsTypeName)
			g.Indented(func() {
				g.Fgenf(w, "%s", g.Indent)
				g.Fgenf(w, "super(\"%s\", name, args, opts);\n", token)
			})
		}

		// generate component resources and local variables
		g.Indented(func() {
			// assign default values to config inputs
			for _, configVar := range configVars {
				if configVar.DefaultValue != nil {
					g.Fgenf(w, "%sargs.%s = args.%s || %v;\n",
						g.Indent,
						makeValidIdentifier(configVar.Name()),
						makeValidIdentifier(configVar.Name()),
						configVar.DefaultValue)
				}
			}

			for _, node := range pcl.Linearize(component.Program) {
				switch node := node.(type) {
				case *pcl.LocalVariable:
					g.genLocalVariable(w, node)
					g.Fgen(w, "\n")
				case *pcl.Component:
					if node.Options == nil {
						node.Options = &pcl.ResourceOptions{}
					}

					if node.Options.Parent == nil {
						node.Options.Parent = model.ConstantReference(&model.Constant{
							Name: "this",
						})
					}
					g.genComponent(w, node)
					g.Fgen(w, "\n")
				case *pcl.Resource:
					if node.Options == nil {
						node.Options = &pcl.ResourceOptions{}
					}

					if node.Options.Parent == nil {
						node.Options.Parent = model.ConstantReference(&model.Constant{
							Name: "this",
						})
					}
					g.genResource(w, node)
					g.Fgen(w, "\n")
				}
			}

			registeredOutputs := &model.ObjectConsExpression{}
			for _, output := range outputs {
				// assign the output fields
				outputProperty := makeValidIdentifier(output.Name())
				switch expr := output.Value.(type) {
				case *model.ScopeTraversalExpression:
					_, ok := expr.Parts[0].(*pcl.Resource)
					if ok && len(expr.Parts) == 1 {
						// special case: the output is a Resource type
						g.Fgenf(w, "%sthis.%s = pulumi.output(%v);\n",
							g.Indent, outputProperty,
							g.lowerExpression(output.Value, output.Type()))
					} else {
						g.Fgenf(w, "%sthis.%s = %v;\n",
							g.Indent, outputProperty,
							g.lowerExpression(output.Value, output.Type()))
					}
				default:
					g.Fgenf(w, "%sthis.%s = %v;\n",
						g.Indent, outputProperty,
						g.lowerExpression(output.Value, output.Type()))
				}
				// add the outputs to abject for registration
				registeredOutputs.Items = append(registeredOutputs.Items, model.ObjectConsItem{
					Key: &model.LiteralValueExpression{
						Tokens: syntax.NewLiteralValueTokens(cty.StringVal(outputProperty)),
						Value:  cty.StringVal(outputProperty),
					},
					Value: output.Value,
				})
			}

			if len(outputs) == 0 {
				g.Fgenf(w, "%sthis.registerOutputs();\n", g.Indent)
			} else {
				g.Fgenf(w, "%sthis.registerOutputs(%v);\n", g.Indent, registeredOutputs)
			}
		})

		g.Fgenf(w, "%s}\n", g.Indent)
	})
	g.Fgen(w, "}\n")
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

func resourceRequiresAsyncMain(r *pcl.Resource) bool {
	if r.Options == nil || r.Options.Range == nil {
		return false
	}

	return model.ContainsPromises(r.Options.Range.Type())
}

func outputRequiresAsyncMain(ov *pcl.OutputVariable) bool {
	outputName := ov.LogicalName()
	return makeValidIdentifier(outputName) != outputName
}

// resourceTypeName computes the NodeJS package, module, and type name for the given resource.
func resourceTypeName(r *pcl.Resource) (string, string, string, hcl.Diagnostics) {
	// Compute the resource type from the Pulumi type token.
	pcl.FixupPulumiPackageTokens(r)
	pkg, module, member, diagnostics := r.DecomposeToken()

	if r.Schema != nil {
		module = moduleName(module, r.Schema.PackageReference)
	}

	return makeValidIdentifier(pkg), module, title(member), diagnostics
}

func moduleName(module string, pkg schema.PackageReference) string {
	// Normalize module.
	if pkg != nil {
		if a, err := pkg.Language("nodejs"); err == nil {
			pkgInfo, _ := a.(NodePackageInfo)
			if m, ok := pkgInfo.ModuleToPackage[module]; ok {
				module = m
			}
		}
	}
	return strings.ToLower(strings.ReplaceAll(module, "/", "."))
}

// makeResourceName returns the expression that should be emitted for a resource's "name" parameter given its base name
// and the count variable name, if any.
func (g *generator) makeResourceName(baseName, count string) string {
	if count == "" {
		if g.isComponent {
			return fmt.Sprintf("`${name}-%s`", baseName)
		}
		return fmt.Sprintf(`"%s"`, baseName)
	}

	if g.isComponent {
		return fmt.Sprintf("`${name}-%s-${%s}`", baseName, count)
	}
	return fmt.Sprintf("`%s-${%s}`", baseName, count)
}

func (g *generator) genResourceOptions(opts *pcl.ResourceOptions) string {
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

	// Reference: https://www.pulumi.com/docs/iac/concepts/options/
	if opts.Parent != nil {
		appendOption("parent", opts.Parent)
	}
	if opts.Provider != nil {
		appendOption("provider", opts.Provider)
	}
	if opts.Providers != nil {
		appendOption("providers", opts.Providers)
	}
	if opts.DependsOn != nil {
		appendOption("dependsOn", opts.DependsOn)
	}
	if opts.Protect != nil {
		appendOption("protect", opts.Protect)
	}
	if opts.RetainOnDelete != nil {
		appendOption("retainOnDelete", opts.RetainOnDelete)
	}
	if opts.IgnoreChanges != nil {
		appendOption("ignoreChanges", opts.IgnoreChanges)
	}
	if opts.DeletedWith != nil {
		appendOption("deletedWith", opts.DeletedWith)
	}
	if opts.ImportID != nil {
		appendOption("import", opts.ImportID)
	}

	if object == nil {
		return ""
	}

	var buffer bytes.Buffer
	g.Fgenf(&buffer, ", %v", g.lowerExpression(object, nil))
	return buffer.String()
}

// genResourceDeclaration handles the generation of instantiations of resources.
func (g *generator) genResourceDeclaration(w io.Writer, r *pcl.Resource, needsDefinition bool) {
	pkg, module, memberName, diagnostics := resourceTypeName(r)
	g.diagnostics = append(g.diagnostics, diagnostics...)

	if module != "" {
		module = "." + module
	}

	qualifiedMemberName := fmt.Sprintf("%s%s.%s", pkg, module, memberName)

	optionsBag := g.genResourceOptions(r.Options)

	name := r.LogicalName()
	variableName := makeValidIdentifier(r.Name())

	if needsDefinition {
		g.genTrivia(w, r.Definition.Tokens.GetType(""))
		for _, l := range r.Definition.Tokens.GetLabels(nil) {
			g.genTrivia(w, l)
		}
		g.genTrivia(w, r.Definition.Tokens.GetOpenBrace())
	}

	instantiate := func(resName string) {
		g.Fgenf(w, "new %s(%s, {", qualifiedMemberName, resName)
		indenter := func(f func()) { f() }
		if len(r.Inputs) > 1 {
			indenter = g.Indented
		}
		indenter(func() {
			fmtString := "%s: %.v"
			if len(r.Inputs) > 1 {
				fmtString = "\n" + g.Indent + "%s: %.v,"
			}

			for _, attr := range r.Inputs {
				propertyName := attr.Name
				if !isLegalIdentifier(propertyName) {
					propertyName = fmt.Sprintf("%q", propertyName)
				}

				// Rewrite variable names separate from lowering to avoid rewriting
				// keywords that are generated elsewhere (e.g. `this` in the case of
				// setting a resource parent).
				x, diagnostics := g.RewriteVariableRenames(attr.Value, attr.Value.Type())
				g.diagnostics = append(g.diagnostics, diagnostics...)

				if r.Schema != nil {
					destType, diagnostics := r.InputType.Traverse(hcl.TraverseAttr{Name: attr.Name})
					g.diagnostics = append(g.diagnostics, diagnostics...)

					g.Fgenf(w, fmtString, propertyName,
						g.lowerExpression(x, destType.(model.Type)))
				} else {
					g.Fgenf(w, fmtString, propertyName, x)
				}
			}
		})
		if len(r.Inputs) > 1 {
			g.Fgenf(w, "\n%s", g.Indent)
		}
		g.Fgenf(w, "}%s)", optionsBag)
	}

	if r.Options != nil && r.Options.Range != nil {
		rangeType := r.Options.Range.Type()
		rangeExpr := r.Options.Range
		if model.ContainsOutputs(r.Options.Range.Type()) {
			rangeExpr = g.lowerExpression(rangeExpr, rangeType)
			if model.InputType(model.BoolType).ConversionFrom(rangeType) == model.SafeConversion {
				g.Fgenf(w, "%slet %s: %s | undefined;\n", g.Indent, variableName, qualifiedMemberName)
			} else {
				g.Fgenf(w, "%sconst %s: %s[] = [];\n", g.Indent, variableName, qualifiedMemberName)
			}

			switch expr := rangeExpr.(type) {
			case *model.FunctionCallExpression:
				if expr.Name == pcl.IntrinsicApply {
					applyArgs, applyLambda := pcl.ParseApplyCall(expr)
					// Step 1: generate the apply function call:
					if len(applyArgs) == 1 {
						// If we only have a single output, just generate a normal `.apply`
						g.Fgenf(w, "%.20v.apply(", applyArgs[0])
					} else {
						// Otherwise, generate a call to `pulumi.all([]).apply()`.
						g.Fgen(w, "pulumi.all([")
						for i, o := range applyArgs {
							if i > 0 {
								g.Fgen(w, ", ")
							}
							g.Fgenf(w, "%v", o)
						}
						g.Fgen(w, "]).apply(")
					}

					// Step 2: apply lambda function arguments
					switch len(applyLambda.Signature.Parameters) {
					case 0:
						g.Fgen(w, "()")
					case 1:
						g.Fgenf(w, "%s", applyLambda.Signature.Parameters[0].Name)
					default:
						g.Fgen(w, "([")
						for i, p := range applyLambda.Signature.Parameters {
							if i > 0 {
								g.Fgen(w, ", ")
							}
							g.Fgenf(w, "%s", p.Name)
						}
						g.Fgen(w, "])")
					}

					// Step 3: The function body is where the resources are generated:
					// The function body is also a non-output value so we rewrite the range of
					// the resource declaration to this non-output value
					g.Fgen(w, " => {\n")
					g.Indented(func() {
						r.Options.Range = applyLambda.Body
						g.genResourceDeclaration(w, r, false)
					})
					g.Fgenf(w, "%s});\n", g.Indent)
					return
				}

				// If we have anything else that returns output, just generate a normal `.apply`
				g.Fgenf(w, "%.20v.apply(rangeBody => {\n", rangeExpr)
				g.Indented(func() {
					r.Options.Range = model.VariableReference(&model.Variable{
						Name:         "rangeBody",
						VariableType: model.ResolveOutputs(rangeExpr.Type()),
					})
					g.genResourceDeclaration(w, r, false)
				})
				g.Fgenf(w, "%s});\n", g.Indent)
				return
			case *model.TupleConsExpression, *model.ForExpression:
				// A list or list generator that contains outputs looks like list(output(T))
				// ideally we want this to be output(list(T)) and then call apply:
				// so we call pulumi.all to lift the elements of the list, then call apply
				g.Fgenf(w, "pulumi.all(%.20v).apply(rangeBody => {\n", rangeExpr)
				g.Indented(func() {
					r.Options.Range = model.VariableReference(&model.Variable{
						Name:         "rangeBody",
						VariableType: model.ResolveOutputs(rangeExpr.Type()),
					})
					g.genResourceDeclaration(w, r, false)
				})
				g.Fgenf(w, "%s});\n", g.Indent)
				return

			default:
				// If we have anything else that returns output, just generate a normal `.apply`
				g.Fgenf(w, "%.20v.apply(rangeBody => {\n", rangeExpr)
				g.Indented(func() {
					r.Options.Range = model.VariableReference(&model.Variable{
						Name:         "rangeBody",
						VariableType: model.ResolveOutputs(rangeExpr.Type()),
					})
					g.genResourceDeclaration(w, r, false)
				})
				g.Fgenf(w, "%s});\n", g.Indent)
				return
			}
		}
		if model.InputType(model.BoolType).ConversionFrom(rangeType) == model.SafeConversion {
			if needsDefinition {
				g.Fgenf(w, "%slet %s: %s | undefined;\n", g.Indent, variableName, qualifiedMemberName)
			}

			g.Fgenf(w, "%sif (%.v) {\n", g.Indent, rangeExpr)
			g.Indented(func() {
				g.Fgenf(w, "%s%s = ", g.Indent, variableName)
				instantiate(g.makeResourceName(name, ""))
				g.Fgenf(w, ";\n")
			})
			g.Fgenf(w, "%s}\n", g.Indent)
		} else {
			if needsDefinition {
				g.Fgenf(w, "%sconst %s: %s[] = [];\n", g.Indent, variableName, qualifiedMemberName)
			}
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

func (g *generator) genResource(w io.Writer, r *pcl.Resource) {
	g.genResourceDeclaration(w, r, true)
}

// genResource handles the generation of instantiations of non-builtin resources.
func (g *generator) genComponent(w io.Writer, component *pcl.Component) {
	componentName := component.DeclarationName()

	optionsBag := g.genResourceOptions(component.Options)

	name := component.LogicalName()
	variableName := makeValidIdentifier(component.Name())

	g.genTrivia(w, component.Definition.Tokens.GetType(""))
	for _, l := range component.Definition.Tokens.GetLabels(nil) {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, component.Definition.Tokens.GetOpenBrace())
	configVars := component.Program.ConfigVariables()
	// collect here all the deferred output variables
	// these must be declared before the component instantiation
	componentInputs := slice.Prealloc[*model.Attribute](len(component.Inputs))
	var componentDeferredOutputVariables []*pcl.DeferredOutputVariable
	for _, attr := range component.Inputs {
		expr, deferredOutputs := pcl.ExtractDeferredOutputVariables(g.program, component, attr.Value)
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

	declareDeferredOutputVariables := func() {
		for _, output := range componentDeferredOutputVariables {
			outputType := output.Expr.Type()
			typeParameter := computeConfigTypeParam(outputType)
			g.Fgenf(w, "%s", g.Indent)
			g.Fgenf(w, "const [%s, resolve%s] = pulumi.deferredOutput<%s>();\n",
				output.Name,
				title(output.Name),
				typeParameter)
		}
	}

	instantiate := func(resName string) {
		if len(configVars) == 0 {
			g.Fgenf(w, "new %s(%s%s)", componentName, resName, optionsBag)
			return
		}
		g.Fgenf(w, "new %s(%s, {", componentName, resName)
		indenter := func(f func()) { f() }
		if len(componentInputs) > 1 {
			indenter = g.Indented
		}
		indenter(func() {
			fmtString := "%s: %.v"
			if len(componentInputs) > 1 {
				fmtString = "\n" + g.Indent + "%s: %.v,"
			}

			for _, attr := range componentInputs {
				propertyName := attr.Name
				if !isLegalIdentifier(propertyName) {
					propertyName = fmt.Sprintf("%q", propertyName)
				}

				loweredExpression := g.lowerExpression(attr.Value, attr.Value.Type())
				g.Fgenf(w, fmtString, propertyName, loweredExpression)
			}
		})
		if len(component.Inputs) > 1 {
			g.Fgenf(w, "\n%s", g.Indent)
		}
		g.Fgenf(w, "}%s)", optionsBag)
	}

	if component.Options != nil && component.Options.Range != nil {
		rangeType := model.ResolveOutputs(component.Options.Range.Type())
		rangeExpr := g.lowerExpression(component.Options.Range, rangeType)

		if model.InputType(model.BoolType).ConversionFrom(rangeType) == model.SafeConversion {
			g.Fgenf(w, "%slet %s: %s | undefined;\n", g.Indent, variableName, componentName)
			g.Fgenf(w, "%sif (%.v) {\n", g.Indent, rangeExpr)
			g.Indented(func() {
				declareDeferredOutputVariables()
				g.Fgenf(w, "%s%s = ", g.Indent, variableName)
				instantiate(g.makeResourceName(name, ""))
				g.Fgenf(w, ";\n")
			})
			g.Fgenf(w, "%s}\n", g.Indent)
		} else {
			g.Fgenf(w, "%sconst %s: %s[] = [];\n", g.Indent, variableName, componentName)

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
				declareDeferredOutputVariables()
				g.Fgenf(w, "%s%s.push(", g.Indent, variableName)
				instantiate(resName)
				g.Fgenf(w, ");\n")
			})
			g.Fgenf(w, "%s}\n", g.Indent)
		}
	} else {
		declareDeferredOutputVariables()
		g.Fgenf(w, "%sconst %s = ", g.Indent, variableName)
		instantiate(g.makeResourceName(name, ""))
		g.Fgenf(w, ";\n")
	}

	// resolve the deferred output variables from this component
	for _, output := range g.deferredOutputVariables {
		if output.SourceComponent.Name() == component.Name() {
			g.Fgenf(w, "%s", g.Indent)
			expr := g.lowerExpression(output.Expr, output.Expr.Type())
			if _, ok := output.Expr.(*model.ScopeTraversalExpression); ok {
				g.Fgenf(w, "resolve%s(%v);\n", title(output.Name), expr)
			} else {
				g.Fgenf(w, "resolve%s(pulumi.output(%v));\n", title(output.Name), expr)
			}
		}
	}

	g.genTrivia(w, component.Definition.Tokens.GetCloseBrace())
}

func computeConfigTypeParam(configType model.Type) string {
	switch pcl.UnwrapOption(configType) {
	case model.StringType:
		return "string"
	case model.NumberType, model.IntType:
		return "number"
	case model.BoolType:
		return "boolean"
	case model.DynamicType:
		return "any"
	default:
		switch complexType := pcl.UnwrapOption(configType).(type) {
		case *model.ListType:
			return fmt.Sprintf("Array<%s>", computeConfigTypeParam(complexType.ElementType))
		case *model.MapType:
			return fmt.Sprintf("Record<string, %s>", computeConfigTypeParam(complexType.ElementType))
		case *model.OutputType:
			return computeConfigTypeParam(complexType.ElementType)
		case *model.ObjectType:
			if len(complexType.Properties) == 0 {
				return "any"
			}

			attributeKeys := []string{}
			for attributeKey := range complexType.Properties {
				attributeKeys = append(attributeKeys, attributeKey)
			}
			// get deterministically sorted attribute keys
			sort.Strings(attributeKeys)

			var elementTypes []string
			for _, propertyName := range attributeKeys {
				propertyType := complexType.Properties[propertyName]
				elementType := fmt.Sprintf("%s?: %s", propertyName, computeConfigTypeParam(propertyType))
				elementTypes = append(elementTypes, elementType)
			}

			return fmt.Sprintf("{%s}", strings.Join(elementTypes, ", "))
		default:
			return "any"
		}
	}
}

func (g *generator) genConfigVariable(w io.Writer, v *pcl.ConfigVariable) {
	if !g.configCreated {
		g.Fprintf(w, "%sconst config = new pulumi.Config();\n", g.Indent)
		g.configCreated = true
	}

	getType := "Object"
	switch pcl.UnwrapOption(v.Type()) {
	case model.StringType:
		getType = ""
	case model.NumberType, model.IntType:
		getType = "Number"
	case model.BoolType:
		getType = "Boolean"
	}

	typeParam := ""
	if getType == "Object" {
		// compute the type parameter T for the call to config.getObject<T>(...)
		computedTypeParam := computeConfigTypeParam(v.Type())
		typeParam = fmt.Sprintf("<%s>", computedTypeParam)
	}

	getOrRequire := "get"
	if v.DefaultValue == nil && !model.IsOptionalType(v.Type()) {
		getOrRequire = "require"
	}

	if v.Description != "" {
		for _, line := range strings.Split(v.Description, "\n") {
			g.Fgenf(w, "%s// %s\n", g.Indent, line)
		}
	}

	name := makeValidIdentifier(v.Name())
	g.Fgenf(w, "%[1]sconst %[2]s = config.%[3]s%[4]s%[5]s(\"%[6]s\")",
		g.Indent, name, getOrRequire, getType, typeParam, v.LogicalName())
	if v.DefaultValue != nil && !model.IsOptionalType(v.Type()) {
		g.Fgenf(w, " || %.v", g.lowerExpression(v.DefaultValue, v.DefaultValue.Type()))
	}
	g.Fgenf(w, ";\n")
}

func (g *generator) genLocalVariable(w io.Writer, v *pcl.LocalVariable) {
	g.genTrivia(w, v.Definition.Tokens.Name)
	vName := makeValidIdentifier(v.Name())
	vValue := g.lowerExpression(v.Definition.Value, v.Type())
	g.Fgenf(w, "%sconst %s = %.3v;\n", g.Indent, vName, vValue)
}

func (g *generator) genOutputVariable(w io.Writer, v *pcl.OutputVariable) {
	if g.asyncMain {
		// skip generating the output variables as export constants
		// when we are inside an async main program because we export them as a single object
		return
	}

	// TODO(pdg): trivia
	g.Fgenf(w, "%sexport const %s = %.3v;\n", g.Indent,
		makeValidIdentifier(v.Name()), g.lowerExpression(v.Value, v.Type()))
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	message := "not yet implemented: " + fmt.Sprintf(reason, vs...)
	g.diagnostics = append(g.diagnostics, &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  message,
		Detail:   message,
	})
	g.Fgenf(w, "(() => throw new Error(%q))()", fmt.Sprintf(reason, vs...))
}
