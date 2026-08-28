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
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// The credentials preflight may probe cloud metadata services or STS-like endpoints; timeouts on
// hosts without instance metadata are the slow path, so give the check a generous but bounded budget.
// The budget covers plugin launch, GetSchema, CheckConfig and Configure for every opted-in package.
const defaultCredentialsPreflightTimeout = 15 * time.Second

// cloudProvider describes a provider package that opted into the `pulumi new` credentials
// preflight through its schema.
type cloudProvider struct {
	// pkg is the provider package name, which is also the stack config namespace.
	pkg string
	// displayName is how the provider is referred to in warnings.
	displayName string
	// docURL points at the provider's installation & configuration docs; may be empty.
	docURL string
}

// credentialsCheckEnabled reports whether this run should preflight cloud credentials: only
// interactive runs that actually set up a project, and only when the user hasn't opted out.
func credentialsCheckEnabled(args newArgs) bool {
	return args.interactive && !args.generateOnly && !args.offline && !env.SkipNewCredentialsCheck.Value()
}

// providerConfigProperties extracts the plaintext values in the provider's config
// namespace from the saved stack config for use as CheckConfig inputs. Secure values
// are skipped.
func providerConfigProperties(cp cloudProvider, cfg config.Map) property.Map {
	m := map[string]property.Value{}
	for k, v := range cfg {
		if k.Namespace() != cp.pkg || v.Secure() {
			continue
		}
		plain, err := v.Value(config.NopDecrypter)
		if err != nil {
			continue
		}
		m[k.Name()] = property.New(plain)
	}
	return property.NewMap(m)
}

// preflightCloudCredentials runs the best-effort cloud credentials check for a freshly
// created project against every required provider that opted in through its schema. It
// reports whether a warning was printed.
func preflightCloudCredentials(
	ctx context.Context, args newArgs, host plugin.Host, proj *workspace.Project, root string,
	s backend.Stack, packages []workspace.PackageDescriptor, opts display.Options,
) bool {
	if !credentialsCheckEnabled(args) || s == nil || len(packages) == 0 {
		return false
	}
	ps, err := cmdStack.LoadProjectStack(ctx, cmdutil.Diag(), proj, s, "")
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "err", err)
		return false
	}

	// Providers send their RPCs on the plugin context's base context rather than the
	// context passed to each call, so bound the whole preflight by giving it a context of
	// its own: plugin launch, GetSchema, CheckConfig and Configure all share one deadline.
	tctx, cancel := context.WithTimeout(ctx, defaultCredentialsPreflightTimeout)
	defer cancel()
	pctx, err := plugin.NewContextWithHost(tctx, cmdutil.Diag(), cmdutil.Diag(), host, root, root, nil)
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "err", err)
		return false
	}
	defer contract.IgnoreClose(pctx)

	warned := false
	for _, pkg := range packages {
		// A parameterized package's plugin name is the base plugin (e.g. terraform-provider),
		// not the provider itself, so it cannot be loaded from the descriptor alone.
		if pkg.Kind != apitype.ResourcePlugin || pkg.Parameterization != nil {
			continue
		}
		if tctx.Err() != nil {
			break
		}
		if preflightPackage(tctx, host, pctx, pkg.PluginDescriptor, ps.Config, args.stdout, opts) {
			warned = true
		}
	}
	return warned
}

// preflightPackage loads a single provider, asks its schema whether it wants the credentials
// check and, if so, runs it. It reports whether a warning was printed.
func preflightPackage(
	ctx context.Context, host plugin.Host, pctx *plugin.Context, desc workspace.PluginDescriptor,
	cfg config.Map, stdout io.Writer, opts display.Options,
) bool {
	prov, err := host.Provider(pctx, desc, env.Global())
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "provider", desc.Name, "err", err)
		return false
	}
	defer contract.IgnoreClose(prov)

	cp, ok := cloudProviderFromSchema(ctx, prov, desc.Name)
	if !ok {
		return false
	}
	return checkCloudCredentials(ctx, cp, prov, providerConfigProperties(cp, cfg), stdout, opts)
}

// cloudProviderFromSchema fetches the provider's schema and returns the preflight metadata it
// declares. ok is false when the provider did not opt into the check or its schema is unavailable.
func cloudProviderFromSchema(ctx context.Context, prov plugin.Provider, pkg string) (cp cloudProvider, ok bool) {
	resp, err := prov.GetSchema(ctx, plugin.GetSchemaRequest{})
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "provider", pkg, "err", err)
		return cloudProvider{}, false
	}
	var info schema.PackageInfoSpec
	if err := json.Unmarshal(resp.Schema, &info); err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "provider", pkg, "err", err)
		return cloudProvider{}, false
	}
	if !info.ValidateCredentialsOnNew {
		return cloudProvider{}, false
	}
	cp = cloudProvider{pkg: pkg, displayName: info.DisplayName, docURL: info.ConfigurationDocsURL}
	if cp.displayName == "" {
		cp.displayName = pkg
	}
	return cp, true
}

// checkCloudCredentials calls the cloud provider's CheckConfig and then Configure, which is
// where providers validate credentials and initialise their clients, and prints an advisory
// warning on failure. ctx bounds the check: if it expires the check stays silent. It reports
// whether a warning was printed.
func checkCloudCredentials(
	ctx context.Context, cp cloudProvider, prov plugin.Provider, news property.Map,
	stdout io.Writer, opts display.Options,
) bool {
	urn := resource.NewURN("dev", "default", "", tokens.Type("pulumi:providers:"+cp.pkg), "default")

	resp, err := prov.CheckConfig(ctx, plugin.CheckConfigRequest{URN: urn, News: news})
	if err == nil && len(resp.Failures) == 0 {
		err = configureProvider(ctx, prov, urn, resp.Properties, news)
	}
	if ctx.Err() != nil {
		slog.DebugContext(ctx, "credentials check timed out or was cancelled", "provider", cp.pkg)
		return false
	}

	var headline string
	var details []string
	switch {
	case err != nil:
		headline = fmt.Sprintf("Could not validate your %s credentials:", cp.displayName)
		details = strings.Split(strings.TrimSpace(errorMessage(err)), "\n")
	case len(resp.Failures) > 0:
		headline = fmt.Sprintf("The %s provider reported problems with this stack's configuration:", cp.displayName)
		for _, f := range resp.Failures {
			reason := strings.ReplaceAll(strings.TrimSpace(f.Reason), "\n", "\n    ")
			if f.Property != "" {
				reason = string(f.Property) + ": " + reason
			}
			details = append(details, reason)
		}
	default:
		return false
	}

	warning := opts.Color.Colorize(colors.SpecWarning + "warning:" + colors.Reset)
	fmt.Fprintf(stdout, "%s %s\n", warning, headline)
	for _, line := range details {
		fmt.Fprintf(stdout, "    %s\n", line)
	}
	fmt.Fprintln(stdout, "Your project was created successfully, but `pulumi up` may fail until this is resolved.")
	if cp.docURL != "" {
		fmt.Fprintf(stdout, "For help configuring the %s provider, see %s\n", cp.displayName, cp.docURL)
	}
	fmt.Fprintln(stdout)
	return true
}

// configureProvider calls Configure with the checked config and waits for it to
// complete, returning any error it produced. Plugin-backed providers run Configure
// asynchronously, so the result is awaited through plugin.ConfigureAwaiter.
func configureProvider(
	ctx context.Context, prov plugin.Provider, urn resource.URN, checked, news property.Map,
) error {
	inputs := checked
	if inputs.Len() == 0 {
		inputs = news
	}
	name := urn.Name()
	typ := urn.Type()
	id := resource.ID("preflight")
	if _, err := prov.Configure(ctx, plugin.ConfigureRequest{
		URN:    &urn,
		Name:   &name,
		Type:   &typ,
		ID:     &id,
		Inputs: resource.ToResourcePropertyMap(inputs),
	}); err != nil {
		return err
	}
	if awaiter, ok := prov.(plugin.ConfigureAwaiter); ok {
		return awaiter.AwaitConfigure(ctx)
	}
	return nil
}

// errorMessage unwraps gRPC status errors to their message so the provider's own
// wording is shown without the "rpc error: code = ..." prefix.
func errorMessage(err error) string {
	if rpcErr, ok := rpcerror.FromError(err); ok {
		return rpcErr.Message()
	}
	return err.Error()
}
