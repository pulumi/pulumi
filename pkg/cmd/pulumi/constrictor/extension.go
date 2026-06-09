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
	"errors"

	"github.com/spf13/cobra"
)

// AddExtensionFlag registers the shared --extension flag, binding it to target.
// Setting it requests the package's extension shape (served by the base provider)
// instead of a replacement.
func AddExtensionFlag(cmd *cobra.Command, target *bool) {
	cmd.Flags().BoolVar(target, "extension", false,
		"Add the package as an extension of its base provider, not a replacement")
}

// ExtensionArgs returns the package parameters from the positional args, where
// args[0] is the source.
//
// Without asExtension, every token after the source is returned. With asExtension
// the parameters must come after `--`:
//
//	<source> --extension -- <parameter>...
//
// The slot before `--` is reserved for replacement parameters, which can't be
// combined with an extension yet, so it's rejected for now.
func ExtensionArgs(cmd *cobra.Command, args []string, asExtension bool) ([]string, error) {
	if !asExtension {
		return args[1:], nil
	}
	// With --extension every parameter goes after `--`; nothing may sit between the
	// provider and the `--`. (That slot is reserved for replacement parameters, which
	// can't be combined with an extension yet.)
	paramsMustFollowDash := errors.New("with --extension, parameters must come after '--', as in: " +
		"'<provider> --extension -- <parameter>...'",
	)
	split := cmd.ArgsLenAtDash()
	if split < 0 {
		// ArgsLenAtDash reports -1 when there is no `--`.
		if len(args) > 1 {
			return nil, paramsMustFollowDash
		}
		return args[1:], nil
	}
	if split < 1 {
		split = 1
	}
	if base := args[1:split]; len(base) > 0 {
		return nil, paramsMustFollowDash
	}
	return args[split:], nil
}
