// Copyright 2026, Pulumi Corporation.
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

package cli

import (
	"github.com/spf13/cobra"
)

func newEnvProviderCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage login providers within an environment",
		Long:  "[EXPERIMENTAL] Manage providers within an environment\n",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newEnvProviderAWSLoginCmd(env))
	cmd.AddCommand(newEnvProviderAzureLoginCmd(env))
	cmd.AddCommand(newEnvProviderGCPLoginCmd(env))

	return cmd
}
