// Copyright 2026, Pulumi Corporation.
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

package cli

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// inferFixture wires up an envCommand with the minimum dependencies needed by the inference
// code: a virtual filesystem, an account (for parseRef defaults), and a workspace for the Pulumi
// IaC fallback.
type inferFixture struct {
	cmd *envCommand
	fs  testFS
}

func newInferFixture() *inferFixture {
	fs := testFS{MapFS: fstest.MapFS{}}
	esc := &escCommand{
		fs:         fs,
		ws:         &pkgWorkspace.MockContext{},
		stdin:      strings.NewReader(""),
		stderr:     io.Discard,
		colors:     colors.Never,
		pulumiHome: "pulumi-home",
		account:    Account{DefaultOrg: "default-org"},
	}
	return &inferFixture{
		cmd: &envCommand{esc: esc},
		fs:  fs,
	}
}

// setPulumiStack points the fixture's workspace at a Pulumi project rooted at root with the given
// selected stack. The stack's config file, if any, comes from the fixture's filesystem.
func (f *inferFixture) setPulumiStack(root, projectName, stack string) {
	f.cmd.esc.ws = &pkgWorkspace.MockContext{
		ReadProjectF: func(dir string) (*workspace.Project, string, error) {
			return &workspace.Project{Name: tokens.PackageName(projectName)}, root, nil
		},
		ReadProjectStackF: func(
			sink diag.Sink, project *workspace.Project, root, stackName string,
		) (*workspace.ProjectStack, string, error) {
			path := filepath.Join(root, "Pulumi."+stackName+".yaml")
			var data []byte
			if file, ok := f.fs.MapFS[path]; ok {
				data = file.Data
			}
			ps, err := workspace.LoadProjectStackBytes(sink, project, data, path, encoding.YAML)
			return ps, path, err
		},
		NewF: func(dir string) (pkgWorkspace.W, error) {
			return &pkgWorkspace.MockW{
				SettingsF: func() *pkgWorkspace.Settings {
					return &pkgWorkspace.Settings{Stack: stack}
				},
			}, nil
		},
	}
}

// writeFile adds a file to the fixture's virtual filesystem at the given (slash-separated, no
// leading slash) path.
func (f *inferFixture) writeFile(path, contents string) {
	f.fs.MapFS[path] = &fstest.MapFile{Data: []byte(contents), Mode: 0o600}
}

// writeDotESC adds a .esc.yaml to the fixture's virtual filesystem and records the user's
// acceptance of it.
func (f *inferFixture) writeDotESC(t *testing.T, path, contents string) {
	f.writeFile(path, contents)
	require.NoError(t, f.cmd.setTrust(path, []byte(contents), trustAccept))
}

func TestInferFSEnv_StringRef(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.writeDotESC(t, "home/user/project/.esc.yaml", "environment: my-org/my-project/my-env@v1\n")

	desc, source, err := f.cmd.inferFSEnv("home/user/project")
	require.NoError(t, err)

	ref, ok := desc.(environmentRef)
	require.True(t, ok, "expected environmentRef, got %T", desc)
	assert.Equal(t, "my-org", ref.orgName)
	assert.Equal(t, "my-project", ref.projectName)
	assert.Equal(t, "my-env", ref.envName)
	assert.Equal(t, "v1", ref.version)
	assert.Equal(t, ".esc.yaml", source)
}

func TestInferFSEnv_LegacyOneIdentifier(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.writeDotESC(t, "home/user/.esc.yaml", "environment: my-env\n")

	desc, source, err := f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)

	ref, ok := desc.(environmentRef)
	require.True(t, ok)
	assert.Equal(t, "default-org", ref.orgName)
	assert.Equal(t, client.DefaultProject, ref.projectName)
	assert.Equal(t, "my-env", ref.envName)
	assert.Equal(t, ".esc.yaml", source)
}

func TestInferFSEnv_Imports(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.writeDotESC(t, "home/user/.esc.yaml", `environment:
  organization: my-org
  imports:
    - my-project/my-env
    - other-project/other-env
`)

	desc, source, err := f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)

	list, ok := desc.(importList)
	require.True(t, ok, "expected importList, got %T", desc)
	assert.Equal(t, "my-org", list.orgName)
	assert.Equal(t, []string{"my-project/my-env", "other-project/other-env"}, list.imports)
	assert.Equal(t, ".esc.yaml", source)
}

func TestInferFSEnv_WalksUp(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.writeDotESC(t, "home/user/.esc.yaml", "environment: my-env\n")

	desc, source, err := f.cmd.inferFSEnv("home/user/project/sub/dir")
	require.NoError(t, err)

	ref, ok := desc.(environmentRef)
	require.True(t, ok)
	assert.Equal(t, "my-env", ref.envName)
	assert.Equal(t, filepath.Join("..", "..", "..", ".esc.yaml"), source)
}

func TestInferFSEnv_NoFile(t *testing.T) {
	t.Parallel()
	f := newInferFixture()

	desc, source, err := f.cmd.inferFSEnv("home/user/project")
	require.NoError(t, err)
	assert.Empty(t, source)
	assert.Nil(t, desc)
}

func TestInferFSEnv_EmptyEnvironmentField(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.writeFile("home/user/.esc.yaml", "environment:\n")

	desc, source, err := f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)
	assert.Empty(t, source)
	assert.Nil(t, desc)
}

func TestInferFSEnv_InvalidYAML(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.writeFile("home/user/.esc.yaml", "environment: { unclosed\n")

	_, _, err := f.cmd.inferFSEnv("home/user")
	assert.Error(t, err)
}

func TestInferFSEnv_NearestWins(t *testing.T) {
	t.Parallel()
	// A .esc.yaml in a closer ancestor takes precedence over one further up the tree.
	f := newInferFixture()
	f.writeDotESC(t, "home/.esc.yaml", "environment: outer-env\n")
	f.writeDotESC(t, "home/user/.esc.yaml", "environment: inner-env\n")

	desc, source, err := f.cmd.inferFSEnv("home/user/project")
	require.NoError(t, err)

	ref, ok := desc.(environmentRef)
	require.True(t, ok)
	assert.Equal(t, "inner-env", ref.envName)
	assert.Equal(t, filepath.Join("..", ".esc.yaml"), source)
}

func TestInferPulumiIaCEnv_NoProject(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	// The default workspace mock reports no Pulumi project.

	desc, source, err := f.cmd.inferPulumiIaCEnv("home/user")
	require.NoError(t, err)
	assert.Empty(t, source)
	assert.Nil(t, desc)
}

func TestInferPulumiIaCEnv_NoStackSelected(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.setPulumiStack("proj", "my-project", "")

	desc, source, err := f.cmd.inferPulumiIaCEnv("proj")
	require.NoError(t, err)
	assert.Empty(t, source)
	assert.Nil(t, desc)
}

func TestInferPulumiIaCEnv_NoImports(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.setPulumiStack("proj", "my-project", "my-org/my-project/prod")
	f.writeFile("proj/Pulumi.prod.yaml", "config:\n  my-project:foo: bar\n")

	desc, source, err := f.cmd.inferPulumiIaCEnv("proj")
	require.NoError(t, err)
	assert.Empty(t, source)
	assert.Nil(t, desc)
}

func TestInferPulumiIaCEnv_NoStackConfig(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.setPulumiStack("proj", "my-project", "my-org/my-project/prod")

	desc, source, err := f.cmd.inferPulumiIaCEnv("proj")
	require.NoError(t, err)
	assert.Empty(t, source)
	assert.Nil(t, desc)
}

func TestInferPulumiIaCEnv_OK(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.setPulumiStack("proj", "my-project", "my-org/my-project/prod")
	f.writeFile("proj/Pulumi.prod.yaml", `environment:
  imports:
    - my-project/my-env
`)

	desc, source, err := f.cmd.inferPulumiIaCEnv("proj")
	require.NoError(t, err)

	list, ok := desc.(importList)
	require.True(t, ok)
	assert.Equal(t, "my-org", list.orgName)
	assert.Equal(t, []string{"my-project/my-env"}, list.imports)
	assert.Equal(t,
		"Pulumi stack my-org/my-project/prod (Pulumi.prod.yaml)",
		source)
}

func TestInferPulumiIaCEnv_UnqualifiedStackName(t *testing.T) {
	t.Parallel()
	// Settings written by older CLIs may store an unqualified stack name; the owner is then the
	// account's default org.
	f := newInferFixture()
	f.setPulumiStack("proj", "my-project", "prod")
	f.writeFile("proj/Pulumi.prod.yaml", `environment:
  imports:
    - my-project/my-env
`)

	desc, source, err := f.cmd.inferPulumiIaCEnv("proj")
	require.NoError(t, err)

	list, ok := desc.(importList)
	require.True(t, ok)
	assert.Equal(t, "default-org", list.orgName)
	assert.Equal(t,
		"Pulumi stack prod (Pulumi.prod.yaml)",
		source)
}

func TestInferPulumiIaCEnv_InlineValuesOnly(t *testing.T) {
	t.Parallel()
	// An environment block with only inline values imports nothing.
	f := newInferFixture()
	f.setPulumiStack("proj", "my-project", "my-org/my-project/prod")
	f.writeFile("proj/Pulumi.prod.yaml", `environment:
  values:
    foo: bar
`)

	desc, source, err := f.cmd.inferPulumiIaCEnv("proj")
	require.NoError(t, err)
	assert.Empty(t, source)
	assert.Nil(t, desc)
}

func TestInferPulumiIaCEnv_MalformedStackConfig(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.setPulumiStack("proj", "my-project", "my-org/my-project/prod")
	f.writeFile("proj/Pulumi.prod.yaml", "environment: { unclosed\n")

	_, _, err := f.cmd.inferPulumiIaCEnv("proj")
	assert.Error(t, err)
}

func TestInferDefaultEnv_FSWins(t *testing.T) {
	t.Parallel()
	// When both .esc.yaml and Pulumi context are available, .esc.yaml wins.
	f := newInferFixture()
	f.cmd.esc.cwd = "proj"
	f.writeDotESC(t, "proj/.esc.yaml", "environment: fs-env\n")
	f.setPulumiStack("proj", "my-project", "my-org/my-project/dev")
	f.writeFile("proj/Pulumi.dev.yaml", "environment:\n  imports:\n    - a/b\n")

	desc, source, err := f.cmd.inferDefaultEnv()
	require.NoError(t, err)

	ref, ok := desc.(environmentRef)
	require.True(t, ok)
	assert.Equal(t, "fs-env", ref.envName)
	assert.Equal(t, ".esc.yaml", source)
}

func TestInferDefaultEnv_FallsBackToPulumi(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.cmd.esc.cwd = "proj"
	f.setPulumiStack("proj", "my-project", "my-org/my-project/dev")
	f.writeFile("proj/Pulumi.dev.yaml", "environment:\n  imports:\n    - a/b\n")

	desc, source, err := f.cmd.inferDefaultEnv()
	require.NoError(t, err)

	list, ok := desc.(importList)
	require.True(t, ok)
	assert.Equal(t, "my-org", list.orgName)
	assert.Equal(t,
		"Pulumi stack my-org/my-project/dev (Pulumi.dev.yaml)",
		source)
}

func TestInferDefaultEnv_NoSourceReturnsNil(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	f.cmd.esc.cwd = "home/user"
	// No .esc.yaml and no Pulumi project.

	desc, source, err := f.cmd.inferDefaultEnv()
	require.NoError(t, err)
	assert.Empty(t, source)
	assert.Nil(t, desc)
}

func TestInferFSEnv_UntrustedNonInteractive(t *testing.T) {
	t.Parallel()
	// A .esc.yaml the user has not accepted is ignored with a warning when the session is
	// non-interactive.
	f := newInferFixture()
	var stderr bytes.Buffer
	f.cmd.esc.stderr = &stderr
	f.writeFile("home/user/.esc.yaml", "environment: my-env\n")

	desc, source, err := f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)
	assert.Nil(t, desc)
	assert.Empty(t, source)
	assert.Contains(t, stderr.String(), "ignoring untrusted .esc.yaml")
}

func TestInferFSEnv_PromptAccept(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	var stderr bytes.Buffer
	f.cmd.esc.stderr = &stderr
	f.cmd.esc.interactive = true
	f.cmd.esc.stdin = strings.NewReader("y\n")
	f.writeFile("home/user/.esc.yaml", "environment: my-env\n")

	desc, _, err := f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)
	ref, ok := desc.(environmentRef)
	require.True(t, ok)
	assert.Equal(t, "my-env", ref.envName)
	assert.Contains(t, stderr.String(), "Accept this default environment configuration?")

	// The decision is recorded: a second resolution does not prompt.
	stderr.Reset()
	f.cmd.esc.stdin = strings.NewReader("")
	desc, _, err = f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)
	require.NotNil(t, desc)
	assert.Empty(t, stderr.String())
}

func TestInferFSEnv_PromptDeny(t *testing.T) {
	t.Parallel()
	f := newInferFixture()
	var stderr bytes.Buffer
	f.cmd.esc.stderr = &stderr
	f.cmd.esc.interactive = true
	f.cmd.esc.stdin = strings.NewReader("n\n")
	f.writeFile("home/user/.esc.yaml", "environment: my-env\n")

	desc, _, err := f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)
	assert.Nil(t, desc)

	// The denial is recorded: a second resolution does not prompt and still resolves nothing.
	stderr.Reset()
	f.cmd.esc.stdin = strings.NewReader("")
	desc, _, err = f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)
	assert.Nil(t, desc)
	assert.Empty(t, stderr.String())
}

func TestInferFSEnv_PromptNoAnswer(t *testing.T) {
	t.Parallel()
	// An empty answer ignores the file for this invocation only: no decision is recorded and the
	// next resolution prompts again.
	f := newInferFixture()
	var stderr bytes.Buffer
	f.cmd.esc.stderr = &stderr
	f.cmd.esc.interactive = true
	f.cmd.esc.stdin = strings.NewReader("\n")
	f.writeFile("home/user/.esc.yaml", "environment: my-env\n")

	desc, _, err := f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)
	assert.Nil(t, desc)

	stderr.Reset()
	f.cmd.esc.stdin = strings.NewReader("y\n")
	desc, _, err = f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)
	require.NotNil(t, desc)
	assert.Contains(t, stderr.String(), "Accept this default environment configuration?")
}

func TestInferFSEnv_ChangedContentsRePrompt(t *testing.T) {
	t.Parallel()
	// Editing an accepted .esc.yaml invalidates the recorded decision.
	f := newInferFixture()
	f.writeDotESC(t, "home/user/.esc.yaml", "environment: my-env\n")
	f.writeFile("home/user/.esc.yaml", "environment: other-env\n")

	desc, source, err := f.cmd.inferFSEnv("home/user")
	require.NoError(t, err)
	assert.Nil(t, desc)
	assert.Empty(t, source)
}
