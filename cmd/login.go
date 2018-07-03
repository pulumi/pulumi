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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

func newLoginCmd() *cobra.Command {
	var cloudURL string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log into the Pulumi Cloud",
		Long:  "Log into the Pulumi Cloud.  You can script by using PULUMI_ACCESS_TOKEN environment variable.",
		Args:  cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			displayOptions := backend.DisplayOptions{
				Color: cmdutil.GetGlobalColorization(),
			}

			var b backend.Backend
			var err error

			if local.IsLocalBackendURL(cloudURL) {
				b, err = local.Login(cmdutil.Diag(), cloudURL)
			} else {
				b, err = cloud.Login(commandContext(), cmdutil.Diag(), cloudURL, displayOptions)
			}

			if err != nil {
				return err
			}

			if currentUser, err := b.CurrentUser(); err == nil {
				fmt.Printf("Logged into %s as %s\n", b.Name(), currentUser)
			} else {
				fmt.Printf("Logged into %s\n", b.Name())
			}

			return nil
		}),
	}
	cmd.PersistentFlags().StringVarP(&cloudURL, "cloud-url", "c", "", "A cloud URL to log into")

	return cmd
}
