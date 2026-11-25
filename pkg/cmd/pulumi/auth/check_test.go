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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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

func runCheckCredentials(t *testing.T, prov plugin.Provider, target *deploy.Target) (string, error) {
	t.Helper()
	var stdout bytes.Buffer
	err := checkCredentials(t.Context(), &stdout, prov, target)
	return stdout.String(), err
}

func TestCheckCredentialsSuccess(t *testing.T) {
	t.Parallel()

	stdout, err := runCheckCredentials(t, credentialsProvider(awsSchema), nil)
	require.NoError(t, err)
	assert.Contains(t, stdout, "No stack selected; using credentials from the environment")
	assert.Contains(t, stdout, "Your AWS credentials are valid")
}

func TestCheckCredentialsUsesStack(t *testing.T) {
	t.Parallel()

	prov := credentialsProvider(awsSchema)
	prov.ConfigureF = func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
		assert.Equal(t, "my-profile", req.Inputs.Get("profile").AsString())
		return plugin.ConfigureResponse{}, nil
	}

	stdout, err := runCheckCredentials(t, prov, &deploy.Target{
		Name:      tokens.MustParseStackName("dev"),
		Config:    config.Map{config.MustMakeKey("aws", "profile"): config.NewValue("my-profile")},
		Decrypter: config.NopDecrypter,
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "Using configuration from stack dev")
	assert.Contains(t, stdout, "Your AWS credentials are valid")
}

func TestStackTargetMergesEnvironmentAndProjectConfig(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()
	proj := &workspace.Project{
		Name: "proj",
		Config: map[string]workspace.ProjectConfigType{
			"aws:region": {Value: "us-west-2"},
			"aws:token":  {Value: "from-project"},
		},
	}
	sc := backend.StackConfiguration{
		Environment: esc.NewValue(map[string]esc.Value{
			"aws:profile":   esc.NewValue("from-esc"),
			"aws:secretKey": {Value: "from-esc-secret", Secret: true},
			"aws:token":     esc.NewValue("from-esc"),
		}),
		Config:    config.Map{config.MustMakeKey("aws", "token"): config.NewValue("from-stack")},
		Decrypter: sm.Decrypter(),
	}

	target, err := stackTarget(t.Context(), tokens.MustParseStackName("dev"), proj, sc, sm)
	require.NoError(t, err)

	prov := credentialsProvider(awsSchema)
	configured := false
	prov.ConfigureF = func(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
		configured = true
		assert.Equal(t, "us-west-2", req.Inputs.Get("region").AsString())
		assert.Equal(t, "from-esc", req.Inputs.Get("profile").AsString())
		assert.Equal(t, "from-esc-secret", req.Inputs.Get("secretKey").AsString())
		assert.True(t, req.Inputs.Get("secretKey").Secret())
		assert.Equal(t, "from-stack", req.Inputs.Get("token").AsString())
		return plugin.ConfigureResponse{}, nil
	}
	_, err = runCheckCredentials(t, prov, target)
	require.NoError(t, err)
	assert.True(t, configured)
}

func TestStackTargetDoesNotRequireProjectConfig(t *testing.T) {
	t.Parallel()

	proj := &workspace.Project{
		Name:   "proj",
		Config: map[string]workspace.ProjectConfigType{"required": {}},
	}
	sm := b64.NewBase64SecretsManager()
	sc := backend.StackConfiguration{Config: config.Map{}, Decrypter: sm.Decrypter()}

	_, err := stackTarget(t.Context(), tokens.MustParseStackName("dev"), proj, sc, sm)
	require.NoError(t, err)
}

func TestCheckCredentialsSecretCannotBeDecrypted(t *testing.T) {
	t.Parallel()

	stdout, err := runCheckCredentials(t, credentialsProvider(awsSchema), &deploy.Target{
		Name:      tokens.MustParseStackName("dev"),
		Config:    config.Map{config.MustMakeKey("aws", "secretKey"): config.NewSecureValue("ciphertext")},
		Decrypter: config.NewErrorCrypter("bad passphrase"),
	})
	require.ErrorContains(t, err, "bad passphrase")
	assert.NotContains(t, stdout, "credentials are valid")
}

func TestCheckCredentialsRejected(t *testing.T) {
	t.Parallel()

	prov := credentialsProvider(awsSchema)
	prov.ConfigureF = func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
		return plugin.ConfigureResponse{}, status.Error(codes.Unknown, "no valid credential sources found")
	}

	stdout, err := runCheckCredentials(t, prov, nil)
	require.ErrorContains(t, err, "Could not validate your AWS credentials:")
	assert.NotContains(t, stdout, "credentials are valid")
}

func TestCheckCredentialsNotSupported(t *testing.T) {
	t.Parallel()

	prov := credentialsProvider(`{"name":"random"}`)
	prov.CheckConfigF = func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
		t.Fatal("CheckConfig must not be called for a provider that did not opt in")
		return plugin.CheckConfigResponse{}, nil
	}

	stdout, err := runCheckCredentials(t, prov, nil)
	require.ErrorContains(t, err, "the random provider does not support credential checks")
	assert.NotContains(t, stdout, "credentials are valid")
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
	err := checkCredentials(ctx, &stdout, prov, nil)
	require.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, stdout.String(), "credentials are valid")
}

func TestLoadCheckTargetWithoutProject(t *testing.T) {
	t.Parallel()

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func(string) (*workspace.Project, string, error) {
			return nil, "", workspace.ErrProjectNotFound
		},
	}
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	opts := display.Options{Color: colors.Never}

	target, err := loadCheckTarget(t.Context(), sink, ws, &cmdBackend.MockLoginManager{}, "", opts)
	require.NoError(t, err)
	assert.Nil(t, target)

	_, err = loadCheckTarget(t.Context(), sink, ws, &cmdBackend.MockLoginManager{}, "dev", opts)
	require.ErrorContains(t, err, "--stack can only be used inside a Pulumi project")
}

func TestLoadCheckTargetNotLoggedIn(t *testing.T) {
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

	target, err := loadCheckTarget(t.Context(), sink, ws, lm, "", display.Options{Color: colors.Never})
	require.NoError(t, err)
	assert.Nil(t, target)
}
