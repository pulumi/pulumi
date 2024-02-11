package gen

import (
	"bytes"
	"fmt"
	gofmt "go/format"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/hashicorp/hcl/v2"
	"github.com/iancoleman/strcase"

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

const (
	IndexToken   = "index"
	fromBase64Fn = "fromBase64"
)

type generator struct {
	// The formatter to use when generating code.
	*format.Formatter
	program             *pcl.Program
	packages            map[string]*schema.Package
	contexts            map[string]map[string]*pkgContext
	diagnostics         hcl.Diagnostics
	spills              *spills
	jsonTempSpiller     *jsonSpiller
	ternaryTempSpiller  *tempSpiller
	readDirTempSpiller  *readDirSpiller
	splatSpiller        *splatSpiller
	optionalSpiller     *optionalSpiller
	inlineInvokeSpiller *inlineInvokeSpiller
	scopeTraversalRoots codegen.StringSet
	arrayHelpers        map[string]*promptToInputArrayHelper
	isErrAssigned       bool
	tmpVarCount         int
	configCreated       bool
	externalCache       *Cache

	// Tracks imports for a file as we generate code.
	importer *fileImporter

	// inGenTupleConExprListArgs indicates that a the generator is processing an args list within a TupleConExpression.
	inGenTupleConExprListArgs bool
	isPtrArg                  bool
	isComponent               bool

	// User-configurable options
	assignResourcesToVariables bool // Assign resource to a new variable instead of _.
}

// GenerateProgramOptions are used to configure optional generator behavior.
type GenerateProgramOptions struct {
	AssignResourcesToVariables bool // Assign resource to a new variable instead of _.
	ExternalCache              *Cache
}

func GenerateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	pcl.MapProvidersAsResources(program)
	return GenerateProgramWithOptions(program, GenerateProgramOptions{})
}

func newGenerator(program *pcl.Program, opts GenerateProgramOptions) (*generator, error) {
	packages, contexts := map[string]*schema.Package{}, map[string]map[string]*pkgContext{}
	packageDefs, err := programPackageDefs(program)
	if err != nil {
		return nil, err
	}

	if opts.ExternalCache == nil {
		opts.ExternalCache = globalCache
	}

	for _, pkg := range packageDefs {
		packages[pkg.Name], contexts[pkg.Name] = pkg, getPackages("tool", pkg, opts.ExternalCache)
	}

	g := &generator{
		program:             program,
		packages:            packages,
		contexts:            contexts,
		spills:              &spills{counts: map[string]int{}},
		jsonTempSpiller:     &jsonSpiller{},
		ternaryTempSpiller:  &tempSpiller{},
		readDirTempSpiller:  &readDirSpiller{},
		splatSpiller:        &splatSpiller{},
		optionalSpiller:     &optionalSpiller{},
		inlineInvokeSpiller: &inlineInvokeSpiller{},
		scopeTraversalRoots: codegen.NewStringSet(),
		arrayHelpers:        make(map[string]*promptToInputArrayHelper),
		externalCache:       opts.ExternalCache,
		importer:            newFileImporter(),
	}

	// Apply any generate options.
	g.assignResourcesToVariables = opts.AssignResourcesToVariables

	g.Formatter = format.NewFormatter(g)
	return g, nil
}

type ObjectTypeFromConfigMetadata = struct {
	TypeName      string
	ComponentName string
}

func annotateObjectTypedConfig(componentName string, typeName string, objectType *model.ObjectType) *model.ObjectType {
	objectType.Annotations = append(objectType.Annotations, &ObjectTypeFromConfigMetadata{
		TypeName:      typeName,
		ComponentName: componentName,
	})

	return objectType
}

func configObjectTypeName(variableName string) string {
	return Title(variableName) + "Args"
}

// collectObjectTypedConfigVariables returns the object types in config variables need to be emitted
// as classes.
func collectObjectTypedConfigVariables(component *pcl.Component) map[string]*model.ObjectType {
	objectTypes := map[string]*model.ObjectType{}
	for _, config := range component.Program.ConfigVariables() {
		componentName := Title(component.Name())
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

func componentInputElementType(pclType model.Type) string {
	switch pclType {
	case model.BoolType:
		return "pulumi.BoolInput"
	case model.IntType:
		return "pulumi.IntInput"
	case model.NumberType:
		return "pulumi.Float64Input"
	case model.StringType:
		return "pulumi.StringInput"
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
			} else {
				return "interface{}"
			}
		default:
			return "interface{}"
		}
	}
}

func componentInputType(pclType model.Type) string {
	switch pclType := pclType.(type) {
	case *model.ListType:
		elementType := componentInputElementType(pclType.ElementType)
		return "[]" + elementType
	case *model.MapType:
		elementType := componentInputElementType(pclType.ElementType)
		return "map[string]" + elementType
	default:
		return componentInputElementType(pclType)
	}
}

func (g *generator) genComponentArgs(w io.Writer, componentName string, component *pcl.Component) {
	configVariables := component.Program.ConfigVariables()
	argsTypeName := Title(componentName) + "Args"

	objectTypedConfigVars := collectObjectTypedConfigVariables(component)
	variableNames := pcl.SortedStringKeys(objectTypedConfigVars)
	// generate resource args for this component
	for _, variableName := range variableNames {
		objectType := objectTypedConfigVars[variableName]
		objectTypeName := configObjectTypeName(variableName)
		g.Fprintf(w, "type %s struct {\n", objectTypeName)
		g.Indented(func() {
			propertyNames := pcl.SortedStringKeys(objectType.Properties)
			for _, propertyName := range propertyNames {
				propertyType := objectType.Properties[propertyName]
				inputType := componentInputType(propertyType)
				g.Fprintf(w, "%s%s %s\n",
					g.Indent,
					Title(propertyName),
					inputType)
			}
		})
		g.Fprintf(w, "%s}\n\n", g.Indent)
	}

	g.Fgenf(w, "type %s struct {\n", argsTypeName)
	g.Indented(func() {
		for _, config := range configVariables {
			g.Fgenf(w, g.Indent)
			fieldName := Title(config.LogicalName())
			inputType := componentInputType(config.Type())
			switch configType := config.Type().(type) {
			case *model.ObjectType:
				// for objects of type T, generate T as is
				inputType = "*" + configObjectTypeName(config.Name())
			case *model.ListType:
				// for list(T) where T is an object type, generate T[]
				switch configType.ElementType.(type) {
				case *model.ObjectType:
					objectTypeName := configObjectTypeName(config.Name())
					inputType = "[]*" + objectTypeName
				}
			case *model.MapType:
				// for map(T) where T is an object type, generate Dictionary<string, T>
				switch configType.ElementType.(type) {
				case *model.ObjectType:
					objectTypeName := configObjectTypeName(config.Name())
					inputType = "map[string]*" + objectTypeName
				}
			}
			g.Fgenf(w, "%s %s\n", fieldName, inputType)
		}
	})
	g.Fgenf(w, "}\n\n")
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

func (g *generator) genComponentType(w io.Writer, componentName string, component *pcl.Component) {
	outputs := component.Program.OutputVariables()
	componentTypeName := Title(componentName)
	g.Fgenf(w, "type %s struct {\n", componentTypeName)
	g.Indented(func() {
		g.Fgenf(w, g.Indent)
		g.Fgenf(w, "pulumi.ResourceState\n")
		for _, output := range outputs {
			g.Fgenf(w, g.Indent)
			fieldName := Title(output.LogicalName())
			fieldType := "pulumi.AnyOutput" // TODO: update this
			g.Fgenf(w, "%s %s\n", fieldName, fieldType)
			g.Fgenf(w, "")
		}
	})
	g.Fgenf(w, "}\n\n")
}

func (g *generator) genComponentDefinition(w io.Writer, componentName string, component *pcl.Component) {
	componentTypeName := Title(componentName)
	argsTypeName := Title(componentName) + "Args"
	g.Fgenf(w, "func New%s(\n", componentTypeName)
	g.Indented(func() {
		g.Fgenf(w, "%sctx *pulumi.Context,\n", g.Indent)
		g.Fgenf(w, "%sname string,\n", g.Indent)
		g.Fgenf(w, "%sargs *%s,\n", g.Indent, argsTypeName)
		g.Fgenf(w, "%sopts ...pulumi.ResourceOption,\n", g.Indent)
	})

	g.Fgenf(w, ") (*%s, error) {\n", componentTypeName)

	g.Indented(func() {
		g.Fgenf(w, "%svar componentResource %s\n", g.Indent, componentTypeName)
		token := "components:index:" + componentTypeName
		g.Fgenf(w, "%serr := ctx.RegisterComponentResource(\"%s\", ", g.Indent, token)
		g.Fgenf(w, "name, &componentResource, opts...)\n")
		g.Fgenf(w, "%sif err != nil {\n", g.Indent)
		g.Indented(func() {
			g.Fgenf(w, "%sreturn nil, err\n", g.Indent)
		})
		g.Fgenf(w, "%s}\n", g.Indent)

		// because of the RegisterRemoteComponentResource call
		g.isErrAssigned = true

		for _, node := range pcl.Linearize(component.Program) {
			switch node := node.(type) {
			case *pcl.LocalVariable:
				g.genLocalVariable(w, node)
			case *pcl.Resource:
				if node.Options == nil {
					node.Options = &pcl.ResourceOptions{}
				}

				node.Options.Parent = model.VariableReference(&model.Variable{
					Name: "&componentResource",
				})

				g.genResource(w, node)
			case *pcl.Component:
				if node.Options == nil {
					node.Options = &pcl.ResourceOptions{}
				}

				node.Options.Parent = model.VariableReference(&model.Variable{
					Name: "&componentResource",
				})

				g.genComponent(w, node)
			}
		}

		outputs := component.Program.OutputVariables()

		if len(outputs) == 0 {
			g.Fgenf(w, "err = %sctx.RegisterResourceOutputs(&componentResource, pulumi.Map{})\n", g.Indent)
			g.Fgenf(w, "%sif err != nil {\n", g.Indent)
			g.Indented(func() {
				g.Fgenf(w, "%sreturn nil, err\n", g.Indent)
			})
			g.Fgenf(w, "%s}\n", g.Indent)
		} else {
			g.Fgenf(w, "err = %sctx.RegisterResourceOutputs(&componentResource, pulumi.Map{\n", g.Indent)
			g.Indented(func() {
				for _, output := range outputs {
					g.Fgenf(w, "%s\"%s\": %v,\n", g.Indent, output.LogicalName(), output.Value)
				}
			})
			g.Fgenf(w, "%s})\n", g.Indent)

			g.Fgenf(w, "%sif err != nil {\n", g.Indent)
			g.Indented(func() {
				g.Fgenf(w, "%sreturn nil, err\n", g.Indent)
			})
			g.Fgenf(w, "%s}\n", g.Indent)

			for _, output := range outputs {
				g.Fgenf(w, "%scomponentResource.%s = %v\n", g.Indent, Title(output.Name()), output.Value)
			}
		}
		g.Fgenf(w, "%sreturn &componentResource, nil\n", g.Indent)
	})
	g.Fgenf(w, "}\n")
}

func GenerateProgramWithOptions(program *pcl.Program, opts GenerateProgramOptions) (
	map[string][]byte, hcl.Diagnostics, error,
) {
	g, err := newGenerator(program, opts)
	if err != nil {
		return nil, nil, err
	}

	// Linearize the nodes into an order appropriate for procedural code generation.
	nodes := pcl.Linearize(program)

	helpers := g.collectImports(program)

	var progPostamble bytes.Buffer
	for _, n := range nodes {
		g.collectScopeRoots(n)
	}

	for _, n := range nodes {
		g.genNode(&progPostamble, n)
	}

	helpers = helpers.Union(g.collectImports(program))

	g.genPostamble(&progPostamble, nodes)

	// We must generate the program first and the preamble second and finally cat the two together.
	// This is because nested object/tuple cons expressions can require imports that aren't
	// present in resource declarations or invokes alone. Expressions are lowered when the program is generated
	// and this must happen first so we can access types via __convert intrinsics.
	var index bytes.Buffer
	g.genPreamble(&index, program, helpers)
	g.Fprintf(&index, "func main() {\n")
	g.Fprintf(&index, "pulumi.Run(func(ctx *pulumi.Context) error {\n")
	index.Write(progPostamble.Bytes())

	mainProgramContent := index.Bytes()
	// Run Go formatter on the code before saving to disk
	formattedSource, err := gofmt.Source(mainProgramContent)
	if err == nil {
		// if we were able to format the code, use prefer the formatted version
		mainProgramContent = formattedSource
	} else {
		// add a warning diagnostic when there is a formatting error
		g.diagnostics = g.diagnostics.Append(&hcl.Diagnostic{
			Severity: hcl.DiagWarning,
			Subject:  &hcl.Range{Filename: "main.go"},
			Summary:  "could not format go code",
			Detail:   err.Error(),
		})
	}

	files := map[string][]byte{
		"main.go": mainProgramContent,
	}

	for componentDir, component := range program.CollectComponents() {
		g.importer.Reset()

		componentFilename := filepath.Base(componentDir)
		componentName := component.DeclarationName()
		componentGenerator, err := newGenerator(component.Program, opts)
		componentGenerator.isComponent = true
		componentHelperse := componentGenerator.collectImports(component.Program)
		for _, n := range component.Program.Nodes {
			componentGenerator.collectScopeRoots(n)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("could not create a new generator: %w", err)
		}
		var componentBuffer bytes.Buffer
		componentGenerator.genPreamble(&componentBuffer, component.Program, componentHelperse)

		componentGenerator.genComponentArgs(&componentBuffer, componentName, component)
		componentGenerator.genComponentType(&componentBuffer, componentName, component)
		componentGenerator.genComponentDefinition(&componentBuffer, componentName, component)
		componentContent := componentBuffer.Bytes()
		formattedComponentSource, err := gofmt.Source(componentContent)
		if err == nil {
			// if we were able to format the code, use prefer the formatted version
			componentContent = formattedComponentSource
		} else {
			// add a warning diagnostic when there is a formatting error
			componentGenerator.diagnostics = componentGenerator.diagnostics.Append(&hcl.Diagnostic{
				Severity: hcl.DiagWarning,
				Subject:  &hcl.Range{Filename: componentFilename + ".go"},
				Summary:  "could not format go code",
				Detail:   err.Error(),
			})
		}
		files[componentFilename+".go"] = componentContent
	}
	return files, g.diagnostics, nil
}

func GenerateProjectFiles(project workspace.Project, program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	files, diagnostics, err := GenerateProgram(program)
	if err != nil {
		return files, diagnostics, err
	}
	if diagnostics.HasErrors() {
		return files, diagnostics, err
	}

	// Set the runtime to "go" then marshal to Pulumi.yaml
	project.Runtime = workspace.NewProjectRuntimeInfo("go", nil)
	projectBytes, err := encoding.YAML.Marshal(project)
	if err != nil {
		return nil, diagnostics, err
	}
	files["Pulumi.yaml"] = projectBytes

	// Build a go.mod based on the packages used by program
	var gomod bytes.Buffer
	gomod.WriteString("module " + project.Name.String() + "\n")
	gomod.WriteString(`
go 1.20

require (
	github.com/pulumi/pulumi/sdk/v3 v3.30.0
`)

	// For each package add a PackageReference line
	packages, err := programPackageDefs(program)
	if err != nil {
		return nil, diagnostics, err
	}
	for _, p := range packages {
		if p.Name == "pulumi" {
			continue
		}
		if err := p.ImportLanguages(map[string]schema.Language{"go": Importer}); err != nil {
			return nil, diagnostics, err
		}

		if p.Version != nil && p.Version.Major <= 0 {
			// Let `go mod tidy` resolve pre-1.0 and non-module package versions on InstallDependencies,
			// as it better handles the way we use `importBasePath`. What we need is a `modulePath`. `go
			// get` handles these cases, which are not parseable, they depend on retrieving the target
			// repository and downloading it to disk.
			//
			// Here are two cases, first the parseable case:
			//
			// * go get github.com/pulumi/pulumi-aws/sdk/v5/go/aws@v5.3.0
			//          ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ module path
			//                                           ~~ major version
			//                                              ~~~~~~ package path - can be any number of path parts
			//                                                     ~~~~~~ version
			//
			// Here, we can cut on the major version.

			// * go get github.com/pulumi/pulumi-aws-native/sdk/go/aws@v0.16.0
			//          ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ module path
			//                                                  ~~~~~~ package path - can be any number of path parts
			//                                                         ~~~~~~~ version
			//
			// Here we cannot cut on the major version, as it isn't present. The only way to resolve this
			// package is to pull the repo.
			//
			// Fortunately for these pre-1.0 releases, `go mod tidy` on the generated repo will at least
			// add the module based on the import generated in the .go files, but it will always get the
			// latest version.

			if info, ok := p.Language["go"]; ok {
				if info, ok := info.(GoPackageInfo); ok && info.ModulePath != "" {
					fmt.Fprintf(&gomod, " %s v%s\n", info.ModulePath, p.Version.String())
				}
			}
			continue
		}

		// Relatively safe default, this works for Pulumi provider packages:
		vPath := ""
		if p.Version != nil && p.Version.Major > 1 {
			vPath = fmt.Sprintf("/v%d", p.Version.Major)
		}
		packageName := fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s", p.Name, vPath, p.Name)
		if langInfo, found := p.Language["go"]; found {
			goInfo, ok := langInfo.(GoPackageInfo)
			if ok && goInfo.ImportBasePath != "" {
				separatorIndex := strings.Index(goInfo.ImportBasePath, vPath)
				if separatorIndex < 0 {
					packageName = ""
				} else {
					modulePrefix := goInfo.ImportBasePath[:separatorIndex]
					packageName = fmt.Sprintf("%s%s", modulePrefix, vPath)
				}
			}
		}

		version := ""
		if p.Version != nil {
			version = "v" + p.Version.String()
		}
		if packageName != "" {
			fmt.Fprintf(&gomod, "	%s %s\n", packageName, version)
		}
	}

	gomod.WriteString(")")

	files["go.mod"] = gomod.Bytes()

	return files, diagnostics, nil
}

func GenerateProject(
	directory string, project workspace.Project,
	program *pcl.Program, localDependencies map[string]string,
) error {
	files, diagnostics, err := GenerateProjectFiles(project, program)
	if err != nil {
		return err
	}

	if diagnostics.HasErrors() {
		return diagnostics
	}

	for filename, data := range files {
		outPath := path.Join(directory, filename)
		err := os.WriteFile(outPath, data, 0o600)
		if err != nil {
			return fmt.Errorf("could not write output program: %w", err)
		}
	}

	return nil
}

var packageContexts sync.Map

func getPackages(tool string, pkg *schema.Package, cache *Cache) map[string]*pkgContext {
	if v, ok := packageContexts.Load(pkg); ok {
		return v.(map[string]*pkgContext)
	}

	if err := pkg.ImportLanguages(map[string]schema.Language{"go": Importer}); err != nil {
		return nil
	}

	var goPkgInfo GoPackageInfo
	if goInfo, ok := pkg.Language["go"].(GoPackageInfo); ok {
		goPkgInfo = goInfo
	}
	v, err := generatePackageContextMap(tool, pkg.Reference(), goPkgInfo, cache)
	contract.AssertNoErrorf(err, "Could not generate package context map")
	packageContexts.Store(pkg, v)
	return v
}

func (g *generator) collectScopeRoots(n pcl.Node) {
	diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
		if st, ok := n.(*model.ScopeTraversalExpression); ok {
			g.scopeTraversalRoots.Add(st.RootName)
		}
		return n, nil
	})
	contract.Assertf(len(diags) == 0, "Expcted no diagnostics, got %d", len(diags))
}

// genPreamble generates package decl, imports, and opens the main func
func (g *generator) genPreamble(w io.Writer, program *pcl.Program, preambleHelperMethods codegen.StringSet) {
	g.Fprint(w, "package main\n\n")
	g.Fprintf(w, "import (\n")

	g.importer.Import("github.com/pulumi/pulumi/sdk/v3/go/pulumi", "pulumi")
	for idx, group := range g.importer.ImportGroups() {
		if idx > 0 {
			g.Fprintf(w, "\n")
		}
		for _, imp := range group {
			g.Fprintf(w, "\t%s\n", imp)
		}
	}
	g.Fprintf(w, ")\n")

	// If we collected any helper methods that should be added, write them just before the main func
	for _, preambleHelperMethodBody := range preambleHelperMethods.SortedValues() {
		g.Fprintf(w, "%s\n\n", preambleHelperMethodBody)
	}
}

func (g *generator) collectTypeImports(program *pcl.Program, t schema.Type) {
	var token string
	switch t := t.(type) {
	case *schema.InputType:
		g.collectTypeImports(program, t.ElementType)
		return
	case *schema.OptionalType:
		g.collectTypeImports(program, t.ElementType)
		return
	case *schema.ArrayType:
		g.collectTypeImports(program, t.ElementType)
		return
	case *schema.MapType:
		g.collectTypeImports(program, t.ElementType)
		return
	case *schema.UnionType:
		for _, t := range t.ElementTypes {
			g.collectTypeImports(program, t)
		}
		return
	case *schema.ObjectType:
		token = t.Token
	case *schema.EnumType:
		token = t.Token
	case *schema.TokenType:
		token = t.Token
	case *schema.ResourceType:
		token = t.Token
	}
	if token == "" {
		return
	}

	var tokenRange hcl.Range
	pkg, mod, name, _ := pcl.DecomposeToken(token, tokenRange)
	vPath, err := g.getVersionPath(program, pkg)
	if err != nil {
		panic(err)
	}
	g.addPulumiImport(pkg, vPath, mod, name)
}

// collect Imports returns two sets of packages imported by the program, std lib packages and pulumi packages
func (g *generator) collectImports(program *pcl.Program) (helpers codegen.StringSet) {
	helpers = codegen.NewStringSet()

	// Accumulate import statements for the various providers
	for _, n := range program.Nodes {
		if r, isResource := n.(*pcl.Resource); isResource {
			pcl.FixupPulumiPackageTokens(r)
			pkg, mod, name, _ := r.DecomposeToken()
			if pkg == "pulumi" {
				if mod == "providers" {
					pkg = name
					mod = ""
				} else if mod == "" {
					continue
				}
			}
			vPath, err := g.getVersionPath(program, pkg)
			if err != nil {
				if r.Schema != nil {
					panic(err)
				}
				// for unknown resources, make a best guess
				vPath = "/v1"
			}

			g.addPulumiImport(pkg, vPath, mod, name)
		}
		if _, isConfigVar := n.(*pcl.ConfigVariable); isConfigVar {
			g.importer.Import("github.com/pulumi/pulumi/sdk/v3/go/pulumi/config", "config")
		}

		diags := n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
			if call, ok := n.(*model.FunctionCallExpression); ok {
				if call.Name == pcl.Invoke {
					tokenArg := call.Args[0]
					token := tokenArg.(*model.TemplateExpression).Parts[0].(*model.LiteralValueExpression).Value.AsString()
					tokenRange := tokenArg.SyntaxNode().Range()
					pkg, mod, name, diagnostics := pcl.DecomposeToken(token, tokenRange)
					if call.Type() == model.DynamicType {
						// then this is an unknown function, create a dummy import for it
						dummyVersionPath := "/v1"
						g.addPulumiImport(pkg, dummyVersionPath, mod, name)
						return call, nil
					}

					contract.Assertf(len(diagnostics) == 0, "Expected no diagnostics, got %d", len(diagnostics))

					vPath, err := g.getVersionPath(program, pkg)
					if err != nil {
						panic(err)
					}
					g.addPulumiImport(pkg, vPath, mod, name)
				} else if call.Name == pcl.IntrinsicConvert {
					g.collectConvertImports(program, call)
				}

				// Checking to see if this function call deserves its own dedicated helper method in the preamble
				if helperMethodBody, ok := getHelperMethodIfNeeded(call.Name, g.Indent); ok {
					helpers.Add(helperMethodBody)
				}
			}
			return n, nil
		})
		contract.Assertf(len(diags) == 0, "Expected no diagnostics, got %d", len(diags))

		if g.isComponent {
			// needed for resource names
			g.importer.Import("fmt", "fmt")
		}

		diags = n.VisitExpressions(nil, func(n model.Expression) (model.Expression, hcl.Diagnostics) {
			if call, ok := n.(*model.FunctionCallExpression); ok {
				for _, fnPkg := range g.genFunctionPackages(call) {
					// Currently, for all stdlib packages,
					//   basename($import) == $name
					// In the future, we may need to add a
					// mapping from $import to $name.
					g.importer.Import(fnPkg, path.Base(fnPkg) /* name */)
				}
			}
			if t, ok := n.(*model.TemplateExpression); ok {
				if len(t.Parts) > 1 {
					g.importer.Import("fmt", "fmt")
				}
			}
			return n, nil
		})
		contract.Assertf(len(diags) == 0, "Expected no diagnostics, got %d", len(diags))
	}

	return helpers
}

func (g *generator) collectConvertImports(
	program *pcl.Program,
	call *model.FunctionCallExpression,
) {
	if schemaType, ok := pcl.GetSchemaForType(call.Type()); ok {
		// Sometimes code for a `__convert` call does not
		// really use the import of the result type. In such
		// cases it is important not to generate a
		// non-compiling unused import. Detect some of these
		// cases here.
		//
		// Fully solving this is deferred for later:
		// TODO[pulumi/pulumi#8324].
		switch arg0 := call.Args[0].(type) {
		case *model.TemplateExpression:
			if lit, ok := arg0.Parts[0].(*model.LiteralValueExpression); ok &&
				call.Type().AssignableFrom(lit.Type()) {
				return
			}
		case *model.ScopeTraversalExpression:
			if call.Type().AssignableFrom(arg0.Type()) {
				return
			}
		}
		g.collectTypeImports(program, schemaType)
	}
}

func (g *generator) getVersionPath(program *pcl.Program, pkg string) (string, error) {
	for _, p := range program.PackageReferences() {
		if p.Name() == pkg {
			if ver := p.Version(); ver != nil && ver.Major > 1 {
				return fmt.Sprintf("/v%d", ver.Major), nil
			}
			return "", nil
		}
	}

	return "", fmt.Errorf("could not find package version information for pkg: %s", pkg)
}

func (g *generator) getGoPackageInfo(pkg string) (GoPackageInfo, bool) {
	p, ok := g.packages[pkg]
	if !ok {
		return GoPackageInfo{}, false
	}
	info, ok := p.Language["go"].(GoPackageInfo)
	return info, ok
}

func (g *generator) addPulumiImport(pkg, versionPath, mod, name string) {
	importPath := func(mod, importBasePath string) string {
		if importBasePath == "" {
			importBasePath = fmt.Sprintf("github.com/pulumi/pulumi-%s/sdk%s/go/%s", pkg, versionPath, pkg)
		}

		if mod != "" && mod != IndexToken {
			return fmt.Sprintf("%s/%s", importBasePath, mod)
		}
		return importBasePath
	}

	// We do this before we let the user set overrides. That way the user can still have a
	// module named IndexToken.
	info, hasInfo := g.getGoPackageInfo(pkg) // We're allowing `info` to be zero-initialized
	if !hasInfo {
		path := importPath(mod, "")
		// users hasn't provided any extra overrides
		if mod == "" || mod == IndexToken {
			mod = pkg
		}

		if strings.Contains(mod, "-") {
			// convert the dashed package name into camelCase
			mod = strcase.ToLowerCamel(mod)
		}
		g.importer.Import(path, mod)
		return
	}

	if m, ok := info.ModuleToPackage[mod]; ok {
		mod = m
	}

	path := importPath(mod, info.ImportBasePath)
	if alias, ok := info.PackageImportAliases[path]; ok {
		g.importer.Import(path, alias)
		return
	}

	// Trim off anything after the first '/'.
	// This handles transforming modules like s3/bucket to s3 (as found in
	// aws:s3/bucket:Bucket).
	mod = strings.SplitN(mod, "/", 2)[0]

	path = importPath(mod, info.ImportBasePath)
	pkgName := mod
	if len(pkgName) == 0 || pkgName == IndexToken {
		// If mod is empty, then the package is the root package.
		pkgName = pkg
	}
	g.importer.Import(path, pkgName)
}

// genPostamble closes the method
func (g *generator) genPostamble(w io.Writer, nodes []pcl.Node) {
	if !g.isComponent {
		g.Fprint(w, "return nil\n")
		g.Fprintf(w, "})\n")
		g.Fprintf(w, "}\n")
	}

	g.genHelpers(w)
}

func (g *generator) genHelpers(w io.Writer) {
	for _, v := range g.arrayHelpers {
		v.generateHelperMethod(w)
	}
}

func (g *generator) genNode(w io.Writer, n pcl.Node) {
	switch n := n.(type) {
	case *pcl.Resource:
		g.genResource(w, n)
	case *pcl.Component:
		g.genComponent(w, n)
	case *pcl.OutputVariable:
		g.genOutputAssignment(w, n)
	case *pcl.ConfigVariable:
		g.genConfigVariable(w, n)
	case *pcl.LocalVariable:
		g.genLocalVariable(w, n)
	}
}

var resourceType = model.NewOpaqueType("pulumi.Resource")

func (g *generator) lowerResourceOptions(opts *pcl.ResourceOptions) (*model.Block, []interface{}) {
	if opts == nil {
		return nil, nil
	}

	var block *model.Block
	var temps []interface{}
	appendOption := func(name string, value model.Expression, destType model.Type) {
		if block == nil {
			block = &model.Block{
				Type: "options",
				Body: &model.Body{},
			}
		}

		value, valueTemps := g.lowerExpression(value, destType)
		temps = append(temps, valueTemps...)

		block.Body.Items = append(block.Body.Items, &model.Attribute{
			Tokens: syntax.NewAttributeTokens(name),
			Name:   name,
			Value:  value,
		})
	}

	if opts.Parent != nil {
		appendOption("Parent", opts.Parent, model.DynamicType)
	}
	if opts.Provider != nil {
		appendOption("Provider", opts.Provider, model.DynamicType)
	}
	if opts.DependsOn != nil {
		appendOption("DependsOn", opts.DependsOn, model.NewListType(resourceType))
	}
	if opts.Protect != nil {
		appendOption("Protect", opts.Protect, model.BoolType)
	}
	if opts.RetainOnDelete != nil {
		appendOption("RetainOnDelete", opts.RetainOnDelete, model.BoolType)
	}
	if opts.IgnoreChanges != nil {
		appendOption("IgnoreChanges", opts.IgnoreChanges, model.NewListType(model.StringType))
	}

	return block, temps
}

func (g *generator) genResourceOptions(w io.Writer, block *model.Block) {
	if block == nil {
		return
	}

	for _, item := range block.Body.Items {
		attr := item.(*model.Attribute)
		g.Fgenf(w, ", pulumi.%s(%v)", attr.Name, attr.Value)
	}
}

func (g *generator) genResource(w io.Writer, r *pcl.Resource) {
	resName, resNameVar := r.LogicalName(), makeValidIdentifier(r.Name())
	pkg, mod, typ, _ := r.DecomposeToken()
	originalMod := mod
	if pkg == "pulumi" && mod == "providers" {
		pkg = typ
		mod = ""
		typ = "Provider"
	}
	if mod == "" || strings.HasPrefix(mod, "/") || strings.HasPrefix(mod, "index/") {
		originalMod = mod
		mod = pkg
	}

	// Compute resource options
	options, temps := g.lowerResourceOptions(r.Options)
	g.genTemps(w, temps)

	if r.Schema != nil {
		// Add conversions to input properties
		for _, input := range r.Inputs {
			destType, diagnostics := r.InputType.Traverse(hcl.TraverseAttr{Name: input.Name})
			g.diagnostics = append(g.diagnostics, diagnostics...)
			expr, temps := g.lowerExpression(input.Value, destType.(model.Type))
			expr, invokeTemps := g.rewriteInlineInvokes(expr)
			for _, t := range invokeTemps {
				temps = append(temps, t)
			}
			input.Value = expr
			g.genTemps(w, temps)
		}
	}

	if r.Schema == nil {
		// for unknown resource the type name of the resource should be upper-case
		typ = Title(typ)
	}

	modOrAlias := g.getModOrAlias(pkg, mod, originalMod)

	instantiate := func(varName, resourceName string, w io.Writer) {
		if g.scopeTraversalRoots.Has(varName) || strings.HasPrefix(varName, "__") {
			g.Fgenf(w, "%s, err := %s.New%s(ctx, %s, ", varName, modOrAlias, typ, resourceName)
		} else {
			assignment := ":="
			if g.isErrAssigned {
				assignment = "="
			}
			if g.assignResourcesToVariables {
				g.Fgenf(w, "%s, err := %s.New%s(ctx, %s, ",
					strcase.ToLowerCamel(resourceName), modOrAlias, typ, resourceName)
			} else {
				g.Fgenf(w, "_, err %s %s.New%s(ctx, %s, ", assignment, modOrAlias, typ, resourceName)
			}
		}
		g.isErrAssigned = true

		if len(r.Inputs) > 0 {
			g.Fgenf(w, "&%s.%sArgs{\n", modOrAlias, typ)
			for _, attr := range r.Inputs {
				g.Fgenf(w, "%s: %.v,\n", strings.Title(attr.Name), attr.Value)
			}
			g.Fprint(w, "}")
		} else {
			g.Fprint(w, "nil")
		}
		g.genResourceOptions(w, options)
		g.Fprint(w, ")\n")
		g.Fgenf(w, "if err != nil {\n")
		if g.isComponent {
			g.Fgenf(w, "return nil, err\n")
		} else {
			g.Fgenf(w, "return err\n")
		}
		g.Fgenf(w, "}\n")
	}

	g.genTrivia(w, r.Definition.Tokens.GetType(""))
	for _, l := range r.Definition.Tokens.GetLabels(nil) {
		g.genTrivia(w, l)
	}
	g.genTrivia(w, r.Definition.Tokens.GetOpenBrace())
	if r.Options != nil && r.Options.Range != nil {
		rangeType := model.ResolveOutputs(r.Options.Range.Type())
		rangeExpr, temps := g.lowerExpression(r.Options.Range, rangeType)
		g.genTemps(w, temps)

		g.Fgenf(w, "var %s []*%s.%s\n", resNameVar, modOrAlias, typ)

		// ahead of range statement declaration generate the resource instantiation
		// to detect and removed unused k,v variables
		var buf bytes.Buffer
		resourceName := fmt.Sprintf(`fmt.Sprintf("%s-%%v", key0)`, resName)
		if g.isComponent {
			resourceName = fmt.Sprintf(`fmt.Sprintf("%%s-%s-%%v", name, key0)`, resName)
		}
		instantiate("__res", resourceName, &buf)
		instantiation := buf.String()
		isValUsed := strings.Contains(instantiation, "val0")
		valVar := "_"
		if isValUsed {
			valVar = "val0"
		}
		if model.InputType(model.NumberType).ConversionFrom(rangeExpr.Type()) != model.NoConversion {
			g.Fgenf(w, "for index := 0; index < %.v; index++ {\n", rangeExpr)
			g.Indented(func() {
				g.Fgenf(w, "%skey0 := index\n", g.Indent)
				g.Fgenf(w, "%s%s := index\n", g.Indent, valVar)
			})
		} else {
			g.Fgenf(w, "for key0, %s := range %.v {\n", valVar, rangeExpr)
		}

		g.Fgen(w, instantiation)
		g.Fgenf(w, "%[1]s = append(%[1]s, __res)\n", resNameVar)
		g.Fgenf(w, "}\n")

	} else {
		resourceName := fmt.Sprintf("%q", resName)
		if g.isComponent {
			resourceName = fmt.Sprintf(`fmt.Sprintf("%%s-%s", name)`, resName)
		}
		instantiate(resNameVar, resourceName, w)
	}
}

func AnnotateComponentInputs(component *pcl.Component) {
	componentName := Title(component.Name())
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

func (g *generator) genComponent(w io.Writer, r *pcl.Component) {
	resName, resNameVar := r.LogicalName(), makeValidIdentifier(r.Name())
	// Compute resource options
	options, temps := g.lowerResourceOptions(r.Options)
	g.genTemps(w, temps)

	AnnotateComponentInputs(r)

	configVariables := r.Program.ConfigVariables()
	// Add conversions to input properties
	for _, input := range r.Inputs {
		for _, config := range configVariables {
			if config.Name() == input.Name {
				destType := config.Type()
				expr, temps := g.lowerExpression(input.Value, destType)
				input.Value = expr
				g.genTemps(w, temps)
			}
		}
	}

	componentName := r.DeclarationName()

	instantiate := func(varName, resourceName string, w io.Writer) {
		if g.scopeTraversalRoots.Has(varName) || strings.HasPrefix(varName, "__") {
			g.Fgenf(w, "%s, err := New%s(ctx, %s, ", varName, componentName, resourceName)
		} else {
			assignment := ":="
			if g.isErrAssigned {
				assignment = "="
			}
			if g.assignResourcesToVariables {
				g.Fgenf(w, "%s, err := New%s(ctx, %s, ",
					strcase.ToLowerCamel(resourceName), componentName, resourceName)
			} else {
				g.Fgenf(w, "_, err %s New%s(ctx, %s, ", assignment, componentName, resourceName)
			}
		}
		g.isErrAssigned = true

		if len(r.Inputs) > 0 {
			g.Fgenf(w, "&%sArgs{\n", componentName)
			for _, attr := range r.Inputs {
				g.Fgenf(w, "%s: %.v,\n", strings.Title(attr.Name), attr.Value)
			}
			g.Fprint(w, "}")
		} else {
			g.Fprint(w, "nil")
		}
		g.genResourceOptions(w, options)
		g.Fprint(w, ")\n")
		g.Fgenf(w, "if err != nil {\n")
		if g.isComponent {
			g.Fgenf(w, "return nil, err\n")
		} else {
			g.Fgenf(w, "return err\n")
		}
		g.Fgenf(w, "}\n")
	}

	if r.Options != nil && r.Options.Range != nil {
		rangeType := model.ResolveOutputs(r.Options.Range.Type())
		rangeExpr, temps := g.lowerExpression(r.Options.Range, rangeType)
		g.genTemps(w, temps)

		g.Fgenf(w, "var %s []*%s\n", resNameVar, componentName)

		// ahead of range statement declaration generate the resource instantiation
		// to detect and removed unused k,v variables
		var buf bytes.Buffer
		resourceName := fmt.Sprintf(`fmt.Sprintf("%s-%%v", key0)`, resName)
		if g.isComponent {
			resourceName = fmt.Sprintf(`fmt.Sprintf("%%s-%s-%%v", name, key0)`, resName)
		}
		instantiate("__res", resourceName, &buf)
		instantiation := buf.String()
		isValUsed := strings.Contains(instantiation, "val0")
		valVar := "_"
		if isValUsed {
			valVar = "val0"
		}
		if model.InputType(model.NumberType).ConversionFrom(rangeExpr.Type()) != model.NoConversion {
			g.Fgenf(w, "for index := 0; index < %.v; index++ {\n", rangeExpr)
			g.Indented(func() {
				g.Fgenf(w, "%skey0 := index\n", g.Indent)
				g.Fgenf(w, "%s%s := index\n", g.Indent, valVar)
			})
		} else {
			g.Fgenf(w, "for key0, %s := range %.v {\n", valVar, rangeExpr)
		}

		g.Fgen(w, instantiation)
		g.Fgenf(w, "%[1]s = append(%[1]s, __res)\n", resNameVar)
		g.Fgenf(w, "}\n")

	} else {
		resourceName := fmt.Sprintf("%q", resName)
		if g.isComponent {
			resourceName = fmt.Sprintf(`fmt.Sprintf("%%s-%s", name)`, resName)
		}
		instantiate(resNameVar, resourceName, w)
	}
}

func (g *generator) genOutputAssignment(w io.Writer, v *pcl.OutputVariable) {
	expr, temps := g.lowerExpression(v.Value, v.Type())
	g.genTemps(w, temps)
	g.Fgenf(w, "ctx.Export(%q, %.3v)\n", v.LogicalName(), expr)
}

func (g *generator) genTemps(w io.Writer, temps []interface{}) {
	singleReturn := ""
	g.genTempsMultiReturn(w, temps, singleReturn)
}

func (g *generator) genTempsMultiReturn(w io.Writer, temps []interface{}, zeroValueType string) {
	genZeroValueDecl := false

	if zeroValueType != "" {
		for _, t := range temps {
			switch t.(type) {
			case *spillTemp, *readDirTemp:
				genZeroValueDecl = true
			default:
			}
		}
		if genZeroValueDecl {
			// TODO add entropy to var name
			// currently only used inside anonymous functions (no scope collisions)
			g.Fgenf(w, "var _zero %s\n", zeroValueType)
		}

	}

	for _, t := range temps {
		switch t := t.(type) {
		case *ternaryTemp:
			// TODO derive from ambient context
			isInput := false
			g.Fgenf(w, "var %s %s\n", t.Name, g.argumentTypeName(t.Value.TrueResult, t.Type(), isInput))
			g.Fgenf(w, "if %.v {\n", t.Value.Condition)
			g.Fgenf(w, "%s = %.v\n", t.Name, t.Value.TrueResult)
			g.Fgenf(w, "} else {\n")
			g.Fgenf(w, "%s = %.v\n", t.Name, t.Value.FalseResult)
			g.Fgenf(w, "}\n")
		case *spillTemp:
			bytesVar := "tmp" + strings.ToUpper(t.Variable.Name)
			g.Fgenf(w, "%s, err := json.Marshal(", bytesVar)
			args := t.Value.(*model.FunctionCallExpression).Args[0]
			g.Fgenf(w, "%.v)\n", args)
			g.Fgenf(w, "if err != nil {\n")
			if genZeroValueDecl {
				g.Fgenf(w, "return _zero, err\n")
			} else {
				g.Fgenf(w, "return err\n")
			}
			g.Fgenf(w, "}\n")
			g.Fgenf(w, "%s := string(%s)\n", t.Variable.Name, bytesVar)
			g.isErrAssigned = true
		case *readDirTemp:
			tmpSuffix := strings.Split(t.Name, "files")[1]
			g.Fgenf(w, "%s, err := os.ReadDir(%.v)\n", t.Name, t.Value.Args[0])
			g.Fgenf(w, "if err != nil {\n")
			if genZeroValueDecl {
				g.Fgenf(w, "return _zero, err\n")
			} else {
				g.Fgenf(w, "return err\n")
			}
			g.Fgenf(w, "}\n")
			namesVar := "fileNames" + tmpSuffix
			g.Fgenf(w, "%s := make([]string, len(%s))\n", namesVar, t.Name)
			iVar := "key" + tmpSuffix
			valVar := "val" + tmpSuffix
			g.Fgenf(w, "for %s, %s := range %s {\n", iVar, valVar, t.Name)
			g.Fgenf(w, "%s[%s] = %s.Name()\n", namesVar, iVar, valVar)
			g.Fgenf(w, "}\n")
			g.isErrAssigned = true
		case *splatTemp:
			argTyp := g.argumentTypeName(t.Value.Each, t.Value.Each.Type(), false)
			if strings.Contains(argTyp, ".") {
				g.Fgenf(w, "var %s %sArray\n", t.Name, argTyp)
			} else {
				g.Fgenf(w, "var %s []%s\n", t.Name, argTyp)
			}
			g.Fgenf(w, "for _, val0 := range %.v {\n", t.Value.Source)
			g.Fgenf(w, "%s = append(%s, %.v)\n", t.Name, t.Name, t.Value.Each)
			g.Fgenf(w, "}\n")
		case *optionalTemp:
			g.Fgenf(w, "%s := %.v\n", t.Name, t.Value)
		case *inlineInvokeTemp:
			g.Fgenf(w, "%s, err := %.v\n", t.Name, t.Value)
			g.Fgenf(w, "if err != nil {\n")
			g.Fgenf(w, "return err\n")
			g.Fgenf(w, "}\n")
			g.isErrAssigned = true
		default:
			contract.Failf("unexpected temp type: %v", t)
		}
	}
}

func (g *generator) genLocalVariable(w io.Writer, v *pcl.LocalVariable) {
	expr, temps := g.lowerExpression(v.Definition.Value, v.Type())
	g.genTemps(w, temps)
	name := makeValidIdentifier(v.Name())
	assignment := ":="
	if !g.scopeTraversalRoots.Has(v.Name()) {
		name = "_"
		if g.isErrAssigned {
			assignment = "="
		}
	}
	g.genTrivia(w, v.Definition.Tokens.Name)
	switch expr := expr.(type) {
	case *model.FunctionCallExpression:
		switch expr.Name {
		case pcl.Invoke:
			// OutputVersionedInvoke does not return an error
			noError, _, _ := pcl.RecognizeOutputVersionedInvoke(expr)
			if noError {
				if name == "_" {
					assignment = "="
				}
				g.Fgenf(w, "%s %s %.3v;\n", name, assignment, expr)
			} else {
				g.Fgenf(w, "%s, err %s %.3v;\n", name, assignment, expr)
				g.isErrAssigned = true
				g.Fgenf(w, "if err != nil {\n")
				g.Fgenf(w, "return err\n")
				g.Fgenf(w, "}\n")
			}
		case pcl.IntrinsicApply:
			if name == "_" {
				assignment = "="
			}
			g.Fgenf(w, "%s %s ", name, assignment)
			g.genApply(w, expr)
			g.Fgenf(w, "\n")
		case "join", "mimeType",
			"fileArchive", "remoteArchive", "assetArchive",
			"fileAsset", "stringAsset", "remoteAsset",
			"toBase64":
			if name == "_" {
				assignment = "="
			}
			g.Fgenf(w, "%s %s %.3v;\n", name, assignment, expr)
		case fromBase64Fn:
			tmpVar := fmt.Sprintf("%s%d", "tmpVar", g.tmpVarCount)
			g.Fgenf(w, "%s, _ := %.3v;\n", tmpVar, expr)
			if name == "_" {
				assignment = "="
			}
			g.Fgenf(w, "%s %s string(%s)\n", name, assignment, tmpVar)
			g.tmpVarCount++
		default:
			if name == "_" {
				assignment = "="
			}
			g.Fgenf(w, "%s %s %.3v;\n", name, assignment, expr)
		}
	default:
		g.Fgenf(w, "%s := %.3v;\n", name, expr)

	}
}

func (g *generator) genConfigVariable(w io.Writer, v *pcl.ConfigVariable) {
	if !g.configCreated {
		g.Fprint(w, "cfg := config.New(ctx, \"\")\n")
		g.configCreated = true
	}

	getType := ""
	switch v.Type() {
	case model.StringType: // Already default
	case model.NumberType:
		getType = "Float64"
	case model.IntType:
		getType = "Int"
	case model.BoolType:
		getType = "Bool"
	case model.DynamicType:
		getType = "Object"
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
	if v.DefaultValue == nil {
		g.Fgenf(w, "%s := cfg.%s%s(\"%s\")\n", name, getOrRequire, getType, v.LogicalName())
	} else {
		expr, temps := g.lowerExpression(v.DefaultValue, v.DefaultValue.Type())
		g.genTemps(w, temps)
		switch expr := expr.(type) {
		case *model.FunctionCallExpression:
			switch expr.Name {
			case pcl.Invoke:
				g.Fgenf(w, "%s, err := %.3v;\n", name, expr)
				g.isErrAssigned = true
				g.Fgenf(w, "if err != nil {\n")
				g.Fgenf(w, "return err\n")
				g.Fgenf(w, "}\n")
			}
		default:
			switch v.Type() {
			// Go will default to interpreting integers (i.e. 3) as ints, even if the config is Number
			case model.NumberType:
				g.Fgenf(w, "%s := float64(%.3v);\n", name, expr)
			default:
				g.Fgenf(w, "%s := %.3v;\n", name, expr)
			}
		}
		switch v.Type() {
		case model.StringType:
			g.Fgenf(w, "if param := cfg.Get(\"%s\"); param != \"\"{\n", v.LogicalName())
		case model.NumberType:
			g.Fgenf(w, "if param := cfg.GetFloat64(\"%s\"); param != 0 {\n", v.LogicalName())
		case model.IntType:
			g.Fgenf(w, "if param := cfg.GetInt(\"%s\"); param != 0 {\n", v.LogicalName())
		case model.BoolType:
			g.Fgenf(w, "if param := cfg.GetBool(\"%s\"); param {\n", v.LogicalName())
		default:
			g.Fgenf(w, "if param := cfg.GetObject(\"%s\"); param != nil {\n", v.LogicalName())
		}
		g.Fgenf(w, "%s = param\n", name)
		g.Fgen(w, "}\n")
	}
}

// useLookupInvokeForm takes a token for an invoke and determines whether to use the
// .Get or .Lookup form. The Go SDK has collisions in .Get methods that require renaming.
// For instance, gen.go creates a resource getter for AWS VPCs that collides with a function:
// GetVPC resource getter: https://github.com/pulumi/pulumi-aws/blob/7835df354694e2f9f23371602a9febebc6b45be8/sdk/go/aws/ec2/getVpc.go#L15
// LookupVPC function: https://github.com/pulumi/pulumi-aws/blob/7835df354694e2f9f23371602a9febebc6b45be8/sdk/go/aws/ec2/getVpc.go#L15
// Given that the naming here is not consisten, we must reverse the process from gen.go.
//
//nolint:lll
func (g *generator) useLookupInvokeForm(token string) bool {
	pkg, module, member, _ := pcl.DecomposeToken(token, *new(hcl.Range))
	modSplit := strings.Split(module, "/")
	mod := modSplit[0]
	fn := Title(member)
	if mod == IndexToken && len(modSplit) >= 2 {
		// e.g. "aws:index/getPartition:getPartition" where module is "index/getPartition"
		mod = ""
		fn = Title(modSplit[1])
	} else {
		// e.g. for type "ec2/getVpc:getVpcArgs"
		if _, has := g.contexts[pkg][mod]; !has {
			mod = module
		}
	}
	fnLookup := "Lookup" + fn[3:]
	pkgContext, has := g.contexts[pkg][mod]
	if has && pkgContext.names.Has(fnLookup) {
		return true
	}
	return false
}

// getModOrAlias attempts to reconstruct the import statement and check if the imported package
// is aliased, returning that alias if available.
func (g *generator) getModOrAlias(pkg, mod, originalMod string) string {
	info, ok := g.getGoPackageInfo(pkg)
	if !ok {
		needsAliasing := strings.Contains(mod, "-")
		if needsAliasing {
			return strcase.ToLowerCamel(mod)
		}
		return mod
	}

	importPath := func(mod string) string {
		importBasePath := info.ImportBasePath
		if mod != "" && mod != IndexToken {
			return fmt.Sprintf("%s/%s", importBasePath, mod)
		}
		return importBasePath
	}

	if m, ok := info.ModuleToPackage[mod]; ok {
		mod = m
	} else {
		mod = originalMod
	}

	path := importPath(mod)
	if alias, ok := info.PackageImportAliases[path]; ok {
		return g.importer.Import(path, alias)
	}

	// Trim off anything after the first '/'.
	// This handles transforming modules like s3/bucket to s3 (as found in
	// aws:s3/bucket:Bucket).
	mod = strings.SplitN(mod, "/", 2)[0]

	path = importPath(mod)
	pkgName := mod
	if len(pkgName) == 0 || pkgName == IndexToken {
		// If mod is empty, then the package is the root package.
		pkgName = pkg
	}
	return g.importer.Import(path, pkgName)
}

// Go needs complete package definitions in order to properly resolve names.
//
// TODO: naming decisions should really be encoded statically so that they can be decided locally.
func programPackageDefs(program *pcl.Program) ([]*schema.Package, error) {
	refs := program.PackageReferences()
	defs := make([]*schema.Package, len(refs))
	for i, ref := range refs {
		def, err := ref.Definition()
		if err != nil {
			return nil, err
		}
		defs[i] = def
	}
	return defs, nil
}

// fileImporter tracks imports in a single generated file.
// It ensures that there are no conflicts between imports
// with the same package name.
type fileImporter struct {
	// used holds all identifier names that have been used
	// for imports in this file, and the imports that they refer to.
	used map[string]string // identifier name -> import path

	// For import paths where the package name is not unique,
	// this map holds the name that was used for the import.
	aliases map[string]string // import path -> alias
}

func newFileImporter() *fileImporter {
	return &fileImporter{
		used:    make(map[string]string),
		aliases: make(map[string]string),
	}
}

// Import imports a package with the given import path
// and returns the name that should be used to refer to it.
//
// name is the name of the package at the import path.
// This must always match the 'package' statement in the imported package.
//
// Note that returned name may be different from the name argument.
// This happens when the name is already used for another import,
// and requires an alias to avoid a conflict.
//
//	foo := i.Import("example.com/foo", "foo")
//	if foo == "foo" {
//	    fmt.Printf(`import "example.com/foo"`)
//	} else {
//	    fmt.Printf(`import %s "example.com/foo"`, foo)
//	}
//
// Import will never return the same name for two different import paths
// in the same file.
//
// For example:
//
//	aws1 := i.Import("example.com/foo/aws", "aws")
//	aws2 := i.Import("example.com/bar/aws", "aws")
//	fmt.Println(aws1 == aws2) // false
func (fi *fileImporter) Import(importPath string, name string) (actualName string) {
	contract.Requiref(importPath != "", "importPath", "must not be empty")
	contract.Requiref(name != "", "name", "must not be empty (importPath: %q)", importPath)

	// For readability, always add an alias if the package name
	// does not match the base name of the import path.
	// For example, "example.com/foo-go" with package "foo"
	// should get:
	//
	//	import foo "example.com/foo-go"
	if filepath.Base(importPath) != name {
		defer func() { fi.aliases[importPath] = actualName }()
	}

	// Already imported using the same name.
	if imported, ok := fi.used[name]; ok && imported == importPath {
		return name // no alias
	}

	// Preferred name has not yet been used.
	if _, ok := fi.used[name]; !ok {
		fi.used[name] = importPath
		return name
	}

	// Already imported with an alias. Use that alias.
	if other, ok := fi.aliases[importPath]; ok {
		return other
	}

	// The name is taken. We need a unique unused alias.
	// If the import path has at least two "/"s,
	// we'll try to combine the last two parts of the path
	// into a single identifier.
	//
	// For example, if "github.com/pulumi/pulumi-aws/sdk/go/awsx/s3"
	// conflicts, we'll try to use "awsxs3" instead.
	if idx := secondLastIndex(importPath, "/"); idx != -1 {
		// "example.com/foo/bar/baz" -> "bar/baz"
		candidate := importPath[idx+1:]
		if candidate, ok := toIdentifier(candidate); ok {
			if _, ok := fi.used[candidate]; !ok {
				fi.used[candidate] = importPath
				fi.aliases[importPath] = candidate
				return candidate
			}
		}
	}

	// If that doesn't work, we'll just append a number.
	for i := 2; ; i++ {
		candidate := name + strconv.Itoa(i)
		if _, ok := fi.used[candidate]; ok {
			continue // already used
		}

		fi.used[candidate] = importPath
		fi.aliases[importPath] = candidate
		return candidate
	}
}

// Reports all imports made with Import so far as groups of import specs.
// These can be used to generate an import block with separate import groups.
//
// The specs are sorted by import path.
//
// Usage:
//
//	fmt.Printf("import (\n")
//	for _, group := range i.ImportGroups() {
//		for _, spec := range group {
//			fmt.Printf("    %s\n", spec)
//		}
//	}
//
// This example collapses all imports into a single group.
// Typically, you would want to separate the groups with a blank line.
func (fi *fileImporter) ImportGroups() [][]string {
	importPaths := make([]string, 0, len(fi.used))
	for _, importPath := range fi.used {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)

	// We currently generate only two groups:
	//
	// 1. stdlib imports
	// 2. everything else
	//
	// We can determine if an import is stdlib by checking if its path
	// contains a '.'.
	// See https://github.com/golang/go/issues/32819 for discussion,
	// but:
	//
	// > Import paths without dots are reserved for the standard library
	// > and toolchain
	var (
		stdlib []string
		other  []string
	)
	for _, importPath := range importPaths {
		var spec string
		if alias, ok := fi.aliases[importPath]; ok {
			spec = alias + " " + strconv.Quote(importPath)
			// e.g. foo "example.com/foo"
		} else {
			spec = strconv.Quote(importPath)
			// e.g. "example.com/foo"
		}
		if strings.Contains(importPath, ".") {
			other = append(other, spec)
		} else {
			stdlib = append(stdlib, spec)
		}
	}

	var groups [][]string
	if len(stdlib) > 0 {
		groups = append(groups, stdlib)
	}
	if len(other) > 0 {
		groups = append(groups, other)
	}
	return groups
}

// Reset resets the importer to its initial state.
func (fi *fileImporter) Reset() {
	fi.used = make(map[string]string)
	fi.aliases = make(map[string]string)
}

// Turns a string into an identifier by dropping all characters
// that are not alphamumeric or underscore.
//
// Returns false if the string cannot be turned into an identifier.
func toIdentifier(s string) (_ string, ok bool) {
	var b strings.Builder
	for _, r := range s {
		if r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	o := b.String()
	return o, token.IsIdentifier(o)
}

// Returns the index of the second-to-last occurrence of needle in haystack.
// If there is no second-to-last occurrence, returns -1.
func secondLastIndex(haystack, needle string) int {
	// Note that we can't just do []byte(haystack) and then iterate backwards
	// because of unicode.
	// Instead we'll use strings.LastIndex twice.
	last := strings.LastIndex(haystack, needle)
	if last == -1 {
		return -1
	}
	return strings.LastIndex(haystack[:last], needle)
}
