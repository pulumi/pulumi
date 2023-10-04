// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

func newEnvOpenCmd(envcmd *envCommand) *cobra.Command {
	var preview bool
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
			case "detailed", "flat", "json", "string":
				// OK
			case "shell":
				if len(path) != 0 {
					return fmt.Errorf("output format 'shell' may not be used with a property path")
				}
			default:
				return fmt.Errorf("unknown output format %q", format)
			}

			var env *esc.Environment
			var diags []client.EnvironmentDiagnostic
			if preview {
				yaml, _, getErr := envcmd.esc.client.GetEnvironment(ctx, orgName, envName)
				if getErr != nil {
					return fmt.Errorf("getting environment: %w", getErr)
				}
				env, diags, err = envcmd.esc.client.CheckYAMLEnvironment(ctx, orgName, yaml)
			} else {
				env, diags, err = envcmd.openEnvironment(ctx, orgName, envName, duration)
			}
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				return envcmd.writePropertyEnvironmentDiagnostics(envcmd.esc.stderr, diags)
			}

			val := esc.NewValue(env.Properties)
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
				enc := json.NewEncoder(envcmd.esc.stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(body)
			case "detailed":
				enc := json.NewEncoder(envcmd.esc.stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(val)
			case "flat":
				var rows []cmdutil.TableRow
				visitor := func(path resource.PropertyPath, v esc.Value) error {
					rows = append(rows, cmdutil.TableRow{Columns: []string{
						path.String(),
						fmt.Sprintf("%v", v.Value),
						fmt.Sprintf("%v:%v:%v", v.Trace.Def.Environment, v.Trace.Def.Begin.Line, v.Trace.Def.Begin.Column),
					}})
					return nil
				}
				err = visitLeafValues(nil, val, visitor)
				contract.IgnoreError(err)

				cmdutil.PrintTable(cmdutil.Table{
					Headers: []string{"PATH", "VALUE", "DEFINITION"},
					Rows:    rows,
				})
				return nil
			case "shell":
				vars, ok := env.Properties["environmentVariables"].Value.(map[string]esc.Value)
				if !ok {
					return nil
				}
				for k, v := range vars {
					if strValue, ok := v.Value.(string); ok {
						fmt.Printf("export %v=%q\n", k, strValue)
					}
				}
				return nil
			case "string":
				fmt.Printf("%v\n", val.ToString(false))
				return nil
			default:
				// NOTE: we shouldn't get here. This was checked at the beginning of the function.
				return fmt.Errorf("unknown output format %q", format)
			}
		},
	}

	cmd.Flags().DurationVarP(
		&duration, "lifetime", "l", 2*time.Hour,
		"the lifetime of the opened environment in the form HhMm (e.g. 2h, 1h30m, 15m)")
	cmd.Flags().StringVarP(
		&format, "format", "f", "json",
		"the output format to use. May be 'flat', 'json', 'detailed', or 'shell'")
	cmd.Flags().BoolVarP(
		&preview, "preview", "p", false,
		"true to preview the opened environment")

	return cmd
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
	open, err := env.esc.client.GetOpenEnvironment(ctx, envID)
	return open, nil, err
}

func visitLeafValues(
	path resource.PropertyPath,
	v esc.Value,
	visitor func(path resource.PropertyPath, v esc.Value) error,
) error {
	switch vv := v.Value.(type) {
	case []esc.Value:
		for i, v := range vv {
			if err := visitLeafValues(append(path, i), v, visitor); err != nil {
				return err
			}
		}
		return nil
	case map[string]esc.Value:
		keys := maps.Keys(vv)
		sort.Strings(keys)
		for _, k := range keys {
			if err := visitLeafValues(append(path, k), vv[k], visitor); err != nil {
				return err
			}
		}
		return nil
	default:
		if err := visitor(path, v); err != nil {
			return fmt.Errorf("%v: %w", path, err)
		}
		return nil
	}
}
