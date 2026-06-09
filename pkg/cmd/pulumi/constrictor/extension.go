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
		"Treat the package as an extension of the base provider rather than a replacement")
}

// ExtensionArgs returns the provider parameters to apply, given the command's
// positional args (args[0] is the package source).
//
// With asExtension, the parameters are split at `--`: tokens before it
// parameterize the base provider (replacement), tokens after it are the
// extension's. Combining the two in one invocation is not yet supported, so a
// non-empty base errors. Without asExtension, every token after the source is
// returned unchanged.
func ExtensionArgs(cmd *cobra.Command, args []string, asExtension bool) ([]string, error) {
	if !asExtension {
		return args[1:], nil
	}
	split := cmd.ArgsLenAtDash()
	if split < 1 {
		split = 1
	}
	if base := args[1:split]; len(base) > 0 {
		return nil, fmt.Errorf("passing base (replacement) parameters together with " +
			"an extension is not yet supported")
	}
	return args[split:], nil
}
