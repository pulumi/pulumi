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

package credentialhelper_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshFailureSurvivesHTTPRetries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		reason          string
		initialResponse string
		backendRequests int32
	}{
		{
			reason:          "expired",
			initialResponse: `{"version":1,"accessToken":"stale","expiresAt":"2000-01-01T00:00:00Z"}`,
			backendRequests: 0,
		},
		{
			reason:          "rejected",
			initialResponse: `{"version":1,"accessToken":"stale"}`,
			backendRequests: 1,
		},
	} {
		t.Run(test.reason, func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assert.Equal(t, "token stale", r.Header.Get("Authorization"))
				if test.reason == "rejected" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				// Accept requests in the expiry case to expose any fallback to stale credentials.
				_, _ = fmt.Fprint(w, `{"githubLogin":"alice"}`)
			}))
			defer server.Close()
			responses, err := json.Marshal(map[string]string{
				"initial": test.initialResponse,
				// Both refresh responses are invalid: the helper supplied an initial token,
				// so refresh must supply a token too.
				"expired":  `{"version":1}`,
				"rejected": `{"version":1}`,
			})
			require.NoError(t, err)
			executable, err := os.Executable()
			require.NoError(t, err)
			log := filepath.Join(t.TempDir(), "requests.jsonl")
			session := credentialhelper.NewSession(io.Discard)
			require.NoError(t, session.Configure(executable,
				[]string{"--", "-credential-helper-test", log, string(responses)}))
			credentials, err := session.Resolve(t.Context(), server.URL)
			require.NoError(t, err)
			c := client.NewClientWithCredentials(credentials, false, nil)
			_, _, _, err = c.GetPulumiAccountDetails(t.Context())
			require.ErrorContains(t, err, "credentials helper refresh requires HTTP credentials")
			assert.Equal(t, test.backendRequests, requests.Load(), "no additional backend requests after token refresh fails")
			calls, err := os.ReadFile(log)
			require.NoError(t, err)
			assert.Equal(t, 2, strings.Count(string(calls), "\n"), "one initial helper call and one failed refresh")
		})
	}
}
