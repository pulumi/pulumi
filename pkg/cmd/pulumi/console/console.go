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

package console

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewConsoleCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var stackName string
	cmd := &cobra.Command{
		Use:   "console",
		Short: "Opens the current stack in the Pulumi Console",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current working directory: %w", err)
			}

			// Try to read the current project
			project, _, err := ws.ReadProject(cwd)
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			currentBackend, err := cmdBackend.CurrentBackend(ctx, ws, lm, project, opts)
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
					if stack == nil {
						fmt.Fprintf(cmd.OutOrStdout(), "Stack '%s' does not exist. "+
							"Run `pulumi stack init` to create a new stack.\n", ref.Name())
						return nil
					}
				} else {
					stack, err = state.CurrentStack(ctx, ws, currentBackend)
					if err != nil {
						return err
					}
					if stack == nil {
						fmt.Fprintln(cmd.OutOrStdout(), "No stack is currently selected. "+
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
				launchConsole(cmd.OutOrStdout(), url)
				return nil
			}
			return errors.New("this command is not available for your backend. " +
				"To migrate to the Pulumi Cloud backend, " +
				"please see https://www.pulumi.com/docs/iac/concepts/state-and-backends/#pulumi-cloud-backend")
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to view",
	)
	return cmd
}

// launchConsole prints the console URL and, in an interactive session, opens it in the browser.
func launchConsole(out io.Writer, url string) {
	fmt.Fprintln(out, url)
	if !cmdutil.Interactive() {
		return
	}
	if openErr := browser.OpenURL(url); openErr != nil {
		fmt.Fprintln(out, "We couldn't launch your web browser for some reason. Please visit the URL above.")
	}
}
