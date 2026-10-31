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
	"bytes"
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

func setupMockPolicyTemplateDir(t *testing.T) string {
	t.Helper()
	templateDir := t.TempDir()
	t.Setenv("PULUMI_POLICY_TEMPLATE_PATH", templateDir)

	gitDir := filepath.Join(templateDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/master\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte(`[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
[remote "origin"]
	url = https://github.com/pulumi/templates-policy.git
`), 0o600))

	subDir := filepath.Join(templateDir, "aws-typescript")
	require.NoError(t, os.MkdirAll(subDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "PulumiPolicy.yaml"), []byte(`runtime: nodejs
description: A minimal AWS TypeScript policy pack
`), 0o600))

	return templateDir
}

//nolint:paralleltest // modifies environment
func TestPolicyNewListTemplates(t *testing.T) {
	setupMockPolicyTemplateDir(t)

	var stdout bytes.Buffer
	cmd := newPolicyNewCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--list-templates", "--offline"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	expectedOut := "Available Templates:\n  aws-typescript    A minimal AWS TypeScript policy pack\n"
	assert.Equal(t, expectedOut, stdout.String())
}

//nolint:paralleltest // modifies environment
func TestPolicyNewTemplateNotFound(t *testing.T) {
	setupMockPolicyTemplateDir(t)

	cmd := newPolicyNewCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"nonexistent", "--offline", "--force"})
	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)

	expectedErr := "template 'nonexistent' not found\n\n" +
		"Run 'pulumi policy new --list-templates' to see available templates\n\n" +
		"Available Templates:\n" +
		"  aws-typescript    A minimal AWS TypeScript policy pack"
	assert.EqualError(t, err, expectedErr)
}

func TestPolicyNewHelp(t *testing.T) {
	t.Parallel()

	cmd := newPolicyNewCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	assert.Contains(t, stdout.String(), "--list-templates")
	assert.Contains(t, stdout.String(), "aws-typescript")
	assert.Contains(t, stdout.String(), "aws-python")
	assert.Contains(t, stdout.String(), "aws-opa")
}
