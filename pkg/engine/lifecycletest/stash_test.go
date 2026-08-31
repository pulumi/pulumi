// Copyright 2025, Pulumi Corporation.
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

package lifecycletest

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

func TestStashImportError(t *testing.T) {
	t.Parallel()

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _ = monitor.RegisterResource("pulumi:index:Stash", "stash", true, deploytest.ResourceOptions{
			ImportID: "someid",
		})
		// The resource registration fails, and the engine knows this and
		// cancels the deployment. RegisterResource will not return.
		t.Fatalf("We should not return from RegisterResource")
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.True(t, result.IsBail(err))
	require.ErrorContains(t, err, "stash can not be imported")
}

func TestStash(t *testing.T) {
	t.Parallel()

	input := resource.NewProperty("first")
	expectedOutput := input
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:index:Stash", "stash", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"input": input,
			},
		})
		require.NoError(t, err)
		require.Equal(t, input, resp.Outputs["input"])
		require.Equal(t, expectedOutput, resp.Outputs["output"])

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	input = resource.NewProperty("second")
	_, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

// andReducer registers a boolean "oldOutput && newInput" reducer on the given callback server
// and returns the callback to smuggle into a Stash resource's inputs. invocations is incremented
// each time the reducer callback is actually invoked.
func andReducer(t *testing.T, callbacks *deploytest.CallbackServer, invocations *int32) *pulumirpc.Callback {
	cb, err := callbacks.Allocate(func(reqBytes []byte) (proto.Message, error) {
		var req pulumirpc.StashReduceRequest
		if err := proto.Unmarshal(reqBytes, &req); err != nil {
			return nil, err
		}
		atomic.AddInt32(invocations, 1)
		// old_input is available too but this reducer doesn't need it.
		old := req.OldOutput.GetBoolValue()
		cur := req.NewInput.GetBoolValue()
		return &pulumirpc.StashReduceResponse{
			Reduced: structpb.NewBoolValue(old && cur),
		}, nil
	})
	require.NoError(t, err)
	return cb
}

func stashInputs(cb *pulumirpc.Callback, input resource.PropertyValue) resource.PropertyMap {
	return resource.PropertyMap{
		"input": input,
		"reducer": resource.NewProperty(resource.PropertyMap{
			"target": resource.NewProperty(cb.Target),
			"token":  resource.NewProperty(cb.Token),
		}),
	}
}

// runStashReducerScenario runs two sequential updates against a Stash resource with an `AND` reducer,
// returning the resulting snapshots and how many times the reducer callback was invoked overall.
func runStashReducerScenario(
	t *testing.T, first, second bool,
) (firstSnap, secondSnap *deploySnapshot, reducerCalls int32) {
	var invocations int32
	inputVal := resource.NewProperty(first)
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		cb := andReducer(t, callbacks, &invocations)

		_, err = monitor.RegisterResource("pulumi:index:Stash", "stash", true, deploytest.ResourceOptions{
			Inputs: stashInputs(cb, inputVal),
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}

	firstSnap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	inputVal = resource.NewProperty(second)
	secondSnap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, firstSnap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	return firstSnap, secondSnap, atomic.LoadInt32(&invocations)
}

// deploySnapshot is a small alias to keep the helper signature readable.
type deploySnapshot = deploy.Snapshot

// stashResourceState returns the Stash resource from the snapshot.
func stashResourceState(t *testing.T, snap *deploySnapshot) *pkgresource.State {
	t.Helper()
	for _, r := range snap.Resources {
		if r.Type == "pulumi:index:Stash" {
			return r
		}
	}
	t.Fatalf("no stash resource in snapshot")
	return nil
}

func assertStashState(t *testing.T, r *pkgresource.State, wantInput, wantOutput resource.PropertyValue) {
	t.Helper()
	require.True(t, wantInput.DeepEquals(r.Inputs["input"]), "input: want %v got %v", wantInput, r.Inputs["input"])
	require.True(t, wantOutput.DeepEquals(r.Outputs["output"]), "output: want %v got %v", wantOutput, r.Outputs["output"])
	// The reducer callback and __reduced must never appear in persisted state.
	_, hasReducerIn := r.Inputs["reducer"]
	require.False(t, hasReducerIn, "reducer must not persist in inputs")
	_, hasReducerOut := r.Outputs["reducer"]
	require.False(t, hasReducerOut, "reducer must not persist in outputs")
	_, hasReducedOut := r.Outputs["__reduced"]
	require.False(t, hasReducedOut, "__reduced must not surface as an output")
}

func TestStashReducer_CreateSkipsReducer(t *testing.T) {
	t.Parallel()

	trueV := resource.NewProperty(true)
	firstSnap, _, calls := runStashReducerScenario(t, true, true)
	// On the very first run the reducer is not invoked, and on the true/true update the reducer
	// runs once producing true && true == true (no change).
	require.Equal(t, int32(1), calls)
	assertStashState(t, stashResourceState(t, firstSnap), trueV, trueV)
}

func TestStashReducer_TrueToFalse(t *testing.T) {
	t.Parallel()

	_, secondSnap, calls := runStashReducerScenario(t, true, false)
	require.Equal(t, int32(1), calls, "reducer invoked once on update")
	assertStashState(t, stashResourceState(t, secondSnap),
		resource.NewProperty(false), resource.NewProperty(false))
}

func TestStashReducer_FalseToTrue(t *testing.T) {
	t.Parallel()

	// Motivating case: once output is false, a subsequent true input must not flip it back.
	_, secondSnap, calls := runStashReducerScenario(t, false, true)
	require.Equal(t, int32(1), calls)
	assertStashState(t, stashResourceState(t, secondSnap),
		resource.NewProperty(true), resource.NewProperty(false))
}

func TestStashReducer_TrueToTrue(t *testing.T) {
	t.Parallel()

	firstSnap, secondSnap, calls := runStashReducerScenario(t, true, true)
	// The reducer runs once on the second update (create doesn't invoke it), true && true == true,
	// so the snapshot output remains true and there is no meaningful state change.
	require.Equal(t, int32(1), calls)
	trueV := resource.NewProperty(true)
	assertStashState(t, stashResourceState(t, firstSnap), trueV, trueV)
	assertStashState(t, stashResourceState(t, secondSnap), trueV, trueV)
}

func TestStashReducer_FalseToFalse(t *testing.T) {
	t.Parallel()

	firstSnap, secondSnap, calls := runStashReducerScenario(t, false, false)
	require.Equal(t, int32(1), calls)
	falseV := resource.NewProperty(false)
	assertStashState(t, stashResourceState(t, firstSnap), falseV, falseV)
	assertStashState(t, stashResourceState(t, secondSnap), falseV, falseV)
}

// TestStashReducer_Preview verifies that in preview mode with an unknown input the reducer is
// not invoked (we can't run user code against an unknown), but the resource is still handled.
func TestStashReducer_Preview(t *testing.T) {
	t.Parallel()

	var invocations int32
	inputVal := resource.MakeComputed(resource.NewProperty(false))
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		cb := andReducer(t, callbacks, &invocations)
		_, err = monitor.RegisterResource("pulumi:index:Stash", "stash", true, deploytest.ResourceOptions{
			Inputs: stashInputs(cb, inputVal),
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	// Preview: DryRun=true.
	p.Options.HostF = hostF
	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Equal(t, int32(0), atomic.LoadInt32(&invocations),
		"reducer must not be invoked when input is unknown")
}

// TestStashReducer_ReducerArgs verifies that the reducer callback receives old_input, old_output
// and new_input as distinct values, so a reducer can distinguish "input just changed" from
// "output changed via reduction".
func TestStashReducer_ReducerArgs(t *testing.T) {
	t.Parallel()

	// Track the arguments the reducer sees across invocations.
	type args struct{ oldInput, oldOutput, newInput string }
	var seen []args
	var seenLock sync.Mutex

	inputVal := resource.NewProperty("first")
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		cb, err := callbacks.Allocate(func(reqBytes []byte) (proto.Message, error) {
			var req pulumirpc.StashReduceRequest
			if err := proto.Unmarshal(reqBytes, &req); err != nil {
				return nil, err
			}
			seenLock.Lock()
			seen = append(seen, args{
				oldInput:  req.OldInput.GetStringValue(),
				oldOutput: req.OldOutput.GetStringValue(),
				newInput:  req.NewInput.GetStringValue(),
			})
			seenLock.Unlock()
			// Reducer concatenates old_output and new_input to prove it can use both.
			return &pulumirpc.StashReduceResponse{
				Reduced: structpb.NewStringValue(req.OldOutput.GetStringValue() + "+" + req.NewInput.GetStringValue()),
			}, nil
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:index:Stash", "stash", true, deploytest.ResourceOptions{
			Inputs: stashInputs(cb, inputVal),
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// After the first run: no reducer invocation (create), output == input == "first".
	require.Empty(t, seen, "reducer must not be invoked on create")
	assertStashState(t, stashResourceState(t, snap),
		resource.NewProperty("first"), resource.NewProperty("first"))

	// Second run: input becomes "second". Reducer should see old_input="first",
	// old_output="first", new_input="second", and produce "first+second".
	inputVal = resource.NewProperty("second")
	snap, err = lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	seenLock.Lock()
	defer seenLock.Unlock()
	require.Len(t, seen, 1)
	require.Equal(t, args{oldInput: "first", oldOutput: "first", newInput: "second"}, seen[0])
	assertStashState(t, stashResourceState(t, snap),
		resource.NewProperty("second"), resource.NewProperty("first+second"))
}
