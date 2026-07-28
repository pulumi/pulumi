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

package policy

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // changes directory for process
func TestCreatingPolicyPackWithPromptedName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	tempdir := tempProjectDir(t)
	t.Chdir(tempdir)

	args := newPolicyArgs{
		templateNameOrURL: "aws-javascript",
	}

	err := runNewPolicyPack(t.Context(), args)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempdir, "PulumiPolicy.yaml"))
	assert.FileExists(t, filepath.Join(tempdir, "index.js"))
}

//nolint:paralleltest // changes directory for process
func TestPolicyNewRuntimeOptions(t *testing.T) {
	templateDir := t.TempDir()
	err := os.WriteFile(filepath.Join(templateDir, "PulumiPolicy.yaml"), []byte(`runtime: python
version: 0.0.1
`), 0o600)
	require.NoError(t, err)

	targetDir := t.TempDir()
	t.Chdir(targetDir)
	cmd := newPolicyNewCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		templateDir,
		"--generate-only",
		"--runtime-options", "toolchain=uv,virtualenv=.venv",
	})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	proj, _, _, err := ReadPolicyProject(targetDir)
	require.NoError(t, err)
	assert.Equal(t, "uv", proj.Runtime.Options()["toolchain"])
	assert.Equal(t, ".venv", proj.Runtime.Options()["virtualenv"])
}

//nolint:paralleltest // changes directory for process
func TestInvalidPolicyPackTemplateName(t *testing.T) {
	skipIfShortOrNoPulumiAccessToken(t)

	// A template that will never exist.
	const nonExistantTemplate = "this-is-not-the-template-youre-looking-for"

	t.Run("RemoteTemplateNotFound", func(t *testing.T) {
		tempdir := tempProjectDir(t)
		t.Chdir(tempdir)

		args := newPolicyArgs{
			templateNameOrURL: nonExistantTemplate,
		}

		err := runNewPolicyPack(t.Context(), args)
		assert.Error(t, err)
		assertNotFoundError(t, err)
	})

	t.Run("LocalTemplateNotFound", func(t *testing.T) {
		tempdir := tempProjectDir(t)
		t.Chdir(tempdir)

		args := newPolicyArgs{
			generateOnly:      true,
			offline:           true,
			templateNameOrURL: nonExistantTemplate,
		}

		err := runNewPolicyPack(t.Context(), args)
		assert.Error(t, err)
		assertNotFoundError(t, err)
	})
}
