// Copyright 2016-2020, Pulumi Corporation.
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

package auto

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentUpdateError(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#8122] - investigate underlying sporadic 404 error
	t.Skip("disabled as flaky and resource-intensive")

	n := 50
	ctx := context.Background()
	pName := "conflict_error"
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "conflict_error")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	c := make(chan error)

	// parallel updates to cause conflict
	for i := 0; i < n; i++ {
		go func() {
			_, err := s.Up(ctx)
			c <- err
		}()
	}

	conflicts := 0
	var otherErrors []error

	for i := 0; i < n; i++ {
		err := <-c
		if err != nil {
			if IsConcurrentUpdateError(err) {
				conflicts++
			} else {
				otherErrors = append(otherErrors, err)
			}
		}
	}

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")

	if len(otherErrors) > 0 {
		t.Logf("Concurrent updates incurred %d non-conflict errors, including:", len(otherErrors))
		for _, err := range otherErrors {
			t.Error(err)
		}
	}

	// should have at least one conflict
	assert.Greater(t, conflicts, 0)
}

func TestInlineConcurrentUpdateError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pName := "inline_conflict_error"
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		time.Sleep(5 * time.Second)
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	c := make(chan error)

	// parallel updates to cause conflict
	for i := 0; i < 50; i++ {
		go func() {
			_, err := s.Up(ctx)
			c <- err
		}()
	}

	conflicts := 0

	for i := 0; i < 50; i++ {
		err := <-c
		if IsConcurrentUpdateError(err) {
			conflicts++
		}
	}

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")

	// should have at least one conflict
	assert.Greater(t, conflicts, 0)
}

const compilationErrProj = "compilation_error"

func TestCompilationErrorGo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, compilationErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "compilation_error", "go")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.True(t, IsCompilationError(err), "%v is not a compilation error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

func TestSelectStack404Error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, "testproj", sName)

	// initialize
	pDir := filepath.Join(".", "test", "testproj")
	opts := []LocalWorkspaceOption{WorkDir(pDir)}
	w, err := NewLocalWorkspace(ctx, opts...)
	require.NoError(t, err, "failed to initialize workspace")

	// attempt to select stack that has not been created.
	_, err = SelectStack(ctx, stackName, w)
	assert.True(t, IsSelectStack404Error(err), "%v is not a 404 error", err)
}

func TestCreateStack409Error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, "testproj", sName)

	// initialize first stack
	pDir := filepath.Join(".", "test", "testproj")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	// initialize workspace for dupe stack
	opts := []LocalWorkspaceOption{WorkDir(pDir)}
	w, err := NewLocalWorkspace(ctx, opts...)
	require.NoError(t, err, "failed to initialize workspace")

	// attempt to create a dupe stack.
	_, err = NewStack(ctx, stackName, w)
	assert.True(t, IsCreateStack409Error(err), "%v is not a 409 error", err)
}

func TestCompilationErrorDotnet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, compilationErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "compilation_error", "dotnet")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.True(t, IsCompilationError(err), "%v is not a compilation error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

func TestCompilationErrorTypescript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, compilationErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "compilation_error", "typescript")

	cmd := exec.Command("bun", "install")
	cmd.Dir = pDir
	err := cmd.Run()
	require.NoError(t, err, "failed to install project dependencies")

	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.True(t, IsCompilationError(err), "%v is not a compilation error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

const runtimeErrProj = "runtime_error"

func TestRuntimeErrorGo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "go")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

func TestRuntimeErrorInlineGo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, runtimeErrProj, func(ctx *pulumi.Context) error {
		panic("great sadness")
	})
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

func TestRuntimeErrorPython(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir, err := filepath.Abs(filepath.Join(".", "test", "errors", "runtime_error", "python"))
	require.NoError(t, err)

	tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
		Toolchain:  toolchain.Pip,
		Root:       pDir,
		Virtualenv: "venv",
	})
	require.NoError(t, err)
	err = tc.InstallDependencies(context.Background(), pDir, false, /*useLanguageVersionTools */
		true /*showOutput*/, os.Stdout, os.Stderr)
	require.NoError(t, err, "failed to install project dependencies")

	pySDK, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk", "python"))
	require.NoError(t, err)

	// install Pulumi Python SDK from the current source tree, -e means no-copy, ref directly
	pyCmd, err := tc.ModuleCommand(context.Background(), "pip", "install", "-e", pySDK)
	require.NoError(t, err, "failed to install the local SDK")

	pyCmd.Dir = pDir
	err = pyCmd.Run()
	require.NoError(t, err, "failed to link venv against in-source pulumi")

	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)
	assert.ErrorContains(t, err, "IndexError: list index out of range")

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

func TestRuntimeErrorJavascript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "javascript")

	cmd := exec.Command("bun", "install")
	cmd.Dir = pDir
	err := cmd.Run()
	require.NoError(t, err, "failed to install project dependencies")

	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")
	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

func TestRuntimeErrorTypescript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "typescript")

	cmd := exec.Command("bun", "install")
	cmd.Dir = pDir
	err := cmd.Run()
	require.NoError(t, err, "failed to install project dependencies")

	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

func TestRuntimeErrorDotnet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "dotnet")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoError(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		require.NoError(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}
