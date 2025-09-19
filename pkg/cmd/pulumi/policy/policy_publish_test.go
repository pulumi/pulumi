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
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestPolicyPublishCmd_Metadata tests that vcs metadata is included with the publish command.
func TestPolicyPublishCmd_Metadata(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdirectory")
	err := os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)

	runCommand := func(dir, name string, args ...string) (string, string) {
		var outBuffer, errBuffer bytes.Buffer

		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Stdout = &outBuffer
		cmd.Stderr = &errBuffer

		err := cmd.Run()
		require.NoError(t, err)

		return outBuffer.String(), errBuffer.String()
	}

	// Initialize a git repo with a commit in it.
	runCommand(tmpDir, "git", "init", "-b", "master")
	runCommand(tmpDir, "git", "config", "user.email", "repo-user@example.com")
	runCommand(tmpDir, "git", "config", "user.name", "repo-user")
	runCommand(tmpDir, "git", "remote", "add", "origin", "git@github.com:repo-owner-name/repo-repo-name")
	runCommand(tmpDir, "git", "checkout", "-b", "master")
	err = os.WriteFile(filepath.Join(subDir, "PulumiPolicy.yml"), []byte("runtime: mock\nversion: 0.0.1\n"), 0o600)
	require.NoError(t, err)
	runCommand(tmpDir, "git", "add", ".")
	runCommand(tmpDir, "git", "commit", "-m", "repo-message")

	gitHead, _ := runCommand(tmpDir, "git", "rev-parse", "HEAD")

	var metadata map[string]string

	mockPolicyPack := &backend.MockPolicyPack{
		PublishF: func(ctx context.Context, opts backend.PublishOperation) error {
			metadata = opts.Metadata
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
					return mockPolicyPack, nil
				},
			}, nil
		},
	}

	cmd := policyPublishCmd{
		getwd: func() (string, error) {
			return subDir, nil
		},
	}

	err = cmd.Run(context.Background(), lm, []string{})
	require.NoError(t, err)

	assertEnvValue := func(env map[string]string, key, val string) {
		t.Helper()
		got, ok := env[key]
		if !ok {
			t.Errorf("Didn't find expected metadata key %q (full env %+v)", key, env)
		} else {
			assert.EqualValues(t, val, got, "got different value for metadata %v than expected", key)
		}
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
