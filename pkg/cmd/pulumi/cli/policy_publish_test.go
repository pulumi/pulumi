package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyPublishCmd_default(t *testing.T) {
	t.Parallel()

	mockPolicyPack := &backend.MockPolicyPack{
		PublishF: func(ctx context.Context, opts backend.PublishOperation) result.Result {
			return nil
		},
	}

	cmd := policyPublishCmd{
		getwd: func() (string, error) {
			cwd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			return filepath.Join(cwd, "testdata/policy"), nil
		},
		loginToCloud: func(context.Context, string, *workspace.Project, bool, display.Options) (backend.Backend, error) {
			return &backend.MockBackend{
				GetPolicyPackF: func(ctx context.Context, name string, d diag.Sink) (backend.PolicyPack, error) {
					assert.Contains(t, name, "org1")
					return mockPolicyPack, nil
				},
			}, nil
		},
		defaultOrg: func(*workspace.Project) (string, error) {
			return "org1", nil
		},
	}

	err := cmd.Run(context.Background(), []string{})
	require.NoError(t, err)
}

func TestPolicyPublishCmd_orgNamePassedIn(t *testing.T) {
	t.Parallel()

	mockPolicyPack := &backend.MockPolicyPack{
		PublishF: func(ctx context.Context, opts backend.PublishOperation) result.Result {
			return nil
		},
	}

	cmd := policyPublishCmd{
		getwd: func() (string, error) {
			cwd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			return filepath.Join(cwd, "testdata/policy"), nil
		},
		loginToCloud: func(context.Context, string, *workspace.Project, bool, display.Options) (backend.Backend, error) {
			return &backend.MockBackend{
				GetPolicyPackF: func(ctx context.Context, name string, d diag.Sink) (backend.PolicyPack, error) {
					assert.Contains(t, name, "org1")
					return mockPolicyPack, nil
				},
			}, nil
		},
	}

	err := cmd.Run(context.Background(), []string{"org1"})
	require.NoError(t, err)
}
