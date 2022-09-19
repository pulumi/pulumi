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
	"fmt"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/shared"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newConsoleCmd() *cobra.Command {
	var stackName string
	cmd := &cobra.Command{
		Use:   "console",
		Short: "Opens the current stack in the Pulumi Console",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}
			currentBackend, err := shared.CurrentBackend(ctx, opts)
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
					stack, err = currentBackend.GetStack(ctx, ref)
					if err != nil {
						return err
					}
				} else {
					stack, err = state.CurrentStack(ctx, currentBackend)
					if err != nil {
						return err
					}
					if stack == nil {
						fmt.Println("No stack is currently selected. " +
							"Run `pulumi stack select` to select a stack.")
						return nil
					}
				}

				// Open the stack specific URL (e.g. app.pulumi.com/{org}/{project}/{stack}) for this
				// stack if a stack is selected and is a cloud stack, else open the cloud backend URL
				// home page, e.g. app.pulumi.com.
				url, err := cloudBackend.StackConsoleURL(stack.Ref())
				if err != nil {
					// Open the cloud backend home page if retrieving the stack
					// console URL fails.
					url = cloudBackend.URL()
				}
				launchConsole(url)
				return nil
			}
			fmt.Println("This command is not available for your backend. " +
				"To migrate to the Pulumi Service backend, " +
				"please see https://www.pulumi.com/docs/intro/concepts/state/#adopting-the-pulumi-service-backend")
			return nil
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to view")
	return cmd
}

// launchConsole attempts to open the console in the browser using the specified URL.
func launchConsole(url string) {
	if openErr := open.Run(url); openErr != nil {
		fmt.Printf("We couldn't launch your web browser for some reason. \n"+
			"Please visit: %s", url)
	}
}
