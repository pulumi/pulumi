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
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/filestate"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newLoginCmd() *cobra.Command {
	var cloudURL string
	var localMode bool

	cmd := &cobra.Command{
		Use:   "login [<url>]",
		Short: "Log into the Pulumi service",
		Long: "Log into the Pulumi service.\n" +
			"\n" +
			"The service manages your stack's state reliably. Simply run\n" +
			"\n" +
			"    $ pulumi login\n" +
			"\n" +
			"and this command will prompt you for an access token, including a way to launch your web browser to\n" +
			"easily obtain one. You can script by using PULUMI_ACCESS_TOKEN environment variable.\n" +
			"\n" +
			"By default, this will log into app.pulumi.com. If you prefer to log into a separate instance\n" +
			"of the Pulumi service, such as Pulumi Enterprise, specify a <url>. For example, run\n" +
			"\n" +
			"    $ pulumi login https://pulumi.acmecorp.com\n" +
			"\n" +
			"to log in to a Pulumi Enterprise server running at the pulumi.acmecorp.com domain.\n" +
			"\n" +
			"For https:// URLs, the CLI will speak REST to a service that manages state and concurrency control.\n" +
			"If you prefer to operate Pulumi independently of a service, and entirely local to your computer,\n" +
			"pass file://<path>, where <path> will be where state checkpoints will be stored. For instance,\n" +
			"\n" +
			"    $ pulumi login file://~\n" +
			"\n" +
			"will store your state information on your computer underneath ~/.pulumi. It is then up to you to\n" +
			"manage this state, including backing it up, using it in a team environment, and so on.\n" +
			"\n" +
			"As a shortcut, you may pass --local to use your home directory (this is an alias for file://~):\n" +
			"\n" +
			"    $ pulumi login --local\n",
		Args: cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			displayOptions := backend.DisplayOptions{
				Color: cmdutil.GetGlobalColorization(),
			}

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

			var be backend.Backend
			var err error
			if filestate.IsLocalBackendURL(cloudURL) {
				be, err = filestate.Login(cmdutil.Diag(), cloudURL)
			} else {
				be, err = httpstate.Login(commandContext(), cmdutil.Diag(), cloudURL, displayOptions)
			}
			if err != nil {
				return errors.Wrapf(err, "problem logging in")
			}

			if currentUser, err := be.CurrentUser(); err == nil {
				fmt.Printf("Logged into %s as %s (%s)\n", be.Name(), currentUser, be.URL())
			} else {
				fmt.Printf("Logged into %s (%s)\n", be.Name(), be.URL())
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(&cloudURL, "cloud-url", "c", "", "A cloud URL to log into")
	cmd.PersistentFlags().BoolVarP(&localMode, "local", "l", false, "Use Pulumi in local-only mode")

	return cmd
}
