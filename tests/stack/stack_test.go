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

package stack

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestStackCommands(t *testing.T) {
	// stack init, stack ls, stack rm, stack ls
	t.Run("SanityTest", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "foo")

		stacks, current := integration.GetStacks(e)
		assert.Equal(t, 1, len(stacks))
		assert.NotNil(t, current)
		if current == nil {
			t.Logf("stacks: %v, current: %v", stacks, current)
			t.Fatalf("No current stack?")
		}

		assert.Equal(t, "foo", *current)
		assert.Contains(t, stacks, "foo")

		e.RunCommand("pulumi", "stack", "rm", "foo", "--yes")

		stacks, _ = integration.GetStacks(e)
		assert.Equal(t, 0, len(stacks))
	})

	t.Run("StackSelect", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "blighttown")
		e.RunCommand("pulumi", "stack", "init", "majula")
		e.RunCommand("pulumi", "stack", "init", "lothric")

		// Last one created is always selected.
		stacks, current := integration.GetStacks(e)
		if current == nil {
			t.Fatalf("No stack was labeled as current among: %v", stacks)
		} else {
			assert.Equal(t, "lothric", *current)
		}

		// Select works
		e.RunCommand("pulumi", "stack", "select", "blighttown")
		stacks, current = integration.GetStacks(e)
		if current == nil {
			t.Fatalf("No stack was labeled as current among: %v", stacks)
		} else {
			assert.Equal(t, "blighttown", *current)
		}

		// Error
		out, err := e.RunCommandExpectError("pulumi", "stack", "select", "anor-londo")
		assert.Empty(t, out)
		// local: "no stack with name 'anor-londo' found"
		// cloud: "Stack 'integration-test-59f645ba/pulumi-test/anor-londo' not found"
		assert.Contains(t, err, "anor-londo")
		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})

	t.Run("StackInitNoSelect", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "first")
		e.RunCommand("pulumi", "stack", "init", "second")

		// Last one created is always selected.
		stacks, current := integration.GetStacks(e)
		if current == nil {
			t.Fatalf("No stack was labeled as current among: %v", stacks)
		} else {
			assert.Equal(t, "second", *current)
		}

		// Specifying `--no-select` prevents selection.
		e.RunCommand("pulumi", "stack", "init", "third", "--no-select")
		stacks, current = integration.GetStacks(e)
		if current == nil {
			t.Fatalf("No stack was labeled as current among: %v", stacks)
		} else {
			// "second" should still be selected.
			assert.Equal(t, "second", *current)
		}

		assert.Equal(t, 3, len(stacks))
		assert.Contains(t, stacks, "first")
		assert.Contains(t, stacks, "second")
		assert.Contains(t, stacks, "third")
	})

	t.Run("StackUnselect", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)
		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "one")
		e.RunCommand("pulumi", "stack", "init", "two")

		// Last one created is always selected.
		stacks, current := integration.GetStacks(e)
		if current == nil {
			t.Fatalf("No stack was labeled as current among: %v", stacks)
		} else {
			assert.Equal(t, "two", *current)
		}

		e.RunCommand("pulumi", "stack", "unselect")
		_, updatedCurrentStack := integration.GetStacks(e)
		if updatedCurrentStack != nil {
			t.Fatal("No stack should be selected after unselect was executed")
		}
	})

	t.Run("StackRm", func(t *testing.T) {
		t.Parallel()

		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)

		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "blighttown")
		e.RunCommand("pulumi", "stack", "init", "majula")
		e.RunCommand("pulumi", "stack", "init", "lothric")
		stacks, _ := integration.GetStacks(e)
		assert.Equal(t, 3, len(stacks))

		e.RunCommand("pulumi", "stack", "rm", "majula", "--yes")
		stacks, _ = integration.GetStacks(e)
		assert.Equal(t, 2, len(stacks))
		assert.Contains(t, stacks, "blighttown")
		assert.Contains(t, stacks, "lothric")

		e.RunCommand("pulumi", "stack", "rm", "lothric", "--yes")
		stacks, _ = integration.GetStacks(e)
		assert.Equal(t, 1, len(stacks))
		assert.Contains(t, stacks, "blighttown")

		e.RunCommand("pulumi", "stack", "rm", "blighttown", "--yes")
		stacks, _ = integration.GetStacks(e)
		assert.Equal(t, 0, len(stacks))

		// Error
		out, err := e.RunCommandExpectError("pulumi", "stack", "rm", "anor-londo", "--yes")
		assert.Empty(t, out)
		// local: .pulumi/stacks/pulumi-test/anor-londo.json: no such file or directory
		// cloud:  Stack 'integration-test-59f645ba/pulumi-test/anor-londo' not found
		assert.Contains(t, err, "anor-londo")
	})

	// Test that stack import fails if the version of the deployment we give it is not
	// one that the CLI supports.
	t.Run("CheckpointVersioning", func(t *testing.T) {
		t.Parallel()

		versions := []int{
			apitype.DeploymentSchemaVersionCurrent + 1,
			stack.DeploymentSchemaVersionOldestSupported - 1,
		}

		for _, deploymentVersion := range versions {
			//nolint:paralleltest // mutates environment variables
			t.Run(fmt.Sprintf("Version%d", deploymentVersion), func(t *testing.T) {
				e := ptesting.NewEnvironment(t)
				defer e.DeleteIfNotFailed()

				integration.CreateBasicPulumiRepo(e)
				e.SetBackend(e.LocalURL())
				e.RunCommand("pulumi", "stack", "init", "the-abyss")
				stacks, _ := integration.GetStacks(e)
				assert.Equal(t, 1, len(stacks))

				stackFile := path.Join(e.RootPath, "stack.json")
				e.RunCommand("pulumi", "stack", "export", "--file", "stack.json")
				stackJSON, err := os.ReadFile(stackFile)
				if !assert.NoError(t, err) {
					t.FailNow()
				}

				var deployment apitype.UntypedDeployment
				err = json.Unmarshal(stackJSON, &deployment)
				if !assert.NoError(t, err) {
					t.FailNow()
				}

				deployment.Version = deploymentVersion
				bytes, err := json.Marshal(deployment)
				assert.NoError(t, err)
				err = os.WriteFile(stackFile, bytes, os.FileMode(os.O_CREATE))
				if !assert.NoError(t, err) {
					t.FailNow()
				}

				stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "import", "--file", "stack.json")
				assert.Empty(t, stdout)
				switch {
				case deploymentVersion > apitype.DeploymentSchemaVersionCurrent:
					assert.Contains(t, stderr, "the stack 'the-abyss' is newer than what this version of the Pulumi CLI understands")
				case deploymentVersion < stack.DeploymentSchemaVersionOldestSupported:
					assert.Contains(t, stderr, "the stack 'the-abyss' is too old")
				}
			})
		}
	})

	t.Run("FixingInvalidResources", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()
		stackName := addRandomSuffix("invalid-resources")
		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("../integration/stack_dependencies")
		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", stackName)
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		e.RunCommand("yarn", "install")
		e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
		// We're going to futz with the stack a little so that one of the resources we just created
		// becomes invalid.
		stackFile := path.Join(e.RootPath, "stack.json")
		e.RunCommand("pulumi", "stack", "export", "--file", "stack.json")
		stackJSON, err := os.ReadFile(stackFile)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		var deployment apitype.UntypedDeployment
		err = json.Unmarshal(stackJSON, &deployment)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		t.Setenv("PULUMI_CONFIG_PASSPHRASE", "correct horse battery staple")
		snap, err := stack.DeserializeUntypedDeployment(
			context.Background(),
			&deployment, stack.DefaultSecretsProvider)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		// Let's say that the the CLI crashed during the deletion of the last resource and we've now got
		// invalid resources in the snapshot.
		res := snap.Resources[len(snap.Resources)-1]
		snap.PendingOperations = append(snap.PendingOperations, resource.Operation{
			Resource: res,
			Type:     resource.OperationTypeDeleting,
		})
		v3deployment, err := stack.SerializeDeployment(context.Background(), snap, false /* showSecrets */)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		data, err := json.Marshal(&v3deployment)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		deployment.Deployment = data
		bytes, err := json.Marshal(&deployment)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		err = os.WriteFile(stackFile, bytes, os.FileMode(os.O_CREATE))
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
		_, stderr := e.RunCommand("pulumi", "stack", "import", "--file", "stack.json")
		assert.Contains(t, stderr, fmt.Sprintf("removing pending operation 'deleting' on '%s'", res.URN))
		// The engine should be happy now that there are no invalid resources.
		e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
		e.RunCommand("pulumi", "stack", "rm", "--yes", "--force")
	})
}

func TestStackBackups(t *testing.T) {
	t.Run("StackBackupCreatedSanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer e.DeleteIfNotFailed()

		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("../integration/stack_outputs/nodejs")

		// We're testing that backups are created so ensure backups aren't disabled.
		disableCheckpointBackups := env.DIYBackendDisableCheckpointBackups.Var().Name()
		t.Setenv(disableCheckpointBackups, "")

		const stackName = "imulup"

		// Get the path to the backup directory for this project.
		backupDir, err := getStackProjectBackupDir(e, "stack_outputs", stackName)
		assert.NoError(t, err, "getting stack project backup path")
		defer func() {
			if !t.Failed() {
				// Cleanup the backup directory.
				os.RemoveAll(backupDir)
			}
		}()

		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", stackName)

		// Build the project.
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		e.RunCommand("yarn", "install")

		// Now run pulumi up.
		before := time.Now().UnixNano()
		e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")
		after := time.Now().UnixNano()

		// Verify the backup directory contains a single backup.
		files, err := os.ReadDir(backupDir)
		assert.NoError(t, err, "getting the files in backup directory")
		files = filterOutAttrsFiles(files)
		fileNames := getFileNames(files)
		assert.Equal(t, 1, len(files), "Files: %s", strings.Join(fileNames, ", "))
		fileName := files[0].Name()

		// Verify the backup file.
		assertBackupStackFile(t, stackName, files[0], before, after)

		// Now run pulumi destroy.
		before = time.Now().UnixNano()
		e.RunCommand("pulumi", "destroy", "--non-interactive", "--yes", "--skip-preview")
		after = time.Now().UnixNano()

		// Verify the backup directory has been updated with 1 additional backups.
		files, err = os.ReadDir(backupDir)
		assert.NoError(t, err, "getting the files in backup directory")
		files = filterOutAttrsFiles(files)
		fileNames = getFileNames(files)
		assert.Equal(t, 2, len(files), "Files: %s", strings.Join(fileNames, ", "))

		// Verify the new backup file.
		for _, file := range files {
			// Skip the file we previously verified.
			if file.Name() == fileName {
				continue
			}

			assertBackupStackFile(t, stackName, file, before, after)
		}

		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})
}

//nolint:paralleltest // mutates environment variables
func TestDestroySetsEncryptionsalt(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	const stackName = "imulup"
	stackFile := filepath.Join(e.RootPath, "Pulumi.imulup.yaml")
	var expectedSalt string

	// Set up the environment.
	{
		e.Setenv("PULUMI_CONFIG_PASSPHRASE", "")

		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("../integration/stack_outputs/nodejs")

		e.SetBackend(e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", stackName)

		// Build the project.
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		e.RunCommand("yarn", "install")

		e.RunCommand("pulumi", "config", "set", "--secret", "token", "cookie")

		// Now run pulumi up.
		e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")

		// See what the encryptionsalt is
		stackYAML, err := os.ReadFile(stackFile)
		require.NoError(t, err)
		var stackConfig workspace.ProjectStack
		err = yaml.Unmarshal(stackYAML, &stackConfig)
		require.NoError(t, err)
		expectedSalt = stackConfig.EncryptionSalt
	}

	// Remove `encryptionsalt` from `Pulumi.imulup.yaml`.
	preamble := "secretsprovider: passphrase\n"
	err := os.WriteFile(stackFile, []byte(preamble), 0o600)
	assert.NoError(t, err, "writing Pulumi.imulup.yaml")

	// Now run pulumi destroy.
	e.RunCommand("pulumi", "destroy", "--non-interactive", "--yes", "--skip-preview")

	// Check that the stack file has the right `encryptionsalt` set.
	stackYAML, err := os.ReadFile(stackFile)
	require.NoError(t, err)
	var stackConfig workspace.ProjectStack
	err = yaml.Unmarshal(stackYAML, &stackConfig)
	require.NoError(t, err)
	assert.Equal(t, expectedSalt, stackConfig.EncryptionSalt)

	e.RunCommand("pulumi", "stack", "rm", "--yes")
}

func TestStackRenameAfterCreate(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	stackName := addRandomSuffix("stack-rename")
	integration.CreateBasicPulumiRepo(e)
	e.SetBackend(e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", stackName)

	newName := addRandomSuffix("renamed-stack")
	e.RunCommand("pulumi", "stack", "rename", newName)
}

// TestStackRenameServiceAfterCreateBackend tests a few edge cases about renaming
// stacks owned by organizations in the service backend.
func TestStackRenameAfterCreateServiceBackend(t *testing.T) {
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Use the current username as the "organization" in certain operations.
	username, _ := e.RunCommand("pulumi", "whoami")
	orgName := strings.TrimSpace(username)

	// Create a basic project.
	stackName := addRandomSuffix("stack-rename-svcbe")
	stackRenameBase := addRandomSuffix("renamed-stack-svcbe")
	integration.CreateBasicPulumiRepo(e)
	e.RunCommand("pulumi", "stack", "init", fmt.Sprintf("%s/%s", orgName, stackName))

	// Create some configuration so that a per-project YAML file is generated.
	e.RunCommand("pulumi", "config", "set", "xyz", "abc")

	// Try to rename the stack to itself. This should fail.
	e.RunCommandExpectError("pulumi", "stack", "rename", stackName)

	// Try to rename this stack to a name outside of the current "organization".
	// This should fail since it is not currently legal to do so.
	e.RunCommandExpectError("pulumi", "stack", "rename", "fakeorg/"+stackRenameBase)

	// Next perform a legal rename. This should work.
	e.RunCommand("pulumi", "stack", "rename", orgName+"/"+stackRenameBase+"2")
	stdoutXyz2, _ := e.RunCommand("pulumi", "config", "get", "xyz")
	assert.Equal(t, "abc", strings.Trim(stdoutXyz2, "\r\n"))
}

func TestStackRemoteConfig(t *testing.T) {
	t.Parallel()
	// This test requires the service, as only the service supports orgs.
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	createRemoteConfigStack := func(t *testing.T) (*ptesting.Environment, string, string, string) {
		t.Helper()
		e := ptesting.NewEnvironment(t)
		e.Cleanup(func() {
			e.DeleteIfNotFailed()
		})

		stackName, err := resource.NewUniqueHex("test-name-", 8, -1)
		require.NoError(t, err)

		integration.CreateBasicPulumiRepo(e)
		stdOut, stdErr := e.RunCommand("pulumi", "stack", "init", stackName, "--remote-config")
		e.Cleanup(func() {
			e.RunCommand("pulumi", "stack", "rm", stackName, "--yes")
			e.RunCommand("pulumi", "env", "rm", "pulumi-test/"+stackName, "--yes")
		})
		return e, stackName, stdOut, stdErr
	}

	t.Run("stack init creates env", func(t *testing.T) {
		t.Parallel()

		e, stackName, stdOut, _ := createRemoteConfigStack(t)
		assert.Contains(t, stdOut, "Created environment pulumi-test/"+stackName+" for stack configuration")
		openOut, openErr := e.RunCommand("pulumi", "env", "open", "pulumi-test/"+stackName)
		assert.Empty(t, openErr)
		assert.Equal(t, "{}\n", openOut, "creates empty env")
	})

	t.Run("set config warning", func(t *testing.T) {
		t.Parallel()

		e, stackName, _, _ := createRemoteConfigStack(t)
		configSetOut, configSetErr := e.RunCommandExpectError(
			"pulumi", "config", "set", "provider-name:key.subkey", "value")
		assert.Empty(t, configSetOut)
		expectedConfigSetErr := fmt.Sprintf(
			"config set not supported for remote stack config: "+
				"use `pulumi env set pulumi-test/%s pulumiConfig.provider-name:key.subkey value",
			stackName)
		assert.Contains(t, configSetErr, expectedConfigSetErr, "directs user to use 'env set'")
	})

	t.Run("set secret warning", func(t *testing.T) {
		t.Parallel()

		e, stackName, _, _ := createRemoteConfigStack(t)
		configSetOut, configSetErr := e.RunCommandExpectError(
			"pulumi", "config", "set", "--secret", "secretKey", "password")
		assert.Empty(t, configSetOut)
		newVar := fmt.Sprintf(
			"config set not supported for remote stack config: "+
				"use `pulumi env set pulumi-test/%s pulumiConfig.pulumi-test:secretKey --secret <value>",
			stackName)
		assert.Contains(t, configSetErr, newVar, "should hide secret values")
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		e, stackName, _, _ := createRemoteConfigStack(t)
		envSetOut, envSetErr := e.RunCommand(
			"pulumi", "env", "set", "pulumi-test/"+stackName, "pulumiConfig.pulumi-test:key", "value")
		assert.Empty(t, envSetOut)
		assert.Empty(t, envSetErr)

		getOut, getErr := e.RunCommand("pulumi", "config", "get", "key")
		assert.Empty(t, getErr)
		assert.Equal(t, "value\n", getOut)
	})

	t.Run("get secret", func(t *testing.T) {
		t.Parallel()

		e, stackName, _, _ := createRemoteConfigStack(t)
		envSetOut, envSetErr := e.RunCommand(
			"pulumi", "env", "set", "pulumi-test/"+stackName, "pulumiConfig.pulumi-test:key", "--secret", "password")
		assert.Empty(t, envSetOut)
		assert.Empty(t, envSetErr)

		getOut, getErr := e.RunCommand("pulumi", "config", "get", "key")
		assert.Empty(t, getErr)
		assert.Equal(t, "password\n", getOut)

		configOut, configErr := e.RunCommand("pulumi", "config")
		assert.Empty(t, configErr)
		assert.Contains(t, configOut, "key", "includes key")
		assert.NotContains(t, configOut, "password", "hides secret value")
	})

	t.Run("rm warning", func(t *testing.T) {
		t.Parallel()

		e, stackName, _, _ := createRemoteConfigStack(t)
		configRmOut, configRmErr := e.RunCommandExpectError("pulumi", "config", "rm", "key")
		assert.Empty(t, configRmOut)
		expectedConfigRmErr := fmt.Sprintf(
			"config rm not supported for remote stack config: "+
				"use `pulumi env rm pulumi-test/%s pulumiConfig.pulumi-test:key",
			stackName)
		assert.Contains(t, configRmErr, expectedConfigRmErr, "direct user to use 'env rm'")

		envRmOut, envRmErr := e.RunCommand("pulumi", "env", "rm", "pulumi-test/"+stackName, "pulumiConfig.foo")
		assert.Empty(t, envRmOut)
		assert.Empty(t, envRmErr)
	})
}

func TestLocalStateLocking(t *testing.T) {
	t.Skip() // TODO[pulumi/pulumi#7269] flaky test
	t.Parallel()
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	integration.CreateBasicPulumiRepo(e)
	e.ImportDirectory("../integration/single_resource")
	e.SetBackend(e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "foo")
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")

	count := 10
	stderrs := make(chan string, count)

	// Run 10 concurrent updates
	for i := 0; i < count; i++ {
		go func() {
			_, stderr, err := e.GetCommandResults("pulumi", "up", "--non-interactive", "--skip-preview", "--yes")
			if err == nil {
				stderrs <- "" // success marker
			} else {
				stderrs <- stderr
			}
		}()
	}

	// Ensure that only one of the concurrent updates succeeded, and that failures
	// were due to locking (and not state file corruption)
	numsuccess := 0
	numerrors := 0

	for i := 0; i < count; i++ {
		stderr := <-stderrs
		if stderr == "" {
			assert.Equal(t, 0, numsuccess, "more than one concurrent update succeeded")
			numsuccess++
		} else {
			phrase := "the stack is currently locked by 1 lock(s)"
			if !strings.Contains(stderr, phrase) {
				numerrors++
				t.Logf("unexplaiend stderr::\n%s", stderr)
				assert.Lessf(t, numerrors, 2, "More than one unexplained error has occurred")
			}
		}
	}

	// Run 10 concurrent previews
	for i := 0; i < count; i++ {
		go func() {
			_, stderr, err := e.GetCommandResults("pulumi", "preview", "--non-interactive")
			if err == nil {
				stderrs <- "" // success marker
			} else {
				stderrs <- stderr
			}
		}()
	}

	// Ensure that all of the concurrent previews succeed.
	for i := 0; i < count; i++ {
		stderr := <-stderrs
		assert.Equal(t, "", stderr)
	}
}

// stackFileFormatAsserters returns a function to assert that the current file
// format is for gzip and plain formats respectively.
func stackFileFormatAsserters(t *testing.T, e *ptesting.Environment, projectName, stackName string) (func(), func()) {
	stacksDir := filepath.Join(".pulumi", "stacks", projectName)
	pathStack := filepath.Join(stacksDir, stackName+".json")
	pathStackGzip := pathStack + ".gz"
	pathStackBak := pathStack + ".bak"
	pathStackBakGzip := pathStack + ".gz.bak"
	sizeGzip := int64(-1)
	sizeJSON := int64(-1)

	doAssert := func(gzip bool) {
		gzipStackInfo, err := os.Stat(filepath.Join(e.CWD, pathStackGzip))
		if err == nil {
			sizeGzip = gzipStackInfo.Size()
		}
		jsonStackInfo, err := os.Stat(filepath.Join(e.CWD, pathStack))
		if err == nil {
			sizeJSON = jsonStackInfo.Size()
		}

		// We need to make sure that an out of date state file doesn't exist
		assert.Equal(t, gzip, e.PathExists(pathStackGzip), "gzip stack file ")
		assert.Equal(t, !gzip, e.PathExists(pathStack), "Raw json stack file")
		if gzip {
			assert.True(t, e.PathExists(pathStackBakGzip), "gzip backup")
		} else {
			assert.True(t, e.PathExists(pathStackBak), "raw backup")
		}
		if sizeGzip != -1 && sizeJSON != -1 {
			assert.Greater(t, sizeJSON, sizeGzip, "Json file smaller than gzip")
		}
		if t.Failed() {
			fmt.Printf("Stacks dir state at time of failure (gzip: %t):\n", gzip)
			files, _ := os.ReadDir(e.CWD + "/" + stacksDir)
			for _, file := range files {
				fi, err := file.Info()
				if err != nil {
					fmt.Printf("failed to read file info: %s\n", file.Name())
					continue
				}
				fmt.Println(fi.Name(), fi.Size())
			}
		}
	}
	return func() { doAssert(true) }, func() { doAssert(false) }
}

func TestLocalStateGzip(t *testing.T) { //nolint:paralleltest
	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	stackName := addRandomSuffix("gzip-state")
	integration.CreateBasicPulumiRepo(e)
	e.ImportDirectory("../integration/stack_dependencies")
	e.SetBackend(e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", stackName)
	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")
	e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")

	assertGzipFileFormat, assertPlainFileFormat := stackFileFormatAsserters(t, e, "stack_dependencies", stackName)
	gzipEnvVar := env.DIYBackendGzip.Var().Name()
	switchGzipOff := func() { e.Setenv(gzipEnvVar, "0") }
	switchGzipOn := func() { e.Setenv(gzipEnvVar, "1") }
	pulumiUp := func() { e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview") }

	// Test "pulumi up" with gzip compression on and off.
	// Default is no gzip compression
	switchGzipOff()
	assertPlainFileFormat()

	// Enable Gzip compression
	switchGzipOn()
	pulumiUp()
	// Running "pulumi up" 2 times is important because normally, first the
	// `.json` becomes `.json.gz`, then the `.json.bak` becomes `.json.gz.bak`.
	pulumiUp()
	assertGzipFileFormat()

	pulumiUp()
	assertGzipFileFormat()

	// Disable Gzip compression
	switchGzipOff()
	pulumiUp()
	assertPlainFileFormat()

	pulumiUp()
	assertPlainFileFormat()

	// Check stack history is still good even with mixed gzip / json files
	rawHistory, _ := e.RunCommand("pulumi", "stack", "history", "--json")
	var history []interface{}
	if err := json.Unmarshal([]byte(rawHistory), &history); err != nil {
		t.Fatalf("Can't unmarshall history json")
	}
	assert.Equal(t, 6, len(history), "Stack history doesn't match reality")
}

func getFileNames(infos []os.DirEntry) []string {
	result := slice.Prealloc[string](len(infos))
	for _, i := range infos {
		result = append(result, i.Name())
	}
	return result
}

func filterOutAttrsFiles(files []os.DirEntry) []os.DirEntry {
	var result []os.DirEntry
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".attrs" {
			result = append(result, f)
		}
	}
	return result
}

func assertBackupStackFile(t *testing.T, stackName string, file os.DirEntry, before int64, after int64) {
	assert.False(t, file.IsDir())
	fi, err := file.Info()
	assert.NoError(t, err)
	assert.True(t, fi.Size() > 0)
	split := strings.Split(file.Name(), ".")
	assert.Equal(t, 3, len(split), "Split: %s", strings.Join(split, ", "))
	assert.Equal(t, stackName, split[0])
	parsedTime, err := strconv.ParseInt(split[1], 10, 64)
	assert.NoError(t, err, "parsing the time in the stack backup filename")
	assert.True(t, parsedTime > before, "False: %v > %v", parsedTime, before)
	assert.True(t, parsedTime < after, "False: %v < %v", parsedTime, after)
}

func getStackProjectBackupDir(e *ptesting.Environment, projectName, stackName string) (string, error) {
	return filepath.Join(e.RootPath,
		workspace.BookkeepingDir,
		workspace.BackupDir,
		projectName,
		stackName,
	), nil
}

func addRandomSuffix(s string) string {
	b := make([]byte, 4)
	_, err := cryptorand.Read(b)
	contract.AssertNoErrorf(err, "error generating random suffix")
	return s + "-" + hex.EncodeToString(b)
}

func TestStackTags(t *testing.T) {
	t.Parallel()

	// This test requires the service, as only the service supports stack tags.
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}
	if os.Getenv("PULUMI_TEST_OWNER") == "" {
		t.Skipf("Skipping: PULUMI_TEST_OWNER is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()
	stackName, err := resource.NewUniqueHex("test-", 8, -1)
	contract.AssertNoErrorf(err, "resource.NewUniqueHex should not fail with no maximum length is set")

	integration.CreateBasicPulumiRepo(e)
	e.ImportDirectory("testdata/simple_tags")

	e.RunCommand("pulumi", "stack", "init", stackName)

	e.RunCommand("pulumi", "stack", "tag", "set", "tagA", "valueA")
	e.RunCommand("pulumi", "stack", "tag", "set", "tagB", "valueB")

	lsTags := func() map[string]string {
		stdout, _ := e.RunCommand("pulumi", "stack", "tag", "ls", "--json")
		var tags map[string]string
		err = json.Unmarshal([]byte(stdout), &tags)
		require.NoError(t, err, "parsing the tags json")
		return tags
	}

	tags := lsTags()
	assert.Equal(t, "valueA", tags["tagA"], "tagA should be set to valueA")
	assert.Equal(t, "valueB", tags["tagB"], "tagB should be set to valueB")

	e.RunCommand("pulumi", "stack", "tag", "rm", "tagA")
	tags = lsTags()
	assert.NotContains(t, tags, "tagA", "tagA should be removed")

	e.RunCommand("yarn", "link", "@pulumi/pulumi")
	e.RunCommand("yarn", "install")
	e.RunCommand("pulumi", "up", "--non-interactive", "--yes", "--skip-preview")

	tags = lsTags()
	assert.Equal(t, "hello", tags["tagS"], "tagS should be set to hello")
	assert.Equal(t, "true", tags["tagB"], "tagB should be set to true")
	assert.Equal(t, "42", tags["tagN"], "tagN should be set to 42")
}

//nolint:paralleltest // pulumi new is not parallel safe
func TestNewStackConflictingOrg(t *testing.T) {
	// This test requires the service, as only the service supports orgs.
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	project, err := resource.NewUniqueHex("test-name-", 8, -1)
	require.NoError(t, err)

	// `new` wants to work in an empty directory but our use of local url means we have a
	// ".pulumi" directory at root.
	projectDir := filepath.Join(e.RootPath, project)
	err = os.Mkdir(projectDir, 0o700)
	require.NoError(t, err)

	e.CWD = projectDir

	orgs := []string{"moolumi", "pulumi-test"}
	for _, org := range orgs {
		stackRef := fmt.Sprintf("%s/%s/stack", org, project)
		// Ensure projects no longer exists. Ignoring errors.
		_, _, err := e.GetCommandResults("pulumi", "stack", "rm", "-s", stackRef)
		_ = err
	}
	for _, org := range orgs {
		stackRef := fmt.Sprintf("%s/%s/stack", org, project)
		e.RunCommand("pulumi", "new", "yaml", "-s", stackRef, "--yes", "--force")
		e.RunCommand("pulumi", "up", "--yes")
		e.RunCommand("pulumi", "destroy", "--yes", "--remove")
	}
}
