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

package lifecycletest

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// The tests in this file run updates against a fakeCloud, and check that the engine never deletes a resource that
// another still references, and that the snapshot it saves tracks the cloud's resources.

var (
	diffNone                = plugin.DiffResult{Changes: plugin.DiffNone}
	diffUpdate              = plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"v"}}
	diffReplace             = plugin.DiffResult{Changes: plugin.DiffSome, ReplaceKeys: []resource.PropertyKey{"v"}}
	diffDeleteBeforeReplace = plugin.DiffResult{
		Changes: plugin.DiffSome, ReplaceKeys: []resource.PropertyKey{"v"}, DeleteBeforeReplace: true,
	}
)

// fakeCloud is a provider backed by an in-memory cloud. A resource's ID is its name and a generation, e.g. "a1", and
// an input whose value is another resource's ID references that resource.
type fakeCloud struct {
	// diffs is what Diff returns for a resource, by name, when its inputs have changed. The default is diffReplace.
	diffs map[string]plugin.DiffResult
	// deletedWith maps a resource's name to the name of a resource it references and is deleted with in the cloud.
	deletedWith map[string]string
	// fail lists the provider calls that fail, e.g. "update a1".
	fail map[string]bool
	// waits makes an event wait for another, to force an interleaving of parallel steps. An event is a provider call,
	// e.g. "delete a1", or one a test program records with happened. The wait times out after a second, in case the
	// engine orders the events the other way.
	waits map[string]string

	mu         sync.Mutex
	live       map[resource.ID]liveResource
	created    map[string]int
	events     map[string]chan struct{}
	violations []string
}

type liveResource struct {
	name   string
	inputs resource.PropertyMap
}

func newFakeCloud() *fakeCloud {
	return &fakeCloud{
		live:    map[resource.ID]liveResource{},
		created: map[string]int{},
		events:  map[string]chan struct{}{},
	}
}

// call records a provider call, and returns an error if it should fail.
func (c *fakeCloud) call(call string) error {
	c.happened(call)
	if c.fail[call] {
		return fmt.Errorf("%s failed", call)
	}
	return nil
}

// happened records an event, after waiting for any event it must follow.
func (c *fakeCloud) happened(event string) {
	if before, ok := c.waits[event]; ok {
		c.waitFor(before)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch := c.eventChan(event); !isClosed(ch) {
		close(ch)
	}
}

// waitFor blocks until the given event has happened, or a second has passed.
func (c *fakeCloud) waitFor(event string) {
	c.mu.Lock()
	ch := c.eventChan(event)
	c.mu.Unlock()
	select {
	case <-ch:
	case <-time.After(time.Second):
	}
}

func (c *fakeCloud) eventChan(event string) chan struct{} {
	if _, ok := c.events[event]; !ok {
		c.events[event] = make(chan struct{})
	}
	return c.events[event]
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// remove deletes a resource, and everything deleted with it, recording a violation if anything still references it.
func (c *fakeCloud) remove(id resource.ID) {
	r, ok := c.live[id]
	if !ok {
		c.violations = append(c.violations, fmt.Sprintf("deleted %s, which doesn't exist", id))
		return
	}
	delete(c.live, id)
	for _, other := range slices.Sorted(maps.Keys(c.live)) {
		o, ok := c.live[other]
		if !ok || !slices.ContainsFunc(slices.Collect(maps.Values(o.inputs)), func(v resource.PropertyValue) bool {
			return v.IsString() && v.StringValue() == string(id)
		}) {
			continue
		}
		if c.deletedWith[o.name] == r.name {
			c.remove(other)
		} else {
			c.violations = append(c.violations, fmt.Sprintf("deleted %s while %s still referenced it", id, other))
		}
	}
}

func withID(inputs resource.PropertyMap, id resource.ID) resource.PropertyMap {
	outputs := inputs.Copy()
	outputs["id"] = resource.NewProperty(string(id))
	return outputs
}

func (c *fakeCloud) provider() plugin.Provider {
	return &deploytest.Provider{
		DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
			_ = c.call("diff " + req.URN.Name())
			if req.OldInputs.Equals(req.NewInputs) {
				return diffNone, nil
			}
			if diff, ok := c.diffs[req.URN.Name()]; ok {
				return diff, nil
			}
			return diffReplace, nil
		},
		CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
			c.mu.Lock()
			c.created[req.URN.Name()]++
			id := resource.ID(req.URN.Name() + strconv.Itoa(c.created[req.URN.Name()]))
			c.mu.Unlock()
			if err := c.call("create " + string(id)); err != nil {
				return plugin.CreateResponse{}, err
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			c.live[id] = liveResource{req.URN.Name(), req.Properties}
			return plugin.CreateResponse{ID: id, Properties: withID(req.Properties, id), Status: resource.StatusOK}, nil
		},
		UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
			if err := c.call("update " + string(req.ID)); err != nil {
				return plugin.UpdateResponse{Status: resource.StatusOK}, err
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			if _, ok := c.live[req.ID]; !ok {
				c.violations = append(c.violations, fmt.Sprintf("updated %s, which doesn't exist", req.ID))
			}
			c.live[req.ID] = liveResource{req.URN.Name(), req.NewInputs}
			return plugin.UpdateResponse{Properties: withID(req.NewInputs, req.ID), Status: resource.StatusOK}, nil
		},
		DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
			if err := c.call("delete " + string(req.ID)); err != nil {
				return plugin.DeleteResponse{}, err
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			c.remove(req.ID)
			return plugin.DeleteResponse{}, nil
		},
	}
}

// run runs one update per program, starting from the old snapshot, and returns the result of the last. Every update
// but the last must succeed.
func (c *fakeCloud) run(
	t *testing.T, old *deploy.Snapshot, opts UpdateOptions, programs ...func(*deploytest.ResourceMonitor),
) (*deploy.Snapshot, error) {
	if old != nil {
		for _, r := range old.Resources {
			c.live[r.ID] = liveResource{r.URN.Name(), r.Inputs}
			c.created[r.URN.Name()]++
		}
	}

	var program func(*deploytest.ResourceMonitor)
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		program(monitor)
		return nil
	})
	loader := deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
		return c.provider(), nil
	}, deploytest.WithoutGrpc)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{
		T:                t,
		SkipDisplayTests: true,
		UpdateOptions:    opts,
		HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loader),
	}}

	snap, err := old, error(nil)
	for i, prog := range programs {
		require.NoError(t, err)
		program = prog
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		snap, err = lt.TestOp(Update).RunWithContextStep(
			ctx, p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, strconv.Itoa(i))
		require.NoError(t, ctx.Err(), "the update hung")
		cancel()
	}
	return snap, err
}

// requireConsistent checks that no resource was deleted while another referenced it, and that the snapshot tracks
// exactly the resources that exist in the cloud.
func (c *fakeCloud) requireConsistent(t *testing.T, snap *deploy.Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	require.Empty(t, c.violations)
	require.NotNil(t, snap)
	var tracked []resource.ID
	for _, r := range snap.Resources {
		if !providers.IsProviderType(r.Type) && !r.PendingReplacement {
			tracked = append(tracked, r.ID)
		}
	}
	require.ElementsMatch(t, slices.Collect(maps.Keys(c.live)), tracked, "the snapshot doesn't track the cloud")
}

// register registers a resource with input v, and an input referencing the ID of each of refs.
func register(
	t *testing.T, monitor *deploytest.ResourceMonitor, name, v string, refs ...*deploytest.RegisterResourceResponse,
) *deploytest.RegisterResourceResponse {
	return registerWith(t, monitor, name, v, deploytest.ResourceOptions{}, refs...)
}

func registerWith(
	t *testing.T, monitor *deploytest.ResourceMonitor, name, v string, opts deploytest.ResourceOptions,
	refs ...*deploytest.RegisterResourceResponse,
) *deploytest.RegisterResourceResponse {
	opts.Inputs = resource.PropertyMap{"v": resource.NewProperty(v)}
	opts.PropertyDeps = map[resource.PropertyKey][]resource.URN{}
	opts.SupportsResultReporting = true
	for _, ref := range refs {
		key := resource.PropertyKey(ref.URN.Name())
		if id, ok := ref.Outputs["id"]; ok {
			opts.Inputs[key] = id
		}
		opts.PropertyDeps[key] = []resource.URN{ref.URN}
		opts.Dependencies = append(opts.Dependencies, ref.URN)
	}
	resp, err := monitor.RegisterResource("pkgA:m:typA", name, true, opts)
	require.NoError(t, err)
	return resp
}

// oldState returns the state of a resource with input v = "1", and an input referencing the ID of each of refs.
func oldState(name string, id resource.ID, refs ...*pkgresource.State) *pkgresource.State {
	urn := resource.NewURN("test", "test", "", "pkgA:m:typA", name)
	inputs := resource.PropertyMap{"v": resource.NewProperty("1")}
	propertyDeps := map[resource.PropertyKey][]resource.URN{}
	deps := make([]resource.URN, 0, len(refs))
	for _, ref := range refs {
		key := resource.PropertyKey(ref.URN.Name())
		inputs[key] = resource.NewProperty(string(ref.ID))
		propertyDeps[key] = []resource.URN{ref.URN}
		deps = append(deps, ref.URN)
	}
	return &pkgresource.State{
		Type: urn.Type(), URN: urn, Custom: true, ID: id, Inputs: inputs, Outputs: withID(inputs, id),
		Dependencies: deps, PropertyDependencies: propertyDeps,
	}
}

// The old b depends on a. The program replaces b (create-before-delete) without that dependency, then replaces a
// (delete-before-replace). a mustn't be deleted until the old b is, at the end of the update.
func TestDeleteBeforeReplaceAfterCreateBeforeDeleteOfDependent(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace}

	snap, err := cloud.run(t, nil, UpdateOptions{},
		func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", "1")
			register(t, m, "b", "1", a)
		},
		func(m *deploytest.ResourceMonitor) {
			register(t, m, "b", "2")
			register(t, m, "a", "2")
		})
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}

// The old b depends on a. The program replaces a (delete-before-replace), which first deletes b, and then registers b
// without that dependency and without waiting for a. The new b is created before the old b is deleted.
func TestDeleteBeforeReplaceRecreateOvertakesDelete(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace}
	cloud.waits = map[string]string{"delete b1": "registered b"}

	snap, err := cloud.run(t, nil, UpdateOptions{},
		func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", "1")
			register(t, m, "b", "1", a)
		},
		func(m *deploytest.ResourceMonitor) {
			var wg sync.WaitGroup
			wg.Go(func() { register(t, m, "a", "2") })
			cloud.waitFor("diff b")
			register(t, m, "b", "1")
			cloud.happened("registered b")
			wg.Wait()
		})
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}

// The old b depends on a. Without waiting for each other, the program updates b to drop that dependency, then
// replaces a (delete-before-replace). a mustn't be deleted until b's update has run.
func TestDeleteBeforeReplaceWaitsForDependentUpdate(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace, "b": diffUpdate}
	cloud.waits = map[string]string{"update b1": "delete a1"}

	snap, err := cloud.run(t, nil, UpdateOptions{},
		func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", "1")
			register(t, m, "b", "1", a)
		},
		func(m *deploytest.ResourceMonitor) {
			var wg sync.WaitGroup
			wg.Go(func() { register(t, m, "b", "2") })
			cloud.waitFor("diff b")
			register(t, m, "a", "2")
			wg.Wait()
		})
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}

// The old c depends on a and b. Without waiting for each other, the program replaces a and b (delete-before-replace).
// a's replacement deletes c first, and b mustn't be deleted until it has.
func TestDeleteBeforeReplaceWaitsForDependentDelete(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace, "b": diffDeleteBeforeReplace}
	cloud.waits = map[string]string{"delete c1": "delete b1"}

	snap, err := cloud.run(t, nil, UpdateOptions{},
		func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", "1")
			b := register(t, m, "b", "1")
			register(t, m, "c", "1", a, b)
		},
		func(m *deploytest.ResourceMonitor) {
			var wg sync.WaitGroup
			wg.Go(func() { register(t, m, "a", "2") })
			cloud.waitFor("diff c")
			wg.Go(func() { register(t, m, "b", "2") })
			wg.Wait()
			register(t, m, "c", "1")
		})
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}

// The snapshot holds a, and a pending-delete copy of x that depends on a. The program replaces a
// (delete-before-replace), which deletes the copy of x first, and then creates x.
//
// The engine panics. Enabling panic recovery, which means this test can't run in parallel, moves the panic to this
// goroutine, where it can be caught.
func TestDeleteBeforeReplaceOfPendingDeleteDependent(t *testing.T) {
	t.Setenv("PULUMI_DEV", "true")
	t.Setenv("PULUMI_GOROUTINE_PANIC_RECOVERY", "true")

	a := oldState("a", "a1")
	x := oldState("x", "x1", a)
	x.Delete = true

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace}

	var snap *deploy.Snapshot
	var err error
	require.NotPanics(t, func() {
		snap, err = cloud.run(t, &deploy.Snapshot{Resources: []*pkgresource.State{a, x}}, UpdateOptions{},
			func(m *deploytest.ResourceMonitor) {
				register(t, m, "a", "2")
				register(t, m, "x", "1")
			})
	})
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}

// The snapshot holds a, x, and a pending-delete copy of x that depends on a. The program replaces a
// (delete-before-replace), which deletes the copy of x first, and keeps x.
func TestDeleteBeforeReplaceOfPendingDeleteDependentWithLiveCopy(t *testing.T) {
	t.Parallel()

	a := oldState("a", "a1")
	x := oldState("x", "x1")
	xCopy := oldState("x", "x2", a)
	xCopy.Delete = true

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace}

	snap, err := cloud.run(t, &deploy.Snapshot{Resources: []*pkgresource.State{a, x, xCopy}}, UpdateOptions{},
		func(m *deploytest.ResourceMonitor) {
			register(t, m, "a", "2")
			register(t, m, "x", "1")
		})
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}

// With --continue-on-error, the program replaces a (delete-before-replace), and deleting a fails. a's registration
// must fail, rather than leave the program waiting for it forever.
func TestContinueOnErrorFailedDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace}
	cloud.fail = map[string]bool{"delete a1": true}

	snap, err := cloud.run(t, nil, UpdateOptions{ContinueOnError: true},
		func(m *deploytest.ResourceMonitor) { register(t, m, "a", "1") },
		func(m *deploytest.ResourceMonitor) { register(t, m, "a", "2") })
	require.ErrorContains(t, err, "delete a1 failed")
	cloud.requireConsistent(t, snap)
}

// The old b depends on a. With --continue-on-error, a's update fails, and the program then replaces b
// (delete-before-replace) without that dependency. Deleting the old b is skipped, as it depends on a, so the new b
// mustn't be created either.
func TestContinueOnErrorSkippedDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffUpdate, "b": diffDeleteBeforeReplace}
	cloud.fail = map[string]bool{"update a1": true}

	snap, err := cloud.run(t, nil, UpdateOptions{ContinueOnError: true},
		func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", "1")
			register(t, m, "b", "1", a)
		},
		func(m *deploytest.ResourceMonitor) {
			register(t, m, "a", "2")
			register(t, m, "b", "2")
		})
	require.ErrorContains(t, err, "update a1 failed")
	cloud.requireConsistent(t, snap)
}

// The old b depends on a. With --continue-on-error, the program replaces a (delete-before-replace), which first
// deletes b, and that fails. When the program then registers b, the engine mustn't recreate it.
func TestContinueOnErrorFailedDeleteOfDependent(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace}
	cloud.fail = map[string]bool{"delete b1": true}

	snap, err := cloud.run(t, nil, UpdateOptions{ContinueOnError: true},
		func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", "1")
			register(t, m, "b", "1", a)
		},
		func(m *deploytest.ResourceMonitor) {
			var wg sync.WaitGroup
			wg.Go(func() { register(t, m, "a", "2") })
			cloud.waitFor("delete b1")
			register(t, m, "b", "1")
			wg.Wait()
		})
	require.ErrorContains(t, err, "delete b1 failed")
	cloud.requireConsistent(t, snap)
}

// The old b depends on a. With --continue-on-error, the program's update of b to drop that dependency fails, and the
// program then replaces a (delete-before-replace). b still depends on a, so a mustn't be deleted before it.
func TestContinueOnErrorFailedUpdateOfDependent(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace, "b": diffUpdate}
	cloud.fail = map[string]bool{"update b1": true}

	snap, err := cloud.run(t, nil, UpdateOptions{ContinueOnError: true},
		func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", "1")
			register(t, m, "b", "1", a)
		},
		func(m *deploytest.ResourceMonitor) {
			register(t, m, "b", "2")
			register(t, m, "a", "2")
		})
	require.ErrorContains(t, err, "update b1 failed")
	cloud.requireConsistent(t, snap)
}

// The old b depends on a. With --continue-on-error, the program creates c, which fails, updates b to depend on c
// instead of a, which is skipped, and drops a. The old b still depends on a, so a mustn't be deleted.
func TestContinueOnErrorSkippedUpdateOfDependent(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"b": diffUpdate}
	cloud.fail = map[string]bool{"create c1": true}

	snap, err := cloud.run(t, nil, UpdateOptions{ContinueOnError: true},
		func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", "1")
			register(t, m, "b", "1", a)
		},
		func(m *deploytest.ResourceMonitor) {
			c := register(t, m, "c", "1")
			register(t, m, "b", "1", c)
		})
	require.ErrorContains(t, err, "create c1 failed")
	cloud.requireConsistent(t, snap)
}

// The snapshot holds w, x which is deleted with w, and a pending-delete copy of w. The program keeps w and drops x.
// The w that x is deleted with is kept, so x must be deleted itself.
func TestDeletedWithPendingDeleteCopy(t *testing.T) {
	t.Parallel()

	w := oldState("w", "w1")
	x := oldState("x", "x1", w)
	x.DeletedWith = w.URN
	wCopy := oldState("w", "w2")
	wCopy.Delete = true

	cloud := newFakeCloud()
	cloud.deletedWith = map[string]string{"x": "w"}

	snap, err := cloud.run(t, &deploy.Snapshot{Resources: []*pkgresource.State{w, x, wCopy}}, UpdateOptions{},
		func(m *deploytest.ResourceMonitor) { register(t, m, "w", "1") })
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}

// x is deleted with w, and also depends on d. The program drops all three. d mustn't be deleted until x is, which
// happens when w is deleted.
func TestDeletedWithOutlivesOtherDependencies(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.deletedWith = map[string]string{"x": "w"}

	snap, err := cloud.run(t, nil, UpdateOptions{},
		func(m *deploytest.ResourceMonitor) {
			w := register(t, m, "w", "1")
			d := register(t, m, "d", "1", w)
			registerWith(t, m, "x", "1", deploytest.ResourceOptions{DeletedWith: w.URN}, w, d)
		},
		func(m *deploytest.ResourceMonitor) {})
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}

// The old b is replaced with a. The program updates b to no longer be replaced with a, then replaces a
// (delete-before-replace). b mustn't be deleted.
func TestDeleteBeforeReplaceOfFormerReplaceWith(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace, "b": diffUpdate}

	snap, err := cloud.run(t, nil, UpdateOptions{},
		func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", "1")
			registerWith(t, m, "b", "1", deploytest.ResourceOptions{ReplaceWith: []resource.URN{a.URN}})
		},
		func(m *deploytest.ResourceMonitor) {
			register(t, m, "b", "2")
			register(t, m, "a", "2")
		})
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}

// b depends on a and is replaced with it, and c depends on b. The program replaces a (delete-before-replace). The
// provider says b needn't be replaced, but replaceWith replaces it anyway, so c must be deleted before b.
func TestDeleteBeforeReplaceOfReplaceWithDependents(t *testing.T) {
	t.Parallel()

	cloud := newFakeCloud()
	cloud.diffs = map[string]plugin.DiffResult{"a": diffDeleteBeforeReplace, "b": diffNone}

	program := func(v string) func(*deploytest.ResourceMonitor) {
		return func(m *deploytest.ResourceMonitor) {
			a := register(t, m, "a", v)
			b := registerWith(t, m, "b", "1", deploytest.ResourceOptions{ReplaceWith: []resource.URN{a.URN}}, a)
			register(t, m, "c", "1", b)
		}
	}
	snap, err := cloud.run(t, nil, UpdateOptions{}, program("1"), program("2"))
	require.NoError(t, err)
	cloud.requireConsistent(t, snap)
}
