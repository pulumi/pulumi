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
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	gogen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/python"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func newConvertCmd() *cobra.Command {
	var language string

	cmd := &cobra.Command{
		Use:    "convert",
		Args:   cmdutil.MaximumNArgs(0),
		Hidden: !hasExperimentalCommands(),
		Short:  "Convert resource declarations into a pulumi program",
		Long: "Convert resource declarations into a pulumi program.\n" +
			"\n" +
			"The PCL program to convert should be supplied on stdin.\n",
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {

			var programGenerator programGeneratorFunc
			switch language {
			case "dotnet":
				programGenerator = dotnet.GenerateProgram
			case "go":
				programGenerator = gogen.GenerateProgram
			case "nodejs":
				programGenerator = nodejs.GenerateProgram
			case "python":
				programGenerator = python.GenerateProgram
			default:
				return result.Errorf("cannot generate programs for %v", language)
			}

			parser := syntax.NewParser()
			err := parser.ParseFile(os.Stdin, "main.pp")
			if err != nil {
				return result.Errorf("could not read stdin: %w", err)
			}
			if parser.Diagnostics.HasErrors() {
				return result.Errorf("could not parse input: %v", parser.Diagnostics)
			}
			pclProgram, diagnostics, err := pcl.BindProgram(parser.Files)
			if err != nil {
				return result.Errorf("could not bind input program: %w", err)
			}
			if diagnostics.HasErrors() {
				return result.Errorf("could not bind input program: %v", diagnostics)
			}

			files, diagnostics, err := programGenerator(pclProgram)
			if err != nil {
				return result.Errorf("could not generate output program: %w", err)
			}
			if diagnostics.HasErrors() {
				return result.Errorf("could not generate output program: %v", diagnostics)
			}

			for filename, data := range files {
				ioutil.WriteFile(filename, data, 0644)
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(
		//nolint:lll
		&language, "language", "", "Which language plugin to use to generate the pulumi program")

	return cmd
}
