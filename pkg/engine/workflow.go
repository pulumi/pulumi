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
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
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

const workflowResourceType = "pulumi:index:Workflow"

// workflowProgressor implements deploy.WorkflowProgressor: it advances every workflow the
// deployment registered by driving the fsa scheduler with nested, scoped deployments. Node
// programs and edge conditions are closures in the user's program, reached through the callbacks
// facility. Node resources live in the main snapshot, marked with an owner (workflow URN + node)
// and namespaced by a per-node project qualifier; nested deployments share the outer deployment's
// events/snapshot manager, so their steps persist and display like any other.
type workflowProgressor struct {
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

func newWorkflowProgressor(
	plugctx *plugin.Context,
	backendClient deploy.BackendClient,
	resourceHooks *deploy.ResourceHooks,
	stackName tokens.StackName,
	organization tokens.Name,
	projectName tokens.PackageName,
	parallel int32,
) *workflowProgressor {
	return &workflowProgressor{
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

func (x *workflowProgressor) Progress(ctx context.Context, d *deploy.Deployment,
	persistState func(urn resource.URN, outputs resource.PropertyMap) error,
) error {
	var wfs []*pkgresource.State
	d.News().Range(func(_ resource.URN, s *pkgresource.State) bool {
		if s.Type == workflowResourceType {
			wfs = append(wfs, s)
		}
		return true
	})
	slices.SortFunc(wfs, func(a, b *pkgresource.State) int {
		return strings.Compare(string(a.URN), string(b.URN))
	})

	var errs []error
	for _, wf := range wfs {
		if err := x.progressWorkflow(ctx, d, wf, persistState); err != nil {
			errs = append(errs, fmt.Errorf("workflow %v: %w", wf.URN, err))
		}
	}
	return errors.Join(errs...)
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
	nextID     int64
}

// workflowRun is the in-flight state of progressing one workflow within one deployment.
type workflowRun struct {
	x  *workflowProgressor
	d  *deploy.Deployment
	wf *pkgresource.State
	g  *wfGraph
	st *wfState

	// m guards slices: during Progress, node deployments run on concurrent goroutines and each
	// replaces its slice as it completes.
	m sync.Mutex
	// slices holds the current resource states owned by each node, initialized from the previous
	// snapshot and replaced by each nested deployment's result.
	slices map[string][]*pkgresource.State
}

func (x *workflowProgressor) progressWorkflow(
	ctx context.Context, d *deploy.Deployment, wf *pkgresource.State,
	persistState func(urn resource.URN, outputs resource.PropertyMap) error,
) error {
	g, err := parseWorkflowGraph(wf.Inputs)
	if err != nil {
		return err
	}
	wf.Lock.Lock()
	st, err := parseWorkflowState(wf.Outputs)
	wf.Lock.Unlock()
	if err != nil {
		return err
	}

	run := &workflowRun{x: x, d: d, wf: wf, g: g, st: st, slices: map[string][]*pkgresource.State{}}
	if prev := d.Prev(); prev != nil {
		for _, res := range prev.Resources {
			if owner, node, ok := deploy.ParseWorkflowOwner(res.Owner); ok && owner == wf.URN {
				run.slices[node] = append(run.slices[node], res)
			}
		}
	}

	info := func(f string, a ...any) {
		x.plugctx.Diag.Infof(diag.Message(wf.URN, "workflow: "+f), a...)
	}
	warn := func(f string, a ...any) {
		x.plugctx.Diag.Warningf(diag.Message(wf.URN, "workflow: "+f), a...)
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
		outs, err := run.runNode(ctx, c.node, c.data)
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
		final, err := run.progress(ctx, alive, info)
		if err != nil {
			errs = append(errs, err)
		}
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
		st.cursors = append(alive, doomed...)
		doomed = nil
	}

	// 4. GC, after everything else: nodes with owned resources but no definition are destroyed,
	// and any resident cursor is deleted with its node.
	if len(errs) == 0 {
		for _, name := range slices.Sorted(maps.Keys(run.slices)) {
			if _, ok := g.nodes[name]; ok {
				continue
			}
			if err := run.destroyNode(ctx, name); err != nil {
				errs = append(errs, fmt.Errorf("destroying removed node %q: %w", name, err))
				continue
			}
			delete(run.slices, name)
			info("destroyed removed node %q", name)
		}
		for _, c := range doomed {
			warn("deleted cursor %d along with removed node %q", c.id, c.node)
		}
	}

	// Persist the workflow's durable state — cursor positions survive even a failed run — through
	// the executor's regular resource-outputs path.
	wf.Lock.Lock()
	outputs := wf.Outputs.Copy()
	wf.Lock.Unlock()
	outputs["cursors"] = renderCursors(st.cursors)
	outputs["entrySeeds"] = renderEntrySeeds(st.entrySeeds)
	if err := persistState(wf.URN, outputs); err != nil {
		errs = append(errs, fmt.Errorf("persisting workflow state: %w", err))
	}

	return errors.Join(errs...)
}

// progress runs one fsa.Progress call over the workflow graph and returns the surviving cursors.
func (run *workflowRun) progress(
	ctx context.Context, cursors []*wfCursor, info func(string, ...any),
) ([]*wfCursor, error) {
	m := fsa.New[*wfCursor]()
	fsaNodes := make(map[string]fsa.Node, len(run.g.nodes))
	nodeNames := make(map[fsa.Node]string, len(run.g.nodes))
	for _, name := range slices.Sorted(maps.Keys(run.g.nodes)) {
		n := m.NewNode(func(fctx context.Context, _ fsa.FSA[*wfCursor], _ fsa.Edge, c *wfCursor) error {
			outs, err := run.runNode(fctx, name, c.data)
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
	for _, e := range run.g.edges {
		m.NewEdge(func(fctx context.Context, _ fsa.FSA[*wfCursor], c *wfCursor) (fsa.ConditionResult, error) {
			pass, err := run.x.invokeCondition(fctx, e.Cond, c)
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

	// Independent cursors evaluate conditions and deploy their nodes in parallel. The FSA
	// serializes all per-cursor and per-node-entry work itself; the only cross-goroutine state is
	// workflowRun.slices, which is locked.
	asyncRunner := func(ctx context.Context, f func(context.Context)) error {
		go f(ctx)
		return nil
	}
	err := m.Progress(ctx, asyncRunner)

	var final []*wfCursor
	m.Cursors(func(c *wfCursor, n fsa.Node) bool {
		c.node = nodeNames[n]
		final = append(final, c)
		return true
	})
	sortCursors(final)
	return final, err
}

// sliceProject namespaces a node's resources within the shared snapshot: URNs embed the project,
// so per-node projects make slice URNs (including default providers and the node's root stack)
// unique by construction.
func (run *workflowRun) sliceProject(node string) tokens.PackageName {
	return tokens.PackageName(fmt.Sprintf("%s-wf-%s-%s", run.x.projectName, run.wf.URN.Name(), node))
}

func (run *workflowRun) sliceSnapshot(node string) *deploy.Snapshot {
	run.m.Lock()
	resources := run.slices[node]
	run.m.Unlock()
	if len(resources) == 0 {
		return nil
	}
	manifest := deploy.Manifest{}
	manifest.Magic = manifest.NewMagic()
	var sm secrets.Manager
	var extensions map[apitype.ExtensionRef]apitype.Extension
	if prev := run.d.Prev(); prev != nil {
		sm = prev.SecretsManager
		extensions = prev.Extensions
	}
	return deploy.NewSnapshot(manifest, sm, resources, nil, deploy.SnapshotMetadata{}, nil, extensions)
}

// runNode reconciles one node by running its program as a nested deployment scoped to the node's
// slice, sharing the outer deployment's events (persistence and display). Returns the node's stack
// outputs. The slice is updated even when the deployment fails, so a later up heals what committed.
func (run *workflowRun) runNode(ctx context.Context, node string, data map[string]any) (map[string]any, error) {
	x := run.x
	cb := run.g.nodes[node]
	project := run.sliceProject(node)
	runner := func(monitorAddr string) *promise.Promise[struct{}] {
		cs := &promise.CompletionSource[struct{}]{}
		go func() {
			payload := workflowNodeRequest{
				MonitorAddr: monitorAddr,
				Project:     string(project),
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

	newSlice, err := run.runNested(ctx, node, func(target *deploy.Target, panicErrs chan<- error) deploy.Source {
		runinfo := &deploy.EvalRunInfo{
			Proj: &workspace.Project{
				Name:    project,
				Runtime: workspace.NewProjectRuntimeInfo("workflow-node", nil),
			},
			Pwd:     x.plugctx.Pwd,
			Program: ".",
			Target:  target,
		}
		return deploy.NewEvalSource(x.plugctx, runinfo, nil, x.resourceHooks,
			deploy.EvalSourceOptions{Parallel: x.parallel}, panicErrs, nil, runner)
	})
	if err != nil {
		return nil, err
	}
	for _, res := range newSlice {
		if res.Type == resource.RootStackType && res.Parent == "" {
			res.Lock.Lock()
			defer res.Lock.Unlock()
			return res.Outputs.Mappable(), nil
		}
	}
	return nil, nil
}

// destroyNode tears down a node's slice by running a nested deployment with an empty program.
func (run *workflowRun) destroyNode(ctx context.Context, node string) error {
	remaining, err := run.runNested(ctx, node, func(*deploy.Target, chan<- error) deploy.Source {
		return deploy.NewNullSource(run.sliceProject(node))
	})
	if err != nil {
		return err
	}
	if len(remaining) > 0 {
		return fmt.Errorf("%d resources were not deleted", len(remaining))
	}
	return nil
}

// runNested executes a nested deployment over node's slice. Steps flow through the outer
// deployment's events — persisting into the shared snapshot and rendering in the display — and are
// also journaled locally to compute the slice's resulting resources, which replace the slice (even
// on failure, so retries see what committed).
func (run *workflowRun) runNested(
	ctx context.Context,
	node string,
	makeSource func(target *deploy.Target, panicErrs chan<- error) deploy.Source,
) ([]*pkgresource.State, error) {
	x := run.x
	prev := run.sliceSnapshot(node)
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
	events := &workflowTeeEvents{outer: run.d.Events(), journal: journal}
	opts := &deploy.Options{
		Parallel: x.parallel,
		Owner:    deploy.MakeWorkflowOwner(run.wf.URN, node),
	}
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
	var resources []*pkgresource.State
	if snap != nil {
		resources = snap.Resources
		run.m.Lock()
		run.slices[node] = resources
		run.m.Unlock()
	}
	return resources, errors.Join(execErr, snapErr)
}

// workflowTeeEvents forwards nested-deployment events to the outer deployment's events (shared
// snapshot manager and display) while journaling steps locally to rebuild the slice's resources.
type workflowTeeEvents struct {
	outer   deploy.Events
	journal *TestJournal
}

type workflowTeeMutation struct {
	outer   any
	journal SnapshotMutation
}

func (t *workflowTeeEvents) OnSnapshotWrite(base *deploy.Snapshot) error {
	return errors.Join(t.outer.OnSnapshotWrite(base), t.journal.Write(base))
}

func (t *workflowTeeEvents) OnRebuiltBaseState() error {
	return errors.Join(t.outer.OnRebuiltBaseState(), t.journal.RebuiltBaseState())
}

func (t *workflowTeeEvents) OnResourceStepPre(step deploy.Step) (any, error) {
	octx, err := t.outer.OnResourceStepPre(step)
	if err != nil {
		return nil, err
	}
	jctx, err := t.journal.BeginMutation(step)
	if err != nil {
		return nil, err
	}
	return workflowTeeMutation{outer: octx, journal: jctx}, nil
}

func (t *workflowTeeEvents) OnResourceStepPost(
	ctx any, step deploy.Step, status resource.Status, err error,
) error {
	tee := ctx.(workflowTeeMutation)
	return errors.Join(
		t.outer.OnResourceStepPost(tee.outer, step, status, err),
		tee.journal.End(step, err == nil || status == resource.StatusPartialFailure),
	)
}

func (t *workflowTeeEvents) OnResourceOutputs(step deploy.Step) error {
	return errors.Join(t.outer.OnResourceOutputs(step), t.journal.RegisterResourceOutputs(step))
}

func (t *workflowTeeEvents) OnPolicyViolation(urn resource.URN, d plugin.AnalyzeDiagnostic) {
	t.outer.OnPolicyViolation(urn, d)
}

func (t *workflowTeeEvents) OnPolicyRemediation(urn resource.URN, r plugin.Remediation,
	before, after property.Map,
) {
	t.outer.OnPolicyRemediation(urn, r, before, after)
}

func (t *workflowTeeEvents) OnPolicyAnalyzeSummary(s plugin.PolicySummary) {
	t.outer.OnPolicyAnalyzeSummary(s)
}

func (t *workflowTeeEvents) OnPolicyRemediateSummary(s plugin.PolicySummary) {
	t.outer.OnPolicyRemediateSummary(s)
}

func (t *workflowTeeEvents) OnPolicyAnalyzeStackSummary(s plugin.PolicySummary) {
	t.outer.OnPolicyAnalyzeStackSummary(s)
}

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

func (x *workflowProgressor) invokeCondition(ctx context.Context, cb wfCallback, c *wfCursor) (bool, error) {
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
func (x *workflowProgressor) invokeCallback(ctx context.Context, cb wfCallback, payload any) ([]byte, error) {
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

func (x *workflowProgressor) callbackClient(target string) (*deploy.CallbacksClient, error) {
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

// workflowNodeName constrains node names: they become part of owner markings ('#'-separated) and
// slice project names, so keep them to a safe identifier charset.
var workflowNodeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

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
		if !workflowNodeName.MatchString(string(name)) {
			return nil, fmt.Errorf("invalid node name %q", name)
		}
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
	return st, nil
}

func renderCursors(cursors []*wfCursor) resource.PropertyValue {
	rendered := make([]resource.PropertyValue, len(cursors))
	for i, c := range cursors {
		rendered[i] = resource.NewProperty(resource.PropertyMap{
			"id":        resource.NewProperty(float64(c.id)),
			"node":      resource.NewProperty(c.node),
			"data":      resource.NewProperty(resource.NewPropertyMapFromMap(c.data)),
			"enteredAt": resource.NewProperty(c.enteredAt.Format(time.RFC3339Nano)),
		})
	}
	return resource.NewProperty(rendered)
}

func renderEntrySeeds(seeds map[string]string) resource.PropertyValue {
	rendered := resource.PropertyMap{}
	for k, v := range seeds {
		rendered[resource.PropertyKey(k)] = resource.NewProperty(v)
	}
	return resource.NewProperty(rendered)
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
