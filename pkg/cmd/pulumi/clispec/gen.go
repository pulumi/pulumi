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
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// A single CLI flag.
type Flag struct {
	// The canonical flag name.
	Name string `json:"name"`

	// If true, this flag is required.
	Required bool `json:"required,omitempty"`

	// The primitive type of the flag ("string", "boolean", "int", ...).
	Type string `json:"type"`

	// The description of the flag.
	Description string `json:"description,omitempty"`

	// Allows for arrays to be passed by flags.
	Repeatable bool `json:"repeatable,omitempty"`

	// If true, this flag should be omitted from generated options types.
	Omit bool `json:"omit,omitempty"`

	// If set, this flag should be preset to this value when invoking the CLI.
	// The type depends on the flag type: boolean, string, number, or []string.
	Preset any `json:"preset,omitempty"`
}

// A set of subcommands.
type Menu struct {
	// Always "menu".
	Type string `json:"type"`

	// True if this menu is also directly executable as a command.
	Executable bool `json:"executable,omitempty"`

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

// Overrides types

// FlagRule describes an override for a single flag.
type FlagRule struct {
	Omit   *bool `json:"omit,omitempty"`
	Preset any   `json:"preset,omitempty"`
}

// OverridesScope describes overrides for a command path.
type OverridesScope struct {
	Path      []string            `json:"path"`
	Propagate bool                `json:"propagate"`
	Flags     map[string]FlagRule `json:"flags"`
}

// Overrides is the top-level automation overrides file.
type Overrides struct {
	Version int              `json:"version"`
	Scopes  []OverridesScope `json:"scopes"`
}

func NewGenCLISpecCmd(root *cobra.Command) *cobra.Command {
	var overridesPath string

	cmd := &cobra.Command{
		Use:    "generate-cli-spec",
		Short:  "Generate a Pulumi CLI specification as JSON",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var overrides *Overrides
			if overridesPath != "" {
				data, err := os.ReadFile(overridesPath)
				if err != nil {
					return fmt.Errorf("failed to read overrides file: %w", err)
				}
				overrides = &Overrides{}
				if err := json.Unmarshal(data, overrides); err != nil {
					return fmt.Errorf("failed to parse overrides file: %w", err)
				}
			}

			spec := buildStructure(root, overrides, nil, nil)

			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(spec); err != nil {
				return fmt.Errorf("failed to encode CLI specification: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&overridesPath, "overrides", "",
		"Path to an automation-overrides.json file to merge into the spec")

	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	return cmd
}

// getMergedRules collects all applicable scopes for a command path and merges
// their flag rules. Deeper scopes override shallower ones per-flag.
func getMergedRules(overrides *Overrides, commandPath []string) map[string]FlagRule {
	if overrides == nil || len(overrides.Scopes) == 0 {
		return nil
	}

	type scored struct {
		depth int
		scope OverridesScope
	}
	var applicable []scored
	for _, scope := range overrides.Scopes {
		if len(scope.Path) > len(commandPath) {
			continue
		}
		match := true
		for i := 0; i < len(scope.Path); i++ {
			if scope.Path[i] != commandPath[i] {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if len(scope.Path) < len(commandPath) && !scope.Propagate {
			continue
		}
		applicable = append(applicable, scored{depth: len(scope.Path), scope: scope})
	}

	if len(applicable) == 0 {
		return nil
	}

	// Stable sort by depth ascending.
	for i := 1; i < len(applicable); i++ {
		for j := i; j > 0 && applicable[j-1].depth > applicable[j].depth; j-- {
			applicable[j-1], applicable[j] = applicable[j], applicable[j-1]
		}
	}

	merged := make(map[string]FlagRule)
	for _, s := range applicable {
		for name, rule := range s.scope.Flags {
			existing := merged[name]
			if rule.Omit != nil {
				existing.Omit = rule.Omit
			}
			if rule.Preset != nil {
				existing.Preset = rule.Preset
			}
			merged[name] = existing
		}
	}
	return merged
}

// applyOverrides merges overrides into a flags map for a given command path.
// inherited is the accumulated set of flags from parent nodes, used so that
// command-specific overrides can target inherited flags.
func applyOverrides(
	flags map[string]Flag, inherited map[string]Flag, overrides *Overrides, commandPath []string,
) map[string]Flag {
	rules := getMergedRules(overrides, commandPath)
	if len(rules) == 0 {
		return flags
	}

	result := make(map[string]Flag, len(flags))
	for name, flag := range flags {
		if rule, ok := rules[name]; ok {
			if rule.Omit != nil && *rule.Omit {
				flag.Omit = true
			}
			if rule.Preset != nil {
				flag.Preset = rule.Preset
			}
		}
		result[name] = flag
	}

	// For rules targeting inherited flags not redefined locally, copy the
	// inherited flag and apply the override so it appears in the spec at this level.
	for name, rule := range rules {
		if _, exists := result[name]; exists {
			continue
		}
		base, ok := inherited[name]
		if !ok {
			continue
		}
		if rule.Omit != nil && *rule.Omit {
			base.Omit = true
		}
		if rule.Preset != nil {
			base.Preset = rule.Preset
		}
		result[name] = base
	}

	return result
}

func buildStructure(
	cmd *cobra.Command, overrides *Overrides, breadcrumbs []string, inherited map[string]Flag,
) any {
	localFlags := collectFlags(cmd)
	merged := applyOverrides(localFlags, inherited, overrides, breadcrumbs)

	// Accumulated flags for children = inherited + this node's local flags (unmodified).
	childInherited := make(map[string]Flag, len(inherited)+len(localFlags))
	for k, v := range inherited {
		childInherited[k] = v
	}
	for k, v := range merged {
		childInherited[k] = v
	}

	subcommands := cmd.Commands()
	if len(subcommands) > 0 {
		menu := Menu{
			Type:       "menu",
			Executable: isExecutable(cmd),
			Flags:      merged,
		}

		menu.Commands = make(map[string]any)
		for _, subcmd := range subcommands {
			childPath := append(append([]string{}, breadcrumbs...), subcmd.Name())
			processed := buildStructure(subcmd, overrides, childPath, childInherited)
			if processed != nil {
				menu.Commands[subcmd.Name()] = processed
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
		Flags:       merged,
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
			Required:    isFlagRequired(f),
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

func isFlagRequired(flag *pflag.Flag) bool {
	if flag == nil {
		return false
	}

	if flag.Annotations == nil {
		return false
	}

	_, required := flag.Annotations[cobra.BashCompOneRequiredFlag]
	return required
}

func isExecutable(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}

	return cmd.RunE != nil || cmd.Run != nil
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
