// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tests

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/backend"
	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// deleteIfNotFailed deletes the files in the testing environment if the testcase has
// not failed. (Otherwise they are left to aid debugging.)
func deleteIfNotFailed(e *ptesting.Environment) {
	if !e.T.Failed() {
		e.DeleteEnvironment()
	}
}

// assertHasNoHistory runs `pulumi history` and confirms an error that the stack has not
// ever been updated.
func assertHasNoHistory(e *ptesting.Environment) {
	// NOTE: pulumi returns with exit code 0 in this scenario.
	out, err := e.RunCommand("pulumi", "history")
	assert.Equal(e.T, "", err)
	assert.Equal(e.T, "Stack has never been updated\n", out)
}

func assertEnvValue(t *testing.T, update backend.UpdateInfo, key, val string) {
	t.Helper()
	v, ok := update.Environment[key]
	if !ok {
		t.Errorf("Did not find key %q in update environment", key)
	} else {
		assert.Equal(t, val, v, "Comparing Environment values for key %q", key)
	}
}

func assertEnvKeyNotFound(t *testing.T, update backend.UpdateInfo, key string) {
	t.Helper()
	_, found := update.Environment[key]
	assert.False(t, found, "Did not expect to find key %q in update environment", key)
}

func TestHistoryCommand(t *testing.T) {
	// We fail if no stack is selected.
	t.Run("NoStackSelected", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)
		integration.CreateBasicPulumiRepo(e)

		out, err := e.RunCommandExpectError("pulumi", "history")
		assert.Equal(t, "", out)
		assert.Contains(t, err, "error: no current stack detected")
	})

	// We don't display any history for a stack that has never been updated.
	t.Run("NoUpdates", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)
		integration.CreateBasicPulumiRepo(e)

		e.RunCommand("pulumi", "stack", "init", "no-updates-test", "--local")
		assertHasNoHistory(e)
	})

	// The "history" command uses the currently selected stack.
	t.Run("CurrentlySelectedStack", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)
		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("integration/stack_outputs")

		e.RunCommand("pulumi", "stack", "init", "stack-without-updates", "--local")
		e.RunCommand("pulumi", "stack", "init", "history-test", "--local")

		// Update the history-test stack.
		e.RunCommand("npm", "run", "build")
		e.RunCommand("pulumi", "update", "-m", "updating stack...")

		// Confirm we see the update message in thie history output.
		out, err := e.RunCommand("pulumi", "history")
		assert.Equal(t, "", err)
		assert.Contains(t, out, "updating stack...")

		// Change stack and confirm the history command honors the selected stack.
		e.RunCommand("pulumi", "stack", "select", "stack-without-updates")
		assertHasNoHistory(e)

		// Change stack back, and confirm still has history.
		e.RunCommand("pulumi", "stack", "select", "history-test")
		out, err = e.RunCommand("pulumi", "history")
		assert.Equal(t, "", err)
		assert.Contains(t, out, "updating stack...")
	})

	// That the history command contains accurate data about the update history.
	t.Run("Data(Deploy,Kind,Result)", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)
		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("integration/stack_outputs")

		e.RunCommand("pulumi", "stack", "init", "history-test", "--local")

		// Update the history-test stack.
		e.RunCommand("npm", "run", "build")
		e.RunCommand("pulumi", "update", "-m", "first update (successful)")

		// Now we "break" the program, by adding gibberish to bin/index.js.
		indexJS := path.Join(e.CWD, "bin", "index.js")
		origContents, err := ioutil.ReadFile(indexJS)
		assert.NoError(t, err, "Reading bin/index.js")

		var invalidJS bytes.Buffer
		invalidJS.Write(origContents)
		invalidJS.WriteString("\n\n... with random text -> syntax error, too")
		err = ioutil.WriteFile(indexJS, invalidJS.Bytes(), os.ModePerm)
		assert.NoError(t, err, "Writing bin/index.js")

		e.RunCommandExpectError("pulumi", "update", "-m", "second update (failure)")

		// Fix it
		err = ioutil.WriteFile(indexJS, origContents, os.ModePerm)
		assert.NoError(t, err, "Writing bin/index.js")
		e.RunCommand("pulumi", "update", "-m", "third update (successful)")

		// Destroy
		e.RunCommand("pulumi", "destroy", "-m", "fourth update (destroy)", "--yes")

		// Confirm the history is as expected. Output as JSON and parse the result.
		stdout, stderr := e.RunCommand("pulumi", "history", "--output-json")
		assert.Equal(t, "", stderr)

		var updateRecords []backend.UpdateInfo
		err = json.Unmarshal([]byte(stdout), &updateRecords)
		if err != nil {
			t.Fatalf("Error marshalling `pulumi history` output as JSON: %v", err)
		}
		if len(updateRecords) != 4 {
			t.Fatalf("didn't get expected number of updates from testcase. Raw history output:\n%v", stdout)
		}

		// The most recent updates are listed first.
		update := updateRecords[0]
		assert.Equal(t, 4, update.Version)
		assert.Equal(t, "fourth update (destroy)", update.Message)
		assert.True(t, backend.DestroyUpdate == update.Kind)
		assert.True(t, backend.SucceededResult == update.Result)

		update = updateRecords[1]
		assert.Equal(t, 3, update.Version)
		assert.Equal(t, "third update (successful)", update.Message)
		assert.True(t, backend.DeployUpdate == update.Kind)
		assert.True(t, backend.SucceededResult == update.Result)

		update = updateRecords[2]
		assert.Equal(t, 2, update.Version)
		assert.Equal(t, "second update (failure)", update.Message)
		assert.True(t, backend.DeployUpdate == update.Kind)
		assert.True(t, backend.FailedResult == update.Result)

		update = updateRecords[3]
		assert.Equal(t, 1, update.Version)
		assert.Equal(t, "first update (successful)", update.Message)
		assert.True(t, backend.DeployUpdate == update.Kind)
		assert.True(t, backend.SucceededResult == update.Result)

		if t.Failed() {
			t.Logf("Test failed. Printing raw history output:\n%v", stdout)
		}

		// Call stack rm to run the "delete history file too" codepath.
		e.RunCommand("pulumi", "stack", "rm", "--yes")
	})

	// We include git-related data in the environment metadata.
	t.Run("Data(Environment[Git])", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)
		integration.CreateBasicPulumiRepo(e)
		e.ImportDirectory("integration/stack_outputs")

		e.RunCommand("pulumi", "stack", "init", "history-test", "--local")
		e.RunCommand("npm", "run", "build")

		// Update 1, git repo that has no commits.
		e.RunCommand("pulumi", "update", "-m", "first update (git repo has no commits)")

		// Update 2, repo has commit, but no remote.
		e.RunCommand("git", "add", ".")
		e.RunCommand("git", "commit", "-m", "First commit of test files")
		e.RunCommand("pulumi", "update", "-m", "second update (git commit, no remote)")

		// Update 3, repo has remote and is dirty (by rewriting index.ts).
		indexTS := path.Join(e.CWD, "index.ts")
		origContents, err := ioutil.ReadFile(indexTS)
		assert.NoError(t, err, "Reading index.ts")

		err = ioutil.WriteFile(indexTS, []byte("change to file..."), os.ModePerm)
		assert.NoError(t, err, "writing index.ts")

		e.RunCommand("git", "remote", "add", "origin", "git@github.com:rick/c-132")
		e.RunCommand("pulumi", "update", "-m", "third update (is dirty, has remote)")

		// Update 4, repo is now clean again.
		err = ioutil.WriteFile(indexTS, origContents, os.ModePerm)
		assert.NoError(t, err, "writing index.ts")

		e.RunCommand("pulumi", "update", "-m", "fourth update (is clean)")

		// Confirm the history is as expected. Output as JSON and parse the result.
		stdout, stderr := e.RunCommand("pulumi", "history", "--output-json")
		assert.Equal(t, "", stderr)

		var updateRecords []backend.UpdateInfo
		err = json.Unmarshal([]byte(stdout), &updateRecords)
		if err != nil {
			t.Fatalf("Error marshalling `pulumi history` output as JSON: %v", err)
		}
		if len(updateRecords) != 4 {
			t.Fatalf("didn't get expected number of updates from testcase. Raw history output:\n%v", stdout)
		}

		// The first update doesn't have any git information, since
		// nothing has been committed yet.
		t.Log("Checking first update...")
		update := updateRecords[3]
		assertEnvKeyNotFound(e.T, update, "git.head")

		// The second update has a commit.
		t.Log("Checking second update...")
		update = updateRecords[2]
		// Get the commit SHA for the commit we make before the second update. It isn't 100% stable since
		// `CreateBasicPulumiRepo` runs `pulumi init` with a flag based on the current time.
		headSHA, ok := update.Environment["git.head"]
		assert.True(t, ok, "Didn't find git.head in environment")
		assert.Equal(t, 40, len(headSHA), "Commit SHA was not expected length")
		assertEnvValue(e.T, update, "git.dirty", "false")
		// The github-related info is still not set though.
		assertEnvKeyNotFound(e.T, update, "github.login")
		assertEnvKeyNotFound(e.T, update, "github.repo")

		// The third commit sets a remote (which we detect as a GitHub repo) and is now dirty.
		t.Log("Checking third update...")
		update = updateRecords[1]
		assertEnvValue(e.T, update, "git.head", headSHA)
		assertEnvValue(e.T, update, "git.dirty", "true")
		assertEnvValue(e.T, update, "github.login", "rick")
		assertEnvValue(e.T, update, "github.repo", "c-132")

		// The fourth commit is clean (by restoring to the previous commit).
		t.Log("Checking fourth update...")
		update = updateRecords[0]
		assertEnvValue(e.T, update, "git.head", headSHA)
		assertEnvValue(e.T, update, "git.dirty", "false")
		assertEnvValue(e.T, update, "github.login", "rick")
		assertEnvValue(e.T, update, "github.repo", "c-132")

		if t.Failed() {
			t.Logf("Test failed. Printing raw history output:\n%v", stdout)
		}
	})
}
