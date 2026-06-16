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
	"fmt"

	"github.com/google/shlex"
	"github.com/spf13/cobra"
)

// AddExtensionFlag registers the shared --extension flag. When set, the package is
// added as an extension of its base provider.
func AddExtensionFlag(cmd *cobra.Command) {
	cmd.Flags().String("extension", "",
		"Add the package as an extension of its base provider rather than a replacement. "+
			"The value is the extension's parameters as one shell-quoted string, e.g. "+
			`--extension "-c crd.yaml -n gateway"`)
}

// ExtensionArgs resolves a package command's parameters and whether the package is
// being added as an extension. args[0] is the source.
//
// For a replacement, the parameters are the positional tokens after the source
// (args[1:]); for an extension, they come from the --extension flag value instead.
func ExtensionArgs(cmd *cobra.Command, args []string) (params []string, asExtension bool, err error) {
	replacement := args[1:]
	if !cmd.Flags().Changed("extension") {
		return replacement, false, nil
	}
	if len(replacement) > 0 {
		return nil, true, errors.New(
			"combining replacement parameters with --extension is not supported yet")
	}
	value, err := cmd.Flags().GetString("extension")
	if err != nil {
		return nil, true, err
	}
	params, err = shlex.Split(value)
	if err != nil {
		return nil, true, fmt.Errorf("parse --extension parameters: %w", err)
	}
	return params, true, nil
}
