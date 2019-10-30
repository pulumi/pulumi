// Copyright 2016-2019, Pulumi Corporation.
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
package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreatingPolicyPackWithArgsSpecifiedName(t *testing.T) {
	skipIfShort(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	var args = newPolicyArgs{
		interactive:       false,
		templateNameOrURL: "aws-typescript",
	}

	err := runNewPolicyPack(args)
	assert.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempdir, "PulumiPolicy.yaml"))
	assert.FileExists(t, filepath.Join(tempdir, "index.ts"))
}

func TestCreatingPolicyPackWithPromptedName(t *testing.T) {
	skipIfShort(t)

	tempdir, _ := ioutil.TempDir("", "test-env")
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	var args = newPolicyArgs{
		interactive:       true,
		templateNameOrURL: "aws-javascript",
	}

	err := runNewPolicyPack(args)
	assert.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempdir, "PulumiPolicy.yaml"))
	assert.FileExists(t, filepath.Join(tempdir, "index.js"))
}

func TestInvalidPolicyPackTemplateName(t *testing.T) {
	skipIfShort(t)

	// A template that will never exist.
	const nonExistantTemplate = "this-is-not-the-template-youre-looking-for"

	t.Run("RemoteTemplateNotFound", func(t *testing.T) {
		t.Parallel()
		tempdir, _ := ioutil.TempDir("", "test-env")
		defer os.RemoveAll(tempdir)
		assert.NoError(t, os.Chdir(tempdir))

		var args = newPolicyArgs{
			templateNameOrURL: nonExistantTemplate,
		}

		err := runNewPolicyPack(args)
		assert.Error(t, err)

		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("LocalTemplateNotFound", func(t *testing.T) {
		t.Parallel()

		tempdir, _ := ioutil.TempDir("", "test-env")
		defer os.RemoveAll(tempdir)
		assert.NoError(t, os.Chdir(tempdir))

		var args = newPolicyArgs{
			generateOnly:      true,
			offline:           true,
			templateNameOrURL: nonExistantTemplate,
			yes:               true,
		}

		err := runNewPolicyPack(args)
		assert.Error(t, err)

		assert.Contains(t, err.Error(), "not found")
	})
}
