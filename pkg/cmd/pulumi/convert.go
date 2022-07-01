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
	"os"

	"github.com/spf13/cobra"

	javagen "github.com/pulumi/pulumi-java/pkg/codegen/java"
	yamlgen "github.com/pulumi/pulumi-yaml/pkg/pulumiyaml/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type projectGeneratorFunc func(directory string, project workspace.Project, p *pcl.Program) error

func newConvertCmd() *cobra.Command {
	var outDir string
	var language string

	cmd := &cobra.Command{
		Use:    "convert",
		Args:   cmdutil.MaximumNArgs(0),
		Hidden: !hasExperimentalCommands(),
		Short:  "Convert resource declarations into a pulumi program",
		Long: "Convert resource declarations into a pulumi program.\n" +
			"\n" +
			"The YAML program to convert will default to the manifest in the current working directory.\n" +
			"You may also specify '-f' for the file path or '-d' for the directory path containing the manifests.\n",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {

			var projectGenerator projectGeneratorFunc
			switch language {
			case "csharp", "c#":
				projectGenerator = dotnet.GenerateProject
			case langGo:
				projectGenerator = gogen.GenerateProject
			case "typescript":
				projectGenerator = nodejs.GenerateProject
			case langPython:
				projectGenerator = python.GenerateProject
			case "java":
				projectGenerator = javagen.GenerateProject
			case "yaml": // nolint: goconst
				projectGenerator = yamlgen.GenerateProject
			default:
				return result.Errorf("cannot generate programs for %q language", language)
			}

			cwd, err := os.Getwd()
			if err != nil {
				return result.FromError(fmt.Errorf("could not resolve current working directory"))
			}

			if outDir != "." {
				err := os.MkdirAll(outDir, 0755)
				if err != nil {
					return result.FromError(fmt.Errorf("could not create output directory: %w", err))
				}
			}

			proj, pclProgram, err := yamlgen.Eject(cwd, nil)
			if err != nil {
				return result.FromError(fmt.Errorf("could not load yaml program: %w", err))
			}

			err = projectGenerator(outDir, *proj, pclProgram)
			if err != nil {
				return result.FromError(fmt.Errorf("could not generate output program: %w", err))
			}

			// Project should now exist at outDir. Run installDependencies in that directory
			// Change the working directory to the specified directory.
			if err := os.Chdir(outDir); err != nil {
				return result.FromError(fmt.Errorf("changing the working directory: %w", err))
			}

			// Load the project, to
			proj, root, err := readProject()
			if err != nil {
				return result.FromError(err)
			}

			projinfo := &engine.Projinfo{Proj: proj, Root: root}
			pwd, _, ctx, err := engine.ProjectInfoContext(projinfo, nil, nil, cmdutil.Diag(), cmdutil.Diag(), false, nil)
			if err != nil {
				return result.FromError(err)
			}

			defer ctx.Close()

			if err := installDependencies(ctx, &proj.Runtime, pwd); err != nil {
				return result.FromError(err)
			}

			return nil
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
		&outDir, "out", ".", "The output directory to write the convert project to")

	return cmd
}
