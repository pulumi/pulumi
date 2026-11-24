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

package do

import (
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureGlobalProjectUsesPulumiHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PULUMI_HOME", home)

	proj, root, err := ensureGlobalProject()
	require.NoError(t, err)

	assert.Equal(t, globalProjectName, string(proj.Name))
	assert.Equal(t, filepath.Join(home, globalProjectName), root)
	require.FileExists(t, filepath.Join(root, "Pulumi.yaml"))
}

func TestEnsureGlobalStackFromUnrelatedCwd(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_BACKEND_URL", "file://"+filepath.ToSlash(t.TempDir()))
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test")

	proj, root, err := ensureGlobalProject()
	require.NoError(t, err)

	stack, err := ensureGlobalStack(t.Context(), diagtest.LogSink(t), pkgWorkspace.Instance,
		cmdBackend.DefaultLoginManager, proj, root, display.Options{})
	require.NoError(t, err)

	assert.Equal(t, globalStackName, stack.Ref().Name().String())
	require.FileExists(t, filepath.Join(root, "Pulumi.default.yaml"))
}
