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
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Provider describes a provider package's credentials check metadata, as declared by its schema.
type Provider struct {
	// Pkg is the provider package name, which is also the stack config namespace.
	Pkg string
	// DisplayName is how the provider is referred to in messages.
	DisplayName string
	// DocURL points at the provider's installation & configuration docs; may be empty.
	DocURL string
	// Supported reports whether the provider opted into the check through its schema.
	Supported bool
}

// FromSchema fetches the provider's schema and returns the credentials check metadata it declares.
func FromSchema(ctx context.Context, prov plugin.Provider, request plugin.GetSchemaRequest) (Provider, error) {
	resp, err := prov.GetSchema(ctx, request)
	if err != nil {
		return Provider{}, err
	}
	var info schema.PackageInfoSpec
	if err := json.Unmarshal(resp.Schema, &info); err != nil {
		return Provider{}, fmt.Errorf("unmarshal schema: %w", err)
	}
	cp := Provider{
		Pkg:         info.Name,
		DisplayName: info.DisplayName,
		DocURL:      info.ConfigurationDocsURL,
		Supported:   info.ValidateCredentialsOnNew,
	}
	if cp.DisplayName == "" {
		cp.DisplayName = cp.Pkg
	}
	return cp, nil
}

// ConfigProperties extracts the values in the provider's config namespace for use as CheckConfig
// inputs. Values that dec cannot decrypt are skipped.
func ConfigProperties(pkg string, cfg config.Map, dec config.Decrypter) property.Map {
	m := map[string]property.Value{}
	for k, v := range cfg {
		if k.Namespace() != pkg {
			continue
		}
		plain, err := v.Value(dec)
		if err != nil {
			continue
		}
		m[k.Name()] = property.New(plain)
	}
	return property.NewMap(m)
}

// Probe calls the cloud provider's CheckConfig and then Configure, which is where providers
// validate credentials and initialise their clients. It returns an error describing what the
// provider rejected, or ctx's error if ctx is done.
func Probe(ctx context.Context, cp Provider, prov plugin.Provider, news property.Map) error {
	urn := resource.NewURN("dev", "default", "", tokens.Type("pulumi:providers:"+cp.Pkg), "default")

	resp, err := prov.CheckConfig(ctx, plugin.CheckConfigRequest{URN: urn, News: news})
	if err == nil && len(resp.Failures) == 0 {
		err = configureProvider(ctx, prov, urn, resp.Properties)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return &problem{
			provider: cp,
			headline: fmt.Sprintf("Could not validate your %s credentials:", cp.DisplayName),
			details:  strings.Split(strings.TrimSpace(errorMessage(err)), "\n"),
		}
	}
	if len(resp.Failures) == 0 {
		return nil
	}
	p := &problem{
		provider: cp,
		headline: fmt.Sprintf("The %s provider reported problems with its configuration:", cp.DisplayName),
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

// problem is the error Probe reports when a provider rejects the configuration or cannot validate
// the user's credentials.
type problem struct {
	provider Provider
	headline string
	details  []string
}

func (p *problem) Error() string {
	var b strings.Builder
	b.WriteString(p.headline)
	for _, line := range p.details {
		fmt.Fprintf(&b, "\n    %s", line)
	}
	// Providers often link their configuration docs in the error itself; don't repeat the link.
	details := strings.Join(p.details, "\n")
	if docURL := p.provider.DocURL; docURL != "" && !strings.Contains(details, strings.TrimSuffix(docURL, "/")) {
		fmt.Fprintf(&b, "\nFor help configuring the %s provider, see %s", p.provider.DisplayName, docURL)
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
