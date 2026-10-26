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

package auth

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const awsSchema = `{"name":"aws","displayName":"AWS","validateCredentialsOnNew":true,` +
	`"configurationDocsUrl":"https://www.pulumi.com/registry/packages/aws/installation-configuration/"}`

func credentialsProvider(schema string) *plugin.MockProvider {
	return &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			return plugin.GetSchemaResponse{Schema: []byte(schema)}, nil
		},
		CheckConfigF: func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			return plugin.CheckConfigResponse{Properties: req.News}, nil
		},
		ConfigureF: func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
			return plugin.ConfigureResponse{}, nil
		},
	}
}

func runCheckCredentials(t *testing.T, prov plugin.Provider, cfg checkConfig) (string, error) {
	t.Helper()
	var stdout bytes.Buffer
	err := checkCredentials(t.Context(), &stdout, prov, cfg)
	return stdout.String(), err
}

func TestCheckCredentialsSuccess(t *testing.T) {
	t.Parallel()

	stdout, err := runCheckCredentials(t, credentialsProvider(awsSchema), checkConfig{})
	require.NoError(t, err)
	assert.Contains(t, stdout, "No stack selected; using credentials from the environment")
	assert.Contains(t, stdout, "Your AWS credentials are valid")
}

func TestCheckCredentialsUsesStackConfig(t *testing.T) {
	t.Parallel()

	prov := credentialsProvider(awsSchema)
	prov.ConfigureF = func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
		assert.Equal(t, "pulumi:providers:aws", string(*req.Type))
		assert.Equal(t, "my-profile", req.Inputs.Get("profile").AsString())
		assert.Equal(t, "ciphertext", req.Inputs.Get("secretKey").AsString())
		assert.Equal(t, 2, req.Inputs.Len())
		return plugin.ConfigureResponse{}, nil
	}
	cfg := checkConfig{
		stack: "dev",
		cfg: config.Map{
			config.MustMakeKey("aws", "profile"):    config.NewValue("my-profile"),
			config.MustMakeKey("aws", "secretKey"):  config.NewSecureValue("ciphertext"),
			config.MustMakeKey("proj", "unrelated"): config.NewValue("value"),
		},
		decrypter: config.NopDecrypter,
	}

	stdout, err := runCheckCredentials(t, prov, cfg)
	require.NoError(t, err)
	assert.Contains(t, stdout, "Using configuration from stack dev")
	assert.Contains(t, stdout, "Your AWS credentials are valid")
}

func TestCheckCredentialsConfigureError(t *testing.T) {
	t.Parallel()

	prov := credentialsProvider(awsSchema)
	prov.ConfigureF = func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
		return plugin.ConfigureResponse{}, status.Error(codes.Unknown,
			"unable to validate AWS credentials.\nDetails: no valid credential sources found")
	}

	stdout, err := runCheckCredentials(t, prov, checkConfig{})
	require.ErrorContains(t, err, "Could not validate your AWS credentials:")
	assert.ErrorContains(t, err, "\n    Details: no valid credential sources found")
	assert.ErrorContains(t, err, "\nFor help configuring the AWS provider, see ")
	assert.NotContains(t, stdout, "credentials are valid")
}

func TestCheckCredentialsCheckConfigFailures(t *testing.T) {
	t.Parallel()

	prov := credentialsProvider(awsSchema)
	prov.CheckConfigF = func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
		return plugin.CheckConfigResponse{Failures: []plugin.CheckFailure{
			{Property: "region", Reason: "expected a valid region"},
		}}, nil
	}

	_, err := runCheckCredentials(t, prov, checkConfig{})
	require.ErrorContains(t, err, "\n    region: expected a valid region")
}

func TestCheckCredentialsNotSupported(t *testing.T) {
	t.Parallel()

	prov := credentialsProvider(`{"name":"random"}`)
	prov.CheckConfigF = func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
		t.Fatal("CheckConfig must not be called for a provider that did not opt in")
		return plugin.CheckConfigResponse{}, nil
	}

	_, err := runCheckCredentials(t, prov, checkConfig{})
	require.ErrorContains(t, err, "the random provider does not support credential checks")
}

func TestCheckCredentialsCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	prov := credentialsProvider(awsSchema)
	prov.ConfigureF = func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
		cancel()
		return plugin.ConfigureResponse{}, context.Canceled
	}

	var stdout bytes.Buffer
	err := checkCredentials(ctx, &stdout, prov, checkConfig{})
	require.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, stdout.String(), "credentials are valid")
}

func TestLoadCheckConfigWithoutProject(t *testing.T) {
	t.Parallel()

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	opts := display.Options{Color: colors.Never}

	cfg, err := loadCheckConfig(t.Context(), sink, ws, &cmdBackend.MockLoginManager{}, "", opts)
	require.NoError(t, err)
	assert.Equal(t, checkConfig{}, cfg)

	_, err = loadCheckConfig(t.Context(), sink, ws, &cmdBackend.MockLoginManager{}, "dev", opts)
	require.ErrorContains(t, err, "--stack can only be used inside a Pulumi project")
}

func TestLoadCheckConfigNotLoggedIn(t *testing.T) {
	t.Setenv("PULUMI_BACKEND_URL", "https://api.example.com")

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return &workspace.Project{Name: "proj"}, "", nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		CurrentF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool,
		) (backend.Backend, error) {
			return nil, nil
		},
		LoginF: func(
			context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool,
			colors.Colorization, ...cmdBackend.LoginOption,
		) (backend.Backend, error) {
			t.Fatal("the check must not log in when no stack is given")
			return nil, nil
		},
	}
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})

	cfg, err := loadCheckConfig(t.Context(), sink, ws, lm, "", display.Options{Color: colors.Never})
	require.NoError(t, err)
	assert.Equal(t, checkConfig{}, cfg)
}
