// Copyright 2016-2023, Pulumi Corporation.
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

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:lll
type StackSelectArgs struct {
	Stack           string `argsShort:"s" argsUsage:"The name of the stack to select"`
	Create          bool   `argsShort:"c" argsUsage:"If selected stack does not exist, create it"`
	SecretsProvider string `argsUsage:"Use with --create flag, The type of the provider that should be used to encrypt and decrypt secrets\n    (possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)" argsDefault:"default"`
}

// newStackSelectCmd handles both the "local" and "cloud" scenarios in its implementation.
func newStackSelectCmd(
	v *viper.Viper,
	parentStackCmd *cobra.Command,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "select [<stack>]",
		Short: "Switch the current workspace to the given stack",
		Long: "Switch the current workspace to the given stack.\n" +
			"\n" +
			"Selecting a stack allows you to use commands like `config`, `preview`, and `update`\n" +
			"without needing to type the stack name each time.\n" +
			"\n" +
			"If no <stack> argument is supplied, you will be prompted to select one interactively.\n" +
			"If provided stack name is not found you may pass the --create flag to create and select it",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cmdArgs []string) error {
			args := UnmarshalArgs[StackSelectArgs](v, cmd)

			ctx := cmd.Context()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Try to read the current project
			project, root, err := readProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			b, err := currentBackend(ctx, project, opts)
			if err != nil {
				return err
			}

			if len(cmdArgs) > 0 {
				if args.Stack != "" {
					return errors.New("only one of --stack or argument stack name may be specified, not both")
				}

				args.Stack = cmdArgs[0]
			}

			if args.Stack != "" {
				// A stack was given, ask the backend about it.
				stackRef, stackErr := b.ParseStackReference(args.Stack)
				if stackErr != nil {
					return stackErr
				}

				s, stackErr := b.GetStack(ctx, stackRef)
				if stackErr != nil {
					return stackErr
				} else if s != nil {
					return state.SetCurrentStack(stackRef.String())
				}
				// If create flag was passed and stack was not found, create it and select it.
				if args.Create && args.Stack != "" {
					s, err := stackInit(ctx, b, args.Stack, root, false, args.SecretsProvider)
					if err != nil {
						return err
					}
					return state.SetCurrentStack(s.Ref().String())
				}

				return fmt.Errorf("no stack named '%s' found", stackRef)
			}

			// If no stack was given, prompt the user to select a name from the available ones.
			stack, err := chooseStack(ctx, b, stackOfferNew|stackSetCurrent, opts)
			if err != nil {
				return err
			}

			contract.Assertf(stack != nil, "must select a stack")
			return state.SetCurrentStack(stack.Ref().String())
		}),
	}

	parentStackCmd.AddCommand(cmd)
	BindFlags[StackSelectArgs](v, cmd)

	return cmd
}
