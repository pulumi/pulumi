// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

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
