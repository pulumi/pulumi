// Copyright 2025, Pulumi Corporation.
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
	"context"
	"io"
	"testing"

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoginURLResolution tests the complete URL resolution logic in the login command,
// including command-line arguments, flags, environment variables, project settings,
// stored credentials, and OIDC token defaults.
func TestLoginURLResolution(t *testing.T) {
	// Helper to create a mock backend that captures the URL it was called with
	type capturedLogin struct {
		url       string
		oidcToken string
	}

	credsF := func() (workspace.Credentials, error) {
		return workspace.Credentials{Current: "https://stored-creds.example.com"}, nil
	}

	tests := []struct {
		name        string
		args        []string             // command-line arguments
		flags       map[string]string    // flag values to set
		ws          pkgWorkspace.Context // workspace mock
		envVars     map[string]string    // environment variables to set
		expectedURL string               // URL that should be passed to Login/LoginFromAuthContext
		expectError bool                 // whether an error is expected
	}{
		{
			name:        "command argument takes precedence",
			args:        []string{"s3://my-bucket"},
			ws:          &pkgWorkspace.MockContext{},
			expectedURL: "s3://my-bucket",
		},
		{
			name: "cloud-url flag takes precedence",
			flags: map[string]string{
				"cloud-url": "https://custom.example.com",
			},
			ws:          &pkgWorkspace.MockContext{},
			expectedURL: "https://custom.example.com",
		},
		{
			name: "local flag sets file:// backend",
			flags: map[string]string{
				"local": "true",
			},
			ws:          &pkgWorkspace.MockContext{},
			expectedURL: "file://~",
		},
		{
			name: "local flag conflicts with cloud-url",
			flags: map[string]string{
				"local":     "true",
				"cloud-url": "https://example.com",
			},
			ws:          &pkgWorkspace.MockContext{},
			expectError: true,
		},
		{
			name: "argument and cloud-url flag conflict",
			args: []string{"s3://bucket"},
			flags: map[string]string{
				"cloud-url": "https://example.com",
			},
			ws:          &pkgWorkspace.MockContext{},
			expectError: true,
		},
		{
			name: "environment variable used when no explicit URL",
			ws: &pkgWorkspace.MockContext{
				GetStoredCredentialsF: credsF,
			},
			envVars: map[string]string{
				"PULUMI_BACKEND_URL": "https://env-backend.example.com",
			},
			expectedURL: "https://env-backend.example.com",
		},
		{
			name: "project backend URL used when no env var",
			ws: &pkgWorkspace.MockContext{
				ReadProjectF: func() (*workspace.Project, string, error) {
					return &workspace.Project{
						Backend: &workspace.ProjectBackend{URL: "https://project-backend.example.com"},
					}, "", nil
				},
				GetStoredCredentialsF: credsF,
			},
			expectedURL: "https://project-backend.example.com",
		},
		{
			name: "stored credentials used as fallback",
			ws: &pkgWorkspace.MockContext{
				GetStoredCredentialsF: credsF,
			},
			expectedURL: "https://stored-creds.example.com",
		},
		{
			name: "OIDC token defaults to api.pulumi.com",
			flags: map[string]string{
				"oidc-token": "test-token",
				"oidc-org":   "test-org",
			},
			ws:          &pkgWorkspace.MockContext{},
			expectedURL: "https://api.pulumi.com",
		},
		{
			name: "OIDC token with explicit URL uses explicit URL",
			args: []string{"https://custom.example.com"},
			flags: map[string]string{
				"oidc-token": "test-token",
				"oidc-org":   "test-org",
			},
			ws:          &pkgWorkspace.MockContext{},
			expectedURL: "https://custom.example.com",
		},
		{
			name: "OIDC token respects env var over default",
			flags: map[string]string{
				"oidc-token": "test-token",
				"oidc-org":   "test-org",
			},
			ws: &pkgWorkspace.MockContext{},
			envVars: map[string]string{
				"PULUMI_BACKEND_URL": "https://env-backend.example.com",
			},
			expectedURL: "https://env-backend.example.com",
		},
		{
			name:        "empty URL without OIDC triggers interactive (captured as empty)",
			ws:          &pkgWorkspace.MockContext{},
			expectedURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cannot use t.Parallel() with t.Setenv()

			// Set environment variables for this test
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Track what URL was passed to the login manager
			var captured capturedLogin
			mockLoginManager := &backend.MockLoginManager{
				LoginF: func(
					ctx context.Context,
					ws pkgWorkspace.Context,
					sink diag.Sink,
					url string,
					project *workspace.Project,
					setCurrent bool,
					insecure bool,
					color colors.Colorization,
				) (pkgBackend.Backend, error) {
					captured.url = url
					return &pkgBackend.MockBackend{
						URLF:  func() string { return url },
						NameF: func() string { return "test-backend" },
						CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
							return "test-user", nil, nil, nil
						},
					}, nil
				},
				LoginFromAuthContextF: func(
					ctx context.Context,
					sink diag.Sink,
					url string,
					project *workspace.Project,
					setCurrent bool,
					insecure bool,
					authContext workspace.AuthContext,
				) (pkgBackend.Backend, error) {
					captured.url = url
					captured.oidcToken = authContext.Token
					return &pkgBackend.MockBackend{
						URLF:  func() string { return url },
						NameF: func() string { return "test-backend" },
						CurrentUserF: func() (string, []string, *workspace.TokenInformation, error) {
							return "test-user", nil, nil, nil
						},
					}, nil
				},
			}

			// Replace the default login manager temporarily
			// Create and execute command
			cmd := NewLoginCmd(tt.ws, mockLoginManager)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetContext(context.Background())

			// Set args first
			cmd.SetArgs(tt.args)

			// Parse flags to populate persistent flags
			// We need to call ParseFlags to process the flags before setting them manually
			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			// Set flags using PersistentFlags since that's where they're defined
			for name, value := range tt.flags {
				err := cmd.PersistentFlags().Set(name, value)
				require.NoError(t, err, "failed to set flag %s=%s", name, value)
			}

			// Execute
			err := cmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedURL, captured.url,
					"login should be called with expected URL")
			}
		})
	}
}
