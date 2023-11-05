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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newConfigEnvRmCmd(stackRef *string) *cobra.Command {
	var showSecrets bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove environments from a stack",
		Long:  "Removes an environment from a stack's import list.",
		Args:  cmdutil.ExactArgs(1),
		Run: cmdutil.RunFunc(func(_ *cobra.Command, args []string) error {
			return editStackEnvironment(*stackRef, showSecrets, yes, func(stack *workspace.ProjectStack) error {
				stack.Environment = stack.Environment.Remove(args[0])
				return nil
			})
		}),
	}

	cmd.Flags().BoolVar(
		&showSecrets, "show-secrets", false,
		"Show secret values in plaintext instead of ciphertext")
	cmd.Flags().BoolVarP(
		&yes, "yes", "y", false,
		"True to save changes without prompting")

	return cmd
}
