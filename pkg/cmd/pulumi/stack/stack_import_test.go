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

package stack

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain also runs this test executable as a credential helper subprocess.
// Tests use os.Executable() with -credential-helper-test and a JSON response or responses keyed by backend URL.
// It reads the backend URL from stdin and writes its response to stdout. Other invocations run tests normally.
func TestMain(m *testing.M) {
	for i, arg := range os.Args {
		if arg != "-credential-helper-test" {
			continue
		}
		response := os.Args[i+1]
		var request struct {
			SelectedBackendURL string `json:"selectedBackendUrl"`
		}
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
			os.Exit(1)
		}
		var responses map[string]json.RawMessage
		if json.Unmarshal([]byte(response), &responses) == nil {
			if selected, ok := responses[request.SelectedBackendURL]; ok {
				response = string(selected)
			}
		}
		_, err := os.Stdout.WriteString(response)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func stackHelperClient(t *testing.T, backendURL, response string) (*httpstate.ClientSource, *client.Client) {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	session := credentialhelper.NewSession(io.Discard)
	require.NoError(t, session.Configure(executable, []string{"--", "-credential-helper-test", response}))
	resolved, err := session.Resolve(t.Context(), backendURL)
	require.NoError(t, err)
	require.NoError(t, session.SelectBackend(resolved, backendURL))
	source := httpstate.NewClientSource(session, nil)
	c, err := source.ClientForURL(t.Context(), backendURL, false)
	require.NoError(t, err)
	return source, c
}

//nolint:paralleltest // Isolates process credentials.
func TestStackImportHelperCredentials(t *testing.T) {
	ptesting.IsolateCredentials(t)
	ciphertext := base64.StdEncoding.EncodeToString([]byte("source-ciphertext"))
	// The target uses a helper token to encrypt imported secrets. Tt must not receive the source header.
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token helper-token", r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("X-Source-Session"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/capabilities":
			_ = json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{})
		case "/api/stacks/target-owner/target-project/target-stack/encrypt":
			var req apitype.EncryptValueRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decoding encryption request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			assert.Equal(t, `"secret-value"`, string(req.Plaintext))
			_ = json.NewEncoder(w).Encode(apitype.EncryptValueResponse{Ciphertext: []byte("target-ciphertext")})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer targetServer.Close()

	// The source uses a helper header to decrypt the checkpoint. it must not receive the target token.
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "source-session", r.Header.Get("X-Source-Session"))
		assert.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/capabilities":
			_ = json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{})
		case "/api/stacks/source-owner/source-project/source-stack/batch-decrypt":
			_ = json.NewEncoder(w).Encode(apitype.BatchDecryptResponse{
				Plaintexts: map[string][]byte{ciphertext: []byte(`"secret-value"`)},
			})
		default:
			t.Errorf("unexpected source request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer sourceServer.Close()
	sourceURL := sourceServer.URL
	// One helper returns different credentials depending on the backend URL in its request.
	responses, err := json.Marshal(map[string]json.RawMessage{
		targetServer.URL: json.RawMessage(`{"version":1,"accessToken":"helper-token"}`),
		sourceURL:        json.RawMessage(`{"version":1,"headers":{"X-Source-Session":["source-session"]}}`),
	})
	require.NoError(t, err)
	source, apiClient := stackHelperClient(t, targetServer.URL, string(responses))
	require.NoError(t, credentialhelper.SetCurrent(targetServer.URL))

	// The checkpoint's secrets manager still points to the source backend.
	state, err := json.Marshal(map[string]string{
		"url": sourceURL, "owner": "source-owner", "project": "source-project", "stack": "source-stack",
	})
	require.NoError(t, err)
	deploymentJSON, err := json.Marshal(apitype.DeploymentV3{
		SecretsProviders: &apitype.SecretsProvidersV1{Type: service.Type, State: state},
		Resources: []apitype.ResourceV3{{
			URN:  "urn:pulumi:target-stack::target-project::pulumi:pulumi:Stack::target-project-target-stack",
			Type: resource.RootStackType,
			Outputs: map[string]any{"secret": map[string]any{
				resource.SigKey: resource.SecretSig, "ciphertext": ciphertext,
			}},
		}},
	})
	require.NoError(t, err)
	input, err := json.Marshal(apitype.UntypedDeployment{Version: 3, Deployment: deploymentJSON})
	require.NoError(t, err)
	be := &httpstate.MockHTTPBackend{FClient: func() *client.Client { return apiClient }}
	ref := &backend.MockStackReference{
		StringV: "target-owner/target-project/target-stack", NameV: tokens.MustParseStackName("target-stack"),
	}
	stk := &backend.MockStack{
		BackendF: func() backend.Backend { return be }, RefF: func() backend.StackReference { return ref },
		DefaultSecretManagerF: func(ctx context.Context, info *workspace.ProjectStack) (secrets.Manager, error) {
			return service.NewServiceSecretsManager(ctx, apiClient, client.StackIdentifier{
				Owner: "target-owner", Project: "target-project", Stack: ref.Name(),
			}, info)
		},
	}
	be.GetStackF = func(context.Context, backend.StackReference) (backend.Stack, error) { return stk, nil }
	var imported *apitype.UntypedDeployment
	be.ImportDeploymentF = func(_ context.Context, _ backend.Stack, deployment *apitype.UntypedDeployment) error {
		imported = deployment
		return nil
	}
	lm := &cmdBackend.MockLoginManager{Clients: source, LoginF: func(
		context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
		...cmdBackend.LoginOption,
	) (backend.Backend, error) {
		return be, nil
	}}
	cmd := newStackImportCmd(&pkgWorkspace.MockContext{}, lm, cmdBackend.SecretsProvider(lm))
	cmd.SetIn(bytes.NewReader(input))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--stack", ref.String()})
	// Import must decrypt with the source credentials, then re-encrypt with the target credentials.
	err = cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	require.NotNil(t, imported)
	var deployment apitype.DeploymentV3
	require.NoError(t, json.Unmarshal(imported.Deployment, &deployment))
	assert.JSONEq(t,
		`{"url":"`+targetServer.URL+`","owner":"target-owner","project":"target-project","stack":"target-stack"}`,
		string(deployment.SecretsProviders.State))
	assert.Equal(t, map[string]any{
		resource.SigKey: resource.SecretSig, "ciphertext": base64.StdEncoding.EncodeToString([]byte("target-ciphertext")),
	}, deployment.Resources[0].Outputs["secret"])
	assert.NotContains(t, string(imported.Deployment), "secret-value")

	// Neither backend's helper credentials are persisted, and the target remains selected.
	account, err := workspace.GetAccount(targetServer.URL)
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)
	creds, err := workspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, targetServer.URL, creds.Current)
	assert.Empty(t, creds.Accounts[sourceURL].AccessToken)
}

// Regression test for https://github.com/pulumi/pulumi/issues/20956. Ensure that if we import a deployment
// file that's using the "service" secrets manager that we reconfigure it for the target stack, rather than
// just reusing the existing configuration which might be pointing to a different stack.
func TestStackImport_ChangeServiceSecrets(t *testing.T) {
	t.Parallel()

	settings := &pkgWorkspace.Settings{
		Stack: "org/proj/stk", //nolint:staticcheck
	}
	w := &pkgWorkspace.MockW{
		SettingsF: func() *pkgWorkspace.Settings {
			return settings
		},
	}

	newSmState := json.RawMessage(`{"url":"https://api.pulumi.com","owner":"org","project":"proj","stack":"stk"}`)
	newSm := &secrets.MockSecretsManager{
		TypeF: func() string { return "service" },
		StateF: func() json.RawMessage {
			return newSmState
		},
		DecrypterF: func() config.Decrypter {
			return config.NopDecrypter
		},
		EncrypterF: func() config.Encrypter {
			return config.NopEncrypter
		},
	}

	var be *backend.MockBackend
	stk := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV: tokens.MustParseStackName("stk"),
			}
		},
		BackendF: func() backend.Backend { return be },
		DefaultSecretManagerF: func(_ context.Context, info *workspace.ProjectStack) (secrets.Manager, error) {
			return newSm, nil
		},
	}
	be = &backend.MockBackend{
		URLF: func() string { return "https://api.pulumi.com" },
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			assert.Equal(t, "org/proj/stk", ref.String())
			return stk, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, deployment *apitype.UntypedDeployment) error {
			// Assert that the secrets manager being used for the import is the new one created for the target stack
			v3deployment, err := stack.UnmarshalUntypedDeployment(ctx, deployment)
			if err != nil {
				return err
			}
			assert.Equal(t, "service", v3deployment.SecretsProviders.Type)
			assert.Equal(t, newSmState, v3deployment.SecretsProviders.State)
			return nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		NewF: func(_ string) (pkgWorkspace.W, error) {
			return w, nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
			project *workspace.Project, setCurrent, insecure bool, color colors.Colorization, _ ...cmdBackend.LoginOption,
		) (backend.Backend, error) {
			return be, nil
		},
	}
	sp := &secrets.MockProvider{}
	var smState json.RawMessage
	sm := &secrets.MockSecretsManager{
		TypeF: func() string { return "service" },
		StateF: func() json.RawMessage {
			return smState
		},
		DecrypterF: func() config.Decrypter {
			return config.NopDecrypter
		},
		EncrypterF: func() config.Encrypter {
			return config.NopEncrypter
		},
	}
	sp = sp.Add("service", func(state json.RawMessage) (secrets.Manager, error) {
		smState = state
		return sm, nil
	})

	cmd := newStackImportCmd(ws, lm, sp)

	var stdinBuf bytes.Buffer
	importDeployment := `{
	"version": 3,
	"deployment": {
		"secrets_providers": {
			"type": "service",
			"state": {
				"url": "https://api.pulumi.com",
				"owner": "src-org",
				"project": "proj",
				"stack": "stk"
			}
		}
	}
}`
	stdinBuf.WriteString(importDeployment)

	var stdoutBuf bytes.Buffer
	cmd.SetOut(&stdoutBuf)
	cmd.SetIn(&stdinBuf)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
}

// Repro for https://github.com/pulumi/pulumi/issues/21846.
// Importing a deployment with service secrets should succeed even if the target
// stack's default secrets manager mutates ProjectStack as a side effect.
func TestStackImport_ServiceSecrets_DefaultSecretManagerMutatesProjectStack(t *testing.T) { //nolint: paralleltest
	wd := t.TempDir()
	t.Chdir(wd)

	// Write out a minimal Pulumi.yaml to ensure we have a project to load.
	err := os.WriteFile("Pulumi.yaml", []byte("name: proj\nruntime: mock"), 0o600)
	require.NoError(t, err)
	// And write a minimal Pulumi.<stack>.yaml to ensure we have a stack to load.
	err = os.WriteFile("Pulumi.stk.yaml", []byte("config:\n  proj:key: value"), 0o600)
	require.NoError(t, err)

	w := &pkgWorkspace.MockW{
		SettingsF: func() *pkgWorkspace.Settings {
			return &pkgWorkspace.Settings{
				Stack: "org/proj/stk", //nolint:staticcheck
			}
		},
	}

	newSm := &secrets.MockSecretsManager{
		TypeF:  func() string { return "passphrase" },
		StateF: func() json.RawMessage { return nil },
		DecrypterF: func() config.Decrypter {
			return config.NopDecrypter
		},
		EncrypterF: func() config.Encrypter {
			return config.NopEncrypter
		},
	}

	var be *backend.MockBackend
	stk := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV: tokens.MustParseStackName("stk"),
			}
		},
		BackendF: func() backend.Backend { return be },
		DefaultSecretManagerF: func(_ context.Context, info *workspace.ProjectStack) (secrets.Manager, error) {
			// Simulate a backend that configures secrets in ProjectStack as a side effect.
			info.SecretsProvider = "passphrase"
			info.EncryptionSalt = "v1:dummy"
			return newSm, nil
		},
	}
	importCalled := false
	be = &backend.MockBackend{
		URLF: func() string { return "https://api.pulumi.com" },
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			assert.Equal(t, "org/proj/stk", ref.String())
			return stk, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, deployment *apitype.UntypedDeployment) error {
			importCalled = true

			v3deployment, err := stack.UnmarshalUntypedDeployment(ctx, deployment)
			if err != nil {
				return err
			}

			// Ensure the imported deployment was rewritten to use the stack's
			// default secrets manager.
			assert.Equal(t, "passphrase", v3deployment.SecretsProviders.Type)
			assert.Nil(t, v3deployment.SecretsProviders.State)
			return nil
		},
	}

	ws := &pkgWorkspace.MockContext{
		NewF: func(_ string) (pkgWorkspace.W, error) {
			return w, nil
		},
		ReadProjectF: func(_ string) (*workspace.Project, string, error) {
			return &workspace.Project{Name: "proj"}, "Pulumi.yaml", nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
			project *workspace.Project, setCurrent, insecure bool, color colors.Colorization, _ ...cmdBackend.LoginOption,
		) (backend.Backend, error) {
			return be, nil
		},
	}
	var smState json.RawMessage
	sm := &secrets.MockSecretsManager{
		TypeF: func() string { return "service" },
		StateF: func() json.RawMessage {
			return smState
		},
		DecrypterF: func() config.Decrypter {
			return config.NopDecrypter
		},
		EncrypterF: func() config.Encrypter {
			return config.NopEncrypter
		},
	}
	sp := (&secrets.MockProvider{}).Add(
		"service", func(state json.RawMessage) (secrets.Manager, error) {
			smState = state
			return sm, nil
		})

	cmd := newStackImportCmd(ws, lm, sp)

	var stdinBuf bytes.Buffer
	importDeployment := `{
	"version": 3,
	"deployment": {
		"secrets_providers": {
			"type": "service",
			"state": {
				"url": "https://api.pulumi.com",
				"owner": "src-org",
				"project": "proj",
				"stack": "stk"
			}
		}
	}
}`
	stdinBuf.WriteString(importDeployment)

	var stdoutBuf bytes.Buffer
	cmd.SetOut(&stdoutBuf)
	cmd.SetIn(&stdinBuf)
	cmd.SetArgs([]string{})

	err = cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	require.True(t, importCalled)

	// Assert the config file was updated
	configBytes, err := os.ReadFile("Pulumi.stk.yaml")
	require.NoError(t, err)
	assert.Contains(t, string(configBytes), "secretsprovider: passphrase")
	assert.Contains(t, string(configBytes), "encryptionsalt: v1:dummy")
	assert.Contains(t, string(configBytes), "proj:key: value")
}
