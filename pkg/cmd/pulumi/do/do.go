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
	"unicode"

	"github.com/google/shlex"
	"github.com/hashicorp/hcl/v2"
	"github.com/pgavlin/fx/v2/maps"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/texttheater/golang-levenshtein/levenshtein"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdConvert "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/convert"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
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
	newHost func(ctx context.Context, d, statusD diag.Sink) (plugin.Host, error),
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
		newHost = func(ctx context.Context, d, statusD diag.Sink) (plugin.Host, error) {
			// The host is owned by the do command (closed via cleanup), so its lifetime context is
			// uncancellable. Plugin logs route through the command's diagnostics sinks, so a
			// provider's output reaches the command's stdout/stderr the same way it does without a
			// pre-constructed host.
			reg := cmdCmd.NewDefaultRegistry(ctx, lm, ws, nil, d, env.Global())
			return pkghost.New(
				context.WithoutCancel(ctx), d, statusD, nil, pkgWorkspace.EnsureLanguageInstalled,
				schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
				packageworkspace.NewResolverServer(reg))
		}
	}
	if loadConverterPlugin == nil {
		loadConverterPlugin = cmdConvert.LoadConverterPlugin
	}

	var pkg string
	var dryrun bool
	var showSecrets bool
	var stateless bool

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
		// If we're inside a Pulumi project, the working directory the plugin host runs in should be
		// the project's pwd, not whatever the user happened to invoke `pulumi` from. Snapshot that
		// here so plugin.NewContext / pluginFromSource see the project-relative path; the rest of
		// the PCL evaluation state (project name, stack identity, ...) is derived lazily by
		// packageCommand.evalContext().
		if proj != nil {
			wd, _, err = (&engine.Projinfo{Proj: proj, Root: root}).GetPwdMain()
			if err != nil {
				return nil, nil, fmt.Errorf("get project working directory: %w", err)
			}
		}

		ctx := cmd.Context()

		host, err := newHost(ctx, sink, sink)
		if err != nil {
			return nil, nil, fmt.Errorf("create plugin host: %w", err)
		}

		pctx, err := plugin.NewContext(
			ctx, sink, sink, host, nil, wd, nil, false,
			nil)
		if err != nil {
			contract.IgnoreClose(host)
			return nil, nil, fmt.Errorf("create plugin context: %w", err)
		}

		p, err := pluginFromSource(ctx, pctx, wd, pkgargs[0])
		if err != nil {
			// Close the plugin context we opened above since we're not returning it to the caller.
			contract.IgnoreClose(pctx)
			contract.IgnoreClose(host)
			return nil, nil, fmt.Errorf("load provider: %w", err)
		}
		cleanup := func() {
			contract.IgnoreClose(p)
			contract.IgnoreClose(pctx)
			// host is owned here, closed after the context
			contract.IgnoreClose(host)
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

		boundpkg, err := packages.BindSpec(spec, schema.NewPluginLoader(pctx))
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

		subcmd, err := (&packageCommand{
			pkg:               pkg,
			args:              pargs,
			converter:         loadConverter,
			loaderTarget:      pctx.LoaderAddr(),
			packageDescriptor: packageDescriptor,
			provider:          p,
			spec:              boundpkg,
			dryrun:            dryrun,
			showSecrets:       showSecrets,
			stateless:         stateless,
			wd:                wd,
			proj:              proj,
			root:              root,
			ws:                ws,
			lm:                lm,
			sink:              sink,
		}).newCommand()
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		// Replace the short name in Use with the full token so the usage
		// string shows e.g. "pulumi do aws:s3:Bucket" instead of "pulumi do Bucket".
		if len(pargs) > 0 {
			subcmd.Use = pargs[0] + strings.TrimPrefix(subcmd.Use, subcmd.Name())
		}

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
		// Insert a fake "do" node so the command path reads "pulumi do <token>"
		// instead of "pulumi <token>". Don't copy flags here — they're already
		// on subcmd from the copy above.
		fakeDo := &cobra.Command{Use: cmd.Use}
		fakeDo.SetContext(cmd.Context())
		fakeDo.SetOut(cmd.OutOrStdout())
		fakeDo.SetErr(cmd.ErrOrStderr())
		fakeDo.SetIn(cmd.InOrStdin())
		fakeDo.AddCommand(subcmd)
		fullArgs = append([]string{subcmd.Name()}, fullArgs...)

		parent := cmd.Parent()
		current := fakeDo
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
		Use:   "do <pkg:mod:typ> [command]",
		Short: "[EXPERIMENTAL] Interact directly with cloud resources",
		Long: `[EXPERIMENTAL] Interact with any cloud

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
    set --input to convert from another format)

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
	cmd.PersistentFlags().BoolVar(&stateless, "stateless", false,
		"Run create/patch/delete directly against the provider without persisting state. "+
			"Required for now: the stateful (engine-driven) implementation is still in development, "+
			"so create/patch/delete error out unless --stateless is set.")
	cmd.PersistentFlags().StringVar(
		&pkg, "package", "", "The package to load, in the form 'name@version' or "+
			"a path to a plugin binary or folder. If the package supports "+
			"parameterization, additional space-separated parameters can be "+
			"included after the package name, e.g. --package \"name@version "+
			"param1 \\\"multi word param\\\"\"")

	return cmd
}

// currentStackIdentity reads the workspace's currently selected stack and splits it into an organization
// and short stack name. Stacks are persisted as one of:
//   - "<org>/<project>/<stack>" — the modern form (cloud backend and the DIY backend's "organization" prefix)
//   - "<org>/<stack>"           — legacy cloud form, no embedded project
//   - "<stack>"                 — very old / unqualified DIY
//
// Errors reading the workspace are swallowed: the stack identity is best-effort context for PCL
// evaluation, not a hard requirement, and `do` must stay usable when no workspace is configured.
func currentStackIdentity(ws pkgWorkspace.Context) (organization, stack string) {
	w, err := ws.New()
	if err != nil {
		return "", ""
	}
	name := w.Settings().Stack
	if name == "" {
		return "", ""
	}
	parts := strings.Split(name, "/")
	switch len(parts) {
	case 1:
		return "", parts[0]
	case 2:
		return parts[0], parts[1]
	default:
		return parts[0], parts[len(parts)-1]
	}
}

type packageCommand struct {
	pkg               string
	args              []string
	converter         func(string) (plugin.Converter, error)
	loaderTarget      string
	packageDescriptor *codegenrpc.GetSchemaRequest
	provider          plugin.Provider
	providerFile      string
	providerURN       string
	format            string
	spec              *schema.Package
	dryrun            bool
	showSecrets       bool
	stateless         bool

	// wd / proj / root capture the working-directory and project-loading state from buildSubcommand
	// — kept here rather than baked into a snapshot of functionEvalContext so the evalContext()
	// method can re-read the workspace's currently-selected stack each time a subcommand runs
	// (test fixtures that mutate the workspace between Execute() calls otherwise see stale data).
	wd   string
	proj *workspace.Project
	root string

	// ws / lm let configureProvider open the current stack's backend when --provider is set so it
	// can read the referenced provider resource's Inputs. Plumbed from NewDoCmd.
	ws   pkgWorkspace.Context
	lm   cmdBackend.LoginManager
	sink diag.Sink
}

// evalContext builds the PCL evaluation context from the workspace state we captured at construction
// time. Computed on demand so the stack selection follows ws (helpful in tests, and matches the
// "best-effort, no login required" intent — currentStackIdentity reads only the local workspace).
func (pc *packageCommand) evalContext() functionEvalContext {
	ec := functionEvalContext{WorkingDir: pc.wd}
	if pc.proj != nil {
		ec.ProjectName = string(pc.proj.Name)
		ec.RootDirectory = pc.root
		// When a stack is selected in the workspace, expose its organization and short name to the
		// PCL runtime so input files can reference pulumi.organization / pulumi.stack the same way
		// a program would.
		ec.Organization, ec.Stack = currentStackIdentity(pc.ws)
	}
	return ec
}

func (pc *packageCommand) newCommand() (*cobra.Command, error) {
	// Based on token (if present) we need to work out if this is a package, module, resource, or function command.

	if len(pc.args) == 0 || strings.Count(pc.args[0], ":") == 0 {
		// No token (or just a package name), so this is just the package command.
		return pc.newPackageCommand(), nil
	}

	// Try and look it up, it's either a module, resource, or function.
	if fun, ok := pc.spec.GetFunction(pc.args[0]); ok {
		return pc.newFunctionCommand(fun), nil
	}
	if res, ok := pc.spec.GetResource(pc.args[0]); ok {
		return pc.newResourceCommand(res), nil
	}
	if pc.isKnownModule(pc.args[0]) {
		return pc.newModuleCommand(), nil
	}

	return nil, pc.unknownTokenError(pc.args[0])
}

// isKnownModule checks whether `typed` (e.g. "aws:s3" or "pkg:mod1/mod2") matches a module in the schema.
func (pc *packageCommand) isKnownModule(typed string) bool {
	// inModule reports whether the token lives in module `typed` or a descendant of it. Modules nest
	// on "/", so a parent like "pkg:mod1" matches a token in "pkg:mod1/mod2".
	_, name, _ := strings.Cut(typed, ":")
	if name == "" {
		return false
	}
	inModule := func(token string) bool {
		mod := pc.spec.TokenToModule(token)
		return mod == name || strings.HasPrefix(mod, name+"/")
	}
	for _, fn := range pc.spec.Functions {
		if !fn.IsMethod && inModule(fn.Token) {
			return true
		}
	}
	for _, res := range pc.spec.Resources {
		if inModule(res.Token) {
			return true
		}
	}
	return false
}

func (pc *packageCommand) moduleToken(token string) string {
	mod := pc.spec.TokenToModule(token)
	if mod == "" {
		return ""
	}
	return string(tokens.Token(token).Package()) + ":" + mod
}

func (pc *packageCommand) unknownTokenError(typed string) error {
	msg := fmt.Sprintf("unknown function, resource, or module %q in package %q", typed, pc.spec.Name)
	suggestions := pc.suggestTokens(typed)
	if len(suggestions) == 0 {
		return cmdCmd.ConfigurationError{Message: msg}
	}
	var b strings.Builder
	b.WriteString(msg)
	b.WriteString("\n\nDid you mean this?\n")
	for _, s := range suggestions {
		fmt.Fprintf(&b, "\t%s\n", s)
	}
	return cmdCmd.ConfigurationError{Message: b.String()}
}

// suggestTokens returns tokens whose module and name are individually close to the typed value's
// components. The displayed form comes from spec.CanonicalizeToken.
func (pc *packageCommand) suggestTokens(typed string) []string {
	_, typedMod, typedName, diags := pcl.DecomposeToken(typed, hcl.Range{})
	if diags.HasErrors() {
		return nil
	}

	op := levenshtein.DefaultOptionsWithSub
	op.Matches = func(r1, r2 rune) bool {
		return unicode.ToLower(r1) == unicode.ToLower(r2)
	}
	// Scale the allowed edit distance with the longer string
	closeEnough := func(a, b string) bool {
		threshold := 2
		if max(len(a), len(b)) < 6 {
			threshold = 1
		}
		return levenshtein.DistanceForStrings([]rune(a), []rune(b), op) <= threshold
	}

	seen := map[string]struct{}{}
	var suggestions []string
	consider := func(token string) {
		_, _, name, diags := pcl.DecomposeToken(token, hcl.Range{})
		if diags.HasErrors() {
			return
		}
		if !closeEnough(typedName, name) {
			return
		}
		// Compare against the canonical module (the form the user types), not the raw module. Bridged providers nest
		// submodules so the raw form looks nothing like what the user typed (e.g. "s3/bucket" vs "s3"). "index" stands
		// in for a missing module so typing pkg:Type still ranks against tokens that live in that module.
		canonicalMod := pc.spec.TokenToModule(token)
		if canonicalMod == "" {
			canonicalMod = "index"
		}
		modMatch := strings.EqualFold(typedMod, canonicalMod) ||
			typedMod == "index" ||
			closeEnough(typedMod, canonicalMod)
		if !modMatch {
			return
		}
		display := pc.spec.CanonicalizeToken(token)
		if _, ok := seen[display]; ok {
			return
		}
		seen[display] = struct{}{}
		suggestions = append(suggestions, display)
	}
	for _, fn := range pc.spec.Functions {
		if fn.IsMethod {
			continue
		}
		consider(fn.Token)
	}
	for _, res := range pc.spec.Resources {
		consider(res.Token)
	}

	// If the user typed a 2-segment value (pkg:something), the second segment may have been intended as a module name,
	// e.g. "aws:s4" probably means "aws:s3". Search for modules so the suggestions aren't limited to leaf tokens.
	if typedMod == "index" {
		considerModule := func(token string) {
			mod := pc.spec.TokenToModule(token)
			if mod == "" || !closeEnough(typedName, mod) {
				return
			}
			display := pc.moduleToken(token)
			if _, ok := seen[display]; ok {
				return
			}
			seen[display] = struct{}{}
			suggestions = append(suggestions, display)
		}
		for _, fn := range pc.spec.Functions {
			if !fn.IsMethod {
				considerModule(fn.Token)
			}
		}
		for _, res := range pc.spec.Resources {
			considerModule(res.Token)
		}
	}

	slices.Sort(suggestions)
	return suggestions
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

		if mod := pc.moduleToken(fn.Token); mod == "" {
			functions[fn.Token] = fn
		} else {
			modules[mod] = struct{}{}
		}
	}
	for _, res := range pc.spec.Resources {
		if mod := pc.moduleToken(res.Token); mod == "" {
			resources[res.Token] = res
		} else {
			modules[mod] = struct{}{}
		}
	}

	var help strings.Builder
	if len(modules) > 0 {
		fmt.Fprintln(&help, "Modules:")
		for mod := range maps.Sorted(modules) {
			fmt.Fprintf(&help, "  %s\n", mod)
		}
		fmt.Fprintln(&help, "")
	}
	if len(functions) > 0 {
		fmt.Fprintln(&help, "Functions:")
		for _, fn := range maps.Sorted(functions) {
			tok := pc.spec.CanonicalizeToken(fn.Token)
			fmt.Fprintf(&help, "  %s\n", tok)
		}
		fmt.Fprintln(&help, "")
	}
	if len(resources) > 0 {
		fmt.Fprintln(&help, "Resources:")
		for _, res := range maps.Sorted(resources) {
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
			modules[pc.moduleToken(fn.Token)] = struct{}{}
		}
	}
	for _, res := range pc.spec.Resources {
		mod := pc.spec.TokenToModule(res.Token)
		if mod == name {
			resources[res.Token] = res
		} else if strings.HasPrefix(mod, name+"/") {
			modules[pc.moduleToken(res.Token)] = struct{}{}
		}
	}

	var help strings.Builder
	if len(modules) > 0 {
		fmt.Fprintln(&help, "Modules:")
		for mod := range maps.Sorted(modules) {
			fmt.Fprintf(&help, "  %s\n", mod)
		}
		fmt.Fprintln(&help, "")
	}
	if len(functions) > 0 {
		fmt.Fprintln(&help, "Functions:")
		for _, fn := range maps.Sorted(functions) {
			tok := pc.spec.CanonicalizeToken(fn.Token)
			fmt.Fprintf(&help, "  %s\n", tok)
		}
		fmt.Fprintln(&help, "")
	}
	if len(resources) > 0 {
		fmt.Fprintln(&help, "Resources:")
		for _, res := range maps.Sorted(resources) {
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
