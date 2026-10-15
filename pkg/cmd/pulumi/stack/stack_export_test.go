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
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exportTestBackend struct {
	httpstate.MockHTTPBackend
	deployment *apitype.UntypedDeployment
	version    string
}

func (b *exportTestBackend) ExportDeploymentForVersion(
	_ context.Context, _ backend.Stack, version string,
) (*apitype.UntypedDeployment, error) {
	b.version = version
	return b.deployment, nil
}

// TestMain also serves as the credentials helper subprocess.
func TestMain(m *testing.M) {
	if len(os.Args) > 2 && os.Args[len(os.Args)-2] == "--stack-credentials-helper" {
		response := os.Args[len(os.Args)-1]
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
	session := credentialhelper.NewSession()
	require.NoError(t, session.Configure(executable, []string{"--", "--stack-credentials-helper", response}))
	resolved, err := session.Resolve(t.Context(), backendURL)
	require.NoError(t, err)
	require.NoError(t, session.SelectBackend(resolved, backendURL))
	source := httpstate.NewClientSource(session, nil)
	c, err := source.ClientForURL(t.Context(), backendURL, false)
	require.NoError(t, err)
	return source, c
}

//nolint:paralleltest // Isolates process credentials.
func TestStackExportHelperCredentials(t *testing.T) {
	ptesting.IsolateCredentials(t)
	for _, auth := range []struct {
		name, response, header, value, version string
	}{
		{"token", `{"version":1,"accessToken":"helper-token"}`, "Authorization", "token helper-token", ""},
		{"headers", `{"version":1,"headers":{"X-Session":["helper-session"]}}`, "X-Session", "helper-session", "42"},
	} {
		t.Run(auth.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, auth.value, r.Header.Get(auth.header))
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/capabilities":
					_ = json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{})
				case "/api/stacks/owner/project/stack/batch-decrypt":
					_ = json.NewEncoder(w).Encode(apitype.BatchDecryptResponse{
						Plaintexts: map[string][]byte{base64.StdEncoding.EncodeToString([]byte("ciphertext")): []byte(`"secret-value"`)},
					})
				case "/api/stacks/owner/project/stack/decrypt":
					_ = json.NewEncoder(w).Encode(apitype.DecryptValueResponse{Plaintext: []byte(`"secret-value"`)})
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			source, _ := stackHelperClient(t, server.URL, auth.response)
			state, err := json.Marshal(map[string]string{
				"url": server.URL, "owner": "owner", "project": "project", "stack": "stack",
			})
			require.NoError(t, err)
			deploymentJSON, err := json.Marshal(apitype.DeploymentV3{
				SecretsProviders: &apitype.SecretsProvidersV1{Type: "service", State: state},
				Resources: []apitype.ResourceV3{{
					URN: "urn:pulumi:stack::project::pulumi:pulumi:Stack::project-stack", Type: resource.RootStackType,
					Outputs: map[string]any{"secret": map[string]any{
						resource.SigKey: resource.SecretSig, "ciphertext": base64.StdEncoding.EncodeToString([]byte("ciphertext")),
					}},
				}},
			})
			require.NoError(t, err)
			be := &exportTestBackend{deployment: &apitype.UntypedDeployment{Version: 3, Deployment: deploymentJSON}}
			ref := &backend.MockStackReference{StringV: "owner/project/stack", NameV: tokens.MustParseStackName("stack")}
			stk := &backend.MockStack{
				BackendF: func() backend.Backend { return be },
				RefF:     func() backend.StackReference { return ref },
			}
			be.ParseStackReferenceF = func(string) (backend.StackReference, error) { return ref, nil }
			be.GetStackF = func(context.Context, backend.StackReference) (backend.Stack, error) { return stk, nil }
			be.ExportDeploymentF = func(context.Context, backend.Stack) (*apitype.UntypedDeployment, error) {
				return be.deployment, nil
			}
			lm := &cmdBackend.MockLoginManager{Clients: source, LoginF: func(
				context.Context, pkgWorkspace.Context, diag.Sink, string, *workspace.Project, bool, bool, colors.Colorization,
			) (backend.Backend, error) {
				return be, nil
			}}
			cmd := newStackExportCmd(&pkgWorkspace.MockContext{}, lm, cmdBackend.SecretsProvider(lm))
			var out bytes.Buffer
			cmd.SetOut(&out)
			args := []string{"--stack", "owner/project/stack", "--show-secrets"}
			if auth.version != "" {
				args = append(args, "--version", auth.version)
			}
			cmd.SetArgs(args)
			require.NoError(t, cmd.ExecuteContext(t.Context()))
			assert.Equal(t, auth.version, be.version)
			var exported apitype.UntypedDeployment
			require.NoError(t, json.Unmarshal(out.Bytes(), &exported))
			var deployment apitype.DeploymentV3
			require.NoError(t, json.Unmarshal(exported.Deployment, &deployment))
			assert.Equal(t, map[string]any{resource.SigKey: resource.SecretSig, "plaintext": `"secret-value"`},
				deployment.Resources[0].Outputs["secret"])
			account, err := workspace.GetAccount(server.URL)
			require.NoError(t, err)
			assert.Empty(t, account.AccessToken)
		})
	}
}
