// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"
	yamlgen "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

func newPackageExampleCmd() *cobra.Command {
	var outDir string
	var language string
	var generateOnly bool
	var mappings []string
	var strict bool

	cmd := &cobra.Command{
		Use: "example <schema> <resource-token>",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			packageName := args[0]
			resourceToken := args[1]
			pkg, err := schemaFromSchemaSource(packageName)
			if err != nil {
				return err
			}

			var resourceSchema *schema.Resource
			for _, r := range pkg.Resources {
				if r.Token == resourceToken {
					resourceSchema = r
					break
				}
			}

			if resourceSchema == nil {
				return fmt.Errorf("resource %q not found in schema", resourceToken)
			}

			// create a temp directory
			dir, err := os.MkdirTemp("", "example")
			if err != nil {
				return err
			}

			defer os.RemoveAll(dir)

			examplePclPath := filepath.Join(dir, "main.pp")
			pcl := genCreationExampleSyntax(resourceSchema)
			fileError := os.WriteFile(examplePclPath, []byte(pcl), 0666)
			if fileError != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			outputDirectory := filepath.Join(cwd, outDir)
			return runExamples(env.Global(), args, dir, mappings, language, outputDirectory, generateOnly, strict)
		}),
	}

	cmd.PersistentFlags().StringVar(
		//nolint:lll
		&language, "language", "", "Which language plugin to use to generate the pulumi project")
	if err := cmd.MarkPersistentFlagRequired("language"); err != nil {
		panic("failed to mark 'language' as a required flag")
	}

	cmd.PersistentFlags().StringVar(
		//nolint:lll
		&outDir, "out", ".", "The output directory to write the converted project to")

	cmd.PersistentFlags().BoolVar(
		//nolint:lll
		&generateOnly, "generate-only", false, "Generate the converted program(s) only; do not install dependencies")

	cmd.PersistentFlags().StringSliceVar(
		//nolint:lll
		&mappings, "mappings", []string{}, "Any mapping files to use in the conversion")

	cmd.PersistentFlags().BoolVar(
		&strict, "strict", false, "If strict is set the conversion will fail on errors such as missing variables")

	return cmd
}

func runExamples(
	e env.Env,
	args []string,
	cwd string, mappings []string, language string,
	outDir string, generateOnly bool, strict bool,
) error {
	pCtx, err := newPluginContext(cwd)
	if err != nil {
		return fmt.Errorf("create plugin host: %w", err)
	}
	defer contract.IgnoreClose(pCtx.Host)

	from := "pcl"

	// Translate well known languages to runtimes
	switch language {
	case "csharp", "c#":
		language = "dotnet"
	case "typescript":
		language = "nodejs"
	}

	var projectGenerator projectGeneratorFunction
	switch language {
	case "dotnet":
		projectGenerator = generatorWrapper(
			func(targetDirectory string, proj workspace.Project, program *pcl.Program) error {
				return dotnet.GenerateProject(targetDirectory, proj, program, nil /*localDependencies*/)
			}, language)
	case "java":
		projectGenerator = generatorWrapper(javagen.GenerateProject, language)
	case "yaml":
		projectGenerator = generatorWrapper(yamlgen.GenerateProject, language)
	case "pulumi", "pcl":
		// No plugin for PCL to install dependencies with
		generateOnly = true
		projectGenerator = pclGenerateProject
	default:
		projectGenerator = func(
			sourceDirectory, targetDirectory string,
			proj *workspace.Project,
			loader schema.ReferenceLoader,
			strict bool,
		) (hcl.Diagnostics, error) {
			contract.Requiref(proj != nil, "proj", "must not be nil")

			programInfo := plugin.NewProgramInfo(cwd, cwd, "entry", nil)
			languagePlugin, err := pCtx.Host.LanguageRuntime(language, programInfo)
			if err != nil {
				return nil, err
			}

			loaderServer := schema.NewLoaderServer(loader)
			grpcServer, err := plugin.NewServer(pCtx, schema.LoaderRegistration(loaderServer))
			if err != nil {
				return nil, err
			}
			defer contract.IgnoreClose(grpcServer)

			projectBytes, err := encoding.JSON.Marshal(proj)
			if err != nil {
				return nil, err
			}
			projectJSON := string(projectBytes)

			diagnostics, err := languagePlugin.GenerateProject(
				sourceDirectory, targetDirectory, projectJSON,
				strict, grpcServer.Addr(), nil /*localDependencies*/)
			if err != nil {
				return diagnostics, err
			}
			return diagnostics, nil
		}
	}

	if outDir != "." {
		err := os.MkdirAll(outDir, 0o755)
		if err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	log := func(sev diag.Severity, msg string) {
		pCtx.Diag.Logf(sev, diag.RawMessage("", msg))
	}

	installProvider := func(provider tokens.Package) *semver.Version {
		// If auto plugin installs are disabled just return nil, the mapper will still carry on
		if env.DisableAutomaticPluginAcquisition.Value() {
			return nil
		}

		pluginSpec := workspace.PluginSpec{
			Name: string(provider),
			Kind: workspace.ResourcePlugin,
		}
		version, err := pkgWorkspace.InstallPlugin(pluginSpec, log)
		if err != nil {
			pCtx.Diag.Warningf(diag.Message("", "failed to install provider %q: %v"), provider, err)
			return nil
		}
		return version
	}

	loader := schema.NewPluginLoader(pCtx.Host)
	mapper, err := convert.NewPluginMapper(
		convert.DefaultWorkspace(), convert.ProviderFactoryFromHost(pCtx.Host),
		from, mappings, installProvider)
	if err != nil {
		return fmt.Errorf("create provider mapper: %w", err)
	}

	pclDirectory, err := os.MkdirTemp("", "pulumi-convert")
	if err != nil {
		return fmt.Errorf("create temporary directory: %w", err)
	}
	defer os.RemoveAll(pclDirectory)

	pCtx.Diag.Infof(diag.Message("", "Converting from %s..."), from)
	if from == "pcl" {
		// The source code is PCL, we don't need to do anything here, just repoint pclDirectory to it, but
		// remove the temp dir we just created first
		err = os.RemoveAll(pclDirectory)
		if err != nil {
			return fmt.Errorf("remove temporary directory: %w", err)
		}
		pclDirectory = cwd
	} else {
		converter, err := loadConverterPlugin(pCtx, from, log)
		if err != nil {
			return fmt.Errorf("load converter plugin: %w", err)
		}
		defer contract.IgnoreClose(converter)

		mapperServer := convert.NewMapperServer(mapper)
		loaderServer := schema.NewLoaderServer(loader)
		grpcServer, err := plugin.NewServer(pCtx,
			convert.MapperRegistration(mapperServer),
			schema.LoaderRegistration(loaderServer))
		if err != nil {
			return err
		}
		defer contract.IgnoreClose(grpcServer)

		resp, err := converter.ConvertProgram(pCtx.Request(), &plugin.ConvertProgramRequest{
			SourceDirectory: cwd,
			TargetDirectory: pclDirectory,
			MapperTarget:    grpcServer.Addr(),
			LoaderTarget:    grpcServer.Addr(),
			Args:            args,
		})
		if err != nil {
			return err
		}

		// We're done with the converter plugin now so can close it
		err = converter.Close()
		if err != nil {
			// Don't hard exit if we fail to close the converter but do tell the user
			pCtx.Diag.Warningf(diag.Message("", "failed to close converter plugin: %v"), err)
		}
		err = grpcServer.Close()
		if err != nil {
			// Again just warn
			pCtx.Diag.Warningf(diag.Message("", "failed to close mapping server: %v"), err)
		}

		// These diagnostics come directly from the converter and so _should_ be user friendly. So we're just
		// going to print them.
		printDiagnostics(pCtx.Diag, resp.Diagnostics)
		if resp.Diagnostics.HasErrors() {
			// If we've got error diagnostics then program generation failed, we've printed the error above so
			// just return a plain message here.
			return errors.New("conversion failed")
		}
	}

	// Load the project from the pcl directory if there is one. We default to a project with just
	// the name of the original directory.
	proj := &workspace.Project{Name: tokens.PackageName(filepath.Base(cwd))}
	path, _ := workspace.DetectProjectPathFrom(pclDirectory)
	if path != "" {
		proj, err = workspace.LoadProject(path)
		if err != nil {
			return fmt.Errorf("load project: %w", err)
		}
	}

	pCtx.Diag.Infof(diag.Message("", "Converting to %s..."), language)
	diagnostics, err := projectGenerator(pclDirectory, outDir, proj, loader, strict)
	// If we have error diagnostics then program generation failed, print an error to the user that they
	// should raise an issue about this
	if diagnostics.HasErrors() {
		// Don't print the notice about this being a bug if we're in strict mode
		if !strict {
			fmt.Fprintln(os.Stderr, "================================================================================")
			fmt.Fprintln(os.Stderr, "The Pulumi CLI encountered a code generation error. This is a bug!")
			fmt.Fprintln(os.Stderr, "We would appreciate a report: https://github.com/pulumi/pulumi/issues/")
			fmt.Fprintln(os.Stderr, "Please provide all of the below text in your report.")
			fmt.Fprintln(os.Stderr, "================================================================================")
			fmt.Fprintf(os.Stderr, "Pulumi Version:   %s\n", version.Version)
		}
		printDiagnostics(pCtx.Diag, diagnostics)
		if err != nil {
			return fmt.Errorf("could not generate output program: %w", err)
		}

		return errors.New("could not generate output program")
	}

	if err != nil {
		return fmt.Errorf("could not generate output program: %w", err)
	}

	// If we've got code generation warnings only print them if we've got PULUMI_DEV set or emitting pcl
	if e.GetBool(env.Dev) || language == "pcl" {
		printDiagnostics(pCtx.Diag, diagnostics)
	}

	// Project should now exist at outDir. Run installDependencies in that directory (if requested)
	if !generateOnly {
		// Change the working directory to the specified directory.
		if err := os.Chdir(outDir); err != nil {
			return fmt.Errorf("changing the working directory: %w", err)
		}

		proj, root, err := readProject()
		if err != nil {
			return err
		}

		projinfo := &engine.Projinfo{Proj: proj, Root: root}
		_, main, ctx, err := engine.ProjectInfoContext(projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil, nil)
		if err != nil {
			return err
		}
		defer ctx.Close()

		if err := installDependencies(ctx, &proj.Runtime, main); err != nil {
			return err
		}
	}

	return nil
}

func genCreationExampleSyntax(r *schema.Resource) string {
	indentSize := 0
	buffer := bytes.Buffer{}
	write := func(format string, args ...interface{}) {
		buffer.WriteString(fmt.Sprintf(format, args...))
	}

	indent := func() {
		buffer.WriteString(strings.Repeat(" ", indentSize))
	}

	indended := func(f func()) {
		indentSize += 2
		f()
		indentSize -= 2
	}

	seenTypes := codegen.NewStringSet()
	var writeValue func(valueType schema.Type)
	writeValue = func(valueType schema.Type) {
		switch valueType {
		case schema.BoolType:
			write("false")
		case schema.IntType:
			write("0")
		case schema.NumberType:
			write("0.0")
		case schema.StringType:
			write("\"string\"")
		case schema.ArchiveType:
			write("fileArchive(\"./path/to/archive\")")
		case schema.AssetType:
			write("stringAsset(\"content\")")
		}

		switch valueType := valueType.(type) {
		case *schema.ArrayType:
			write("[")
			writeValue(valueType.ElementType)
			write("]")
		case *schema.MapType:
			write("{\n")
			indended(func() {
				indent()
				write("\"string\" = ")
				writeValue(valueType.ElementType)
				write("\n")
			})
			indent()
			write("}")
		case *schema.ObjectType:
			if seenTypes.Has(valueType.Token) && objectTypeHasRecursiveReference(valueType) {
				write("notImplemented(%q)", valueType.Token)
				return
			}

			seenTypes.Add(valueType.Token)
			write("{\n")
			indended(func() {
				for _, p := range valueType.Properties {
					indent()
					write("%s = ", p.Name)
					writeValue(p.Type)
					write("\n")
				}
			})
			indent()
			write("}")
		case *schema.ResourceType:
			write("notImplemented(%q)", valueType.Token)
		case *schema.EnumType:
			cases := make([]string, len(valueType.Elements))
			for index, c := range valueType.Elements {
				if stringCase, ok := c.Value.(string); ok && stringCase != "" {
					cases[index] = stringCase
				} else if intCase, ok := c.Value.(int); ok {
					cases[index] = strconv.Itoa(intCase)
				} else {
					if c.Name != "" {
						cases[index] = c.Name
					}
				}
			}

			write(fmt.Sprintf("%q", strings.Join(cases, "|")))
		case *schema.UnionType:
			if isUnionOfObjects(valueType) {
				possibleTypes := make([]string, len(valueType.ElementTypes))
				for index, elem := range valueType.ElementTypes {
					objectType := elem.(*schema.ObjectType)
					_, _, typeName := decomposeToken(objectType.Token)
					possibleTypes[index] = typeName
				}
				write("notImplemented(\"" + strings.Join(possibleTypes, "|") + "\")")
			}

			for _, elem := range valueType.ElementTypes {
				if isPrimitiveType(elem) {
					writeValue(elem)
					return
				}
			}
		case *schema.InputType:
			writeValue(valueType.ElementType)
		case *schema.OptionalType:
			writeValue(valueType.ElementType)
		case *schema.TokenType:
			writeValue(valueType.UnderlyingType)
		}
	}

	write("resource \"example\" %q {\n", r.Token)
	indended(func() {
		for _, p := range r.InputProperties {
			indent()
			write("%s = ", p.Name)
			writeValue(codegen.ResolvedType(p.Type))
			write("\n")
		}
	})

	write("}")
	return buffer.String()
}

func isPrimitiveType(t schema.Type) bool {
	switch t {
	case schema.BoolType, schema.IntType, schema.NumberType, schema.StringType:
		return true
	default:
		switch argType := t.(type) {
		case *schema.OptionalType:
			return isPrimitiveType(argType.ElementType)
		case *schema.EnumType, *schema.ResourceType:
			return true
		}
		return false
	}
}

func isUnionOfObjects(schemaType *schema.UnionType) bool {
	for _, elementType := range schemaType.ElementTypes {
		if _, isObjectType := elementType.(*schema.ObjectType); !isObjectType {
			return false
		}
	}

	return true
}

func objectTypeHasRecursiveReference(objectType *schema.ObjectType) bool {
	isRecursive := false
	codegen.VisitTypeClosure(objectType.Properties, func(t schema.Type) {
		if objectRef, ok := t.(*schema.ObjectType); ok {
			if objectRef.Token == objectType.Token {
				isRecursive = true
			}
		}
	})

	return isRecursive
}

func decomposeToken(token string) (string, string, string) {
	pkg, mod, member, _ := pcl.DecomposeToken(token, hcl.Range{})
	return pkg, mod, member
}
