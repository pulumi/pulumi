# A subclassable `delivery.Gate` — worked example

This is a motivating example for [component inheritance](../../inheritance.md): a small
Delivery-as-Code gate hierarchy that shows why cross-language component subclassing is worth
having, using the `Gate` / `Condition` / `Wake` vocabulary from the Build & Delivery model.

## The tension it resolves

The Build & Delivery model settles on **"compose, don't extend"**: a base `Gate` awaits one
boolean, and the typed gates (`ApprovalGate`, `MetricGate`, …) are separate resources that fold
in their own check and wake. That rule exists in part because, until now, a *component* could
not be subclassed across languages — so "extend" was not on the table, and the typed gates are
either provider-authored custom resources or hand-written SDK overlays. The practical costs:

- The set of gate types is fixed by whoever authored the provider. A team that wants a
  *governed* gate of their own — a change-window gate, a quorum gate, a "wait for a Jira
  transition" gate — either petitions the platform team or reimplements the base
  suspend/resume/status/wake plumbing themselves.
- A gate authored in one language cannot be extended in another. A platform team on TypeScript
  cannot hand a Python team a `Gate` base to build on.

Component inheritance removes both costs, which turns "compose, don't extend" into a genuine
*choice* rather than a limitation. The base `Gate` (the hard part) is written once; every team
subclasses it, in their own language, to add just their check.

## The files

- `schema.json` — the `delivery` package. An **abstract** base component `Gate` (inputs
  `until`/`wake`; outputs `status`/`resumeWhen`; an inherited method `explain()`), and two
  concrete subclasses, `ApprovalGate` and `MetricGate`, that `extends` it. This is the package a
  platform team publishes; SDKs generate from it in every language.
- `consumer-ts/index.ts` — a rollout that gates staging behind a `MetricGate` and prod behind an
  `ApprovalGate`, holds them in one `Gate[]`, and calls the inherited `explain()` on each. The
  point: different gate *types*, one uniform `Gate` surface.
- `consumer-py/change_window_gate.py` — the payoff. A customer's **own** gate type,
  `ChangeWindowGate`, authored in **Python** by subclassing the (TypeScript-authored)
  `delivery.Gate`. It supplies its boolean and a time `wake`; it inherits `status`,
  `resumeWhen`, and `explain()` untouched.
- `consumer-py/__main__.py` — a rollout that mixes a platform `ApprovalGate` with the customer's
  `ChangeWindowGate`, treating both uniformly as `Gate`s.

## What is actually verified

Everything below was run against the working-tree implementation (not asserted):

- **Schema binds; SDKs generate** for Node.js, Python, and Go from `schema.json`, through the
  same codegen the conformance suite exercises.
- **The TypeScript rollout type-checks** (`tsc`, strict) against the generated SDK: subclass
  instantiation, the polymorphic `Gate[]`, the inherited `explain()` method, and inherited
  outputs (`status`, `resumeWhen`) all resolve.
- **The abstract base is enforced**: `new delivery.Gate(...)` is a compile error (TS2511).
- **The Python cross-language subclass resolves**: `ChangeWindowGate` is a subclass of the
  generated `delivery.Gate`, is a `ComponentResource`, carries its own type token
  (`acme:delivery:ChangeWindowGate`), and inherits `explain()`.

Writing this example also surfaced and fixed a real bug: the Python codegen for a
user-authored subclass emitted `pulumi.runtime.get_type_token(...)`, but that helper was
exported only as `pulumi.get_type_token` — so subclassing a generated component in a Python
*program* would have failed at runtime. The conformance suite did not catch it because it only
exercises generated-derived components, not user-authored subclasses. `get_type_token` is now
re-exported from `pulumi.runtime`, with a regression test asserting every helper the codegen
emits under that namespace resolves.

## What is *not* yet true

- The real `pulumi-delivery` provider models `Gate` as a **custom resource** (its `Create`
  returns the engine's `awaiting` disposition). To ship a subclassable `Gate`, `Gate` would be
  re-modeled as an abstract **component** that wraps the await primitive — a provider change,
  not a language change. This example uses that component shape directly to show the authoring
  experience.
- End-to-end *deploy* of a user-authored subclass (the Python `ChangeWindowGate`) is covered by
  SDK unit tests, not yet by the cross-language integration matrix (that is deferred, pending a
  branch-built CLI; see the inheritance spec's rollout notes). Generated-derived components
  (the `ApprovalGate`/`MetricGate` shape) *are* proven end-to-end in the Go, PCL, and Python
  conformance runners.
