// Copyright 2016-2025, Pulumi Corporation.
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

package templates

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/util/testutil"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // replaces global login manager / changes cwd
func TestGetBackend(t *testing.T) {
	ctx := testContext(t)

	t.Run("success without project (env URL)", func(t *testing.T) {
		start := t.TempDir()
		t.Chdir(start)

		source := newImpl(ctx, "",
			ScopeAll, workspace.TemplateKindPulumiProject,
			templateRepository(workspace.TemplateRepository{}, nil),
			env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}),
		)

		mockB := &backend.MockBackend{
			SupportsTemplatesF: func() bool { return false },
			NameF:              func() string { return "cloud" },
		}
		testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
			CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent bool,
			) (backend.Backend, error) {
				require.NotEmpty(t, url)
				require.Nil(t, project)
				return mockB, nil
			},
		})

		got := source.getBackend(ctx, env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}))
		require.NotNil(t, got)
		assert.Equal(t, mockB, got)
	})

	t.Run("read project error (not ErrProjectNotFound) -> nil", func(t *testing.T) {
		start := t.TempDir()
		t.Chdir(start)

		// Write a malformed Pulumi.yaml to force a parse error
		require.NoError(t, os.WriteFile("Pulumi.yaml", []byte("not: [valid"), 0o600))

		source := newImpl(ctx, "",
			ScopeAll, workspace.TemplateKindPulumiProject,
			templateRepository(workspace.TemplateRepository{}, nil),
			env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}),
		)

		// Should not reach LoginManager if ReadProject fails
		testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{})

		got := source.getBackend(ctx, env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}))
		assert.Nil(t, got)
	})

	t.Run("non-interactive missing env -> nil (no addError)", func(t *testing.T) {
		start := t.TempDir()
		t.Chdir(start)

		source := newImpl(ctx, "",
			ScopeAll, workspace.TemplateKindPulumiProject,
			templateRepository(workspace.TemplateRepository{}, nil),
			env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}),
		)

		testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
			CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent bool,
			) (backend.Backend, error) {
				return nil, backenderr.MissingEnvVarForNonInteractiveError{}
			},
		})

		got := source.getBackend(ctx, env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}))
		assert.Nil(t, got)
	})

	t.Run("other login error -> nil (adds error)", func(t *testing.T) {
		start := t.TempDir()
		t.Chdir(start)

		source := newImpl(ctx, "",
			ScopeAll, workspace.TemplateKindPulumiProject,
			templateRepository(workspace.TemplateRepository{}, nil),
			env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}),
		)

		testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
			CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent bool,
			) (backend.Backend, error) {
				return nil, errors.New("boom")
			},
		})

		got := source.getBackend(ctx, env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}))
		assert.Nil(t, got)
	})

	t.Run("no logged-in user (nil, nil) -> nil", func(t *testing.T) {
		start := t.TempDir()
		t.Chdir(start)

		source := newImpl(ctx, "",
			ScopeAll, workspace.TemplateKindPulumiProject,
			templateRepository(workspace.TemplateRepository{}, nil),
			env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}),
		)

		testutil.MockLoginManager(t, &cmdBackend.MockLoginManager{
			CurrentF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink,
				url string, project *workspace.Project, setCurrent bool,
			) (backend.Backend, error) {
				return nil, nil
			},
		})

		got := source.getBackend(ctx, env.NewEnv(env.MapStore{"PULUMI_CLOUD_URL": "https://app.pulumi.com"}))
		assert.Nil(t, got)
	})
}

//nolint:paralleltest
func TestSupportsCloudTemplates(t *testing.T) {
	ctx := testContext(t)
	source := newImpl(ctx, "",
		ScopeAll, workspace.TemplateKindPulumiProject,
		templateRepository(workspace.TemplateRepository{}, nil),
		env.NewEnv(env.MapStore{}),
	)

	t.Run("nil backend", func(t *testing.T) {
		assert.False(t, source.supportsCloudTemplates(ctx, nil))
	})

	t.Run("backend does not support templates", func(t *testing.T) {
		mockB := &backend.MockBackend{
			SupportsTemplatesF: func() bool { return false },
			NameF:              func() string { return "mock" },
		}
		assert.False(t, source.supportsCloudTemplates(ctx, mockB))
	})

	t.Run("backend supports templates", func(t *testing.T) {
		mockB := &backend.MockBackend{
			SupportsTemplatesF: func() bool { return true },
		}
		assert.True(t, source.supportsCloudTemplates(ctx, mockB))
	})
}
