// Copyright 2023, Pulumi Corporation.
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
	"encoding/json"
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
		PublishF: func(ctx context.Context, opts backend.PublishOperation) (backend.PublishResult, error) {
			return backend.PublishResult{}, nil
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
				GetDefaultOrgF: func(ctx context.Context) (string, error) {
					return "org1", nil
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

	err := cmd.Run(t.Context(), lm, []string{})
	require.NoError(t, err)
}

func TestPolicyPublishCmd_orgNamePassedIn(t *testing.T) {
	t.Parallel()

	mockPolicyPack := &backend.MockPolicyPack{
		PublishF: func(ctx context.Context, opts backend.PublishOperation) (backend.PublishResult, error) {
			return backend.PublishResult{}, nil
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

	err := cmd.Run(t.Context(), lm, []string{"org1"})
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
		PublishF: func(ctx context.Context, opts backend.PublishOperation) (backend.PublishResult, error) {
			metadata = opts.Metadata
			return backend.PublishResult{}, nil
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

	err := cmd.Run(t.Context(), lm, []string{})
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

func publishTestLoginManager(t *testing.T, pack backend.PolicyPack) *cmdBackend.MockLoginManager {
	t.Helper()
	return &cmdBackend.MockLoginManager{
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
					return pack, nil
				},
				GetDefaultOrgF: func(ctx context.Context) (string, error) {
					return "org1", nil
				},
			}, nil
		},
	}
}

func testdataGetwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, "testdata"), nil
}

func TestPolicyPublish_JSONOutputIsPureJSON(t *testing.T) {
	t.Parallel()

	pack := &backend.MockPolicyPack{
		PublishF: func(ctx context.Context, op backend.PublishOperation) (backend.PublishResult, error) {
			_, err := op.Progress.Write([]byte("Obtaining policy metadata from policy plugin\n"))
			require.NoError(t, err)
			return backend.PublishResult{
				Name:       "pack",
				VersionTag: "1.2.0",
				URL:        "https://app.pulumi.com/org1/insights/policypacks/pack/1.2.0",
			}, nil
		},
	}

	var stdout, stderr bytes.Buffer
	cmd := policyPublishCmd{getwd: testdataGetwd, stdout: &stdout, stderr: &stderr, json: true}
	require.NoError(t, cmd.Run(t.Context(), publishTestLoginManager(t, pack), []string{}))

	var got policyPublishJSON
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	assert.Equal(t, policyPublishJSON{
		Organization: "org1",
		Name:         "pack",
		VersionTag:   "1.2.0",
		URL:          "https://app.pulumi.com/org1/insights/policypacks/pack/1.2.0",
	}, got)
	assert.Contains(t, stderr.String(), "Obtaining policy metadata from policy plugin")
}

func TestPolicyPublish_DefaultProgressOnStdout(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	pack := &backend.MockPolicyPack{
		PublishF: func(ctx context.Context, op backend.PublishOperation) (backend.PublishResult, error) {
			assert.Same(t, &stdout, op.Progress)
			return backend.PublishResult{}, nil
		},
	}

	cmd := policyPublishCmd{getwd: testdataGetwd, stdout: &stdout, stderr: &stderr}
	require.NoError(t, cmd.Run(t.Context(), publishTestLoginManager(t, pack), []string{"org1"}))
	assert.Empty(t, stdout.String())
}
