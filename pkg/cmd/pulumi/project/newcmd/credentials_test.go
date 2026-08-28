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
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const awsDocURL = "https://www.pulumi.com/registry/packages/aws/installation-configuration/"

var (
	awsProvider   = cloudProvider{pkg: "aws", displayName: "AWS", docURL: awsDocURL}
	azureProvider = cloudProvider{pkg: "azure-native", displayName: "Azure"}
	gcpProvider   = cloudProvider{pkg: "gcp", displayName: "Google Cloud"}
)

func resourcePackage(name, version string) workspace.PackageDescriptor {
	v := semver.MustParse(version)
	return workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
		Kind:    apitype.ResourcePlugin,
		Name:    name,
		Version: &v,
	}}
}

// schemaProvider returns a mock whose GetSchema serves the given schema document.
func schemaProvider(schemaJSON string) *plugin.MockProvider {
	return &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
		},
	}
}

func runCheck(
	t *testing.T, cp cloudProvider, prov plugin.Provider, news property.Map, timeout time.Duration,
) (bool, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	var buf bytes.Buffer
	problem := probeCredentials(ctx, cp, prov, news)
	if problem != nil {
		credentialsPreflight{stdout: &buf, opts: display.Options{Color: colors.Never}}.printWarning(cp, problem)
	}
	return problem != nil, buf.String()
}

func echoCheckConfig(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func TestCredentialsCheckEnabled(t *testing.T) {
	tests := []struct {
		name    string
		args    newArgs
		env     string
		enabled bool
	}{
		{name: "interactive", args: newArgs{interactive: true}, enabled: true},
		{name: "non-interactive", args: newArgs{interactive: false}},
		{name: "generate-only", args: newArgs{interactive: true, generateOnly: true}},
		{name: "offline", args: newArgs{interactive: true, offline: true}},
		{name: "kill switch", args: newArgs{interactive: true}, env: "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("PULUMI_SKIP_NEW_CREDENTIALS_CHECK", tt.env)
			}
			assert.Equal(t, tt.enabled, credentialsCheckEnabled(tt.args))
		})
	}
}

func TestCloudProviderFromSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		err      error
		expected cloudProvider
		ok       bool
	}{
		{
			name: "opted in",
			schema: `{"name":"aws","displayName":"AWS","validateCredentialsOnNew":true,` +
				`"configurationDocsUrl":"` + awsDocURL + `"}`,
			expected: awsProvider,
			ok:       true,
		},
		{
			name:     "opted in without display name or docs",
			schema:   `{"name":"aws","validateCredentialsOnNew":true}`,
			expected: cloudProvider{pkg: "aws", displayName: "aws"},
			ok:       true,
		},
		{name: "not opted in", schema: `{"name":"aws","displayName":"AWS","configurationDocsUrl":"` + awsDocURL + `"}`},
		{name: "explicitly opted out", schema: `{"name":"random","validateCredentialsOnNew":false}`},
		{name: "invalid schema", schema: `not json`},
		{name: "schema error", err: errors.New("boom")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prov := schemaProvider(tt.schema)
			if tt.err != nil {
				prov.GetSchemaF = func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
					return plugin.GetSchemaResponse{}, tt.err
				}
			}
			cp, ok := cloudProviderFromSchema(t.Context(), prov, "aws")
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.expected, cp)
		})
	}
}

func TestPreflightCloudCredentialsPackages(t *testing.T) {
	t.Parallel()

	optedIn := func(name, displayName string) string {
		return `{"name":"` + name + `","displayName":"` + displayName + `","validateCredentialsOnNew":true}`
	}
	schemas := map[string]string{
		"aws":    optedIn("aws", "AWS"),
		"gcp":    optedIn("gcp", "Google Cloud"),
		"random": `{"name":"random"}`,
	}

	parameterized := resourcePackage("aws", "7.0.0")
	parameterized.Parameterization = &workspace.Parameterization{Name: "other", Version: semver.MustParse("1.0.0")}
	language := resourcePackage("nodejs", "3.0.0")
	language.Kind = apitype.LanguagePlugin

	tests := []struct {
		name     string
		packages []workspace.PackageDescriptor
		loaded   []string // providers expected to be launched, in order
		checked  []string // providers expected to have CheckConfig called
		warnings []string // display names expected in the output
	}{
		{name: "no packages"},
		{
			name:     "single opted-in package",
			packages: []workspace.PackageDescriptor{resourcePackage("aws", "7.0.0")},
			loaded:   []string{"aws"},
			checked:  []string{"aws"},
			warnings: []string{"AWS"},
		},
		{
			name:     "package that did not opt in is loaded but not checked",
			packages: []workspace.PackageDescriptor{resourcePackage("random", "4.0.0")},
			loaded:   []string{"random"},
		},
		{
			name: "every opted-in package is checked",
			packages: []workspace.PackageDescriptor{
				resourcePackage("random", "4.0.0"), resourcePackage("aws", "7.0.0"), resourcePackage("gcp", "8.0.0"),
			},
			loaded:   []string{"random", "aws", "gcp"},
			checked:  []string{"aws", "gcp"},
			warnings: []string{"AWS", "Google Cloud"},
		},
		{
			name:     "parameterized and language packages are skipped",
			packages: []workspace.PackageDescriptor{parameterized, language},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var loaded, checked []string
			host := &plugin.MockHost{
				ProviderF: func(_ *plugin.Context, desc workspace.PluginDescriptor, _ env.Env) (plugin.Provider, error) {
					loaded = append(loaded, desc.Name)
					prov := schemaProvider(schemas[desc.Name])
					prov.CheckConfigF = func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
						checked = append(checked, desc.Name)
						assert.Equal(t, "pulumi:providers:"+desc.Name, string(req.URN.Type()))
						return plugin.CheckConfigResponse{}, status.Error(codes.Unknown, "no credentials")
					}
					return prov, nil
				},
			}

			sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
			pctx, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
			require.NoError(t, err)
			defer pctx.Close()

			var buf bytes.Buffer
			pf := credentialsPreflight{
				host: host, pctx: pctx, cfg: config.Map{}, stdout: &buf, opts: display.Options{Color: colors.Never},
			}
			for _, pkg := range tt.packages {
				if pkg.Kind != apitype.ResourcePlugin || pkg.Parameterization != nil {
					continue
				}
				pf.checkPackage(t.Context(), pkg.PluginDescriptor)
			}

			assert.Equal(t, tt.loaded, loaded)
			assert.Equal(t, tt.checked, checked)
			for _, name := range tt.warnings {
				assert.Contains(t, buf.String(), "Could not validate your "+name+" credentials")
			}
			assert.Equal(t, len(tt.warnings), strings.Count(buf.String(), "warning:"))
		})
	}
}

func TestProviderConfigProperties(t *testing.T) {
	t.Parallel()

	cfg := config.Map{
		config.MustMakeKey("aws", "region"):            config.NewValue("us-east-1"),
		config.MustMakeKey("aws", "secret"):            config.NewSecureValue("ciphertext"),
		config.MustMakeKey("azure-native", "location"): config.NewValue("WestUS2"),
		config.MustMakeKey("gcp", "project"):           config.NewValue("my-project"),
		config.MustMakeKey("proj", "etcetc"):           config.NewValue("unrelated"),
	}
	props := providerConfigProperties(awsProvider.pkg, cfg)
	require.Equal(t, 1, props.Len())
	assert.Equal(t, "us-east-1", props.Get("region").AsString())

	props = providerConfigProperties(azureProvider.pkg, cfg)
	require.Equal(t, 1, props.Len())
	assert.Equal(t, "WestUS2", props.Get("location").AsString())

	props = providerConfigProperties(gcpProvider.pkg, cfg)
	require.Equal(t, 1, props.Len())
	assert.Equal(t, "my-project", props.Get("project").AsString())
}

func TestCheckCloudCredentialsRPCError(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			assert.Equal(t, "pulumi:providers:aws", string(req.URN.Type()))
			assert.Empty(t, req.Name)
			assert.Empty(t, req.Type)
			assert.Equal(t, "us-east-1", req.News.Get("region").AsString())
			return plugin.CheckConfigResponse{},
				status.Error(codes.Unknown, "unable to validate AWS credentials.\nDetails: no valid credential sources found")
		},
	}

	news := property.NewMap(map[string]property.Value{"region": property.New("us-east-1")})
	warned, out := runCheck(t, awsProvider, mock, news, time.Second)
	assert.True(t, warned)
	assert.Contains(t, out, "warning:")
	assert.Contains(t, out, "Could not validate your AWS credentials")
	assert.Contains(t, out, "unable to validate AWS credentials.")
	assert.Contains(t, out, "    Details: no valid credential sources found")
	assert.NotContains(t, out, "rpc error")
	assert.Contains(t, out, "For help configuring the AWS provider, see "+awsDocURL)
}

func TestCheckCloudCredentialsFailures(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			return plugin.CheckConfigResponse{Failures: []plugin.CheckFailure{
				{Property: "region", Reason: "expected a valid region"},
				{Reason: "unable to validate AWS credentials.\nDetails: no valid credential sources found"},
			}}, nil
		},
		ConfigureF: func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
			t.Fatal("Configure must not be called when CheckConfig fails")
			return plugin.ConfigureResponse{}, nil
		},
	}

	warned, out := runCheck(t, awsProvider, mock, property.Map{}, time.Second)
	assert.True(t, warned)
	assert.Contains(t, out, "The AWS provider reported problems with this stack's configuration")
	assert.Contains(t, out, "    region: expected a valid region")
	assert.Contains(t, out, "    unable to validate AWS credentials.\n    Details: no valid credential sources found")
	assert.Contains(t, out, awsDocURL)
}

func TestCheckCloudCredentialsSuccess(t *testing.T) {
	t.Parallel()

	configured := false
	mock := &plugin.MockProvider{
		CheckConfigF: echoCheckConfig,
		ConfigureF: func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
			configured = true
			require.NotNil(t, req.URN)
			require.NotNil(t, req.Name)
			require.NotNil(t, req.Type)
			require.NotNil(t, req.ID)
			assert.Equal(t, "pulumi:providers:aws", string(*req.Type))
			assert.Equal(t, "default", *req.Name)
			assert.Equal(t, "us-east-1", req.Inputs["region"].StringValue())
			return plugin.ConfigureResponse{}, nil
		},
	}

	news := property.NewMap(map[string]property.Value{"region": property.New("us-east-1")})
	warned, out := runCheck(t, awsProvider, mock, news, time.Second)
	assert.False(t, warned)
	assert.True(t, configured)
	assert.Empty(t, out)
}

func TestCheckCloudCredentialsConfigureError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider cloudProvider
		message  string
	}{
		{awsProvider, "unable to validate AWS credentials.\nDetails: InvalidClientTokenId"},
		{azureProvider, "failed to get authorizer: please run `az login`"},
		{gcpProvider, "google: could not find default credentials"},
	}
	for _, tt := range tests {
		t.Run(tt.provider.pkg, func(t *testing.T) {
			t.Parallel()

			providerType := "pulumi:providers:" + tt.provider.pkg
			mock := &plugin.MockProvider{
				CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
					assert.Equal(t, providerType, string(req.URN.Type()))
					return plugin.CheckConfigResponse{Properties: req.News}, nil
				},
				ConfigureF: func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					assert.Equal(t, providerType, string(*req.Type))
					return plugin.ConfigureResponse{}, status.Error(codes.Unknown, tt.message)
				},
			}

			warned, out := runCheck(t, tt.provider, mock, property.Map{}, time.Second)
			assert.True(t, warned)
			assert.Contains(t, out, "Could not validate your "+tt.provider.displayName+" credentials")
			for line := range strings.SplitSeq(tt.message, "\n") {
				assert.Contains(t, out, "    "+line)
			}
			assert.NotContains(t, out, "rpc error")
			if tt.provider.docURL != "" {
				assert.Contains(t, out, "For help configuring the "+tt.provider.displayName+" provider, see "+tt.provider.docURL)
			} else {
				// Providers that publish no docs link get no help line rather than a dangling one.
				assert.NotContains(t, out, "For help configuring")
			}
		})
	}
}

// awaitingProvider mimics plugin-backed providers, whose Configure completes
// asynchronously and reports its result through AwaitConfigure.
type awaitingProvider struct {
	*plugin.MockProvider
	await func(context.Context) error
}

func (p *awaitingProvider) AwaitConfigure(ctx context.Context) error { return p.await(ctx) }

func TestCheckCloudCredentialsAwaitConfigure(t *testing.T) {
	t.Parallel()

	newMock := func() *plugin.MockProvider {
		return &plugin.MockProvider{
			CheckConfigF: echoCheckConfig,
			ConfigureF: func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
				return plugin.ConfigureResponse{}, nil
			},
		}
	}

	t.Run("error is surfaced", func(t *testing.T) {
		t.Parallel()

		prov := &awaitingProvider{MockProvider: newMock(), await: func(context.Context) error {
			return errors.New("missing required configuration key \"aws:region\": where AWS operations will take place")
		}}

		warned, out := runCheck(t, awsProvider, prov, property.Map{}, time.Second)
		assert.True(t, warned)
		assert.Contains(t, out, "Could not validate your AWS credentials")
		assert.Contains(t, out, "    missing required configuration key \"aws:region\"")
	})

	t.Run("timeout is silent", func(t *testing.T) {
		t.Parallel()

		prov := &awaitingProvider{MockProvider: newMock(), await: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}

		warned, out := runCheck(t, awsProvider, prov, property.Map{}, 10*time.Millisecond)
		assert.False(t, warned)
		assert.Empty(t, out)
	})
}

func TestCheckCloudCredentialsTimeout(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: func(ctx context.Context, _ plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			<-ctx.Done()
			return plugin.CheckConfigResponse{}, ctx.Err()
		},
	}

	warned, out := runCheck(t, awsProvider, mock, property.Map{}, 10*time.Millisecond)
	assert.False(t, warned)
	assert.Empty(t, out)
}
