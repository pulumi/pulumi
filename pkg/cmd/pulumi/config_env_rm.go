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
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newConfigEnvRmCmd(parent *configEnvCmd) *cobra.Command {
	impl := configEnvRmCmd{parent: parent}

	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove environments from a stack",
		Long:  "Removes an environment from a stack's import list.",
		Args:  cmdutil.ExactArgs(1),
		Run:   cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error { return impl.run(cmd.Context(), args) }),
	}

	cmd.Flags().BoolVar(
		&impl.showSecrets, "show-secrets", false,
		"Show secret values in plaintext instead of ciphertext")
	cmd.Flags().BoolVarP(
		&impl.yes, "yes", "y", false,
		"True to save changes without prompting")

	return cmd
}

type configEnvRmCmd struct {
	parent *configEnvCmd

	showSecrets bool
	yes         bool
}

func (cmd *configEnvRmCmd) run(ctx context.Context, args []string) error {
	return cmd.parent.editStackEnvironment(
		ctx, cmd.showSecrets, cmd.yes, func(stack *workspace.ProjectStack) error {
			stack.Environment = stack.Environment.Remove(args[0])
			return nil
		})
}
