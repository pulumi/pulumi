// Copyright 2016-2021, Pulumi Corporation.
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

package pulumi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The test is extracted from a panic using pulumi-docker and minified
// while still reproducing the panic. The issue is that the resource
// constructor `NewImage` processes `StringInput` and logs into a
// captured `*pulumi.Context` from `ApplyT`. The user program passes a
// vanilla `String`. The `ApplyT` is not tracked against the context
// join group, but the logging is, which causes the logging statement
// to appear as "dynamic" work that appeared unexpectedly after "all
// work was done", and race with the program completion `Wait`.
//
// The test is was made to pass by using a custom-made `workGroup`.
func TestLoggingFromApplyCausesNoPanics(t *testing.T) {
	// Usually panics on iteration 100-200
	for i := 0; i < 1000; i++ {
		t.Logf("Iteration %d\n", i)
		mocks := &testMonitor{}
		err := RunErr(func(ctx *Context) error {
			String("X").ToStringOutput().ApplyT(func(string) int {
				err := ctx.Log.Debug("Zzz", &LogArgs{})
				assert.NoError(t, err)
				return 0
			})
			return nil
		}, WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
	}
}

// An extended version of `TestLoggingFromApplyCausesNoPanics`, more
// realistically demonstrating the original usage pattern.
func TestLoggingFromResourceApplyCausesNoPanics(t *testing.T) {
	// Usually panics on iteration 100-200
	for i := 0; i < 1000; i++ {
		t.Logf("Iteration %d\n", i)
		mocks := &testMonitor{}
		err := RunErr(func(ctx *Context) error {
			_, err := NewLoggingTestResource(t, ctx, "res", String("A"))
			assert.NoError(t, err)
			return nil
		}, WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
	}
}

type LoggingTestResource struct {
	ResourceState
	TestOutput StringOutput
}

func NewLoggingTestResource(
	t *testing.T,
	ctx *Context,
	name string,
	input StringInput,
	opts ...ResourceOption) (*LoggingTestResource, error) {

	resource := &LoggingTestResource{}
	err := ctx.RegisterComponentResource("test:go:NewLoggingTestResource", name, resource, opts...)
	if err != nil {
		return nil, err
	}

	resource.TestOutput = input.ToStringOutput().ApplyT(func(inputValue string) (string, error) {
		time.Sleep(10)
		err := ctx.Log.Debug("Zzz", &LogArgs{})
		assert.NoError(t, err)
		return inputValue, nil
	}).(StringOutput)

	outputs := Map(map[string]Input{
		"testOutput": resource.TestOutput,
	})

	err = ctx.RegisterResourceOutputs(resource, outputs)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

// A contrived test demonstrating queueing work dynamically (`ApplyT`
// called from a separate goroutine). This used to cause a panic but
// is now resolved by using `workGroup`.
func TestWaitingCausesNoPanics(t *testing.T) {
	for i := 0; i < 10; i++ {
		mocks := &testMonitor{}
		err := RunErr(func(ctx *Context) error {
			o, set, _ := ctx.NewOutput()
			go func() {
				set(1)
				o.ApplyT(func(x interface{}) interface{} { return x })
			}()
			return nil
		}, WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
	}
}
