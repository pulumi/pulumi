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

// Package credentialcheck validates a cloud provider's credentials by configuring the provider, for
// providers that opt into this through the `validateCredentialsOnNew` schema field.
package credentialcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// ErrUnsupported is returned by [Check] for a provider that did not opt into credential checks.
var ErrUnsupported = errors.New("does not support credential checks")

// Info describes a provider package, as declared by its schema.
type Info struct {
	// Pkg is the provider package name, which is also the stack config namespace.
	Pkg string
	// DisplayName is how the provider is referred to in messages.
	DisplayName string
	// DocURL points at the provider's installation & configuration docs; may be empty.
	DocURL string
}

// Check validates the credentials of prov using target's configuration for the provider. A nil
// target supplies no configuration, leaving the provider with only ambient credentials.
//
// It returns a [*Problem] if the provider rejects the configuration or credentials, and wraps
// [ErrUnsupported] if the provider did not opt in. Any other error means the check could not run,
// including ctx's error if ctx is done. The returned Info is set once the schema has been read.
func Check(
	ctx context.Context, prov plugin.Provider, request plugin.GetSchemaRequest, target *deploy.Target,
) (Info, error) {
	resp, err := prov.GetSchema(ctx, request)
	if err != nil {
		return Info{}, fmt.Errorf("getting schema: %w", err)
	}
	var spec schema.PackageInfoSpec
	if err := json.Unmarshal(resp.Schema, &spec); err != nil {
		return Info{}, fmt.Errorf("unmarshal schema: %w", err)
	}
	info := Info{Pkg: spec.Name, DisplayName: spec.DisplayName, DocURL: spec.ConfigurationDocsURL}
	if info.DisplayName == "" {
		info.DisplayName = info.Pkg
	}
	if !spec.ValidateCredentialsOnNew {
		return info, fmt.Errorf("the %s provider %w", info.Pkg, ErrUnsupported)
	}
	news, err := target.GetPackageConfig(tokens.Package(info.Pkg))
	if err != nil {
		return info, fmt.Errorf("reading %s configuration: %w", info.Pkg, err)
	}
	return info, probe(ctx, info, prov, news)
}

// probe calls the cloud provider's CheckConfig and then Configure, which is where providers
// validate credentials and initialise their clients.
func probe(ctx context.Context, info Info, prov plugin.Provider, news property.Map) error {
	urn := resource.NewURN("dev", "default", "", tokens.Type("pulumi:providers:"+info.Pkg), "default")

	resp, err := prov.CheckConfig(ctx, plugin.CheckConfigRequest{URN: urn, News: news})
	if err == nil && len(resp.Failures) == 0 {
		err = configureProvider(ctx, prov, urn, resp.Properties)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return &Problem{
			info:     info,
			headline: fmt.Sprintf("Could not validate your %s credentials:", info.DisplayName),
			details:  strings.Split(strings.TrimSpace(errorMessage(err)), "\n"),
		}
	}
	if len(resp.Failures) == 0 {
		return nil
	}
	p := &Problem{
		info:     info,
		headline: fmt.Sprintf("The %s provider reported problems with its configuration:", info.DisplayName),
	}
	for _, f := range resp.Failures {
		reason := strings.ReplaceAll(strings.TrimSpace(f.Reason), "\n", "\n    ")
		if f.Property != "" {
			reason = string(f.Property) + ": " + reason
		}
		p.details = append(p.details, reason)
	}
	return p
}

// Problem is the error [Check] reports when a provider rejects the configuration or cannot validate
// the user's credentials.
type Problem struct {
	info     Info
	headline string
	details  []string
}

func (p *Problem) Error() string {
	var b strings.Builder
	b.WriteString(p.headline)
	for _, line := range p.details {
		fmt.Fprintf(&b, "\n    %s", line)
	}
	// Providers often link their configuration docs in the error itself; don't repeat the link.
	details := strings.Join(p.details, "\n")
	if docURL := p.info.DocURL; docURL != "" && !strings.Contains(details, strings.TrimSuffix(docURL, "/")) {
		fmt.Fprintf(&b, "\nFor help configuring the %s provider, see %s", p.info.DisplayName, docURL)
	}
	return b.String()
}

// configureProvider calls Configure with the checked config and returns any error it produced.
func configureProvider(
	ctx context.Context, prov plugin.Provider, urn resource.URN, inputs property.Map,
) error {
	name := urn.Name()
	typ := urn.Type()
	id := resource.ID("preflight")
	_, err := prov.Configure(ctx, plugin.ConfigureRequest{
		URN:    &urn,
		Name:   &name,
		Type:   &typ,
		ID:     &id,
		Inputs: inputs,
	})
	return err
}

// errorMessage unwraps gRPC status errors to their message so the provider's own
// wording is shown without the "rpc error: code = ..." prefix.
func errorMessage(err error) string {
	if rpcErr, ok := rpcerror.FromError(err); ok {
		return rpcErr.Message()
	}
	return err.Error()
}
