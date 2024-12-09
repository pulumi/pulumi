// Copyright 2024, Pulumi Corporation.
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

package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newConfigSetCmd(stack *string) *cobra.Command {
	var plaintext bool
	var secret bool
	var path bool

	setCmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Set configuration value",
		Long: "Configuration values can be accessed when a stack is being deployed and used to configure behavior. \n" +
			"If a value is not present on the command line, pulumi will prompt for the value. Multi-line values\n" +
			"may be set by piping a file to standard in.\n\n" +
			"The `--path` flag can be used to set a value inside a map or list:\n\n" +
			"  - `pulumi config set --path 'names[0]' a` " +
			"will set the value to a list with the first item `a`.\n" +
			"  - `pulumi config set --path parent.nested value` " +
			"will set the value of `parent` to a map `nested: value`.\n" +
			"  - `pulumi config set --path '[\"parent.name\"][\"nested.name\"]' value` will set the value of \n" +
			"    `parent.name` to a map `nested.name: value`.",
		Args: cmdutil.RangeArgs(1, 2),
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ws := pkgWorkspace.Instance
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			project, _, err := ws.ReadProject()
			if err != nil {
				return err
			}

			// Ensure the stack exists.
			s, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.OfferNew|cmdStack.SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}

			key, err := ParseConfigKey(args[0])
			if err != nil {
				return fmt.Errorf("invalid configuration key: %w", err)
			}

			var value string
			switch {
			case len(args) == 2:
				value = args[1]
			//nolint:gosec // os.Stdin.Fd() == 0: uintptr -> int conversion is always safe
			case !term.IsTerminal(int(os.Stdin.Fd())):
				b, readerr := io.ReadAll(os.Stdin)
				if readerr != nil {
					return readerr
				}
				value = cmdutil.RemoveTrailingNewline(string(b))
			case !cmdutil.Interactive():
				return errors.New("config value must be specified in non-interactive mode")
			case secret:
				value, err = cmdutil.ReadConsoleNoEcho("value")
				if err != nil {
					return err
				}
			default:
				value, err = cmdutil.ReadConsole("value")
				if err != nil {
					return err
				}
			}

			ps, err := cmdStack.LoadProjectStack(project, s)
			if err != nil {
				return err
			}

			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()

			// Encrypt the config value if needed.
			var v config.Value
			if secret {
				// We're always going to save, so can ignore the bool for if getStackEncrypter changed the
				// config data.
				c, _, cerr := ssml.GetEncrypter(ctx, s, ps)
				if cerr != nil {
					return cerr
				}
				enc, eerr := c.EncryptValue(ctx, value)
				if eerr != nil {
					return eerr
				}
				v = config.NewSecureValue(enc)
			} else {
				v = config.NewValue(value)

				// If we saved a plaintext configuration value, and --plaintext was not passed, warn the user.
				if !plaintext && looksLikeSecret(key, value) {
					return fmt.Errorf("config value for '%s' looks like a secret; "+
						"rerun with --secret to encrypt it, or --plaintext if you meant to store in plaintext",
						key)
				}
			}

			err = ps.Config.Set(key, v, path)
			if err != nil {
				return err
			}

			return cmdStack.SaveProjectStack(s, ps)
		}),
	}

	setCmd.PersistentFlags().BoolVar(
		&path, "path", false,
		"The key contains a path to a property in a map or list to set")
	setCmd.PersistentFlags().BoolVar(
		&plaintext, "plaintext", false,
		"Save the value as plaintext (unencrypted)")
	setCmd.PersistentFlags().BoolVar(
		&secret, "secret", false,
		"Encrypt the value instead of storing it in plaintext")

	return setCmd
}
