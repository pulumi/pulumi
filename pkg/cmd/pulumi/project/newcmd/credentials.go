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

	// remediate offers to run a command the provider suggested to fix a problem, and reports whether
	// it ran successfully. It is nil when the preflight must not stop to ask, as under --yes.
	remediate func(ctx context.Context, problem *credentialcheck.Problem) bool
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

	pf := credentialsPreflight{host: host, pctx: pctx, cfg: cfg, stdout: args.stdout, opts: opts}
	if !args.yes {
		pf.remediate = remediationOffer{
			stdout: args.stdout,
			confirm: func(prompt string) bool {
				return ui.PromptUser(prompt, []string{"yes", "no"}, "yes", opts.Color) == "yes"
			},
			run: runRemediation,
		}.offer
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

// checkPackage checks the credentials of a single provider and prints any problem it finds. If the
// provider suggests a command to fix the problem and the user runs it, it checks them again.
func (pf credentialsPreflight) checkPackage(ctx context.Context, desc workspace.PackageDescriptor) {
	problem := pf.checkOnce(ctx, desc)
	if problem == nil {
		return
	}
	pf.printWarning(problem)
	if pf.remediate == nil || !pf.remediate(ctx, problem) {
		return
	}

	// The provider is loaded again, since it may have cached the credentials that failed.
	info := problem.Info()
	if problem = pf.checkOnce(ctx, desc); ctx.Err() != nil {
		return
	}
	if problem != nil {
		pf.printWarning(problem)
		return
	}
	fmt.Fprintf(pf.stdout, "%s credentials verified. %s\n\n", info.DisplayName, cmdutil.EmojiOr("✅", ""))
}

// checkOnce loads a single provider, asks its schema whether it wants the credentials check and,
// if so, runs it and returns any problem it finds.
func (pf credentialsPreflight) checkOnce(
	ctx context.Context, desc workspace.PackageDescriptor,
) *credentialcheck.Problem {
	prov, err := pf.host.Provider(pf.pctx, desc.PluginDescriptor, env.Global())
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name, "err", err)
		return nil
	}
	defer contract.IgnoreClose(prov)

	request := plugin.GetSchemaRequest{}
	if parameterization := desc.Parameterization; parameterization != nil {
		resp, err := prov.Parameterize(ctx, plugin.ParameterizeRequest{Parameters: &plugin.ParameterizeValue{
			Name: parameterization.Name, Version: parameterization.Version, Value: parameterization.Value,
		}})
		if err != nil {
			slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name, "err", err)
			return nil
		}
		if resp.Name != parameterization.Name || !resp.Version.EQ(parameterization.Version) {
			slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name,
				"err", fmt.Errorf("unexpected parameterization response: %s@%s", resp.Name, resp.Version))
			return nil
		}
		request.SubpackageName = resp.Name
		request.SubpackageVersion = &resp.Version
	}

	target := &deploy.Target{Config: plaintextConfig(pf.cfg), Decrypter: config.NopDecrypter}
	_, err = credentialcheck.Check(ctx, prov, request, target)
	if problem, ok := errors.AsType[*credentialcheck.Problem](err); ok {
		return problem
	} else if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name, "err", err)
	}
	return nil
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
