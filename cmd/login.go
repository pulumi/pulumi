// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"github.com/spf13/cobra"

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
			if local.IsLocalBackendURL(cloudURL) {
				_, err := local.Login(cmdutil.Diag())
				return err
			}

			_, err := cloud.Login(cmdutil.Diag(), cloudURL)
			return err
		}),
	}
	cmd.PersistentFlags().StringVarP(&cloudURL, "cloud-url", "c", "", "A cloud URL to log into")
	return cmd
}
