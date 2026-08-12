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

package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// workflowExecutor implements deploy.WorkflowExecutor: it runs pulumi:index:Workflow resources by
// driving the fsa scheduler with nested per-node deployments. Node programs and edge conditions are
// closures in the user's program, reached through the callbacks facility; each node's sub-state is a
// checkpoint-shaped snapshot serialized into the workflow resource's own state.
type workflowExecutor struct {
	plugctx       *plugin.Context
	backendClient deploy.BackendClient
	resourceHooks *deploy.ResourceHooks
	stackName     tokens.StackName
	organization  tokens.Name
	projectName   tokens.PackageName
	parallel      int32

	m         sync.Mutex
	callbacks map[string]*deploy.CallbacksClient
}

func newWorkflowExecutor(
	plugctx *plugin.Context,
	backendClient deploy.BackendClient,
	resourceHooks *deploy.ResourceHooks,
	stackName tokens.StackName,
	organization tokens.Name,
	projectName tokens.PackageName,
	parallel int32,
) *workflowExecutor {
	return &workflowExecutor{
		plugctx:       plugctx,
		backendClient: backendClient,
		resourceHooks: resourceHooks,
		stackName:     stackName,
		organization:  organization,
		projectName:   projectName,
		parallel:      parallel,
		callbacks:     map[string]*deploy.CallbacksClient{},
	}
}

// wfCallback identifies a closure in the user's program, reachable via the callbacks service.
type wfCallback struct {
	Target string
	Token  string
}

type wfEdge struct {
	From, To string
	Cond     wfCallback
}

// wfGraph is the workflow's shape, parsed from the resource's inputs.
type wfGraph struct {
	nodes   map[string]wfCallback
	edges   []wfEdge // in definition order: first passing edge wins
	entries map[string]map[string]any
}

// wfCursor is user data plus a position, persisted in the workflow's state.
type wfCursor struct {
	id        int64
	node      string
	data      map[string]any
	enteredAt time.Time
}

// wfState is the durable state of a workflow, persisted in the resource's output properties.
type wfState struct {
	cursors    []*wfCursor
	entrySeeds map[string]string // node -> hash of the last admitted entry seed
	nodeStates map[string]string // node -> JSON-serialized apitype.DeploymentV3 sub-snapshot
	nextID     int64
}

func (x *workflowExecutor) Update(
	ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
) (resource.PropertyMap, resource.Status, error) {
	g, err := parseWorkflowGraph(news)
	if err != nil {
		return nil, resource.StatusUnknown, err
	}
	st, err := parseWorkflowState(olds)
	if err != nil {
		return nil, resource.StatusUnknown, err
	}

	info := func(f string, a ...any) {
		x.plugctx.Diag.Infof(diag.Message(urn, "workflow: "+f), a...)
	}
	warn := func(f string, a ...any) {
		x.plugctx.Diag.Warningf(diag.Message(urn, "workflow: "+f), a...)
	}

	// Cursors on nodes no longer in the definition are deleted with their node during GC.
	var alive, doomed []*wfCursor
	for _, c := range st.cursors {
		if _, ok := g.nodes[c.node]; ok {
			alive = append(alive, c)
		} else {
			doomed = append(doomed, c)
		}
	}
	sortCursors(alive)

	var errs []error

	// 1. Reconcile every node currently hosting a cursor: node-body and config edits converge on
	// every up, and an up that previously died mid-entry is healed here by plain reconciliation.
	for _, c := range alive {
		outs, err := x.runNode(ctx, st, g, c.node, c.data)
		if err != nil {
			errs = append(errs, fmt.Errorf("reconciling node %q: %w", c.node, err))
			break
		}
		maps.Copy(c.data, outs)
	}

	// 2. Admit entries: each node has one entry slot; a seed whose hash differs from the recorded
	// one admits a new cursor. Initial placement does not run the node's program.
	if len(errs) == 0 {
		for _, node := range slices.Sorted(maps.Keys(g.entries)) {
			seed := g.entries[node]
			h := stableHash(seed)
			if st.entrySeeds[node] == h {
				continue
			}
			st.nextID++
			c := &wfCursor{
				id:        st.nextID,
				node:      node,
				data:      maps.Clone(seed),
				enteredAt: time.Now().UTC(),
			}
			alive = append(alive, c)
			st.entrySeeds[node] = h
			info("admitted cursor %d at node %q", c.id, node)
		}
	}

	// 3. Progress the automaton. Conditions are sampled once per visit; every up is a fresh visit.
	if len(errs) == 0 {
		final, err := x.progress(ctx, g, st, alive, info)
		if err != nil {
			errs = append(errs, err)
		}
		if final != nil {
			superseded := make(map[int64]bool, len(alive))
			for _, c := range alive {
				superseded[c.id] = true
			}
			for _, c := range final {
				delete(superseded, c.id)
			}
			for _, id := range slices.Sorted(maps.Keys(superseded)) {
				info("cursor %d was superseded and deleted", id)
			}
			st.cursors = final
		} else {
			st.cursors = alive
		}
	} else {
		st.cursors = append(alive, doomed...)
		doomed = nil
	}

	// 4. GC, after everything else: nodes in state but not in the definition are destroyed, and any
	// resident cursor is deleted with its node.
	if len(errs) == 0 {
		for _, name := range slices.Sorted(maps.Keys(st.nodeStates)) {
			if _, ok := g.nodes[name]; ok {
				continue
			}
			if err := x.destroyNode(ctx, st, name); err != nil {
				errs = append(errs, fmt.Errorf("destroying removed node %q: %w", name, err))
				continue
			}
			delete(st.nodeStates, name)
			info("destroyed removed node %q", name)
		}
		for _, c := range doomed {
			warn("deleted cursor %d along with removed node %q", c.id, c.node)
		}
	}

	outs := renderWorkflowState(news, st)
	if len(errs) > 0 {
		return outs, resource.StatusPartialFailure, errors.Join(errs...)
	}
	return outs, resource.StatusOK, nil
}

func (x *workflowExecutor) Destroy(
	ctx context.Context, urn resource.URN, olds resource.PropertyMap,
) (resource.Status, error) {
	st, err := parseWorkflowState(olds)
	if err != nil {
		return resource.StatusUnknown, err
	}
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(st.nodeStates)) {
		if err := x.destroyNode(ctx, st, name); err != nil {
			errs = append(errs, fmt.Errorf("destroying node %q: %w", name, err))
			continue
		}
		delete(st.nodeStates, name)
	}
	if len(errs) > 0 {
		return resource.StatusPartialFailure, errors.Join(errs...)
	}
	return resource.StatusOK, nil
}

// progress runs one fsa.Progress call over the workflow graph and returns the surviving cursors. A
// nil cursor slice with a non-nil error means the run failed before any structural change could be
// observed.
func (x *workflowExecutor) progress(
	ctx context.Context, g *wfGraph, st *wfState, cursors []*wfCursor, info func(string, ...any),
) ([]*wfCursor, error) {
	// st is shared with runNode through the node closures; SyncRunner keeps access serial.
	// ponytail: SyncRunner serializes node deployments; switch to a concurrent runner (plus a lock
	// around st) when parallel node deploys matter.
	m := fsa.New[*wfCursor]()
	fsaNodes := make(map[string]fsa.Node, len(g.nodes))
	nodeNames := make(map[fsa.Node]string, len(g.nodes))
	for _, name := range slices.Sorted(maps.Keys(g.nodes)) {
		n := m.NewNode(func(fctx context.Context, _ fsa.FSA[*wfCursor], _ fsa.Edge, c *wfCursor) error {
			outs, err := x.runNode(fctx, st, g, name, c.data)
			if err != nil {
				return fmt.Errorf("node %q: %w", name, err)
			}
			maps.Copy(c.data, outs)
			c.node = name
			c.enteredAt = time.Now().UTC()
			info("cursor %d entered node %q", c.id, name)
			return nil
		})
		fsaNodes[name] = n
		nodeNames[n] = name
	}
	for _, e := range g.edges {
		m.NewEdge(func(fctx context.Context, _ fsa.FSA[*wfCursor], c *wfCursor) (fsa.ConditionResult, error) {
			pass, err := x.invokeCondition(fctx, e.Cond, c)
			if err != nil {
				return fsa.ConditionUnknown, fmt.Errorf("condition %s -> %s: %w", e.From, e.To, err)
			}
			if pass {
				return fsa.ConditionPass, nil
			}
			return fsa.ConditionFail, nil
		}, fsaNodes[e.From], fsaNodes[e.To])
	}
	for _, c := range cursors {
		m.NewCursor(c, fsaNodes[c.node])
	}

	err := m.Progress(ctx, fsa.SyncRunner)

	var final []*wfCursor
	m.Cursors(func(c *wfCursor, n fsa.Node) bool {
		c.node = nodeNames[n]
		final = append(final, c)
		return true
	})
	sortCursors(final)
	return final, err
}

// runNode reconciles one node's sub-state by running its program as a nested deployment with the
// given cursor data as config. The resulting snapshot is persisted into st even when the deployment
// fails, so a later up can heal by reconciliation. Returns the node's stack outputs.
func (x *workflowExecutor) runNode(
	ctx context.Context, st *wfState, g *wfGraph, name string, data map[string]any,
) (map[string]any, error) {
	prev, err := x.loadNodeSnapshot(ctx, st, name)
	if err != nil {
		return nil, err
	}

	cb := g.nodes[name]
	runner := func(monitorAddr string) *promise.Promise[struct{}] {
		cs := &promise.CompletionSource[struct{}]{}
		go func() {
			payload := workflowNodeRequest{
				MonitorAddr: monitorAddr,
				Project:     string(x.projectName),
				Stack:       x.stackName.String(),
				Config:      workflowNodeConfig(data),
				Parallel:    x.parallel,
			}
			if _, err := x.invokeCallback(ctx, cb, payload); err != nil {
				cs.Reject(err)
			} else {
				cs.Fulfill(struct{}{})
			}
		}()
		return cs.Promise()
	}

	snap, err := x.runNested(ctx, prev, func(target *deploy.Target, panicErrs chan<- error) deploy.Source {
		runinfo := &deploy.EvalRunInfo{
			Proj: &workspace.Project{
				Name:    x.projectName,
				Runtime: workspace.NewProjectRuntimeInfo("workflow-node", nil),
			},
			Pwd:     x.plugctx.Pwd,
			Program: ".",
			Target:  target,
		}
		return deploy.NewEvalSource(x.plugctx, runinfo, nil, x.resourceHooks,
			deploy.EvalSourceOptions{Parallel: x.parallel}, panicErrs, nil, runner)
	})
	if snap != nil {
		serialized, serr := x.serializeNodeSnapshot(ctx, snap)
		if serr != nil {
			err = errors.Join(err, serr)
		} else {
			st.nodeStates[name] = serialized
		}
	}
	if err != nil {
		return nil, err
	}
	return workflowStackOutputs(snap), nil
}

// destroyNode tears down a node's sub-state by running a nested deployment with an empty program.
func (x *workflowExecutor) destroyNode(ctx context.Context, st *wfState, name string) error {
	prev, err := x.loadNodeSnapshot(ctx, st, name)
	if err != nil {
		return err
	}
	if prev == nil {
		return nil
	}
	snap, err := x.runNested(ctx, prev, func(*deploy.Target, chan<- error) deploy.Source {
		return deploy.NewNullSource(x.projectName)
	})
	if err != nil {
		if snap != nil {
			// Keep the partial result so a retry destroys only what remains.
			if serialized, serr := x.serializeNodeSnapshot(ctx, snap); serr == nil {
				st.nodeStates[name] = serialized
			}
		}
		return err
	}
	if snap != nil && len(snap.Resources) > 0 {
		return fmt.Errorf("%d resources were not deleted", len(snap.Resources))
	}
	return nil
}

// runNested executes a nested deployment against prev and returns the resulting snapshot, built by
// replaying a journal of the steps taken. The snapshot is returned (for persistence) even when
// execution fails.
func (x *workflowExecutor) runNested(
	ctx context.Context,
	prev *deploy.Snapshot,
	makeSource func(target *deploy.Target, panicErrs chan<- error) deploy.Source,
) (*deploy.Snapshot, error) {
	target := &deploy.Target{
		Name:         x.stackName,
		Organization: x.organization,
		Config:       config.Map{},
		Decrypter:    config.NopDecrypter,
		Snapshot:     prev,
	}
	// ponytail: buffered channel bounds panic reports; a hung nested deployment is not recovered here.
	panicErrs := make(chan error, 16)
	source := makeSource(target, panicErrs)

	journal := NewTestJournal()
	events := &workflowJournalEvents{journal: journal}
	opts := &deploy.Options{Parallel: x.parallel}
	depl, err := deploy.NewDeployment(
		x.plugctx, opts, events, target, prev, nil, source, x.backendClient, x.resourceHooks)
	if err != nil {
		contract.IgnoreClose(journal)
		return nil, err
	}
	_, execErr := depl.Execute(ctx)
	contract.IgnoreClose(journal)

	select {
	case perr := <-panicErrs:
		execErr = errors.Join(execErr, fmt.Errorf("panic in nested deployment: %v", perr))
	default:
	}

	snap, snapErr := journal.Snap(prev)
	if snap != nil && snap.SecretsManager == nil {
		snap.SecretsManager = b64.NewBase64SecretsManager()
	}
	return snap, errors.Join(execErr, snapErr)
}

// workflowJournalEvents adapts deploy.Events onto a TestJournal so nested deployments can rebuild
// their snapshot without a backend-attached snapshot manager.
type workflowJournalEvents struct {
	journal *TestJournal
}

func (e *workflowJournalEvents) OnSnapshotWrite(base *deploy.Snapshot) error {
	return e.journal.Write(base)
}

func (e *workflowJournalEvents) OnRebuiltBaseState() error {
	return e.journal.RebuiltBaseState()
}

func (e *workflowJournalEvents) OnResourceStepPre(step deploy.Step) (any, error) {
	return e.journal.BeginMutation(step)
}

func (e *workflowJournalEvents) OnResourceStepPost(
	ctx any, step deploy.Step, status resource.Status, err error,
) error {
	return ctx.(SnapshotMutation).End(step, err == nil || status == resource.StatusPartialFailure)
}

func (e *workflowJournalEvents) OnResourceOutputs(step deploy.Step) error {
	return e.journal.RegisterResourceOutputs(step)
}

func (e *workflowJournalEvents) OnPolicyViolation(resource.URN, plugin.AnalyzeDiagnostic) {}
func (e *workflowJournalEvents) OnPolicyRemediation(resource.URN, plugin.Remediation,
	property.Map, property.Map) {
}
func (e *workflowJournalEvents) OnPolicyAnalyzeSummary(plugin.PolicySummary)      {}
func (e *workflowJournalEvents) OnPolicyRemediateSummary(plugin.PolicySummary)    {}
func (e *workflowJournalEvents) OnPolicyAnalyzeStackSummary(plugin.PolicySummary) {}

// -- Callback plumbing --

// workflowNodeRequest is the JSON payload sent to a node-program callback. The SDK side constructs
// a fresh Pulumi context against MonitorAddr and runs the node's closure under it.
type workflowNodeRequest struct {
	MonitorAddr string            `json:"monitorAddr"`
	Project     string            `json:"project"`
	Stack       string            `json:"stack"`
	Config      map[string]string `json:"config"`
	Parallel    int32             `json:"parallel"`
}

// workflowConditionRequest is the JSON payload sent to an edge-condition callback.
type workflowConditionRequest struct {
	Data        map[string]any `json:"data"`
	When        time.Time      `json:"when"`
	Fingerprint string         `json:"fingerprint"`
}

type workflowConditionResponse struct {
	Pass bool `json:"pass"`
}

func (x *workflowExecutor) invokeCondition(ctx context.Context, cb wfCallback, c *wfCursor) (bool, error) {
	resp, err := x.invokeCallback(ctx, cb, workflowConditionRequest{
		Data:        c.data,
		When:        c.enteredAt,
		Fingerprint: stableHash(c.data),
	})
	if err != nil {
		return false, err
	}
	var r workflowConditionResponse
	if err := json.Unmarshal(resp, &r); err != nil {
		return false, fmt.Errorf("invalid condition response: %w", err)
	}
	return r.Pass, nil
}

// invokeCallback invokes a user closure over the callbacks service with a JSON payload; the
// response is JSON wrapped in a protobuf StringValue.
func (x *workflowExecutor) invokeCallback(ctx context.Context, cb wfCallback, payload any) ([]byte, error) {
	client, err := x.callbackClient(cb.Target)
	if err != nil {
		return nil, err
	}
	req, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	resp, err := client.Invoke(ctx, &pulumirpc.CallbackInvokeRequest{Token: cb.Token, Request: req})
	if err != nil {
		return nil, err
	}
	var sv wrapperspb.StringValue
	if err := proto.Unmarshal(resp.Response, &sv); err != nil {
		return nil, fmt.Errorf("invalid callback response: %w", err)
	}
	return []byte(sv.Value), nil
}

func (x *workflowExecutor) callbackClient(target string) (*deploy.CallbacksClient, error) {
	x.m.Lock()
	defer x.m.Unlock()
	if client, ok := x.callbacks[target]; ok {
		return client, nil
	}
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to callbacks service %s: %w", target, err)
	}
	client := deploy.NewCallbacksClient(conn)
	x.callbacks[target] = client
	return client, nil
}

// -- Sub-snapshot codec --

func (x *workflowExecutor) loadNodeSnapshot(
	ctx context.Context, st *wfState, name string,
) (*deploy.Snapshot, error) {
	raw, ok := st.nodeStates[name]
	if !ok {
		return nil, nil
	}
	var d3 apitype.DeploymentV3
	if err := json.Unmarshal([]byte(raw), &d3); err != nil {
		return nil, fmt.Errorf("unmarshaling node %q sub-state: %w", name, err)
	}
	snap, err := stack.DeserializeDeploymentV3(ctx, d3, b64.Base64SecretsProvider)
	if err != nil {
		return nil, fmt.Errorf("deserializing node %q sub-state: %w", name, err)
	}
	return snap, nil
}

func (x *workflowExecutor) serializeNodeSnapshot(ctx context.Context, snap *deploy.Snapshot) (string, error) {
	d3, err := stack.SerializeDeployment(ctx, snap, false /*showSecrets*/)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(d3)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// workflowStackOutputs extracts the root stack outputs from a node's snapshot.
func workflowStackOutputs(snap *deploy.Snapshot) map[string]any {
	if snap == nil {
		return nil
	}
	for _, res := range snap.Resources {
		if res.Type == resource.RootStackType && res.Parent == "" {
			return res.Outputs.Mappable()
		}
	}
	return nil
}

// workflowNodeConfig renders cursor data as config for a node program, namespaced under "workflow:".
// Non-string values are JSON-encoded, matching how structured config is delivered to programs.
func workflowNodeConfig(data map[string]any) map[string]string {
	out := make(map[string]string, len(data))
	for k, v := range data {
		if s, ok := v.(string); ok {
			out["workflow:"+k] = s
			continue
		}
		b, err := json.Marshal(v)
		if err != nil {
			b = []byte(fmt.Sprintf("%v", v))
		}
		out["workflow:"+k] = string(b)
	}
	return out
}

// stableHash returns a deterministic hash of a JSON-able value: encoding/json sorts map keys, so
// equal maps hash equal.
func stableHash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", v))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sortCursors(cs []*wfCursor) {
	sort.Slice(cs, func(i, j int) bool { return cs[i].id < cs[j].id })
}

// -- State codec: PropertyMap <-> wfGraph/wfState --

func parseWorkflowGraph(news resource.PropertyMap) (*wfGraph, error) {
	g := &wfGraph{
		nodes:   map[string]wfCallback{},
		entries: map[string]map[string]any{},
	}

	nodes, err := propObject(news, "nodes")
	if err != nil {
		return nil, err
	}
	for name, v := range nodes {
		cb, err := parseCallback(v)
		if err != nil {
			return nil, fmt.Errorf("node %q: %w", name, err)
		}
		g.nodes[string(name)] = cb
	}

	if edges, ok := news["edges"]; ok {
		if !edges.IsArray() {
			return nil, errors.New(`property "edges" must be an array`)
		}
		for i, v := range edges.ArrayValue() {
			if !v.IsObject() {
				return nil, fmt.Errorf("edge %d: must be an object", i)
			}
			o := v.ObjectValue()
			from, err := propString(o, "from")
			if err != nil {
				return nil, fmt.Errorf("edge %d: %w", i, err)
			}
			to, err := propString(o, "to")
			if err != nil {
				return nil, fmt.Errorf("edge %d: %w", i, err)
			}
			cond, err := parseCallback(resource.NewProperty(o))
			if err != nil {
				return nil, fmt.Errorf("edge %d: %w", i, err)
			}
			if _, ok := g.nodes[from]; !ok {
				return nil, fmt.Errorf("edge %d: unknown node %q", i, from)
			}
			if _, ok := g.nodes[to]; !ok {
				return nil, fmt.Errorf("edge %d: unknown node %q", i, to)
			}
			g.edges = append(g.edges, wfEdge{From: from, To: to, Cond: cond})
		}
	}

	if entries, ok := news["entries"]; ok {
		if !entries.IsObject() {
			return nil, errors.New(`property "entries" must be an object`)
		}
		for node, seed := range entries.ObjectValue() {
			if _, ok := g.nodes[string(node)]; !ok {
				return nil, fmt.Errorf("entry at unknown node %q", node)
			}
			if !seed.IsObject() {
				return nil, fmt.Errorf("entry seed for node %q must be an object", node)
			}
			g.entries[string(node)] = seed.ObjectValue().Mappable()
		}
	}

	return g, nil
}

func parseCallback(v resource.PropertyValue) (wfCallback, error) {
	if !v.IsObject() {
		return wfCallback{}, errors.New("callback must be an object")
	}
	o := v.ObjectValue()
	target, err := propString(o, "target")
	if err != nil {
		return wfCallback{}, err
	}
	token, err := propString(o, "token")
	if err != nil {
		return wfCallback{}, err
	}
	return wfCallback{Target: target, Token: token}, nil
}

func parseWorkflowState(olds resource.PropertyMap) (*wfState, error) {
	st := &wfState{
		entrySeeds: map[string]string{},
		nodeStates: map[string]string{},
	}
	if olds == nil {
		return st, nil
	}

	if v, ok := olds["cursors"]; ok && v.IsArray() {
		for i, cv := range v.ArrayValue() {
			if !cv.IsObject() {
				return nil, fmt.Errorf("cursor %d: must be an object", i)
			}
			o := cv.ObjectValue()
			node, err := propString(o, "node")
			if err != nil {
				return nil, fmt.Errorf("cursor %d: %w", i, err)
			}
			idv, ok := o["id"]
			if !ok || !idv.IsNumber() {
				return nil, fmt.Errorf("cursor %d: missing numeric id", i)
			}
			c := &wfCursor{id: int64(idv.NumberValue()), node: node}
			if dv, ok := o["data"]; ok && dv.IsObject() {
				c.data = dv.ObjectValue().Mappable()
			} else {
				c.data = map[string]any{}
			}
			if tv, ok := o["enteredAt"]; ok && tv.IsString() {
				t, err := time.Parse(time.RFC3339Nano, tv.StringValue())
				if err != nil {
					return nil, fmt.Errorf("cursor %d: invalid enteredAt: %w", i, err)
				}
				c.enteredAt = t
			}
			st.cursors = append(st.cursors, c)
			if c.id > st.nextID {
				st.nextID = c.id
			}
		}
	}
	if v, ok := olds["entrySeeds"]; ok && v.IsObject() {
		for k, sv := range v.ObjectValue() {
			if sv.IsString() {
				st.entrySeeds[string(k)] = sv.StringValue()
			}
		}
	}
	if v, ok := olds["nodeStates"]; ok && v.IsObject() {
		for k, sv := range v.ObjectValue() {
			if sv.IsString() {
				st.nodeStates[string(k)] = sv.StringValue()
			}
		}
	}
	return st, nil
}

func renderWorkflowState(news resource.PropertyMap, st *wfState) resource.PropertyMap {
	outs := resource.PropertyMap{}
	maps.Copy(outs, news)

	cursors := make([]resource.PropertyValue, len(st.cursors))
	for i, c := range st.cursors {
		cursors[i] = resource.NewProperty(resource.PropertyMap{
			"id":        resource.NewProperty(float64(c.id)),
			"node":      resource.NewProperty(c.node),
			"data":      resource.NewProperty(resource.NewPropertyMapFromMap(c.data)),
			"enteredAt": resource.NewProperty(c.enteredAt.Format(time.RFC3339Nano)),
		})
	}
	outs["cursors"] = resource.NewProperty(cursors)

	seeds := resource.PropertyMap{}
	for k, v := range st.entrySeeds {
		seeds[resource.PropertyKey(k)] = resource.NewProperty(v)
	}
	outs["entrySeeds"] = resource.NewProperty(seeds)

	states := resource.PropertyMap{}
	for k, v := range st.nodeStates {
		states[resource.PropertyKey(k)] = resource.NewProperty(v)
	}
	outs["nodeStates"] = resource.NewProperty(states)

	return outs
}

func propObject(m resource.PropertyMap, key string) (resource.PropertyMap, error) {
	v, ok := m[resource.PropertyKey(key)]
	if !ok {
		return nil, fmt.Errorf("missing required property %q", key)
	}
	if !v.IsObject() {
		return nil, fmt.Errorf("property %q must be an object", key)
	}
	return v.ObjectValue(), nil
}

func propString(m resource.PropertyMap, key string) (string, error) {
	v, ok := m[resource.PropertyKey(key)]
	if !ok {
		return "", fmt.Errorf("missing required property %q", key)
	}
	if !v.IsString() {
		return "", fmt.Errorf("property %q must be a string", key)
	}
	return v.StringValue(), nil
}
