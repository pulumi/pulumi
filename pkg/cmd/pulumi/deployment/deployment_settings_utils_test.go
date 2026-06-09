// Copyright 2016, Pulumi Corporation.
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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	require.NotNil(t, deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth)
	require.NotNil(t, deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth)
	require.NotNil(t, deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName)
	assert.Equal(t, "jdoe", deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Value)
	require.NotNil(t, deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.Password)
	assert.Equal(t, "AAABAMcGtHDraogfM3Qk4WyaNp3F/syk2cjHPQTb6Hu6ps8=",
		deploymentFile.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.Password.Ciphertext)
	require.NotNil(t, deploymentFile.DeploymentSettings.Operation)
	require.NotNil(t, deploymentFile.DeploymentSettings.Operation.Options)
	require.False(t, deploymentFile.DeploymentSettings.Operation.Options.SkipInstallDependencies)
	require.True(t, deploymentFile.DeploymentSettings.Operation.Options.SkipIntermediateDeployments)
	require.False(t, deploymentFile.DeploymentSettings.Operation.Options.DeleteAfterDestroy)
	require.False(t, deploymentFile.DeploymentSettings.Operation.Options.RemediateIfDriftDetected)
	require.NotNil(t, deploymentFile.DeploymentSettings.Operation.OIDC)
	assert.Nil(t, deploymentFile.DeploymentSettings.Operation.OIDC.Azure)
	assert.Nil(t, deploymentFile.DeploymentSettings.Operation.OIDC.GCP)
	require.NotNil(t, deploymentFile.DeploymentSettings.Operation.OIDC.AWS)
	assert.Equal(t, "the_session_name", deploymentFile.DeploymentSettings.Operation.OIDC.AWS.SessionName)
	assert.Equal(t, "the_role", deploymentFile.DeploymentSettings.Operation.OIDC.AWS.RoleARN)
	duration, _ := time.ParseDuration("1h0m0s")
	assert.Equal(t, apitype.DeploymentDuration(duration), deploymentFile.DeploymentSettings.Operation.OIDC.AWS.Duration)
	assert.Equal(t, []string{"policy:arn"}, deploymentFile.DeploymentSettings.Operation.OIDC.AWS.PolicyARNs)
	assert.Equal(t, "51035bee-a4d6-4b63-9ff6-418775c5da8d", *deploymentFile.DeploymentSettings.AgentPoolID)
}
