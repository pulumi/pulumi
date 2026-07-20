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

package stack

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Resets the currently selected stack from the current workspace such that
// next time the users get prompted with a stack to select
func newStackUnselectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unselect",
		Short: "Resets stack selection from the current workspace",
		Long: "Resets stack selection from the current workspace.\n" +
			"\n" +
			"This way, next time pulumi needs to execute an operation, the user is prompted with one of the stacks to select\n" +
			"from.\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := pkgWorkspace.Instance
			proj, _, err := ws.ReadProject("")
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			backendURL, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), proj)
			if err != nil {
				return err
			}

			currentWorkspace, err := ws.New("")
			if err != nil {
				return err
			}

			name, _ := currentWorkspace.Settings().StackForBackend(backendURL)
			if name == "" {
				return nil
			}

			if err := state.SetCurrentStack(ws, backendURL, ""); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Could not unselect the current stack from the workspace: %s", err)
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Stack was unselected")
			return nil
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	return cmd
}
