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
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"
	tfgen "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tf2pulumi/convert"
	yamlgen "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	aferoUtil "github.com/pulumi/pulumi/pkg/v3/util/afero"
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

// pclGenerateProject writes out a pcl.Program directly as .pp files
func pclGenerateProject(sourceDirectory, targetDirectory string, loader schema.ReferenceLoader) error {
	program, err := pclBindProgram(sourceDirectory, loader)
	if err == nil {
		// If we successfully bound the program, write out the .pp files from it
		// We don't write out a Pulumi.yaml for PCL, just the .pp files.
		return writeProgram(targetDirectory, nil, program)
	}
	// We couldn't bind the program so print that for the user to see but then just copy the filetree across
	fmt.Printf("Could not bind program: %v", err)

	// Copy the source directory to the target directory
	return aferoUtil.CopyDir(afero.NewOsFs(), sourceDirectory, targetDirectory)
}

// pclBindProgram binds a PCL in the given directory.
func pclBindProgram(directory string, loader schema.ReferenceLoader) (*pcl.Program, error) {
	parser := hclsyntax.NewParser()
	// Load all .pp files in the directory
	files, err := os.ReadDir(directory)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		path := filepath.Join(directory, fileName)

		if filepath.Ext(path) == ".pp" {
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}

			err = parser.ParseFile(file, filepath.Base(path))
			if err != nil {
				return nil, err
			}
			diags := parser.Diagnostics
			if diags.HasErrors() {
				return nil, diags
			}
		}
	}

	if err != nil {
		return nil, err
	}

	program, pdiags, err := pcl.BindProgram(parser.Files,
		pcl.Loader(loader),
		pcl.DirPath(directory),
		pcl.ComponentBinder(pcl.ComponentProgramBinderFromFileSystem()))
	if err != nil {
		return nil, err
	}
	if pdiags.HasErrors() || program == nil {
		return nil, fmt.Errorf("internal error: %w", pdiags)
	}

	return program, nil
}

func runConvert(
	e env.Env,
	cwd string, mappings []string, from string, language string,
	outDir string, generateOnly bool,
) result.Result {
	wrapper := func(generator projectGeneratorFunc) func(string, string, schema.ReferenceLoader) error {
		return func(sourceDirectory, targetDirectory string, loader schema.ReferenceLoader) error {
			program, err := pclBindProgram(sourceDirectory, loader)
			if err != nil {
				return fmt.Errorf("load pcl program: %w", err)
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

	var projectGenerator func(string, string, schema.ReferenceLoader) error
	switch language {
	case "csharp", "c#":
		projectGenerator = wrapper(dotnet.GenerateProject)
	case "go":
		projectGenerator = wrapper(gogen.GenerateProject)
	case "typescript":
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
		fallthrough

	default:
		return result.Errorf("cannot generate programs for %q language", language)
	}

	if outDir != "." {
		err := os.MkdirAll(outDir, 0o755)
		if err != nil {
			return result.FromError(fmt.Errorf("create output directory: %w", err))
		}
	}

	pCtx, err := newPluginContext(cwd)
	if err != nil {
		return result.FromError(fmt.Errorf("create plugin host: %w", err))
	}
	defer contract.IgnoreClose(pCtx.Host)
	loader := schema.NewPluginLoader(pCtx.Host)
	mapper, err := convert.NewPluginMapper(pCtx.Host, from, mappings)
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
			return result.FromError(fmt.Errorf("plugin source %q: %w", from, err))
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

	err = projectGenerator(pclDirectory, outDir, loader)
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
