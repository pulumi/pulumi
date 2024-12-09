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
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newConfigSetAllCmd(stack *string) *cobra.Command {
	var plaintextArgs []string
	var secretArgs []string
	var path bool

	setCmd := &cobra.Command{
		Use:   "set-all --plaintext key1=value1 --plaintext key2=value2 --secret key3=value3",
		Short: "Set multiple configuration values",
		Long: "pulumi set-all allows you to set multiple configuration values in one command.\n\n" +
			"Each key-value pair must be preceded by either the `--secret` or the `--plaintext` flag to denote whether \n" +
			"it should be encrypted:\n\n" +
			"  - `pulumi config set-all --secret key1=value1 --plaintext key2=value --secret key3=value3`\n\n" +
			"The `--path` flag can be used to set values inside a map or list:\n\n" +
			"  - `pulumi config set-all --path --plaintext \"names[0]\"=a --plaintext \"names[1]\"=b` \n" +
			"    will set the value to a list with the first item `a` and second item `b`.\n" +
			"  - `pulumi config set-all --path --plaintext parent.nested=value --plaintext parent.other=value2` \n" +
			"    will set the value of `parent` to a map `{nested: value, other: value2}`.\n" +
			"  - `pulumi config set-all --path --plaintext '[\"parent.name\"].[\"nested.name\"]'=value` will set the \n" +
			"    value of `parent.name` to a map `nested.name: value`.",
		Args: cmdutil.NoArgs,
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
			stack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.OfferNew,
				opts,
			)
			if err != nil {
				return err
			}

			ps, err := cmdStack.LoadProjectStack(project, stack)
			if err != nil {
				return err
			}

			for _, ptArg := range plaintextArgs {
				key, value, err := parseKeyValuePair(ptArg)
				if err != nil {
					return err
				}
				v := config.NewValue(value)

				err = ps.Config.Set(key, v, path)
				if err != nil {
					return err
				}
			}

			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()

			for _, sArg := range secretArgs {
				key, value, err := parseKeyValuePair(sArg)
				if err != nil {
					return err
				}
				// We're always going to save, so can ignore the bool for if getStackEncrypter changed the
				// config data.
				c, _, cerr := ssml.GetEncrypter(ctx, stack, ps)
				if cerr != nil {
					return cerr
				}
				enc, eerr := c.EncryptValue(ctx, value)
				if eerr != nil {
					return eerr
				}
				v := config.NewSecureValue(enc)

				err = ps.Config.Set(key, v, path)
				if err != nil {
					return err
				}
			}

			return cmdStack.SaveProjectStack(stack, ps)
		}),
	}

	setCmd.PersistentFlags().BoolVar(
		&path, "path", false,
		"Parse the keys as paths in a map or list rather than raw strings")
	setCmd.PersistentFlags().StringArrayVar(
		&plaintextArgs, "plaintext", []string{},
		"Marks a value as plaintext (unencrypted)")
	setCmd.PersistentFlags().StringArrayVar(
		&secretArgs, "secret", []string{},
		"Marks a value as secret to be encrypted")

	return setCmd
}
