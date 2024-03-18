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
//go:build !xplatform_acceptance

package auto

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/python"
	"github.com/stretchr/testify/assert"
)

func TestConcurrentUpdateError(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#8122] - investigate underlying sporadic 404 error
	t.Skip("disabled as flaky and resource-intensive")

	n := 50
	ctx := context.Background()
	pName := "conflict_error"
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "conflict_error")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
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
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	if len(otherErrors) > 0 {
		t.Logf("Concurrent updates incurred %d non-conflict errors, including:", len(otherErrors))
		for _, err := range otherErrors {
			t.Error(err)
		}
	}

	if conflicts == 0 {
		t.Errorf("Expected at least one conflict error from the %d concurrent updates, but got none", n)
	}
}

func TestInlineConcurrentUpdateError(t *testing.T) {
	t.Parallel()

	t.Skip("disabled, see https://github.com/pulumi/pulumi/issues/5312")
	ctx := context.Background()
	pName := "inline_conflict_error"
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		time.Sleep(1 * time.Second)
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
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
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	// should have at least one conflict
	assert.Greater(t, conflicts, 0)
}

const compilationErrProj = "compilation_error"

func TestCompilationErrorGo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, compilationErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "compilation_error", "go")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.Error(t, err)
	assert.True(t, IsCompilationError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestSelectStack404Error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, "testproj", sName)

	// initialize
	pDir := filepath.Join(".", "test", "testproj")
	opts := []LocalWorkspaceOption{WorkDir(pDir)}
	w, err := NewLocalWorkspace(ctx, opts...)
	if err != nil {
		t.Errorf("failed to initialize workspace, err: %v", err)
		t.FailNow()
	}

	// attempt to select stack that has not been created.
	_, err = SelectStack(ctx, stackName, w)
	assert.Error(t, err)
	assert.True(t, IsSelectStack404Error(err))
}

func TestCreateStack409Error(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, "testproj", sName)

	// initialize first stack
	pDir := filepath.Join(".", "test", "testproj")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	// initialize workspace for dupe stack
	opts := []LocalWorkspaceOption{WorkDir(pDir)}
	w, err := NewLocalWorkspace(ctx, opts...)
	if err != nil {
		t.Errorf("failed to initialize workspace, err: %v", err)
		t.FailNow()
	}

	// attempt to create a dupe stack.
	_, err = NewStack(ctx, stackName, w)
	assert.Error(t, err)
	assert.True(t, IsCreateStack409Error(err))
}

func TestCompilationErrorDotnet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, compilationErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "compilation_error", "dotnet")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.Error(t, err)
	assert.True(t, IsCompilationError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestCompilationErrorTypescript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, compilationErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "compilation_error", "typescript")

	cmd := exec.Command("yarn", "install")
	cmd.Dir = pDir
	err := cmd.Run()
	if err != nil {
		t.Errorf("failed to install project dependencies")
		t.FailNow()
	}

	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.Error(t, err)
	assert.True(t, IsCompilationError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

const runtimeErrProj = "runtime_error"

func TestRuntimeErrorGo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "go")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.Error(t, err)
	assert.True(t, IsRuntimeError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorInlineGo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, runtimeErrProj, func(ctx *pulumi.Context) error {
		panic("great sadness")
	})
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.Error(t, err)
	if !assert.True(t, IsRuntimeError(err)) {
		t.Logf("%v is not a runtime error", err)
	}

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorPython(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir, err := filepath.Abs(filepath.Join(".", "test", "errors", "runtime_error", "python"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	err = python.InstallDependencies(context.Background(), pDir, "venv", true /*showOutput*/)
	if err != nil {
		t.Errorf("failed to create a venv and install project dependencies: %v", err)
		t.FailNow()
	}

	pySDK, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk", "python", "env", "src"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	// install Pulumi Python SDK from the current source tree, -e means no-copy, ref directly
	pyCmd := python.VirtualEnvCommand(filepath.Join(pDir, "venv"), "python", "-m", "pip", "install", "-e", pySDK)
	pyCmd.Dir = pDir
	err = pyCmd.Run()
	if err != nil {
		t.Errorf("failed to link venv against in-source pulumi: %v", err)
		t.FailNow()
	}

	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.Error(t, err)
	assert.True(t, IsRuntimeError(err), "%+v", err)
	assert.Contains(t, fmt.Sprintf("%v", err), "IndexError: list index out of range")

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorJavascript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "javascript")

	cmd := exec.Command("yarn", "install")
	cmd.Dir = pDir
	err := cmd.Run()
	if err != nil {
		t.Errorf("failed to install project dependencies")
		t.FailNow()
	}

	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.Error(t, err)
	assert.True(t, IsRuntimeError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorTypescript(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "typescript")

	cmd := exec.Command("yarn", "install")
	cmd.Dir = pDir
	err := cmd.Run()
	if err != nil {
		t.Errorf("failed to install project dependencies")
		t.FailNow()
	}

	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.Error(t, err)
	assert.True(t, IsRuntimeError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorDotnet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sName := randomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "dotnet")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	_, err = s.Up(ctx)
	assert.Error(t, err)
	assert.True(t, IsRuntimeError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}
