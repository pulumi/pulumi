// Copyright 2016-2026, Pulumi Corporation.
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

package constrictor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// An annotation key that is hopefully not going to conflict with anything else.
const annotationKey = "constrictor:args"

type Arg struct {
	// A human-readable name for the argument, not used by Cobra.
	Name string `json:"name"`

	// The type of the argument, defaulting to "string".
	Type string `json:"type,omitempty"`

	// Usage is an optional override for how this argument appears in the usage string.
	// If not provided, Name is used. This allows specifying formats like "<org-name>/<policy-pack-name>"
	// instead of just the argument name.
	Usage string `json:"usage,omitempty"`
}

// The specification for command arguments.
type Arguments struct {
	Args []Arg `json:"args"`

	// Required is the number of required leading arguments.
	Required int `json:"required"`

	// Variadic indicates that the last argument may repeat.
	Variadic bool `json:"variadic,omitempty"`
}

// Declare the arguments for a command.
//
// By default, Cobra describes arguments to a command using a predicate.
// This causes a problem when we want to generate a CLI specification as we
// can't know the names or types of the arguments. To fix this, we introduce
// the `Arguments` type.
//
// In a slight divergence from the way flags work, this function takes all
// the arguments for a command in one go. It does this because it also then
// converts the data structure directly into the argument predicate. In
// contraist, if we took the arguments one at a time, we would need to have
// some way to tell the command when all arguments have been provided, which
// would either involve introducing a new function, or wrapping the entire
// command type and making substantial changes to the CLI structure.
//
// Assuming a Use string contains the command name, this function will create
// the argument predicate, and produce a correct Use string to match.
func AttachArgs(cmd *cobra.Command, spec *Arguments) {
	if cmd == nil || spec == nil {
		return
	}

	args, err := compile(spec)
	if err != nil {
		return
	}
	cmd.Args = args

	// If a command has a Use string, we use `cobra.Command.Name` (which assumes
	// the first word is the command name) to get the command name and produce a
	// better Use string.
	cmdName := cmd.Name()
	argString, err := generateUseString(spec)
	if err != nil {
		return
	}
	if argString != "" {
		cmd.Use = cmdName + " " + argString
	} else {
		cmd.Use = cmdName
	}

	bytes, err := json.Marshal(spec)
	if err != nil {
		return
	}

	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}

	cmd.Annotations[annotationKey] = string(bytes)
}

// Get the argument specification for a command.
//
// As well as creating the argument predicate and usage string, we also store
// the argument specification in the command annotations to allow us to read
// it elsewhere. This function attempts to decode an argument specification.
func ExtractArgs(cmd *cobra.Command) (*Arguments, error) {
	if cmd == nil || cmd.Annotations == nil {
		return nil, fmt.Errorf("command has no annotations")
	}

	raw, ok := cmd.Annotations[annotationKey]
	if !ok || raw == "" {
		return nil, fmt.Errorf("command has no arguments")
	}

	var spec Arguments
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	return &spec, nil
}

// Compile the argument specification into a cobra.PositionalArgs validator.
//
// Where possible, we use the standard cmdutil / cobra helpers.
func compile(spec *Arguments) (cobra.PositionalArgs, error) {
	if spec == nil {
		return nil, fmt.Errorf("no arguments provided")
	}

	argCount := len(spec.Args)
	required := spec.Required

	// No arguments
	if argCount == 0 && required == 0 && !spec.Variadic {
		return cmdutil.NoArgs, nil
	}

	// Specific arguments.
	if !spec.Variadic && argCount > 0 && required == argCount {
		argNames := make([]string, argCount)
		for i, arg := range spec.Args {
			argNames[i] = arg.Name
		}

		return cmdutil.SpecificArgs(argNames), nil
	}

	// Repeated arguments.
	if spec.Variadic {
		if required == 0 {
			return cobra.ArbitraryArgs, nil
		}

		return cmdutil.MinimumNArgs(required), nil
	}

	// Up to N optional arguments.
	if !spec.Variadic && required == 0 && argCount > 0 {
		return cmdutil.MaximumNArgs(argCount), nil
	}

	// A range of arguments.
	if !spec.Variadic && required > 0 && argCount > required {
		return cmdutil.RangeArgs(required, argCount), nil
	}

	return nil, fmt.Errorf("unknown argument specification")
}

// Generate a Use string from an arguments specification.
//
// * <arg> for required arguments
// * [arg] for optional arguments
// * <arg>... for required variadic (at least one)
// * [arg]... for optional variadic (zero or more)
func generateUseString(spec *Arguments) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("no arguments provided")
	}

	argCount := len(spec.Args)
	if argCount == 0 && spec.Required == 0 && !spec.Variadic {
		return "", fmt.Errorf("no arguments provided")
	}

	var parts []string
	for i, arg := range spec.Args {
		// Use Usage if provided, otherwise fall back to Name
		argName := arg.Usage
		if argName == "" {
			argName = arg.Name
		}

		var part string
		if i < spec.Required {
			part = "<" + argName + ">"
		} else {
			part = "[" + argName + "]"
		}
		parts = append(parts, part)
	}

	if spec.Variadic && len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		parts[len(parts)-1] = lastPart + "..."
	}

	return strings.Join(parts, " "), nil
}
