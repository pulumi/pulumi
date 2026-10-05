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

package backend

import (
	"context"
	"testing"

	pkgauth "github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentBackendReturnsCloudURLError(t *testing.T) {
	ptesting.IsolateCredentials(t)
	ws := &pkgWorkspace.MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	backend, err := CurrentBackend(t.Context(), ws, &MockLoginManager{}, nil, display.Options{})
	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, backend)
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestNonInteractiveCurrentBackendReturnsCloudURLError(t *testing.T) {
	ptesting.IsolateCredentials(t)
	ws := &pkgWorkspace.MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	}

	backend, err := NonInteractiveCurrentBackend(t.Context(), ws, &MockLoginManager{}, nil)
	require.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, backend)
}

func TestNonInteractiveCurrentBackendPassesDefaultURL(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv(env.BackendURL.Var().Name(), "https://api.noninteractive.example.com")

	var gotURL string
	_, err := NonInteractiveCurrentBackend(t.Context(), &pkgWorkspace.MockContext{}, &MockLoginManager{
		CurrentF: func(
			ctx context.Context,
			ws pkgWorkspace.Context,
			sink diag.Sink,
			url string,
			project *workspace.Project,
			setCurrent bool,
		) (pkgBackend.Backend, error) {
			gotURL = url
			return nil, nil
		},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "https://api.noninteractive.example.com", gotURL)
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentBackendResolvesSelectionOnce(t *testing.T) {
	const helperURL = "https://helper.example.com"
	for _, mode := range []string{"interactive", "noninteractive"} {
		t.Run(mode, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			reads, helperCalls := 0, 0
			ws := &pkgWorkspace.MockContext{
				GetStoredCredentialsF: func() (workspace.Credentials, error) {
					reads++
					return workspace.Credentials{
						Accounts: map[string]workspace.Account{helperURL: {Insecure: true}},
					}, nil
				},
			}
			session := pkgauth.NewSessionWithHelperFunc(func(
				_ context.Context, request credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				helperCalls++
				assert.Equal(t, 1, reads, "resolve configuration once before invoking the helper")
				assert.Empty(t, request.SelectedBackendURL)
				return &credentialhelper.Response{BackendURL: helperURL}, nil
			})
			wantBackend := &pkgBackend.MockBackend{}
			lm := &MockLoginManager{
				HelperSession: session,
				CurrentF: func(
					_ context.Context, _ pkgWorkspace.Context, _ diag.Sink,
					url string, _ *workspace.Project, setCurrent bool,
				) (pkgBackend.Backend, error) {
					assert.Equal(t, helperURL, url)
					assert.True(t, setCurrent)
					assert.Equal(t, 1, reads)
					return wantBackend, nil
				},
				LoginF: func(
					_ context.Context, _ pkgWorkspace.Context, _ diag.Sink, url string, _ *workspace.Project,
					setCurrent, insecure bool, _ colors.Colorization,
				) (pkgBackend.Backend, error) {
					assert.Equal(t, helperURL, url)
					assert.True(t, setCurrent)
					assert.True(t, insecure, "look up TLS settings for the prepared URL")
					assert.Equal(t, 2, reads, "read configuration, then TLS settings")
					return wantBackend, nil
				},
			}
			var got pkgBackend.Backend
			var err error
			if mode == "interactive" {
				got, err = CurrentBackend(t.Context(), ws, lm, nil, display.Options{})
			} else {
				got, err = NonInteractiveCurrentBackend(t.Context(), ws, lm, nil)
			}
			require.NoError(t, err)
			assert.Same(t, wantBackend, got)
			assert.Equal(t, 1, helperCalls)
		})
	}
}
