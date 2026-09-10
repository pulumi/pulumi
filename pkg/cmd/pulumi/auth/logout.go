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

package auth

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewLogoutCmd(ws pkgWorkspace.Context) *cobra.Command {
	var cloudURL string
	var localMode bool
	var all bool

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of a Pulumi state backend",
		Long: "Log out of a Pulumi state backend.\n" +
			"\n" +
			"This command deletes the credentials stored on this machine for a single login. With no\n" +
			"arguments, it logs you out of the current backend:\n" +
			"\n" +
			"    $ pulumi logout\n" +
			"\n" +
			"You can be logged in to several backends at once. To choose one, pass its URL, written the\n" +
			"same way you wrote it when logging in:\n" +
			"\n" +
			"    $ pulumi logout https://api.pulumi.acmecorp.com\n" +
			"    $ pulumi logout s3://my-pulumi-state-bucket\n" +
			"\n" +
			"To log out of every backend at once, pass `--all`:\n" +
			"\n" +
			"    $ pulumi logout --all\n" +
			"\n" +
			"`--local` is a shortcut for `file://~`, matching `pulumi login --local`.\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			logOutOfEverything := func() error {
				if err := deleteAllAccounts(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(),
					"Removed stored credentials that could no longer be decrypted; logged out of everything")
				return nil
			}
			// If a <cloud> was specified as an argument, use it.
			if len(args) > 0 {
				if cloudURL != "" || all {
					return errors.New("only one of --all, --cloud-url or argument URL may be specified, not both")
				}
				cloudURL = args[0]
			}

			// For local mode, store state by default in the user's home directory.
			if localMode {
				if cloudURL != "" {
					return errors.New("a URL may not be specified when --local mode is enabled")
				}
				cloudURL = "file://~"
			}

			var err error
			if all {
				err = deleteAllAccounts()
				fmt.Fprintln(cmd.OutOrStdout(), "Logged out of everything")
			} else {
				if cloudURL == "" {
					cwd, err := os.Getwd()
					if err != nil {
						return fmt.Errorf("getting current working directory: %w", err)
					}

					// Try to read the current project
					project, _, err := ws.ReadProject(cwd)
					if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
						return err
					}

					cloudURL, err = pkgWorkspace.GetCurrentCloudURLWithAgentFallback(ws, env.Global(), project)
					if err != nil {
						// Removing everything does not require reading the file.
						if workspace.IsUndecryptableCredentials(err) {
							return logOutOfEverything()
						}
						return fmt.Errorf("could not determine current cloud: %w", err)
					}

					// Default to the default cloud URL. This means a `pulumi logout` will delete the
					// credentials for pulumi.com if there's no "current" user set in the credentials file.
					cloudURL = httpstate.ValueOrDefaultURL(ws, cloudURL)
				}

				err = deleteAccount(cloudURL)
				if workspace.IsUndecryptableCredentials(err) {
					return logOutOfEverything()
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Logged out of %s\n", cloudURL)
			}

			return err
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "url"},
		},
		Required: 0,
	})

	cmd.PersistentFlags().BoolVar(&all, "all", false,
		"Log out of all backends")
	cmd.PersistentFlags().StringVarP(&cloudURL, "cloud-url", "c", "",
		"A cloud URL to log out of (defaults to the current backend)")
	cmd.PersistentFlags().BoolVarP(&localMode, "local", "l", false,
		"Log out of local-only mode (an alias for `file://~`)")

	return cmd
}

// deleteAllAccounts removes user credentials and, in agent mode, any shared
// temporary agent credentials.
func deleteAllAccounts() error {
	if !workspace.AgentCredentialsFallbackEnabled() {
		return workspace.DeleteAllAccounts()
	}
	if err := workspace.DeleteAllAccounts(); err != nil {
		return workspace.DeleteAgentCredentials()
	}
	return workspace.DeleteAgentCredentials()
}

// deleteAccount removes credentials for a cloud URL, falling back to shared
// temporary agent credentials when default credentials are unavailable.
func deleteAccount(cloudURL string) error {
	if !workspace.AgentCredentialsFallbackEnabled() {
		return workspace.DeleteAccount(cloudURL)
	}
	creds, err := workspace.GetStoredCredentials()
	// Tokenless backends, such as DIY backends, still belong to the default credential store.
	if err == nil && credentialsContainAccount(creds, cloudURL) {
		return workspace.DeleteAccount(cloudURL)
	}
	return workspace.DeleteAgentAccount(cloudURL)
}

func credentialsContainAccount(creds workspace.Credentials, cloudURL string) bool {
	if creds.Current == cloudURL {
		return true
	}
	if _, ok := creds.AccessTokens[cloudURL]; ok {
		return true
	}
	_, ok := creds.Accounts[cloudURL]
	return ok
}
