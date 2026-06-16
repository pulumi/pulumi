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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestLoadEnvPreambleRoutesToWorkingCopy verifies the C1 fix: when a stack is checked out, loadEnvPreamble
// resolves the effective config file to the working copy, so config env add/rm/ls operate on it rather
// than mutating the shared remote environment.
//
//nolint:paralleltest // mutates the working directory via t.Chdir
func TestLoadEnvPreambleRoutesToWorkingCopy(t *testing.T) {
	ctx := t.Context()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), []byte("name: proj\nruntime: nodejs\n"), 0o600))
	t.Chdir(dir)

	path, err := cmdStack.WorkingCopyPath(tokens.QName("dev"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("environment:\n  - proj/base\nconfig:\n  proj:a: b\n"), 0o600))

	settings := &pkgWorkspace.Settings{
		Checkouts: map[string]pkgWorkspace.Checkout{checkoutFQN: {EnvRef: "proj/env", FilePath: path}},
	}
	stackRef, configFile := "", ""
	parent := &configEnvCmd{
		diags:            diag.DefaultSink(&bytes.Buffer{}, &bytes.Buffer{}, diag.FormatOptions{Color: colors.Never}),
		stackRef:         &stackRef,
		configFile:       &configFile,
		loadProjectStack: cmdStack.LoadProjectStack,
		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				return &workspace.Project{Name: "proj"}, "", nil
			},
			NewF: func() (pkgWorkspace.W, error) {
				return &pkgWorkspace.MockW{SettingsF: func() *pkgWorkspace.Settings { return settings }}, nil
			},
		},
		requireStack: func(
			_ context.Context, _ diag.Sink, _ pkgWorkspace.Context, _ cmdBackend.LoginManager,
			_ string, _ cmdStack.LoadOption, _ display.Options, _ string,
		) (backend.Stack, error) {
			return checkoutTestStack("proj/env", "", "etag", 1), nil
		},
	}

	ps, _, _, err := parent.loadEnvPreamble(ctx)
	require.NoError(t, err)
	assert.Equal(t, path, configFile, "configFile is resolved to the working copy")
	require.NotNil(t, ps.Environment)
	assert.Equal(t, []string{"proj/base"}, ps.Environment.Imports(), "imports loaded from the working copy")
}
