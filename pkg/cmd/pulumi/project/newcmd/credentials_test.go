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
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var awsProvider, azureProvider, gcpProvider = cloudProviders[0], cloudProviders[1], cloudProviders[2]

func resourcePackage(name, version string) workspace.PackageDescriptor {
	v := semver.MustParse(version)
	return workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
		Kind:    apitype.ResourcePlugin,
		Name:    name,
		Version: &v,
	}}
}

func runCheck(
	t *testing.T, cp cloudProvider, prov plugin.Provider, news property.Map, timeout time.Duration,
) (bool, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	var buf bytes.Buffer
	warned := checkCloudCredentials(ctx, cp, prov, news, &buf, display.Options{Color: colors.Never})
	return warned, buf.String()
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

func TestFindCloudProvider(t *testing.T) {
	t.Parallel()

	parameterized := resourcePackage("aws", "7.0.0")
	parameterized.Parameterization = &workspace.Parameterization{Name: "other", Version: semver.MustParse("1.0.0")}

	tests := []struct {
		name     string
		packages []workspace.PackageDescriptor
		expected string // provider package, or "" when none should match
	}{
		{name: "aws", packages: []workspace.PackageDescriptor{resourcePackage("aws", "7.0.0")}, expected: "aws"},
		{
			name:     "azure-native",
			packages: []workspace.PackageDescriptor{resourcePackage("azure-native", "3.0.0")},
			expected: "azure-native",
		},
		{name: "gcp", packages: []workspace.PackageDescriptor{resourcePackage("gcp", "8.0.0")}, expected: "gcp"},
		{name: "no packages"},
		{name: "classic azure does not match", packages: []workspace.PackageDescriptor{resourcePackage("azure", "6.0.0")}},
		{name: "random", packages: []workspace.PackageDescriptor{resourcePackage("random", "4.0.0")}},
		{
			name:     "multiple packages",
			packages: []workspace.PackageDescriptor{resourcePackage("random", "4.0.0"), resourcePackage("aws", "7.0.0")},
			expected: "aws",
		},
		{name: "parameterized on an aws base plugin does not match", packages: []workspace.PackageDescriptor{parameterized}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cp, desc := findCloudProvider(tt.packages)
			if tt.expected == "" {
				assert.Nil(t, cp)
				return
			}
			require.NotNil(t, cp)
			assert.Equal(t, tt.expected, cp.pkg)
			// The exact package the program depends on is what gets loaded.
			assert.Equal(t, tt.expected, desc.Name)
			assert.Equal(t, apitype.ResourcePlugin, desc.Kind)
			require.NotNil(t, desc.Version)
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
	props := providerConfigProperties(awsProvider, cfg)
	require.Equal(t, 1, props.Len())
	assert.Equal(t, "us-east-1", props.Get("region").AsString())

	props = providerConfigProperties(azureProvider, cfg)
	require.Equal(t, 1, props.Len())
	assert.Equal(t, "WestUS2", props.Get("location").AsString())

	props = providerConfigProperties(gcpProvider, cfg)
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
	assert.Contains(t, out, awsProvider.docURL)
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
	assert.Contains(t, out, awsProvider.docURL)
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
			assert.Contains(t, out, "For help configuring the "+tt.provider.displayName+" provider, see "+tt.provider.docURL)
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
