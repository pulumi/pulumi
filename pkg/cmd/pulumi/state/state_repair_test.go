// Copyright 2024, Pulumi Corporation.
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

package state

import (
	"bytes"
	"context"
	"encoding/json"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // State repairing modifies the DisableIntegrityChecking global variable
func TestStateRepair_ExitsIfTheStateIsAlreadyValid(t *testing.T) {
	// Arrange.
	cases := []struct {
		name      string
		resources []*resource.State
	}{
		{
			name:      "empty",
			resources: []*resource.State{},
		},
		{
			name: "no dependencies",
			resources: []*resource.State{
				{URN: "a"},
				{URN: "b"},
				{URN: "c"},
			},
		},
		{
			name: "valid dependencies",
			resources: []*resource.State{
				{URN: "a"},
				{URN: "b", Dependencies: []resource.URN{"a"}},
				{URN: "c", Dependencies: []resource.URN{"b"}},
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			fx := newStateRepairCmdFixture(t, []*resource.State{})

			// Act.
			err := fx.cmd.run(context.Background())

			// Assert.
			assert.NoError(t, err)
			assert.Contains(t, fx.stdout.String(), "already valid")
			assert.Nil(t, fx.imported, "Import should not have proceeded")
		})
	}
}

//nolint:paralleltest // State repairing modifies the DisableIntegrityChecking global variable
func TestStateRepair_ConfirmationIncludesReorderSummary(t *testing.T) {
	// Survey (the library we currently use for managing prompt input) does not currently pick up inputs when tested under
	// Windows. Consequently we skip if the test is running on Windows.
	if runtime.GOOS == "windows" {
		t.Skip("Skipping: mocking input to Survey is not supported on Windows")
	}

	// Arrange.
	fx := newStateRepairCmdFixture(t, []*resource.State{
		{URN: "b", Dependencies: []resource.URN{"a"}},
		{URN: "a"},
	})

	fx.stdin.buf.WriteString("no\r\n")

	// Act.
	err := fx.cmd.run(context.Background())

	// Assert.
	assert.NoError(t, err)
	assert.Contains(t, fx.stdout.String(), "will be reordered")
	assert.NotContains(t, fx.stdout.String(), "will be modified")
}

//nolint:paralleltest // State repairing modifies the DisableIntegrityChecking global variable
func TestStateRepair_ConfirmationIncludesModificationSummary(t *testing.T) {
	// Survey (the library we currently use for managing prompt input) does not currently pick up inputs when tested under
	// Windows. Consequently we skip if the test is running on Windows.
	if runtime.GOOS == "windows" {
		t.Skip("Skipping: mocking input to Survey is not supported on Windows")
	}

	// Arrange.
	fx := newStateRepairCmdFixture(t, []*resource.State{
		{URN: "c", Dependencies: []resource.URN{"d"}},
	})

	fx.stdin.buf.WriteString("no\r\n")

	// Act.
	err := fx.cmd.run(context.Background())

	// Assert.
	assert.NoError(t, err)
	assert.NotContains(t, fx.stdout.String(), "will be reordered")
	assert.Contains(t, fx.stdout.String(), "will be modified")
}

//nolint:paralleltest // State repairing modifies the DisableIntegrityChecking global variable
func TestStateRepair_ConfirmationIncludesCombinedSummaries(t *testing.T) {
	// Survey (the library we currently use for managing prompt input) does not currently pick up inputs when tested under
	// Windows. Consequently we skip if the test is running on Windows.
	if runtime.GOOS == "windows" {
		t.Skip("Skipping: mocking input to Survey is not supported on Windows")
	}

	// Arrange.
	fx := newStateRepairCmdFixture(t, []*resource.State{
		{URN: "b", Dependencies: []resource.URN{"a"}},
		{URN: "a"},
		{URN: "c", Dependencies: []resource.URN{"d"}},
	})

	fx.stdin.buf.WriteString("no\r\n")

	// Act.
	err := fx.cmd.run(context.Background())

	// Assert.
	assert.NoError(t, err)
	assert.Contains(t, fx.stdout.String(), "will be reordered")
	assert.Contains(t, fx.stdout.String(), "will be modified")
}

//nolint:paralleltest // State repairing modifies the DisableIntegrityChecking global variable
func TestStateRepair_PromptsForConfirmationAndCancels(t *testing.T) {
	// Survey (the library we currently use for managing prompt input) does not currently pick up inputs when tested under
	// Windows. Consequently we skip if the test is running on Windows.
	if runtime.GOOS == "windows" {
		t.Skip("Skipping: mocking input to Survey is not supported on Windows")
	}

	// Arrange.
	fx := newStateRepairCmdFixture(t, []*resource.State{
		{URN: "b", Dependencies: []resource.URN{"a"}},
		{URN: "a"},
	})

	fx.stdin.buf.WriteString("no\r\n")

	// Act.
	err := fx.cmd.run(context.Background())

	// Assert.
	assert.NoError(t, err)
	assert.Contains(t, fx.stdout.String(), "Confirm?")
	assert.Nil(t, fx.imported, "Import should not have proceeded")
}

//nolint:paralleltest // State repairing modifies the DisableIntegrityChecking global variable
func TestStateRepair_PromptsForConfirmationAndProceeds(t *testing.T) {
	// Survey (the library we currently use for managing prompt input) does not currently pick up inputs when tested under
	// Windows. Consequently we skip if the test is running on Windows.
	if runtime.GOOS == "windows" {
		t.Skip("Skipping: mocking input to Survey is not supported on Windows")
	}

	// Arrange.
	fx := newStateRepairCmdFixture(t, []*resource.State{
		{URN: "b", Dependencies: []resource.URN{"a"}},
		{URN: "a"},
	})

	fx.stdin.buf.WriteString("yes\r\n")

	// Act.
	err := fx.cmd.run(context.Background())

	// Assert.
	assert.NoError(t, err)
	assert.Contains(t, fx.stdout.String(), "Confirm?")
	assert.NotNil(t, fx.imported, "Import should have proceeded")
}

//nolint:paralleltest // State repairing modifies the DisableIntegrityChecking global variable
func TestStateRepair_SkipsConfirmationIfYesFlagIsSet(t *testing.T) {
	// Arrange.
	fx := newStateRepairCmdFixture(t, []*resource.State{
		{URN: "b", Dependencies: []resource.URN{"a"}},
		{URN: "a"},
	})
	fx.cmd.Args.Yes = true

	// Act.
	err := fx.cmd.run(context.Background())

	// Assert.
	assert.NoError(t, err)
	assert.NotContains(t, fx.stdout.String(), "Confirm?")
	assert.NotNil(t, fx.imported, "Import should have proceeded")
}

//nolint:paralleltest // State repairing modifies the DisableIntegrityChecking global variable
func TestStateRepair_DoesNotWriteIfRepairFails(t *testing.T) {
	// Arrange.
	//
	// Dangling provider references can't be fixed, so this snapshot should fail to repair.
	fx := newStateRepairCmdFixture(t, []*resource.State{
		{URN: "a", Provider: "urn:pulumi:stack::project::pulumi:providers:p::x::id"},
	})
	fx.cmd.Args.Yes = true

	// Act.
	err := fx.cmd.run(context.Background())

	// Assert.
	assert.ErrorContains(t, err, "unknown provider")
	assert.Contains(t, fx.stderr.String(), "Failed to repair")
	assert.Nil(t, fx.imported, "Import should not have proceeded")
}

//nolint:paralleltest // State repairing modifies the DisableIntegrityChecking global variable
func TestStateRepair_RepairsSnapshots(t *testing.T) {
	// Arrange.
	fx := newStateRepairCmdFixture(t, []*resource.State{
		{URN: "b", Dependencies: []resource.URN{"a"}},
		{URN: "a"},
	})
	fx.cmd.Args.Yes = true

	// Act.
	err := fx.cmd.run(context.Background())

	// Assert.
	assert.NoError(t, err)
	assert.Contains(t, fx.stdout.String(), "State repaired successfully")
	assert.Equal(t, "a", string(fx.imported.Resources[0].URN))
	assert.Equal(t, "b", string(fx.imported.Resources[1].URN))
}

type stateRepairCmdFixture struct {
	cmd *stateRepairCmd

	stdin  *mockFileReader
	stdout *mockFileWriter
	stderr *bytes.Buffer

	imported *apitype.DeploymentV3
}

func newStateRepairCmdFixture(
	t *testing.T,
	resources []*resource.State,
) *stateRepairCmdFixture {
	fx := &stateRepairCmdFixture{
		stdin:  &mockFileReader{fd: 0},
		stdout: &mockFileWriter{fd: 1},
		stderr: &bytes.Buffer{},
	}

	var s backend.Stack

	b := &backend.MockBackend{
		GetStackF: func(context.Context, backend.StackReference) (backend.Stack, error) {
			return s, nil
		},
		ImportDeploymentF: func(_ context.Context, _ backend.Stack, d *apitype.UntypedDeployment) error {
			err := json.Unmarshal(d.Deployment, &fx.imported)
			assert.NoError(t, err)
			return nil
		},
	}

	s = &backend.MockStack{
		BackendF: func() backend.Backend {
			return b
		},
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			sm := b64.NewBase64SecretsManager()
			return deploy.NewSnapshot(deploy.Manifest{}, sm, resources, nil, deploy.SnapshotMetadata{}), nil
		},
	}

	ws := &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{Backend: &workspace.ProjectBackend{URL: "file://url"}}, "", nil
		},
	}

	lm := &cmdBackend.MockLoginManager{
		CurrentF: func(
			context.Context,
			pkgWorkspace.Context,
			diag.Sink,
			string,
			*workspace.Project,
			bool,
		) (backend.Backend, error) {
			return b, nil
		},
		LoginF: func(
			context.Context,
			pkgWorkspace.Context,
			diag.Sink,
			string,
			*workspace.Project,
			bool,
			colors.Colorization,
		) (backend.Backend, error) {
			return b, nil
		},
	}

	fx.cmd = &stateRepairCmd{
		Args: &stateRepairArgs{
			Stack:     "organization/project/stack",
			Colorizer: colors.Never,
		},

		Stdin:  fx.stdin,
		Stdout: fx.stdout,
		Stderr: fx.stderr,

		Workspace:    ws,
		LoginManager: lm,
	}

	return fx
}

type mockFileReader struct {
	buf bytes.Buffer
	fd  uintptr
}

func (m *mockFileReader) Read(p []byte) (n int, err error) {
	return m.buf.Read(p)
}

func (m *mockFileReader) Fd() uintptr {
	return m.fd
}

type mockFileWriter struct {
	buf bytes.Buffer
	fd  uintptr
}

func (m *mockFileWriter) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockFileWriter) Fd() uintptr {
	return m.fd
}

func (m *mockFileWriter) String() string {
	return m.buf.String()
}
