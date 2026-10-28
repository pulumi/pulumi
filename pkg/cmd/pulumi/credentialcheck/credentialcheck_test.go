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

package credentialcheck

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const awsDocURL = "https://www.pulumi.com/registry/packages/aws/installation-configuration/"

var (
	awsProvider   = Provider{Pkg: "aws", DisplayName: "AWS", DocURL: awsDocURL}
	azureProvider = Provider{Pkg: "azure-native", DisplayName: "Azure"}
	gcpProvider   = Provider{Pkg: "gcp", DisplayName: "Google Cloud"}
)

func schemaProvider(schemaJSON string) *plugin.MockProvider {
	return &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
		},
	}
}

func runCheck(t *testing.T, cp Provider, prov plugin.Provider, news property.Map) (bool, string) {
	t.Helper()
	err := Probe(t.Context(), cp, prov, news)
	if err == nil {
		return false, ""
	}
	return true, err.Error()
}

func echoCheckConfig(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func TestFromSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schema   string
		err      error
		expected Provider
		wantErr  bool
	}{
		{
			name: "opted in",
			schema: `{"name":"aws","displayName":"AWS","validateCredentialsOnNew":true,` +
				`"configurationDocsUrl":"` + awsDocURL + `"}`,
			expected: Provider{Pkg: "aws", DisplayName: "AWS", DocURL: awsDocURL, Supported: true},
		},
		{
			name:     "opted in without display name or docs",
			schema:   `{"name":"aws","validateCredentialsOnNew":true}`,
			expected: Provider{Pkg: "aws", DisplayName: "aws", Supported: true},
		},
		{
			name:     "not opted in",
			schema:   `{"name":"aws","displayName":"AWS","configurationDocsUrl":"` + awsDocURL + `"}`,
			expected: Provider{Pkg: "aws", DisplayName: "AWS", DocURL: awsDocURL},
		},
		{
			name:     "explicitly opted out",
			schema:   `{"name":"random","validateCredentialsOnNew":false}`,
			expected: Provider{Pkg: "random", DisplayName: "random"},
		},
		{name: "invalid schema", schema: `not json`, wantErr: true},
		{name: "schema error", err: errors.New("boom"), wantErr: true},
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
			cp, err := FromSchema(t.Context(), prov, plugin.GetSchemaRequest{})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, cp)
		})
	}
}

func TestConfigProperties(t *testing.T) {
	t.Parallel()

	cfg := config.Map{
		config.MustMakeKey("aws", "region"):            config.NewValue("us-east-1"),
		config.MustMakeKey("aws", "secret"):            config.NewSecureValue("ciphertext"),
		config.MustMakeKey("azure-native", "location"): config.NewValue("WestUS2"),
		config.MustMakeKey("gcp", "project"):           config.NewValue("my-project"),
		config.MustMakeKey("proj", "etcetc"):           config.NewValue("unrelated"),
	}
	// Values that cannot be decrypted are skipped.
	noSecrets := config.NewErrorCrypter("no secrets")
	props := ConfigProperties(awsProvider.Pkg, cfg, noSecrets)
	require.Equal(t, 1, props.Len())
	assert.Equal(t, "us-east-1", props.Get("region").AsString())

	props = ConfigProperties(azureProvider.Pkg, cfg, noSecrets)
	require.Equal(t, 1, props.Len())
	assert.Equal(t, "WestUS2", props.Get("location").AsString())

	props = ConfigProperties(gcpProvider.Pkg, cfg, noSecrets)
	require.Equal(t, 1, props.Len())
	assert.Equal(t, "my-project", props.Get("project").AsString())

	props = ConfigProperties(awsProvider.Pkg, cfg, config.NopDecrypter)
	require.Equal(t, 2, props.Len())
	assert.Equal(t, "us-east-1", props.Get("region").AsString())
	assert.Equal(t, "ciphertext", props.Get("secret").AsString())
}

func TestProbeRPCError(t *testing.T) {
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
	warned, out := runCheck(t, awsProvider, mock, news)
	assert.True(t, warned)
	assert.Contains(t, out, "Could not validate your AWS credentials")
	assert.Contains(t, out, "unable to validate AWS credentials.")
	assert.Contains(t, out, "    Details: no valid credential sources found")
	assert.NotContains(t, out, "rpc error")
	assert.Contains(t, out, "For help configuring the AWS provider, see "+awsDocURL)
}

func TestProbeFailures(t *testing.T) {
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

	warned, out := runCheck(t, awsProvider, mock, property.Map{})
	assert.True(t, warned)
	assert.Contains(t, out, "The AWS provider reported problems with its configuration")
	assert.Contains(t, out, "    region: expected a valid region")
	assert.Contains(t, out, "    unable to validate AWS credentials.\n    Details: no valid credential sources found")
	assert.Contains(t, out, awsDocURL)
}

func TestProbeSuccess(t *testing.T) {
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
			assert.Equal(t, "us-east-1", req.Inputs.Get("region").AsString())
			return plugin.ConfigureResponse{}, nil
		},
	}

	news := property.NewMap(map[string]property.Value{"region": property.New("us-east-1")})
	warned, out := runCheck(t, awsProvider, mock, news)
	assert.False(t, warned)
	assert.True(t, configured)
	assert.Empty(t, out)
}

func TestProbeConfigureError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		provider Provider
		message  string
	}{
		{awsProvider, "unable to validate AWS credentials.\nDetails: InvalidClientTokenId"},
		{azureProvider, "failed to get authorizer: please run `az login`"},
		{gcpProvider, "google: could not find default credentials"},
	}
	for _, tt := range tests {
		t.Run(tt.provider.Pkg, func(t *testing.T) {
			t.Parallel()

			providerType := "pulumi:providers:" + tt.provider.Pkg
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

			warned, out := runCheck(t, tt.provider, mock, property.Map{})
			assert.True(t, warned)
			assert.Contains(t, out, "Could not validate your "+tt.provider.DisplayName+" credentials")
			for line := range strings.SplitSeq(tt.message, "\n") {
				assert.Contains(t, out, "    "+line)
			}
			assert.NotContains(t, out, "rpc error")
			if tt.provider.DocURL != "" {
				assert.Contains(t, out, "For help configuring the "+tt.provider.DisplayName+" provider, see "+tt.provider.DocURL)
			} else {
				// Providers that publish no docs link get no help line rather than a dangling one.
				assert.NotContains(t, out, "For help configuring")
			}
		})
	}
}

func TestProbeErrorIncludesDocURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		checkFailures []plugin.CheckFailure
		configureErr  error
	}{
		{
			name:         "configure error",
			configureErr: status.Error(codes.Unknown, "unable to validate AWS credentials.\nSee "+awsDocURL),
		},
		{
			name: "check failure",
			checkFailures: []plugin.CheckFailure{
				{Reason: "unable to validate AWS credentials.\nSee " + awsDocURL},
			},
		},
		{
			name: "without trailing slash",
			configureErr: status.Error(codes.Unknown,
				"unable to validate AWS credentials.\nSee "+strings.TrimSuffix(awsDocURL, "/")+" for details"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &plugin.MockProvider{
				CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
					return plugin.CheckConfigResponse{Properties: req.News, Failures: tt.checkFailures}, nil
				},
				ConfigureF: func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
					return plugin.ConfigureResponse{}, tt.configureErr
				},
			}

			warned, out := runCheck(t, awsProvider, mock, property.Map{})
			assert.True(t, warned)
			assert.Equal(t, 1, strings.Count(out, strings.TrimSuffix(awsDocURL, "/")))
			assert.NotContains(t, out, "For help configuring")
		})
	}
}

func TestProbeConfigureTimeout(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: echoCheckConfig,
		ConfigureF: func(ctx context.Context, _ plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
			<-ctx.Done()
			return plugin.ConfigureResponse{}, ctx.Err()
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	err := Probe(ctx, awsProvider, mock, property.Map{})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestProbeTimeout(t *testing.T) {
	t.Parallel()

	mock := &plugin.MockProvider{
		CheckConfigF: func(ctx context.Context, _ plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			<-ctx.Done()
			return plugin.CheckConfigResponse{}, ctx.Err()
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	err := Probe(ctx, awsProvider, mock, property.Map{})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
