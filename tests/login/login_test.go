// Copyright 2021-2024, Pulumi Corporation.
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

package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogin(t *testing.T) {
	t.Parallel()

	t.Run("RespectsEnvVar", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)

		// Running pulumi logout --all twice shouldn't result in an error
		e.RunCommand("pulumi", "logout", "--all")
		e.RunCommand("pulumi", "logout", "--all")
	})
}

func TestInsecureLogin(t *testing.T) {
	t.Parallel()

	// Setup a mock server to act as a Pulumi Service backend.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reply to the whoami request with a mock user.
		if r.URL.Path == "/api/user" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"githubLogin":"mock-user","name":"Mock User","email":"mock-user@example.com"}`))
			require.NoError(t, err)
			return
		}
		if r.URL.Path == "/api/capabilities" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{}`))
			require.NoError(t, err)
			return
		}
		if r.URL.Path == "/api/user/organizations/default" {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"githubLogin":"mock-user"}`))
			require.NoError(t, err)
			return
		}
		require.Fail(t, "%v", r)
	}))
	t.Cleanup(server.Close)

	// Login to the mock server using an insecure connection.
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// First check that without "--insecure" we get tls err
	_, stderr := e.RunCommandExpectError("pulumi", "login", "--cloud-url", server.URL)
	assert.Contains(t, stderr, "x509: certificate signed by unknown authority")

	// Now login with "--insecure" and expect success.
	e.RunCommand("pulumi", "login", "--cloud-url", server.URL, "--insecure")

	// Then run another command to verify that the CLI remembers to stay in insecure mode.
	stdout, _ := e.RunCommand("pulumi", "whoami")
	assert.Contains(t, stdout, "mock-user")
}
