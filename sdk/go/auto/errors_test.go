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

package auto

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/python/toolchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentUpdateError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pName := "conflict_error"
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	// The program blocks until this file is removed, letting us hold one update inside the program while a
	// second update races to produce a concurrent update error.
	block := filepath.Join(t.TempDir(), "block")
	require.NoError(t, os.WriteFile(block, nil, 0o600))

	// initialize
	pDir := filepath.Join(".", "test", "errors", "conflict_error")
	s, err := NewStackLocalSource(ctx, stackName, pDir)
	require.NoErrorf(t, err, "failed to initialize stack")

	s.Workspace().SetEnvVar("PULUMI_TEST_BLOCK_FILE", block)

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	c := make(chan error)

	// parallel updates to cause conflict
	for range 2 {
		go func() { _, err := s.Up(ctx); c <- err }()
	}

	// One stack successfully entered the program and is waiting on the block file to be removed. The only way
	// for a stack to return is to error before the program, so we assert that's the concurrent update.
	err = <-c
	assert.Truef(t, IsConcurrentUpdateError(err), "found %s", err)

	// Release the remaining stack to complete, then block until it does.
	require.NoError(t, os.Remove(block))
	assert.Nil(t, <-c)

	// -- pulumi destroy --
	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

func TestInlineConcurrentUpdateError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	pName := "inline_conflict_error"
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, pName, sName)

	block := make(chan struct{})

	// initialize
	s, err := NewStackInlineSource(ctx, stackName, pName, func(ctx *pulumi.Context) error {
		<-block
		ctx.Export("exp_static", pulumi.String("foo"))
		return nil
	})
	require.NoErrorf(t, err, "failed to initialize stack")

	defer func() {
		// -- pulumi stack rm --
		err = s.Workspace().RemoveStack(ctx, s.Name())
		assert.Nil(t, err, "failed to remove stack. Resources have leaked.")
	}()

	c := make(chan error)

	// parallel updates to cause conflict
	for range 2 {
		go func() { _, err := s.Up(ctx); c <- err }()
	}

	// One stack successfully entered the program and is waiting on block to close. The only way for a stack
	// to return is to error before the program, so we assert that's the concurrent update.
	err = <-c
	assert.Truef(t, IsConcurrentUpdateError(err), "found %s", err)

	// Release the remaining stack to complete, then block until it does.
	close(block)
	assert.Nil(t, <-c)

	// -- pulumi destroy --
	_, err = s.Destroy(ctx)
	require.NoError(t, err, "destroy failed")
}

const compilationErrProj = "compilation_error"

func TestCompilationErrorGo(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
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
	assert.True(t, IsCompilationError(err), "%v is not a compilation error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestSelectStack404Error(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
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
	assert.True(t, IsSelectStack404Error(err), "%v is not a 404 error", err)
}

func TestCreateStack409Error(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
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
	assert.True(t, IsCreateStack409Error(err), "%v is not a 409 error", err)
}

func TestCompilationErrorDotnet(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
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
	assert.True(t, IsCompilationError(err), "%v is not a compilation error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestCompilationErrorTypescript(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, compilationErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "compilation_error", "typescript")

	cmd := exec.Command("npm", "install")
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
	assert.True(t, IsCompilationError(err), "%v is not a compilation error", err)

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

	ctx := t.Context()
	sName := ptesting.RandomStackName()
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
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorInlineGo(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
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
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorPython(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir, err := filepath.Abs(filepath.Join(".", "test", "errors", "runtime_error", "python"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	tc, err := toolchain.ResolveToolchain(toolchain.PythonOptions{
		Toolchain:  toolchain.Pip,
		Root:       pDir,
		Virtualenv: "venv",
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	err = tc.InstallDependencies(t.Context(), pDir, false, /*useLanguageVersionTools */
		true /*showOutput*/, os.Stdout, os.Stderr)
	if err != nil {
		t.Errorf("failed to create a venv and install project dependencies: %v", err)
		t.FailNow()
	}

	pySDK, err := filepath.Abs(filepath.Join("..", "..", "..", "sdk", "python"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	// install Pulumi Python SDK from the current source tree, -e means no-copy, ref directly
	pyCmd, err := tc.ModuleCommand(t.Context(), "pip", "install", "-e", pySDK)
	if err != nil {
		t.Errorf("failed to install the local SDK: %v", err)
		t.FailNow()
	}
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
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)
	assert.ErrorContains(t, err, "IndexError: list index out of range")

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorJavascript(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "javascript")

	cmd := exec.Command("npm", "install")
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
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorTypescript(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
	stackName := FullyQualifiedStackName(pulumiOrg, runtimeErrProj, sName)

	// initialize
	pDir := filepath.Join(".", "test", "errors", "runtime_error", "typescript")

	cmd := exec.Command("npm", "install")
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
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

func TestRuntimeErrorDotnet(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	sName := ptesting.RandomStackName()
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
	assert.True(t, IsRuntimeError(err), "%v is not a runtime error", err)

	// -- pulumi destroy --

	_, err = s.Destroy(ctx)
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}
}

// errPredicate pairs one of the exported error predicates with an autoError it is expected to match.
type errPredicate struct {
	name    string
	matches func(error) bool
	err     autoError
}

func errPredicates() []errPredicate {
	cause := errors.New("exit status 255")
	return []errPredicate{
		{
			name:    "IsConcurrentUpdateError",
			matches: IsConcurrentUpdateError,
			err: newAutoError(cause, "",
				"error: [409] Conflict: Another update is currently in progress.", 255),
		},
		{
			name:    "IsSelectStack404Error",
			matches: IsSelectStack404Error,
			err:     newAutoError(cause, "", "error: no stack named 'dev' found", 255),
		},
		{
			name:    "IsCreateStack409Error",
			matches: IsCreateStack409Error,
			err:     newAutoError(cause, "", "error: stack 'dev' already exists", 255),
		},
		{
			name:    "IsCompilationError",
			matches: IsCompilationError,
			err:     newAutoError(cause, "Build FAILED.", "", 255),
		},
		{
			name:    "IsRuntimeError",
			matches: IsRuntimeError,
			err:     newAutoError(cause, "pulumi:pulumi:Stack failed with an unhandled exception:", "", 255),
		},
		{
			name:    "IsUnexpectedEngineError",
			matches: IsUnexpectedEngineError,
			err: newAutoError(cause,
				"The Pulumi CLI encountered a fatal error. This is a bug!", "", 255),
		},
	}
}

func TestErrorPredicatesMatchUnwrapped(t *testing.T) {
	t.Parallel()

	for _, p := range errPredicates() {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, p.matches(p.err), "%s did not match its own autoError", p.name)
		})
	}
}

// Wrapping an error with fmt.Errorf("...: %w", err) is the standard Go idiom, and autoError is
// unexported so callers cannot reach it with errors.As themselves. The predicates must therefore
// look through wrapping.
func TestErrorPredicatesMatchWrapped(t *testing.T) {
	t.Parallel()

	for _, p := range errPredicates() {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()

			wrapped := fmt.Errorf("stack operation failed: %w", p.err)
			assert.True(t, p.matches(wrapped), "%s did not match a singly wrapped autoError", p.name)

			doubleWrapped := fmt.Errorf("deploy: %w", wrapped)
			assert.True(t, p.matches(doubleWrapped), "%s did not match a doubly wrapped autoError", p.name)
		})
	}
}

// autoError must unwrap to the error it was created from so that errors.Is and errors.As can reach
// the underlying cause.
func TestAutoErrorUnwrapsCause(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	var ae error = newAutoError(sentinel, "", "", 255)

	assert.ErrorIs(t, ae, sentinel)
	assert.ErrorIs(t, fmt.Errorf("stack operation failed: %w", ae), sentinel)
}

type causeError struct{ msg string }

func (e *causeError) Error() string { return e.msg }

func TestAutoErrorAsCause(t *testing.T) {
	t.Parallel()

	cause := &causeError{msg: "boom"}
	var ae error = newAutoError(cause, "", "", 255)

	var target *causeError
	require.ErrorAs(t, ae, &target)
	assert.Equal(t, cause, target)

	target = nil
	require.ErrorAs(t, fmt.Errorf("stack operation failed: %w", ae), &target)
	assert.Equal(t, cause, target)
}
