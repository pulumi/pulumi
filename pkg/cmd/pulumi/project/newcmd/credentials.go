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
)

const (
	awsPackageName = "aws"
	//nolint:gosec // this is a documentation URL, not a credential
	awsCredentialsDocURL = "https://www.pulumi.com/registry/packages/aws/installation-configuration/"

	// The credentials preflight may probe STS or IMDS; IMDS timeouts on non-EC2 hosts
	// are the slow path, so give the check a generous but bounded budget.
	defaultCredentialsPreflightTimeout = 15 * time.Second
)

// shouldCheckAWSCredentials reports whether the best-effort AWS credentials preflight
// should run: only for interactive runs that actually set up a project from an AWS
// template, and only when the user hasn't opted out.
func shouldCheckAWSCredentials(args newArgs, template cmdTemplates.ProjectTemplate) bool {
	if !args.interactive || args.generateOnly || args.offline || env.SkipNewCredentialsCheck.Value() {
		return false
	}
	for key := range template.Config {
		if strings.HasPrefix(key, awsPackageName+":") {
			return true
		}
	}
	return false
}

// awsConfigProperties extracts the plaintext aws:-namespaced values from the saved
// stack config for use as CheckConfig inputs. Secure values are skipped.
func awsConfigProperties(cfg config.Map) property.Map {
	m := map[string]property.Value{}
	for k, v := range cfg {
		if k.Namespace() != awsPackageName || v.Secure() {
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

// providerLoadFunc loads the AWS resource provider; satisfied in production by a
// closure over the plugin host and in tests by a mock factory.
type providerLoadFunc func() (plugin.Provider, error)

// checkAWSCredentials runs a best-effort credentials preflight by calling the AWS
// provider's CheckConfig, which validates credentials for bridged providers. It
// prints an advisory warning on failure and never fails the command. It reports
// whether a warning was printed.
func checkAWSCredentials(
	ctx context.Context,
	load providerLoadFunc,
	news property.Map,
	stdout io.Writer,
	opts display.Options,
	timeout time.Duration,
) bool {
	prov, err := load()
	if err != nil {
		slog.DebugContext(ctx, "skipping AWS credentials check", "err", err)
		return false
	}

	urn := resource.NewURN("dev", "default", "", tokens.Type("pulumi:providers:"+awsPackageName), "default")

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
		contract.IgnoreClose(prov)
	case <-time.After(timeout):
		slog.DebugContext(ctx, "AWS credentials check timed out")
		return false
	case <-ctx.Done():
		return false
	}

	if r.err == nil && len(r.resp.Failures) == 0 {
		return false
	}

	warning := opts.Color.Colorize(colors.SpecWarning + "warning:" + colors.Reset)
	switch {
	case r.err != nil:
		fmt.Fprintf(stdout, "%s Could not validate your AWS credentials:\n", warning)
		for line := range strings.SplitSeq(strings.TrimSpace(rpcerror.Convert(r.err).Message()), "\n") {
			fmt.Fprintf(stdout, "    %s\n", line)
		}
		fmt.Fprintln(stdout, "Your project was created successfully, "+
			"but `pulumi up` may fail until AWS credentials are configured.")
		fmt.Fprintf(stdout, "For help configuring credentials, see %s\n\n", awsCredentialsDocURL)
	case len(r.resp.Failures) > 0:
		fmt.Fprintf(stdout, "%s The AWS provider reported problems with this stack's configuration:\n", warning)
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
		fmt.Fprintf(stdout, "For help configuring the AWS provider, see %s\n\n", awsCredentialsDocURL)
	}
	return true
}
