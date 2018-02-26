// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tests

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
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

func TestStackErrors(t *testing.T) {
	t.Run("NoRepository", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "rm", "does-not-exist", "--yes")
		assert.Empty(t, stdout, "expected nothing to be written to stdout")
		assert.Contains(t, stderr, "error: no repository")
	})
}

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
		e.RunCommand("pulumi", "stack", "init", "--local", "foo")

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

	t.Run("StackInit", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)

		// Confirm passing both --local and --ppc fails.
		out, err := e.RunCommandExpectError("pulumi", "stack", "init", "foo", "--local", "--ppc", "bar")
		assert.Empty(t, out, "expected no stdout")
		assert.Contains(t, err, "cannot pass both --local and --ppc; PPCs only available in cloud mode")

		// Confirm passing both --local and --remote fails.
		out, err = e.RunCommandExpectError("pulumi", "stack", "init", "foo", "--local", "--remote")
		assert.Empty(t, out, "expected no stdout")
		assert.Contains(t, err, "cannot pass both --local and --remote")

		// Confirm passing --remote while logged out fails.
		e.RunCommand("pulumi", "logout")
		out, err = e.RunCommandExpectError("pulumi", "stack", "init", "foo", "--remote")
		assert.Empty(t, out, "expected no stdout")
		assert.Contains(t, err, "error: you must be logged in to create stacks in the Pulumi Cloud.")
		assert.Contains(t, err, "Run `pulumi login` to log in.")

		// Confirm stack init without --local works when logged out.
		e.RunCommand("pulumi", "stack", "init", "foo")
		stacks, current := integration.GetStacks(e)
		assert.Equal(t, 1, len(stacks))
		assert.NotNil(t, current)
		assert.Equal(t, "foo", *current)
		assert.Contains(t, stacks, "foo")
	})

	t.Run("StackSelect", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)
		e.RunCommand("pulumi", "stack", "init", "--local", "blighttown")
		e.RunCommand("pulumi", "stack", "init", "--local", "majula")
		e.RunCommand("pulumi", "stack", "init", "--local", "lothric")

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
	})

	t.Run("StackRm", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()

		integration.CreateBasicPulumiRepo(e)

		e.RunCommand("pulumi", "stack", "init", "--local", "blighttown")
		e.RunCommand("pulumi", "stack", "init", "--local", "majula")
		e.RunCommand("pulumi", "stack", "init", "--local", "lothric")
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

		// On macOS, e.RootPath will be something like:
		//     /var/folders/00/wttg611s0fl_91hpm8ff6g6c0000gn/T/test-env756909896
		// However, `/var` is actually a symbolic link to `/private/var`.
		// We evaluate the symbolic links to ensure the root path the test uses is
		// the same path that `pulumi` commands see.
		root, err := filepath.EvalSymlinks(e.RootPath)
		assert.NoError(t, err, "evaluating symbolic links of e.RootPath")

		// Get the path to the backup directory for this project.
		backupDir, err := getStackProjectBackupDir(root)
		assert.NoError(t, err, "getting stack project backup path")
		defer func() {
			if !t.Failed() {
				// Cleanup the backup directory.
				os.RemoveAll(backupDir)
			}
		}()

		// Create a stack.
		const stackName = "imulup"
		e.RunCommand("pulumi", "stack", "init", "--local", stackName)

		// Build the project.
		e.RunCommand("yarn", "install")
		e.RunCommand("yarn", "link", "@pulumi/pulumi")
		e.RunCommand("yarn", "run", "build")

		// Now run pulumi update.
		before := time.Now().UnixNano()
		e.RunCommand("pulumi", "update")
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
		e.RunCommand("pulumi", "destroy", "--yes")
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

func getStackProjectBackupDir(projectDir string) (string, error) {
	user, err := user.Current()
	if user == nil || err != nil {
		return "", fmt.Errorf("failed to get current user")
	}

	h := sha1.New()
	_, err = h.Write([]byte(projectDir))
	if err != nil {
		return "", fmt.Errorf("failed generating sha1")
	}
	hash := hex.EncodeToString(h.Sum(nil))

	return filepath.Join(
		user.HomeDir,
		workspace.BookkeepingDir,
		workspace.BackupDir,
		filepath.Base(projectDir)+"-"+hash,
	), nil
}
