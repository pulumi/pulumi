// Copyright 2026-2026, Pulumi Corporation.
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
