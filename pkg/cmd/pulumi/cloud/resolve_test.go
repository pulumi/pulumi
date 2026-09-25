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

package cloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// The credential probe must target the same cloud URL CurrentBackend will
// build, so it validates the token for the backend we actually use. The
// regression this guards: PULUMI_BACKEND_URL is honoured by the backend build
// (via GetCurrentCloudURL) but was ignored by the probe outside a project
// directory, so the probe validated a different backend's token.
func TestResolveCloudURL(t *testing.T) {
	// Sets PULUMI_BACKEND_URL via t.Setenv, so cannot run in parallel.

	const backendURL = "https://backend.example.com"
	const credsURL = "https://app.example.com"
	const projectBackendURL = "https://project.example.com"

	storedCreds := func() (workspace.Credentials, error) {
		return workspace.Credentials{Current: credsURL}, nil
	}
	projectWithBackend := &workspace.Project{
		Backend: &workspace.ProjectBackend{URL: projectBackendURL},
	}

	tests := []struct {
		name        string
		backendEnv  string // PULUMI_BACKEND_URL value; "" means unset
		project     *workspace.Project
		expectedURL string
	}{
		{
			// The bug: outside a project directory the probe ignored
			// PULUMI_BACKEND_URL and fell back to stored credentials.
			name:        "PULUMI_BACKEND_URL honoured without a project",
			backendEnv:  backendURL,
			project:     nil,
			expectedURL: backendURL,
		},
		{
			name:        "PULUMI_BACKEND_URL honoured with a project",
			backendEnv:  backendURL,
			project:     projectWithBackend,
			expectedURL: backendURL,
		},
		{
			name:        "stored credentials without a project",
			project:     nil,
			expectedURL: credsURL,
		},
		{
			name:        "project backend without PULUMI_BACKEND_URL",
			project:     projectWithBackend,
			expectedURL: projectBackendURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set unconditionally (empty for the unset cases) so a
			// PULUMI_BACKEND_URL in the developer's environment can't leak in.
			t.Setenv(env.BackendURL.Var().Name(), tt.backendEnv)

			ws := &pkgWorkspace.MockContext{GetStoredCredentialsF: storedCreds}
			got := resolveCloudURL(ws, tt.project)
			assert.Equal(t, tt.expectedURL, got)

			// The probe must resolve the same URL CurrentBackend resolves.
			build, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), tt.project)
			require.NoError(t, err)
			assert.Equal(t, build, got,
				"probe URL must match the URL CurrentBackend resolves")
		})
	}
}
