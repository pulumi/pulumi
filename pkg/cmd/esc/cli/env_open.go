// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newEnvOpenCmd(envcmd *envCommand) *cobra.Command {
	var duration time.Duration
	var format string

	cmd := &cobra.Command{
		Use:   "open [<org-name>/][<project-name>/]<environment-name>[@<version>] [property path]",
		Args:  cobra.MaximumNArgs(2),
		Short: "Open the environment with the given name.",
		Long: "Open the environment with the given name and return the result\n" +
			"\n" +
			"This command opens the environment with the given name. The result is written to\n" +
			"stdout as JSON. If a property path is specified, only retrieves that property.\n",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := envcmd.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := envcmd.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			_ = args

			var path resource.PropertyPath
			if len(args) == 1 {
				p, err := resource.ParsePropertyPath(args[0])
				if err != nil {
					return fmt.Errorf("invalid property path %v: %w", args[0], err)
				}
				path = p
			}

			switch format {
			case "detailed", "json", "yaml", "string":
				// OK
			case "dotenv", "shell":
				if len(path) != 0 {
					return fmt.Errorf("output format '%s' may not be used with a property path", format)
				}
			default:
				return fmt.Errorf("unknown output format %q", format)
			}

			env, diags, err := envcmd.openEnvironment(ctx, ref, duration)
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				return envcmd.writePropertyEnvironmentDiagnostics(envcmd.esc.stderr, diags)
			}

			return envcmd.renderValue(envcmd.esc.stdout, env, path, format, false, true)
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

func (env *envCommand) renderValue(
	out io.Writer,
	e *esc.Environment,
	path resource.PropertyPath,
	format string,
	pretend bool,
	showSecrets bool,
) error {
	if e == nil {
		return nil
	}

	val := esc.NewValue(e.Properties)
	if len(path) != 0 {
		if vv, ok := getEnvValue(val, path); ok {
			val = *vv
		} else {
			val = esc.Value{}
		}
	}

	switch format {
	case "json":
		body := val.ToJSON(!showSecrets)
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(body)
	case "yaml":
		body := val.ToJSON(!showSecrets)
		enc := yaml.NewEncoder(out)
		enc.SetIndent(3)
		return enc.Encode(body)
	case "detailed":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(val)
	case "dotenv":
		_, environ, _, err := env.prepareEnvironment(e, PrepareOptions{Pretend: pretend, Quote: true, Redact: !showSecrets})
		if err != nil {
			return err
		}
		for _, kvp := range environ {
			fmt.Fprintln(out, kvp)
		}
		return nil
	case "shell":
		_, environ, _, err := env.prepareEnvironment(e, PrepareOptions{Pretend: pretend, Quote: true, Redact: !showSecrets})
		if err != nil {
			return err
		}
		for _, kvp := range environ {
			fmt.Fprintf(out, "export %v\n", kvp)
		}
		return nil
	case "string":
		fmt.Fprintf(out, "%v\n", val.ToString(!showSecrets))
		return nil
	default:
		// NOTE: we shouldn't get here. This was checked at the beginning of the function.
		return fmt.Errorf("unknown output format %q", format)
	}

}

// prepareEnvironment prepares the envvar and temporary file projections for an environment. Returns the paths to
// temporary files, environment variable pairs, and secret values.
func (env *envCommand) prepareEnvironment(e *esc.Environment, opts PrepareOptions) (files, environ, secrets []string, err error) {
	opts.fs = env.esc.fs
	return PrepareEnvironment(e, &opts)
}

func (env *envCommand) removeTemporaryFiles(paths []string) {
	removeTemporaryFiles(env.esc.fs, paths)
}

func (env *envCommand) openEnvironment(
	ctx context.Context,
	ref environmentRef,
	duration time.Duration,
) (*esc.Environment, []client.EnvironmentDiagnostic, error) {
	envID, diags, err := env.esc.client.OpenEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, ref.version, duration)
	if err != nil {
		return nil, nil, err
	}
	if len(diags) != 0 {
		return nil, diags, err
	}
	open, err := env.esc.client.GetOpenEnvironmentWithProject(ctx, ref.orgName, ref.projectName, ref.envName, envID)
	return open, nil, err
}
