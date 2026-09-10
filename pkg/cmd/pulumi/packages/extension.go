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

package packages

import (
	"errors"
	"fmt"

	"github.com/google/shlex"
	"github.com/spf13/cobra"
)

// AddExtensionFlag registers the shared --extension flag on a package command. It is a
// holds the extension's provider-defined parameters as one shell-quoted string.
//
// PreRunE resolves it: when the flag is set, its value is shlex-split into params and
// isExtension is true; otherwise params defaults to the positional tokens after the
// source and isExtension is false. Combining positional parameters with --extension is
// rejected.
func AddExtensionFlag(cmd *cobra.Command, params *[]string, isExtension *bool) {
	var extension string
	cmd.Flags().StringVar(&extension, "extension", "",
		"Add an extension layered onto a base provider rather than a replacement. "+
			"The value is the extension's provider-defined parameters as one shell-quoted "+
			`string, e.g. --extension "key=value ..."`)

	inner := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if inner != nil {
			if err := inner(cmd, args); err != nil {
				return err
			}
		}
		if cmd.Flags().Changed("extension") {
			*isExtension = true
			if len(args) > 1 {
				return errors.New("combining replacement parameters with --extension is not supported yet")
			}
			toks, err := shlex.Split(extension)
			if err != nil {
				return fmt.Errorf("parse --extension parameters: %w", err)
			}
			*params = toks
			return nil
		}
		if len(args) > 0 {
			*params = args[1:]
		}
		return nil
	}
}
