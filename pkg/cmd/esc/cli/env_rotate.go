// Copyright 2025, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/spf13/cobra"
)

func newEnvRotateCmd(envcmd *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate [<org-name>/][<project-name>/]<environment-name> [path(s) to rotate]",
		Short: "Rotate secrets in an environment",
		Long: "Rotate secrets in an environment\n" +
			"\n" +
			"Optionally accepts any number of Property Paths as additional arguments. If given any paths, will only rotate secrets at those paths.\n",
		SilenceUsage: true,
		Hidden:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := envcmd.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := envcmd.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}

			if ref.version != "" {
				return fmt.Errorf("the rotate command does not accept environments at specific versions")
			}

			rotationPaths := []string{}
			for _, arg := range args[1:] {
				_, err := resource.ParsePropertyPath(arg)
				if err != nil {
					return fmt.Errorf("'%s' is an invalid property path: %w", arg, err)
				}
				rotationPaths = append(rotationPaths, arg)
			}

			diags, err := envcmd.esc.client.RotateEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, rotationPaths)
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				return envcmd.writePropertyEnvironmentDiagnostics(envcmd.esc.stderr, diags)
			}

			fmt.Fprintf(envcmd.esc.stdout, "Environment '%s' rotated.\n", args[0])
			return nil
		},
	}

	return cmd
}
