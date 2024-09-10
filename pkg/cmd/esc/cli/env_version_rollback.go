// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

func newEnvVersionRollbackCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback [<org-name>/][<project-name>/]<environment-name>@<version>",
		Args:  cobra.ExactArgs(1),
		Short: "Roll back to a specific version",
		Long: "Roll back to a specific version\n" +
			"\n" +
			"This command rolls an environment's definition back to the specified\n" +
			"version. The environment's definition will be replaced with the\n" +
			"definition at that version, creating a new revision.\n",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version == "" {
				return errors.New("please specify a version")
			}
			_ = args

			yaml, _, _, err := env.esc.client.GetEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, ref.version, false)
			if err != nil {
				return err
			}
			diags, err := env.esc.client.UpdateEnvironmentWithProject(ctx, ref.orgName, ref.projectName, ref.envName, yaml, "")
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				err = env.writeYAMLEnvironmentDiagnostics(env.esc.stderr, ref.envName, yaml, diags)
				contract.IgnoreError(err)
				return errors.New("could not roll back: too many errors")
			}
			fmt.Fprintln(env.esc.stdout, "Environment updated.")
			return nil
		},
	}

	return cmd
}
