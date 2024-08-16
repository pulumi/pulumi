// Copyright 2016-2018, Pulumi Corporation.
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
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type LogoutArgs struct {
	CloudURL  string `args:"cloud-url" argsShort:"c" argsUsage:"A cloud URL to log out of (defaults to current cloud)"`
	LocalMode bool   `args:"local" argsShort:"l" argsUsage:"Log out of using local mode"`
	All       bool   `args:"all" argsUsage:"Logout of all backends"`
}

func newLogoutCmd(
	v *viper.Viper,
	parentPublicCmd *cobra.Command,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout <url>",
		Short: "Log out of the Pulumi Cloud",
		Long: "Log out of the Pulumi Cloud.\n" +
			"\n" +
			"This command deletes stored credentials on the local machine for a single login.\n" +
			"\n" +
			"Because you may be logged into multiple backends simultaneously, you can optionally pass\n" +
			"a specific URL argument, formatted just as you logged in, to log out of a specific one.\n" +
			"If no URL is provided, you will be logged out of the current backend." +
			"\n\n" +
			"If you would like to log out of all backends simultaneously, you can pass `--all`,\n\n" +
			"    $ pulumi logout --all",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cliArgs []string) error {
			args := UnmarshalArgs[LogoutArgs](v, cmd)

			// If a <cloud> was specified as an argument, use it.
			if len(cliArgs) > 0 {
				if args.CloudURL != "" || args.All {
					return errors.New("only one of --all, --cloud-url or argument URL may be specified, not both")
				}
				args.CloudURL = cliArgs[0]
			}

			// For local mode, store state by default in the user's home directory.
			if args.LocalMode {
				if args.CloudURL != "" {
					return errors.New("a URL may not be specified when --local mode is enabled")
				}
				args.CloudURL = "file://~"
			}

			var err error
			if args.All {
				err = workspace.DeleteAllAccounts()
			} else {
				if args.CloudURL == "" {
					// Try to read the current project
					project, _, err := readProject()
					if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
						return err
					}

					args.CloudURL, err = workspace.GetCurrentCloudURL(project)
					if err != nil {
						return fmt.Errorf("could not determine current cloud: %w", err)
					}

					// Default to the default cloud URL. This means a `pulumi logout` will delete the
					// credentials for pulumi.com if there's no "current" user set in the credentials file.
					args.CloudURL = httpstate.ValueOrDefaultURL(args.CloudURL)
				}

				err = workspace.DeleteAccount(args.CloudURL)
			}
			fmt.Printf("Logged out of %s\n", args.CloudURL)
			return err
		}),
	}

	parentPublicCmd.AddCommand(cmd)
	BindFlags[LogoutArgs](v, cmd)

	return cmd
}
