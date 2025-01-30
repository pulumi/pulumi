// Copyright 2025, Pulumi Corporation.

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/spf13/cobra"
)

func newEnvRotateCmd(envcmd *envCommand) *cobra.Command {
	var duration time.Duration
	var format string

	cmd := &cobra.Command{
		Use:   "rotate [<org-name>/][<project-name>/]<environment-name> [path(s) to rotate]",
		Short: "Rotate secrets and open the environment",
		Long: "Rotate secrets and open the environment\n" +
			"\n" +
			"Optionally accepts any number of Property Paths as additional arguments. If given any paths, will only rotate secrets at those paths.\n" +
			"\n" +
			"This command opens the environment with the given name. The result is written to\n" +
			"stdout as JSON.\n",
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

			switch format {
			case "detailed", "json", "yaml", "string", "dotenv", "shell":
				// OK
			default:
				return fmt.Errorf("unknown output format %q", format)
			}

			env, diags, err := envcmd.rotateEnvironment(ctx, ref, duration, rotationPaths)
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				return envcmd.writePropertyEnvironmentDiagnostics(envcmd.esc.stderr, diags)
			}

			return envcmd.renderValue(envcmd.esc.stdout, env, resource.PropertyPath{}, format, false, true)
		},
	}

	cmd.Flags().DurationVarP(
		&duration, "lifetime", "l", 2*time.Hour,
		"the lifetime of the opened environment in the form HhMm (e.g. 2h, 1h30m, 15m)")
	cmd.Flags().StringVarP(
		&format, "format", "f", "json",
		"the output format to use. May be 'dotenv', 'json', 'yaml', 'detailed', or 'shell'")

	return cmd
}

func (env *envCommand) rotateEnvironment(
	ctx context.Context,
	ref environmentRef,
	duration time.Duration,
	rotationPaths []string,
) (*esc.Environment, []client.EnvironmentDiagnostic, error) {
	envID, diags, err := env.esc.client.RotateEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, duration, rotationPaths)
	if err != nil {
		return nil, nil, err
	}
	if len(diags) != 0 {
		return nil, diags, err
	}
	open, err := env.esc.client.GetOpenEnvironmentWithProject(ctx, ref.orgName, ref.projectName, ref.envName, envID)
	return open, nil, err
}
