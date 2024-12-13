// Copyright 2023-2024, Pulumi Corporation.
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

package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyPublishCmd_default(t *testing.T) {
	t.Parallel()

	mockPolicyPack := &backend.MockPolicyPack{
		PublishF: func(ctx context.Context, opts backend.PublishOperation) error {
			return nil
		},
	}

	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			ctx context.Context,
			ws pkgWorkspace.Context,
			sink diag.Sink,
			url string,
			project *workspace.Project,
			setCurrent bool,
			color colors.Colorization,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				GetPolicyPackF: func(ctx context.Context, name string, d diag.Sink) (backend.PolicyPack, error) {
					assert.Contains(t, name, "org1")
					return mockPolicyPack, nil
				},
			}, nil
		},
	}

	cmd := policyPublishCmd{
		getwd: func() (string, error) {
			cwd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			return filepath.Join(cwd, "testdata"), nil
		},
		defaultOrg: func(*workspace.Project) (string, error) {
			return "org1", nil
		},
	}

	err := cmd.Run(context.Background(), lm, []string{})
	require.NoError(t, err)
}

func TestPolicyPublishCmd_orgNamePassedIn(t *testing.T) {
	t.Parallel()

	mockPolicyPack := &backend.MockPolicyPack{
		PublishF: func(ctx context.Context, opts backend.PublishOperation) error {
			return nil
		},
	}

	lm := &cmdBackend.MockLoginManager{
		LoginF: func(
			ctx context.Context,
			ws pkgWorkspace.Context,
			sink diag.Sink,
			url string,
			project *workspace.Project,
			setCurrent bool,
			color colors.Colorization,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				GetPolicyPackF: func(ctx context.Context, name string, d diag.Sink) (backend.PolicyPack, error) {
					assert.Contains(t, name, "org1")
					return mockPolicyPack, nil
				},
			}, nil
		},
	}

	cmd := policyPublishCmd{
		getwd: func() (string, error) {
			cwd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			return filepath.Join(cwd, "testdata"), nil
		},
	}

	err := cmd.Run(context.Background(), lm, []string{"org1"})
	require.NoError(t, err)
}
