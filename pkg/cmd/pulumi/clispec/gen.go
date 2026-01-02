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
	"os"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// NewGenCLISpecCmd returns a new command that, when run, generates a CLI specification JSON file.
//
// This is a bit tricky as there's a lot that we can't infer from the CLI specification. For example, if a command has
// arguments, we can't infer what they are, or how many of them are required. We could try parsing the usage string,
// but that seems like the wrong approach. Potentially, we'll come back to this later and maybe extend the Cobra
// command structure to reify the argument structure.
func NewGenCLISpecCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "generate-cli-spec [OUTPUT_PATH]",
		Args:   cmdutil.MaximumNArgs(1),
		Short:  "Generate Pulumi CLI specification as JSON",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputPath := "specification.json"
			if len(args) > 0 {
				outputPath = args[0]
			}

			spec := generateSpec(root)

			file, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("failed to create specification: %w", err)
			}
			defer file.Close()

			encoder := json.NewEncoder(file)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(spec); err != nil {
				return fmt.Errorf("failed to encode specification: %w", err)
			}

			fmt.Printf("CLI specification written to: %s\n", outputPath)
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
		Structure:      processCommand(cmd, true),
	}
}

// -- COMMANDS

type MenuStructure struct {
	Type           string         `json:"type"`
	AvailableFlags []Flag         `json:"available_flags"`
	Commands       map[string]any `json:"commands,omitempty"`
}

type CommandStructure struct {
	Type           string `json:"type"`
	AvailableFlags []Flag `json:"available_flags"`
	Arguments      any    `json:"arguments,omitempty"`
	Documentation  string `json:"documentation,omitempty"`
}

func processCommand(cmd *cobra.Command, isRoot bool) any {
	if cmd.Hidden && !isRoot {
		return nil
	}

	subcommands := cmd.Commands()
	isMenu := false

	for _, subcmd := range subcommands {
		if !subcmd.Hidden {
			isMenu = true
			break
		}
	}

	flags := extractFlags(cmd)

	// Menus are commands that have (visible) subcommands.
	if isMenu {
		menu := MenuStructure{
			Type:           "menu",
			AvailableFlags: flags,
			Commands:       make(map[string]any),
		}

		for _, subcmd := range subcommands {
			processed := processCommand(subcmd, false)
			if processed != nil {
				menu.Commands[subcmd.Name()] = processed
			}
		}

		return menu
	}

	// Commands are considered the "leaves" of the tree.
	command := CommandStructure{
		Type:           "command",
		AvailableFlags: flags,
	}

	if cmd.Long != "" {
		command.Documentation = strings.TrimSpace(cmd.Long)
	} else if cmd.Short != "" {
		command.Documentation = strings.TrimSpace(cmd.Short)
	}

	command.Arguments = inferArguments(cmd)
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

func extractFlags(cmd *cobra.Command) []Flag {
	flags := []Flag{}
	seen := make(map[string]bool)

	root := cmd
	for root.HasParent() {
		root = root.Parent()
	}

	// Persistent flags are just "stuff inherited from the parent". We don't have to recurse any higher,
	// as the parent command's persistent flags will become persistent flags in the child.
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			flag := extractFlagInfo(f)
			if flag != nil && !seen[flag.LongName] {
				flags = append(flags, *flag)
				seen[flag.LongName] = true
			}
		}
	})

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			flag := extractFlagInfo(f)
			if flag != nil && !seen[flag.LongName] {
				flags = append(flags, *flag)
				seen[flag.LongName] = true
			}
		}
	})

	return flags
}

func extractFlagInfo(f *pflag.Flag) *Flag {
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

	return flag
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
}

type Cardinality struct {
	AtLeast int `json:"at_least,omitempty"`
	AtMost  int `json:"at_most,omitempty"`
}

func inferArguments(cmd *cobra.Command) any {
	if detected := detectKnownArgumentPredicate(cmd); detected != nil {
		return detected
	}

	return HeterogeneousArguments{
		Type:              "heterogeneous",
		Specifications:    []ArgSpec{},
		RequiredArguments: 0,
	}
}

func detectKnownArgumentPredicate(cmd *cobra.Command) any {
	if cmd.Args == nil {
		return nil
	}

	argsFunc := reflect.ValueOf(cmd.Args)
	if argsFunc.Kind() != reflect.Func {
		return nil
	}

	knownPredicates := []struct {
		name string
		fn   any
	}{
		{"cmdutil.NoArgs", cmdutil.NoArgs},
		{"cobra.NoArgs", cobra.NoArgs},
		{"cobra.ArbitraryArgs", cobra.ArbitraryArgs},
	}

	for _, pred := range knownPredicates {
		predFunc := reflect.ValueOf(pred.fn)
		if predFunc.Kind() == reflect.Func {
			if argsFunc.Pointer() == predFunc.Pointer() {
				// For NoArgs, return empty arguments
				if pred.name == "cmdutil.NoArgs" || pred.name == "cobra.NoArgs" {
					return HeterogeneousArguments{
						Type:              "heterogeneous",
						Specifications:    []ArgSpec{},
						RequiredArguments: 0,
					}
				}
			}
		}
	}

	return nil
}
