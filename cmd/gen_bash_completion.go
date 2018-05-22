// Copyright 2016-2018, Pulumi Corporation.
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

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// newGenBashCompletionCmd returns a new command that, when run, generates a bash completion script for the CLI.
// It is hidden by default since it's not commonly used outside of our own build processes.
func newGenBashCompletionCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "gen-bash-completion <FILE>",
		Args:   cmdutil.ExactArgs(1),
		Short:  "Generate a bash completion script for the Pulumi CLI",
		Hidden: true,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return root.GenBashCompletionFile(args[0])
		}),
	}
}
