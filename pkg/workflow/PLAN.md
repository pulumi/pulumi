# Workflow as a Pulumi resource — plan

Status: design agreed 2026-08-21, no code written yet.

Project the `pkg/workflow` package (built on `pkg/util/fsa`) as a built-in
resource that Pulumi programs declare in any language. The engine is the core
consumer of `pkg/workflow`; node programs and edge conditions are closures in
the user's program reached through the callbacks facility.

## 1. Example (Go)

```go
image, err := docker.GetDeploy(ctx, ...) // SHA of the latest artifact
if err != nil {
    return err
}

_, err = pulumi.NewWorkflow(ctx, "pipeline", func(w *workflow.Context) error {
    ci := w.Node("ci", nil) // waypoint: no program
    w.Cursor(ci, "release", pulumi.Map{"sha": image.SHA})

    deploy := func(stage string) workflow.NodeFunc {
        return func(ctx *pulumi.Context, c *workflow.Cursor) error {
            endpoint, err := doDeploy(ctx, stage, c.Require[string]("sha"))
            if err != nil {
                return err
            }
            c.Set("endpoint", endpoint) // Output, resolved when the program ends
            return nil
        }
    }
    smokeTests := func(ctx context.Context, c *workflow.Cursor) (bool, error) {
        return checkDeployment(ctx, c.Require[string]("endpoint")) == 200, nil
    }

    dev := w.Node("dev", deploy("dev"))
    staging := w.Node("staging", deploy("staging"))
    prod := w.Node("prod", deploy("prod"))

    w.Edge("nightly", ci, dev, func(ctx context.Context, c *workflow.Cursor) (bool, error) {
        return time.Since(w.AtNode(ctx, dev).LastRun) > 24*time.Hour, nil
    })
    w.Edge("smoke tests", dev, staging, smokeTests)
    w.And("prod gate", staging, prod, workflow.EdgeMap{
        "smoke tests":     smokeTests,
        "manual approval": approval("Deploy to production"), // user-defined for now, see §7
    })
    w.Or("rollback", prod, prod, workflow.EdgeMap{
        "break glass": approval("Rollback prod to previous!"),
        "post deploy smoke tests": func(ctx context.Context, c *workflow.Cursor) (bool, error) {
            ok, err := smokeTests(ctx, c)
            return !ok, err
        },
    }.OnPass(func(ctx context.Context, c *workflow.Cursor) error {
        prev, ok := w.AtNode(ctx, prod).Previous()
        if !ok {
            return errors.New("nothing to roll back to")
        }
        c.Set("sha", prev.Outputs["sha"])
        return nil
    }))
    return nil
})
```

## 2. SDK surface (`sdk/go/pulumi/workflow`)

```go
// Registers the workflow resource. def runs on every up/preview and only
// describes the graph; registration is what touches state.
func pulumi.NewWorkflow(ctx *pulumi.Context, name string,
    def func(*workflow.Context) error, opts ...pulumi.ResourceOption) (*pulumi.Workflow, error)

type Context struct{ /* unexported */ }
func (w *Context) Node(name string, run NodeFunc) Node              // run == nil → waypoint
func (w *Context) Cursor(at Node, name string, inputs pulumi.Map)   // declarative entry via a "<name>-entry" waypoint (§3)
func (w *Context) CursorAt(at Node, name string, inputs pulumi.Map) // declarative entry placed directly on at (§3)
func (w *Context) Edge(name string, from, to Node, cond EdgeFunc)
func (w *Context) And (name string, from, to Node, conds EdgeMap)
func (w *Context) Or  (name string, from, to Node, conds EdgeMap)
func (w *Context) Join(name string, from JoinMap, to Node, merge MergeFunc)
func (w *Context) AtNode(ctx context.Context, n Node) NodeState     // snapshot carried by the callback ctx

type NodeFunc  = func(ctx *pulumi.Context, c *Cursor) error        // nested program
type EdgeFunc  = func(ctx context.Context, c *Cursor) (bool, error)
type PassFunc  = func(ctx context.Context, c *Cursor) error
type MergeFunc = func(ctx context.Context, in []Candidate) (*Merged, error) // nil ⇒ reject, cursors stay

type EdgeMap map[string]EdgeFunc
func (m EdgeMap) OnPass(f PassFunc) EdgeMap   // wraps each condition: f runs after a condition returns true
type JoinMap map[Node]EdgeFunc                // nil ⇒ always
type Candidate struct{ From Node; Cursor *Cursor }
type Merged    struct{ Name string; Values map[string]any }

type Cursor struct{ /* unexported */ }
func (c *Cursor) Name() string
func (c *Cursor) Get[T any](key string) (T, bool)   // //go:build go1.27; (any, bool) otherwise
func (c *Cursor) Require[T any](key string) T       // //go:build go1.27; any otherwise; panics if missing
func (c *Cursor) Set(key string, v any)             // plain Go value or pulumi.Input
func (c *Cursor) Delete(key string)

type NodeState struct {
    Occupant *string   // cursor id, nil if empty
    History  []Visit   // newest first, unbounded
    LastRun  time.Time // zero if the program never ran
}
func (s NodeState) Previous() (Visit, bool)   // History[1]
type Visit struct {
    Cursor        string
    Entered, Left time.Time        // Left zero while occupied
    Inputs        map[string]any   // values on entry (immutable)
    Outputs       map[string]any   // values after the node program
    Err           string
}
```

Notes:
- `Get`/`Require` are generic methods on Go ≥ 1.27 (language change in 1.27;
  a `//go:build go1.27` file constraint raises the per-file language version
  without bumping `sdk/go.mod`). `cursor_go127.go` / `cursor_go126.go`
  (`//go:build !go1.27`) hold the two declarations.
- `Set` accepts an `Output` only inside a `NodeFunc` (resolved when the
  program ends, secretness preserved); in edge/pass/merge callbacks an Output
  is a callback error. `Set` never panics in an edge.
- There is no user-visible view type; `AtNode` reads the snapshot the engine
  attached to the callback's context. In a `NodeFunc`: `w.AtNode(ctx.Context(), n)`.
- Other SDKs mirror this shape; node functions are async functions run under a
  nested monitor (TS: AsyncLocalStorage settings, Python: contextvars — to be
  verified before committing).

## 3. Semantics

**Cursor immutability.** A cursor sitting on a node is immutable: `entered` =
the from-node's `outgoing` + the crossing edge's overlay (+ `OnPass`). The
workflow resource stores `{id, node, entered}` per cursor; the node resource
stores it as the occupant's visit.

**Node program = `entered → outgoing`.** It runs on arrival and again on every
`pulumi up` while the cursor stays (reconcile), always from the same
`entered`. `outgoing` (the `Set`s) is persisted on the visit and overwritten
by each run. Edges leaving the node read `outgoing` — fresh within the same
Progress, last run's value across ups. So `c.Set("n", c.Require[int]("n")+1)`
yields `n+1` in `outgoing` on every run and `n+2` only after re-entering the
node via a self-loop.

**Reconcile after the cursor leaves.** Every node that has ever been visited
keeps reconciling on every up from its last visit's `entered`
(`History[0].Inputs`), including visits that ended in error (so failures
retry). Its `Set`s are discarded: nothing downstream can change a node's state.

**Edge overlays.** `Set` in an edge/pass/merge callback writes a
per-invocation overlay, visible to later conditions of the same composite
edge and to `OnPass`, and applied only as part of the move across that edge
caused by that invocation; otherwise discarded.
- `And`: conditions run in parallel, each on its own snapshot; any false fails
  the edge; overlays merge with precedence by sorted condition name (Go maps
  have no literal order).
- `Or`: sequential in sorted name order, first true wins, only its overlay survives.
- `Join`: each branch's overlay lands on its `Candidate`; `Merged.Values`
  override branch overlays, which merge by sorted node name.
- `EdgeMap.OnPass(f)`: `f` runs after each condition that returns true; its
  `Set`s ride that condition's overlay, so under `And` it may run N times and
  is discarded if the edge fails.

**Entries are declarative.** The engine diffs resolved inputs against the
last-placed value for `name`. Changed or new ⇒ a placement (FSA replacement
rules apply; `CursorReplaced` if an occupant is overwritten). Unchanged ⇒
no-op. Placement is not an arrival: the placed node's program first runs when
the workflow reconciles, after this up's edges were asked. `w.Cursor(node,
name, inputs)` is SDK sugar that recovers arrival semantics: it places the
cursor on a synthetic `"<name>-entry"` waypoint with an always-passing `enter`
edge into `node`, so the cursor *moves* into `node` — running its program
before its outgoing edges — within the same up. `w.CursorAt(node, name,
inputs)` places directly on `node`. Removed from the program ⇒ the
cursor it placed keeps going. Cursor ids are `name#generation`; merged cursors
take `Merged.Name` with a fresh generation. Unknown inputs are rejected in `up`.

**Failure.** FSA semantics: a node/edge/merge error aborts the up, in-flight
nodes finish and commit, state is persisted, next up resumes from persisted
cursors. Reconcile errors fail like a resource update and retry next up.

**Undefined nodes.** A node in state but not in the program: its subgraph and
node resource are deleted; cursors on it stay in workflow state with a warning
and resume if the node is redefined (`NodeUndefined`-and-continue).

**preview.** No callbacks, no cursor moves; render the persisted diagram as a
normal info diagnostic and mark entries whose inputs changed.

**destroy / refresh.** Destroy: nested resources in reverse, node resources,
then the workflow. Refresh: nested resources refresh normally; workflow/node
resources are no-ops.

**Nested workflows** fall out of normal arrival/reconcile: a workflow
registered inside a node program progresses whenever that node runs.

## 4. Resource model

- `pulumi:index:Workflow` (builtin provider). Inputs: nodes and edges
  (callback tokens, condition names, kinds), entries. Outputs: cursors
  `[{id, node, entered}]`, node records, the diagram (a PoC affordance —
  the diagram as a resource output does not go into prod).
- `pulumi:index:WorkflowNode`, child of the workflow, one per defined node.
  State: occupant, visits (`entered`/`outgoing`/times/err), `lastRun`. The
  node program's resources are its children; each run is a nested deployment
  sharing the outer snapshot.

## 5. One `pulumi up` = one `Progress`

1. Program registers the workflow (callbacks registered with the language
   host's callback server; graph + entries sent as inputs).
2. The workflow's Create/Update step commits its state; then, before the
   program's registration completes, the engine runs `Progress`. Each cursor
   movement runs the node program as a nested deployment that registers the
   node resource under the workflow and the program's resources under the
   node; conditions/merges are callbacks. The program receives this run's
   cursors/diagram as the resource's outputs.
3. After Progress, every existing (visited at least once) node that did not
   run during this Progress is reconciled: its program is run as a real
   nested deployment in normal mode against its subgraph, in parallel across
   nodes, producing whatever steps reconciliation yields. There is no `Same`
   shortcut; the run is what keeps children alive. Nodes that ran during
   Progress are skipped; never-visited nodes and waypoints get a nested
   deployment that registers just their node resource.
4. Workflow outputs (state, node records, cursors, diagram) recorded.

## 6. Protocol

New `proto/pulumi/workflow.proto`, carried over the existing `Callback`
service (same mechanism as transforms):
- `NodeRequest{monitor_addr, project, stack, config, parallel, cursor{id, values}, view, reconcile}` → `NodeResponse{outgoing}`
- `ConditionRequest{cursor, view, edge, condition}` → `ConditionResponse{pass, overlay}`
- `PassRequest{cursor, view, edge, condition}` → `PassResponse{overlay}` (if not folded into the SDK wrapper)
- `MergeRequest{candidates[{from, cursor}], view, edge}` → `MergeResponse{merged?{name, values}}`
- `view`: per-node `NodeState` snapshot taken when the callback is dispatched.
Callback payloads always carry edge and condition names.

## 7. Approval

Out of band via the Service, API to be defined later. For now no
`workflow.Approval` in the SDK; examples/tests define an `EdgeFunc` that reads
a local JSON file keyed by `(workflow, edge, condition, cursor id)`. Approvals
bind to the cursor id; since cursors are immutable at a node, a grant stays
valid until the cursor moves.

## 8. Display

`pkg/workflow/display` folds the event stream into an ephemeral info
diagnostic (`LogRequest.ephemeral`) on the workflow URN, re-emitted on every
event so it updates within a Progress:
- ASCII graph: nodes with occupant label, edges by name.
- One line per cursor: `release#4 @ staging — checking "prod gate"/"manual approval"`
  or `release#3 @ prod — parked` ("checking" = between `EdgeStarted` and
  `EdgeFinished`; conditions of a single edge may run in parallel, but a
  cursor checks one edge at a time).
The engine also emits the structured `pkg/workflow` events in the event
stream so the Service can render its own view.

## 9. Implied `pkg/workflow` changes

- `And` evaluates in parallel with overlay merge; `Or` stays sequential.
- Split today's `pending` into `entered` (immutable) and `outgoing`
  (overwritten per run); edges read `outgoing`.
- Reconcile hook: run the node function for every visited node whose
  function did not run in this Progress (replaces the consumer's use of
  `NodeUntouched`).
- Cursor ids `name#gen`; per-node visit history in state.
- Callback-facing payloads carry edge/condition names and the view snapshot.

## 10. Decision log

| # | Decision |
|---|----------|
| 1 | Diagram via temporary (ephemeral) info diagnostic; `pkg/workflow/display` package. |
| 2 | History unbounded for now. |
| 3 | `Set` in an edge never panics; overlay semantics above. |
| 4 | No `Fingerprint()`/`Values()`; generic `Get`/`Require` behind `go1.27` build tags. |
| 5 | No exported view type; `AtNode(ctx, n)` reads the snapshot from ctx. |
| 6 | Entries declarative; node programs reconcile every up from the entered cursor. |
| 7 | Reconcile failure = resource update failure; retried next up. |
| 8 | `And` parallel, precedence by sorted name; `Or` first-true; `Join` like `And` + merge override. |
| 9 | Approval keyed on cursor id; Service API later; JSON-file `EdgeFunc` for now. |
| 10 | `OnPass` is a method on `EdgeMap` wrapping each condition. |
| 11 | preview runs no callbacks. |
| 12 | Nested workflows need nothing special. |
| 13 | Reconcile = full normal-mode nested deployment per existing untouched node; no `Same` shortcut. |
