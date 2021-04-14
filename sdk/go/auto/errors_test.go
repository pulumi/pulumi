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
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

func TestConcurrentUpdateError(t *testing.T) {
	t.Skip("disabled, see https://github.com/pulumi/pulumi/issues/5312")
	ctx := context.Background()
	pName := "conflict_error"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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

func TestInlineConcurrentUpdateError(t *testing.T) {
	t.Skip("disabled, see https://github.com/pulumi/pulumi/issues/5312")
	ctx := context.Background()
	pName := "inline_conflict_error"
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	assert.NotNil(t, err)
	assert.True(t, IsCompilationError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestSelectStack404Error(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	assert.NotNil(t, err)
	assert.True(t, IsSelectStack404Error(err))
}

func TestCreateStack409Error(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	assert.NotNil(t, err)
	assert.True(t, IsCreateStack409Error(err))
}

func TestCompilationErrorDotnet(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	assert.NotNil(t, err)
	assert.True(t, IsCompilationError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestCompilationErrorTypescript(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	assert.NotNil(t, err)
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
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorInlineGo(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, runtimeErrProj, func(ctx *pulumi.Context) error {
		var x []string
		ctx.Export("a", pulumi.String(x[0]))
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

	_, err = s.Up(ctx)
	assert.NotNil(t, err)
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
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "python")

	cmd := exec.Command("python3", "-m", "venv", "venv")
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
	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err), "%+v", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorJavascript(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorTypescript(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorDotnet(t *testing.T) {
	ctx := context.Background()
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
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
	assert.NotNil(t, err)
	assert.True(t, IsRuntimeError(err))

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}
