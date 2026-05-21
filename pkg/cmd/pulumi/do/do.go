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
	"os"
	"slices"
	"strings"

	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdConvert "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/convert"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

func NewDoCmd(
	lm cmdBackend.LoginManager, ws pkgWorkspace.Context,
	pluginFromSource func(context.Context, *plugin.Context, string, string) (plugin.Provider, error),
	newHost func() (plugin.Host, error),
	loadConverterPlugin func(
		*plugin.Context, string, func(sev diag.Severity, msg string),
	) (plugin.Converter, error),
) *cobra.Command {
	if pluginFromSource == nil {
		pluginFromSource = func(
			ctx context.Context,
			pctx *plugin.Context, wd, source string,
		) (plugin.Provider, error) {
			registry := cmdCmd.NewDefaultRegistry(ctx, lm, ws, nil, pctx.Diag, env.Global())
			p, _, err := packages.ProviderFromSource(ws, pctx, source, registry, env.Global(), 0 /* unbounded concurrency */)
			return p, err
		}
	}
	if newHost == nil {
		newHost = func() (plugin.Host, error) {
			return nil, nil
		}
	}
	if loadConverterPlugin == nil {
		loadConverterPlugin = cmdConvert.LoadConverterPlugin
	}

	var pkg string
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
		// If we don't have any args and no package then this is just `pulumi do` so return nil and the caller handle
		// calling help.
		if len(pargs) == 0 && pkg == "" {
			return nil, nil, nil
		}

		// If --package was passed use that, else set it based on the token
		if pkg == "" {
			pkg, _, _ = strings.Cut(pargs[0], ":")
		}

		// package may be in the form "name@version" and further may have space separated parameters, e.g.
		// "name@version param1 \"multi word param\"".
		pkgargs, err := shlex.Split(pkg)
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

		host, err := newHost()
		if err != nil {
			return nil, nil, fmt.Errorf("create plugin host: %w", err)
		}

		pctx, err := plugin.NewContext(
			ctx, sink, sink, host, nil, wd, nil, false,
			nil, schema.NewLoaderServerFromHost)
		if err != nil {
			return nil, nil, fmt.Errorf("create plugin context: %w", err)
		}

		p, err := pluginFromSource(ctx, pctx, wd, pkgargs[0])
		if err != nil {
			// Close the plugin context we opened above since we're not returning it to the caller.
			contract.IgnoreClose(pctx)
			return nil, nil, fmt.Errorf("load provider: %w", err)
		}
		cleanup := func() {
			contract.IgnoreClose(p)
			contract.IgnoreClose(pctx)
		}

		// Parse "name@version" out of pkgargs[0] so we can also describe the package to downstream consumers
		// (such as snippet converters) in the form the codegen loader expects.
		pkgName := pkgargs[0]
		var pkgVersion string
		if at := strings.Index(pkgName, "@"); at != -1 {
			pkgVersion = pkgName[at+1:]
			pkgName = pkgName[:at]
		}
		packageDescriptor := &codegenrpc.GetSchemaRequest{
			Package: pkgName,
			Version: pkgVersion,
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
			packageDescriptor.Parameterization = &codegenrpc.Parameterization{
				Name:    resp.Name,
				Version: resp.Version.String(),
			}
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

		// If this is a parameterized package, we need to pull of the parameter value from the spec.
		if spec.Parameterization != nil {
			if packageDescriptor.Parameterization == nil {
				return nil, nil, errors.New("provider returned parameterization but no parameterization args were sent")
			}
			packageDescriptor.Parameterization.Value = spec.Parameterization.Parameter
		}

		boundpkg, err := packages.BindSpec(spec)
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("bind schema: %w", err)
		}

		loadConverter := func(name string) (plugin.Converter, error) {
			log := func(sev diag.Severity, msg string) {
				pctx.Diag.Logf(sev, diag.RawMessage("", msg))
			}
			return loadConverterPlugin(pctx, name, log)
		}

		subcmd := (&packageCommand{
			pkg:               pkg,
			args:              pargs,
			evalContext:       evalContext,
			converter:         loadConverter,
			loaderTarget:      pctx.Host.LoaderAddr(),
			packageDescriptor: packageDescriptor,
			provider:          p,
			spec:              boundpkg,
			dryrun:            dryrun,
			showSecrets:       showSecrets,
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
		// pargs[0] (when present) is the token that identifies the subcmd; we've already consumed it to pick the
		// right command, so strip it from the args we hand to cobra. Otherwise cobra would see it as an unknown
		// subcommand and stop dispatching before reaching the operation (create/read/...).
		if len(pargs) > 0 {
			for i, a := range fullArgs {
				if a == pargs[0] {
					fullArgs = slices.Delete(fullArgs, i, i+1)
					break
				}
			}
		}
		// Copy the flags from the `do` command to this new subcommand
		cmd.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			subcmd.Flags().AddFlag(f)
		})
		cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if subcmd.Flags().Lookup(f.Name) == nil {
				subcmd.PersistentFlags().AddFlag(f)
			}
		})
		parent := cmd.Parent()
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

			// We added current as a child of nextParent. To dispatch from nextParent (or any further synthetic
			// ancestor) down to subcmd, cobra's Find needs current.Name() at the front of the args so it consumes
			// it as it descends. Each iteration prepends the level below it.
			fullArgs = append([]string{current.Name()}, fullArgs...)

			current = nextParent
			parent = parent.Parent()
		}
		current.SilenceErrors = true
		current.SilenceUsage = true
		current.SetArgs(fullArgs)

		return current, cleanup, nil
	}

	cmd := &cobra.Command{
		// Hidden for now while we iterate.
		Hidden: true,
		Use:    "do <pkg:mod:typ> [command]",
		Short:  "Interact directly with cloud resources",
		Long: `Interact with any cloud

pulumi do dynamically builds a CLI from any Pulumi provider's schema, giving you
direct CRUD access to cloud resources without a Pulumi program or state file.
Each provider plugin contributes its own resources, functions, and
configuration flags, all discoverable via --help on the provider subcommand.

package will be inferred from the token or passed via --package which can be a
package name or the path to a plugin binary or folder. Further parameters can
be passed after the package name which will be used to parameterize the plugin
loaded.
e.g. pulumi do --package "name@version param1 \"multi word param\"" 

Resource operations: list, create, read, patch, delete
Functions are invoked directly by name.

Provider plugins are auto-installed on first use; you don't need to run
'pulumi plugin install' ahead of time. Run 'pulumi plugin list' to see what is
installed locally.

Provider configuration can be supplied via:
  - the provider's standard environment variables (e.g. AWS_REGION)
  - an input file passed with --provider-file (PCL by default;
    set --provider-format to convert from another format)

Function inputs come from --input-file. PCL is the default; pass --input
to convert from another format such as YAML. Non-PCL formats require a
converter plugin for that format to be installed.`,
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

	cmd.PersistentFlags().BoolVar(&dryrun, "dry-run", false, "Run the operation in preview mode")
	cmd.PersistentFlags().BoolVar(&showSecrets, "show-secrets", false, "Show secret values in output")
	cmd.PersistentFlags().StringVar(
		&pkg, "package", "", "The package to load, in the form 'name@version' or "+
			"a path to a plugin binary or folder. If the package supports "+
			"parameterization, additional space-separated parameters can be "+
			"included after the package name, e.g. --package \"name@version "+
			"param1 \\\"multi word param\\\"\"")

	return cmd
}

type packageCommand struct {
	pkg               string
	args              []string
	evalContext       functionEvalContext
	converter         func(string) (plugin.Converter, error)
	loaderTarget      string
	packageDescriptor *codegenrpc.GetSchemaRequest
	provider          plugin.Provider
	providerFile      string
	providerFormat    string
	spec              *schema.Package
	dryrun            bool
	showSecrets       bool
}

func (pc *packageCommand) newCommand() *cobra.Command {
	// Based on token (if present) we need to work out if this is a package, module, resource, or function command.

	if len(pc.args) == 0 {
		// No token, so this is just the package command.
		return pc.newPackageCommand()
	} else {
		// Count the separators in the token.
		count := strings.Count(pc.args[0], ":")
		if count == 0 {
			// No separators, this is a package
			return pc.newPackageCommand()
		}

		// Try and look it up, it's either a module, resource, or function.
		fun, ok := pc.spec.GetFunction(pc.args[0])
		if ok {
			return pc.newFunctionCommand(fun)
		}
		res, ok := pc.spec.GetResource(pc.args[0])
		if ok {
			return pc.newResourceCommand(res)
		}

		return pc.newModuleCommand()
	}
}

func (pc *packageCommand) newPackageCommand() *cobra.Command {
	shorthelp := fmt.Sprintf("Interact with %s resources and functions", pc.spec.Name)
	longhelp := shorthelp + "."
	if pc.spec.Description != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, pc.spec.Description)
	}

	// If the package can't be inferred from the token then add --package to the help text.
	var flag string
	if len(pc.args) > 0 {
		pkg, _, _ := strings.Cut(pc.args[0], ":")
		if pkg != pc.pkg {
			flag = " --package " + pc.pkg
		}
	} else {
		flag = " --package " + pc.pkg
	}

	longhelp = fmt.Sprintf(
		"%s\n\nRun 'pulumi do%s <module/resource/function> --help' for more details on usage.",
		longhelp, flag)

	modules := map[string]struct{}{}
	functions := map[string]*schema.Function{}
	resources := map[string]*schema.Resource{}
	for _, fn := range pc.spec.Functions {
		if fn.IsMethod {
			continue
		}

		mod := pc.spec.TokenToModule(fn.Token)
		if mod == "" {
			functions[fn.Token] = fn
		} else {
			tok := string(tokens.Token(fn.Token).Package()) + ":" + mod
			modules[tok] = struct{}{}
		}
	}
	for _, res := range pc.spec.Resources {
		mod := pc.spec.TokenToModule(res.Token)
		if mod == "" {
			resources[res.Token] = res
		} else {
			tok := string(tokens.Token(res.Token).Package()) + ":" + mod
			modules[tok] = struct{}{}
		}
	}

	var help strings.Builder
	if len(modules) > 0 {
		fmt.Fprintln(&help, "Modules:")
		for mod := range modules {
			fmt.Fprintf(&help, "  %s\n", mod)
		}
		fmt.Fprintln(&help, "")
	}
	if len(functions) > 0 {
		fmt.Fprintln(&help, "Functions:")
		for _, fn := range functions {
			tok := pc.spec.CanonicalizeToken(fn.Token)
			fmt.Fprintf(&help, "  %s\n", tok)
		}
		fmt.Fprintln(&help, "")
	}
	if len(resources) > 0 {
		fmt.Fprintln(&help, "Resources:")
		for _, res := range resources {
			tok := pc.spec.CanonicalizeToken(res.Token)
			fmt.Fprintf(&help, "  %s\n", tok)
		}
		fmt.Fprintln(&help, "")
	}

	longhelp = fmt.Sprintf("%s\n\n%s", longhelp, help.String())

	use := pc.spec.Name
	if len(pc.args) > 0 {
		use = pc.args[0]
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: shorthelp,
		Long:  longhelp,
		Args:  cobra.NoArgs,
	}

	return cmd
}

func (pc *packageCommand) newModuleCommand() *cobra.Command {
	_, name, _ := strings.Cut(pc.args[0], ":")

	shorthelp := fmt.Sprintf("Functions and resources for the %s module", name)
	longhelp := shorthelp + "."

	// If the package can't be inferred from the token then add --package to the help text.
	var flag string
	if len(pc.args) > 0 {
		pkg, _, _ := strings.Cut(pc.args[0], ":")
		if pkg != pc.pkg {
			flag = " --package " + pc.pkg
		}
	} else {
		flag = " --package " + pc.pkg
	}

	longhelp = fmt.Sprintf(
		"%s\n\nRun 'pulumi do%s <module/resource/function> --help' for more details on usage.",
		longhelp, flag)

	modules := map[string]struct{}{}
	functions := map[string]*schema.Function{}
	resources := map[string]*schema.Resource{}
	for _, fn := range pc.spec.Functions {
		if fn.IsMethod {
			continue
		}

		mod := pc.spec.TokenToModule(fn.Token)
		if mod == name {
			functions[pc.spec.CanonicalizeToken(fn.Token)] = fn
		} else if strings.HasPrefix(mod, name+"/") {
			tok := string(tokens.Token(fn.Token).Package()) + ":" + mod
			modules[tok] = struct{}{}
		}
	}
	for _, res := range pc.spec.Resources {
		mod := pc.spec.TokenToModule(res.Token)
		if mod == name {
			resources[res.Token] = res
		} else if strings.HasPrefix(mod, name+"/") {
			tok := string(tokens.Token(res.Token).Package()) + ":" + mod
			modules[tok] = struct{}{}
		}
	}

	var help strings.Builder
	if len(modules) > 0 {
		fmt.Fprintln(&help, "Modules:")
		for mod := range modules {
			fmt.Fprintf(&help, "  %s\n", mod)
		}
		fmt.Fprintln(&help, "")
	}
	if len(functions) > 0 {
		fmt.Fprintln(&help, "Functions:")
		for _, fn := range functions {
			tok := pc.spec.CanonicalizeToken(fn.Token)
			fmt.Fprintf(&help, "  %s\n", tok)
		}
		fmt.Fprintln(&help, "")
	}
	if len(resources) > 0 {
		fmt.Fprintln(&help, "Resources:")
		for _, res := range resources {
			tok := pc.spec.CanonicalizeToken(res.Token)
			fmt.Fprintf(&help, "  %s\n", tok)
		}
		fmt.Fprintln(&help, "")
	}

	longhelp = fmt.Sprintf("%s\n\n%s", longhelp, help.String())

	use := pc.spec.Name
	if len(pc.args) > 0 {
		use = pc.args[0]
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: shorthelp,
		Long:  longhelp,
		Args:  cobra.NoArgs,
	}

	return cmd
}
