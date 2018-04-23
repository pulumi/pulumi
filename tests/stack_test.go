// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package tests

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/testing/integration"
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
		e.ImportDirectory("integration/stack_outputs")

		// We're testing that backups are created so ensure backups aren't disabled.
		if env := os.Getenv(local.DisableCheckpointBackupsEnvVar); env != "" {
			os.Unsetenv(local.DisableCheckpointBackupsEnvVar)
			defer os.Setenv(local.DisableCheckpointBackupsEnvVar, env)
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
		e.RunCommand("yarn", "run", "build")

		// Now run pulumi update.
		before := time.Now().UnixNano()
		e.RunCommand("pulumi", "update", "--force")
		after := time.Now().UnixNano()

		// Verify the backup directory contains a single backup.
		files, err := ioutil.ReadDir(backupDir)
		assert.NoError(t, err, "getting the files in backup directory")
		if len(files) != 1 {
			t.Fatalf("Didn't find any files in backup directory.")
		}
		fileName := files[0].Name()

		// Verify the backup file.
		assertBackupStackFile(t, stackName, files[0], before, after)

		// Now run pulumi destroy.
		before = time.Now().UnixNano()
		e.RunCommand("pulumi", "destroy", "--force")
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
