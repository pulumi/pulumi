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

package main

import (
	"encoding/json"
	"fmt"
)

// Flag describes a single CLI flag on a command or menu.
type Flag struct {
	// Name is the canonical flag name (for example, "stack").
	Name string `json:"name"`

	// Type is a primitive logical type: "string", "boolean", "int", etc.
	Type string `json:"type"`

	// Description is the user-facing description of the flag.
	Description string `json:"description,omitempty"`

	// Repeatable is true if the flag may appear multiple times (for example, string arrays).
	Repeatable bool `json:"repeatable,omitempty"`

	// Required marks the flag as mandatory. Generated method bodies append it
	// unconditionally instead of guarding on its zero value.
	Required bool `json:"required,omitempty"`

	// Omit hides the flag from the generated Options struct. Typically paired
	// with Preset to force a fixed value on the CLI invocation.
	Omit bool `json:"omit,omitempty"`

	// Preset forces a value onto the CLI invocation. When the flag is also
	// exposed on the Options struct (Omit == false), the preset is only
	// applied when the user has not set the field.
	Preset *PresetValue `json:"preset,omitempty"`
}

// PresetValue is a sum over the primitive values a preset can take:
// bool, string, int, or a []string. Exactly one of the pointer/slice fields
// is populated after unmarshalling.
type PresetValue struct {
	Bool    *bool
	String  *string
	Int     *int
	Strings []string
}

// UnmarshalJSON discriminates the preset payload by JSON kind.
func (p *PresetValue) UnmarshalJSON(data []byte) error {
	// Strings array first so [] is not ambiguous with null.
	var asStrings []string
	if err := json.Unmarshal(data, &asStrings); err == nil {
		p.Strings = asStrings
		return nil
	}

	var asBool bool
	if err := json.Unmarshal(data, &asBool); err == nil {
		p.Bool = &asBool
		return nil
	}

	var asInt int
	if err := json.Unmarshal(data, &asInt); err == nil {
		p.Int = &asInt
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		p.String = &asString
		return nil
	}

	return fmt.Errorf("preset value must be one of bool, string, int, []string; got %s", string(data))
}

// Argument is a positional argument to a command.
type Argument struct {
	// Name is the human-readable name for the argument.
	Name string `json:"name"`

	// Type is the argument type, defaulting to "string" when omitted.
	Type string `json:"type,omitempty"`

	// Usage is an optional override for how the argument appears in the usage string.
	// Mirrors the `Usage` field in the Go struct.
	Usage string `json:"usage,omitempty"`
}

// Arguments is the full positional argument specification for a command.
type Arguments struct {
	// Arguments are all positional arguments (in order).
	Arguments []Argument `json:"arguments"`

	// RequiredArguments is the number of required leading arguments.
	RequiredArguments *int `json:"requiredArguments,omitempty"`

	// Variadic is true if the last argument is variadic.
	Variadic bool `json:"variadic,omitempty"`
}

// Structure is a node in the CLI tree.
//
// It unifies both menus and commands:
//   - Menus have Type == "menu" and may contain child Commands.
//   - Commands have Type == "command" and may contain positional Arguments.
type Structure struct {
	// Type is the node type discriminator ("menu" or "command").
	Type string `json:"type"`

	// Executable is true when a menu can also be invoked as a command in its
	// own right. Meaningless for leaf commands.
	Executable bool `json:"executable,omitempty"`

	// Flags are the flags available at this level of the hierarchy, keyed by
	// their canonical flag name.
	Flags map[string]Flag `json:"flags,omitempty"`

	// Commands are the subcommands in this menu when Type == "menu".
	Commands map[string]Structure `json:"commands,omitempty"`

	// Arguments are the positional arguments for this command when Type == "command".
	Arguments *Arguments `json:"arguments,omitempty"`

	// Description is free-form documentation about what the command does.
	Description string `json:"description,omitempty"`
}
