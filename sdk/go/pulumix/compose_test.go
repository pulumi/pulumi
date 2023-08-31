// Copyright 2016-2023, Pulumi Corporation.
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

package pulumix_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompose_success(t *testing.T) {
	t.Parallel()

	aout := pulumix.Val("1")
	bout := pulumix.Val("2")
	ctx := context.Background()

	result := pulumix.Compose(ctx, func(c *pulumix.C) (int, error) {
		a, err := strconv.Atoi(pulumix.CAwait(c, aout))
		assert.NoError(t, err)

		b, err := strconv.Atoi(pulumix.CAwait(c, bout))
		assert.NoError(t, err)

		return a + b, nil
	})

	v, known, secret, deps, err := pulumix.UnsafeAwait(ctx, result)
	require.NoError(t, err)
	assert.Equal(t, 3, v)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Empty(t, deps)
}

func TestCompose_returnError(t *testing.T) {
	t.Parallel()

	aout := pulumix.Val("1")
	bout := pulumix.Val("not a number")

	ctx := context.Background()

	result := pulumix.Compose(ctx, func(c *pulumix.C) (int, error) {
		_, err := strconv.Atoi(pulumix.CAwait(c, aout))
		assert.NoError(t, err)

		_, err = strconv.Atoi(pulumix.CAwait(c, bout))
		assert.Error(t, err)

		return 0, err
	})

	_, _, _, _, err := pulumix.UnsafeAwait(ctx, result)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid syntax")
}

func TestCompose_failedChildOperation(t *testing.T) {
	t.Parallel()

	foo := pulumix.Val("foo")
	bar := pulumix.Output[string]{
		OutputState: internal.NewOutputState(nil, reflect.TypeOf("")),
	}
	giveErr := errors.New("great sadness")
	internal.RejectOutput(bar, giveErr)

	ctx := context.Background()
	result := pulumix.Compose(ctx, func(c *pulumix.C) (int, error) {
		foo := pulumix.CAwait(c, foo)
		bar := pulumix.CAwait(c, bar)
		t.Errorf("Should not reach here, got: (%v, %v)", foo, bar)
		return 0, nil // appease the compiler
	})

	_, _, _, _, err := pulumix.UnsafeAwait(ctx, result)
	assert.Error(t, err)
	assert.ErrorIs(t, err, giveErr)
}

func TestCompose_secret(t *testing.T) {
	t.Parallel()

	type User struct {
		Username string
		Password string
	}

	username := pulumix.Val("admin")
	password := pulumi.ToSecret(pulumi.String("hunter2")).(pulumi.StringOutput)

	ctx := context.Background()
	result := pulumix.Compose(ctx, func(c *pulumix.C) (*User, error) {
		return &User{
			Username: pulumix.CAwait(c, username),
			Password: pulumix.CAwait(c, password),
		}, nil
	})

	got, known, secret, deps, err := pulumix.UnsafeAwait(ctx, result)
	require.NoError(t, err)
	assert.Equal(t, &User{Username: "admin", Password: "hunter2"}, got)
	assert.True(t, known)
	assert.True(t, secret)
	assert.Empty(t, deps)
}

func TestCompose_unknown(t *testing.T) {
	t.Parallel()

	foo := pulumix.Val("foo")
	bar := pulumix.Output[string]{
		OutputState: internal.NewOutputState(nil, reflect.TypeOf("")),
	}
	internal.FulfillOutput(bar, nil, false /* known */, false, nil, nil)

	ctx := context.Background()
	result := pulumix.Compose(ctx, func(c *pulumix.C) (int, error) {
		fooLen := len(pulumix.CAwait(c, foo))
		barLen := len(pulumix.CAwait(c, bar))
		t.Errorf("Should not reach here, got: (%v, %v)", fooLen, barLen)
		return 0, nil // appease the compiler
	})

	_, known, _, _, err := pulumix.UnsafeAwait(ctx, result)
	require.NoError(t, err)
	assert.False(t, known)
}

func TestCompsoe_dependencies(t *testing.T) {
	t.Parallel()

	type Dependency struct {
		internal.ResourceState

		id int
	}

	dep1 := Dependency{id: 1}
	dep2 := Dependency{id: 2}
	dep3 := Dependency{id: 3}

	a := pulumix.Output[int]{
		OutputState: internal.NewOutputState(nil, reflect.TypeOf(0)),
	}
	internal.FulfillOutput(a, 1, true, false, []internal.Resource{dep1, dep2}, nil)

	b := pulumix.Output[int]{
		OutputState: internal.NewOutputState(nil, reflect.TypeOf(0)),
	}
	internal.FulfillOutput(b, 2, true, false, []internal.Resource{dep2, dep3}, nil)

	ctx := context.Background()
	result := pulumix.Compose(ctx, func(c *pulumix.C) (int, error) {
		a := pulumix.CAwait(c, a)
		b := pulumix.CAwait(c, b)
		return a + b, nil
	})

	v, _, _, deps, err := pulumix.UnsafeAwait(ctx, result)
	require.NoError(t, err)
	assert.Equal(t, 3, v)

	assert.ElementsMatch(t, []internal.Resource{dep1, dep2, dep3}, deps)
}

func TestCompose_panic(t *testing.T) {
	// Uncaptured panics in another goroutine terminate the program.
	// To test this, we need to run the pulumix.Compose call
	// in another process so as not to terminate the test process.

	if os.Getenv("INSIDE_COMPOSE_TEST") == "1" {
		// We're inside the subprocess. Run the invalid pulumix.Compose call.
		ctx := context.Background()
		o := pulumix.Compose(ctx, func(c *pulumix.C) (int, error) {
			panic("great sadness")
		})

		// Block until the goroutine attempts to fulfill the output,
		// which should never happen because the pulumix.Compose call panics.
		_, _, _, _, _ = pulumix.UnsafeAwait(ctx, o)
		os.Exit(1) // unreachable
	}

	t.Parallel()

	exe, err := os.Executable()
	require.NoError(t, err)

	cmd := exec.Command(exe, "-test.run=^"+t.Name()+"$")
	cmd.Env = []string{"INSIDE_COMPOSE_TEST=1"}
	bs, err := cmd.CombinedOutput()
	require.Error(t, err)

	out := string(bs)
	assert.Contains(t, out, "panic: great sadness")
	assert.Contains(t, out, "TestCompose_panic.func")
	assert.Contains(t, out, "compose_test.go")
}

func TestCompose_goexit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	result := pulumix.Compose(ctx, func(c *pulumix.C) (int, error) {
		runtime.Goexit()
		t.Errorf("Should not reach here")
		return 0, nil // appease the compiler
	})

	_, _, _, _, err := pulumix.UnsafeAwait(ctx, result)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "goroutine exited before returning")
}

func TestCompose_retainReference(t *testing.T) {
	t.Parallel()

	var c *pulumix.C

	ctx := context.Background()
	result := pulumix.Compose(ctx, func(c2 *pulumix.C) (int, error) {
		c = c2
		return 0, nil
	})

	_, _, _, _, err := pulumix.UnsafeAwait(ctx, result)
	require.NoError(t, err) // await to ensure the goroutine has finished

	// Awaiting on an output with the illegal composer should panic.
	assert.Panics(t, func() {
		pulumix.CAwait(c, pulumix.Val(42))
	})
}

func TestCompose_contextSideEffect(t *testing.T) {
	t.Parallel()

	// If we are inside a pulumi.Run and use its context,
	// and the composition function has side effects,
	// then we should block the Run function from returning
	// until the composition function has finished.

	insideCompose := make(chan struct{})
	doSideEffect := make(chan struct{})
	var sideEffectDone bool

	go func() {
		// Wait until we're inside the composition function,
		// delay just a little and then allow the side effect to happen.
		// then do the side effect.
		<-insideCompose
		time.Sleep(50 * time.Millisecond)
		close(doSideEffect)
	}()

	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		pulumix.Compose(ctx.Context(), func(c *pulumix.C) (int, error) {
			close(insideCompose)

			// Don't return until we're told to.
			<-doSideEffect
			sideEffectDone = true

			return 0, nil
		})
		return nil
	}, pulumi.WithMocks("project", "stack", &mockResourceMonitor{}))
	require.NoError(t, err)

	assert.True(t, sideEffectDone,
		"RunErr returned before composition function finished")
}
