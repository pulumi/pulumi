// Copyright 2017-2018, Pulumi Corporation.
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
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newLogoutCmd() *cobra.Command {
	var all bool
	var cloudURL string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of the Pulumi Cloud",
		Long:  "Log out of the Pulumi Cloud.  Deletes stored credentials on the local machine.",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// If --all is passed, log out of all clouds.
			if all {
				bes, _, err := cloud.CurrentBackends(cmdutil.Diag())
				if err != nil {
					return errors.Wrap(err, "could not read list of current clouds")
				}
				var result error
				for _, be := range bes {
					if err = cloud.Logout(be.CloudURL()); err != nil {
						result = multierror.Append(result, err)
					}
				}
				return result
			}

			// Otherwise, just log out of a single cloud (either the one specified, or the default).
			return cloud.Logout(cloud.ValueOrDefaultURL(cloudURL))
		}),
	}
	cmd.PersistentFlags().BoolVarP(&all, "all", "a", false, "Log out of all clouds")
	cmd.PersistentFlags().StringVarP(&cloudURL, "cloud-url", "c", "", "A cloud URL to log out of")
	return cmd
}
