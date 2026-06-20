// Copyright 2026, Pulumi Corporation.

package auto

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
)

// contextScopes derives cancellation solely from the caller's context; it never installs a
// process-wide SIGINT/SIGTERM handler the way the CLI's backend.CancellationScopes does.
// This is the one substitution that makes the driver safe to run nested inside another
// process (a provider, a runner): a signal to the host must not be hijacked by a child
// update, and cancellation should flow only from the context the caller controls.
var contextScopes backend.CancellationScopeSource = contextScopeSource{}

type contextScopeSource struct{}

func (contextScopeSource) NewScope(
	ctx context.Context, _ chan<- engine.Event, _ bool,
) backend.CancellationScope {
	cancelCtx, cancelSrc := cancel.NewContext(ctx)
	return &contextScope{ctx: cancelCtx, src: cancelSrc}
}

type contextScope struct {
	ctx *cancel.Context
	src *cancel.Source
}

func (s *contextScope) Context() *cancel.Context { return s.ctx }

// Close releases the derived contexts; it does not cancel an in-flight operation (the
// operation has already returned by the time the backend closes its scope).
func (s *contextScope) Close() { s.src.Terminate() }
