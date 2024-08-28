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
