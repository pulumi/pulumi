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

func awsPackages() []workspace.PackageDescriptor {
	return []workspace.PackageDescriptor{resourcePackage("aws", "7.0.0")}
}

func resourcePackage(name, version string) workspace.PackageDescriptor {
	v := semver.MustParse(version)
	return workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
		Kind:    apitype.ResourcePlugin,
		Name:    name,
		Version: &v,
	}}
}

func TestCredentialsCheckProvider(t *testing.T) {
	tests := []struct {
		name     string
		args     newArgs
		packages []workspace.PackageDescriptor
		env      string
		expected string // provider package, or "" when no check should run
	}{
		{
			name:     "aws package interactive",
			args:     newArgs{interactive: true},
			packages: awsPackages(),
			expected: "aws",
		},
		{
			name:     "azure-native package interactive",
			args:     newArgs{interactive: true},
			packages: []workspace.PackageDescriptor{resourcePackage("azure-native", "3.0.0")},
			expected: "azure-native",
		},
		{
			name: "no packages",
			args: newArgs{interactive: true},
		},
		{
			name:     "classic azure package does not match",
			args:     newArgs{interactive: true},
			packages: []workspace.PackageDescriptor{resourcePackage("azure", "6.0.0")},
		},
		{
			name:     "gcp package interactive",
			args:     newArgs{interactive: true},
			packages: []workspace.PackageDescriptor{resourcePackage("gcp", "8.0.0")},
			expected: "gcp",
		},
		{
			name:     "random package",
			args:     newArgs{interactive: true},
			packages: []workspace.PackageDescriptor{resourcePackage("random", "4.0.0")},
		},
		{
			name:     "multiple packages",
			args:     newArgs{interactive: true},
			packages: []workspace.PackageDescriptor{resourcePackage("random", "4.0.0"), resourcePackage("aws", "7.0.0")},
			expected: "aws",
		},
		{
			name: "parameterized package on an aws base plugin does not match",
			args: newArgs{interactive: true},
			packages: func() []workspace.PackageDescriptor {
				pkg := resourcePackage("aws", "7.0.0")
				pkg.Parameterization = &workspace.Parameterization{Name: "other", Version: semver.MustParse("1.0.0")}
				return []workspace.PackageDescriptor{pkg}
			}(),
		},
		{
			name:     "non-interactive",
			args:     newArgs{interactive: false},
			packages: awsPackages(),
		},
		{
			name:     "generate-only",
			args:     newArgs{interactive: true, generateOnly: true},
			packages: awsPackages(),
		},
		{
			name:     "offline",
			args:     newArgs{interactive: true, offline: true},
			packages: awsPackages(),
		},
		{
			name:     "kill switch",
			args:     newArgs{interactive: true},
			packages: awsPackages(),
			env:      "true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("PULUMI_SKIP_NEW_CREDENTIALS_CHECK", tt.env)
			}
			cp, pkg, ok := credentialsCheckProvider(tt.args, tt.packages)
			assert.Equal(t, tt.expected != "", ok)
			assert.Equal(t, tt.expected, cp.pkg)
			if ok {
				// The exact package the program depends on is what gets loaded.
				assert.Equal(t, tt.expected, pkg.Name)
				assert.Equal(t, apitype.ResourcePlugin, pkg.Kind)
				require.NotNil(t, pkg.Version)
			}
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

	closed := false
	mock := &plugin.MockProvider{
		CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			assert.Equal(t, "pulumi:providers:aws", string(req.URN.Type()))
			assert.Empty(t, req.Name)
			assert.Empty(t, req.Type)
			assert.Equal(t, "us-east-1", req.News.Get("region").AsString())
			return plugin.CheckConfigResponse{},
				status.Error(codes.Unknown, "unable to validate AWS credentials.\nDetails: no valid credential sources found")
		},
		CloseF: func() error {
			closed = true
			return nil
		},
	}

	var buf bytes.Buffer
	news := property.NewMap(map[string]property.Value{"region": property.New("us-east-1")})
	warned := checkCloudCredentials(t.Context(), awsProvider,
		func() (plugin.Provider, error) { return mock, nil },
		news, &buf, display.Options{Color: colors.Never}, time.Second)
	assert.True(t, warned)

	out := buf.String()
	assert.Contains(t, out, "warning:")
	assert.Contains(t, out, "Could not validate your AWS credentials")
	assert.Contains(t, out, "unable to validate AWS credentials.")
	assert.Contains(t, out, "    Details: no valid credential sources found")
	assert.Contains(t, out, awsProvider.docURL)
	assert.True(t, closed)
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
	}

	var buf bytes.Buffer
	warned := checkCloudCredentials(t.Context(), awsProvider,
		func() (plugin.Provider, error) { return mock, nil },
		property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)
	assert.True(t, warned)

	out := buf.String()
	assert.Contains(t, out, "The AWS provider reported problems with this stack's configuration")
	assert.Contains(t, out, "    region: expected a valid region")
	assert.Contains(t, out, "    unable to validate AWS credentials.\n    Details: no valid credential sources found")
	assert.Contains(t, out, awsProvider.docURL)
}

func TestCheckCloudCredentialsSuccess(t *testing.T) {
	t.Parallel()

	configured := false
	mock := &plugin.MockProvider{
		CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			return plugin.CheckConfigResponse{Properties: req.News}, nil
		},
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

	var buf bytes.Buffer
	news := property.NewMap(map[string]property.Value{"region": property.New("us-east-1")})
	warned := checkCloudCredentials(t.Context(), awsProvider,
		func() (plugin.Provider, error) { return mock, nil },
		news, &buf, display.Options{Color: colors.Never}, time.Second)

	assert.False(t, warned)
	assert.True(t, configured)
	assert.Empty(t, buf.String())
}

func TestCheckCloudCredentialsConfigureError(t *testing.T) {
	t.Parallel()

	closed := false
	mock := &plugin.MockProvider{
		CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			return plugin.CheckConfigResponse{Properties: req.News}, nil
		},
		ConfigureF: func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
			return plugin.ConfigureResponse{},
				status.Error(codes.Unknown, "unable to validate AWS credentials.\nDetails: InvalidClientTokenId")
		},
		CloseF: func() error {
			closed = true
			return nil
		},
	}

	var buf bytes.Buffer
	warned := checkCloudCredentials(t.Context(), awsProvider,
		func() (plugin.Provider, error) { return mock, nil },
		property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)
	assert.True(t, warned)

	out := buf.String()
	assert.Contains(t, out, "Could not validate your AWS credentials")
	assert.Contains(t, out, "    unable to validate AWS credentials.\n    Details: InvalidClientTokenId")
	assert.NotContains(t, out, "rpc error")
	assert.Contains(t, out, awsProvider.docURL)
	assert.True(t, closed)
}

func TestCheckCloudCredentialsAzureConfigureError(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			assert.Equal(t, "pulumi:providers:azure-native", string(req.URN.Type()))
			return plugin.CheckConfigResponse{Properties: req.News}, nil
		},
		ConfigureF: func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
			assert.Equal(t, "pulumi:providers:azure-native", string(*req.Type))
			return plugin.ConfigureResponse{},
				status.Error(codes.Unknown, "failed to get authorizer: please run `az login`")
		},
	}

	var buf bytes.Buffer
	warned := checkCloudCredentials(t.Context(), azureProvider,
		func() (plugin.Provider, error) { return mock, nil },
		property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)
	assert.True(t, warned)

	out := buf.String()
	assert.Contains(t, out, "Could not validate your Azure credentials")
	assert.Contains(t, out, "    failed to get authorizer: please run `az login`")
	assert.Contains(t, out, "may fail until Azure credentials are configured")
	assert.Contains(t, out, azureProvider.docURL)
}

func TestCheckCloudCredentialsGCPConfigureError(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			assert.Equal(t, "pulumi:providers:gcp", string(req.URN.Type()))
			return plugin.CheckConfigResponse{Properties: req.News}, nil
		},
		ConfigureF: func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
			assert.Equal(t, "pulumi:providers:gcp", string(*req.Type))
			return plugin.ConfigureResponse{},
				status.Error(codes.Unknown, "google: could not find default credentials")
		},
	}

	var buf bytes.Buffer
	warned := checkCloudCredentials(t.Context(), gcpProvider,
		func() (plugin.Provider, error) { return mock, nil },
		property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)
	assert.True(t, warned)

	out := buf.String()
	assert.Contains(t, out, "Could not validate your Google Cloud credentials")
	assert.Contains(t, out, "    google: could not find default credentials")
	assert.Contains(t, out, "may fail until Google Cloud credentials are configured")
	assert.Contains(t, out, gcpProvider.docURL)
}

func TestCheckCloudCredentialsConfigureNotCalledOnCheckFailure(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			return plugin.CheckConfigResponse{Failures: []plugin.CheckFailure{{Reason: "bad"}}}, nil
		},
		ConfigureF: func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
			t.Fatal("Configure must not be called when CheckConfig fails")
			return plugin.ConfigureResponse{}, nil
		},
	}

	var buf bytes.Buffer
	warned := checkCloudCredentials(t.Context(), awsProvider,
		func() (plugin.Provider, error) { return mock, nil },
		property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)
	assert.True(t, warned)
	assert.Contains(t, buf.String(), "    bad")
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
			CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
				return plugin.CheckConfigResponse{Properties: req.News}, nil
			},
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

		var buf bytes.Buffer
		warned := checkCloudCredentials(t.Context(), awsProvider,
			func() (plugin.Provider, error) { return prov, nil },
			property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)
		assert.True(t, warned)
		assert.Contains(t, buf.String(), "Could not validate your AWS credentials")
		assert.Contains(t, buf.String(), "    missing required configuration key \"aws:region\"")
	})

	t.Run("timeout is silent", func(t *testing.T) {
		t.Parallel()

		prov := &awaitingProvider{MockProvider: newMock(), await: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}

		var buf bytes.Buffer
		warned := checkCloudCredentials(t.Context(), awsProvider,
			func() (plugin.Provider, error) { return prov, nil },
			property.Map{}, &buf, display.Options{Color: colors.Never}, 10*time.Millisecond)
		assert.False(t, warned)
		assert.Empty(t, buf.String())
	})
}

func TestCheckCloudCredentialsTimeout(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	mock := &plugin.MockProvider{
		CheckConfigF: func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			<-release
			return plugin.CheckConfigResponse{}, nil
		},
	}

	var buf bytes.Buffer
	var warned bool
	done := make(chan struct{})
	go func() {
		warned = checkCloudCredentials(t.Context(), awsProvider,
			func() (plugin.Provider, error) { return mock, nil },
			property.Map{}, &buf, display.Options{Color: colors.Never}, 10*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("checkCloudCredentials did not return after its timeout")
	}
	assert.False(t, warned)
	assert.Empty(t, buf.String())
}

func TestCheckCloudCredentialsLoaderError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	warned := checkCloudCredentials(t.Context(), awsProvider,
		func() (plugin.Provider, error) { return nil, errors.New("no such plugin") },
		property.Map{}, &buf, display.Options{Color: colors.Never}, time.Second)

	assert.False(t, warned)
	assert.Empty(t, buf.String())
}
