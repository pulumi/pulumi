// Copyright 2016-2017, Pulumi Corporation
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
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/compiler/core"
	"github.com/pulumi/lumi/pkg/eval"
	"github.com/pulumi/lumi/pkg/resource/deploy"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
)

func newPackEvalCmd() *cobra.Command {
	var configEnv string
	var dotOutput bool
	var cmd = &cobra.Command{
		Use:   "eval [package] [-- [args]]",
		Short: "Evaluate a package and print the resulting objects",
		Long: "Evaluate a package and print the resulting objects\n" +
			"\n" +
			"A graph is a topologically sorted directed-acyclic-graph (DAG), representing a\n" +
			"collection of resources that may be used in a deployment operation like plan or apply.\n" +
			"This graph is produced by evaluating the contents of a blueprint package, and does not\n" +
			"actually perform any updates to the target environment.\n" +
			"\n" +
			"By default, a blueprint package is loaded from the current directory.  Optionally,\n" +
			"a path to a package elsewhere can be provided as the [package] argument.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			contract.Assertf(!dotOutput, "TODO[pulumi/lumi#235]: DOT files not yet supported")

			// First, load and compile the package.
			result := compile(cmd, args)
			if result == nil {
				return nil
			}

			// Now fire up an interpreter so we can run the program.
			e := eval.New(result.B.Ctx(), nil)

			// If configuration was requested, load it up and populate the object state.
			if configEnv != "" {
				envInfo, err := initEnvCmdName(tokens.QName(configEnv), args)
				if err != nil {
					return err
				}
				if err := deploy.InitEvalConfig(result.B.Ctx(), e, envInfo.Target.Config); err != nil {
					return err
				}
			}

			// Finally, execute the entire program, and serialize the return value (if any).
			packArgs := dashdashArgsToMap(args)
			if obj, _ := e.EvaluatePackage(result.Pkg, packArgs); obj != nil {
				fmt.Print(obj)
			}
			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(
		&configEnv, "config-env", "",
		"Apply configuration from the specified environment before evaluating the package")
	cmd.PersistentFlags().BoolVar(
		&dotOutput, "dot", false,
		"Output the graph as a DOT digraph (graph description language)")

	return cmd
}

// dashdashArgsToMap is a simple args parser that places incoming key/value pairs into a map.  These are then used
// during package compilation as inputs to the main entrypoint function.
// IDEA: this is fairly rudimentary; we eventually want to support arrays, maps, and complex types.
func dashdashArgsToMap(args []string) core.Args {
	mapped := make(core.Args)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Eat - or -- at the start.
		if arg[0] == '-' {
			arg = arg[1:]
			if arg[0] == '-' {
				arg = arg[1:]
			}
		}

		// Now find a k=v, and split the k/v part.
		if eq := strings.IndexByte(arg, '='); eq != -1 {
			// For --k=v, simply store v underneath k's entry.
			mapped[tokens.Name(arg[:eq])] = arg[eq+1:]
		} else {
			if i+1 < len(args) && args[i+1][0] != '-' {
				// If the next arg doesn't start with '-' (i.e., another flag) use its value.
				mapped[tokens.Name(arg)] = args[i+1]
				i++
			} else if arg[0:3] == "no-" {
				// For --no-k style args, strip off the no- prefix and store false underneath k.
				mapped[tokens.Name(arg[3:])] = false
			} else {
				// For all other --k args, assume this is a boolean flag, and set the value of k to true.
				mapped[tokens.Name(arg)] = true
			}
		}
	}

	return mapped
}
