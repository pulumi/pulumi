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
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// TestWorkflowLifecycle drives a two-node workflow (dev -> prod, gated) through its full life:
// admission, reconcile-while-occupied, a failing then passing gate, and destroy.
func TestWorkflowLifecycle(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	var mu sync.Mutex
	promote := false
	nodeRuns := map[string]int{}
	runs := func(name string) int {
		mu.Lock()
		defer mu.Unlock()
		return nodeRuns[name]
	}

	// The callback server outlives each program run: workflows advance after the source completes,
	// so the engine invokes node/condition callbacks once the program function has returned. (Real
	// SDKs stay alive in SignalAndWaitForShutdown until the deployment finishes.)
	callbacks, err := deploytest.NewCallbacksServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, callbacks.Close()) })

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// A node program: dials the nested monitor the engine started for this reconcile, registers
		// a root stack, one custom resource configured from the cursor's data, and stack outputs.
		nodeProgram := func(name string) *pulumirpc.Callback {
			cb, err := callbacks.Allocate(func(req []byte) (proto.Message, error) {
				var p struct {
					MonitorAddr string            `json:"monitorAddr"`
					Config      map[string]string `json:"config"`
				}
				if err := json.Unmarshal(req, &p); err != nil {
					return nil, err
				}
				mu.Lock()
				nodeRuns[name]++
				mu.Unlock()

				conn, err := grpc.NewClient(p.MonitorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					return nil, err
				}
				defer conn.Close()
				nested := deploytest.NewResourceMonitor(pulumirpc.NewResourceMonitorClient(conn))

				stackRes, err := nested.RegisterResource(resource.RootStackType, "node-stack", false)
				if err != nil {
					return nil, err
				}
				_, err = nested.RegisterResource("pkgA:m:typA", name, true, deploytest.ResourceOptions{
					Parent: stackRes.URN,
					Inputs: resource.PropertyMap{
						"image": resource.NewProperty(p.Config["workflow:image"]),
					},
				})
				if err != nil {
					return nil, err
				}
				err = nested.RegisterResourceOutputs(stackRes.URN, resource.PropertyMap{
					"deployed": resource.NewProperty(name),
				})
				if err != nil {
					return nil, err
				}
				return wrapperspb.String("{}"), nil
			})
			require.NoError(t, err)
			return cb
		}

		devCB := nodeProgram("dev")
		prodCB := nodeProgram("prod")
		gateCB, err := callbacks.Allocate(func([]byte) (proto.Message, error) {
			mu.Lock()
			pass := promote
			mu.Unlock()
			b, err := json.Marshal(map[string]bool{"pass": pass})
			if err != nil {
				return nil, err
			}
			return wrapperspb.String(string(b)), nil
		})
		require.NoError(t, err)

		asCallback := func(cb *pulumirpc.Callback) resource.PropertyValue {
			return resource.NewProperty(resource.PropertyMap{
				"target": resource.NewProperty(cb.Target),
				"token":  resource.NewProperty(cb.Token),
			})
		}
		_, err = monitor.RegisterResource("pulumi:index:Workflow", "wf", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"nodes": resource.NewProperty(resource.PropertyMap{
					"dev":  asCallback(devCB),
					"prod": asCallback(prodCB),
				}),
				"edges": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"from":   resource.NewProperty("dev"),
						"to":     resource.NewProperty("prod"),
						"target": resource.NewProperty(gateCB.Target),
						"token":  resource.NewProperty(gateCB.Token),
					}),
				}),
				"entries": resource.NewProperty(resource.PropertyMap{
					"dev": resource.NewProperty(resource.PropertyMap{
						"image": resource.NewProperty("v1"),
					}),
				}),
			},
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	project := p.GetProject()

	workflowCursors := func(snap *deploy.Snapshot) []resource.PropertyValue {
		t.Helper()
		for _, res := range snap.Resources {
			if res.Type == "pulumi:index:Workflow" {
				return res.Outputs["cursors"].ArrayValue()
			}
		}
		require.Fail(t, "workflow resource not found in snapshot")
		return nil
	}
	cursorNode := func(c resource.PropertyValue) string {
		return c.ObjectValue()["node"].StringValue()
	}
	// Node resources live in the main snapshot, marked as owned by their (workflow, node) slice
	// and URN-namespaced by a per-node project qualifier.
	wfURN := resource.URN("urn:pulumi:test::test::pulumi:index:Workflow::wf")
	ownedResource := func(snap *deploy.Snapshot, node string) *pkgresource.State {
		for _, res := range snap.Resources {
			if res.Type == "pkgA:m:typA" && res.Owner == deploy.MakeWorkflowOwner(wfURN, node) {
				return res
			}
		}
		return nil
	}

	// Up 1: the entry admits a cursor at dev. Initial placement runs no node program, and there was
	// no occupant to reconcile; the gate is polled once (fails), so the cursor parks at dev.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	cursors := workflowCursors(snap)
	require.Len(t, cursors, 1)
	require.Equal(t, "dev", cursorNode(cursors[0]))
	require.Equal(t, 0, runs("dev"))
	require.Equal(t, 0, runs("prod"))
	require.Nil(t, ownedResource(snap, "dev"))

	// Up 2: dev now hosts a cursor, so it is reconciled (the node program runs); the gate still
	// fails and the cursor stays parked at dev.
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	cursors = workflowCursors(snap)
	require.Len(t, cursors, 1)
	require.Equal(t, "dev", cursorNode(cursors[0]))
	require.Equal(t, 1, runs("dev"))
	require.Equal(t, 0, runs("prod"))
	dev := ownedResource(snap, "dev")
	require.NotNil(t, dev)
	require.Equal(t, resource.URN("urn:pulumi:test::test-wf-wf-dev::pkgA:m:typA::dev"), dev.URN)
	require.Equal(t, resource.PropertyMap{
		"image": resource.NewProperty("v1"),
	}, dev.Inputs)

	// Up 3: the gate opens; the cursor moves to prod, running prod's program as it enters.
	mu.Lock()
	promote = true
	mu.Unlock()
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	cursors = workflowCursors(snap)
	require.Len(t, cursors, 1)
	require.Equal(t, "prod", cursorNode(cursors[0]))
	require.Equal(t, 2, runs("dev"))
	require.Equal(t, 1, runs("prod"))
	// The cursor's data picked up the node's stack outputs as it left dev and entered prod.
	require.Equal(t, resource.PropertyMap{
		"image":    resource.NewProperty("v1"),
		"deployed": resource.NewProperty("prod"),
	}, cursors[0].ObjectValue()["data"].ObjectValue())
	// Both slices are live in the main snapshot: dev keeps its resources after the cursor left.
	require.NotNil(t, ownedResource(snap, "dev"))
	require.NotNil(t, ownedResource(snap, "prod"))

	// Destroy: owned resources are swept with everything else — no workflow-specific destroy path.
	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "3")
	require.NoError(t, err)
	require.Empty(t, snap.Resources)
}

// TestWorkflowParallelNodes proves independent cursors deploy their nodes concurrently: two
// disconnected chains move in the same up, and the entered nodes' programs rendezvous — if node
// programs ran serially, the first would block on the barrier and the test would fail.
func TestWorkflowParallelNodes(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	var wg sync.WaitGroup
	wg.Add(2)
	barrier := func() error {
		wg.Done()
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
			return nil
		case <-time.After(1 * time.Minute):
			return errors.New("rendezvous timed out: node programs did not overlap")
		}
	}

	callbacks, err := deploytest.NewCallbacksServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, callbacks.Close()) })

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		nodeProgram := func(name string, meet bool) *pulumirpc.Callback {
			cb, err := callbacks.Allocate(func(req []byte) (proto.Message, error) {
				var p struct {
					MonitorAddr string `json:"monitorAddr"`
				}
				if err := json.Unmarshal(req, &p); err != nil {
					return nil, err
				}
				if meet {
					if err := barrier(); err != nil {
						return nil, err
					}
				}
				conn, err := grpc.NewClient(p.MonitorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					return nil, err
				}
				defer conn.Close()
				nested := deploytest.NewResourceMonitor(pulumirpc.NewResourceMonitorClient(conn))
				stackRes, err := nested.RegisterResource(resource.RootStackType, "node-stack", false)
				if err != nil {
					return nil, err
				}
				_, err = nested.RegisterResource("pkgA:m:typA", name, true, deploytest.ResourceOptions{
					Parent: stackRes.URN,
				})
				if err != nil {
					return nil, err
				}
				return wrapperspb.String("{}"), nil
			})
			require.NoError(t, err)
			return cb
		}

		pass, err := callbacks.Allocate(func([]byte) (proto.Message, error) {
			return wrapperspb.String(`{"pass":true}`), nil
		})
		require.NoError(t, err)

		asCallback := func(cb *pulumirpc.Callback) resource.PropertyValue {
			return resource.NewProperty(resource.PropertyMap{
				"target": resource.NewProperty(cb.Target),
				"token":  resource.NewProperty(cb.Token),
			})
		}
		edge := func(from, to string) resource.PropertyValue {
			return resource.NewProperty(resource.PropertyMap{
				"from":   resource.NewProperty(from),
				"to":     resource.NewProperty(to),
				"target": resource.NewProperty(pass.Target),
				"token":  resource.NewProperty(pass.Token),
			})
		}
		seed := func(v string) resource.PropertyValue {
			return resource.NewProperty(resource.PropertyMap{"run": resource.NewProperty(v)})
		}
		_, err = monitor.RegisterResource("pulumi:index:Workflow", "wf", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"nodes": resource.NewProperty(resource.PropertyMap{
					"a1": asCallback(nodeProgram("a1", false)),
					"a2": asCallback(nodeProgram("a2", true)),
					"b1": asCallback(nodeProgram("b1", false)),
					"b2": asCallback(nodeProgram("b2", true)),
				}),
				"edges": resource.NewProperty([]resource.PropertyValue{
					edge("a1", "a2"),
					edge("b1", "b2"),
				}),
				"entries": resource.NewProperty(resource.PropertyMap{
					"a1": seed("a"),
					"b1": seed("b"),
				}),
			},
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	nodes := map[string]bool{}
	for _, res := range snap.Resources {
		if res.Type == "pulumi:index:Workflow" {
			for _, c := range res.Outputs["cursors"].ArrayValue() {
				nodes[c.ObjectValue()["node"].StringValue()] = true
			}
		}
	}
	require.Equal(t, map[string]bool{"a2": true, "b2": true}, nodes)
}
