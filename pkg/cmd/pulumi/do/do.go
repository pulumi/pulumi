// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package do

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/shlex"
	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewDoCmd(
	lm cmdBackend.LoginManager, ws pkgWorkspace.Context,
	pluginFromSource func(context.Context, diag.Sink, string, string) (io.Closer, plugin.Provider, error),
) *cobra.Command {
	if pluginFromSource == nil {
		pluginFromSource = func(
			ctx context.Context, sink diag.Sink, wd, source string,
		) (io.Closer, plugin.Provider, error) {
			pctx, err := plugin.NewContext(
				ctx, sink, sink, nil, nil, wd, nil, false,
				nil, schema.NewLoaderServerFromHost)
			if err != nil {
				return nil, nil, fmt.Errorf("create plugin context: %w", err)
			}

			registry := cmdCmd.NewDefaultRegistry(ctx, lm, ws, nil, sink, env.Global())
			p, _, err := packages.ProviderFromSource(pctx, source, registry, env.Global(), 0 /* unbounded concurrency */)
			return pctx, p, err
		}
	}

	var dryrun bool

	cmd := &cobra.Command{
		// Hidden for now while we iterate.
		Hidden: true,
		Use:    "do",
		Short:  "Interact directly with cloud resources",
		Long: `Interact with any cloud

pulumi do dynamically builds a CLI from any Pulumi provider's schema, giving you
direct CRUD access to cloud resources without a Pulumi program or state file.
Each provider plugin contributes its own resources, functions, and
configuration flags, all discoverable via --help on the provider subcommand.

Resource operations: list, create, read, patch, delete
Functions are invoked directly by name.

Provider plugins are auto-installed on first use; you don't need to run
'pulumi plugin install' ahead of time. Run 'pulumi package list' to see what is
installed locally.

Provider configuration can be supplied via:
  - --<provider>-<property> flags (e.g. --aws-native-region us-east-1)
  - the provider's standard environment variables (e.g. AWS_REGION)
  - a YAML file passed with --<provider>-file
  - a Pulumi ESC environment passed with --env/-e (see below)`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// We have to do our own flag parsing because we're dynamically building the CLI from provider schemas.
			// Parse the arguments so far, but don't error on unknown flags since those will be dynamically added by
			// provider plugins.
			flags := cmd.Flags()
			flags.ParseErrorsAllowlist.UnknownFlags = true
			err := flags.Parse(args)
			if errors.Is(err, pflag.ErrHelp) {
				return cmd.Help()
			}
			if err != nil {
				return fmt.Errorf("parse arguments: %w", err)
			}

			pargs := flags.Args()
			// If we don't have any args then this is just `pulumi do` so print help.
			if len(pargs) == 0 {
				return cmd.Help()
			}

			// package may be in the form "name@version" and further may have space separated parameters, e.g.
			// "name@version param1 "multi word param"".
			pkgargs, err := shlex.Split(pargs[0])
			if err != nil {
				return fmt.Errorf("parse package arguments: %w", err)
			}

			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
			sink := diag.DefaultSink(cmd.OutOrStdout(), cmd.ErrOrStderr(), diag.FormatOptions{
				Color: cmdutil.GetGlobalColorization(),
			})

			proj, root, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return fmt.Errorf("read project: %w", err)
			}
			if proj != nil {
				wd = root
			}

			ctx := cmd.Context()
			pctx, p, err := pluginFromSource(ctx, sink, wd, pkgargs[0])
			if err != nil {
				return fmt.Errorf("load provider: %w", err)
			}
			defer p.Close()
			defer pctx.Close()

			var schemaRequest plugin.GetSchemaRequest
			if len(pkgargs) > 1 {
				resp, err := p.Parameterize(ctx, plugin.ParameterizeRequest{
					Parameters: &plugin.ParameterizeArgs{Args: pkgargs[1:]},
				})
				if err != nil {
					return fmt.Errorf("parameterize provider: %w", err)
				}
				schemaRequest.SubpackageName = resp.Name
				schemaRequest.SubpackageVersion = &resp.Version
			}

			getSchema, err := p.GetSchema(ctx, schemaRequest)
			if err != nil {
				return fmt.Errorf("get schema: %w", err)
			}
			var spec schema.PackageSpec
			err = json.Unmarshal(getSchema.Schema, &spec)
			if err != nil {
				return fmt.Errorf("unmarshal schema: %w", err)
			}

			boundpkg, err := packages.BindSpec(spec)
			if err != nil {
				return fmt.Errorf("bind schema: %w", err)
			}

			subcmd := newPackageCommand(dryrun, pargs, p, boundpkg)
			subcmd.SetOut(cmd.OutOrStdout())
			subcmd.SetErr(cmd.ErrOrStderr())
			subcmd.SetIn(cmd.InOrStdin())
			subcmd.SetArgs(args)
			cmd.AddCommand(subcmd)
			return subcmd.ExecuteContext(ctx)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.UnrestrictedArgs)

	cmd.PersistentFlags().BoolVar(&dryrun, "dry-run", false, "Run the operation in preview mode.")

	return cmd
}

func newPackageCommand(dryrun bool, args []string, p plugin.Provider, spec *schema.Package) *cobra.Command {
	cmd := &cobra.Command{
		Use:   args[0],
		Short: fmt.Sprintf("Interact with %s resources and functions", spec.Name),
		Long: fmt.Sprintf(
			"Interact with %s resources and functions.\n\nRun 'pulumi do %s <module/resource/function> --help' for more details on usage.",
			spec.Name, strings.Join(args, " ")),
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddGroup(&cobra.Group{
		ID:    "Functions",
		Title: "Functions",
	})
	cmd.AddGroup(&cobra.Group{
		ID:    "Resources",
		Title: "Resources",
	})
	cmd.AddGroup(&cobra.Group{
		ID:    "Modules",
		Title: "Modules",
	})

	moduleCommands := map[string]*cobra.Command{}

	for _, fn := range spec.Functions {
		mod := ensureModuleCommand(args, cmd, moduleCommands, spec.TokenToModule(fn.Token))
		mod.AddCommand(newFunctionCommand(dryrun, p, fn))
	}

	for _, fn := range spec.Resources {
		mod := ensureModuleCommand(args, cmd, moduleCommands, spec.TokenToModule(fn.Token))
		mod.AddCommand(newResourceCommand(args, p, fn))
	}

	return cmd
}

func ensureModuleCommand(args []string, providerCmd *cobra.Command, cmds map[string]*cobra.Command, mod string) *cobra.Command {
	if mod == "" {
		return providerCmd
	}
	cmd, ok := cmds[mod]
	if !ok {
		shorthelp := fmt.Sprintf("Functions and resources for the %s module", mod)
		longhelp := fmt.Sprintf("%s.\n\nRun 'pulumi do %s <resource/function> --help' for more details on usage.",
			shorthelp, strings.Join(args, " "))

		cmd = &cobra.Command{
			Use:     mod,
			GroupID: "Modules",
			Short:   shorthelp,
			Long:    longhelp,
			Args:    cobra.NoArgs,
		}
		cmd.AddGroup(&cobra.Group{
			ID:    "Functions",
			Title: "Functions",
		})
		cmd.AddGroup(&cobra.Group{
			ID:    "Resources",
			Title: "Resources",
		})
		cmds[mod] = cmd
		providerCmd.AddCommand(cmd)
	}
	return cmd
}

func addFlag(cmd *cobra.Command, param *schema.Property) {
	var flagfn func(string, string)

	typ := param.Type
	if opt, ok := typ.(*schema.OptionalType); ok {
		typ = opt.ElementType
	}

	switch typ {
	case schema.StringType:
		flagfn = func(s1, s2 string) {
			cmd.Flags().String(s1, "", s2)
		}
	case schema.NumberType:
		flagfn = func(s1, s2 string) {
			cmd.Flags().Float64(s1, 0, s2)
		}
	case schema.BoolType:
		flagfn = func(s1, s2 string) {
			cmd.Flags().Bool(s1, false, s2)
		}
	default:
		return
	}

	flagfn(param.Name, param.Comment)
}

// jsonifyPropertyMap converts a PropertyMap to a JSON string for display purposes. This strips things like
// secrets and outputs down to their underlying values.
func jsonifyPropertyMap(props resource.PropertyMap) (string, error) {
	plain := make(map[string]any)
	for k, v := range props {
		key := string(k)
		if v.IsSecret() || (v.IsOutput() && v.OutputValue().Secret) {
			plain[key] = "[secret]"
			continue
		}

		if v.IsComputed() || (v.IsOutput() && !v.OutputValue().Known) {
			plain[key] = "<unknown>"
			continue
		}

		if v.IsOutput() {
			v = v.OutputValue().Element
		}

		plain[string(k)] = v.V
	}

	json, err := ui.MakeJSONString(plain, true)
	if err != nil {
		return "", err
	}
	return json, nil
}

func newFunctionCommand(dryrun bool, p plugin.Provider, fn *schema.Function) *cobra.Command {
	_, _, name, diags := pcl.DecomposeToken(fn.Token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "token should decompose")

	shorthelp := fmt.Sprintf("Invoke the %s function", name)
	longhelp := shorthelp + "."
	if fn.Comment != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, fn.Comment)
	}

	cmd := &cobra.Command{
		Use:     name,
		GroupID: "Functions",
		Short:   shorthelp,
		Long:    longhelp,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Pull all the flags into a map[string]any to pass to the provider plugin.
			inputs := resource.PropertyMap{}
			if fn.Inputs != nil {
				for _, param := range fn.Inputs.Properties {
					name := resource.PropertyKey(param.Name)
					flag := cmd.Flags().Lookup(param.Name)
					if flag != nil {
						typ := param.Type
						opt, isOptional := typ.(*schema.OptionalType)
						if isOptional {
							typ = opt.ElementType
						}

						if !isOptional && !flag.Changed {
							return fmt.Errorf("missing required parameter --%s", param.Name)
						}

						if flag.Changed {
							switch typ {
							case schema.StringType:
								inputs[name] = resource.NewProperty(flag.Value.String())
							case schema.NumberType:
								v, err := cmd.Flags().GetFloat64(param.Name)
								contract.AssertNoErrorf(err, "expected float64 flag")
								inputs[name] = resource.NewProperty(v)
							case schema.BoolType:
								v, err := cmd.Flags().GetBool(param.Name)
								contract.AssertNoErrorf(err, "expected bool flag")
								inputs[name] = resource.NewProperty(v)
							default:
								contract.Failf("%v not yet supported", typ)
							}
						}
					}
				}
			}

			response, err := p.Invoke(cmd.Context(), plugin.InvokeRequest{
				Tok:     tokens.ModuleMember(fn.Token),
				Args:    inputs,
				Preview: dryrun,
			})
			if err != nil {
				return err
			}

			// Print the response as JSON to stdout.
			outputs, err := jsonifyPropertyMap(response.Properties)
			if err != nil {
				return fmt.Errorf("failed to convert outputs to JSON: %w", err)
			}

			fmt.Fprint(cmd.OutOrStdout(), outputs)
			return nil
		},
	}

	// For each top-level basic parameter add a flag to the function command.
	if fn.Inputs != nil {
		for _, param := range fn.Inputs.Properties {
			addFlag(cmd, param)
		}
	}

	return cmd
}

func newResourceCommand(args []string, p plugin.Provider, fn *schema.Resource) *cobra.Command {
	_, _, name, diags := pcl.DecomposeToken(fn.Token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "token should decompose")

	shorthelp := fmt.Sprintf("Operate on the %s resource", name)
	longhelp := shorthelp + "."
	if fn.Comment != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, fn.Comment)
	}

	cmd := &cobra.Command{
		Use:     name,
		GroupID: "Resources",
		Short:   shorthelp,
		Long:    longhelp,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("resource operations not implemented yet")
		},
	}

	return cmd
}
