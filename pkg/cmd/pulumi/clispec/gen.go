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

package clispec

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// A single CLI flag.
type Flag struct {
	// The canonical flag name.
	Name string `json:"name"`

	// The primitive type of the flag ("string", "boolean", "int", ...).
	Type string `json:"type"`

	// The description of the flag.
	Description string `json:"description,omitempty"`

	// Allows for arrays to be passed by flags.
	Repeatable bool `json:"repeatable,omitempty"`
}

// A set of subcommands.
type Menu struct {
	// Always "menu".
	Type string `json:"type"`

	// Executable is true when this command has a Run or RunE implementation.
	// When false, the command is a pure menu of subcommands and does nothing when invoked by itself.
	Executable bool `json:"executable"`

	// Flags specific to this menu (not including inherited flags).
	Flags map[string]Flag `json:"flags,omitempty"`

	// The set of subcommands in this menu.
	Commands map[string]any `json:"commands,omitempty"`
}

// A command in the CLI.
type Command struct {
	// Always "command".
	Type string `json:"type"`

	// Flags specific to this command (not including inherited flags).
	Flags map[string]Flag `json:"flags,omitempty"`

	// Positional arguments.
	Arguments *constrictor.Arguments `json:"arguments,omitempty"`

	// The description of the command.
	Description string `json:"description,omitempty"`
}

func NewGenCLISpecCmd(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "generate-cli-spec",
		Short:  "Generate a Pulumi CLI specification as JSON",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			spec := buildStructure(root)

			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(spec); err != nil {
				return fmt.Errorf("failed to encode CLI specification: %w", err)
			}

			return nil
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	return cmd
}

func buildStructure(cmd *cobra.Command) any {
	subcommands := cmd.Commands()
	if len(subcommands) > 0 {
		menu := Menu{
			Type:        "menu",
			Executable:  cmd.Runnable(),
			Flags:       collectFlags(cmd),
		}

		if len(subcommands) > 0 {
			menu.Commands = make(map[string]any)
			for _, subcmd := range subcommands {
				processed := buildStructure(subcmd)
				if processed != nil {
					menu.Commands[subcmd.Name()] = processed
				}
			}
		}

		return menu
	}

	description := cmd.Long
	if description == "" {
		description = cmd.Short
	}

	command := Command{
		Type:        "command",
		Flags:       collectFlags(cmd),
		Arguments:   extractArguments(cmd),
		Description: strings.TrimSpace(description),
	}

	return command
}

func collectFlags(cmd *cobra.Command) map[string]Flag {
	flags := make(map[string]Flag)

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		flag := Flag{
			Name:        f.Name,
			Description: f.Usage,
		}

		switch f.Value.Type() {
		case "bool":
			flag.Type = "boolean"
		case "stringSlice", "stringArray":
			flag.Type = "string"
			flag.Repeatable = true
		case "int", "int32", "int64":
			flag.Type = "int"
		default:
			flag.Type = "string"
		}

		flags[flag.Name] = flag
	})

	if len(flags) == 0 {
		return nil
	}

	return flags
}

func extractArguments(cmd *cobra.Command) *constrictor.Arguments {
	spec, err := constrictor.ExtractArgs(cmd)
	if err != nil {
		_, _ = fmt.Fprintf(
			cmd.ErrOrStderr(),
			"warning: failed to extract constrictor arguments for command %q: %v\n",
			cmd.CommandPath(),
			err,
		)
		return nil
	}

	return spec
}
