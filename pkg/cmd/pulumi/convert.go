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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/afero"
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

	aferoUtil "github.com/pulumi/pulumi/pkg/v3/util/afero"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

type projectGeneratorFunc func(directory string, project workspace.Project, p *pcl.Program) error

func loadConverterPlugin(
	ctx *plugin.Context,
	name string,
	log func(sev diag.Severity, msg string),
) (plugin.Converter, error) {
	// Try and load the converter plugin for this
	converter, err := plugin.NewConverter(ctx, name, nil)
	if err != nil {
		// If NewConverter returns a MissingError, we can try and install the plugin if it was missing and try again,
		// unless auto plugin installs are turned off.
		if env.DisableAutomaticPluginAcquisition.Value() {
			return nil, fmt.Errorf("load %q: %w", name, err)
		}

		var me *workspace.MissingError
		if !errors.As(err, &me) {
			// Not a MissingError, return the original error.
			return nil, fmt.Errorf("load %q: %w", name, err)
		}

		pluginSpec := workspace.PluginSpec{
			Kind: workspace.ConverterPlugin,
			Name: name,
		}

		_, err = pkgWorkspace.InstallPlugin(pluginSpec, log)
		if err != nil {
			return nil, fmt.Errorf("install %q: %w", name, err)
		}

		converter, err = plugin.NewConverter(ctx, name, nil)
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", name, err)
		}
	}
	return converter, nil
}

func newConvertCmd() *cobra.Command {
	var outDir string
	var from string
	var language string
	var generateOnly bool
	var mappings []string
	var strict bool

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert Pulumi programs from a supported source program into other supported languages",
		Long: "Convert Pulumi programs from a supported source program into other supported languages.\n" +
			"\n" +
			"The source program to convert will default to the current working directory.\n" +
			"\n" +
			"Valid source languages: yaml, terraform, bicep, arm\n" +
			"\n" +
			"Valid target languages: typescript, python, csharp, go, java, yaml" +
			"\n" +
			"Example command usage:" +
			"\n" +
			"    pulumi convert --from yaml --language java --out . \n",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current working directory: %w", err)
			}

			return runConvert(env.Global(), args, cwd, mappings, from, language, outDir, generateOnly, strict)
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
		&from, "from", "yaml", "Which converter plugin to use to read the source program")

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

// prints the diagnostics to the diagnostic sink
func printDiagnostics(sink diag.Sink, diagnostics hcl.Diagnostics) {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == hcl.DiagError {
			sink.Errorf(diag.Message("", "%s"), diagnostic)
		} else {
			sink.Warningf(diag.Message("", "%s"), diagnostic)
		}
	}
}

// writeProgram writes a project and pcl program to the given directory
func writeProgram(directory string, proj *workspace.Project, program *pcl.Program) error {
	fs := afero.NewOsFs()
	err := program.WriteSource(afero.NewBasePathFs(fs, directory))
	if err != nil {
		return fmt.Errorf("writing program: %w", err)
	}

	// Write out the Pulumi.yaml file if we've got one
	if proj != nil {
		projBytes, err := encoding.YAML.Marshal(proj)
		if err != nil {
			return fmt.Errorf("marshaling project: %w", err)
		}

		err = afero.WriteFile(fs, filepath.Join(directory, "Pulumi.yaml"), projBytes, 0o644)
		if err != nil {
			return fmt.Errorf("writing project: %w", err)
		}
	}

	return nil
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
			return diagnostics, fmt.Errorf("failed to bind program")
		}
		return diagnostics, generator(targetDirectory, *proj, program)
	}
}

func runConvert(
	e env.Env,
	args []string,
	cwd string, mappings []string, from string, language string,
	outDir string, generateOnly bool, strict bool,
) error {
	pCtx, err := newPluginContext(cwd)
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

			languagePlugin, err := pCtx.Host.LanguageRuntime(cwd, cwd, language, nil)
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
	if from == "yaml" {
		proj, program, err := yamlgen.Eject(cwd, loader)
		if err != nil {
			return fmt.Errorf("load yaml program: %w", err)
		}
		err = writeProgram(pclDirectory, proj, program)
		if err != nil {
			return fmt.Errorf("write program to intermediate directory: %w", err)
		}
	} else if from == "pcl" {
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
			return fmt.Errorf("conversion failed")
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

		return fmt.Errorf("could not generate output program")
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
		pwd, _, ctx, err := engine.ProjectInfoContext(projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil, nil)
		if err != nil {
			return err
		}
		defer ctx.Close()

		if err := installDependencies(ctx, &proj.Runtime, pwd); err != nil {
			return err
		}
	}

	return nil
}

func newPluginContext(cwd string) (*plugin.Context, error) {
	sink := diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: cmdutil.GetGlobalColorization(),
	})
	pluginCtx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, true, nil)
	if err != nil {
		return nil, err
	}
	return pluginCtx, nil
}
