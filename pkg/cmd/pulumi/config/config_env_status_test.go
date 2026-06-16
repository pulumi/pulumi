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

package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestConfigEnvCheckedOutStatus verifies the bare `config env` status of a checked-out stack reports the
// source environment/revision and the working copy's imports, not the remote environment.
//
//nolint:paralleltest // mutates the working directory via t.Chdir
func TestConfigEnvCheckedOutStatus(t *testing.T) {
	ctx := t.Context()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: proj\nruntime: nodejs\n"), 0o600))
	t.Chdir(dir)

	path, err := cmdStack.WorkingCopyPath(tokens.QName("dev"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("environment:\n  - proj/base\nconfig:\n  proj:a: b\n"), 0o600))

	var stdout bytes.Buffer
	cmd := &configEnvCmd{
		stdout:           &stdout,
		diags:            diag.DefaultSink(&bytes.Buffer{}, &bytes.Buffer{}, diag.FormatOptions{Color: colors.Never}),
		loadProjectStack: cmdStack.LoadProjectStack,
	}
	marker := &pkgWorkspace.Checkout{EnvRef: "proj/env", Revision: 4, FilePath: path}

	require.NoError(t, cmd.runCheckedOutStatus(ctx, &workspace.Project{Name: "proj"}, discardStack(), marker, false))

	out := stdout.String()
	assert.Contains(t, out, "checked out")
	assert.Contains(t, out, "proj/env")
	assert.Contains(t, out, "revision 4")
	assert.Contains(t, out, "proj/base", "imports come from the working copy")
}
