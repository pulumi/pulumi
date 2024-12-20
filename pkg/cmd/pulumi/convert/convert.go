// Copyright 2016-2024, Pulumi Corporation.
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

package convert

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	cmdDiag "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/diag"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/newcmd"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packagecmd"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	aferoUtil "github.com/pulumi/pulumi/pkg/v3/util/afero"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

type projectGeneratorFunc func(directory string, project workspace.Project, p *pcl.Program) error

func NewConvertCmd() *cobra.Command {
	var outDir string
	var from string
	var language string
	var generateOnly bool
	var mappings []string
	var strict bool
	var name string

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert Pulumi programs from a supported source program into other supported languages",
		Long: "Convert Pulumi programs from a supported source program into other supported languages.\n" +
			"\n" +
			"The source program to convert will default to the current working directory.\n" +
			"\n" +
			"Valid source languages: yaml, terraform, bicep, arm, kubernetes\n" +
			"\n" +
			"Valid target languages: typescript, python, csharp, go, java, yaml" +
			"\n" +
			"Example command usage:" +
			"\n" +
			"    pulumi convert --from yaml --language java --out . \n",
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current working directory: %w", err)
			}

			return runConvert(
				cmd.Context(),
				pkgWorkspace.Instance,
				env.Global(),
				args,
				cwd,
				mappings,
				from,
				language,
				outDir,
				generateOnly,
				strict,
				name,
			)
		}),
	}

	cmd.PersistentFlags().StringVar(
		&language, "language", "", "Which language plugin to use to generate the Pulumi project")
	if err := cmd.MarkPersistentFlagRequired("language"); err != nil {
		panic("failed to mark 'language' as a required flag")
	}

	cmd.PersistentFlags().StringVar(
		&from, "from", "yaml", "Which converter plugin to use to read the source program")

	cmd.PersistentFlags().StringVar(
		&outDir, "out", ".", "The output directory to write the converted project to")

	cmd.PersistentFlags().BoolVar(
		&generateOnly, "generate-only", false, "Generate the converted program(s) only; do not install dependencies")

	cmd.PersistentFlags().StringSliceVar(
		&mappings, "mappings", []string{}, "Any mapping files to use in the conversion")

	cmd.PersistentFlags().BoolVar(
		&strict, "strict", false, "Fail the conversion on errors such as missing variables")

	cmd.PersistentFlags().StringVar(
		&name, "name", "", "The name to use for the converted project; defaults to the directory of the source project")

	return cmd
}

// Same pcl.BindDirectory but recovers from panics
func safePclBindDirectory(sourceDirectory string, loader schema.ReferenceLoader, strict bool,
) (program *pcl.Program, diagnostics hcl.Diagnostics, err error) {
	// PCL binding can be quite panic'y but even it panics we want to write out the intermediate PCL generated
	// from the converter, so we use a recover statement here.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic binding program: %v", r)
		}
	}()

	extraOptions := make([]pcl.BindOption, 0)
	if !strict {
		extraOptions = append(extraOptions, pcl.NonStrictBindOptions()...)
	}

	program, diagnostics, err = pcl.BindDirectory(sourceDirectory, loader, extraOptions...)
	return
}

// pclGenerateProject writes out a pcl.Program directly as .pp files
func pclGenerateProject(
	sourceDirectory, targetDirectory string, proj *workspace.Project, loader schema.ReferenceLoader, strict bool,
) (hcl.Diagnostics, error) {
	_, diagnostics, bindErr := safePclBindDirectory(sourceDirectory, loader, strict)
	// We always try to copy the source directory to the target directory even if binding failed
	copyErr := aferoUtil.CopyDir(afero.NewOsFs(), sourceDirectory, targetDirectory)
	// And then we return the combined diagnostics and errors
	var err error
	if bindErr != nil || copyErr != nil {
		err = multierror.Append(bindErr, copyErr)
	}
	return diagnostics, err
}

type projectGeneratorFunction func(
	string, string, *workspace.Project, schema.ReferenceLoader, bool,
) (hcl.Diagnostics, error)

func generatorWrapper(generator projectGeneratorFunc, targetLanguage string) projectGeneratorFunction {
	return func(
		sourceDirectory, targetDirectory string, proj *workspace.Project, loader schema.ReferenceLoader, strict bool,
	) (hcl.Diagnostics, error) {
		contract.Requiref(proj != nil, "proj", "must not be nil")

		extraOptions := make([]pcl.BindOption, 0)
		if !strict {
			extraOptions = append(extraOptions, pcl.NonStrictBindOptions()...)
		}

		program, diagnostics, err := pcl.BindDirectory(sourceDirectory, loader, extraOptions...)
		if err != nil {
			return diagnostics, fmt.Errorf("failed to bind program: %w", err)
		} else if program == nil {
			// We've already printed the diagnostics above
			return diagnostics, errors.New("failed to bind program")
		}
		return diagnostics, generator(targetDirectory, *proj, program)
	}
}

func runConvert(
	ctx context.Context,
	ws pkgWorkspace.Context,
	e env.Env,
	args []string,
	cwd string,
	mappings []string,
	from string,
	language string,
	outDir string,
	generateOnly bool,
	strict bool,
	name string,
) error {
	// Validate the supplied name if one was specified. If one was not supplied,
	// default to the directory of the source project.
	if name != "" {
		err := pkgWorkspace.ValidateProjectName(name)
		if err != nil {
			return fmt.Errorf("'%s' is not a valid project name: %w", name, err)
		}
	} else {
		name = filepath.Base(cwd)
	}

	pCtx, err := packagecmd.NewPluginContext(cwd)
	if err != nil {
		return fmt.Errorf("create plugin host: %w", err)
	}
	defer contract.IgnoreClose(pCtx.Host)

	// Translate well known sources to plugins
	switch from {
	case "tf":
		from = "terraform"
	case "":
		from = "yaml"
	}

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
		projectGenerator = generatorWrapper(
			func(targetDirectory string, proj workspace.Project, program *pcl.Program) error {
				return javagen.GenerateProject(targetDirectory, proj, program, nil /*localDependencies*/)
			}, language)
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

			programInfo := plugin.NewProgramInfo(cwd, cwd, ".", nil)
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

			var diags hcl.Diagnostics
			ds, err := languagePlugin.GenerateProject(
				sourceDirectory, targetDirectory, projectJSON,
				strict, grpcServer.Addr(), nil /*localDependencies*/)
			diags = append(diags, ds...)
			if err != nil {
				return nil, err
			}

			packageBlockDescriptors, ds, err := getPackagesToGenerateSdks(sourceDirectory)
			diags = append(diags, ds...)
			if err != nil {
				return diags, fmt.Errorf("error parsing pcl: %w", err)
			}

			err = generateAndLinkSdksForPackages(
				ctx,
				ws,
				language,
				filepath.Join(targetDirectory, "sdks"),
				targetDirectory,
				packageBlockDescriptors,
			)
			if err != nil {
				return diags, fmt.Errorf("error generating packages: %w", err)
			}

			return diags, nil
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
			Kind: apitype.ResourcePlugin,
		}
		version, err := pkgWorkspace.InstallPlugin(pCtx.Base(), pluginSpec, log)
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
		converter, err := LoadConverterPlugin(pCtx, from, log)
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
		cmdDiag.PrintDiagnostics(pCtx.Diag, resp.Diagnostics)
		if resp.Diagnostics.HasErrors() {
			// If we've got error diagnostics then program generation failed, we've printed the error above so
			// just return a plain message here.
			return errors.New("conversion failed")
		}
	}

	// Load the project from the pcl directory if there is one. We default to a project with just
	// the name of the original directory.
	proj := &workspace.Project{Name: tokens.PackageName(name)}
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
		cmdDiag.PrintDiagnostics(pCtx.Diag, diagnostics)
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
		cmdDiag.PrintDiagnostics(pCtx.Diag, diagnostics)
	}

	// Project should now exist at outDir. Run installDependencies in that directory (if requested)
	if !generateOnly {
		// Change the working directory to the specified directory.
		if err := os.Chdir(outDir); err != nil {
			return fmt.Errorf("changing the working directory: %w", err)
		}

		proj, root, err := ws.ReadProject()
		if err != nil {
			return err
		}

		projinfo := &engine.Projinfo{Proj: proj, Root: root}
		_, main, ctx, err := engine.ProjectInfoContext(projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), nil, false, nil, nil)
		if err != nil {
			return err
		}
		defer ctx.Close()

		if err := newcmd.InstallDependencies(ctx, &proj.Runtime, main); err != nil {
			return err
		}
	}

	return nil
}

// getPackagesToGenerateSdks parses the pcl files back in to read the package
// blocks and for sdk generation.
func getPackagesToGenerateSdks(
	sourceDirectory string,
) (map[string]*schema.PackageDescriptor, hcl.Diagnostics, error) {
	files, err := os.ReadDir(sourceDirectory)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read source directory %s: %w", sourceDirectory, err)
	}

	parser := hclsyntax.NewParser()
	_, err = pcl.ParseFiles(parser, sourceDirectory, files)
	if err != nil {
		return nil, nil, fmt.Errorf("could not parse PCL files: %w", err)
	}

	allPackageDescriptors := make(map[string]*schema.PackageDescriptor)

	var diagnostics hcl.Diagnostics
	for _, file := range parser.Files {
		packageDescriptors, diags := pcl.ReadPackageDescriptors(file)
		diagnostics = append(diagnostics, diags...)
		for packageName, descriptor := range packageDescriptors {
			if _, ok := allPackageDescriptors[packageName]; ok {
				message := fmt.Sprintf("package %q was already defined", packageName)
				subjectRange := file.Body.Range()
				diagnostics = append(diagnostics, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  message,
					Detail:   message,
					Subject:  &subjectRange,
				})
				continue
			}
			allPackageDescriptors[packageName] = descriptor
		}
	}

	if len(diagnostics) != 0 {
		var errorDiags hcl.Diagnostics
		for _, d := range diagnostics {
			if d.Severity == hcl.DiagError {
				errorDiags = append(errorDiags, d)
			}
		}

		if len(errorDiags) != 0 {
			return nil, diagnostics, nil
		}
	}

	return allPackageDescriptors, diagnostics, nil
}

func generateAndLinkSdksForPackages(
	ctx context.Context,
	ws pkgWorkspace.Context,
	language string,
	sdkTargetDirectory string,
	convertOutputDirectory string,
	pkgs map[string]*schema.PackageDescriptor,
) error {
	for _, pkg := range pkgs {
		tempOut, err := os.MkdirTemp("", "gen-sdk-for-dependency-")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}

		if pkg.Parameterization == nil {
			// Only generate SDKs for packages that have parameterization for now, others should be implicit.
			continue
		}

		pkgSchema, err := packagecmd.SchemaFromSchemaSourceValueArgs(
			ctx,
			pkg.Name,
			pkg.Parameterization.Value,
		)
		if err != nil {
			return fmt.Errorf("creating package schema: %w", err)
		}

		err = packagecmd.GenSDK(
			language,
			tempOut,
			pkgSchema,
			/*overlays*/ "",
			/*local*/ true,
		)
		if err != nil {
			return fmt.Errorf("error generating sdk: %w", err)
		}

		sdkOut := filepath.Join(sdkTargetDirectory, pkg.Parameterization.Name)
		err = packagecmd.CopyAll(sdkOut, filepath.Join(tempOut, language))
		if err != nil {
			return fmt.Errorf("failed to move SDK to project: %w", err)
		}

		err = os.RemoveAll(tempOut)
		if err != nil {
			return fmt.Errorf("could not remove temp dir: %w", err)
		}

		fmt.Printf("Generated local SDK for package '%s:%s'\n", pkg.Name, pkg.Parameterization.Name)

		// If we don't change the working directory, the workspace instance (when
		// reading project etc) will not be correct when doing the local sdk
		// linking, causing errors.
		returnToStartingDir, err := fsutil.Chdir(convertOutputDirectory)
		if err != nil {
			return fmt.Errorf("could not change to output directory: %w", err)
		}
		defer returnToStartingDir()

		_, _, err = ws.ReadProject()
		if err != nil {
			return fmt.Errorf("generated root is not a valid pulumi workspace %q: %w", convertOutputDirectory, err)
		}

		sdkRelPath := filepath.Join("sdks", pkg.Parameterization.Name)
		err = packagecmd.LinkPackage(ws, language, "./", pkgSchema, sdkRelPath)
		if err != nil {
			return fmt.Errorf("failed to link SDK to project: %w", err)
		}
	}

	return nil
}
