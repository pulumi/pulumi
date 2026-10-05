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

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceSecretsUseResolvedCredentials(t *testing.T) {
	for _, token := range []string{"memory-only-token", ""} {
		t.Run(token, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wantToken := ""
				if token != "" {
					wantToken = "token " + token
				}
				assert.Equal(t, wantToken, r.Header.Get("Authorization"))

				switch r.URL.Path {
				case "/api/capabilities":
					fmt.Fprint(w, `{}`)
				case "/api/stacks/owner/project/stack/decrypt":
					fmt.Fprintf(w, `{"plaintext":%q}`, base64.StdEncoding.EncodeToString([]byte("decrypted")))
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			t.Setenv("PULUMI_ACCESS_TOKEN", "ambient")
			resolve := func(_ context.Context, url string) (*auth.Credentials, error) {
				assert.Equal(t, server.URL, url)
				return &auth.Credentials{
					BackendURL: url,
					HTTPAuth:   auth.NewHTTPAuth(url, token, nil),
				}, nil
			}
			ctx := t.Context()
			state, err := json.Marshal(serviceSecretsManagerState{
				URL: server.URL, Owner: "owner", Project: "project", Stack: "stack",
			})
			require.NoError(t, err)
			manager, err := NewServiceSecretsManagerFromState(ctx, resolve, state)
			require.NoError(t, err)
			plaintext, err := manager.Decrypter().DecryptValue(ctx, base64.StdEncoding.EncodeToString([]byte("ciphertext")))
			require.NoError(t, err)
			assert.Equal(t, "decrypted", plaintext)
		})
	}
}
