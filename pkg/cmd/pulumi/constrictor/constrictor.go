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

package constrictor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

// A key for the Cobra annotation containing the argument structure.
const cobraAnnotationKey = "constrictor:args"

// An argument to a Cobra command.
type Argument struct {
	// A human-readable name for the argument.
	Name string `json:"name"` // required
	// The type of the argument, defaulting to "string".
	Type string `json:"type,omitempty"`
	// How it should appear in the usage string, defaulting to the name. This is
	// useful when the argument name actually gives us some information, such as
	// "<org-name>/<policy-pack-name>" instead of just "policy-pack-name".
	Usage string `json:"usage,omitempty"`
}

// The specification for a Cobra command's arguments.
type Arguments struct {
	// Each argument's specification.
	Arguments []Argument `json:"arguments"`
	// The number of required leading arguments.
	Required int `json:"requiredArguments,omitempty"`
	// If true, the last argument is assumed to be repeated. For example, a
	// command might take a series of URNs, so we represent this as a variadic
	// structure with one required argument and one argument specification.
	Variadic bool `json:"variadic,omitempty"`
}

// A convenience shorthand for no arguments.
var NoArgs = &Arguments{
	Arguments: []Argument{},
	Required:  0,
	Variadic:  false,
}

// Declare the arguments for a Cobra command.
//
// By default, Cobra describes arguments to a command using a predicate: the
// arguments are provided as a list of strings, and the predicate says whether
// the arguments are valid. This causes problems when we want to generate code
// on top of the CLI, as we have no way to interrogate a command's arguments:
// how many are there, what are their types, which are required, and so on.
//
// This library provides a structured format for arguments to Cobra commands
// that can be stored as an annotation on the command, but can also then be
// compiled down to an argument predicate for Cobra. As an added benefit, we
// can also use this structure to generate the usage string to ensure that it
// stays up-to-date.
//
// This library diverges slightly from the way flags work in that all commands
// must be specified at the same time. This is because we have to know when we
// have all the argument information in order to compile the predicate. If we
// didn't do this, we'd either have to have an explicit call to finalise the
// arguments, or overwrite the Cobra command structure, which would be a much
// more invasive change.
func AttachArguments(cmd *cobra.Command, arguments *Arguments) {
	if cmd == nil || arguments == nil {
		return // Has no effect.
	}

	cmd.Args = createCobraArgsPredicate(arguments)

	name := cmd.Name()
	cmd.Use = createUsageString(name, arguments)

	serialised, err := json.Marshal(arguments)
	contract.AssertNoErrorf(err, "failed to marshal arguments")

	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}

	cmd.Annotations[cobraAnnotationKey] = string(serialised)
}

// Create a Cobra argument predicate from an arguments specification.
//
// The only thing we need to worry about is the upper and lower bound of
// arguments as this is all we can do with the arguments predicate anyway.
func createCobraArgsPredicate(specification *Arguments) cobra.PositionalArgs {
	return cmdutil.ArgsFunc(func(cmd *cobra.Command, arguments []string) error {
		if len(arguments) < specification.Required {
			return fmt.Errorf("requires at least %d arg(s), only received %d", len(specification.Arguments), len(arguments))
		}

		if !specification.Variadic && len(arguments) > len(specification.Arguments) {
			return fmt.Errorf("accepts at most %d arg(s), received %d", len(specification.Arguments), len(arguments))
		}

		// We don't currently type-check in the arguments predicate, but we could.
		return nil
	})
}

// Create a usage string for a Cobra command from an arguments specification.
//
// Individual arguments can override the usage string with the `Usage` field.
// Otherwise, we wrap required argument names in angle brackets and optional
// argument names in square brackets. Variadic arguments are indicated with an
// ellipsis.
func createUsageString(name string, arguments *Arguments) string {
	if name == "" || len(arguments.Arguments) == 0 {
		return name
	}

	parts := []string{name}
	for index, argument := range arguments.Arguments {
		if argument.Usage != "" {
			parts = append(parts, argument.Usage)
			continue
		}

		if index < arguments.Required {
			parts = append(parts, fmt.Sprintf("<%s>", argument.Name))
		} else {
			parts = append(parts, fmt.Sprintf("[%s]", argument.Name))
		}
	}

	joined := strings.Join(parts, " ")
	if arguments.Variadic {
		return joined + "..."
	}

	return joined
}
