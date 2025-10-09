// Copyright 2023-2025, Pulumi Corporation.
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
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
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
		CurrentF: func(
			ctx context.Context,
			ws pkgWorkspace.Context,
			sink diag.Sink,
			url string,
			project *workspace.Project,
			setCurrent bool,
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
		defaultOrg: func(context.Context, backend.Backend, *workspace.Project) (string, error) {
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
		CurrentF: func(
			ctx context.Context,
			ws pkgWorkspace.Context,
			sink diag.Sink,
			url string,
			project *workspace.Project,
			setCurrent bool,
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

// TestPolicyPublishCmd_Metadata tests that vcs metadata is included with the publish command.
func TestPolicyPublishCmd_Metadata(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	subDir := filepath.Join(e.RootPath, "subdirectory")

	// Initialize a git repo with a commit in it.
	e.RunCommand("git", "init", "-b", "master")
	e.RunCommand("git", "config", "user.email", "repo-user@example.com")
	e.RunCommand("git", "config", "user.name", "repo-user")
	e.RunCommand("git", "remote", "add", "origin", "git@github.com:repo-owner-name/repo-repo-name")
	e.RunCommand("git", "checkout", "-b", "master")
	e.CWD = subDir
	e.WriteTestFile("PulumiPolicy.yml", "runtime: mock\nversion: 0.0.1\n")
	e.CWD = e.RootPath
	e.RunCommand("git", "add", ".")
	e.RunCommand("git", "commit", "-m", "repo-message")

	gitHead, _ := e.RunCommand("git", "rev-parse", "HEAD")

	var metadata map[string]string

	mockPolicyPack := &backend.MockPolicyPack{
		PublishF: func(ctx context.Context, opts backend.PublishOperation) error {
			metadata = opts.Metadata
			return nil
		},
	}

	lm := &cmdBackend.MockLoginManager{
		CurrentF: func(
			ctx context.Context,
			ws pkgWorkspace.Context,
			sink diag.Sink,
			url string,
			project *workspace.Project,
			setCurrent bool,
		) (backend.Backend, error) {
			return &backend.MockBackend{
				GetPolicyPackF: func(ctx context.Context, name string, d diag.Sink) (backend.PolicyPack, error) {
					return mockPolicyPack, nil
				},
			}, nil
		},
	}

	cmd := policyPublishCmd{
		getwd: func() (string, error) {
			// Return the sub directory to ensure we get the subdirectory metadata.
			return subDir, nil
		},
	}

	err := cmd.Run(context.Background(), lm, []string{})
	require.NoError(t, err)

	assertEnvValue := func(env map[string]string, key, val string) {
		t.Helper()
		got, ok := env[key]
		require.True(t, ok, "Didn't find expected metadata key %q (full env %+v)", key, env)
		assert.Equal(t, val, got, "got different value for metadata %v than expected", key)
	}

	assertEnvValue(metadata, backend.VCSRepoOwner, "repo-owner-name")
	assertEnvValue(metadata, backend.VCSRepoName, "repo-repo-name")
	assertEnvValue(metadata, backend.VCSRepoKind, "github.com")
	assertEnvValue(metadata, backend.VCSRepoRoot, "subdirectory")
	assertEnvValue(metadata, backend.GitHeadName, "refs/heads/master")
	assertEnvValue(metadata, backend.GitHead, strings.TrimSpace(gitHead))
	assertEnvValue(metadata, backend.GitCommitter, "repo-user")
	assertEnvValue(metadata, backend.GitCommitterEmail, "repo-user@example.com")
	assertEnvValue(metadata, backend.GitAuthor, "repo-user")
	assertEnvValue(metadata, backend.GitAuthorEmail, "repo-user@example.com")
}
