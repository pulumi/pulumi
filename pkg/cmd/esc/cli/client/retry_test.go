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

package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoWithRetryRetries429ForAllPolicies(t *testing.T) {
	t.Parallel()

	for _, policy := range []retryPolicy{retryNone, retryGetMethod, retryAllMethods} {
		t.Run(policy.String(), func(t *testing.T) {
			t.Parallel()

			tries := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tries++
				if tries == 1 {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			req, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("body"))
			require.NoError(t, err)

			res, err := doWithRetry(server.Client(), req, policy)
			require.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, 2, tries)
			assert.Equal(t, http.StatusOK, res.StatusCode)
		})
	}
}
