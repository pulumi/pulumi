// Copyright 2016-2022, Pulumi Corporation.
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

package auto

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optremotepreview"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

const (
	remoteTestRepo       = "https://github.com/pulumi/test-repo.git"
	remoteTestRepoBranch = "refs/heads/master"
)

func testRemoteStackGitSourceErrors(t *testing.T, fn func(ctx context.Context, stackName string, repo GitRepo,
	opts ...RemoteWorkspaceOption) (RemoteStack, error),
) {
	ctx := context.Background()

	const stack = "owner/project/stack"

	tests := map[string]struct {
		stack           string
		repo            GitRepo
		executorImage   *ExecutorImage
		err             string
		inheritSettings bool
	}{
		"stack empty": {
			stack: "",
			err:   `stack name "" must be fully qualified`,
		},
		"stack just name": {
			stack: "name",
			err:   `stack name "name" must be fully qualified`,
		},
		"stack just name & owner": {
			stack: "owner/name",
			err:   `stack name "owner/name" must be fully qualified`,
		},
		"stack just sep": {
			stack: "/",
			err:   `stack name "/" must be fully qualified`,
		},
		"stack just two seps": {
			stack: "//",
			err:   `stack name "//" must be fully qualified`,
		},
		"stack just three seps": {
			stack: "///",
			err:   `stack name "///" must be fully qualified`,
		},
		"stack invalid": {
			stack: "owner/project/stack/wat",
			err:   `stack name "owner/project/stack/wat" must be fully qualified`,
		},
		"repo setup": {
			stack:           stack,
			repo:            GitRepo{Setup: func(context.Context, Workspace) error { return nil }},
			inheritSettings: true,
			err:             "repo.Setup cannot be used with remote workspaces",
		},
		"no url": {
			stack: stack,
			repo:  GitRepo{},
			err:   "repo.URL is required if RemoteInheritSettings(true) is not set",
		},
		"no branch or commit": {
			stack: stack,
			repo:  GitRepo{URL: remoteTestRepo},
			err:   "either repo.Branch or repo.CommitHash is required if RemoteInheritSettings(true) is not set",
		},
		"both branch and commit": {
			stack: stack,
			repo:  GitRepo{URL: remoteTestRepo, Branch: "branch", CommitHash: "commit"},
			err:   "repo.Branch and repo.CommitHash cannot both be specified",
		},
		"both ssh private key and path": {
			stack: stack,
			repo: GitRepo{
				URL:    remoteTestRepo,
				Branch: "branch",
				Auth:   &GitAuth{SSHPrivateKey: "key", SSHPrivateKeyPath: "path"},
			},
			err: "repo.Auth.SSHPrivateKey and repo.Auth.SSHPrivateKeyPath cannot both be specified",
		},
		"executor creds with no image": {
			stack: stack,
			repo: GitRepo{
				URL:    remoteTestRepo,
				Branch: "branch",
			},
			executorImage: &ExecutorImage{
				Credentials: &DockerImageCredentials{
					Username: "user",
					Password: "password",
				},
			},
			err: "executorImage.Image cannot be empty",
		},
		"executor image with username and no password": {
			stack: stack,
			repo: GitRepo{
				URL:    remoteTestRepo,
				Branch: "branch",
			},
			executorImage: &ExecutorImage{
				Image: "image",
				Credentials: &DockerImageCredentials{
					Username: "username",
				},
			},
			err: "executorImage.Credentials.Password cannot be empty",
		},
		"executor image with password and no username": {
			stack: stack,
			repo: GitRepo{
				URL:    remoteTestRepo,
				Branch: "branch",
			},
			executorImage: &ExecutorImage{
				Image: "image",
				Credentials: &DockerImageCredentials{
					Password: "password",
				},
			},
			err: "executorImage.Credentials.Username cannot be empty",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := fn(ctx, tc.stack, tc.repo, RemoteExecutorImage(tc.executorImage),
				RemoteInheritSettings(tc.inheritSettings))
			assert.EqualError(t, err, tc.err)
		})
	}
}

// fetchCommitHash runs `git ls-remote URL branch` to determine the latest commit for the given repo
// URL and branch.
func fetchCommitHash(url, branch string) (string, error) {
	cmd := exec.Command("git", "ls-remote", url, branch)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git ls-remote: %w", err)
	}
	out := strings.TrimSpace(string(output))
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", fmt.Errorf("could not determine commit hash from %q", out)
	}
	return fields[0], nil
}

func testRemoteStackGitSource(
	t *testing.T,
	fn func(ctx context.Context, stackName string, repo GitRepo, opts ...RemoteWorkspaceOption) (RemoteStack, error),
	useCommitHash bool,
	useExecutorImage bool,
) {
	// This test requires the service with access to Pulumi Deployments.
	// Set PULUMI_ACCESS_TOKEN to an access token with access to Pulumi Deployments
	// and set PULUMI_TEST_DEPLOYMENTS_API to any value to enable the test.
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}
	if os.Getenv("PULUMI_TEST_DEPLOYMENTS_API") == "" {
		t.Skipf("Skipping: PULUMI_TEST_DEPLOYMENTS_API is not set")
	}

	ctx := context.Background()
	pName := "go_remote_proj"
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)
	repo := GitRepo{
		URL:         remoteTestRepo,
		ProjectPath: "goproj",
	}
	var executorImage *ExecutorImage
	if useCommitHash {
		commitHash, err := fetchCommitHash(remoteTestRepo, remoteTestRepoBranch)
		require.NoError(t, err)
		repo.CommitHash = commitHash
	} else {
		repo.Branch = remoteTestRepoBranch
	}

	if useExecutorImage {
		executorImage = &ExecutorImage{
			Image: "pulumi/pulumi",
		}
	}

	// initialize
	s, err := fn(ctx, stackName, repo,
		RemotePreRunCommands(
			"pulumi config set bar abc --stack "+stackName,
			"pulumi config set --secret buzz secret --stack "+stackName),
		RemoteSkipInstallDependencies(true),
		RemoteExecutorImage(executorImage),
	)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.stack.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	// -- pulumi up --
	res, err := s.Up(ctx)
	if err != nil {
		t.Errorf("up failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, 3, len(res.Outputs), "expected two plain outputs")
	assert.Equal(t, "foo", res.Outputs["exp_static"].Value)
	assert.False(t, res.Outputs["exp_static"].Secret)
	assert.Equal(t, "abc", res.Outputs["exp_cfg"].Value)
	assert.False(t, res.Outputs["exp_cfg"].Secret)
	assert.Equal(t, "secret", res.Outputs["exp_secret"].Value)
	assert.True(t, res.Outputs["exp_secret"].Secret)
	assert.Equal(t, "update", res.Summary.Kind)
	assert.Equal(t, "succeeded", res.Summary.Result)

	// -- pulumi preview --

	var previewEvents []events.EngineEvent
	prevCh := make(chan events.EngineEvent)
	wg := collectEvents(prevCh, &previewEvents)
	prev, err := s.Preview(ctx, optremotepreview.EventStreams(prevCh))
	if err != nil {
		t.Errorf("preview failed, err: %v", err)
		t.FailNow()
	}
	wg.Wait()
	assert.Equal(t, 1, prev.ChangeSummary[apitype.OpSame])
	steps := countSteps(previewEvents)
	assert.Equal(t, 1, steps)

	// -- pulumi refresh --

	ref, err := s.Refresh(ctx)
	if err != nil {
		t.Errorf("refresh failed, err: %v", err)
		t.FailNow()
	}
	assert.Equal(t, "refresh", ref.Summary.Kind)
	assert.Equal(t, "succeeded", ref.Summary.Result)

	// -- pulumi destroy --

	dRes, err := s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)
}

func TestSelectRemoteStackGitSourceErrors(t *testing.T) {
	t.Parallel()
	testRemoteStackGitSourceErrors(t, SelectRemoteStackGitSource)
}

func TestNewRemoteStackGitSourceErrors(t *testing.T) {
	t.Parallel()
	testRemoteStackGitSourceErrors(t, NewRemoteStackGitSource)
}

func TestNewRemoteStackGitSource(t *testing.T) {
	t.Parallel()
	testRemoteStackGitSource(t, NewRemoteStackGitSource, true /*useCommitHash*/, false /*useExecutorImage*/)
}

func TestUpsertRemoteStackGitSourceErrors(t *testing.T) {
	t.Parallel()
	testRemoteStackGitSourceErrors(t, UpsertRemoteStackGitSource)
}

func TestUpsertRemoteStackGitSource(t *testing.T) {
	t.Parallel()
	testRemoteStackGitSource(t, UpsertRemoteStackGitSource, false /*useCommitHash*/, true /*useExecutorImage*/)
}

func TestIsFullyQualifiedStackName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "fully qualified", input: "owner/project/stack", expected: true},
		{name: "empty", input: "", expected: false},
		{name: "name", input: "name", expected: false},
		{name: "name & owner", input: "owner/name", expected: false},
		{name: "sep", input: "/", expected: false},
		{name: "two seps", input: "//", expected: false},
		{name: "three seps", input: "///", expected: false},
		{name: "invalid", input: "owner/project/stack/wat", expected: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := isFullyQualifiedStackName(tc.input)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
