// Copyright 2025, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newEnvOpenRequestCmd(envcmd *envCommand) *cobra.Command {
	var grantExpiration time.Duration
	var accessDuration time.Duration

	cmd := &cobra.Command{
		Use:   "open-request [<org-name>/][<project-name>/]<environment-name>[@<version>]",
		Args:  cobra.ExactArgs(1),
		Short: "Create a request for opening a protected environment.",
		Long: "Create a request for opening a protected environment with the given name.\n" +
			"\n" +
			"This command creates a request to open a protected environment. The request must be\n" +
			"approved before the environment can be accessed.\n",
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := envcmd.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := envcmd.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}

			resp, err := envcmd.esc.client.CreateEnvironmentOpenRequest(
				ctx,
				ref.orgName,
				ref.projectName,
				ref.envName,
				int(grantExpiration.Seconds()),
				int(accessDuration.Seconds()),
			)
			if err != nil {
				return err
			}

			fmt.Fprintf(envcmd.esc.stdout, "Created environment open request with ID: %s\n", resp.ChangeRequests[0].ChangeRequestID)

			return nil
		},
	}

	cmd.Flags().DurationVar(
		&grantExpiration, "grant-expiration-seconds", 90000*time.Second,
		"expiration time for the grant in seconds")
	cmd.Flags().DurationVar(
		&accessDuration, "access-duration-seconds", 259200*time.Second,
		"duration of access in seconds")

	return cmd
}
