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

// Package needle provides a standardized mechanism to inject command level complex
// variables into context. It can and should be used instead of boilerplate to ensure
// behavior is consistent across commands.
//
// By convention, each [Stitch] function should start with Require or Option.
//
// The main entry point to the package is [Thread]. It should be called once before the
// and passed all desired requests.
package needle

// Maintainers note:
//
// [Request]s are identified by their [*value], so **all** valid implementations of
// `request` need to return a pointer to a global (and thus immutable) [*value].

import (
	"context"
	"iter"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

// Thread ensures that all requests will be fulfilled before cmd.PreRunE is called. It
// should be called outside of cmd's execution, similar to adding flags to cmd.
//
// As an example, if you want to use the stack associated with a command, use:
//
//	var stack backend.Stack
//
//	cmd := &cobra.Command{
//		Use: "print-stack-name",
//		Run: func(cmd *Command, args []string) {
//			fmt.FPrintln(cmd.OutOrStdout(), stack.Ref())
//		},
//	}
//
//	needle.Thread(cmd, nCtx,
//		needle.RequireStack(&stack, "" /* default message */))
//
// This will automatically add the --stack/-s flag, use the local project (if any) to find
// the correct backend, error if the stack param is invalid, drive the stack selection
// chooser (if there isn't a default stack) and ensure that your function body sees a
// valid stack.
func Thread(cmd *cobra.Command, ctx Spindle, requests ...Stitch) {
	ordered := orderRequests(cmd.Context(), requests)

	// state is built now so that prepare can bind flags to it, but the runtime defaults
	// (color, diag sink) depend on flag parsing, so the Context is resolved in PreRunE below.
	var state state

	for _, req := range ordered {
		req.prepare(cmd, &state)
	}

	preRunE := cmd.PreRunE
	if preRunE == nil && cmd.PreRun != nil {
		preRunE = func(cmd *cobra.Command, args []string) error {
			cmd.PreRun(cmd, args)
			return nil
		}
	}

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		state.Spindle = resolveContext(cmd, ctx)
		for _, req := range ordered {
			if err := req.get(cmd, &state); err != nil {
				return err
			}
		}
		for _, req := range requests {
			req.fulfill(&state)
		}
		if preRunE != nil {
			return preRunE(cmd, args)
		}
		return nil
	}
}

// resolveContext applies defaults for any fields the caller left unset.
func resolveContext(cmd *cobra.Command, c Spindle) Spindle {
	if c.Env == nil {
		c.Env = env.NewEnv(env.Global)
	}
	if c.Color == "" {
		c.Color = cmdutil.GetGlobalColorization()
	}
	if c.DiagSink == nil {
		c.DiagSink = diag.DefaultSink(cmd.OutOrStdout(), cmd.ErrOrStderr(), diag.FormatOptions{
			Color: c.Color,
		})
	}
	if c.LM == nil {
		c.LM = cmdBackend.DefaultLoginManager
	}
	return c
}

// Spindle represents the context necessary to thread a [Stitch].
//
// This is where testing fixtures can be injected.
type Spindle struct {
	Env      env.Env
	WS       pkgWorkspace.Context // No default
	Color    colors.Colorization
	DiagSink diag.Sink
	LM       cmdBackend.LoginManager
}

// Either the request or the value should be non-nil
type requestOrValue struct {
	value   *value
	request Stitch
}

func (r requestOrValue) prepare(cmd *cobra.Command, s *state) {
	if r.request != nil {
		if prepare := r.request.self().prepare; prepare != nil {
			prepare(cmd, s, r.request.getPayload())
		}
	} else if prepare := r.value.prepare; prepare != nil {
		prepare(cmd, s, nil)
	}
}

func (r requestOrValue) get(cmd *cobra.Command, s *state) error {
	if r.request != nil {
		return r.request.self().get(cmd, s, r.request.getPayload())
	}
	return r.value.get(cmd, s, nil)
}

func orderRequests(ctx context.Context, requests []Stitch) []requestOrValue {
	order := pdag.New[requestOrValue]()
	nodes := make(map[*value]pdag.Node, len(requests))

	insert := func(req requestOrValue) pdag.Node {
		var v *value
		if req.request != nil {
			v = req.request.self()
		} else {
			v = req.value
		}

		if existing, ok := nodes[v]; ok {
			return existing
		}
		var done pdag.Done
		nodes[v], done = order.NewNode(req)
		done()
		return nodes[v]
	}

	// Ensure that all requests will be seen, so any request specific args are visible.
	for _, req := range requests {
		insert(requestOrValue{request: req})
	}
	// Now add dependencies so we will resolve in edge order.
	for _, req := range requests {
		for dep := range req.dependencies() {
			err := order.NewEdge(insert(requestOrValue{value: dep}), nodes[req.self()])
			contract.AssertNoErrorf(err, "needle: dependency cycle found: %s", err)
		}
	}

	ordered := make([]requestOrValue, 0, len(requests))
	for req, done := range order.Drain(ctx) {
		ordered = append(ordered, req)
		done()
	}
	return ordered
}

type Stitch interface {
	// dependencies that need to fill their state value before self can run
	dependencies() iter.Seq[*value]

	// the request that needs to be completed for the user's Request
	//
	// Must be stable by pointer identity
	self() *value

	// Read the value from state into the out var
	fulfill(*state)

	getPayload() any
}

type request struct {
	*value

	fulfillInto func(*state)

	payload any
}

func (r request) self() *value     { return r.value }
func (r request) fulfill(s *state) { r.fulfillInto(s) }
func (r request) getPayload() any  { return r.payload }

type value struct {
	deps    []*value
	prepare func(*cobra.Command, *state, any)
	get     func(*cobra.Command, *state, any) error
}

func (r *value) dependencies() iter.Seq[*value] { return slices.Values(r.deps) }

type state struct {
	// Always set
	Spindle

	// backend.go
	backend backend.Backend

	// project.go
	project     *workspace.Project
	projectRoot string

	// stack.go
	stack     backend.Stack
	stackFlag string

	// registry.go
	registry registry.Registry
}
