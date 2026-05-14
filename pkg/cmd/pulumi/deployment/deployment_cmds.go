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

package deployment

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

// TODO[https://github.com/pulumi/pulumi/issues/22985]: Not yet implemented.
func newDeploymentCancelCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "cancel",
		Short:  "Cancel an in-progress deployment",
		Long: "[EXPERIMENTAL] Cancel an in-progress deployment.\n" +
			"\n" +
			"Canceling a deployment is a dangerous action and may leave the stack in\n" +
			"an inconsistent state if canceled during the execution of a Pulumi operation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not yet implemented")
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "deployment-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}
