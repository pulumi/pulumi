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

	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
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
