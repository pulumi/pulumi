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

package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/filestate"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newLogoutCmd() *cobra.Command {
	var cloudURL string
	var localMode bool

	cmd := &cobra.Command{
		Use:   "logout <url>",
		Short: "Log out of the Pulumi service",
		Long: "Log out of the Pulumi service.\n" +
			"\n" +
			"This command deletes stored credentials on the local machine for a single login.\n" +
			"\n" +
			"Because you may be logged into multiple backends simultaneously, you can optionally pass\n" +
			"a specific URL argument, formatted just as you logged in, to log out of a specific one.\n" +
			"If none is supplied, you will be logged out of the default cloud.",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// If a <cloud> was specified as an argument, use it.
			if len(args) > 0 {
				if cloudURL != "" {
					return errors.New("only one of --cloud-url or argument URL may be specified, not both")
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

			if cloudURL == "" {
				creds, err := workspace.GetStoredCredentials()
				if err != nil {
					return errors.Wrap(err, "could not determine current cloud")
				}

				cloudURL = creds.Current
			}

			var be backend.Backend
			var err error
			if filestate.IsLocalBackendURL(cloudURL) {
				be, err = filestate.New(cmdutil.Diag(), cloudURL)
			} else {
				be, err = httpstate.New(cmdutil.Diag(), cloudURL)
			}
			if err != nil {
				return err
			}
			return be.Logout()
		}),
	}

	cmd.PersistentFlags().StringVarP(&cloudURL, "cloud-url", "c", "",
		"A cloud URL to log out of (defaults to current cloud)")
	cmd.PersistentFlags().BoolVarP(&localMode, "local", "l", false,
		"Log out of using local mode")

	return cmd
}
