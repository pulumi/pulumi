// Package delivery is a hand-authored PROTOTYPE of the Go interface projection for a
// subclassable component base. It shows the target shape codegen should emit for Go so that an
// abstract base like `Gate` becomes usable polymorphically -- []Gate holding differently-typed
// subclasses -- which struct embedding alone cannot express in Go.
//
// The projection for an abstract base is a triple:
//
//   - Gate:      the INTERFACE consumers hold and pass around (the is-a type).
//   - GateBase:  the embeddable base STRUCT carrying shared state, output accessors, and methods;
//                subclasses embed it, which promotes those accessors/methods so the subclass
//                satisfies Gate automatically.
//   - subclasses (ApprovalGate, MetricGate): concrete structs embedding GateBase.
//
// Field/method name collisions force Get-prefixed accessors (Go cannot have a field and method of
// the same name), which is why the interface exposes GetStatus() rather than a Status field.
package delivery

import "github.com/pulumi/pulumi/sdk/v3/go/pulumi"

// Gate is the polymorphic interface satisfied by every gate. This is what makes []Gate work.
type Gate interface {
	pulumi.Resource
	GetStatus() pulumi.StringOutput
	GetResumeWhen() pulumi.StringMapOutput
	// Explain returns why the gate is holding. Inherited by every subclass.
	Explain() pulumi.StringOutput
}

// GateBase is the embeddable base. Subclasses embed it; embedding pulumi.ResourceState makes them
// satisfy pulumi.Resource, and the methods below make them satisfy Gate.
type GateBase struct {
	pulumi.ResourceState

	Status     pulumi.StringOutput    `pulumi:"status"`
	ResumeWhen pulumi.StringMapOutput `pulumi:"resumeWhen"`
}

func (g *GateBase) GetStatus() pulumi.StringOutput        { return g.Status }
func (g *GateBase) GetResumeWhen() pulumi.StringMapOutput { return g.ResumeWhen }

// Explain is stubbed here (the prototype is about the type shape, not the RPC); the real codegen
// would emit the ctx.Call body.
func (g *GateBase) Explain() pulumi.StringOutput { return g.Status.ToStringOutput() }

// ApprovalGate is a concrete gate. It embeds GateBase, so *ApprovalGate satisfies Gate.
type ApprovalGate struct {
	GateBase

	Approvers pulumi.StringArrayOutput `pulumi:"approvers"`
}

// MetricGate is another concrete gate.
type MetricGate struct {
	GateBase

	Query pulumi.StringOutput `pulumi:"query"`
}

// Compile-time proof that both concrete gates satisfy the Gate interface.
var (
	_ Gate = (*ApprovalGate)(nil)
	_ Gate = (*MetricGate)(nil)
)
