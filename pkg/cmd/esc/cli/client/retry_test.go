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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoWithRetry429(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method    string
		policy    retryPolicy
		wantTries int
	}{
		{http.MethodGet, retryGetMethod, 2},
		{http.MethodPost, retryAllMethods, 2},
		{http.MethodGet, retryNone, 1},
		{http.MethodPost, retryGetMethod, 1},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s-%s", c.method, c.policy), func(t *testing.T) {
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

			req, err := http.NewRequest(c.method, server.URL, strings.NewReader("body"))
			require.NoError(t, err)

			res, err := doWithRetry(server.Client(), req, c.policy)
			require.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, c.wantTries, tries)
			if c.wantTries > 1 {
				assert.Equal(t, http.StatusOK, res.StatusCode)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, res.StatusCode)
			}
		})
	}
}
