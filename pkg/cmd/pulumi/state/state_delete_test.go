// Copyright 2016-2024, Pulumi Corporation.
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

package state

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateDeleteNoArgs(t *testing.T) {
	t.Parallel()

	cmd := newStateDeleteCommand(&pkgWorkspace.MockContext{}, &cmdBackend.MockLoginManager{})
	cmd.SetArgs([]string{})
	err := cmd.ExecuteContext(context.Background())
	assert.ErrorContains(t, err, "Must supply <resource URN> unless pulumi is run interactively")
}

func TestStateDeleteTooManyArgs(t *testing.T) {
	t.Parallel()

	cmd := newStateDeleteCommand(&pkgWorkspace.MockContext{}, &cmdBackend.MockLoginManager{})
	cmd.SetArgs([]string{"urn", "extra"})
	err := cmd.ExecuteContext(context.Background())
	assert.ErrorContains(t, err, "accepts at most 1 arg(s), received 2")
}

func TestStateDeleteAllAndURN(t *testing.T) {
	t.Parallel()

	cmd := newStateDeleteCommand(&pkgWorkspace.MockContext{}, &cmdBackend.MockLoginManager{})
	cmd.SetArgs([]string{"--all", "urn"})
	err := cmd.ExecuteContext(context.Background())
	assert.ErrorContains(t, err, "cannot specify a resource URN when deleting all resources")
}

func TestNoProject(t *testing.T) {
	t.Parallel()

	mockBackend := &backend.MockBackend{}
	ws := &pkgWorkspace.MockContext{}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
			url string, project *workspace.Project, setCurrent bool, color colors.Colorization,
		) (backend.Backend, error) {
			assert.Equal(t, "", url)
			return mockBackend, nil
		},
	}

	cmd := newStateDeleteCommand(ws, lm)
	cmd.SetArgs([]string{"urn:pulumi:proj::stk::pkg:index:typ::res"})
	err := cmd.ExecuteContext(context.Background())
	assert.ErrorContains(t, err, "no Pulumi.yaml project file found")
}

func TestStateDeleteURN(t *testing.T) {
	t.Parallel()

	var mockStack *backend.MockStack

	var savedDeployment *apitype.UntypedDeployment
	mockBackend := &backend.MockBackend{
		GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
			assert.Equal(t, "stk", ref.String())
			return mockStack, nil
		},
		ImportDeploymentF: func(_ context.Context, _ backend.Stack, deployment *apitype.UntypedDeployment) error {
			savedDeployment = deployment
			return nil
		},
	}

	mockStack = &backend.MockStack{
		BackendF: func() backend.Backend {
			return mockBackend
		},
		SnapshotF: func(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
			return &deploy.Snapshot{
				Resources: []*resource.State{
					{
						URN: "urn:pulumi:proj::stk::pkg:index:typ::res",
					},
				},
			}, nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name: "proj",
			}, "/testing/project", nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			_ context.Context, _ pkgWorkspace.Context, _ diag.Sink,
			url string, project *workspace.Project, _ bool, _ colors.Colorization,
		) (backend.Backend, error) {
			assert.Equal(t, "", url)
			assert.Equal(t, tokens.PackageName("proj"), project.Name)
			return mockBackend, nil
		},
	}

	cmd := newStateDeleteCommand(ws, lm)
	cmd.SetArgs([]string{"--stack=stk", "urn:pulumi:proj::stk::pkg:index:typ::res"})
	err := cmd.ExecuteContext(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 3, savedDeployment.Version)
	assert.Equal(t,
		`{"manifest":{"time":"0001-01-01T00:00:00Z","magic":"","version":""},"metadata":{}}`,
		string(savedDeployment.Deployment))
}

func TestStateDeleteDependency(t *testing.T) {
	t.Parallel()

	mockStack := &backend.MockStack{
		SnapshotF: func(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
			return &deploy.Snapshot{
				Resources: []*resource.State{
					{
						URN: "urn:pulumi:proj::stk::pkg:index:typ::dependency",
					},
					{
						URN: "urn:pulumi:proj::stk::pkg:index:typ::dependee",
						Dependencies: []resource.URN{
							"urn:pulumi:proj::stk::pkg:index:typ::dependency",
						},
					},
				},
			}, nil
		},
	}
	mockBackend := &backend.MockBackend{
		GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
			assert.Equal(t, "stk", ref.String())
			return mockStack, nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name: "proj",
			}, "/testing/project", nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			_ context.Context, _ pkgWorkspace.Context, _ diag.Sink,
			url string, project *workspace.Project, _ bool, _ colors.Colorization,
		) (backend.Backend, error) {
			assert.Equal(t, "", url)
			assert.Equal(t, tokens.PackageName("proj"), project.Name)
			return mockBackend, nil
		},
	}

	cmd := newStateDeleteCommand(ws, lm)
	cmd.SetArgs([]string{"--stack=stk", "urn:pulumi:proj::stk::pkg:index:typ::dependency"})
	err := cmd.ExecuteContext(context.Background())
	assert.ErrorContains(t, err,
		"urn:pulumi:proj::stk::pkg:index:typ::dependency can't be safely deleted "+
			"because the following resources depend on it:\n"+
			" * \"dependee\"      (urn:pulumi:proj::stk::pkg:index:typ::dependee)")
}

func TestStateDeleteProtected(t *testing.T) {
	t.Parallel()

	var mockStack *backend.MockStack

	var savedDeployment *apitype.UntypedDeployment
	mockBackend := &backend.MockBackend{
		GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
			assert.Equal(t, "stk", ref.String())
			return mockStack, nil
		},
		ImportDeploymentF: func(_ context.Context, _ backend.Stack, deployment *apitype.UntypedDeployment) error {
			savedDeployment = deployment
			return nil
		},
	}

	mockStack = &backend.MockStack{
		BackendF: func() backend.Backend {
			return mockBackend
		},
		SnapshotF: func(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
			return &deploy.Snapshot{
				Resources: []*resource.State{
					{
						URN:     "urn:pulumi:proj::stk::pkg:index:typ::res",
						Protect: true,
					},
				},
			}, nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name: "proj",
			}, "/testing/project", nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			_ context.Context, _ pkgWorkspace.Context, _ diag.Sink,
			url string, project *workspace.Project, _ bool, _ colors.Colorization,
		) (backend.Backend, error) {
			assert.Equal(t, "", url)
			assert.Equal(t, tokens.PackageName("proj"), project.Name)
			return mockBackend, nil
		},
	}

	cmd := newStateDeleteCommand(ws, lm)
	cmd.SetArgs([]string{"--stack=stk", "urn:pulumi:proj::stk::pkg:index:typ::res"})
	err := cmd.ExecuteContext(context.Background())
	assert.ErrorContains(t, err,
		"urn:pulumi:proj::stk::pkg:index:typ::res can't be safely deleted because it is protected.")
	assert.Nil(t, savedDeployment)

	cmd.SetArgs([]string{"--force", "--stack=stk", "urn:pulumi:proj::stk::pkg:index:typ::res"})
	err = cmd.ExecuteContext(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 3, savedDeployment.Version)
	assert.Equal(t,
		"{\"manifest\":{\"time\":\"0001-01-01T00:00:00Z\",\"magic\":\"\",\"version\":\"\"},\"metadata\":{}}",
		string(savedDeployment.Deployment))
}

func TestStateDeleteAll(t *testing.T) {
	t.Parallel()

	snapshot := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				URN: "urn:pulumi:proj::stk::pkg:index:typ::dependency",
			},
			{
				URN: "urn:pulumi:proj::stk::pkg:index:typ::dependee",
				Dependencies: []resource.URN{
					"urn:pulumi:proj::stk::pkg:index:typ::dependency",
				},
			},
		},
	}

	var mockStack *backend.MockStack

	var mockDeployment *apitype.UntypedDeployment
	mockBackend := &backend.MockBackend{
		GetStackF: func(_ context.Context, ref backend.StackReference) (backend.Stack, error) {
			assert.Equal(t, "stk", ref.String())
			return mockStack, nil
		},
		ImportDeploymentF: func(_ context.Context, _ backend.Stack, deployment *apitype.UntypedDeployment) error {
			mockDeployment = deployment
			return nil
		},
	}

	mockStack = &backend.MockStack{
		BackendF: func() backend.Backend {
			return mockBackend
		},
		SnapshotF: func(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
			return snapshot, nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name: "proj",
			}, "/testing/project", nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			_ context.Context, _ pkgWorkspace.Context, _ diag.Sink,
			url string, project *workspace.Project, _ bool, _ colors.Colorization,
		) (backend.Backend, error) {
			assert.Equal(t, "", url)
			assert.Equal(t, tokens.PackageName("proj"), project.Name)
			return mockBackend, nil
		},
	}

	cmd := newStateDeleteCommand(ws, lm)
	cmd.SetArgs([]string{"--stack=stk", "--all"})
	err := cmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	deployment := apitype.DeploymentV3{}
	err = json.Unmarshal(mockDeployment.Deployment, &deployment)
	require.NoError(t, err)
	assert.Len(t, deployment.Resources, 0)
}
