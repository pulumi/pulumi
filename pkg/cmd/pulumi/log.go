// Copyright 2016-2020, Pulumi Corporation.
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
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newLogCmd() *cobra.Command {
	var stackName string
	cmd := &cobra.Command{
		Use:   "log <updateID> <updateKind> <sequenceNumber> <message>",
		Short: "logs a message to a running update",
		Args:  cmdutil.ExactArgs(4),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			updateID := args[0]
			updateKind := args[1]
			sequenceNumber, err := strconv.Atoi(args[2])
			if err != nil {
				return err
			}
			message := args[3]

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			currentBackend, err := currentBackend(opts)
			if err != nil {
				return err
			}

			// Do a type assertion in order to determine if this is a cloud backend based on whether the assertion
			// succeeds or not.
			cloudBackend, isCloud := currentBackend.(httpstate.Backend)
			if isCloud {
				// we only need to inspect the requested stack if we are using a cloud based backend
				var stack backend.Stack
				if stackName != "" {
					ref, err := currentBackend.ParseStackReference(stackName)
					if err != nil {
						return err
					}
					selectedStack, err := currentBackend.GetStack(commandContext(), ref)
					if err != nil {
						return err
					}
					stack = selectedStack
				} else {
					currentStack, err := state.CurrentStack(commandContext(), currentBackend)
					if err != nil {
						return err
					}
					stack = currentStack
				}

				return cloudBackend.Log(context.Background(), stack.Ref(), updateID, updateKind, message, sequenceNumber)

			}
			fmt.Println("This command is not available for your backend. " +
				"To migrate to the Pulumi Service backend, " +
				"please see https://www.pulumi.com/docs/intro/concepts/state/#adopting-the-pulumi-service-backend")
			return nil
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to log to")
	return cmd
}
