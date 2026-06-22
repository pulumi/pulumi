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

// Package needle provides a standardized mechanism to inject command level complex variables into context.
//
// It can and should be used instead of boilerplate to ensure behavior is consistent across commands.
package needle

import (
	"context"
	"iter"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/util/pdag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func Inject(cmd *cobra.Command, ctx Context, requests ...Request) {
	ordered := orderRequests(cmd.Context(), requests)

	// state is built now so that prepare can bind flags to it, but the runtime defaults
	// (color, diag sink) depend on flag parsing, so the Context is resolved in PreRunE below.
	var state state

	for _, req := range ordered {
		if req.prepare != nil {
			req.prepare(cmd, &state)
		}
	}

	preRunE := cmd.PreRunE
	if preRunE == nil && cmd.PreRun != nil {
		preRunE = func(cmd *cobra.Command, args []string) error {
			cmd.PreRun(cmd, args)
			return nil
		}
	}

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		state.Context = resolveContext(cmd, ctx)
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
func resolveContext(cmd *cobra.Command, c Context) Context {
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

type Context struct {
	Env      env.Env
	WS       pkgWorkspace.Context // No default
	Color    colors.Colorization
	DiagSink diag.Sink
	LM       cmdBackend.LoginManager
}

func orderRequests(ctx context.Context, requests []Request) []*value {
	order := pdag.New[*value]()
	nodes := make(map[*value]pdag.Node, len(requests))

	insert := func(req *value) pdag.Node {
		if existing, ok := nodes[req]; ok {
			return existing
		}
		var done pdag.Done
		nodes[req], done = order.NewNode(req)
		done()
		return nodes[req]
	}

	for _, req := range requests {
		self := insert(req.self())

		for dep := range req.dependencies() {
			err := order.NewEdge(insert(dep), self)
			contract.AssertNoErrorf(err, "needle: dependency cycle found: %s", err)
		}
	}

	ordered := make([]*value, 0, len(requests))
	for req, done := range order.Drain(ctx) {
		ordered = append(ordered, req)
		done()
	}
	return ordered
}

type Request interface {
	// dependencies that need to fill their state value before self can run
	dependencies() iter.Seq[*value]

	// the request that needs to be completed for the user's Request
	self() *value

	// Read the value from state into the out var
	fulfill(*state)
}

type request struct {
	*value

	fulfillInto func(*state)
}

func (r request) self() *value     { return r.value }
func (r request) fulfill(s *state) { r.fulfillInto(s) }

type value struct {
	deps    []*value
	prepare func(*cobra.Command, *state)
	get     func(*cobra.Command, *state) error
}

func (r *value) dependencies() iter.Seq[*value] { return slices.Values(r.deps) }

type state struct {
	// Always set
	Context

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
