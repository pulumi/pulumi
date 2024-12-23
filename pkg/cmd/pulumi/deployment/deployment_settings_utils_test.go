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

package deployment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoLookup(t *testing.T) {
	t.Parallel()
	t.Run("should handle directories that are not a git repo", func(t *testing.T) {
		t.Parallel()
		wd := "/"

		rl, err := newRepoLookup(wd)
		assert.NoError(t, err)
		assert.IsType(t, &noRepoLookupImpl{}, rl)

		dir, err := rl.GetRootDirectory(wd)
		assert.NoError(t, err)
		assert.Equal(t, ".", dir)

		branch := rl.GetBranchName()
		assert.Equal(t, "", branch)

		remote, err := rl.RemoteURL()
		assert.NoError(t, err)
		assert.Equal(t, "", remote)

		root := rl.GetRepoRoot()
		assert.Equal(t, "", root)
	})

	t.Run("should handle directories that are a git repo", func(t *testing.T) {
		t.Parallel()

		repoDir := setUpGitWorkspace(context.Background(), t)
		workDir := filepath.Join(repoDir, "goproj")

		rl, err := newRepoLookup(workDir)
		assert.NoError(t, err)
		assert.IsType(t, &repoLookupImpl{}, rl)

		dir, err := rl.GetRootDirectory(filepath.Join(workDir, "something"))
		assert.NoError(t, err)
		// should assure the directory is using linux path separator as deployments are
		// currently run only on linux images.
		assert.Equal(t, filepath.Join("goproj", "something"), dir)

		branch := rl.GetBranchName()
		assert.Equal(t, "refs/heads/master", branch)

		remote, err := rl.RemoteURL()
		assert.NoError(t, err)
		assert.Equal(t, "https://github.com/pulumi/test-repo.git", remote)

		assert.Equal(t, filepath.Dir(workDir), rl.GetRepoRoot())
	})
}

type relativeDirectoryValidationCase struct {
	Valid bool
	Path  string
}

func TestValidateRelativeDirectory(t *testing.T) {
	t.Parallel()

	repoDir := setUpGitWorkspace(context.Background(), t)
	workDir := filepath.Join(repoDir, "goproj")

	// relative directory values are always linux type paths
	pathsToTest := []relativeDirectoryValidationCase{
		{true, filepath.Join(".", "goproj")},
		{false, filepath.Join(".", "goproj", "child")},
		{false, filepath.Join(".", "goproj", "Pulumi.yaml")},
	}

	for _, c := range pathsToTest {
		err := ValidateRelativeDirectory(filepath.Dir(workDir))(c.Path)
		if c.Valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err, "invalid relative path %s", c.Path)
		}
	}
}

func TestValidateGitURL(t *testing.T) {
	t.Parallel()

	err := ValidateGitURL("https://github.com/pulumi/test-repo.git")
	require.NoError(t, err)

	err = ValidateGitURL("https://something.com")
	require.Error(t, err, "invalid Git URL")
}

func TestValidateShortInput(t *testing.T) {
	t.Parallel()

	err := ValidateShortInput("")
	require.NoError(t, err)

	err = ValidateShortInput("a")
	require.NoError(t, err)

	err = ValidateShortInput(strings.Repeat("a", 256))
	require.NoError(t, err)

	err = ValidateShortInput(strings.Repeat("a", 257))
	require.Error(t, err, "must be 256 characters or less")
}

func TestValidateShortInputNonEmpty(t *testing.T) {
	t.Parallel()

	err := ValidateShortInputNonEmpty("")
	require.Error(t, err, "should not be empty")

	err = ValidateShortInputNonEmpty("a")
	require.NoError(t, err)

	err = ValidateShortInputNonEmpty(strings.Repeat("a", 256))
	require.NoError(t, err)

	err = ValidateShortInputNonEmpty(strings.Repeat("a", 257))
	require.Error(t, err, "must be 256 characters or less")
}

func TestDSFileParsing(t *testing.T) {
	t.Parallel()

	stackDeploymentConfigFile := "Pulumi.stack.deploy.yaml"
	projectDir := t.TempDir()
	pyaml := filepath.Join(projectDir, stackDeploymentConfigFile)

	err := os.WriteFile(pyaml, []byte(`settings:
  sourceContext:
    git:
      repoUrl: git@github.com:pulumi/test-repo.git
      branch: main
      repoDir: .
      gitAuth:
        basicAuth:
          userName: jdoe
          password:
            ciphertext: AAABAMcGtHDraogfM3Qk4WyaNp3F/syk2cjHPQTb6Hu6ps8=
  operationContext:
    oidc:
      aws:
        duration: 1h0m0s
        policyArns:
          - policy:arn
        roleArn: the_role
        sessionName: the_session_name
    options:
      skipInstallDependencies: false
      skipIntermediateDeployments: true
  agentPoolID: 51035bee-a4d6-4b63-9ff6-418775c5da8d`), 0o600)
	require.NoError(t, err)

	deploymentFile, err := workspace.LoadProjectStackDeployment(pyaml)
	require.NoError(t, err)
	require.NotNil(t, deploymentFile)
	require.NotNil(t, deploymentFile.DeploymentSettings.SourceContext)
	require.NotNil(t, deploymentFile.DeploymentSettings.SourceContext.Git)
	require.Equal(t, "git@github.com:pulumi/test-repo.git", deploymentFile.DeploymentSettings.SourceContext.Git.RepoURL)
	require.Equal(t, "main", deploymentFile.DeploymentSettings.SourceContext.Git.Branch)
	require.Equal(t, ".", deploymentFile.DeploymentSettings.SourceContext.Git.RepoDir)
	assert.NotNil(t, deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth)
	assert.NotNil(t, deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth)
	assert.NotNil(t, deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName)
	assert.Equal(t, "jdoe", deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Value)
	assert.NotNil(t, deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.Password)
	assert.Equal(t, "AAABAMcGtHDraogfM3Qk4WyaNp3F/syk2cjHPQTb6Hu6ps8=",
		deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.Password.Ciphertext)
	require.NotNil(t, deploymentFile.DeploymentSettings.Operation)
	require.NotNil(t, deploymentFile.DeploymentSettings.Operation.Options)
	require.False(t, deploymentFile.DeploymentSettings.Operation.Options.SkipInstallDependencies)
	require.True(t, deploymentFile.DeploymentSettings.Operation.Options.SkipIntermediateDeployments)
	require.False(t, deploymentFile.DeploymentSettings.Operation.Options.DeleteAfterDestroy)
	require.False(t, deploymentFile.DeploymentSettings.Operation.Options.RemediateIfDriftDetected)
	assert.NotNil(t, deploymentFile.DeploymentSettings.Operation.OIDC)
	assert.Nil(t, deploymentFile.DeploymentSettings.Operation.OIDC.Azure)
	assert.Nil(t, deploymentFile.DeploymentSettings.Operation.OIDC.GCP)
	assert.NotNil(t, deploymentFile.DeploymentSettings.Operation.OIDC.AWS)
	assert.Equal(t, "the_session_name", deploymentFile.DeploymentSettings.Operation.OIDC.AWS.SessionName)
	assert.Equal(t, "the_role", deploymentFile.DeploymentSettings.Operation.OIDC.AWS.RoleARN)
	duration, _ := time.ParseDuration("1h0m0s")
	assert.Equal(t, apitype.DeploymentDuration(duration), deploymentFile.DeploymentSettings.Operation.OIDC.AWS.Duration)
	assert.Equal(t, []string{"policy:arn"}, deploymentFile.DeploymentSettings.Operation.OIDC.AWS.PolicyARNs)
	assert.Equal(t, "51035bee-a4d6-4b63-9ff6-418775c5da8d", *deploymentFile.DeploymentSettings.AgentPoolID)
}

func setUpGitWorkspace(ctx context.Context, t *testing.T) string {
	workDir, err := os.MkdirTemp("", "pulumi_deployment_settings")
	assert.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(workDir)
	})

	cloneOptions := &git.CloneOptions{
		RemoteName:    "origin",
		URL:           "https://github.com/pulumi/test-repo.git",
		Depth:         1,
		SingleBranch:  true,
		ReferenceName: plumbing.ReferenceName("master"),
	}

	_, err = git.PlainCloneContext(ctx, workDir, false, cloneOptions)
	assert.NoError(t, err)

	return workDir
}
