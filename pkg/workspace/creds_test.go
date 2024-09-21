// Copyright 2024, Pulumi Corporation.
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

package workspace

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func TestGetCurrentCloudURL(t *testing.T) {
	t.Parallel()

	credsF := func() (workspace.Credentials, error) {
		return workspace.Credentials{Current: "https://credentials.com"}, nil
	}

	tests := []struct {
		name           string
		ws             Context
		e              env.Env
		project        *workspace.Project
		expectedString string
		expectedError  error
	}{
		{
			name:           "no project, env, or credentials",
			ws:             &MockContext{},
			e:              env.NewEnv(env.MapStore{}),
			expectedString: "",
		},
		{
			name: "stored credentials",
			ws: &MockContext{
				GetStoredCredentialsF: credsF,
			},
			e:              env.NewEnv(env.MapStore{}),
			expectedString: "https://credentials.com",
		},
		{
			name: "project setting takes precedence",
			ws: &MockContext{
				GetStoredCredentialsF: credsF,
			},
			e:              env.NewEnv(env.MapStore{}),
			project:        &workspace.Project{Backend: &workspace.ProjectBackend{URL: "https://project.com"}},
			expectedString: "https://project.com",
		},
		{
			name: "envvar takes precedence",
			ws: &MockContext{
				GetStoredCredentialsF: credsF,
			},
			e: env.NewEnv(env.MapStore{
				env.BackendURL.Var().Name(): "https://env.com",
			}),
			project:        &workspace.Project{Backend: &workspace.ProjectBackend{URL: "https://project.com"}},
			expectedString: "https://env.com",
		},
		{
			name: "report error from stored credentials",
			ws: &MockContext{
				GetStoredCredentialsF: func() (workspace.Credentials, error) {
					return workspace.Credentials{}, assert.AnError
				},
			},
			e:             env.NewEnv(env.MapStore{}),
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			str, err := GetCurrentCloudURL(tt.ws, tt.e, tt.project)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedString, str)
		})
	}
}
