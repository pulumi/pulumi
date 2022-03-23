// Copyright 2016-2021, Pulumi Corporation.
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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newMockServer(statusCode int, message string) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(statusCode)
			_, err := rw.Write([]byte(message))
			if err != nil {
				return
			}
		}))
}

func newMockClient(server *httptest.Server) *Client {
	return &Client{
		apiURL:   server.URL,
		apiToken: "",
		apiUser:  "",
		diag:     nil,
	}
}

func TestAPIErrorResponses(t *testing.T) {
	t.Parallel()

	t.Run("TestAuthError", func(t *testing.T) {
		t.Parallel()

		// check 401 error is handled
		unauthorizedServer := newMockServer(401, "401: Unauthorized")
		defer unauthorizedServer.Close()

		unauthorizedClient := newMockClient(unauthorizedServer)
		_, _, unauthorizedErr := unauthorizedClient.GetCLIVersionInfo(context.Background())

		assert.Error(t, unauthorizedErr)
		assert.Equal(t, unauthorizedErr.Error(), "this command requires logging in; try running 'pulumi login' first")
	})
	t.Run("TestRateLimitError", func(t *testing.T) {
		t.Parallel()

		// test handling 429: Too Many Requests/rate-limit response
		rateLimitedServer := newMockServer(429, "rate-limit error")
		defer rateLimitedServer.Close()

		rateLimitedClient := newMockClient(rateLimitedServer)
		_, _, rateLimitErr := rateLimitedClient.GetCLIVersionInfo(context.Background())

		assert.Error(t, rateLimitErr)
		assert.Equal(t, rateLimitErr.Error(), "pulumi service: request rate-limit exceeded")
	})
	t.Run("TestDefaultError", func(t *testing.T) {
		t.Parallel()

		// test handling non-standard error message
		defaultErrorServer := newMockServer(418, "I'm a teapot")
		defer defaultErrorServer.Close()

		defaultErrorClient := newMockClient(defaultErrorServer)
		_, _, defaultErrorErr := defaultErrorClient.GetCLIVersionInfo(context.Background())

		assert.Error(t, defaultErrorErr)
	})
}
