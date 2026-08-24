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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// A workflowHarness holds the callbacks a test's workflow program registers: node programs that deploy
// one pkgA resource configured from the cursor's values, and conditions answered by the test.
type workflowHarness struct {
	t         *testing.T
	callbacks *deploytest.CallbackServer

	mu         sync.Mutex
	runs       map[string][]bool // node -> the reconcile flag of each run
	lastValues map[string]map[string]any
}

func newWorkflowHarness(t *testing.T) *workflowHarness {
	callbacks, err := deploytest.NewCallbacksServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, callbacks.Close()) })
	return &workflowHarness{t: t, callbacks: callbacks, runs: map[string][]bool{}, lastValues: map[string]map[string]any{}}
}

func (h *workflowHarness) runsOf(node string) []bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]bool(nil), h.runs[node]...)
}

// program is a node program: it dials the nested monitor the engine started for this run, registers a
// root stack and one pkgA resource whose "image" input comes from the cursor, and leaves "deployed" on
// the cursor.
func (h *workflowHarness) program(name string) *pulumirpc.Callback {
	cb, err := h.callbacks.Allocate(func(b []byte) (proto.Message, error) {
		var req pulumirpc.WorkflowNodeRequest
		if err := proto.Unmarshal(b, &req); err != nil {
			return nil, err
		}
		values, err := plugin.UnmarshalProperties(req.Cursor.Values, plugin.MarshalOptions{KeepSecrets: true})
		if err != nil {
			return nil, err
		}
		h.mu.Lock()
		h.runs[name] = append(h.runs[name], req.Reconcile)
		h.lastValues[name] = values.Mappable()
		h.mu.Unlock()

		conn, err := grpc.NewClient(req.MonitorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		nested := deploytest.NewResourceMonitor(pulumirpc.NewResourceMonitorClient(conn))
		stack, err := nested.RegisterResource(resource.RootStackType, req.Project+"-"+req.Stack, false)
		if err != nil {
			return nil, err
		}
		_, err = nested.RegisterResource("pkgA:m:typA", name, true, deploytest.ResourceOptions{
			Parent: stack.URN,
			Inputs: resource.PropertyMap{"image": values["image"]},
		})
		if err != nil {
			return nil, err
		}
		outputs := values.Copy()
		outputs["deployed"] = resource.NewProperty(name)
		s, err := plugin.MarshalProperties(outputs, plugin.MarshalOptions{KeepSecrets: true})
		if err != nil {
			return nil, err
		}
		return &pulumirpc.WorkflowNodeResponse{Outputs: s}, nil
	})
	require.NoError(h.t, err)
	return cb
}

// condition answers an edge condition with *pass.
func (h *workflowHarness) condition(pass *bool) *pulumirpc.Callback {
	cb, err := h.callbacks.Allocate(func(b []byte) (proto.Message, error) {
		var req pulumirpc.WorkflowConditionRequest
		if err := proto.Unmarshal(b, &req); err != nil {
			return nil, err
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		return &pulumirpc.WorkflowConditionResponse{Pass: *pass}, nil
	})
	require.NoError(h.t, err)
	return cb
}

func callbackProperty(cb *pulumirpc.Callback) resource.PropertyValue {
	return resource.NewProperty(resource.PropertyMap{
		"target": resource.NewProperty(cb.Target),
		"token":  resource.NewProperty(cb.Token),
	})
}

func workflowCursors(t *testing.T, snap *deploy.Snapshot) map[string]string {
	t.Helper()
	for _, res := range snap.Resources {
		if res.Type == deploy.WorkflowType {
			cursors := map[string]string{}
			for _, c := range res.Outputs["cursors"].ArrayValue() {
				o := c.ObjectValue()
				cursors[o["name"].StringValue()] = o["node"].StringValue()
			}
			return cursors
		}
	}
	require.Fail(t, "workflow resource not found in snapshot")
	return nil
}

// nodeRecord returns the workflow's authoritative record of node (its node resource's outputs only mirror it
// as of the node's last run).
func nodeRecord(t *testing.T, snap *deploy.Snapshot, node string) resource.PropertyMap {
	t.Helper()
	for _, res := range snap.Resources {
		if res.Type == deploy.WorkflowType {
			return res.Outputs["records"].ObjectValue()[resource.PropertyKey(node)].ObjectValue()
		}
	}
	require.Fail(t, "workflow resource not found in snapshot")
	return nil
}

func findResource(snap *deploy.Snapshot, urn resource.URN) *pkgresource.State {
	for _, res := range snap.Resources {
		if res.URN == urn {
			return res
		}
	}
	return nil
}

// TestWorkflowLifecycle drives a two-node workflow (dev -> prod, gated) through its life: a cursor is
// placed and deploys dev; dev is reconciled on the next up; the gate opens and the cursor deploys prod
// while dev, now vacated, keeps reconciling; prod is removed from the definition and swept with its
// resources while the orphaned cursor is kept; destroy removes everything.
func TestWorkflowLifecycle(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	h := newWorkflowHarness(t)
	promote := false
	withProd := true

	var registered resource.PropertyMap // the outputs the program receives for the workflow
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		nodes := resource.PropertyMap{
			"dev": resource.NewProperty(resource.PropertyMap{"program": callbackProperty(h.program("dev"))}),
		}
		edges := []resource.PropertyValue{}
		if withProd {
			nodes["prod"] = resource.NewProperty(resource.PropertyMap{"program": callbackProperty(h.program("prod"))})
			edges = append(edges, resource.NewProperty(resource.PropertyMap{
				"name": resource.NewProperty("promote"),
				"kind": resource.NewProperty("single"),
				"from": resource.NewProperty("dev"),
				"to":   resource.NewProperty("prod"),
				"conditions": resource.NewProperty(resource.PropertyMap{
					"promote": callbackProperty(h.condition(&promote)),
				}),
			}))
		}
		resp, err := monitor.RegisterResource(deploy.WorkflowType, "wf", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"nodes": resource.NewProperty(nodes),
				"edges": resource.NewProperty(edges),
				"entries": resource.NewProperty(resource.PropertyMap{
					"release": resource.NewProperty(resource.PropertyMap{
						"node":   resource.NewProperty("dev"),
						"inputs": resource.NewProperty(resource.PropertyMap{"image": resource.NewProperty("v1")}),
					}),
				}),
			},
		})
		require.NoError(t, err)
		registered = resp.Outputs
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	project := p.GetProject()

	wfURN := resource.URN("urn:pulumi:test::test::pulumi:index:Workflow::wf")
	// Node resources and their programs' resources live in the main snapshot, namespaced by a per-node
	// project qualifier: the node resource under the workflow, the program's root stack parented under
	// the node resource. (Children of a root stack carry no type prefix in their URNs.)
	devNode := resource.URN("urn:pulumi:test::test-wf-wf-dev::pulumi:index:Workflow$pulumi:index:WorkflowNode::dev")
	prodNode := resource.URN("urn:pulumi:test::test-wf-wf-prod::pulumi:index:Workflow$pulumi:index:WorkflowNode::prod")
	devStack := resource.URN("urn:pulumi:test::test-wf-wf-dev::pulumi:pulumi:Stack::test-wf-wf-dev-test")
	devRes := resource.URN("urn:pulumi:test::test-wf-wf-dev::pkgA:m:typA::dev")
	prodStack := resource.URN("urn:pulumi:test::test-wf-wf-prod::pulumi:pulumi:Stack::test-wf-wf-prod-test")
	prodRes := resource.URN("urn:pulumi:test::test-wf-wf-prod::pkgA:m:typA::prod")

	// Up 0: the entry places release#1 at dev, whose program runs on arrival. The gate fails, so the
	// cursor parks at dev. Both node resources exist; only dev has program resources. The workflow
	// advanced before its registration completed, so the program saw this run's cursors.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"release#1": "dev"}, workflowCursors(t, snap))
	require.Contains(t, registered, resource.PropertyKey("cursors"))
	assert.Equal(t, []resource.PropertyValue{resource.NewProperty(resource.PropertyMap{
		"name": resource.NewProperty("release#1"), "node": resource.NewProperty("dev"),
	})}, registered["cursors"].ArrayValue())
	assert.Equal(t, []bool{false}, h.runsOf("dev"))
	assert.Empty(t, h.runsOf("prod"))
	require.NotNil(t, findResource(snap, devNode))
	require.NotNil(t, findResource(snap, prodNode))
	assert.Equal(t, wfURN, findResource(snap, devNode).Parent)
	stack := findResource(snap, devStack)
	require.NotNil(t, stack)
	assert.Equal(t, devNode, stack.Parent)
	dev := findResource(snap, devRes)
	require.NotNil(t, dev)
	assert.Equal(t, devStack, dev.Parent)
	assert.Equal(t, resource.PropertyMap{"image": resource.NewProperty("v1")}, dev.Inputs)
	assert.Nil(t, findResource(snap, prodRes))
	devState := nodeRecord(t, snap, "dev")
	assert.Equal(t, "release#1", devState["occupant"].StringValue())
	visits := devState["visits"].ArrayValue()
	require.Len(t, visits, 1)
	// The node resource mirrors its record.
	assert.Equal(t, devState, findResource(snap, devNode).Outputs)
	assert.Equal(t, "release#1", visits[0].ObjectValue()["cursor"].StringValue())
	assert.Equal(t, resource.PropertyMap{
		"image":    resource.NewProperty("v1"),
		"deployed": resource.NewProperty("dev"),
	}, visits[0].ObjectValue()["outputs"].ObjectValue())

	// Up 1: nothing changed. The gate still fails; dev is reconciled from the values release#1 entered
	// with; prod has never run and is left alone.
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"release#1": "dev"}, workflowCursors(t, snap))
	assert.Equal(t, []bool{false, true}, h.runsOf("dev"))
	assert.Empty(t, h.runsOf("prod"))
	assert.Equal(t, map[string]any{"image": "v1"}, h.lastValues["dev"])
	require.NotNil(t, findResource(snap, devRes))

	// Up 2: the gate opens. The cursor moves to prod, running prod's program with the values dev left on
	// it; dev, now vacated, is reconciled too. Both programs' resources are live.
	promote = true
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"release#1": "prod"}, workflowCursors(t, snap))
	assert.Equal(t, []bool{false, true, true}, h.runsOf("dev"))
	assert.Equal(t, []bool{false}, h.runsOf("prod"))
	assert.Equal(t, map[string]any{"image": "v1", "deployed": "dev"}, h.lastValues["prod"])
	require.NotNil(t, findResource(snap, devRes))
	require.NotNil(t, findResource(snap, prodRes))
	devState = nodeRecord(t, snap, "dev")
	_, occupied := devState["occupant"]
	assert.False(t, occupied)
	visits = devState["visits"].ArrayValue()
	require.Len(t, visits, 1)
	_, left := visits[0].ObjectValue()["left"]
	assert.True(t, left, "the visit to dev is closed")
	assert.Equal(t, "release#1", nodeRecord(t, snap, "prod")["occupant"].StringValue())

	// A preview runs no callbacks and moves nothing, but plans no deletion of node resources or their
	// programs' resources either.
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient,
		func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
			for _, e := range events {
				if e.Type == ResourcePreEvent {
					assert.NotEqual(t, deploy.OpDelete, e.Payload().(ResourcePreEventPayload).Metadata.Op,
						"preview must not plan deletes of %v", e.Payload().(ResourcePreEventPayload).Metadata.URN)
				}
			}
			return err
		}, "preview")
	require.NoError(t, err)
	assert.Equal(t, []bool{false, true, true}, h.runsOf("dev"))
	assert.Equal(t, []bool{false}, h.runsOf("prod"))

	// Up 3: prod leaves the definition. Its node resource and program resources are swept; the cursor it
	// held is kept in the workflow's state (and reported) until the node comes back. dev keeps reconciling.
	withProd = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "3")
	require.NoError(t, err)
	assert.Empty(t, workflowCursors(t, snap))
	assert.Nil(t, findResource(snap, prodNode))
	assert.Nil(t, findResource(snap, prodStack))
	assert.Nil(t, findResource(snap, prodRes))
	require.NotNil(t, findResource(snap, devRes))
	assert.Equal(t, []bool{false, true, true, true}, h.runsOf("dev"))
	var state struct {
		Workflow struct {
			Cursors []struct {
				Label string `json:"label"`
				Node  string `json:"node"`
			} `json:"cursors"`
		} `json:"workflow"`
	}
	wf := findResource(snap, wfURN)
	require.NoError(t, json.Unmarshal([]byte(wf.Outputs["state"].StringValue()), &state))
	require.Len(t, state.Workflow.Cursors, 1)
	assert.Equal(t, "release#1", state.Workflow.Cursors[0].Label)
	assert.Equal(t, "prod", state.Workflow.Cursors[0].Node)

	// Destroy: node resources and program resources are swept with everything else.
	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "4")
	require.NoError(t, err)
	assert.Empty(t, snap.Resources)
}

// TestWorkflowParallelNodes proves independent cursors deploy their nodes concurrently: two disconnected
// chains move in the same up, and the entered nodes' programs rendezvous — if node programs ran
// serially, the first would block on the barrier and the test would fail.
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
		program := func(name string, meet bool) resource.PropertyValue {
			cb, err := callbacks.Allocate(func(b []byte) (proto.Message, error) {
				var req pulumirpc.WorkflowNodeRequest
				if err := proto.Unmarshal(b, &req); err != nil {
					return nil, err
				}
				if meet {
					if err := barrier(); err != nil {
						return nil, err
					}
				}
				conn, err := grpc.NewClient(req.MonitorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					return nil, err
				}
				defer conn.Close()
				nested := deploytest.NewResourceMonitor(pulumirpc.NewResourceMonitorClient(conn))
				_, err = nested.RegisterResource("pkgA:m:typA", name, true)
				if err != nil {
					return nil, err
				}
				return &pulumirpc.WorkflowNodeResponse{Outputs: req.Cursor.Values}, nil
			})
			require.NoError(t, err)
			return resource.NewProperty(resource.PropertyMap{"program": callbackProperty(cb)})
		}
		pass, err := callbacks.Allocate(func([]byte) (proto.Message, error) {
			return &pulumirpc.WorkflowConditionResponse{Pass: true}, nil
		})
		require.NoError(t, err)
		edge := func(from, to string) resource.PropertyValue {
			return resource.NewProperty(resource.PropertyMap{
				"name":       resource.NewProperty(from + "-" + to),
				"kind":       resource.NewProperty("single"),
				"from":       resource.NewProperty(from),
				"to":         resource.NewProperty(to),
				"conditions": resource.NewProperty(resource.PropertyMap{"go": callbackProperty(pass)}),
			})
		}
		entry := func(node string) resource.PropertyValue {
			return resource.NewProperty(resource.PropertyMap{
				"node":   resource.NewProperty(node),
				"inputs": resource.NewProperty(resource.PropertyMap{"run": resource.NewProperty(node)}),
			})
		}
		_, err = monitor.RegisterResource(deploy.WorkflowType, "wf", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"nodes": resource.NewProperty(resource.PropertyMap{
					"a1": program("a1", false),
					"a2": program("a2", true),
					"b1": program("b1", false),
					"b2": program("b2", true),
				}),
				"edges":   resource.NewProperty([]resource.PropertyValue{edge("a1", "a2"), edge("b1", "b2")}),
				"entries": resource.NewProperty(resource.PropertyMap{"a": entry("a1"), "b": entry("b1")}),
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
	assert.Equal(t, map[string]string{"a#1": "a2", "b#2": "b2"}, workflowCursors(t, snap))
}

// TestWorkflowUnknownEntryInputs: unknown entry inputs are tolerated by a preview, which never places
// cursors, and rejected by an up, which must diff them against the last placement.
func TestWorkflowUnknownEntryInputs(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	h := newWorkflowHarness(t)

	unknown := true
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		image := resource.NewProperty("v1")
		if unknown {
			image = resource.MakeComputed(resource.NewProperty(""))
		}
		_, err := monitor.RegisterResource(deploy.WorkflowType, "wf", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"nodes": resource.NewProperty(resource.PropertyMap{
					"dev": resource.NewProperty(resource.PropertyMap{"program": callbackProperty(h.program("dev"))}),
				}),
				"entries": resource.NewProperty(resource.PropertyMap{
					"release": resource.NewProperty(resource.PropertyMap{
						"node":   resource.NewProperty("dev"),
						"inputs": resource.NewProperty(resource.PropertyMap{"image": image}),
					}),
				}),
			},
		})
		if unknown {
			return err // An up with unknown inputs is expected to fail its registration.
		}
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	project := p.GetProject()

	// A first-deployment preview: the entry's inputs are unknown, and nothing is placed.
	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// The up resolves them and places the cursor.
	unknown = false
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"release#1": "dev"}, workflowCursors(t, snap))

	// An up whose entry inputs are unknown is rejected.
	unknown = true
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.ErrorContains(t, err, `entry "release": inputs must be known`)
}
