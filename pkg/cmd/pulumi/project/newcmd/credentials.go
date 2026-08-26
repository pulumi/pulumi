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
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"google.golang.org/grpc/status"
)

// The credentials preflight may probe STS, IMDS or Azure CLI/IMDS; timeouts on hosts
// without instance metadata are the slow path, so give the check a generous but bounded budget.
const defaultCredentialsPreflightTimeout = 15 * time.Second

// cloudProvider describes a provider package whose Configure validates cloud credentials,
// making it a candidate for the `pulumi new` credentials preflight.
type cloudProvider struct {
	// pkg is the provider package name, which is also the stack config namespace.
	pkg string
	// displayName is how the cloud is referred to in warnings.
	displayName string
	// docURL points at the provider's installation & configuration docs.
	docURL string
}

var cloudProviders = []cloudProvider{
	{
		pkg:         "aws",
		displayName: "AWS",
		docURL:      "https://www.pulumi.com/registry/packages/aws/installation-configuration/",
	},
	{
		pkg:         "azure-native",
		displayName: "Azure",
		docURL:      "https://www.pulumi.com/registry/packages/azure-native/installation-configuration/",
	},
}

// credentialsCheckProvider returns the cloud provider whose credentials should be
// preflighted for this run, if any: only for interactive runs that actually set up a
// project from a template configuring a known cloud provider, and only when the user
// hasn't opted out.
func credentialsCheckProvider(args newArgs, template cmdTemplates.ProjectTemplate) (cloudProvider, bool) {
	if !args.interactive || args.generateOnly || args.offline || env.SkipNewCredentialsCheck.Value() {
		return cloudProvider{}, false
	}
	for _, cp := range cloudProviders {
		for key := range template.Config {
			if strings.HasPrefix(key, cp.pkg+":") {
				return cp, true
			}
		}
	}
	return cloudProvider{}, false
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

// providerLoadFunc loads the cloud resource provider; satisfied in production by a
// closure over the plugin host and in tests by a mock factory.
type providerLoadFunc func() (plugin.Provider, error)

// checkCloudCredentials runs a best-effort credentials preflight by calling the cloud
// provider's CheckConfig and then Configure, which is where providers validate
// credentials and initialise their clients. It prints an advisory warning on failure
// and never fails the command. It reports whether a warning was printed.
func checkCloudCredentials(
	ctx context.Context,
	cp cloudProvider,
	load providerLoadFunc,
	news property.Map,
	stdout io.Writer,
	opts display.Options,
	timeout time.Duration,
) bool {
	prov, err := load()
	if err != nil {
		slog.DebugContext(ctx, "skipping credentials check", "provider", cp.pkg, "err", err)
		return false
	}

	// A single deadline bounds the whole preflight: CheckConfig plus Configure.
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	urn := resource.NewURN("dev", "default", "", tokens.Type("pulumi:providers:"+cp.pkg), "default")

	type result struct {
		resp plugin.CheckConfigResponse
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		// The provider client sends this RPC on the plugin's lifetime context, not ctx,
		// so a timeout below abandons the call; the plugin is torn down with the host.
		resp, err := prov.CheckConfig(ctx, plugin.CheckConfigRequest{URN: urn, News: news})
		ch <- result{resp, err}
	}()

	var r result
	select {
	case r = <-ch:
	case <-tctx.Done():
		slog.DebugContext(ctx, "credentials check timed out or was cancelled", "provider", cp.pkg)
		return false
	}

	var configureErr error
	if r.err == nil && len(r.resp.Failures) == 0 {
		configureErr = configureProvider(tctx, prov, urn, r.resp.Properties, news)
		if configureErr != nil && tctx.Err() != nil {
			// Timed out or cancelled while waiting on Configure; stay silent as above.
			slog.DebugContext(ctx, "credentials check timed out or was cancelled", "provider", cp.pkg)
			return false
		}
	}
	contract.IgnoreClose(prov)

	if r.err == nil && len(r.resp.Failures) == 0 && configureErr == nil {
		return false
	}

	warning := opts.Color.Colorize(colors.SpecWarning + "warning:" + colors.Reset)
	switch {
	case r.err != nil || configureErr != nil:
		err := r.err
		if err == nil {
			err = configureErr
		}
		fmt.Fprintf(stdout, "%s Could not validate your %s credentials:\n", warning, cp.displayName)
		for line := range strings.SplitSeq(strings.TrimSpace(errorMessage(err)), "\n") {
			fmt.Fprintf(stdout, "    %s\n", line)
		}
		fmt.Fprintf(stdout, "Your project was created successfully, "+
			"but `pulumi up` may fail until %s credentials are configured.\n", cp.displayName)
		fmt.Fprintf(stdout, "For help configuring credentials, see %s\n\n", cp.docURL)
	case len(r.resp.Failures) > 0:
		fmt.Fprintf(stdout, "%s The %s provider reported problems with this stack's configuration:\n",
			warning, cp.displayName)
		for _, f := range r.resp.Failures {
			reason := strings.ReplaceAll(strings.TrimSpace(f.Reason), "\n", "\n    ")
			if f.Property != "" {
				fmt.Fprintf(stdout, "    %s: %s\n", f.Property, reason)
			} else {
				fmt.Fprintf(stdout, "    %s\n", reason)
			}
		}
		fmt.Fprintln(stdout, "Your project was created successfully, "+
			"but `pulumi up` may fail until this is resolved.")
		fmt.Fprintf(stdout, "For help configuring the %s provider, see %s\n\n", cp.displayName, cp.docURL)
	}
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
	if _, ok := status.FromError(err); ok {
		return rpcerror.Convert(err).Message()
	}
	return err.Error()
}
