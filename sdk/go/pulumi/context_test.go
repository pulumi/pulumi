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
	"fmt"
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
func TestLoggingFromApplyCausesNoPanics(t *testing.T) {
	// Usually panics on iteration 100-200
	for i := 0; i < 1000; i++ {
		fmt.Printf("Iteration %d\n", i)
		mocks := &testMonitor{}
		err := RunErr(func(ctx *Context) error {
			String("X").ToStringOutput().ApplyT(func(string) int {
				ctx.Log.Debug("Zzz", &LogArgs{})
				return 0
			})
			return nil
		}, WithMocks("project", "stack", mocks))
		assert.NoError(t, err)
	}
}

type LoggingTestResource struct {
	ResourceState
	TestOutput StringOutput
}

func NewLoggingTestResource(ctx *Context, name string, input StringInput, opts ...ResourceOption) (*LoggingTestResource, error) {
	resource := &LoggingTestResource{}
	err := ctx.RegisterComponentResource("test:go:NewLoggingTestResource", name, resource, opts...)
	if err != nil {
		return nil, err
	}

	resource.TestOutput = input.ToStringOutput().ApplyT(func(inputValue string) (string, error) {
		time.Sleep(10)
		ctx.Log.Debug("Zzz", &LogArgs{})
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

// The following test reproduced a panic but is somewhat contrived. It
// can happen if we queue new work (ApplyT) dynamically (from Apply)
// against a single join group in the Context. If the concurrency
// cards are dealt just right, the program is already executing
// context.wg.Wait() and is about to complete, since wg count is 0,
// when we increment the count via ApplyT. Go surfaces this as a
// panic. Even if the program did not panic, there would be a race
// condition there, since it is unclear whether the new dynamic work
// should have been awaited or not.
//
// Do we care about this case? User programs need to engage goroutines
// and out of band Pulumi work creation, more than just straight
// ApplyT, to get this.
//
// func TestWaitingCausesNoPanics(t *testing.T) {
// 	for i := 0; i < 10; i++ {
// 		mocks := &testMonitor{}
// 		err := RunErr(func(ctx *Context) error {
// 			o, set, _ := ctx.NewOutput()
// 			go func() {
// 				set(1)
// 				o.ApplyT(func(x interface{}) interface{} { return x })
// 			}()
// 			return nil
// 		}, WithMocks("project", "stack", mocks))
// 		assert.NoError(t, err)
// 	}
// }
