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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newConfigCopyCmd(stack *string) *cobra.Command {
	var path bool
	var destinationStackName string

	cpCommand := &cobra.Command{
		Use:   "cp [key]",
		Short: "Copy config to another stack",
		Long: "Copies the config from the current stack to the destination stack. If `key` is omitted,\n" +
			"then all of the config from the current stack will be copied to the destination stack.",
		Args: cmdutil.MaximumNArgs(1),
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

			// Get current stack and ensure that it is a different stack to the destination stack
			currentStack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				*stack,
				cmdStack.SetCurrent,
				opts,
			)
			if err != nil {
				return err
			}
			if currentStack.Ref().Name().String() == destinationStackName {
				return errors.New("current stack and destination stack are the same")
			}
			currentProjectStack, err := cmdStack.LoadProjectStack(project, currentStack)
			if err != nil {
				return err
			}

			// Get the destination stack
			destinationStack, err := cmdStack.RequireStack(
				ctx,
				ws,
				cmdBackend.DefaultLoginManager,
				destinationStackName,
				cmdStack.LoadOnly,
				opts,
			)
			if err != nil {
				return err
			}
			destinationProjectStack, err := cmdStack.LoadProjectStack(project, destinationStack)
			if err != nil {
				return err
			}

			ssml := cmdStack.NewStackSecretsManagerLoaderFromEnv()

			// Do we need to copy a single value or the entire map
			if len(args) > 0 {
				// A single key was specified so we only need to copy that specific value
				return copySingleConfigKey(
					ctx,
					ssml,
					args[0],
					path,
					currentStack,
					currentProjectStack,
					destinationStack,
					destinationProjectStack,
				)
			}

			requiresSaving, err := cmdStack.CopyEntireConfigMap(
				ctx,
				ssml,
				currentStack,
				currentProjectStack,
				destinationStack,
				destinationProjectStack,
			)
			if err != nil {
				return err
			}

			// The use of `requiresSaving` here ensures that there was actually some config
			// that needed saved, otherwise it's an unnecessary save call
			if requiresSaving {
				err := cmdStack.SaveProjectStack(destinationStack, destinationProjectStack)
				if err != nil {
					return err
				}
			}

			return nil
		}),
	}

	cpCommand.PersistentFlags().BoolVar(
		&path, "path", false,
		"The key contains a path to a property in a map or list to set")
	cpCommand.PersistentFlags().StringVarP(
		&destinationStackName, "dest", "d", "",
		"The name of the new stack to copy the config to")

	return cpCommand
}
