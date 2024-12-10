// Copyright 2016-2024, Pulumi Corporation.
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
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

// Resets the currently selected stack from the current workspace such that
// next time the users get prompted with a stack to select
func newStackUnselectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unselect",
		Short: "Resets stack selection from the current workspace",
		Long: "Resets stack selection from the current workspace.\n" +
			"\n" +
			"This way, next time pulumi needs to execute an operation, the user is prompted with one of the stacks to select\n" +
			"from.\n",
		Args: cmdutil.NoArgs,
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			currentWorkspace, err := workspace.New()
			if err != nil {
				return err
			}

			settings := currentWorkspace.Settings()
			if settings.Stack != "" {
				// a stack is selected
				// reset it
				settings.Stack = ""
				err = currentWorkspace.Save()
				if err == nil {
					// saving changes was successful
					fmt.Println("Stack was unselected")
				} else {
					fmt.Printf("Could not unselect the current stack from the workspace: %s", err)
				}

				return err
			}

			return nil
		}),
	}
}
