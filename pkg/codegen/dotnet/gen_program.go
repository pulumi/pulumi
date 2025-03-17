// Copyright 2016-2021, Pulumi Corporation.
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

package dotnet

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
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

type GenerateProgramOptions struct {
	// Determines whether ResourceArg types have an implicit name
	// when constructing a resource. For example:
	// when implicitResourceArgsTypeName is set to true,
	// new Bucket("name", new BucketArgs { ... })
	// becomes
	// new Bucket("name", new() { ... });
	// The latter syntax is only available on .NET 6 or later
	implicitResourceArgsTypeName bool
}

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter
	program *pcl.Program
	// C# namespace map per package.
	namespaces map[string]map[string]string
	// C# codegen compatibility mode per package.
	compatibilities map[string]string
	// A function to convert tokens to module names per package (utilizes the `moduleFormat` setting internally).
	tokenToModules map[string]func(x string) string
	// Type names per invoke function token.
	functionArgs map[string]string
	// keep track of variable identifiers which are the result of an invoke
	// for example "var resourceGroup = GetResourceGroup.Invoke(...)"
	// we will keep track of the reference "resourceGroup"
	//
	// later on when apply a traversal such as resourceGroup.name,
	// we should rewrite it as resourceGroup.Apply(resourceGroupResult => resourceGroupResult.name)
	functionInvokes map[string]*schema.Function
	// Whether awaits are needed, and therefore an async Initialize method should be declared.
	asyncInit            bool
	configCreated        bool
	diagnostics          hcl.Diagnostics
	insideFunctionInvoke bool
	insideAwait          bool
	// Program generation options
	generateOptions GenerateProgramOptions
	isComponent     bool
	// when creating a list of items, we need to know the type of the list
	// if is it a plain list, then `new()` should be used because we are creating List<T>
	// however if we have InputList<T> or anything else, we use `new[]` because InputList<T> can be implicitly casted
	// from an array
	listInitializer         string
	deferredOutputVariables []*pcl.DeferredOutputVariable
}

func (g *generator) resetListInitializer() {
	g.listInitializer = "new[]"
}

func (g *generator) usingDefaultListInitializer() bool {
	return g.listInitializer == "new[]"
}

const (
	pulumiPackage = "pulumi"
	dynamicType   = "dynamic"
)

func GenerateProgramWithOptions(
	program *pcl.Program,
	options GenerateProgramOptions,
) (map[string][]byte, hcl.Diagnostics, error) {
	pcl.MapProvidersAsResources(program)
	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := pcl.Linearize(program)

	// Import C#-specific schema info.
	namespaces := make(map[string]map[string]string)
	compatibilities := make(map[string]string)
	tokenToModules := make(map[string]func(x string) string)
	functionArgs := make(map[string]string)
	packages, err := program.PackageSnapshots()
	if err != nil {
		return nil, nil, err
	}
	for _, p := range packages {
		if err := p.ImportLanguages(map[string]schema.Language{"csharp": Importer}); err != nil {
			return make(map[string][]byte), nil, err
		}

		csharpInfo, hasInfo := p.Language["csharp"].(CSharpPackageInfo)
		if !hasInfo {
			csharpInfo = CSharpPackageInfo{}
		}
		packageNamespaces := csharpInfo.Namespaces
		namespaces[p.Name] = packageNamespaces
		compatibilities[p.Name] = csharpInfo.Compatibility
		tokenToModules[p.Name] = p.TokenToModule

		for _, f := range p.Functions {
			if f.Inputs != nil {
				functionArgs[f.Inputs.Token] = f.Token
			}
		}
	}

	g := &generator{
		program:         program,
		namespaces:      namespaces,
		compatibilities: compatibilities,
		tokenToModules:  tokenToModules,
		functionArgs:    functionArgs,
		functionInvokes: map[string]*schema.Function{},
		generateOptions: options,
		listInitializer: "new[]",
	}

	g.Formatter = format.NewFormatter(g)

	for _, n := range nodes {
		if r, ok := n.(*pcl.Resource); ok && requiresAsyncInit(r) {
			g.asyncInit = true
			break
		}
	}

	var index bytes.Buffer
	g.genPreamble(&index, program)

	g.Indented(func() {
		for _, n := range nodes {
			g.genNode(&index, n)
		}
	})
	g.genPostamble(&index, nodes)

	files := map[string][]byte{
		"Program.cs": index.Bytes(),
	}

	for _, component := range program.CollectComponents() {
		componentName := component.DeclarationName()
		componentNodes := pcl.Linearize(component.Program)

		componentGenerator := &generator{
			program:         component.Program,
			namespaces:      namespaces,
			compatibilities: compatibilities,
			tokenToModules:  tokenToModules,
			functionArgs:    functionArgs,
			functionInvokes: map[string]*schema.Function{},
			generateOptions: options,
			isComponent:     true,
			listInitializer: "new[]",
		}

		componentGenerator.Formatter = format.NewFormatter(componentGenerator)

		var componentBuffer bytes.Buffer
		componentGenerator.genComponentPreamble(&componentBuffer, componentName, component)

		// inside the namespace
		componentGenerator.Indented(func() {
			// inside the class
			componentGenerator.Indented(func() {
				// inside the constructor
				componentGenerator.Indented(func() {
					for _, node := range componentNodes {
						switch node := node.(type) {
						case *pcl.LocalVariable:
							componentGenerator.genLocalVariable(&componentBuffer, node)
						case *pcl.Component:
							// set options { parent = this } for the component resource
							// where "this" is a reference to the component resource itself
							if node.Options == nil {
								node.Options = &pcl.ResourceOptions{}
							}

							if node.Options.Parent == nil {
								node.Options.Parent = model.ConstantReference(&model.Constant{
									Name: "this",
								})
							}
							componentGenerator.genComponent(&componentBuffer, node)
						case *pcl.Resource:
							// set options { parent = this } for the resource
							// where "this" is a reference to the component resource itself
							if node.Options == nil {
								node.Options = &pcl.ResourceOptions{}
							}

							if node.Options.Parent == nil {
								node.Options.Parent = model.ConstantReference(&model.Constant{
									Name: "this",
								})
							}
							componentGenerator.genResource(&componentBuffer, node)
						}
					}
				})
			})
		})

		componentGenerator.genComponentPostamble(&componentBuffer, component)
		files[componentName+".cs"] = componentBuffer.Bytes()
	}

	return files, g.diagnostics, nil
}

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	defaultOptions := GenerateProgramOptions{
		// by default, we generate C# code that targets .NET 6
		implicitResourceArgsTypeName: true,
	}

	return GenerateProgramWithOptions(program, defaultOptions)
}

func GenerateProject(
	directory string, project workspace.Project,
	program *pcl.Program, localDependencies map[string]string,
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

	// Set the runtime to "dotnet" then marshal to Pulumi.yaml
	project.Runtime = workspace.NewProjectRuntimeInfo("dotnet", nil)
	projectBytes, err := encoding.YAML.Marshal(project)
	if err != nil {
		return err
	}
	err = os.WriteFile(path.Join(rootDirectory, "Pulumi.yaml"), projectBytes, 0o600)
	if err != nil {
		return fmt.Errorf("write Pulumi.yaml: %w", err)
	}

	// Build a .csproj based on the packages used by program.
	// Using the current LTS of .NET which is 8.0 as of now.
	var csproj bytes.Buffer
	csproj.WriteString(`<Project Sdk="Microsoft.NET.Sdk">

	<PropertyGroup>
		<OutputType>Exe</OutputType>
		<TargetFramework>net6.0</TargetFramework>
		<Nullable>enable</Nullable>
	</PropertyGroup>
`)

	// Find all the local dependency folders
	folders := mapset.NewSet[string]()
	for _, dep := range localDependencies {
		folders.Add(path.Dir(dep))
	}

	restoreSources := folders.ToSlice()
	sort.Strings(restoreSources)

	if len(restoreSources) > 0 {
		csproj.WriteString(`	<PropertyGroup>
		<RestoreSources>`)
		csproj.WriteString(strings.Join(restoreSources, ";"))
		csproj.WriteString(`;$(RestoreSources)</RestoreSources>
	</PropertyGroup>
`)
	}

	csproj.WriteString("	<ItemGroup>\n")

	// Add local package references
	pkgs := codegen.SortedKeys(localDependencies)
	for _, pkg := range pkgs {
		nugetFilePath := localDependencies[pkg]
		if packageName, version, ok := extractNugetPackageNameAndVersion(nugetFilePath); ok {
			csproj.WriteString(fmt.Sprintf(
				"		<PackageReference Include=\"%s\" Version=\"%s\" />\n",
				packageName, version))
		} else {
			return fmt.Errorf("could not extract package name and version from %s", nugetFilePath)
		}
	}

	if _, hasLocalPulumiReference := localDependencies[pulumiPackage]; !hasLocalPulumiReference {
		csproj.WriteString("		<PackageReference Include=\"Pulumi\" Version=\"3.*\" />\n")
	}

	// For each package add a PackageReference line
	packages, err := program.CollectNestedPackageSnapshots()
	if err != nil {
		return err
	}
	for _, p := range packages {
		if _, isLocal := localDependencies[p.Name]; isLocal {
			continue
		}

		packageTemplate := "		<PackageReference Include=\"%s\" Version=\"%s\" />\n"

		if err := p.ImportLanguages(map[string]schema.Language{"csharp": Importer}); err != nil {
			return err
		}
		if p.Name == pulumiPackage {
			continue
		}

		rootNamespace := "Pulumi"
		if p.Namespace != "" {
			rootNamespace = namespaceName(nil, p.Namespace)
		}

		packageName := rootNamespace + "." + namespaceName(map[string]string{}, p.Name)
		if langInfo, found := p.Language["csharp"]; found {
			csharpInfo, ok := langInfo.(CSharpPackageInfo)
			if ok {
				namespace := namespaceName(csharpInfo.Namespaces, p.Name)
				packageName = fmt.Sprintf("%s.%s", csharpInfo.GetRootNamespace(), namespace)
			}
		}
		if p.Version != nil {
			fmt.Fprintf(&csproj, packageTemplate, packageName, p.Version.String())
		} else {
			fmt.Fprintf(&csproj, packageTemplate, packageName, "*")
		}
	}

	csproj.WriteString(`	</ItemGroup>

</Project>`)

	files[project.Name.String()+".csproj"] = csproj.Bytes()

	// Add the language specific .gitignore
	files[".gitignore"] = []byte(dotnetGitIgnore)

	for filename, data := range files {
		outPath := path.Join(directory, filename)
		err := os.WriteFile(outPath, data, 0o600)
		if err != nil {
			return fmt.Errorf("could not write output program: %w", err)
		}
	}

	return nil
}

// genTrivia generates the list of trivia associated with a given token.
func (g *generator) genTrivia(w io.Writer, token syntax.Token) {
	for _, t := range token.LeadingTrivia {
		if c, ok := t.(syntax.Comment); ok {
			g.genComment(w, c)
		}
	}
	for _, t := range token.TrailingTrivia {
		if c, ok := t.(syntax.Comment); ok {
			g.genComment(w, c)
		}
	}
}

func (g *generator) findFunctionSchema(token string, location *hcl.Range) (*schema.Function, bool) {
	for _, pkg := range g.program.PackageReferences() {
		fn, ok, err := pcl.LookupFunction(pkg, token)
		if !ok {
			continue
		}
		if err != nil {
			g.diagnostics = append(g.diagnostics, &hcl.Diagnostic{
				Severity: hcl.DiagWarning,
				Summary:  fmt.Sprintf("Could not find function schema for '%s'", token),
				Detail:   err.Error(),
				Subject:  location,
			})
			return nil, false
		}
		return fn, true
	}
	return nil, false
}

func (g *generator) isFunctionInvoke(localVariable *pcl.LocalVariable) (*schema.Function, bool) {
	value := localVariable.Definition.Value
	switch value.(type) {
	case *model.FunctionCallExpression:
		call := value.(*model.FunctionCallExpression)
		switch call.Name {
		case pcl.Invoke:
			token := call.Args[0].(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
			return g.findFunctionSchema(token, call.Args[0].SyntaxNode().Range().Ptr())
		}
	}

	return nil, false
}

// genComment generates a comment into the output.
func (g *generator) genComment(w io.Writer, comment syntax.Comment) {
	for _, l := range comment.Lines {
		g.Fgenf(w, "%s//%s\n", g.Indent, l)
	}
}

type programUsings struct {
	systemUsings        codegen.StringSet
	pulumiUsings        codegen.StringSet
	pulumiHelperMethods codegen.StringSet
}

func (g *generator) usingStatements(program *pcl.Program) programUsings {
	systemUsings := codegen.NewStringSet("System.Linq", "System.Collections.Generic")
	pulumiUsings := codegen.NewStringSet()
	preambleHelperMethods := codegen.NewStringSet()
	for _, n := range program.Nodes {
		if r, isResource := n.(*pcl.Resource); isResource {
			pcl.FixupPulumiPackageTokens(r)
			pkg, _, _, _ := r.DecomposeToken()
			if pkg != pulumiPackage {
				namespace := namespaceName(g.namespaces[pkg], pkg)
				var info CSharpPackageInfo
				if r.Schema != nil && r.Schema.PackageReference != nil {
					def, err := r.Schema.PackageReference.Definition()
					contract.AssertNoErrorf(err, "error loading definition for package %q", r.Schema.PackageReference.Name())
					if csharpinfo, ok := def.Language["csharp"].(CSharpPackageInfo); ok {
						info = csharpinfo
					}
				}
				pulumiUsings.Add(fmt.Sprintf("%s = %[2]s.%[1]s", namespace, info.GetRootNamespace()))
			}
		}
		diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
			if call, ok := n.(*model.FunctionCallExpression); ok {
				for _, i := range g.genFunctionUsings(call) {
					if strings.HasPrefix(i, "System") {
						systemUsings.Add(i)
					} else {
						pulumiUsings.Add(i)
					}
				}

				// Checking to see if this function call deserves its own dedicated helper method in the preamble
				if helperMethodBody, ok := getHelperMethodIfNeeded(call.Name, g.Indent); ok {
					preambleHelperMethods.Add(helperMethodBody)
				}
			}
			if _, ok := n.(*model.SplatExpression); ok {
				systemUsings.Add("System.Linq")
			}
			return n, nil
		})
		contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)
	}

	return programUsings{
		systemUsings:        systemUsings,
		pulumiUsings:        pulumiUsings,
		pulumiHelperMethods: preambleHelperMethods,
	}
}

func configObjectTypeName(variableName string) string {
	return Title(variableName) + "Args"
}

func componentInputElementType(pclType model.Type) string {
	switch pclType {
	case model.BoolType:
		return "bool"
	case model.IntType:
		return "int"
	case model.NumberType:
		return "double"
	case model.StringType:
		return "string"
	default:
		switch pclType := pclType.(type) {
		case *model.ListType, *model.MapType:
			return componentInputType(pclType)
		// reduce option(T) to just T
		// the generated args class assumes all properties are optional by default
		case *model.UnionType:
			if len(pclType.ElementTypes) == 2 && pclType.ElementTypes[0] == model.NoneType {
				return componentInputElementType(pclType.ElementTypes[1])
			} else if len(pclType.ElementTypes) == 2 && pclType.ElementTypes[1] == model.NoneType {
				return componentInputElementType(pclType.ElementTypes[0])
			}
			return dynamicType
		default:
			return dynamicType
		}
	}
}

func componentInputType(pclType model.Type) string {
	switch pclType := pclType.(type) {
	case *model.ListType:
		elementType := componentInputElementType(pclType.ElementType)
		return fmt.Sprintf("InputList<%s>", elementType)
	case *model.MapType:
		elementType := componentInputElementType(pclType.ElementType)
		return fmt.Sprintf("InputMap<%s>", elementType)
	default:
		elementType := componentInputElementType(pclType)
		return fmt.Sprintf("Input<%s>", elementType)
	}
}

func componentOutputElementType(pclType model.Type) string {
	switch pclType {
	case model.BoolType:
		return "bool"
	case model.IntType:
		return "int"
	case model.NumberType:
		return "double"
	case model.StringType:
		return "string"
	default:
		switch pclType := pclType.(type) {
		case *model.ListType:
			elementType := componentOutputElementType(pclType.ElementType)
			return fmt.Sprintf("List<%s>", elementType)
		case *model.MapType:
			elementType := componentOutputElementType(pclType.ElementType)
			return fmt.Sprintf("Dictionary<string, %s>", elementType)
		case *model.OutputType:
			// something is already an output
			// get only the element type because we are wrapping these in Output<T> anyway
			return componentOutputElementType(pclType.ElementType)
		default:
			return dynamicType
		}
	}
}

func mainConfigElementType(pclType model.Type) string {
	pclType = pcl.UnwrapOption(pclType)
	switch pclType {
	case model.BoolType:
		return "bool"
	case model.IntType:
		return "int"
	case model.NumberType:
		return "double"
	case model.StringType:
		return "string"
	default:
		switch pclType := pclType.(type) {
		case *model.ListType:
			elementType := mainConfigElementType(pclType.ElementType)
			return fmt.Sprintf("List<%s>", elementType)
		case *model.MapType:
			elementType := mainConfigElementType(pclType.ElementType)
			return fmt.Sprintf("Dictionary<string, %s>", elementType)
		default:
			return dynamicType
		}
	}
}

func componentOutputType(pclType model.Type) string {
	elementType := componentOutputElementType(pclType)
	return fmt.Sprintf("Output<%s>", elementType)
}

type ObjectTypeFromConfigMetadata = struct {
	TypeName      string
	ComponentName string
}

func annotateObjectTypedConfig(componentName string, typeName string, objectType *model.ObjectType) *model.ObjectType {
	objectType.Annotate(&ObjectTypeFromConfigMetadata{
		TypeName:      typeName,
		ComponentName: componentName,
	})

	return objectType
}

// collectComponentObjectTypedConfigVariables returns the object types in config variables need to be emitted
// as classes in custom resource components
func collectComponentObjectTypedConfigVariables(component *pcl.Component) map[string]*model.ObjectType {
	objectTypes := map[string]*model.ObjectType{}
	for _, config := range component.Program.ConfigVariables() {
		componentName := component.DeclarationName()
		typeName := configObjectTypeName(config.Name())
		switch configType := config.Type().(type) {
		case *model.ObjectType:
			objectTypes[config.Name()] = annotateObjectTypedConfig(componentName, typeName, configType)
		case *model.ListType:
			switch elementType := configType.ElementType.(type) {
			case *model.ObjectType:
				objectTypes[config.Name()] = annotateObjectTypedConfig(componentName, typeName, elementType)
			}
		case *model.MapType:
			switch elementType := configType.ElementType.(type) {
			case *model.ObjectType:
				objectTypes[config.Name()] = annotateObjectTypedConfig(componentName, typeName, elementType)
			}
		}
	}

	return objectTypes
}

// collectObjectTypedConfigVariables returns the object types in config variables need to be emitted
// as classes in the main program
func collectObjectTypedConfigVariables(program *pcl.Program) map[string]*model.ObjectType {
	objectTypes := map[string]*model.ObjectType{}
	for _, config := range program.ConfigVariables() {
		typeName := Title(makeValidIdentifier(config.Name()))
		switch configType := pcl.UnwrapOption(config.Type()).(type) {
		case *model.ObjectType:
			objectTypes[typeName] = configType
		case *model.ListType:
			switch elementType := configType.ElementType.(type) {
			case *model.ObjectType:
				objectTypes[typeName] = elementType
			}
		case *model.MapType:
			switch elementType := configType.ElementType.(type) {
			case *model.ObjectType:
				objectTypes[typeName] = elementType
			}
		}
	}

	return objectTypes
}

func (g *generator) genComponentPreamble(w io.Writer, componentName string, component *pcl.Component) {
	// Accumulate other using statements for the various providers and packages. Don't emit them yet, as we need
	// to sort them later on.
	programUsings := g.usingStatements(component.Program)
	systemUsings := programUsings.systemUsings
	pulumiUsings := programUsings.pulumiUsings
	for _, pkg := range systemUsings.SortedValues() {
		g.Fprintf(w, "using %v;\n", pkg)
	}
	g.Fprintln(w, `using Pulumi;`)
	for _, pkg := range pulumiUsings.SortedValues() {
		g.Fprintf(w, "using %v;\n", pkg)
	}
	configVars := component.Program.ConfigVariables()

	g.Fprint(w, "\n")
	g.Fprintln(w, "namespace Components")
	g.Fprintf(w, "{\n")
	g.Indented(func() {
		if len(configVars) > 0 {
			g.Fprintf(w, "%spublic class %sArgs : global::Pulumi.ResourceArgs\n", g.Indent, componentName)
			g.Fprintf(w, "%s{\n", g.Indent)
			g.Indented(func() {
				objectTypedConfigVars := collectComponentObjectTypedConfigVariables(component)
				variableNames := pcl.SortedStringKeys(objectTypedConfigVars)
				// generate resource args for this component
				for _, variableName := range variableNames {
					objectType := objectTypedConfigVars[variableName]
					objectTypeName := configObjectTypeName(variableName)
					g.Fprintf(w, "%spublic class %s : global::Pulumi.ResourceArgs\n", g.Indent, objectTypeName)
					g.Fprintf(w, "%s{\n", g.Indent)
					g.Indented(func() {
						propertyNames := pcl.SortedStringKeys(objectType.Properties)
						for _, propertyName := range propertyNames {
							propertyType := objectType.Properties[propertyName]
							inputType := componentInputType(propertyType)
							g.Fprintf(w, "%s[Input(\"%s\")]\n", g.Indent, propertyName)
							g.Fprintf(w, "%spublic %s? %s { get; set; }\n",
								g.Indent,
								inputType,
								Title(propertyName))
						}
					})
					g.Fprintf(w, "%s}\n\n", g.Indent)
				}

				for _, configVar := range configVars {
					// for simple values, get the primitive type
					inputType := componentInputType(configVar.Type())
					switch configType := configVar.Type().(type) {
					case *model.ObjectType:
						// for objects of type T, generate T as is
						inputType = configObjectTypeName(configVar.Name())
					case *model.ListType:
						// for list(T) where T is an object type, generate T[]
						switch configType.ElementType.(type) {
						case *model.ObjectType:
							objectTypeName := configObjectTypeName(configVar.Name())
							inputType = objectTypeName + "[]"
						}
					case *model.MapType:
						// for map(T) where T is an object type, generate Dictionary<string, T>
						switch configType.ElementType.(type) {
						case *model.ObjectType:
							objectTypeName := configObjectTypeName(configVar.Name())
							inputType = fmt.Sprintf("Dictionary<string, %s>", objectTypeName)
						}
					}

					if configVar.Description != "" {
						g.Fgenf(w, "%s/// <summary>\n", g.Indent)
						for _, line := range strings.Split(configVar.Description, "\n") {
							g.Fgenf(w, "%s/// %s\n", g.Indent, line)
						}
						g.Fgenf(w, "%s/// </summary>\n", g.Indent)
					}
					g.Fprintf(w, "%s[Input(\"%s\")]\n", g.Indent, configVar.LogicalName())
					g.Fprintf(w, "%spublic %s %s { get; set; } = ",
						g.Indent,
						inputType,
						Title(configVar.Name()))

					if configVar.DefaultValue != nil {
						g.Fprintf(w, "%v;\n", g.lowerExpression(configVar.DefaultValue, configVar.DefaultValue.Type()))
					} else {
						g.Fprint(w, "null!;\n")
					}
				}
			})
			g.Fprintf(w, "%s}\n\n", g.Indent)
		}

		g.Fprintf(w, "%spublic class %s : global::Pulumi.ComponentResource\n", g.Indent, componentName)
		g.Fprintf(w, "%s{\n", g.Indent)
		g.Indented(func() {
			for _, outputVar := range component.Program.OutputVariables() {
				var outputType string
				switch expr := outputVar.Value.(type) {
				case *model.ScopeTraversalExpression:
					resource, ok := expr.Parts[0].(*pcl.Resource)
					if ok && len(expr.Parts) == 1 {
						// special case: the output is a Resource type
						outputType = fmt.Sprintf("Output<%s>", g.resourceTypeName(resource))
					} else {
						outputType = componentOutputType(expr.Type())
					}
				default:
					outputType = componentOutputType(expr.Type())
				}

				g.Fprintf(w, "%s[Output(\"%s\")]\n", g.Indent, outputVar.LogicalName())
				g.Fprintf(w, "%spublic %s %s { get; private set; }\n",
					g.Indent,
					outputType,
					Title(outputVar.Name()))
			}

			// If we collected any helper methods that should be added, write them
			for _, preambleHelperMethodBody := range programUsings.pulumiHelperMethods.SortedValues() {
				g.Fprintf(w, "        %s\n\n", preambleHelperMethodBody)
			}

			token := "components:index:" + componentName
			if len(configVars) == 0 {
				// There is no args class
				g.Fgenf(w, "%spublic %s(string name, ComponentResourceOptions? opts = null)\n",
					g.Indent,
					componentName)

				g.Fgenf(w, "%s    : base(\"%s\", name, ResourceArgs.Empty, opts)\n", g.Indent, token)
			} else {
				// There is no args class
				g.Fgenf(w, "%spublic %s(string name, %sArgs args, ComponentResourceOptions? opts = null)\n",
					g.Indent,
					componentName,
					componentName)

				g.Fgenf(w, "%s    : base(\"%s\", name, args, opts)\n", g.Indent, token)
			}

			g.Fgenf(w, "%s{\n", g.Indent)
		})
	})
}

func (g *generator) genComponentPostamble(w io.Writer, component *pcl.Component) {
	outputVars := component.Program.OutputVariables()
	g.Indented(func() {
		g.Indented(func() {
			g.Indented(func() {
				if len(outputVars) == 0 {
					g.Fgenf(w, "%sthis.RegisterOutputs();\n", g.Indent)
				} else {
					// Emit component resource output assignment
					for _, output := range outputVars {
						outputProperty := Title(output.Name())
						switch expr := output.Value.(type) {
						case *model.ScopeTraversalExpression:
							_, ok := expr.Parts[0].(*pcl.Resource)
							if ok && len(expr.Parts) == 1 {
								// special case: the output is a Resource type
								g.Fgenf(w, "%sthis.%s = Output.Create(%.3v);\n",
									g.Indent, outputProperty,
									g.lowerExpression(output.Value, output.Type()))
							} else {
								g.Fgenf(w, "%sthis.%s = %.3v;\n",
									g.Indent, outputProperty,
									g.lowerExpression(output.Value, output.Type()))
							}
						default:

							g.Fgenf(w, "%sthis.%s = %.3v;\n",
								g.Indent, outputProperty,
								g.lowerExpression(output.Value, output.Type()))
						}
					}

					g.Fgen(w, "\n")

					g.Fgenf(w, "%sthis.RegisterOutputs(new Dictionary<string, object?>\n", g.Indent)
					g.Fgenf(w, "%s{\n", g.Indent)
					g.Indented(func() {
						// Emit component resource output properties
						for _, n := range outputVars {
							outputID := fmt.Sprintf(`"%s"`, g.escapeString(n.LogicalName(), false, false))
							g.Fgenf(w, "%s[%s] = %.3v,\n", g.Indent, outputID, g.lowerExpression(n.Value, n.Type()))
						}
					})
					g.Fgenf(w, "%s});\n", g.Indent)
				}
			})
		})
	})

	// closing bracket for the component resource class constructor
	indent := "    "
	g.Fprintf(w, "%s%s}\n", indent, indent)
	// closing bracket for the component resource class
	g.Fprintf(w, "%s}\n", indent)
	// closing bracket for the components namespace
	g.Fprint(w, "}\n")
}

// genPreamble generates using statements, class definition and constructor.
func (g *generator) genPreamble(w io.Writer, program *pcl.Program) {
	// Accumulate other using statements for the various providers and packages. Don't emit them yet, as we need
	// to sort them later on.
	programUsings := g.usingStatements(program)
	systemUsings := programUsings.systemUsings
	pulumiUsings := programUsings.pulumiUsings
	preambleHelperMethods := programUsings.pulumiHelperMethods

	if g.asyncInit {
		systemUsings.Add("System.Threading.Tasks")
	}

	for _, pkg := range systemUsings.SortedValues() {
		g.Fprintf(w, "using %v;\n", pkg)
	}
	g.Fprintln(w, `using Pulumi;`)
	for _, pkg := range pulumiUsings.SortedValues() {
		g.Fprintf(w, "using %v;\n", pkg)
	}

	g.Fprint(w, "\n")

	// If we collected any helper methods that should be added, write them just before the main func
	for _, preambleHelperMethodBody := range preambleHelperMethods.SortedValues() {
		g.Fprintf(w, "\t%s\n\n", preambleHelperMethodBody)
	}

	asyncKeywordWhenNeeded := ""
	if g.asyncInit {
		asyncKeywordWhenNeeded = "async"
	}
	g.Fprintf(w, "return await Deployment.RunAsync(%s() => \n", asyncKeywordWhenNeeded)
	g.Fprint(w, "{\n")
}

// hasOutputVariables checks whether there are any output declarations
func hasOutputVariables(nodes []pcl.Node) bool {
	for _, n := range nodes {
		switch n.(type) {
		case *pcl.OutputVariable:
			return true
		}
	}

	return false
}

// genPostamble closes the method and the class and declares stack output statements.
func (g *generator) genPostamble(w io.Writer, nodes []pcl.Node) {
	if hasOutputVariables(nodes) {
		g.Indented(func() {
			g.Fgenf(w, "%sreturn new Dictionary<string, object?>\n", g.Indent)
			g.Fgenf(w, "%s{\n", g.Indent)
			g.Indented(func() {
				// Emit stack output properties
				for _, n := range nodes {
					switch n := n.(type) {
					case *pcl.OutputVariable:
						outputID := fmt.Sprintf(`"%s"`, g.escapeString(n.LogicalName(), false, false))
						g.Fgenf(w, "%s[%s] = %.3v,\n", g.Indent, outputID, g.lowerExpression(n.Value, n.Type()))
					}
				}
			})
			g.Fgenf(w, "%s};\n", g.Indent)
		})
	}
	// Close lambda call expression
	g.Fprintf(w, "});\n\n")

	// Generate types for object typed config variables
	// those are referenced in config.GetObject<T> where T is one of these generated types
	// they must be generated after the top-level statement call to Deployment.RunAsync
	objectTypedConfigVariables := collectObjectTypedConfigVariables(g.program)
	objectTypeKeys := pcl.SortedStringKeys(objectTypedConfigVariables)
	for _, typeName := range objectTypeKeys {
		objectType := objectTypedConfigVariables[typeName]
		g.Fgenf(w, "public class %s\n{\n", typeName)
		sortedProperties := pcl.SortedStringKeys(objectType.Properties)
		for _, propertyName := range sortedProperties {
			g.Indented(func() {
				property := objectType.Properties[propertyName]
				propertyType := mainConfigElementType(property)
				g.Fgenf(w, "%spublic %s %s { get; set; }\n", g.Indent, propertyType, propertyName)
			})
		}
		g.Fgenf(w, "}\n\n")
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
	case *pcl.Component:
		g.genComponent(w, n)
	}
}

// requiresAsyncInit returns true if the program requires awaits in the code, and therefore an asynchronous
// method must be declared.
func requiresAsyncInit(r *pcl.Resource) bool {
	if r.Options == nil || r.Options.Range == nil {
		return false
	}

	return model.ContainsPromises(r.Options.Range.Type())
}

// resourceTypeName computes the C# class name for the given resource.
func (g *generator) resourceTypeName(r *pcl.Resource) string {
	pcl.FixupPulumiPackageTokens(r)
	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diags := r.DecomposeToken()
	contract.Assertf(len(diags) == 0, "error decomposing token: %v", diags)

	if r.Schema != nil {
		if val1, ok := r.Schema.Language["csharp"]; ok {
			val2, ok := val1.(CSharpResourceInfo)
			contract.Assertf(ok, "dotnet specific settings for resources should be of type CSharpResourceInfo")
			member = val2.Name
		}
	}

	namespaces := g.namespaces[pkg]
	rootNamespace := namespaceName(namespaces, pkg)

	namespace := namespaceName(namespaces, module)
	namespaceTokens := strings.Split(namespace, "/")
	for i, name := range namespaceTokens {
		namespaceTokens[i] = Title(name)
	}
	namespace = strings.Join(namespaceTokens, ".")

	if namespace != "" {
		namespace = "." + namespace
	}

	qualifiedMemberName := fmt.Sprintf("%s%s.%s", rootNamespace, namespace, Title(member))
	return qualifiedMemberName
}

func (g *generator) extractInputPropertyNameMap(r *pcl.Resource) map[string]string {
	// Extract language-specific property names from schema
	csharpInputPropertyNameMap := make(map[string]string)
	if r.Schema != nil {
		for _, inputProperty := range r.Schema.InputProperties {
			if val1, ok := inputProperty.Language["csharp"]; ok {
				if val2, ok := val1.(CSharpPropertyInfo); ok {
					csharpInputPropertyNameMap[inputProperty.Name] = val2.Name
				}
			}
		}
	}
	return csharpInputPropertyNameMap
}

// resourceArgsTypeName computes the C# arguments class name for the given resource.
func (g *generator) resourceArgsTypeName(r *pcl.Resource) string {
	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diags := r.DecomposeToken()
	contract.Assertf(len(diags) == 0, "error decomposing token: %v", diags)

	namespaces := g.namespaces[pkg]
	rootNamespace := namespaceName(namespaces, pkg)
	namespace := namespaceName(namespaces, module)
	if g.compatibilities[pkg] == "kubernetes20" && module != "" {
		namespace = "Types.Inputs." + namespace
	}

	if namespace != "" {
		namespace = "." + namespace
	}

	return fmt.Sprintf("%s%s.%sArgs", rootNamespace, namespace, Title(member))
}

// functionName computes the C# namespace and class name for the given function token.
func (g *generator) functionName(tokenArg model.Expression) (string, string) {
	token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
	tokenRange := tokenArg.SyntaxNode().Range()

	// Compute the resource type from the Pulumi type token.
	pkg, module, member, diags := pcl.DecomposeToken(token, tokenRange)
	contract.Assertf(len(diags) == 0, "error decomposing token: %v", diags)
	namespaces := g.namespaces[pkg]
	rootNamespace := namespaceName(namespaces, pkg)
	namespace := namespaceName(namespaces, module)

	if namespace != "" {
		namespace = "." + namespace
	}

	return rootNamespace, fmt.Sprintf("%s%s.%s", rootNamespace, namespace, Title(member))
}

func (g *generator) toSchemaType(destType model.Type) (schema.Type, bool) {
	schemaType, ok := pcl.GetSchemaForType(destType)
	if !ok {
		return nil, false
	}
	return codegen.UnwrapType(schemaType), true
}

// argumentTypeName computes the C# argument class name for the given expression and model type.
func (g *generator) argumentTypeName(expr model.Expression, destType model.Type) string {
	suffix := "Args"
	if g.insideFunctionInvoke {
		suffix = "InputArgs"
	}
	return g.argumentTypeNameWithSuffix(expr, destType, suffix)
}

func (g *generator) argumentTypeNameWithSuffix(expr model.Expression, destType model.Type, suffix string) string {
	schemaType, ok := g.toSchemaType(destType)
	if !ok {
		return ""
	}

	objType, ok := schemaType.(*schema.ObjectType)
	if !ok {
		return ""
	}

	token := objType.Token
	tokenRange := expr.SyntaxNode().Range()
	qualifier := "Inputs"
	if f, ok := g.functionArgs[token]; ok {
		token = f
		qualifier = ""
	}

	pkg, modName, member, diags := pcl.DecomposeToken(token, tokenRange)
	contract.Assertf(len(diags) == 0, "error decomposing token: %v", diags)
	var module string
	if getModule, ok := g.tokenToModules[pkg]; ok {
		module = getModule(token)
	} else {
		module = strings.SplitN(modName, "/", 2)[0]
	}
	namespaces := g.namespaces[pkg]
	rootNamespace := namespaceName(namespaces, pkg)
	namespace := namespaceName(namespaces, module)
	if strings.ToLower(namespace) == "index" {
		namespace = ""
	}
	if namespace != "" {
		namespace = "." + namespace
	}
	if g.compatibilities[pkg] == "kubernetes20" {
		namespace = ".Types.Inputs" + namespace
	} else if qualifier != "" {
		namespace = namespace + "." + qualifier
	}
	member = member + suffix

	return fmt.Sprintf("%s%s.%s", rootNamespace, namespace, Title(member))
}

// makeResourceName returns the expression that should be emitted for a resource's "name" parameter given its base name
// and the count variable name, if any.
func (g *generator) makeResourceName(baseName, count string) string {
	if count == "" {
		if g.isComponent {
			return fmt.Sprintf(`$"{name}-%s"`, baseName)
		}

		return fmt.Sprintf(`"%s"`, baseName)
	}

	if g.isComponent {
		return fmt.Sprintf("$\"{name}-%s-{%s}\"", baseName, count)
	}

	return fmt.Sprintf("$\"%s-{%s}\"", baseName, count)
}

func (g *generator) genResourceOptions(opts *pcl.ResourceOptions, resourceOptionsType string) string {
	if opts == nil {
		return ""
	}
	var result bytes.Buffer
	appendOption := func(name string, value model.Expression) {
		if result.Len() == 0 {
			_, err := fmt.Fprintf(&result, ", new %s\n%s{", resourceOptionsType, g.Indent)
			g.Indent += "    "
			contract.IgnoreError(err)
		}

		if name == "IgnoreChanges" {
			// ignore changes need to be special cased
			// because new [] { "field" } cannot be implicitly casted to List<string>
			// which is the type of IgnoreChanges
			if changes, isTuple := value.(*model.TupleConsExpression); isTuple {
				g.Fgenf(&result, "\n%sIgnoreChanges =", g.Indent)
				g.Fgenf(&result, "\n%s{", g.Indent)
				g.Indented(func() {
					for _, v := range changes.Expressions {
						g.Fgenf(&result, "\n%s\"%.v\",", g.Indent, v)
					}
				})
				g.Fgenf(&result, "\n%s},", g.Indent)
			} else {
				g.Fgenf(&result, "\n%s%s = %v,", g.Indent, name, g.lowerExpression(value, value.Type()))
			}
		} else if name == "Parent" {
			// special case parent = this, do not escape "this"
			if parent, isThis := value.(*model.ScopeTraversalExpression); isThis {
				if parent.RootName == "this" && len(parent.Parts) == 1 && g.isComponent {
					g.Fgenf(&result, "\n%s%s = this,", g.Indent, name)
				} else {
					g.Fgenf(&result, "\n%s%s = %v,", g.Indent, name, g.lowerExpression(value, value.Type()))
				}
			} else {
				g.Fgenf(&result, "\n%s%s = %v,", g.Indent, name, g.lowerExpression(value, value.Type()))
			}
		} else if name == "DependsOn" {
			// depends on need to be special cased
			// because new [] { resourceA, resourceB } cannot be implicitly casted to InputList<Resource>
			// use syntax DependsOn = { resourceA, resourceB } instead
			if resourcesList, isTuple := value.(*model.TupleConsExpression); isTuple {
				g.Fgenf(&result, "\n%sDependsOn =", g.Indent)
				g.Fgenf(&result, "\n%s{", g.Indent)
				g.Indented(func() {
					for _, resource := range resourcesList.Expressions {
						g.Fgenf(&result, "\n%s%v,", g.Indent, resource)
					}
				})
				g.Fgenf(&result, "\n%s},", g.Indent)
			} else {
				g.Fgenf(&result, "\n%s%s = %v,", g.Indent, name, g.lowerExpression(value, value.Type()))
			}
		} else {
			g.Fgenf(&result, "\n%s%s = %v,", g.Indent, name, g.lowerExpression(value, value.Type()))
		}
	}

	if opts.Parent != nil {
		appendOption("Parent", opts.Parent)
	}
	if opts.Provider != nil {
		appendOption("Provider", opts.Provider)
	}
	if opts.DependsOn != nil {
		appendOption("DependsOn", opts.DependsOn)
	}
	if opts.Protect != nil {
		appendOption("Protect", opts.Protect)
	}
	if opts.RetainOnDelete != nil {
		appendOption("RetainOnDelete", opts.RetainOnDelete)
	}
	if opts.IgnoreChanges != nil {
		appendOption("IgnoreChanges", opts.IgnoreChanges)
	}
	if opts.DeletedWith != nil {
		appendOption("DeletedWith", opts.DeletedWith)
	}

	if result.Len() != 0 {
		g.Indent = g.Indent[:len(g.Indent)-4]
		_, err := fmt.Fprintf(&result, "\n%s}", g.Indent)
		contract.IgnoreError(err)
	}

	return result.String()
}

func AnnotateComponentInputs(component *pcl.Component) {
	componentName := component.DeclarationName()
	configVars := component.Program.ConfigVariables()

	for index := range component.Inputs {
		attribute := component.Inputs[index]
		switch expr := attribute.Value.(type) {
		case *model.ObjectConsExpression:
			for _, configVar := range configVars {
				if configVar.Name() == attribute.Name {
					switch configVar.Type().(type) {
					case *model.ObjectType:
						expr.WithType(func(objectExprType model.Type) *model.ObjectConsExpression {
							switch exprType := objectExprType.(type) {
							case *model.ObjectType:
								typeName := configObjectTypeName(configVar.Name())
								annotateObjectTypedConfig(componentName, typeName, exprType)
							}

							return expr
						})
					case *model.MapType:
						for _, item := range expr.Items {
							switch mapValue := item.Value.(type) {
							case *model.ObjectConsExpression:
								mapValue.WithType(func(objectExprType model.Type) *model.ObjectConsExpression {
									switch exprType := objectExprType.(type) {
									case *model.ObjectType:
										typeName := configObjectTypeName(configVar.Name())
										annotateObjectTypedConfig(componentName, typeName, exprType)
									}

									return mapValue
								})
							}
						}
					}
				}
			}
		case *model.TupleConsExpression:
			for _, configVar := range configVars {
				if configVar.Name() == attribute.Name {
					switch listType := configVar.Type().(type) {
					case *model.ListType:
						switch listType.ElementType.(type) {
						case *model.ObjectType:
							for _, item := range expr.Expressions {
								switch itemExpr := item.(type) {
								case *model.ObjectConsExpression:
									itemExpr.WithType(func(objectExprType model.Type) *model.ObjectConsExpression {
										switch exprType := objectExprType.(type) {
										case *model.ObjectType:
											typeName := configObjectTypeName(configVar.Name())
											annotateObjectTypedConfig(componentName, typeName, exprType)
										}
										return itemExpr
									})
								}
							}
						}
					}
				}
			}
		}
	}
}

func isPlainResourceProperty(r *pcl.Resource, name string) bool {
	if r.Schema == nil {
		return false
	}

	for _, property := range r.Schema.InputProperties {
		if property.Name == name {
			return property.Plain
		}
	}
	return false
}

// genResource handles the generation of instantiations of non-builtin resources.
func (g *generator) genResource(w io.Writer, r *pcl.Resource) {
	qualifiedMemberName := g.resourceTypeName(r)
	csharpInputPropertyNameMap := g.extractInputPropertyNameMap(r)

	// Add conversions to input properties
	if r.Schema != nil {
		for _, input := range r.Inputs {
			destType, diagnostics := r.InputType.Traverse(hcl.TraverseAttr{Name: input.Name})
			g.diagnostics = append(g.diagnostics, diagnostics...)
			input.Value = g.lowerExpression(input.Value, destType.(model.Type))
			if csharpName, ok := csharpInputPropertyNameMap[input.Name]; ok {
				input.Name = csharpName
			}
		}
	}

	pcl.AnnotateResourceInputs(r)
	name := r.LogicalName()
	variableName := makeValidIdentifier(r.Name())
	argsName := g.resourceArgsTypeName(r)
	g.genTrivia(w, r.Definition.Tokens.GetType(""))
	for _, l := range r.Definition.Tokens.GetLabels(nil) {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, r.Definition.Tokens.GetOpenBrace())

	instantiate := func(resName string) {
		if len(r.Inputs) == 0 && r.Options == nil {
			// only resource name is provided
			g.Fgenf(w, "new %s(%s)", qualifiedMemberName, resName)
		} else {
			if g.generateOptions.implicitResourceArgsTypeName {
				g.Fgenf(w, "new %s(%s, new()\n", qualifiedMemberName, resName)
			} else {
				g.Fgenf(w, "new %s(%s, new %s\n", qualifiedMemberName, resName, argsName)
			}

			g.Fgenf(w, "%s{\n", g.Indent)
			g.Indented(func() {
				for _, attr := range r.Inputs {
					g.Fgenf(w, "%s%s =", g.Indent, propertyName(attr.Name))
					if isPlainResourceProperty(r, attr.Name) {
						g.listInitializer = "new()"
					}

					g.Fgenf(w, " %.v,\n", attr.Value)
					g.resetListInitializer()
				}
			})
			g.Fgenf(w, "%s}%s)", g.Indent, g.genResourceOptions(r.Options, "CustomResourceOptions"))
		}
	}

	if r.Options != nil && r.Options.Range != nil {
		rangeType := model.ResolveOutputs(r.Options.Range.Type())
		rangeExpr := g.lowerExpression(r.Options.Range, rangeType)

		g.Fgenf(w, "%svar %s = new List<%s>();\n", g.Indent, variableName, qualifiedMemberName)

		resKey := "Key"
		if model.InputType(model.NumberType).ConversionFrom(rangeExpr.Type()) != model.NoConversion {
			g.Fgenf(w, "%sfor (var rangeIndex = 0; rangeIndex < %.12o; rangeIndex++)\n", g.Indent, rangeExpr)
			g.Fgenf(w, "%s{\n", g.Indent)
			g.Fgenf(w, "%s    var range = new { Value = rangeIndex };\n", g.Indent)
			resKey = "Value"
		} else {
			rangeExpr := &model.FunctionCallExpression{
				Name: "entries",
				Args: []model.Expression{rangeExpr},
			}
			g.Fgenf(w, "%sforeach (var range in %.v)\n", g.Indent, rangeExpr)
			g.Fgenf(w, "%s{\n", g.Indent)
		}

		resName := g.makeResourceName(name, "range."+resKey)
		g.Indented(func() {
			g.Fgenf(w, "%s%s.Add(", g.Indent, variableName)
			instantiate(resName)
			g.Fgenf(w, ");\n")
		})
		g.Fgenf(w, "%s}\n", g.Indent)
	} else {
		g.Fgenf(w, "%svar %s = ", g.Indent, variableName)
		instantiate(g.makeResourceName(name, ""))
		g.Fgenf(w, ";\n\n")
	}

	g.genTrivia(w, r.Definition.Tokens.GetCloseBrace())
}

// genComponent handles the generation of instantiations of non-builtin resources.
func (g *generator) genComponent(w io.Writer, r *pcl.Component) {
	componentName := r.DeclarationName()
	qualifiedMemberName := "Components." + componentName

	name := r.LogicalName()
	variableName := makeValidIdentifier(r.Name())
	argsName := componentName + "Args"

	AnnotateComponentInputs(r)

	configVars := r.Program.ConfigVariables()

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

	declareDeferredOutputVariables := func() {
		for _, output := range componentDeferredOutputVariables {
			typeParameter := componentOutputElementType(output.Expr.Type())
			g.Fgenf(w, "%s", g.Indent)
			g.Fgenf(w, "var %s = new Pulumi.DeferredOutput<%s>();\n",
				output.Name,
				typeParameter)
		}
	}

	instantiate := func(resName string) {
		if len(configVars) == 0 {
			// there is no args type for this component
			g.Fgenf(w, "new %s(%s%s)",
				qualifiedMemberName,
				resName,
				g.genResourceOptions(r.Options, "ComponentResourceOptions"))

			return
		}

		if len(r.Inputs) == 0 && r.Options == nil {
			// only resource name is provided
			g.Fgenf(w, "new %s(%s)", qualifiedMemberName, resName)
		} else {
			if g.generateOptions.implicitResourceArgsTypeName {
				g.Fgenf(w, "new %s(%s, new()\n", qualifiedMemberName, resName)
			} else {
				g.Fgenf(w, "new %s(%s, new %s\n", qualifiedMemberName, resName, argsName)
			}

			g.Fgenf(w, "%s{\n", g.Indent)
			g.Indented(func() {
				for _, attr := range componentInputs {
					g.Fgenf(w, "%s%s =", g.Indent, propertyName(attr.Name))
					value := g.lowerExpression(attr.Value, attr.Value.Type())
					g.Fgenf(w, " %.v,\n", value)
				}
			})
			g.Fgenf(w, "%s}%s)", g.Indent, g.genResourceOptions(r.Options, "ComponentResourceOptions"))
		}
	}

	if r.Options != nil && r.Options.Range != nil {
		rangeType := model.ResolveOutputs(r.Options.Range.Type())
		rangeExpr := g.lowerExpression(r.Options.Range, rangeType)

		g.Fgenf(w, "%svar %s = new List<%s>();\n", g.Indent, variableName, qualifiedMemberName)

		resKey := "Key"
		if model.InputType(model.NumberType).ConversionFrom(rangeExpr.Type()) != model.NoConversion {
			g.Fgenf(w, "%sfor (var rangeIndex = 0; rangeIndex < %.12o; rangeIndex++)\n", g.Indent, rangeExpr)
			g.Fgenf(w, "%s{\n", g.Indent)
			g.Fgenf(w, "%s    var range = new { Value = rangeIndex };\n", g.Indent)
			resKey = "Value"
		} else {
			rangeExpr := &model.FunctionCallExpression{
				Name: "entries",
				Args: []model.Expression{rangeExpr},
			}
			g.Fgenf(w, "%sforeach (var range in %.v)\n", g.Indent, rangeExpr)
			g.Fgenf(w, "%s{\n", g.Indent)
		}

		resName := g.makeResourceName(name, "range."+resKey)
		g.Indented(func() {
			declareDeferredOutputVariables()
			g.Fgenf(w, "%s%s.Add(", g.Indent, variableName)
			instantiate(resName)
			g.Fgenf(w, ");\n")
		})
		g.Fgenf(w, "%s}\n", g.Indent)
	} else {
		declareDeferredOutputVariables()
		g.Fgenf(w, "%svar %s = ", g.Indent, variableName)
		instantiate(g.makeResourceName(name, ""))
		g.Fgenf(w, ";\n\n")
	}

	// Emit the deferred output resolution statements
	for _, output := range g.deferredOutputVariables {
		if output.SourceComponent.Name() == r.Name() {
			g.Fgenf(w, "%s", g.Indent)
			expr := g.lowerExpression(output.Expr, output.Expr.Type())
			if _, ok := output.Expr.(*model.ScopeTraversalExpression); ok {
				g.Fgenf(w, "%s.Resolve(%v);\n", output.Name, expr)
			} else {
				g.Fgenf(w, "%s.Resolve(Output.Create(%v));\n", output.Name, expr)
			}
		}
	}

	g.genTrivia(w, r.Definition.Tokens.GetCloseBrace())
}

func computeConfigTypeParam(configName string, configType model.Type) string {
	typeName := Title(makeValidIdentifier(configName))
	configType = pcl.UnwrapOption(configType)
	switch configType {
	case model.StringType:
		return "string"
	case model.IntType:
		return "int"
	case model.NumberType:
		return "double"
	case model.BoolType:
		return "bool"
	case model.DynamicType:
		return "dynamic"
	default:
		switch complexType := configType.(type) {
		case *model.ObjectType:
			return typeName
		case *model.ListType:
			elementType := computeConfigTypeParam(configName, complexType.ElementType)
			return elementType + "[]"
		case *model.MapType:
			elementType := computeConfigTypeParam(configName, complexType.ElementType)
			return fmt.Sprintf("Dictionary<string, %s>", elementType)
		default:
			return "dynamic"
		}
	}
}

func (g *generator) genConfigVariable(w io.Writer, v *pcl.ConfigVariable) {
	if !g.configCreated {
		g.Fprintf(w, "%svar config = new Config();\n", g.Indent)
		g.configCreated = true
	}

	getType := "Object"
	switch pcl.UnwrapOption(v.Type()) {
	case model.StringType:
		getType = ""
	case model.NumberType:
		getType = "Double"
	case model.IntType:
		getType = "Int32"
	case model.BoolType:
		getType = "Boolean"
	}

	typeParam := ""
	if getType == "Object" {
		// compute the type parameter T for the call to config.GetObject<T>(...)
		computedTypeParam := computeConfigTypeParam(v.Name(), v.Type())
		typeParam = fmt.Sprintf("<%s>", computedTypeParam)
	}

	getOrRequire := "Get"
	if v.DefaultValue == nil {
		getOrRequire = "Require"
	}

	if v.Description != "" {
		for _, line := range strings.Split(v.Description, "\n") {
			g.Fgenf(w, "%s// %s\n", g.Indent, line)
		}
	}

	name := makeValidIdentifier(v.Name())
	if v.DefaultValue != nil && !model.IsOptionalType(v.Type()) {
		typ := v.DefaultValue.Type()
		if _, ok := typ.(*model.PromiseType); ok {
			g.Fgenf(w, "%svar %s = Output.Create(config.%s%s%s(\"%s\"))",
				g.Indent, name, getOrRequire, getType, typeParam, v.LogicalName())
		} else {
			g.Fgenf(w, "%svar %s = config.%s%s%s(\"%s\")",
				g.Indent, name, getOrRequire, getType, typeParam, v.LogicalName())
		}
		expr := g.lowerExpression(v.DefaultValue, v.DefaultValue.Type())
		g.Fgenf(w, " ?? %.v", expr)
	} else {
		g.Fgenf(w, "%svar %s = config.%s%s%s(\"%s\")",
			g.Indent, name, getOrRequire, getType, typeParam, v.LogicalName())
	}
	g.Fgenf(w, ";\n")
}

func (g *generator) genLocalVariable(w io.Writer, localVariable *pcl.LocalVariable) {
	g.genTrivia(w, localVariable.Definition.Tokens.Name)
	variableName := makeValidIdentifier(localVariable.Name())
	value := localVariable.Definition.Value
	functionSchema, isInvokeCall := g.isFunctionInvoke(localVariable)
	if isInvokeCall {
		result := g.lowerExpressionWithoutApplies(value, value.Type())
		g.functionInvokes[variableName] = functionSchema
		g.Fgenf(w, "%svar %s = %v;\n\n", g.Indent, variableName, result)
	} else {
		result := g.lowerExpression(value, value.Type())
		g.Fgenf(w, "%svar %s = %v;\n\n", g.Indent, variableName, result)
	}
}

func (g *generator) genNYI(w io.Writer, reason string, vs ...interface{}) {
	message := "not yet implemented: " + fmt.Sprintf(reason, vs...)
	g.diagnostics = append(g.diagnostics, &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  message,
		Detail:   message,
	})
	g.Fgenf(w, "\"TODO: %s\"", fmt.Sprintf(reason, vs...))
}
