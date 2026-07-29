# Go interface projection — prototype

This is a **hand-authored prototype** of the Go projection component inheritance should emit for
an extensible base, in response to review feedback: in Go, struct embedding gives you inherited
state and behavior but **not** subtype polymorphism — a `*MetricGate` is not assignable to
`*GateBase`, so you cannot hold a heterogeneous `[]Gate`. Go's only mechanism for is-a is an
**interface**.

The projection for an abstract base is a triple:

- `Gate` — the **interface** consumers hold (`[]delivery.Gate`), embedding `pulumi.Resource`
  plus `Get<Prop>()` accessors and the base's methods.
- `GateBase` — the embeddable base **struct** (shared state + accessors + methods). Subclasses
  embed it; the promotion makes them satisfy `Gate` automatically.
- `ApprovalGate` / `MetricGate` — concrete structs embedding `GateBase`.

`Get`-prefixed accessors are forced because Go cannot have a field and a method of the same name.

## Verified

Both files compile and `go vet` clean against the working-tree SDK:

- `delivery.go` — the projection. The `var _ Gate = (*ApprovalGate)(nil)` assertions are
  compile-time proof that both subclasses satisfy the interface.
- `consumer_main.go.txt` — `gates := []delivery.Gate{approval, metric}` compiles (the thing that
  does *not* compile with the v1 struct projection), a uniform loop calls the inherited surface
  (`GetStatus()`, `Explain()`, `URN()`), and a type-switch **downcasts** from the base interface
  back to the concrete subtype — Go's answer to the "QueryInterface" question.

The contrast was confirmed empirically too: the v1 struct shape,
`[]*delivery.GateBase{&ApprovalGate{}, &MetricGate{}}`, fails to compile with
`cannot use *ApprovalGate ... as *GateBase value` — which is exactly why Go needs the interface.

## Not yet wired into codegen

This is the *target shape*, verified sound; wiring `pkg/codegen/go/gen.go` to emit it (interface
+ embeddable base + `Get<Prop>()` accessors, for abstract/extensible bases) is the follow-up. It
does not touch the engine or the single-URN model — it is purely how the Go SDK surfaces the type.
