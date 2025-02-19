// Copyright 2025, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
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

			resp, diags, err := envcmd.esc.client.RotateEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, rotationPaths)
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				return envcmd.writePropertyEnvironmentDiagnostics(envcmd.esc.stderr, diags)
			}
			if resp == nil {
				return nil
			}

			event := resp.SecretRotationEvent

			// Print result of rotation
			var b strings.Builder
			if event.Status == client.RotationEventSucceeded {
				fmt.Fprintf(&b, "Environment '%s' rotated.\n", args[0])
			} else if event.Status == client.RotationEventFailed {
				if event.ErrorMessage != nil {
					fmt.Fprintf(&b, "%vError rotating: %s.%v\n", colors.SpecError, *event.ErrorMessage, colors.Reset)
				} else {
					fmt.Fprintf(&b, "%vEnvironment '%s' rotated with errors.%v\n", colors.SpecWarning, args[0], colors.Reset)
				}
			}
			if event.PostRotationRevision != nil {
				fmt.Fprintf(&b, "New revision '%d' was created.\n", *event.PostRotationRevision)
			}

			var failedRotations []client.SecretRotation
			for _, rotation := range event.Rotations {
				if rotation.Status == client.RotationFailed {
					failedRotations = append(failedRotations, rotation)
				}
			}
			if len(failedRotations) > 0 {
				fmt.Fprintf(&b, "\n%vFailed secrets:%v\n", colors.SpecError, colors.Reset)
				for _, rotation := range failedRotations {
					fmt.Fprintf(&b, "Path: %s, error: %s\n", rotation.EnvironmentPath, *rotation.ErrorMessage)
				}
			}

			fmt.Fprint(envcmd.esc.stdout, envcmd.esc.colors.Colorize(b.String()))

			return nil
		},
	}

	return cmd
}
