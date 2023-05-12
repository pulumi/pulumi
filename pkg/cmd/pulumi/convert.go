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

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"
	tfgen "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/convert"
	yamlgen "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	aferoUtil "github.com/pulumi/pulumi/pkg/v3/util/afero"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

type projectGeneratorFunc func(directory string, project workspace.Project, p *pcl.Program) error

func newConvertCmd() *cobra.Command {
	var outDir string
	var from string
	var language string
	var generateOnly bool
	var mappings []string

	cmd := &cobra.Command{
		Use:   "convert",
		Args:  cmdutil.MaximumNArgs(0),
		Short: "Convert Pulumi programs from a supported source program into other supported languages",
		Long: "Convert Pulumi programs from a supported source program into other supported languages.\n" +
			"\n" +
			"The source program to convert will default to the current working directory.\n",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			cwd, err := os.Getwd()
			if err != nil {
				return result.FromError(fmt.Errorf("get current working directory: %w", err))
			}

			return runConvert(env.Global(), cwd, mappings, from, language, outDir, generateOnly)
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

// Same pclBindProgram but recovers from panics
func safePclBindProgram(sourceDirectory string, loader schema.ReferenceLoader,
) (program *pcl.Program, diagnostics hcl.Diagnostics, err error) {
	// PCL binding can be quite panic'y but even it panics we want to write out the intermediate PCL generated
	// from the converter, so we use a recover statement here.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic binding program: %v", r)
		}
	}()
	program, diagnostics, err = pclBindProgram(sourceDirectory, loader)
	return
}

// pclGenerateProject writes out a pcl.Program directly as .pp files
func pclGenerateProject(sink diag.Sink, sourceDirectory, targetDirectory string, loader schema.ReferenceLoader) error {
	program, diagnostics, err := safePclBindProgram(sourceDirectory, loader)
	printDiagnostics(sink, diagnostics)
	if program != nil {
		// If we successfully bound the program, write out the .pp files from it
		// We don't write out a Pulumi.yaml for PCL, just the .pp files.
		return writeProgram(targetDirectory, nil, program)
	}
	// We couldn't bind the program so print that for the user to see but then just copy the filetree across
	if err != nil {
		logging.Warningf("failed to bind program: %v", err)
	} else {
		// We've already printed the diagnostics above
		logging.Warningf("failed to bind program")
	}

	// Copy the source directory to the target directory
	return aferoUtil.CopyDir(afero.NewOsFs(), sourceDirectory, targetDirectory)
}

// pclBindProgram binds a PCL in the given directory.
func pclBindProgram(directory string, loader schema.ReferenceLoader) (*pcl.Program, hcl.Diagnostics, error) {
	parser := hclsyntax.NewParser()
	// Load all .pp files in the directory
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, nil, err
	}

	parseDiagnostics := make(hcl.Diagnostics, 0)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		path := filepath.Join(directory, fileName)

		if filepath.Ext(path) == ".pp" {
			file, err := os.Open(path)
			if err != nil {
				return nil, nil, err
			}

			err = parser.ParseFile(file, filepath.Base(path))
			if err != nil {
				return nil, nil, err
			}
			parseDiagnostics = append(parseDiagnostics, parser.Diagnostics...)
		}
	}

	if parseDiagnostics.HasErrors() {
		return nil, parseDiagnostics, nil
	}

	program, bindDiagnostics, err := pcl.BindProgram(parser.Files,
		pcl.Loader(loader),
		pcl.DirPath(directory),
		pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))

	// err will be the same as bindDiagnostics if there are errors, but we don't want to return that here.
	// err _could_ also be a context setup error in which case bindDiagnotics will be nil and that we do want to return.
	if bindDiagnostics != nil {
		err = nil
	}

	allDiagnostics := append(parseDiagnostics, bindDiagnostics...)
	return program, allDiagnostics, err
}

func runConvert(
	e env.Env,
	cwd string, mappings []string, from string, language string,
	outDir string, generateOnly bool,
) result.Result {
	wrapper := func(generator projectGeneratorFunc) func(diag.Sink, string, string, schema.ReferenceLoader) error {
		return func(sink diag.Sink, sourceDirectory, targetDirectory string, loader schema.ReferenceLoader) error {
			program, diagnostics, err := pclBindProgram(sourceDirectory, loader)
			printDiagnostics(sink, diagnostics)
			if err != nil {
				return fmt.Errorf("failed to bind program: %w", err)
			} else if program == nil {
				// We've already printed the diagnostics above
				return fmt.Errorf("failed to bind program")
			}

			// Load the project from the target directory if there is one. We default to a project with just
			// the name of the original directory.
			proj := &workspace.Project{Name: tokens.PackageName(filepath.Base(cwd))}
			path, _ := workspace.DetectProjectPathFrom(sourceDirectory)
			if path != "" {
				proj, err = workspace.LoadProject(path)
				if err != nil {
					return fmt.Errorf("load project: %w", err)
				}
			}

			return generator(targetDirectory, *proj, program)
		}
	}

	pCtx, err := newPluginContext(cwd)
	if err != nil {
		return result.FromError(fmt.Errorf("create plugin host: %w", err))
	}
	defer contract.IgnoreClose(pCtx.Host)

	// Translate well known languages to runtimes
	switch language {
	case "csharp", "c#":
		language = "dotnet"
	case "typescript":
		language = "nodejs"
	}

	var projectGenerator func(diag.Sink, string, string, schema.ReferenceLoader) error
	switch language {
	case "dotnet":
		projectGenerator = wrapper(dotnet.GenerateProject)
	case "nodejs":
		projectGenerator = wrapper(nodejs.GenerateProject)
	case "python":
		projectGenerator = wrapper(python.GenerateProject)
	case "java":
		projectGenerator = wrapper(javagen.GenerateProject)
	case "yaml":
		projectGenerator = wrapper(yamlgen.GenerateProject)
	case "pulumi", "pcl":
		if e.GetBool(env.Dev) {
			// No plugin for PCL to install dependencies with
			generateOnly = true
			projectGenerator = pclGenerateProject
			break
		}
		return result.Errorf("cannot generate programs for %q language", language)
	default:
		projectGenerator = func(
			sink diag.Sink,
			sourceDirectory, targetDirectory string,
			loader schema.ReferenceLoader,
		) error {
			program, diagnostics, err := pclBindProgram(sourceDirectory, loader)
			printDiagnostics(sink, diagnostics)
			if err != nil {
				return fmt.Errorf("failed to bind program: %w", err)
			} else if program == nil {
				// We've already printed the diagnostics above
				return fmt.Errorf("failed to bind program")
			}

			// Load the project from the target directory if there is one. We default to a project with just
			// the name of the original directory.
			proj := &workspace.Project{Name: tokens.PackageName(filepath.Base(cwd))}
			path, _ := workspace.DetectProjectPathFrom(sourceDirectory)
			if path != "" {
				proj, err = workspace.LoadProject(path)
				if err != nil {
					return fmt.Errorf("load project: %w", err)
				}
			}

			languagePlugin, err := pCtx.Host.LanguageRuntime(cwd, cwd, language, nil)
			if err != nil {
				return err
			}

			projectBytes, err := encoding.JSON.Marshal(proj)
			if err != nil {
				return err
			}
			projectJSON := string(projectBytes)

			// It feels a bit redundant to parse and bind the program just to turn it back into text, but it
			// means we get binding errors this side of the grpc boundary, and we're likely to do something
			// fancier here at some point (like passing an annotated syntax tree via protobuf rather than just
			// PCL text).
			err = languagePlugin.GenerateProject(targetDirectory, projectJSON, program.Source())
			if err != nil {
				return err
			}

			return nil
		}
	}

	if outDir != "." {
		err := os.MkdirAll(outDir, 0o755)
		if err != nil {
			return result.FromError(fmt.Errorf("create output directory: %w", err))
		}
	}

	loader := schema.NewPluginLoader(pCtx.Host)
	mapper, err := convert.NewPluginMapper(
		convert.DefaultWorkspace(), convert.ProviderFactoryFromHost(pCtx.Host),
		from, mappings)
	if err != nil {
		return result.FromError(fmt.Errorf("create provider mapper: %w", err))
	}

	pclDirectory, err := os.MkdirTemp("", "pulumi-convert")
	if err != nil {
		return result.FromError(fmt.Errorf("create temporary directory: %w", err))
	}

	if from == "" || from == "yaml" {
		proj, program, err := yamlgen.Eject(cwd, loader)
		if err != nil {
			return result.FromError(fmt.Errorf("load yaml program: %w", err))
		}
		err = writeProgram(pclDirectory, proj, program)
		if err != nil {
			return result.FromError(fmt.Errorf("write program to intermediate directory: %w", err))
		}
	} else if from == "pcl" {
		if e.GetBool(env.Dev) {
			// The source code is PCL, we don't need to do anything here, just repoint pclDirectory to it
			pclDirectory = cwd
		} else {
			return result.FromError(fmt.Errorf("unrecognized source %q", from))
		}
	} else if from == "tf" {
		proj, program, err := tfgen.Eject(cwd, loader, mapper)
		if err != nil {
			return result.FromError(fmt.Errorf("load terraform program: %w", err))
		}
		err = writeProgram(pclDirectory, proj, program)
		if err != nil {
			return result.FromError(fmt.Errorf("write program to intermediate directory: %w", err))
		}
	} else {
		// Try and load the converter plugin for this
		converter, err := plugin.NewConverter(pCtx, from, nil)
		if err != nil {
			// If NewConverter returns a MissingError, we can try and install the plugin and try again.
			var me *workspace.MissingError
			if !errors.As(err, &me) {
				// Not a MissingError, return the original error.
				return result.FromError(fmt.Errorf("load plugin source %q: %w", from, err))
			}

			pluginSpec := workspace.PluginSpec{
				Kind: workspace.ConverterPlugin,
				Name: from,
			}

			log := func(sev diag.Severity, msg string) {
				pCtx.Diag.Logf(sev, diag.RawMessage("", msg))
			}

			err = pkgWorkspace.InstallPlugin(pluginSpec, log)
			if err != nil {
				return result.FromError(fmt.Errorf("install plugin source %q: %w", from, err))
			}

			converter, err = plugin.NewConverter(pCtx, from, nil)
			if err != nil {
				return result.FromError(fmt.Errorf("load plugin source %q: %w", from, err))
			}
		}
		defer contract.IgnoreClose(converter)

		pCtx.Diag.Warningf(diag.RawMessage("", "Plugin converters are currently experimental"))

		mapperServer := convert.NewMapperServer(mapper)
		grpcServer, err := plugin.NewServer(pCtx, convert.MapperRegistration(mapperServer))
		if err != nil {
			return result.FromError(err)
		}

		_, err = converter.ConvertProgram(pCtx.Request(), &plugin.ConvertProgramRequest{
			SourceDirectory: cwd,
			TargetDirectory: pclDirectory,
			MapperAddress:   grpcServer.Addr(),
		})
		if err != nil {
			return result.FromError(err)
		}
	}

	err = projectGenerator(pCtx.Diag, pclDirectory, outDir, loader)
	if err != nil {
		return result.FromError(fmt.Errorf("could not generate output program: %w", err))
	}

	// Project should now exist at outDir. Run installDependencies in that directory (if requested)
	if !generateOnly {
		// Change the working directory to the specified directory.
		if err := os.Chdir(outDir); err != nil {
			return result.FromError(fmt.Errorf("changing the working directory: %w", err))
		}

		proj, root, err := readProject()
		if err != nil {
			return result.FromError(err)
		}

		projinfo := &engine.Projinfo{Proj: proj, Root: root}
		pwd, _, ctx, err := engine.ProjectInfoContext(projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil, nil)
		if err != nil {
			return result.FromError(err)
		}
		defer ctx.Close()

		if err := installDependencies(ctx, &proj.Runtime, pwd); err != nil {
			return result.FromError(err)
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
