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

package newcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/credentialcheck"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// credentialsCheckEnabled reports whether this run should preflight cloud credentials: only
// interactive runs that actually set up a project, and only when the user hasn't opted out.
func credentialsCheckEnabled(args newArgs) bool {
	return args.interactive && !args.generateOnly && !args.offline && !env.SkipNewCredentialsCheck.Value()
}

// credentialsPreflight holds what the per-package credentials check needs beyond the
// package itself.
type credentialsPreflight struct {
	host   plugin.Host
	pctx   *plugin.Context
	cfg    config.Map
	stdout io.Writer
	opts   display.Options

	// confirm asks the user a yes-or-no question. It is nil when the preflight must not stop to
	// ask, as under --yes, and then no remediation is offered.
	confirm func(prompt string) bool
	run     func(ctx context.Context, r plugin.CommandRemediation) error
}

// preflightCloudCredentials runs the best-effort cloud credentials check for a freshly
// created project against every required provider that opted in through its schema.
// It only ever prints advisory warnings and never fails the command.
func preflightCloudCredentials(
	ctx context.Context, args newArgs, sink diag.Sink, host plugin.Host, proj *workspace.Project, root string,
	s backend.Stack, packages []workspace.PackageDescriptor, savedConfig config.Map, opts display.Options,
) {
	if !credentialsCheckEnabled(args) || s == nil || len(packages) == 0 {
		return
	}
	// A managed stack's config lives in its environment definition, not under `config:`.
	cfg := savedConfig
	if s.ConfigLocation().ManagedEnvironment == "" {
		ps, err := cmdStack.LoadProjectStack(ctx, sink, proj, s, "")
		if err != nil {
			slog.DebugContext(ctx, "skipping credentials check", "err", err)
			return
		}
		cfg = ps.Config
	}

	// The check has its own plugin context so that the providers it launches are released when it is done.
	cloudCredentialEnv, err := cmdBackend.PluginCloudCredentialEnv(
		ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, proj)
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "err", err)
		return
	}
	pctx, err := plugin.NewContextWithHost(ctx, sink, sink, host, root, root, nil, cloudCredentialEnv)
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "err", err)
		return
	}
	defer contract.IgnoreClose(pctx)

	pf := credentialsPreflight{host: host, pctx: pctx, cfg: cfg, stdout: args.stdout, opts: opts, run: runRemediation}
	if !args.yes {
		pf.confirm = func(prompt string) bool {
			return ui.PromptUser(prompt, []string{"yes", "no"}, "yes", opts.Color) == "yes"
		}
	}
	for _, pkg := range packages {
		if pkg.Kind != apitype.ResourcePlugin || pkg.ExtensionParameterization != nil {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		pf.checkPackage(ctx, pkg)
	}
}

// checkPackage checks the credentials of a single provider and prints any problem it finds,
// checking again if the user runs the fix the provider suggests.
func (pf credentialsPreflight) checkPackage(ctx context.Context, desc workspace.PackageDescriptor) {
	_, err := pf.check(ctx, desc)
	problem, ok := errors.AsType[*credentialcheck.Problem](err)
	if !ok {
		pf.logSkipped(ctx, desc, err)
		return
	}
	pf.printWarning(problem)
	if !pf.offerRemediation(ctx, problem) || ctx.Err() != nil {
		return
	}

	// The provider is loaded again, since it may have cached the credentials that failed.
	info, err := pf.check(ctx, desc)
	if problem, ok := errors.AsType[*credentialcheck.Problem](err); ok {
		pf.printWarning(problem)
	} else if err != nil {
		pf.logSkipped(ctx, desc, err)
	} else {
		fmt.Fprintf(pf.stdout, "%s credentials verified. %s\n\n", info.DisplayName, cmdutil.EmojiOr("✅", ""))
	}
}

// logSkipped logs why the credentials check of desc could not run, if it could not.
func (pf credentialsPreflight) logSkipped(ctx context.Context, desc workspace.PackageDescriptor, err error) {
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name, "err", err)
	}
}

// check loads a single provider and runs [credentialcheck.Check] against it. It returns a
// [credentialcheck.Problem] if the provider rejected the credentials, and any other error if the
// check could not run.
func (pf credentialsPreflight) check(
	ctx context.Context, desc workspace.PackageDescriptor,
) (credentialcheck.Info, error) {
	prov, err := pf.host.Provider(pf.pctx, desc.PluginDescriptor, env.Global())
	if err != nil {
		return credentialcheck.Info{}, err
	}
	defer contract.IgnoreClose(prov)

	request := plugin.GetSchemaRequest{}
	if parameterization := desc.Parameterization; parameterization != nil {
		resp, err := prov.Parameterize(ctx, plugin.ParameterizeRequest{Parameters: &plugin.ParameterizeValue{
			Name: parameterization.Name, Version: parameterization.Version, Value: parameterization.Value,
		}})
		if err != nil {
			return credentialcheck.Info{}, err
		}
		if resp.Name != parameterization.Name || !resp.Version.EQ(parameterization.Version) {
			return credentialcheck.Info{}, fmt.Errorf("unexpected parameterization response: %s@%s", resp.Name, resp.Version)
		}
		request.SubpackageName = resp.Name
		request.SubpackageVersion = &resp.Version
	}

	target := &deploy.Target{Config: plaintextConfig(pf.cfg), Decrypter: config.NopDecrypter}
	return credentialcheck.Check(ctx, prov, request, target)
}

// plaintextConfig returns cfg without its secure values, which this check does not decrypt.
func plaintextConfig(cfg config.Map) config.Map {
	plain := config.Map{}
	for k, v := range cfg {
		if !v.Secure() {
			plain[k] = v
		}
	}
	return plain
}

// printWarning renders a credentials problem as an advisory warning.
func (pf credentialsPreflight) printWarning(problem *credentialcheck.Problem) {
	warning := colors.SpecWarning + "warning:" + colors.Reset + " " + problem.Error() + "\n" +
		"`pulumi up` may fail until this is resolved."
	fmt.Fprintln(pf.stdout, pf.opts.Color.Colorize(colors.HighlightCommands(warning)))
	fmt.Fprintln(pf.stdout)
}
