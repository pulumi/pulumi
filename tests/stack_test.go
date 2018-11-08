// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package tests

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend/filestate"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
)

func TestStackCommands(t *testing.T) {
	// stack init, stack ls, stack rm, stack ls
	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
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
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", "blighttown")
		e.RunCommand("pulumi", "stack", "init", "majula")
		e.RunCommand("pulumi", "stack", "init", "lothric")

		// Last one created is always selected.
		stacks, current := integration.GetStacks(e)
		if current == nil {
			t.Fatalf("No stack was labeled as current among: %v", stacks)
		}
		assert.Equal(t, "lothric", *current)

		// Select works
		e.RunCommand("pulumi", "stack", "select", "blighttown")
		stacks, current = integration.GetStacks(e)
		if current == nil {
			t.Fatalf("No stack was labeled as current among: %v", stacks)
		}
		assert.Equal(t, "blighttown", *current)

		// Error
		out, err := e.RunCommandExpectError("pulumi", "stack", "select", "anor-londo")
		assert.Empty(t, out)
		// local: "no stack with name 'anor-londo' found"
		// cloud: "Stack 'integration-test-59f645ba/pulumi-test/anor-londo' not found"
		assert.Contains(t, err, "anor-londo")
		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})

	t.Run("StackRm", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)

		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
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
		versions := []int{
			apitype.DeploymentSchemaVersionCurrent + 1,
			stack.DeploymentSchemaVersionOldestSupported - 1,
		}

		for _, deploymentVersion := range versions {
			t.Run(fmt.Sprintf("Version%d", deploymentVersion), func(t *testing.T) {
				e := ptesting.NewEnvironment(t)
				defer func() {
					if !t.Failed() {
						e.DeleteEnvironment()
					}
				}()

				integration.CreateBasicPulumiRepo(e)
				e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
				e.RunCommand("pulumi", "stack", "init", "the-abyss")
				stacks, _ := integration.GetStacks(e)
				assert.Equal(t, 1, len(stacks))

				stackFile := path.Join(e.RootPath, "stack.json")
				e.RunCommand("pulumi", "stack", "export", "--file", "stack.json")
				stackJSON, err := ioutil.ReadFile(stackFile)
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
				err = ioutil.WriteFile(stackFile, bytes, os.FileMode(os.O_CREATE))
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
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		stackName := addRandomSuffix("invalid-resources")
		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("integration/stack_dependencies")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", stackName)
		e.RunCommand("yarn", "install")
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		e.RunCommand("pulumi", "update", "--non-interactive", "--skip-preview")
		// We're going to futz with the stack a little so that one of the resources we just created
		// becomes invalid.
		stackFile := path.Join(e.RootPath, "stack.json")
		e.RunCommand("pulumi", "stack", "export", "--file", "stack.json")
		stackJSON, err := ioutil.ReadFile(stackFile)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		var deployment apitype.UntypedDeployment
		err = json.Unmarshal(stackJSON, &deployment)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		snap, err := stack.DeserializeUntypedDeployment(&deployment)
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
		v2deployment := stack.SerializeDeployment(snap)
		data, err := json.Marshal(&v2deployment)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		deployment.Deployment = data
		bytes, err := json.Marshal(&deployment)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		err = ioutil.WriteFile(stackFile, bytes, os.FileMode(os.O_CREATE))
		if !assert.NoError(t, err) {
			t.FailNow()
		}
		_, stderr := e.RunCommand("pulumi", "stack", "import", "--file", "stack.json")
		assert.Contains(t, stderr, fmt.Sprintf("removing pending operation 'deleting' on '%s'", res.URN))
		// The engine should be happy now that there are no invalid resources.
		e.RunCommand("pulumi", "update", "--non-interactive", "--skip-preview")
		e.RunCommand("pulumi", "stack", "rm", "--yes", "--force")
	})
}

func TestStackBackups(t *testing.T) {
	t.Run("StackBackupCreatedSanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("integration/stack_outputs/nodejs")

		// We're testing that backups are created so ensure backups aren't disabled.
		if env := os.Getenv(filestate.DisableCheckpointBackupsEnvVar); env != "" {
			os.Unsetenv(filestate.DisableCheckpointBackupsEnvVar)
			defer os.Setenv(filestate.DisableCheckpointBackupsEnvVar, env)
		}

		const stackName = "imulup"

		// Get the path to the backup directory for this project.
		backupDir, err := getStackProjectBackupDir(e, stackName)
		assert.NoError(t, err, "getting stack project backup path")
		defer func() {
			if !t.Failed() {
				// Cleanup the backup directory.
				os.RemoveAll(backupDir)
			}
		}()

		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
		e.RunCommand("pulumi", "stack", "init", stackName)

		// Build the project.
		e.RunCommand("yarn", "install")
		e.RunCommand("yarn", "link", "@pulumi/pulumi")

		// Now run pulumi update.
		before := time.Now().UnixNano()
		e.RunCommand("pulumi", "up", "--non-interactive", "--skip-preview")
		after := time.Now().UnixNano()

		// Verify the backup directory contains a single backup.
		files, err := ioutil.ReadDir(backupDir)
		assert.NoError(t, err, "getting the files in backup directory")
		assert.Equal(t, 1, len(files))
		fileName := files[0].Name()

		// Verify the backup file.
		assertBackupStackFile(t, stackName, files[0], before, after)

		// Now run pulumi destroy.
		before = time.Now().UnixNano()
		e.RunCommand("pulumi", "destroy", "--non-interactive", "--skip-preview")
		after = time.Now().UnixNano()

		// Verify the backup directory has been updated with 1 additional backups.
		files, err = ioutil.ReadDir(backupDir)
		assert.NoError(t, err, "getting the files in backup directory")
		assert.Equal(t, 2, len(files))

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

func assertBackupStackFile(t *testing.T, stackName string, file os.FileInfo, before int64, after int64) {
	assert.False(t, file.IsDir())
	assert.True(t, file.Size() > 0)
	split := strings.Split(file.Name(), ".")
	assert.Equal(t, 3, len(split))
	assert.Equal(t, stackName, split[0])
	parsedTime, err := strconv.ParseInt(split[1], 10, 64)
	assert.NoError(t, err, "parsing the time in the stack backup filename")
	assert.True(t, parsedTime > before)
	assert.True(t, parsedTime < after)
}

func getStackProjectBackupDir(e *ptesting.Environment, stackName string) (string, error) {
	return filepath.Join(e.RootPath,
		workspace.BookkeepingDir,
		workspace.BackupDir,
		stackName,
	), nil
}

func addRandomSuffix(s string) string {
	b := make([]byte, 4)
	_, err := cryptorand.Read(b)
	contract.AssertNoError(err)
	return s + "-" + hex.EncodeToString(b)
}
