# `pulumi.Workflow`: durable workflows as a Pulumi resource

Status: draft — desired semantics only. No implementation is implied by this document.

A workflow lets a Pulumi program express a long-lived process as a graph. Each **node** is a mini
Pulumi program with its own persistent sub-state. Each **edge** is an arbitrary condition function.
**Cursors** carry user data through the graph and mark execution progress. The runtime is the generic
scheduler in `pkg/util/fsa`; the engine supplies durability, sub-state reconciliation, and the
language-host callback plumbing; each language SDK supplies the authoring surface (Go first).

Related reading:
- [`pulumi:Stack` & `pulumi:MultiStack`](https://app.notion.com/p/3b2fdbdf1cce809f8660cfaaa101f479) —
  nodes will very often contain `pulumi.Stack` resources; workflows sequence *when* stacks deploy,
  `pulumi.Stack` handles *how*.
- [Delivery Scenarios](https://app.notion.com/p/3b2fdbdf1cce803faafae3db4304e7a6) — the five
  scenarios this design must serve; each is worked through below.

## The model

| Workflow concept | FSA concept | Persistence |
|---|---|---|
| `pulumi.Workflow` resource | one `FSA[Cursor]` instance | resource in the parent stack's snapshot |
| Node (`g.DefNode`) | FSA node; entry function = reconcile the node's program | a managed stack in the backend, per node |
| Edge (`g.Edge`) | FSA edge; condition function | graph shape only (conditions are code, never state) |
| Cursor | FSA cursor | `{id, node, data, enteredAt}` in the workflow's state |
| One `pulumi up` | one `Progress(ctx, runner)` call | cursor positions + node-stack references written at the end |

Key consequences of building on the FSA as implemented:

- **No self-wake.** A workflow only advances as part of a `pulumi up`. Conditions are sampled at
  most once per visit, and every `Progress` starts a fresh visit — so every `up` polls each cursor's
  untried edges exactly once, then parks whatever cannot move. Durability lives in state; liveness
  lives in whoever schedules `up` (Deployments cron/webhook, CI). Time spent waiting costs zero
  compute — a parked cursor is a row in a checkpoint.
- **First passing edge wins**, in edge-definition order. A cursor follows exactly one edge per move.
  Definition order is therefore meaningful: put the exceptional edge (rollback, exit) before the
  normal one when both could pass.
- **Joining is overwriting.** A cursor arriving at an occupied node lets the occupant escape if it
  can; a stuck occupant is deleted. Workflow meaning: a newer release *supersedes* a stalled one.
- **Two cursors concurrently moving to the same node is an error** ("race: … both moving"). The `up`
  fails; moves committed before the failure persist (commits are atomic in the loop). Re-running
  `up` re-samples conditions and resolves the pileup one cursor at a time.
- The engine's runner may dispatch node entries concurrently, so independent cursors deploy their
  nodes in parallel within one `up` (same spirit as `MultiStack`).

### Cursor data

A cursor's data is a property map: its entry seed, unioned with the stack outputs of every node it
has passed through, in path order, **later wins**. Inside a node program, `g.Config(ctx)` exposes the
entering cursor's data as ordinary config; `ctx.Export` from a node program writes the outputs that
flow into the cursor as it leaves. Secret outputs stay secret in the cursor's stored data and when
re-injected as config downstream.

`workflow.From(ctx)` (also `g.From`) exposes traversal metadata to conditions:

- `.When` — when the cursor entered the edge's source node (bake-time gates). Timestamps come from
  state, written by the engine; conditions that read wall-clock are non-reproducible between runs by
  nature. Documented behavior, fine for gates.
- `.Fingerprint` — a stable hash of the cursor's data. **Approvals bind to the fingerprint**: an
  approval granted for one fingerprint simply does not exist for another, so any change to the
  cursor's data (new sha, new plan digest) structurally invalidates prior approvals.
- `.Data()` — the moving cursor's data, read-only.

`From` is package-level so helper libraries (approval gates, alarm gates) can be written without
holding a `*Graph`.

### Entries: the one cursor-creation primitive

Each node has one **entry slot**. `g.Entry(node, seed)` declares it; the workflow's state records
the hash of the last-admitted seed per node. A seed whose hash differs from the record admits a new
cursor at the node (initial placement never runs the node's entry function) and updates the record;
an unchanged seed is a no-op. Entry records are bounded by the node count — nothing to GC.

Consequences:

- `g.Entry(dev, {"image": ciImageDigest()})` naturally admits one cursor per new digest.
- A procedure that should re-run with an *identical* payload must carry a distinguishing seed field
  (a run id, an AMI id, a date) — same hash, no new cursor.
- `Entry` is callable from anywhere, including inside node programs (dynamic fan-out). Node programs
  re-execute every `up` while occupied, so cursor creation from a node body must be idempotent —
  which the slot semantics give for free. There is no separate `Spawn`; mid-`Progress` entries are
  progressed in the same `up` (the FSA supports `NewCursor` mid-run).
- Two `Entry` calls for the same node with different seeds in one `up` is an error.
- Removing an `Entry` from code does not delete its cursor; an entry is a birth record, not a
  lifetime.

### Sub-state ("the engine will need to maintain special state")

Each node's sub-state is a **managed stack in the backend**, one per (workflow, node); the workflow
resource's state stores references to these stacks, not their contents. The parent checkpoint stays
small, node history/audit comes for free from the backend, and the console can render a node like
any stack. This cannot be plain parenting in the main snapshot: normal `up` semantics delete any
resource not re-registered, but a node's program only runs when reconciled. The engine reconciles
each node stack independently:

- **Reconciled** (inner `up` of the node's program with the resident/entering cursor's data as
  config) when a cursor **enters** the node, and once per `up` for every node currently **hosting**
  a cursor. Reconcile-while-occupied is what makes node-body and config edits converge on `up` like
  everything else in Pulumi, and it is the crash-recovery path: an `up` that died mid-entry is
  healed by plain reconciliation on the next one.
- **Destroyed** (inner destroy, then stack removal) when the node disappears from the graph
  definition, or when the workflow itself is deleted. A resident cursor does not block this: the
  removed node is destroyed and the cursor is deleted with it (with a visible warning event). Node
  removal is processed **after everything else in the `up`** — the program may register nodes at
  any point, so absence is only known once the program and `Progress` have completed.
- **Left alone** otherwise. A node a cursor has left keeps its resources until re-entered, GC'd, or
  the workflow is destroyed. (A promotion pipeline's `dev` stack stays up after promotion — that is
  the desired behavior.)

Node bodies and conditions are closures in the user's program, so the engine cannot run them from
state alone — it calls back into the language host (the existing callbacks facility used by
transforms).

## Operation semantics

**`pulumi up`**
1. The program runs; `NewWorkflow` registers the graph shape (node names, edges, entries) plus
   callback tokens for every node body and condition.
2. Reconcile every node currently hosting a cursor (inner `up` with that cursor's data).
3. Admit entries: each `g.Entry` whose seed hash differs from its node's record creates a cursor.
4. `Progress`: conditions polled once per cursor-visit; each granted move runs the target node's
   inner `up` with the mover's data (the FSA's eager entry), then commits the position.
5. Node GC, last: nodes present in state but never defined this `up` get their stacks destroyed
   and removed; resident cursors are deleted with their node.
6. Persist cursors, entry records, and node-stack references into the workflow resource's state.

**`pulumi preview`** — static (decided for v1): diff the graph shape, preview the reconcile of
currently-occupied nodes, list parked cursors and their untried edges. Conditions are **not**
evaluated and no moves are simulated. Simulated preview is deferred.

**`pulumi destroy`** (or the workflow removed from the program): destroy and remove every node's
backend stack — nodes are mutually independent (cross-node data flows by value through cursors,
never by live reference), so node destroys run in parallel; within a node, normal dependency order —
then delete the workflow resource. Cursors and entry records go with it. No orphans: this invariant
is what justifies engine-side ownership of the sub-state.

**`pulumi refresh`**: refresh the resources in every node stack. No condition is evaluated; no
cursor moves.

**`--target`**: not supported into workflow internals in v1. Targeting the workflow targets all of it.

## Go SDK surface (sketch)

```go
package workflow

type Graph struct{ /* opaque */ }
type Node struct{ /* opaque */ }

func NewWorkflow(ctx *pulumi.Context, name string, def func(g *Graph) error,
    opts ...pulumi.ResourceOption) (*Workflow, error)

func (g *Graph) DefNode(name string, program func(*pulumi.Context) error) Node
func (g *Graph) Edge(from, to Node, cond func(context.Context) (bool, error))
func (g *Graph) Entry(n Node, seed pulumi.Map)             // per-node slot; admits on seed change
func (g *Graph) Config(ctx *pulumi.Context) config.Config  // entering cursor's data, in node programs
func (g *Graph) Cursors() []CursorInfo                     // positions, for conditions
func (g *Graph) Outputs(n Node) map[string]any             // last-applied outputs of a node

func From(ctx context.Context) Traversal // in conditions; g.From aliases it
type Traversal struct {
    When        time.Time // when the cursor entered the edge's source node
    Fingerprint string    // stable hash of the cursor's data; approvals bind to this
}
func (t Traversal) Data() config.Config

var Always = func(context.Context) (bool, error) { return true, nil }
```

The condition signature `(bool, error)` maps onto the FSA's `(ConditionResult, error)`:
`true → ConditionPass`, `false → ConditionFail`, error → fail the `up`. Node programs get a real
`*pulumi.Context` bound to a nested monitor scope, so everything (stack resources, providers,
exports) works unmodified inside a node.

## Mechanics worth naming

- **Supersede.** Release 42 reaches `dev` while 41 sits parked awaiting approval: 41's conditions
  are re-polled first (an occupant that can move is guaranteed its escape — approvals granted
  between runs are never trampled); if still stuck, 41 is deleted and `dev` reconciles to 42's
  config. Emit a visible event ("cursor 41 superseded by 42 at dev"). This is the batching-at-gates
  behavior every pipeline has: only the newest release behind a gate matters, and deploying it
  subsumes the ones it swallowed.
- **Races.** Two releases passing `ci→dev` in the same `up` fail that `up`; a re-run resolves it.
  To prevent rather than tolerate, gate the edge on `g.Cursors()` ("no one at or moving to dev").
- **Cycles.** Rollback edges and loops are first-class (the FSA commits deferred cycles
  atomically). Re-entering a node starts a fresh visit, so its forward edges re-arm. Data caveat:
  cursor data is later-wins union, so a rollback path must pin the known-good version explicitly
  (e.g. each deploy node exports `lastGood` before overwriting `current`).
- **Fan-out** is `g.Entry` from a node program (see Entries). **Fan-in with data merge does not
  exist**: joining is overwrite. A condition can *wait* on other cursors' positions via
  `g.Cursors()`, and results can flow through node outputs — but two cursors never fuse into one.
  Notably, none of the five delivery scenarios needs an AND-join.
- **Failure.** A node's inner `up` fails → `Progress` fails (collateral cancellation folded into
  the first cause); committed moves persist; the failed node keeps its partial state with the
  cursor recorded mid-entry. The next `up` heals by reconciliation — no special resume machinery,
  because "re-run the program against existing state" is already Pulumi's recovery story. Process
  death mid-`up` is the same case. Recovery is roll-forward, matching IaC reality.
- **Observability.** Cursors, positions, data (secrets masked), arrival times, parked-vs-moving
  render in `pulumi stack` and the service UI from the workflow's state; each node is a real
  backend stack with its own history. Every move, supersede, park, and inner deploy emits engine
  events, so `up` output reads as a narrative ("release-42: staging → prod (approval granted)").
  "Cluster 14 of 30" is a cursor position, not a log grep.

---

## The five delivery scenarios

Each scenario from [Delivery Scenarios](https://app.notion.com/p/3b2fdbdf1cce803faafae3db4304e7a6)
as a complete Go program, with honest **cannot** notes where the model falls short. Packages under
`acme.corp/deploy/internal/…` stand in for the user's glue to CI, alarms, and approvals.

### Scenario 1: the basic service pipeline

Merge → dev (smoke) → daily-cadence staging (integration) → human approval → prod.

```go
package main

import (
	"context"
	"time"

	"acme.corp/deploy/internal/ci"  // image digests, smoke/integration results
	"acme.corp/deploy/internal/dac" // approval-service gates, fingerprint-bound
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/workflow"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := workflow.NewWorkflow(ctx, "service", func(g *workflow.Graph) error {
			deploy := func(env string) func(*pulumi.Context) error {
				return func(ctx *pulumi.Context) error {
					image := g.Config(ctx).Require("image")
					_, err := pulumi.NewStack(ctx, env, pulumi.StackArgs{
						Stack:   env,
						Project: "acme-app",
						Remote:  true,
						Config:  pulumi.StringMap{"image": pulumi.String(image)},
					})
					if err != nil {
						return err
					}
					ctx.Export("deployedAt", pulumi.String(time.Now().Format(time.RFC3339)))
					return nil
				}
			}
			dev := g.DefNode("dev", deploy("dev"))
			staging := g.DefNode("staging", deploy("staging"))
			prod := g.DefNode("prod", deploy("prod"))

			g.Edge(dev, staging, func(ctx context.Context) (bool, error) {
				// Post-deploy tests gate promotion; the daily cadence batches releases.
				ok, err := ci.SmokePassed(ctx, workflow.From(ctx).Data().Require("image"))
				if !ok || err != nil {
					return false, err
				}
				last, _ := time.Parse(time.RFC3339, g.Outputs(staging)["deployedAt"].(string))
				return time.Since(last) > 24*time.Hour, nil
			})
			g.Edge(staging, prod, dac.ServiceApproval("staging to prod"))

			g.Entry(dev, pulumi.Map{"image": pulumi.String(ci.LatestImageDigest())}) // one cursor per digest
			return nil
		})
		return err
	})
}
```

CI runs `pulumi up` on merge; a Deployments cron runs it daily. Ten merges in an afternoon = ten
cursors admitted at `dev`, each superseding the last (batching at the gate); staging holds
yesterday's cut; prod runs whatever was approved last week. `dac.ServiceApproval(name)` asks the
approval service about `workflow.From(ctx).Fingerprint` — an approval that sat open overnight
costs nothing (parked cursor), and if a different image somehow reaches the gate, the fingerprint
differs and the old approval does not apply. Redeploy-of-last-good is an `Entry` at `prod` with
the previous digest (a small CLI/console affordance over the same primitive).

**Cannot:** nothing structural. The cadence condition polls at `up` frequency, so "daily 9am"
really means "the first `up` after the window opens" — acceptable with cron-triggered runs.

### Scenario 2: the infrastructure pipeline

Chained stacks per environment, preview-pinned approval before prod, roll-forward recovery.

```go
package main

import (
	"context"

	"acme.corp/deploy/internal/ci"
	"acme.corp/deploy/internal/dac"
	"acme.corp/deploy/internal/git"
	"acme.corp/deploy/internal/plan" // previews an env's stacks, returns a digest of the diff
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/workflow"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := workflow.NewWorkflow(ctx, "infra", func(g *workflow.Graph) error {
			env := func(name string) func(*pulumi.Context) error {
				return func(ctx *pulumi.Context) error {
					sha := g.Config(ctx).Require("sha")
					layer := func(project string, deps ...pulumi.Resource) (*pulumi.Stack, error) {
						return pulumi.NewStack(ctx, project, pulumi.StackArgs{
							Stack:   name,
							Project: project,
							Remote:  true,
							Config:  pulumi.StringMap{"sha": pulumi.String(sha)},
						}, pulumi.DependsOn(deps))
					}
					// Real programs thread outputs (vpcId, dbUrl) between layers; outputs
					// resolve into the same apply ordering shown here with DependsOn.
					network, err := layer("acme-network")
					if err != nil {
						return err
					}
					data, err := layer("acme-data", network)
					if err != nil {
						return err
					}
					_, err = layer("acme-app", data)
					return err
				}
			}
			dev := g.DefNode("dev", env("dev"))
			staging := g.DefNode("staging", env("staging"))
			// The plan step is a node so its result lands in cursor data — and therefore
			// in the fingerprint the prod approval binds to.
			planProd := g.DefNode("plan-prod", func(ctx *pulumi.Context) error {
				digest, err := plan.Preview(ctx, "prod", g.Config(ctx).Require("sha"))
				if err != nil {
					return err
				}
				ctx.Export("planDigest", pulumi.String(digest))
				return nil
			})
			prod := g.DefNode("prod", env("prod"))

			g.Edge(dev, staging, func(ctx context.Context) (bool, error) {
				return ci.ValidationPassed(ctx, workflow.From(ctx).Data().Require("sha"))
			})
			g.Edge(staging, planProd, workflow.Always)
			// Reviewers approve the previewed diff. The approval is keyed by the cursor's
			// fingerprint, which now includes planDigest: if another change lands (new sha,
			// therefore new plan), the fingerprint changes and the approval is void.
			g.Edge(planProd, prod, dac.ServiceApproval("prod plan"))

			g.Entry(dev, pulumi.Map{"sha": pulumi.String(git.MergedSHA())})
			return nil
		})
		return err
	})
}
```

Cross-stack ordering inside an environment is just Pulumi dependency order between `pulumi.Stack`
resources — nothing workflow-specific. Approval-to-diff binding is the fingerprint: the plan node
exports the digest into cursor data, and `pulumi.Stack` never needs to accept a plan file. An
approval can sit open for days; a superseding change routes through `plan-prod` again and mints a
new fingerprint. Roll-forward recovery is the model's native failure story (revert the commit →
new sha → new cursor → the pipeline runs again; a half-applied chain heals by reconciliation).
Destructive-operation gates belong to policy packs / `pulumi.Stack` options, not the workflow
layer.

**Cannot (narrow):** the fingerprint pins the approval to the *previewed* diff, but drift between
approval and apply can still make the applied diff differ — the digest detects staleness at the
gate, not at apply time. Tightening further would need apply-time plan verification inside
`pulumi.Stack`, which is explicitly not planned.

### Scenario 3: the hands-off multi-region pipeline

Waves of increasing size, bake-time + aggregate-alarm gates, deployment windows, auto-rollback,
multiple releases on the conveyor at once.

```go
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"acme.corp/deploy/internal/alarms"  // aggregate alarm, minimum-data-point aware
	"acme.corp/deploy/internal/git"
	"acme.corp/deploy/internal/windows" // deployment windows
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/workflow"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		breakglass := config.New(ctx, "").GetBool("breakglass") // the emergency lane
		waves := [][]string{
			{"eu-north-1"},
			{"us-east-1"},
			{"ap-south-1", "sa-east-1"},
		}
		bake := []time.Duration{12 * time.Hour, 4 * time.Hour, 2 * time.Hour}

		_, err := workflow.NewWorkflow(ctx, "rollout", func(g *workflow.Graph) error {
			deployRegions := func(regs []string) func(*pulumi.Context) error {
				return func(ctx *pulumi.Context) error {
					sha := g.Config(ctx).Require("sha")
					for _, r := range regs {
						// In-stage mechanics (one-box, cell-by-cell) live in the region
						// stack's own tooling; the workflow orchestrates across stages.
						_, err := pulumi.NewStack(ctx, r, pulumi.StackArgs{
							Stack:   r,
							Project: "acme-service",
							Remote:  true,
							Config:  pulumi.StringMap{"sha": pulumi.String(sha)},
						})
						if err != nil {
							return err
						}
					}
					// Rollback bookkeeping: what ran here before us, and how far we got.
					ctx.Export("lastGood", pulumi.String(g.Config(ctx).Get("current")))
					ctx.Export("current", pulumi.String(sha))
					reached := g.Config(ctx).Get("reached")
					ctx.Export("reached", pulumi.String(strings.TrimPrefix(
						reached+","+strings.Join(regs, ","), ",")))
					return nil
				}
			}

			rollback := g.DefNode("rollback", func(ctx *pulumi.Context) error {
				last := g.Config(ctx).Require("lastGood")
				for _, r := range strings.Split(g.Config(ctx).Require("reached"), ",") {
					_, err := pulumi.NewStack(ctx, r, pulumi.StackArgs{
						Stack:   r,
						Project: "acme-service",
						Remote:  true,
						Config:  pulumi.StringMap{"sha": pulumi.String(last)},
					})
					if err != nil {
						return err
					}
				}
				return nil
			})

			oneBox := g.DefNode("one-box", deployRegions([]string{"one-box"}))
			prev := oneBox
			for i, regs := range waves {
				i := i
				n := g.DefNode(fmt.Sprintf("wave-%d", i), deployRegions(regs))
				// Rollback first: when the alarm fires it must win over promotion
				// (first passing edge wins, in definition order).
				g.Edge(n, rollback, func(ctx context.Context) (bool, error) {
					return alarms.AggregateFiring(ctx)
				})
				g.Edge(prev, n, func(ctx context.Context) (bool, error) {
					t := workflow.From(ctx)
					quiet, err := alarms.QuietSince(ctx, t.When)
					if err != nil || !quiet { // blocker checked at every stage, not once
						return false, err
					}
					if breakglass {
						return true, nil
					}
					return time.Since(t.When) > bake[i] && windows.OpenNow(), nil
				})
				prev = n
			}

			g.Entry(oneBox, pulumi.Map{
				"sha":     pulumi.String(git.MergedSHA()),
				"current": pulumi.String(""),
				"reached": pulumi.String(""),
			})
			return nil
		})
		return err
	})
}
```

The conveyor is the multi-cursor model verbatim: each merge admits a cursor; releases occupy
different waves simultaneously; a faster follow-up supersedes a slower release mid-bake (deploying
the newer sha to that wave subsumes the older). Blockers-at-every-stage fall out of conditions
being per-edge. Bake time is `From(ctx).When` — "nothing happened for 12 hours" is evidence the
condition reads from the alarm history, not a timer the engine runs.

**Cannot (latency, not structure):** "rollback is already running when a human first looks"
requires the alarm to *trigger* an `up` (alarm → webhook → Deployments run); in-model latency is
the `up` cadence, since there is no self-wake. And in-stage mechanics (one-box per cell, rolling
batches, traffic shifting) stay delegated to native tooling (CodeDeploy, Argo, ASG) inside the
wave node — the scenario doc itself reaches the same conclusion.

### Scenario 4: instance patching and AMI rotation

A monthly/CVE-triggered rollout that replaces every instance in the fleet, cluster by cluster,
pausable for days, resumable exactly.

```go
package main

import (
	"context"
	"time"

	"acme.corp/deploy/internal/ami"
	"acme.corp/deploy/internal/fleet" // cluster inventory + health checks
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/workflow"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		emergency := config.New(ctx, "").GetBool("cve-emergency") // the fast lane

		_, err := workflow.NewWorkflow(ctx, "ami-rotation", func(g *workflow.Graph) error {
			test := g.DefNode("test-cluster", func(ctx *pulumi.Context) error {
				// Boot the image, join a cluster, run conformance + baseline scans.
				return fleet.Conformance(ctx, g.Config(ctx).Require("ami"))
			})

			prev, prevName := test, "test-cluster"
			for _, c := range fleet.Clusters() { // static fleet list => generated chain of nodes
				c, gateOn := c, prevName
				n := g.DefNode("cluster-"+c.Name, func(ctx *pulumi.Context) error {
					// ASG instance refresh: native batch-size, per-batch health gate,
					// and capacity-floor semantics live below the workflow.
					return c.Refresh(ctx, g.Config(ctx).Require("ami"),
						fleet.RefreshOpts{MinHealthyPercent: 90})
				})
				g.Edge(prev, n, func(ctx context.Context) (bool, error) {
					healthy, err := fleet.Healthy(ctx, gateOn)
					if err != nil || !healthy {
						return false, err
					}
					if emergency {
						return true, nil
					}
					return time.Since(workflow.From(ctx).When) > 24*time.Hour, nil // soak
				})
				prev, prevName = n, c.Name
			}

			g.Entry(test, pulumi.Map{"ami": pulumi.String(ami.Golden())}) // one cursor per AMI release
			return nil
		})
		return err
	})
}
```

This scenario is the flagship fit. "The run outlives everything" is the definition of a parked
cursor: pausing mid-fleet over a weekend is zero compute and zero live process; resuming is the
next `up`. "Progress is the state that matters" is the cursor's position. Rollback is boring by
construction: an `Entry` with the previous AMI ID runs the same machinery in reverse. Per-batch
gates *within* a cluster delegate to ASG instance refresh; if a team needs hand-rolled batches,
the same loop that generated cluster nodes generates batch nodes.

**Cannot:** nothing structural. A 30-cluster fleet is 30 generated nodes — fine; a 5,000-target
list as individual nodes would strain graph-shape state and UI, at which point batching targets
per node is the answer.

### Scenario 5: stateful systems and operator-driven steps

Kafka broker replacement: drain, wait for recovery metric, replace, optional human checkpoint,
strictly one broker at a time, forward-only.

```go
package main

import (
	"context"
	"strings"

	"acme.corp/deploy/internal/dac"
	"acme.corp/deploy/internal/kafka" // cluster metrics + broker inventory
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/workflow"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := workflow.NewWorkflow(ctx, "broker-roll", func(g *workflow.Graph) error {
			sel := g.DefNode("select", func(ctx *pulumi.Context) error {
				remaining := strings.Split(g.Config(ctx).Require("remaining"), ",")
				ctx.Export("current", pulumi.String(remaining[0]))
				ctx.Export("remaining", pulumi.String(strings.Join(remaining[1:], ",")))
				return nil
			})
			replace := g.DefNode("replace", func(ctx *pulumi.Context) error {
				broker := g.Config(ctx).Require("current")
				drain, err := local.NewCommand(ctx, "drain", &local.CommandArgs{
					// Script step: the reassignment's exit code gates the run.
					Create: pulumi.String(kafka.ReassignAwayCmd(broker)),
				})
				if err != nil {
					return err
				}
				return kafka.ReplaceBroker(ctx, broker,
					pulumi.DependsOn([]pulumi.Resource{drain}))
			})
			done := g.DefNode("done", kafka.FinalRebalanceAndVerify)

			// Exit edge first, so it wins once the list is empty.
			g.Edge(sel, done, func(ctx context.Context) (bool, error) {
				return workflow.From(ctx).Data().Get("remaining") == "", nil
			})
			g.Edge(sel, replace, workflow.Always)
			g.Edge(replace, sel, func(ctx context.Context) (bool, error) {
				urp, err := kafka.UnderReplicatedPartitions(ctx) // metric gate; minutes or hours
				if err != nil || urp != 0 {
					return false, err
				}
				return dac.Approved(ctx, "next-broker") // the human checkpoint between brokers
			})

			g.Entry(sel, pulumi.Map{
				// "run" distinguishes this roll from the last one with the same broker list;
				// an identical seed hash would admit no cursor.
				"run":       pulumi.String("2026-08-golden"),
				"remaining": pulumi.String(strings.Join(kafka.Brokers(), ",")),
			})
			return nil
		})
		return err
	})
}
```

The loop is a cycle with the target list carried in cursor data — `select` peels one broker per
visit, `replace → select` re-arms because entering a node starts a fresh visit. One-at-a-time
ordering is a single cursor; indefinite mid-sequence pause is a parked cursor waiting on the
metric; forward-only recovery is the model's only recovery. When gates pass instantly, one `up`
walks multiple brokers; otherwise each `up` advances as far as the metrics allow.

**Cannot (awkward, not impossible):** scripts-as-resources. The Command provider is the answer for
now; a user needing run-once semantics can build it with `pulumi.Stash`. Note the cadence caveat:
the recovery metric is polled per `up`, so an active roll wants a tight cron (minutes, not hours).

### What cuts across all five — checked against the model

- *Suspend with zero compute, resume quickly*: parked cursors in checkpoint state; the model's
  core strength.
- *Run something and gate on the result / watch a metric and gate on a threshold*: both are just
  conditions; sampled once per `up`, which is exactly "gate re-checked every run".
- *Several releases in flight at once*: multiple cursors; supersede is the batching semantics.
- *Undo varies (redeploy last good / roll forward / manual)*: redeploy = `Entry` with old
  artifact; roll forward = new `Entry` + reconciliation.
- *Same skeleton, different artifact*: the artifact is cursor data; the skeleton is the graph.

## Decided

- Sub-state: node resources live in the main snapshot, marked with an owner (workflow URN + node)
  and URN-namespaced by a per-node project qualifier. Secrets, refresh, display, and export come
  for free; workflow destroy and node GC are ordinary delete sweeps over the marking. (Supersedes
  the earlier managed-stack-per-node and state-in-the-resource designs.)
- Node GC runs after everything else in the `up`; a resident cursor does not block it.
- `up` reconciles every occupied node; preview is static in v1.
- Approvals bind to the cursor-data fingerprint; `pulumi.Stack` will not accept a plan file.
- Entries are per-node slots (seed-hash change admits); bounded by node count, no GC.
- Imperative actions: Command provider for now; run-once composable from `pulumi.Stash`.

## Deferred

- Cursor surgery (`pulumi workflow cursor ls|rm|mv`): not in the prototype.
- Simulated preview (evaluate conditions, preview would-be moves).
- AND-join / merge-on-collision and per-cursor sub-state: needs a fundamental FSA change first;
  none of the five scenarios requires it.
