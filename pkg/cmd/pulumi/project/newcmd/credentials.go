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
	"fmt"
	"io"
	"log/slog"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/credentialcheck"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
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
}

// preflightCloudCredentials runs the best-effort cloud credentials check for a freshly
// created project against every required provider that opted in through its schema.
// It only ever prints advisory warnings and never fails the command.
func preflightCloudCredentials(
	ctx context.Context, args newArgs, sink diag.Sink, host plugin.Host, proj *workspace.Project, root string,
	s backend.Stack, packages []workspace.PackageDescriptor, opts display.Options,
) {
	if !credentialsCheckEnabled(args) || s == nil || len(packages) == 0 {
		return
	}
	ps, err := cmdStack.LoadProjectStack(ctx, sink, proj, s, "")
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "err", err)
		return
	}

	// The check has its own plugin context so that the providers it launches are released when it is done.
	pctx, err := plugin.NewContextWithHost(ctx, sink, sink, host, root, root, nil)
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "err", err)
		return
	}
	defer contract.IgnoreClose(pctx)

	pf := credentialsPreflight{host: host, pctx: pctx, cfg: ps.Config, stdout: args.stdout, opts: opts}
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

// checkPackage loads a single provider, asks its schema whether it wants the credentials
// check and, if so, runs it and prints any problem it finds.
func (pf credentialsPreflight) checkPackage(ctx context.Context, desc workspace.PackageDescriptor) {
	prov, err := pf.host.Provider(pf.pctx, desc.PluginDescriptor, env.Global())
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name, "err", err)
		return
	}
	defer contract.IgnoreClose(prov)

	request := plugin.GetSchemaRequest{}
	if parameterization := desc.Parameterization; parameterization != nil {
		resp, err := prov.Parameterize(ctx, plugin.ParameterizeRequest{Parameters: &plugin.ParameterizeValue{
			Name: parameterization.Name, Version: parameterization.Version, Value: parameterization.Value,
		}})
		if err != nil {
			slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name, "err", err)
			return
		}
		if resp.Name != parameterization.Name || !resp.Version.EQ(parameterization.Version) {
			slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name,
				"err", fmt.Errorf("unexpected parameterization response: %s@%s", resp.Name, resp.Version))
			return
		}
		request.SubpackageName = resp.Name
		request.SubpackageVersion = &resp.Version
	}

	cp, err := credentialcheck.FromSchema(ctx, prov, request)
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name, "err", err)
		return
	}
	if !cp.Supported {
		return
	}
	news := credentialcheck.ConfigProperties(cp.Pkg, pf.cfg, skipSecrets)
	if err := credentialcheck.Probe(ctx, cp, prov, news); err != nil {
		// A cancelled check stays silent.
		if ctx.Err() != nil {
			slog.DebugContext(ctx, "credentials check cancelled", "provider", cp.Pkg)
			return
		}
		pf.printWarning(err)
	}
}

// skipSecrets fails every decryption, so that the check only passes plaintext config to providers.
var skipSecrets = config.NewErrorCrypter("the `pulumi new` credentials check does not decrypt secrets")

// printWarning renders a credentials problem as an advisory warning.
func (pf credentialsPreflight) printWarning(problem error) {
	warning := colors.SpecWarning + "warning:" + colors.Reset + " " + problem.Error() + "\n" +
		"`pulumi up` may fail until this is resolved."
	fmt.Fprintln(pf.stdout, pf.opts.Color.Colorize(colors.HighlightCommands(warning)))
	fmt.Fprintln(pf.stdout)
}
