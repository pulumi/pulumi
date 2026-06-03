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

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func newLogoutCmd(esc *escCommand) *cobra.Command {
	var backendURL string
	var all bool

	cmd := &cobra.Command{
		Use:   "logout <url>",
		Short: "Log out of the Pulumi Cloud",
		Long: "Log out of the Pulumi Cloud.\n" +
			"\n" +
			"This command deletes stored credentials on the local machine for a single login.\n" +
			"\n" +
			"Because you may be logged into multiple backends simultaneously, you can optionally pass\n" +
			"a specific URL argument, formatted just as you logged in, to log out of a specific one.\n" +
			"If no URL is provided, you will be logged out of the current backend.\n" +
			"\n" +
			"If you would like to log out of all backends simultaneously, you can pass `--all`,\n" +
			"\n" +
			"    $ esc logout --all",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// If a <cloud> was specified as an argument, use it.
			if len(args) > 0 {
				if backendURL != "" || all {
					return errors.New("only one of --all, --cloud-url or argument URL may be specified, not both")
				}
				backendURL = args[0]
			}

			var err error
			if all {
				err = esc.workspace.DeleteAllAccounts()
				fmt.Fprintf(esc.stdout, "Logged out of all clouds\n")
			} else {
				if backendURL == "" {
					account, _, err := esc.workspace.GetCurrentAccount(true)
					if err != nil {
						return fmt.Errorf("getting current account: %w", err)
					}
					if account == nil {
						fmt.Fprintf(esc.stdout, "Already logged out.\n")
						return nil
					}
					backendURL = account.BackendURL
				}
				if err := esc.workspace.DeleteAccount(backendURL); err != nil {
					return err
				}
				fmt.Fprintf(esc.stdout, "Logged out of %s\n", backendURL)
			}
			return err
		},
	}

	cmd.PersistentFlags().BoolVar(&all, "all", false,
		"Logout of all backends")
	cmd.PersistentFlags().StringVarP(&backendURL, "cloud-url", "c", "",
		"A cloud URL to log out of (defaults to current cloud)")

	return cmd
}
