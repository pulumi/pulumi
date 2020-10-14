//go:generate go run ./bundler.go -dir=./templates -out=templates.go

package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/types"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v2/codegen/go"
	"github.com/pulumi/pulumi/pkg/v2/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v2/codegen/python"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"golang.org/x/tools/go/packages"
)

var textRegex = regexp.MustCompile(`^s*(?:(?://\s?)|(?:/\*+))?s?(.*?)(?:s*\*+/)?s*$`)

func getCommentText(comment *ast.Comment) (string, bool) {
	// Remove any Pulumi annotations.
	if strings.HasPrefix(comment.Text, "//pulumi:") {
		return "", false
	}

	// Trim spaces and remove any leading or trailing comment markers.
	// Remove any leading or trailing comment markers.
	return textRegex.FindStringSubmatch(comment.Text)[1], true
}

func genDocString(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}

	var text strings.Builder

	// Remove leading blank lines.
	comments := doc.List
	for ; len(comments) > 0; comments = comments[1:] {
		line, ok := getCommentText(comments[0])
		if !ok || line == "" {
			continue
		}
		break
	}

	// Add each block of blank lines followed by text. This will remove any trailing blanks.
	blanks := 0
	for ; len(comments) > 0; comments = comments[1:] {
		line, ok := getCommentText(comments[0])
		switch {
		case !ok:
			continue
		case line == "":
			blanks++
		default:
			for ; blanks > 0; blanks-- {
				text.WriteRune('\n')
			}
			text.WriteString(line)
			text.WriteRune('\n')
		}
	}

	// TODO: parse and normalize Markdown.

	return text.String()
}

func (m *pulumiModule) genTypeRef(rootName string, subject ast.Node, typ types.Type) (schema.TypeSpec, hcl.Diagnostics) {
	switch typ := typ.(type) {
	case *types.Array:
		element, diags := m.genTypeRef(rootName, subject, typ.Elem())
		return schema.TypeSpec{
			Type:  "array",
			Items: &element,
		}, diags
	case *types.Slice:
		element, diags := m.genTypeRef(rootName, subject, typ.Elem())
		return schema.TypeSpec{
			Type:  "array",
			Items: &element,
		}, diags
	case *types.Basic:
		switch typ.Kind() {
		case types.Bool:
			return schema.TypeSpec{Type: "boolean"}, nil
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
			return schema.TypeSpec{Type: "integer"}, nil
		case types.Float32, types.Float64:
			return schema.TypeSpec{Type: "number"}, nil
		case types.String:
			return schema.TypeSpec{Type: "string"}, nil
		default:
			return schema.TypeSpec{}, hcl.Diagnostics{m.errorf(subject, "unsupported type %v referenced by %v", typ, rootName)}
		}
	case *types.Interface:
		if typ.NumMethods() == 0 {
			return schema.TypeSpec{Ref: "pulumi.json#/Any"}, nil
		}
	case *types.Map:
		if !types.ConvertibleTo(typ.Key(), types.Typ[types.String]) {
			return schema.TypeSpec{}, hcl.Diagnostics{m.errorf(subject, "unsupported type %v referenced by %v: map keys must be convertible to string", typ, rootName)}
		}
		value, diags := m.genTypeRef(rootName, subject, typ.Elem())
		return schema.TypeSpec{
			Type:                 "object",
			AdditionalProperties: &value,
		}, diags
	case *types.Named:
		if output, ok := m.pulumiSDK.hasOutputType(typ); ok {
			resolved, ok := m.pulumiPackage.outputTypes.get(output).(types.Type)
			if !ok {
				return schema.TypeSpec{}, hcl.Diagnostics{m.errorf(subject, "unresolved input or output type %v referenced by %v", typ, rootName)}
			}
			return m.genTypeRef(rootName, subject, resolved)
		}

		typeInfo := m.pulumiPackage.typeClosure.get(typ)
		if typeInfo == nil {
			return schema.TypeSpec{Ref: "pulumi.json#/Any"}, hcl.Diagnostics{m.errorf(subject, "unsupported reference to external type %v", typ)}
		}
		return schema.TypeSpec{
			Ref: fmt.Sprintf("#/types/%v", typeInfo.(*pulumiType).token),
		}, nil
	case *types.Pointer:
		return m.genTypeRef(rootName, subject, typ.Elem())
	case *types.Struct:
		return schema.TypeSpec{}, hcl.Diagnostics{m.errorf(subject, "unsupported anonymous struct type referenced by %v", rootName)}
	}
	return schema.TypeSpec{}, hcl.Diagnostics{m.errorf(subject, "unsupported type %v referenced by %v", typ, rootName)}
}

func (m *pulumiModule) genProperty(rootName string, syntax *ast.Field) (string, bool, schema.PropertySpec, hcl.Diagnostics) {
	fieldName := ""
	if len(syntax.Names) != 0 {
		fieldName = syntax.Names[len(syntax.Names)-1].Name
	}
	fieldType, ok := m.goPackage.TypesInfo.Types[syntax.Type]
	if !ok {
		return "", false, schema.PropertySpec{}, hcl.Diagnostics{m.errorf(syntax, "internal error: no type information for %v", fieldName)}
	}

	required := false
	if _, isPtr := fieldType.Type.Underlying().(*types.Pointer); !isPtr {
		required = true
	}

	desc, ok := getPulumiPropertyDesc(syntax)
	if !ok {
		return "", false, schema.PropertySpec{}, nil
	}

	propertyName := desc[0]
	if propertyName == "" {
		propertyName = camelCase(fieldName)
	}

	var diags hcl.Diagnostics
	spec := schema.PropertySpec{}
	for _, opt := range desc[1:] {
		switch opt {
		case "required":
			required = true
		case "optional":
			required = false
		case "deprecated":
			spec.DeprecationMessage = "this field is deprecated"
		case "secret":
			spec.Secret = true
		case "immutable":
			// ignore this
		default:
			diags = append(diags, m.errorf(syntax.Tag, "unknown option '%v' in tag", opt))
		}
	}

	propertyType, propertyDiags := m.genTypeRef(rootName, syntax, fieldType.Type)
	diags = append(diags, propertyDiags...)

	spec.Description = genDocString(syntax.Doc)
	spec.TypeSpec = propertyType

	return propertyName, required, spec, diags
}

func (m *pulumiModule) genObjectType(syntax *ast.TypeSpec, doc *ast.CommentGroup) (schema.ObjectTypeSpec, hcl.Diagnostics) {
	// The importer should have ensured that the underlying type of all Pulumi types is a struct.
	structType := syntax.Type.(*ast.StructType)

	properties := map[string]schema.PropertySpec{}
	var requiredProperties []string
	var diags hcl.Diagnostics
	for _, field := range structType.Fields.List {
		propertyName, isRequired, propertySpec, propertyDiags := m.genProperty(syntax.Name.Name, field)
		diags = append(diags, propertyDiags...)

		if propertyName != "" {
			properties[propertyName] = propertySpec
			if isRequired {
				requiredProperties = append(requiredProperties, propertyName)
			}
		}
	}

	return schema.ObjectTypeSpec{
		Description: genDocString(doc),
		Properties:  properties,
		Required:    requiredProperties,
		Type:        "object",
	}, diags
}

func (m *pulumiModule) genType(typ *pulumiType) (schema.ComplexTypeSpec, hcl.Diagnostics) {
	// TODO: enums
	objectType, diags := m.genObjectType(typ.syntax, typ.doc)
	return schema.ComplexTypeSpec{ObjectTypeSpec: objectType}, diags
}

func (m *pulumiModule) genResource(resource *pulumiResource) (schema.ResourceSpec, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	// Generate the args type.
	argsType := resource.args.(*types.Pointer).Elem().(*types.Named)
	argsSyntax, ok := m.paramsTypes.get(argsType).(*ast.TypeSpec)
	if !ok {
		return schema.ResourceSpec{}, hcl.Diagnostics{m.errorf(resource.syntax, "internal error: no syntax node for argument type %v", argsType)}
	}

	argsObject, argsDiags := m.genObjectType(argsSyntax, nil)
	diags = append(diags, argsDiags...)

	// Generate the state type.
	stateObject, stateDiags := m.genObjectType(resource.syntax, resource.doc)
	diags = append(diags, stateDiags...)

	// TODO: deprecation

	return schema.ResourceSpec{
		ObjectTypeSpec:  stateObject,
		InputProperties: argsObject.Properties,
		RequiredInputs:  argsObject.Required,
		IsComponent:     resource.isComponent,
	}, diags
}

func (m *pulumiModule) genFunction(function *pulumiFunction) (schema.FunctionSpec, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	// Generate the args type.
	argsType := function.args.(*types.Pointer).Elem().(*types.Named)
	argsSyntax, ok := m.paramsTypes.get(argsType).(*ast.TypeSpec)
	if !ok {
		return schema.FunctionSpec{}, hcl.Diagnostics{m.errorf(function.syntax, "internal error: no syntax node for argument type %v", argsType)}
	}
	argsObject, argsDiags := m.genObjectType(argsSyntax, nil)
	diags = append(diags, argsDiags...)

	// Generate the result type.
	resultType := function.result.(*types.Pointer).Elem().(*types.Named)
	resultSyntax, ok := m.paramsTypes.get(resultType).(*ast.TypeSpec)
	if !ok {
		return schema.FunctionSpec{}, hcl.Diagnostics{m.errorf(function.syntax, "internal error: no syntax node for result type %v", resultType)}
	}
	resultObject, resultDiags := m.genObjectType(resultSyntax, nil)
	diags = append(diags, resultDiags...)

	// TODO: deprecation

	return schema.FunctionSpec{
		Description: genDocString(function.syntax.Doc),
		Inputs:      &argsObject,
		Outputs:     &resultObject,
	}, diags
}

func (m *pulumiModule) genModule(packageSpec *schema.PackageSpec) hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, typ := range m.types {
		spec, typeDiags := m.genType(typ)
		diags = append(diags, typeDiags...)
		packageSpec.Types[typ.token] = spec
	}
	for _, resource := range m.resources {
		spec, typeDiags := m.genResource(resource)
		diags = append(diags, typeDiags...)
		packageSpec.Resources[resource.token] = spec
	}
	for _, function := range m.functions {
		spec, typeDiags := m.genFunction(function)
		diags = append(diags, typeDiags...)
		packageSpec.Functions[function.token] = spec
	}
	return diags
}

func (p *pulumiPackage) genPackage() (schema.PackageSpec, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	spec := schema.PackageSpec{
		Name:      p.name,
		Version:   "0.0.1",
		Types:     map[string]schema.ComplexTypeSpec{},
		Resources: map[string]schema.ResourceSpec{},
		Functions: map[string]schema.FunctionSpec{},
		Language: map[string]json.RawMessage{
			"nodejs": rawMessage(nodejs.NodePackageInfo{
				Dependencies:    map[string]string{"@pulumi/pulumi": "^2.0.0"},
				DevDependencies: map[string]string{"@types/node": "^8.0.0"},
			}),
			"python": rawMessage(python.PackageInfo{
				Requires:      map[string]string{"pulumi": ">=2.9.0,<3.0.0"},
				UsesIOClasses: true,
			}),
			"csharp": rawMessage(dotnet.CSharpPackageInfo{
				PackageReferences: map[string]string{
					"Pulumi":                       "2.*",
					"System.Collections.Immutable": "1.6.0",
				},
			}),
		},
	}

	provider, providerDiags := p.providerModule.genResource(p.provider)
	diags = append(diags, providerDiags...)
	spec.Provider = provider

	for _, m := range p.modules {
		moduleDiags := m.genModule(&spec)
		diags = append(diags, moduleDiags...)
	}

	return spec, diags
}

type genImport struct {
	Name string
	Path string
}

type genResource struct {
	Token        string
	ArgsType     string
	ResourceType string
	Constructor  string
}

type genFunction struct {
	Token    string
	ArgsType string
	Function string
}

type genContext struct {
	Package string
	Version string

	ProviderType     string
	ProviderArgsType string

	ProviderImports []genImport
	ResourceImports []genImport
	FunctionImports []genImport

	CustomResources    []genResource
	ComponentResources []genResource
	Functions          []genFunction
}

var templateNames = []string{"provider.go", "provider_configure.go", "provider_functions.go", "provider_resources.go"}

type imports struct {
	pathToName map[string]string
	names      map[string]struct{}
}

func (imports *imports) add(pkg *types.Package) string {
	if name, ok := imports.pathToName[pkg.Path()]; ok {
		return name
	}

	for i, name := 1, pkg.Name(); ; i++ {
		if _, ok := imports.names[name]; !ok {
			if imports.names == nil {
				imports.names = map[string]struct{}{}
			}
			if imports.pathToName == nil {
				imports.pathToName = map[string]string{}
			}

			imports.names[name] = struct{}{}
			imports.pathToName[pkg.Path()] = name
			return name
		}
		name = fmt.Sprintf("%s%v", pkg.Name(), i)
	}
}

func (imports *imports) gen() []genImport {
	generated := make([]genImport, 0, len(imports.pathToName))
	for path, name := range imports.pathToName {
		generated = append(generated, genImport{
			Name: name,
			Path: path,
		})
	}
	return generated
}

func (p *pulumiPackage) genProvider(files map[string][]byte, schema schema.PackageSpec) error {
	var resources []genResource
	var functions []genFunction
	var providerImports, resourceImports, functionImports imports
	for _, m := range p.modules {
		for _, r := range m.resources {
			// TODO: component resources
			if r.isComponent {
				continue
			}

			importName := resourceImports.add(m.goPackage.Types)
			resources = append(resources, genResource{
				Token:        r.token,
				ArgsType:     fmt.Sprintf("%v.%v", importName, r.args.(*types.Pointer).Elem().(*types.Named).Obj().Name()),
				ResourceType: fmt.Sprintf("%v.%v", importName, r.syntax.Name.Name),
			})
		}
		for _, f := range m.functions {
			importName := functionImports.add(m.goPackage.Types)
			functions = append(functions, genFunction{
				Token:    f.token,
				ArgsType: fmt.Sprintf("%v.%v", importName, f.args.(*types.Pointer).Elem().(*types.Named).Obj().Name()),
				Function: fmt.Sprintf("%v.%v", importName, f.syntax.Name.Name),
			})
		}
	}

	providerImportName := providerImports.add(p.providerModule.goPackage.Types)

	context := genContext{
		Package:          p.name,
		Version:          "0.0.1",
		ProviderType:     fmt.Sprintf("%v.%v", providerImportName, p.provider.syntax.Name.Name),
		ProviderArgsType: fmt.Sprintf("%v.%v", providerImportName, p.provider.args.(*types.Pointer).Elem().(*types.Named).Obj().Name()),
		ProviderImports:  providerImports.gen(),
		ResourceImports:  resourceImports.gen(),
		FunctionImports:  functionImports.gen(),
		CustomResources:  resources,
		Functions:        functions,
	}

	schemaBytes, err := json.MarshalIndent(schema, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling schema: %v", err)
	}
	files[filepath.Join("provider", "schema.json")] = schemaBytes
	files[filepath.Join("provider", "schema.go")] = []byte(fmt.Sprintf("package %v\n\nvar pulumiSchema = %#v\n", p.name, schemaBytes))

	for _, templateName := range templateNames {
		var contents bytes.Buffer
		if err = bundledTemplates[templateName+".tmpl"].Execute(&contents, context); err != nil {
			return err
		}

		formatted, err := format.Source(contents.Bytes())
		if err != nil {
			return fmt.Errorf("formatting %v: %v", templateName, err)
		}

		files[filepath.Join("provider", templateName)] = formatted
	}

	return nil
}

func genSDKs(files map[string][]byte, packageSpec schema.PackageSpec) error {
	schema, err := schema.ImportSpec(packageSpec, map[string]schema.Language{
		"dotnet": dotnet.Importer,
		"go":     gogen.Importer,
		"nodejs": nodejs.Importer,
		"python": python.Importer,
	})
	if err != nil {
		return err
	}

	dotnetFiles, err := dotnet.GeneratePackage("go-provider", schema, nil)
	if err != nil {
		return err
	}
	goFiles, err := gogen.GeneratePackage("go-provider", schema)
	if err != nil {
		return err
	}
	nodeFiles, err := nodejs.GeneratePackage("go-provider", schema, nil)
	if err != nil {
		return err
	}
	pythonFiles, err := python.GeneratePackage("go-provider", schema, nil)
	if err != nil {
		return err
	}

	addSDK := func(path string, sdkFiles map[string][]byte) {
		for name, contents := range sdkFiles {
			files[filepath.Join("sdk", path, name)] = contents
		}
	}
	addSDK("dotnet", dotnetFiles)
	addSDK("go", goFiles)
	addSDK("nodejs", nodeFiles)
	addSDK("python", pythonFiles)

	return nil
}

func gatherPulumiPackage(name, rootPackagePath string, goPackages []*packages.Package) (*pulumiPackage, hcl.Diagnostics) {
	pp := &pulumiPackage{
		name:            name,
		rootPackagePath: rootPackagePath,
		typeClosure:     typeSet{},
		outputTypes:     typeSet{},
		roots:           newPackageSet(goPackages...),
	}

	// Gather the provider type, resource types, and functions.
	var diags hcl.Diagnostics
	for _, p := range goPackages {
		moduleDiags := pp.gatherModule(p)
		diags = append(diags, moduleDiags...)
	}

	return pp, diags
}

func (pp *pulumiPackage) importRoots() hcl.Diagnostics {
	var diags hcl.Diagnostics
	if pp.provider == nil {
		return hcl.Diagnostics{&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "no provider type found",
		}}
	}
	providerDiags := pp.importProviderType()
	diags = append(diags, providerDiags...)

	// Import the resources and functions and determine the closure of the referenced types.
	for _, m := range pp.modules {
		moduleDiags := m.importModule()
		diags = append(diags, moduleDiags...)
	}

	return diags
}

func (pp *pulumiPackage) gatherTypeClosure() hcl.Diagnostics {
	// Gather the definitions of the referenced types.
	var diags hcl.Diagnostics
	for _, m := range pp.modules {
		typesDiags := m.gatherMarkedTypes()
		diags = append(diags, typesDiags...)
	}
	return diags
}

func (pp *pulumiPackage) checkComponentResources() hcl.Diagnostics {
	// Check component resource types.
	var diags hcl.Diagnostics
	for _, m := range pp.modules {
		componentDiags := m.checkComponentResources()
		diags = append(diags, componentDiags...)
	}
	return diags
}

func Generate(name, rootPackagePath string, packages ...*packages.Package) (map[string][]byte, hcl.Diagnostics) {
	pp, diags := gatherPulumiPackage(name, rootPackagePath, packages)
	if diags.HasErrors() {
		return nil, diags
	}

	importDiags := pp.importRoots()
	if importDiags.HasErrors() {
		return nil, importDiags
	}

	// Resolve output element types.
	err := pp.resolveOutputTypes()
	if err != nil {
		return nil, hcl.Diagnostics{newError(nil, nil, fmt.Sprintf("internal error: %v", err))}
	}

	closureDiags := pp.gatherTypeClosure()
	diags = append(diags, closureDiags...)

	componentDiags := pp.checkComponentResources()
	diags = append(diags, componentDiags...)

	if diags.HasErrors() {
		return nil, diags
	}

	// Generate the package schema.
	packageSpec, specDiags := pp.genPackage()
	diags = append(diags, specDiags...)

	// Generate the provider files and SDKs.
	files := map[string][]byte{}
	if err = pp.genProvider(files, packageSpec); err != nil {
		return nil, hcl.Diagnostics{newError(nil, nil, fmt.Sprintf("internal error: %v", err))}
	}
	if err = genSDKs(files, packageSpec); err != nil {
		return nil, hcl.Diagnostics{newError(nil, nil, fmt.Sprintf("internal error: %v", err))}
	}

	return files, diags
}
