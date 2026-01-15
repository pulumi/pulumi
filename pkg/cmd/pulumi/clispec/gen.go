// Copyright 2016-2024, Pulumi Corporation.
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

// NewGenCLISpecCmd returns a new command that, when run, generates a CLI specification JSON file.
//
// This is a bit tricky as there's a lot that we can't infer from the CLI specification. For example, if a command has
// arguments, we can't infer what they are, or how many of them are required. We could try parsing the usage string,
// but that seems like the wrong approach. Potentially, we'll come back to this later and maybe extend the Cobra
// command structure to reify the argument structure.
func NewGenCLISpecCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "generate-cli-spec",
		Short:  "Generate Pulumi CLI specification as JSON",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			spec := generateSpec(root)

			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(spec); err != nil {
				return fmt.Errorf("failed to encode specification: %w", err)
			}

			return nil
		},
	}
}

// -- SPECIFICATION

type Specification struct {
	ExecutablePath string `json:"executable_path"`
	Structure      any    `json:"structure"`
}

func generateSpec(cmd *cobra.Command) Specification {
	return Specification{
		ExecutablePath: "pulumi",
		Structure:      processCommand(cmd),
	}
}

// -- COMMANDS

type MenuStructure struct {
	Type     string          `json:"type"`
	Flags    map[string]Flag `json:"flags"`
	Commands map[string]any  `json:"commands,omitempty"`
}

type CommandStructure struct {
	Type              string          `json:"type"`
	Flags             map[string]Flag `json:"flags"`
	Arguments         any             `json:"arguments,omitempty"`
	Documentation     string          `json:"documentation,omitempty"`
	RunsPulumiProgram bool            `json:"runs_pulumi_program,omitempty"`
}

func processCommand(cmd *cobra.Command) any {
	subcommands := cmd.Commands()
	isMenu := len(subcommands) > 0
	flags := extractFlags(cmd)

	// Menus are commands that have (visible) subcommands.
	if isMenu {
		menu := MenuStructure{
			Type:     "menu",
			Flags:    flags,
			Commands: make(map[string]any),
		}

		for _, subcmd := range subcommands {
			processed := processCommand(subcmd)
			if processed != nil {
				menu.Commands[subcmd.Name()] = processed
			}
		}

		return menu
	}

	// Commands are considered the "leaves" of the tree.
	command := CommandStructure{
		Type:  "command",
		Flags: flags,
	}

	if cmd.Long != "" {
		command.Documentation = strings.TrimSpace(cmd.Long)
	} else if cmd.Short != "" {
		command.Documentation = strings.TrimSpace(cmd.Short)
	}

	args := inferArguments(cmd)
	if args != nil {
		command.Arguments = args
	}

	if cmd.Flags().Lookup("client") != nil {
		command.RunsPulumiProgram = true
	}

	return command
}

// -- FLAGS

type Flag struct {
	LongName      string `json:"longName"`
	ShortName     string `json:"shortName,omitempty"`
	Type          string `json:"type"`
	Documentation string `json:"documentation,omitempty"`
	Repeatable    bool   `json:"repeatable,omitempty"`
}

func extractFlags(cmd *cobra.Command) map[string]Flag {
	flags := make(map[string]Flag)

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		flag := &Flag{
			LongName:      f.Name,
			Documentation: f.Usage,
		}

		if f.Shorthand != "" && f.Shorthand != " " {
			flag.ShortName = f.Shorthand
		}

		flagType := f.Value.Type()
		switch flagType {
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

		flags[flag.LongName] = *flag
	})

	return flags
}

// -- ARGUMENTS

type HeterogeneousArguments struct {
	Type              string    `json:"type"`
	Specifications    []ArgSpec `json:"specifications"`
	RequiredArguments int       `json:"required_arguments"`
}

type HomogeneousArguments struct {
	Type          string      `json:"type"`
	Specification ArgSpec     `json:"specification"`
	Cardinality   Cardinality `json:"cardinality"`
}

type ArgSpec struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type Cardinality struct {
	AtLeast int `json:"at_least,omitempty"`
	AtMost  int `json:"at_most,omitempty"`
}

func inferArguments(cmd *cobra.Command) any {
	// Try to extract arguments from constrictor annotations
	constrictorArgs, err := constrictor.ExtractArgs(cmd)
	if err != nil {
		return nil
	}

	if constrictorArgs != nil {
		return convertConstrictorArguments(constrictorArgs)
	}

	return nil
}

func convertConstrictorArguments(args *constrictor.Arguments) any {
	if args == nil {
		return HeterogeneousArguments{
			Type:              "heterogeneous",
			Specifications:    []ArgSpec{},
			RequiredArguments: 0,
		}
	}

	argCount := len(args.Args)
	required := args.Required

	// No arguments.
	if argCount == 0 && required == 0 && !args.Variadic {
		return HeterogeneousArguments{
			Type:              "heterogeneous",
			Specifications:    []ArgSpec{},
			RequiredArguments: 0,
		}
	}

	// Convert constrictor Args to ArgSpecs.
	specs := make([]ArgSpec, argCount)
	for i, arg := range args.Args {
		specs[i] = ArgSpec{
			Name: arg.Name,
			Type: arg.Type,
		}
	}

	// For monotyped variadic arguments, use the homogeneous format.
	if args.Variadic && argCount == 1 {
		cardinality := Cardinality{
			AtLeast: required,
		}

		return HomogeneousArguments{
			Type:          "homogeneous",
			Specification: specs[0],
			Cardinality:   cardinality,
		}
	}

	// Otherwise, assume heterogeneous.
	return HeterogeneousArguments{
		Type:              "heterogeneous",
		Specifications:    specs,
		RequiredArguments: required,
	}
}
