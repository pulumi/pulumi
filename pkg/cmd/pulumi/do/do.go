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
	"slices"
	"strings"

	"github.com/google/shlex"
	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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
			p, _, err := packages.ProviderFromSource(ws, pctx, source, registry, env.Global(), 0 /* unbounded concurrency */)
			return pctx, p, err
		}
	}

	var dryrun bool
	var showSecrets bool

	// buildSubcommand returns the dynamically constructed subcommand along with a cleanup function that must be
	// deferred by the caller. The cleanup tears down the provider gRPC channel — running it as a defer inside
	// buildSubcommand would close the channel before the subcommand's RunE actually invokes the provider.
	buildSubcommand := func(cmd *cobra.Command, args []string) (*cobra.Command, func(), error) {
		// We have to do our own flag parsing because we're dynamically building the CLI from provider schemas.
		// Parse the arguments so far, but don't error on unknown flags since those will be dynamically added by
		// provider plugins.
		flags := cmd.Flags()
		flags.ParseErrorsAllowlist.UnknownFlags = true
		err := flags.Parse(args)
		// We shouldn't ever see --help here
		contract.Assertf(!errors.Is(err, pflag.ErrHelp), "unexpected --help flag")
		if err != nil {
			return nil, nil, fmt.Errorf("parse arguments: %w", err)
		}

		pargs := flags.Args()
		// If we don't have any args then this is just `pulumi do` so return nil and the caller handle calling help.
		if len(pargs) == 0 {
			return nil, nil, nil
		}

		// package may be in the form "name@version" and further may have space separated parameters, e.g.
		// "name@version param1 "multi word param"".
		pkgargs, err := shlex.Split(pargs[0])
		if err != nil {
			return nil, nil, fmt.Errorf("parse package arguments: %w", err)
		}

		wd, err := os.Getwd()
		if err != nil {
			return nil, nil, fmt.Errorf("get working directory: %w", err)
		}
		sink := diag.DefaultSink(cmd.OutOrStdout(), cmd.ErrOrStderr(), diag.FormatOptions{
			Color: cmdutil.GetGlobalColorization(),
		})

		proj, root, err := ws.ReadProject()
		if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
			return nil, nil, fmt.Errorf("read project: %w", err)
		}
		evalContext := functionEvalContext{
			WorkingDir: wd,
		}
		if proj != nil {
			wd, _, err = (&engine.Projinfo{Proj: proj, Root: root}).GetPwdMain()
			if err != nil {
				return nil, nil, fmt.Errorf("get project working directory: %w", err)
			}
			evalContext = functionEvalContext{
				WorkingDir:    wd,
				ProjectName:   string(proj.Name),
				RootDirectory: root,
			}
		}

		ctx := cmd.Context()
		pctx, p, err := pluginFromSource(ctx, sink, wd, pkgargs[0])
		if err != nil {
			return nil, nil, fmt.Errorf("load provider: %w", err)
		}
		cleanup := func() {
			contract.IgnoreClose(p)
			contract.IgnoreClose(pctx)
		}

		var schemaRequest plugin.GetSchemaRequest
		if len(pkgargs) > 1 {
			resp, err := p.Parameterize(ctx, plugin.ParameterizeRequest{
				Parameters: &plugin.ParameterizeArgs{Args: pkgargs[1:]},
			})
			if err != nil {
				cleanup()
				return nil, nil, fmt.Errorf("parameterize provider: %w", err)
			}
			schemaRequest.SubpackageName = resp.Name
			schemaRequest.SubpackageVersion = &resp.Version
		}

		getSchema, err := p.GetSchema(ctx, schemaRequest)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("get schema: %w", err)
		}
		var spec schema.PackageSpec
		err = json.Unmarshal(getSchema.Schema, &spec)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("unmarshal schema: %w", err)
		}

		boundpkg, err := packages.BindSpec(spec)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("bind schema: %w", err)
		}

		subcmd := (&packageCommand{
			args:        pargs,
			evalContext: evalContext,
			provider:    p,
			spec:        boundpkg,
			dryrun:      dryrun,
			showSecrets: showSecrets,
		}).newCommand()
		subcmd.SetContext(cmd.Context())
		subcmd.SetOut(cmd.OutOrStdout())
		subcmd.SetErr(cmd.ErrOrStderr())
		subcmd.SetIn(cmd.InOrStdin())

		// Build a fake command tree so we get accurate 'Usage' but without re-invoking any hooks. The top fake
		// is what we dispatch on, so its args need to include each intermediate command name so cobra's Find can
		// walk down through the shadow tree to subcmd.
		//
		// The first positional in args might be a shlex-quoted package spec with embedded spaces (e.g.
		// "name@version param1 param2"). cobra's Find matches against command names directly, but the subcmd's
		// Name() is just the first token (e.g. "name@version"). Substitute the quoted spec with its first token
		// in the dispatch args so Find can match.
		fullArgs := slices.Clone(args)
		if pargs[0] != pkgargs[0] {
			for i, a := range fullArgs {
				if a == pargs[0] {
					fullArgs[i] = pkgargs[0]
					break
				}
			}
		}
		parent := cmd
		current := subcmd
		for parent != nil {
			nextParent := &cobra.Command{
				Use: parent.Use,
			}
			nextParent.SetContext(cmd.Context())
			nextParent.SetOut(cmd.OutOrStdout())
			nextParent.SetErr(cmd.ErrOrStderr())
			nextParent.SetIn(cmd.InOrStdin())
			nextParent.AddCommand(current)
			parent.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
				nextParent.Flags().AddFlag(f)
			})
			parent.LocalFlags().VisitAll(func(f *pflag.Flag) {
				if nextParent.Flags().Lookup(f.Name) == nil {
					nextParent.PersistentFlags().AddFlag(f)
				}
			})

			// If the real parent has its own parent, the next iteration will create another fake above this one.
			// That fake will be the dispatch target, so include the current level's name in its args so Find walks
			// through this fake.
			if parent.Parent() != nil {
				fullArgs = append([]string{parent.Name()}, fullArgs...)
			}

			current = nextParent
			parent = parent.Parent()
		}
		current.SetArgs(fullArgs)

		return current, cleanup, nil
	}

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
  - the provider's standard environment variables (e.g. AWS_REGION)
  - an input file passed with --provider-file`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			subcmd, cleanup, err := buildSubcommand(cmd, args)
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}
			if subcmd == nil {
				return cmd.Help()
			}
			ctx := cmd.Context()
			err = subcmd.ExecuteContext(ctx)
			return err
		},
	}

	// Save default help to run if there are no args
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		subcmd, cleanup, err := buildSubcommand(cmd, args)
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			cmd.PrintErrln(err)
			return
		}
		if subcmd == nil {
			defaultHelp(cmd, args)
			return
		}
		err = subcmd.ExecuteContext(cmd.Context())
		if err != nil {
			cmd.PrintErrln(err)
		}
	})

	constrictor.AttachArguments(cmd, constrictor.UnrestrictedArgs)

	cmd.PersistentFlags().BoolVar(&dryrun, "dry-run", false, "Run the operation in preview mode")
	cmd.PersistentFlags().BoolVar(&showSecrets, "show-secrets", false, "Show secret values in output")

	return cmd
}

type packageCommand struct {
	args         []string
	evalContext  functionEvalContext
	provider     plugin.Provider
	providerFile string
	spec         *schema.Package
	dryrun       bool
	showSecrets  bool
}

func (pc *packageCommand) newCommand() *cobra.Command {
	shorthelp := fmt.Sprintf("Interact with %s resources and functions", pc.spec.Name)
	longhelp := shorthelp + "."
	if pc.spec.Description != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, pc.spec.Description)
	}
	longhelp = fmt.Sprintf(
		"%s\n\nRun 'pulumi do %s <module/resource/function> --help' for more details on usage.",
		longhelp, strings.Join(pc.args, " "))

	cmd := &cobra.Command{
		Use:   pc.args[0],
		Short: shorthelp,
		Long:  longhelp,
	}
	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.PersistentFlags().StringVar(
		&pc.providerFile, "provider-file", "", "Path to a file containing provider configuration")

	moduleCommands := map[string]*cobra.Command{}
	moduleCommands[""] = cmd // top-level commands with no module go directly under the package command
	for _, fn := range pc.spec.Functions {
		if pc.spec.TokenToModule(fn.Token) == "" {
			ensureCommandGroup(cmd, "Functions", "Functions")
			break
		}
	}
	for _, res := range pc.spec.Resources {
		if pc.spec.TokenToModule(res.Token) == "" {
			ensureCommandGroup(cmd, "Resources", "Resources")
			break
		}
	}
	for _, fn := range pc.spec.Functions {
		if pc.spec.TokenToModule(fn.Token) != "" {
			ensureCommandGroup(cmd, "Modules", "Modules")
			break
		}
	}
	for _, res := range pc.spec.Resources {
		if pc.spec.TokenToModule(res.Token) != "" {
			ensureCommandGroup(cmd, "Modules", "Modules")
			break
		}
	}

	for _, fn := range pc.spec.Functions {
		// Skip methods for now
		if fn.IsMethod {
			continue
		}

		mod := ensureModuleCommand(pc.args, cmd, moduleCommands, pc.spec.TokenToModule(fn.Token))
		ensureCommandGroup(mod, "Functions", "Functions")
		mod.AddCommand(pc.newFunctionCommand(fn))
	}

	for _, fn := range pc.spec.Resources {
		mod := ensureModuleCommand(pc.args, cmd, moduleCommands, pc.spec.TokenToModule(fn.Token))
		ensureCommandGroup(mod, "Resources", "Resources")
		mod.AddCommand(newResourceCommand(pc.args, pc.provider, &pc.providerFile, fn))
	}

	return cmd
}

func ensureModuleCommand(
	args []string, providerCmd *cobra.Command, cmds map[string]*cobra.Command, mod string,
) *cobra.Command {
	if cmd, ok := cmds[mod]; ok {
		return cmd
	}

	contract.Assertf(mod != "", "module should not be empty")

	if before, modName, found := strings.Cut(mod, "/"); found {
		parent := ensureModuleCommand(args, providerCmd, cmds, before)
		parent.RunE = func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		}
		parent.SetHelpFunc(moduleHelpWithoutChildren)
		ensureNestedModulePadding(providerCmd, cmds)

		cmd := newModuleCommand(args, mod, modName, mod, mod)
		cmds[mod] = cmd
		ensureCommandGroup(providerCmd, "Modules", "Modules")
		providerCmd.AddCommand(cmd)
		return cmd
	} else {
		cmd := newModuleCommand(args, mod, mod, mod, mod)
		cmds[mod] = cmd
		ensureCommandGroup(providerCmd, "Modules", "Modules")
		providerCmd.AddCommand(cmd)
		return cmd
	}
}

func ensureNestedModulePadding(providerCmd *cobra.Command, cmds map[string]*cobra.Command) {
	const key = "__nested_module_padding__"
	if _, ok := cmds[key]; ok {
		return
	}
	cmd := &cobra.Command{
		Use:    "________________",
		Hidden: true,
	}
	cmds[key] = cmd
	providerCmd.AddCommand(cmd)
}

func moduleHelpWithoutChildren(cmd *cobra.Command, args []string) {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintln(out, cmd.Long)
	_, _ = fmt.Fprintf(out, "\nUsage:\n  %s [command]\n\n", cmd.CommandPath())
	if cmd.HasAvailableLocalFlags() {
		_, _ = fmt.Fprintln(out, "Flags:")
		_, _ = fmt.Fprint(out, cmd.LocalFlags().FlagUsages())
		_, _ = fmt.Fprintln(out)
	}
	if cmd.HasAvailableInheritedFlags() {
		_, _ = fmt.Fprintln(out, "Global Flags:")
		_, _ = fmt.Fprint(out, cmd.InheritedFlags().FlagUsages())
		_, _ = fmt.Fprintln(out)
	}
	_, _ = fmt.Fprintf(out, "Use %q for more information about a command.\n", cmd.CommandPath()+" [command] --help")
}

func newModuleCommand(args []string, use, shortDisplayName, longDisplayName, helpPath string) *cobra.Command {
	shorthelp := fmt.Sprintf("Functions and resources for the %s module", shortDisplayName)
	longIntro := fmt.Sprintf("Functions and resources for the %s module", longDisplayName)
	longhelp := fmt.Sprintf("%s.\n\nRun 'pulumi do %s %s <resource/function> --help' for more details on usage.",
		longIntro, args[0], helpPath)

	cmd := &cobra.Command{
		Use:     use,
		GroupID: "Modules",
		Short:   shorthelp,
		Long:    longhelp,
		Args:    cobra.NoArgs,
	}
	return cmd
}

func ensureCommandGroup(cmd *cobra.Command, id, title string) {
	for _, group := range cmd.Groups() {
		if group.ID == id {
			return
		}
	}
	cmd.AddGroup(&cobra.Group{
		ID:    id,
		Title: title,
	})
}

func newResourceCommand(args []string, p plugin.Provider, providerFile *string, fn *schema.Resource) *cobra.Command {
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
			return errors.New("resource operations not implemented yet")
		},
	}

	return cmd
}
