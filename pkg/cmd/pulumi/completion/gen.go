// Copyright 2016, Pulumi Corporation.
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

package completion

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/spf13/cobra"
)

// NewGenCompletionCmd returns a new command that, when run, generates a shell completion script for the CLI.
//
// It emits Cobra's modern (V2) completion scripts, which delegate to the
// `pulumi __complete` hidden subcommand at runtime. This is what makes
// `RegisterFlagCompletionFunc` registrations such as the one on `--stack` work
// in interactive shells.
func NewGenCompletionCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gen-completion",
		Aliases: []string{"completion"},
		Short:   "Generate completion scripts for the Pulumi CLI",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("%q is not a supported shell", args[0])
			}
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{
				Name:  "shell",
				Usage: "<shell>",
				Type:  "string",
			},
		},
		Required: 1,
	})

	return cmd
}
