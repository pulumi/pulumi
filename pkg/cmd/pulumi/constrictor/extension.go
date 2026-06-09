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
	"fmt"

	"github.com/spf13/cobra"
)

// AddExtensionFlag registers the shared --extension flag on cmd, binding it to
// target. Commands that accept a parameterized provider use it to request the
// extension shape (served by the base provider) rather than a replacement.
func AddExtensionFlag(cmd *cobra.Command, target *bool) {
	cmd.Flags().BoolVar(target, "extension", false,
		"Add the package as an extension of its base provider, not a replacement")
}

// ExtensionArgs returns the extension parameters from the command's positional
// args (args[0] is the source).
//
// Without asExtension, every token after the source is returned unchanged.
//
// With asExtension, extension parameters go after `--`:
//
//	<source> --extension -- <extension-parameter>...
//
// The positional slot before `--` is for replacement parameters. Combining
// replacement and extension parameters isn't supported yet, so that slot is
// rejected for now; the source and the command's flags are unaffected.
func ExtensionArgs(cmd *cobra.Command, args []string, asExtension bool) ([]string, error) {
	if !asExtension {
		return args[1:], nil
	}
	// With --extension every parameter goes after `--`; nothing may sit between the
	// provider and the `--`. (That slot is reserved for replacement parameters, which
	// can't be combined with an extension yet.)
	paramsMustFollowDash := fmt.Errorf(
		"with --extension, parameters must come after '--', as in: " +
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
