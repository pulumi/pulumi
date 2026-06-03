// Copyright 2024, Pulumi Corporation.
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

func newEnvSettingsCmd(env *envCommand) *cobra.Command {
	registry := NewEnvSettingsRegistry()

	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Manage environment settings",
		Long: "Manage environment settings\n" +
			"\n" +
			"This command manages environment settings such as deletion protection.\n" +
			"\n" +
			"Subcommands exist for reading and updating settings.",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newEnvSettingsGetCmd(env, registry))
	cmd.AddCommand(newEnvSettingsSetCmd(env, registry))

	return cmd
}
