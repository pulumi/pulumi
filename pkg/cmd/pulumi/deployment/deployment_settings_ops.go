// Copyright 2016, Pulumi Corporation.
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

package deployment

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func verifyInteractiveMode(yes bool) error {
	interactive := cmdutil.Interactive()

	if !interactive && !yes {
		return backenderr.ErrNonInteractiveRequiresYes
	}

	return nil
}

func newDeploymentSettingsDestroyCmd() *cobra.Command {
	var stack string
	var yes bool

	cmd := &cobra.Command{
		Hidden:     true,
		Use:        "destroy",
		Aliases:    []string{"down", "dn", "clear"},
		SuggestFor: []string{"delete", "kill", "remove", "rm", "stop"},
		Short:      "Delete all the stack's deployment settings",
		Long:       "",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := verifyInteractiveMode(yes); err != nil {
				return err
			}

			d, err := initializeDeploymentSettingsCmd(
				cmd.Context(), cmd.OutOrStdout(), pkgWorkspace.Instance, stack)
			if err != nil {
				return err
			}

			confirm := askForConfirmation("This action will clear the stack's deployment settings, "+
				"do you want to continue?", d.DisplayOptions.Color, true, yes)

			if !confirm {
				return nil
			}

			err = d.Backend.DestroyStackDeploymentSettings(ctx, d.Stack)
			if err != nil {
				return err
			}

			return nil
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically confirm every confirmation prompt")

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}
