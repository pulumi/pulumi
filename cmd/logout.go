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

	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newLogoutCmd() *cobra.Command {
	var cloudURL string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of the Pulumi Cloud",
		Long:  "Log out of the Pulumi Cloud.  Deletes stored credentials on the local machine.",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if cloudURL == "" {
				creds, err := workspace.GetStoredCredentials()
				if err != nil {
					return errors.Wrap(err, "could not determine current cloud")
				}

				cloudURL = creds.Current
			}

			if local.IsLocalBackendURL(cloudURL) {
				return local.New(cmdutil.Diag(), cloudURL).Logout()
			}

			b, err := cloud.New(cmdutil.Diag(), cloudURL)
			if err != nil {
				return err
			}
			return b.Logout()
		}),
	}
	cmd.PersistentFlags().StringVarP(&cloudURL, "cloud-url", "c", "",
		"A cloud URL to log out of (defaults to current cloud)")
	return cmd
}
