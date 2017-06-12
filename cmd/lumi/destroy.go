// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"github.com/spf13/cobra"

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
)

func newDestroyCmd() *cobra.Command {
	var dryRun bool
	var env string
	var summary bool
	var yes bool
	var cmd = &cobra.Command{
		Use:   "destroy",
		Short: "Destroy an existing environment and its resources",
		Long: "Destroy an existing environment and its resources\n" +
			"\n" +
			"This command deletes an entire existing environment by name.  The current state is\n" +
			"loaded from the associated snapshot file in the workspace.  After running to completion,\n" +
			"all of this environment's resources and associated state will be gone.\n" +
			"\n" +
			"Warning: although old snapshots can be used to recreate an environment, this command\n" +
			"is generally irreversable and should be used with great care.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			info, err := initEnvCmdName(tokens.QName(env), args)
			if err != nil {
				return err
			}
			name := info.Target.Name
			if dryRun || yes ||
				confirmPrompt("This will permanently destroy all resources in the '%v' environment!", name) {
				deployLatest(cmd, info, deployOptions{
					Delete:  true,
					DryRun:  dryRun,
					Summary: summary,
				})
			}
			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&dryRun, "dry-run", "n", false,
		"Don't actually delete resources; just print out the planned deletions")
	cmd.PersistentFlags().StringVarP(
		&env, "env", "e", "",
		"Choose an environment other than the currently selected one")
	cmd.PersistentFlags().BoolVarP(
		&summary, "summary", "s", false,
		"Only display summarization of resources and plan operations")
	cmd.PersistentFlags().BoolVar(
		&yes, "yes", false,
		"Skip confirmation prompts, and proceed with the destruction anyway")

	return cmd
}
