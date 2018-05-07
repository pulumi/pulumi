// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package tests

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"testing"
	"time"

	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

// deleteIfNotFailed deletes the files in the testing environment if the testcase has
// not failed. (Otherwise they are left to aid debugging.)
func deleteIfNotFailed(e *ptesting.Environment) {
	if !e.T.Failed() {
		e.DeleteEnvironment()
	}
}

func TestPulumiNew(t *testing.T) {
	t.Run("SanityTest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// Create a temporary local template.
		template := createTemporaryLocalTemplate(t, "")
		defer deleteTemporaryLocalTemplate(t, template)

		// Create a subdirectory and CD into it.
		subdir := path.Join(e.RootPath, "foo")
		err := os.MkdirAll(subdir, os.ModePerm)
		assert.NoError(t, err, "error creating subdirectory")
		e.CWD = subdir

		// Run pulumi new.
		e.RunCommand("pulumi", "new", template, "--offline", "--generate-only", "--yes")

		assertSuccess(t, subdir, "foo", "")
	})

	t.Run("SanityTestWithManifest", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		const description = "My project description."

		// Create a temporary local template.
		template := createTemporaryLocalTemplate(t, description)
		defer deleteTemporaryLocalTemplate(t, template)

		// Create a subdirectory and CD into it.
		subdir := path.Join(e.RootPath, "foo")
		err := os.MkdirAll(subdir, os.ModePerm)
		assert.NoError(t, err, "error creating subdirectory")
		e.CWD = subdir

		// Run pulumi new.
		e.RunCommand("pulumi", "new", template, "--offline", "--generate-only", "--yes")

		assertSuccess(t, subdir, "foo", description)
	})

	t.Run("NoTemplateSpecified", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// Confirm this will result in an error since it isn't an
		// interactive terminal session.
		_, stderr := e.RunCommandExpectError("pulumi", "new")
		assert.NotEmpty(t, stderr)
	})

	t.Run("InvalidTemplateName", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// An invalid template name.
		template := "/this/is\\not/a/valid/templatename"

		// Confirm this fails.
		_, stderr := e.RunCommandExpectError("pulumi", "new", template)
		assert.NotEmpty(t, stderr)
	})

	t.Run("LocalTemplateNotFound", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// A template that will never exist remotely.
		template := "this-is-not-the-template-youre-looking-for"

		// Confirm this fails.
		_, stderr := e.RunCommandExpectError("pulumi", "new", template, "--offline", "--generate-only", "--yes")
		assert.NotEmpty(t, stderr)

		// Ensure the unknown template dir doesn't remain in the home directory.
		_, err := os.Stat(getTemplateDir(t, template))
		assert.Error(t, err, "dir shouldn't exist")
	})

	t.Run("RemoteTemplateNotFound", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// A template that will never exist remotely.
		template := "this-is-not-the-template-youre-looking-for"

		// Confirm this fails.
		_, stderr := e.RunCommandExpectError("pulumi", "new", template)
		assert.NotEmpty(t, stderr)

		// Ensure the unknown template dir doesn't remain in the home directory.
		_, err := os.Stat(getTemplateDir(t, template))
		assert.Error(t, err, "dir shouldn't exist")
	})

	t.Run("NameDescriptionPassedExplicitly", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// Create a temporary local template.
		template := createTemporaryLocalTemplate(t, "")
		defer deleteTemporaryLocalTemplate(t, template)

		// Run pulumi new.
		e.RunCommand("pulumi", "new", template, "--name", "bar", "--description", "A project.", "--offline", "--generate-only")

		assertSuccess(t, e.CWD, "bar", "A project.")
	})

	t.Run("WorkingDirNameIsntAValidProjectName", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// Create a temporary local template.
		template := createTemporaryLocalTemplate(t, "")
		defer deleteTemporaryLocalTemplate(t, template)

		// Create a subdirectory that contains an invalid char
		// for project names and CD into it.
		subdir := path.Join(e.RootPath, "@foo")
		err := os.MkdirAll(subdir, os.ModePerm)
		assert.NoError(t, err, "error creating subdirectory")
		e.CWD = subdir

		// Run pulumi new.
		e.RunCommand("pulumi", "new", template, "--offline", "--generate-only", "--yes")

		// Assert the default name used is "foo" not "@foo".
		assertSuccess(t, subdir, "foo", "")
	})

	t.Run("ExistingFileNotOverwritten", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// Create a temporary local template.
		template := createTemporaryLocalTemplate(t, "")
		defer deleteTemporaryLocalTemplate(t, template)

		// Create a subdirectory and CD into it.
		subdir := path.Join(e.RootPath, "foo")
		err := os.MkdirAll(subdir, os.ModePerm)
		assert.NoError(t, err, "error creating subdirectory")
		e.CWD = subdir

		// Create an existing Pulumi.yaml.
		err = ioutil.WriteFile(filepath.Join(subdir, "Pulumi.yaml"), []byte("name: blah"), 0600)
		assert.NoError(t, err, "creating Pulumi.yaml")

		// Confirm failure due to existing file.
		_, stderr := e.RunCommandExpectError("pulumi", "new", template, "--offline", "--generate-only", "--yes")
		assert.Contains(t, stderr, "Pulumi.yaml")

		// Confirm the contents of the file wasn't changed.
		content := readFile(t, filepath.Join(subdir, "Pulumi.yaml"))
		assert.Equal(t, "name: blah", content)

		// Confirm no other files were copied over.
		infos, err := ioutil.ReadDir(subdir)
		assert.NoError(t, err, "reading the dir")
		assert.Equal(t, 1, len(infos))
		assert.Equal(t, "Pulumi.yaml", infos[0].Name())
		assert.False(t, infos[0].IsDir())
	})

	t.Run("MultipleExistingFilesNotOverwritten", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// Create a temporary local template.
		template := createTemporaryLocalTemplate(t, "")
		defer deleteTemporaryLocalTemplate(t, template)

		// Create a subdirectory and CD into it.
		subdir := path.Join(e.RootPath, "foo")
		err := os.MkdirAll(subdir, os.ModePerm)
		assert.NoError(t, err, "error creating subdirectory")
		e.CWD = subdir

		// Create an existing Pulumi.yaml.
		err = ioutil.WriteFile(filepath.Join(subdir, "Pulumi.yaml"), []byte("name: blah"), 0600)
		assert.NoError(t, err, "creating Pulumi.yaml")

		// Create an existing test2.txt.
		err = ioutil.WriteFile(filepath.Join(subdir, "test2.txt"), []byte("foo"), 0600)
		assert.NoError(t, err, "creating test2.txt")

		// Confirm failure due to existing files.
		_, stderr := e.RunCommandExpectError("pulumi", "new", template, "--offline", "--generate-only", "--yes")
		assert.Contains(t, stderr, "Pulumi.yaml")
		assert.Contains(t, stderr, "test2.txt")

		// Confirm the contents of Pulumi.yaml wasn't changed.
		content := readFile(t, filepath.Join(subdir, "Pulumi.yaml"))
		assert.Equal(t, "name: blah", content)

		// Confirm the contents of test2.txt wasn't changed.
		content = readFile(t, filepath.Join(subdir, "test2.txt"))
		assert.Equal(t, "foo", content)

		// Confirm no other files were copied over.
		infos, err := ioutil.ReadDir(subdir)
		assert.NoError(t, err, "reading the dir")
		assert.Equal(t, 2, len(infos))
		for _, info := range infos {
			assert.True(t, info.Name() == "Pulumi.yaml" || info.Name() == "test2.txt")
			assert.False(t, info.IsDir())
		}
	})

	t.Run("MultipleExistingFilesForce", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		// Create a temporary local template.
		template := createTemporaryLocalTemplate(t, "")
		defer deleteTemporaryLocalTemplate(t, template)

		// Create a subdirectory and CD into it.
		subdir := path.Join(e.RootPath, "foo")
		err := os.MkdirAll(subdir, os.ModePerm)
		assert.NoError(t, err, "error creating subdirectory")
		e.CWD = subdir

		// Create an existing Pulumi.yaml.
		err = ioutil.WriteFile(filepath.Join(subdir, "Pulumi.yaml"), []byte("name: blah"), 0600)
		assert.NoError(t, err, "creating Pulumi.yaml")

		// Create an existing test2.txt.
		err = ioutil.WriteFile(filepath.Join(subdir, "test2.txt"), []byte("foo"), 0600)
		assert.NoError(t, err, "creating test2.txt")

		// Run pulumi new with --force.
		e.RunCommand("pulumi", "new", template, "--force", "--offline", "--generate-only", "--yes")

		assertSuccess(t, subdir, "foo", "")
	})
}

func assertSuccess(t *testing.T, dir string, expectedProjectName string, expectedProjectDescription string) {
	// Confirm the template file was copied/transformed.
	content := readFile(t, filepath.Join(dir, "Pulumi.yaml"))
	assert.Contains(t, content, fmt.Sprintf("name: %s", expectedProjectName))
	assert.Contains(t, content, fmt.Sprintf("description: %s", expectedProjectDescription))

	// Confirm the test1.txt file was copied.
	content = readFile(t, filepath.Join(dir, "test1.txt"))
	assert.Equal(t, "test1", content)

	// Confirm the test2.txt file was copied.
	content = readFile(t, filepath.Join(dir, "test2.txt"))
	assert.Equal(t, "test2", content)

	// Confirm the sub/blah.json file was copied.
	content = readFile(t, filepath.Join(dir, "sub", "blah.json"))
	assert.Equal(t, "{}", content)

	// Confirm the .pulumi.template.yaml file was skipped.
	_, err := os.Stat(filepath.Join(dir, ".pulumi.template.yaml"))
	assert.Error(t, err)

	infos, err := ioutil.ReadDir(dir)
	assert.NoError(t, err, "reading dir")
	assert.Equal(t, 4, len(infos))
}

func readFile(t *testing.T, filename string) string {
	b, err := ioutil.ReadFile(filename)
	assert.NoError(t, err, "reading file")
	return string(b)
}

func createTemporaryLocalTemplate(t *testing.T, description string) string {
	name := fmt.Sprintf("%v", time.Now().UnixNano())
	dir := getTemplateDir(t, name)
	err := os.MkdirAll(dir, 0700)
	assert.NoError(t, err, "creating temporary template dir")

	text := "name: ${PROJECT}\n" +
		"description: ${DESCRIPTION}\n" +
		"runtime: nodejs\n"
	err = ioutil.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte(text), 0600)
	assert.NoError(t, err, "creating Pulumi.yaml")

	err = ioutil.WriteFile(filepath.Join(dir, "test1.txt"), []byte("test1"), 0600)
	assert.NoError(t, err, "creating test1.txt")

	err = ioutil.WriteFile(filepath.Join(dir, "test2.txt"), []byte("test2"), 0600)
	assert.NoError(t, err, "creating test2.txt")

	err = os.MkdirAll(filepath.Join(dir, "sub"), os.ModePerm)
	assert.NoError(t, err, "creating sub")

	err = ioutil.WriteFile(filepath.Join(dir, "sub", "blah.json"), []byte("{}"), 0600)
	assert.NoError(t, err, "creating sub/blah.json")

	// If description is not empty, write it out in a manifest file.
	if description != "" {
		descriptionBytes := []byte(fmt.Sprintf("description: %s", description))
		err = ioutil.WriteFile(filepath.Join(dir, ".pulumi.template.yaml"), descriptionBytes, 0600)
		assert.NoError(t, err, "creating .pulumi.template.yaml")
	}

	return name
}

func deleteTemporaryLocalTemplate(t *testing.T, name string) {
	err := os.RemoveAll(getTemplateDir(t, name))
	assert.NoError(t, err, "deleting temporary template dir")
}

func getTemplateDir(t *testing.T, name string) string {
	user, err := user.Current()
	assert.NoError(t, err, "getting home directory")
	return filepath.Join(user.HomeDir, workspace.BookkeepingDir, workspace.TemplateDir, name)
}
