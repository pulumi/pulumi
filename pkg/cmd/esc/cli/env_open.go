// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

func newEnvOpenCmd(envcmd *envCommand) *cobra.Command {
	var duration time.Duration
	var format string

	cmd := &cobra.Command{
		Use:   "open [<org-name>/]<environment-name> [property path]",
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

			orgName, envName, args, err := envcmd.getEnvName(args)
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
			case "detailed", "json", "string":
				// OK
			case "dotenv", "shell":
				if len(path) != 0 {
					return fmt.Errorf("output format '%s' may not be used with a property path", format)
				}
			default:
				return fmt.Errorf("unknown output format %q", format)
			}

			env, diags, err := envcmd.openEnvironment(ctx, orgName, envName, duration)
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				return envcmd.writePropertyEnvironmentDiagnostics(envcmd.esc.stderr, diags)
			}

			return envcmd.renderValue(envcmd.esc.stdout, env, path, format, false)
		},
	}

	cmd.Flags().DurationVarP(
		&duration, "lifetime", "l", 2*time.Hour,
		"the lifetime of the opened environment in the form HhMm (e.g. 2h, 1h30m, 15m)")
	cmd.Flags().StringVarP(
		&format, "format", "f", "json",
		"the output format to use. May be 'dotenv', 'json', 'detailed', or 'shell'")

	return cmd
}

func (env *envCommand) renderValue(
	out io.Writer,
	e *esc.Environment,
	path resource.PropertyPath,
	format string,
	pretend bool,
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
		body := val.ToJSON(false)
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(body)
	case "detailed":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(val)
	case "dotenv":
		_, environ, _, err := env.prepareEnvironment(e, prepareOptions{pretend: pretend, quote: true})
		if err != nil {
			return err
		}
		for _, kvp := range environ {
			fmt.Fprintln(out, kvp)
		}
		return nil
	case "shell":
		_, environ, _, err := env.prepareEnvironment(e, prepareOptions{pretend: pretend, quote: true})
		if err != nil {
			return err
		}
		for _, kvp := range environ {
			fmt.Fprintf(out, "export %v\n", kvp)
		}
		return nil
	case "string":
		fmt.Fprintf(out, "%v\n", val.ToString(false))
		return nil
	default:
		// NOTE: we shouldn't get here. This was checked at the beginning of the function.
		return fmt.Errorf("unknown output format %q", format)
	}

}

func getEnvironmentVariables(env *esc.Environment, quote bool) (environ, secrets []string) {
	vars := env.GetEnvironmentVariables()
	keys := maps.Keys(vars)
	sort.Strings(keys)

	for _, k := range keys {
		v := vars[k]
		s := v.Value.(string)

		if v.Secret {
			secrets = append(secrets, s)
		}
		if quote {
			s = strconv.Quote(s)
		}
		environ = append(environ, fmt.Sprintf("%v=%v", k, s))
	}
	return environ, secrets
}

func (env *envCommand) createTemporaryFile(content []byte) (string, error) {
	filename, f, err := env.esc.fs.CreateTemp("", "esc-*")
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(f)

	if _, err = f.Write(content); err != nil {
		contract.IgnoreClose(f)
		rmErr := env.esc.fs.Remove(filename)
		contract.IgnoreError(rmErr)
		return "", err
	}
	return filename, nil
}

func (env *envCommand) createTemporaryFiles(e *esc.Environment, opts prepareOptions) (paths, environ, secrets []string, err error) {
	files := e.GetTemporaryFiles()
	keys := maps.Keys(files)
	sort.Strings(keys)

	for _, k := range keys {
		v := files[k]
		s := v.Value.(string)

		if v.Secret {
			secrets = append(secrets, s)
		}

		path := "[unknown]"
		if !opts.pretend {
			path, err = env.createTemporaryFile([]byte(s))
			if err != nil {
				env.removeTemporaryFiles(paths)
				return nil, nil, nil, err
			}
			paths = append(paths, path)
		}
		if opts.quote {
			path = strconv.Quote(path)
		}
		environ = append(environ, fmt.Sprintf("%v=%v", k, path))
	}
	return paths, environ, secrets, nil
}

func (env *envCommand) removeTemporaryFiles(paths []string) {
	for _, path := range paths {
		err := env.esc.fs.Remove(path)
		contract.IgnoreError(err)
	}
}

// prepareOptions contains options for prepareEnvironment.
type prepareOptions struct {
	quote   bool // True to quote environment variable values
	pretend bool // True to skip actually writing temporary files
}

// prepareEnvironment prepares the envvar and temporary file projections for an environment. Returns the paths to
// temporary files, environment variable pairs, and secret values.
func (env *envCommand) prepareEnvironment(e *esc.Environment, opts prepareOptions) (files, environ, secrets []string, err error) {
	envVars, envSecrets := getEnvironmentVariables(e, opts.quote)

	filePaths, fileVars, fileSecrets, err := env.createTemporaryFiles(e, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating temporary files: %v", err)
	}

	environ = append(envVars, fileVars...)
	secrets = append(envSecrets, fileSecrets...)
	return filePaths, environ, secrets, nil
}

func (env *envCommand) openEnvironment(
	ctx context.Context,
	orgName string,
	envName string,
	duration time.Duration,
) (*esc.Environment, []client.EnvironmentDiagnostic, error) {
	envID, diags, err := env.esc.client.OpenEnvironment(ctx, orgName, envName, duration)
	if err != nil {
		return nil, nil, err
	}
	if len(diags) != 0 {
		return nil, diags, err
	}
	open, err := env.esc.client.GetOpenEnvironment(ctx, orgName, envName, envID)
	return open, nil, err
}
