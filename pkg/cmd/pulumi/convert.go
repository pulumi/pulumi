// Copyright 2016-2022, Pulumi Corporation.
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
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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
				return result.FromError(fmt.Errorf("could not resolve current working directory"))
			}

			return runConvert(cwd, mappings, from, language, outDir, generateOnly)
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

// pclGenerateProject writes out a pcl.Program directly as .pp files
func pclGenerateProject(directory string, project workspace.Project, p *pcl.Program) error {
	if directory != "." {
		err := os.MkdirAll(directory, 0755)
		if err != nil {
			return fmt.Errorf("could not create output directory: %w", err)
		}
	}

	// We don't write out the Pulumi.yaml for PCL, just the .pp files.
	for file, source := range p.Source() {
		outputFile := path.Join(directory, file)
		err := ioutil.WriteFile(outputFile, []byte(source), 0600)
		if err != nil {
			return fmt.Errorf("could not write output program: %w", err)
		}
	}

	return nil
}

// pclEject
func pclEject(directory string, loader schema.ReferenceLoader) (*workspace.Project, *pcl.Program, error) {
	parser := hclsyntax.NewParser()
	// Load all .pp files in the directory
	err := filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) == ".pp" {
			file, err := os.Open(path)
			if err != nil {
				return err
			}

			err = parser.ParseFile(file, filepath.Base(path))
			if err != nil {
				return err
			}
			diags := parser.Diagnostics
			if diags.HasErrors() {
				return diags
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	program, pdiags, err := pcl.BindProgram(parser.Files, pcl.Loader(loader))
	if err != nil {
		return nil, nil, err
	}
	if pdiags.HasErrors() || program == nil {
		return nil, nil, fmt.Errorf("internal error: %w", pdiags)
	}

	return &workspace.Project{Name: "pcl"}, program, nil
}

func runConvert(
	cwd string, mappings []string, from string, language string,
	outDir string, generateOnly bool) result.Result {

	var projectGenerator projectGeneratorFunc
	switch language {
	case "csharp", "c#":
		projectGenerator = dotnet.GenerateProject
	case "go":
		projectGenerator = gogen.GenerateProject
	case "typescript":
		projectGenerator = nodejs.GenerateProject
	case "python": // nolint: goconst
		projectGenerator = python.GenerateProject
	case "java": // nolint: goconst
		projectGenerator = javagen.GenerateProject
	case "yaml": // nolint: goconst
		projectGenerator = yamlgen.GenerateProject
	case "pulumi", "pcl":
		if cmdutil.IsTruthy(os.Getenv("PULUMI_DEV")) {
			projectGenerator = pclGenerateProject
			break
		}
		fallthrough

	default:
		return result.Errorf("cannot generate programs for %q language", language)
	}

	if outDir != "." {
		err := os.MkdirAll(outDir, 0755)
		if err != nil {
			return result.FromError(fmt.Errorf("could not create output directory: %w", err))
		}
	}

	host, err := newPluginHost()
	if err != nil {
		return result.FromError(fmt.Errorf("could not create plugin host: %w", err))
	}
	defer contract.IgnoreClose(host)
	loader := schema.NewPluginLoader(host)
	mapper, err := convert.NewPluginMapper(host, from, mappings)
	if err != nil {
		return result.FromError(fmt.Errorf("could not create provider mapper: %w", err))
	}

	var proj *workspace.Project
	var program *pcl.Program
	if from == "" || from == "yaml" {
		proj, program, err = yamlgen.Eject(cwd, loader)
		if err != nil {
			return result.FromError(fmt.Errorf("could not load yaml program: %w", err))
		}
	} else if from == "pcl" {
		if cmdutil.IsTruthy(os.Getenv("PULUMI_DEV")) {
			// No plugin for PCL to generate with
			generateOnly = true
			proj, program, err = pclEject(cwd, loader)
			if err != nil {
				return result.FromError(fmt.Errorf("could not load pcl program: %w", err))
			}
		} else {
			return result.FromError(fmt.Errorf("unrecognized source %s", from))
		}
	} else if from == "tf" {
		proj, program, err = tfgen.Eject(cwd, loader, mapper)
		if err != nil {
			return result.FromError(fmt.Errorf("could not load terraform program: %w", err))
		}
	} else {
		return result.FromError(fmt.Errorf("unrecognized source %s", from))
	}

	err = projectGenerator(outDir, *proj, program)
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
		pwd, _, ctx, err := engine.ProjectInfoContext(projinfo, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil)
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

func newPluginHost() (plugin.Host, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sink := diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
		Color: cmdutil.GetGlobalColorization(),
	})
	pluginCtx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, true, nil)
	if err != nil {
		return nil, err
	}
	return pluginCtx.Host, nil
}
