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

// Package adder provides standardized, lazily-resolved access to the values
// commands commonly need — the current project, backend, stack, and registry —
// so commands don't re-implement the resolution boilerplate.
//
// Values are resolved by calling methods on [Environment] (e.g. [Environment.Backend])
// from a command's RunE. Resolution is lazy — nothing is computed until a code
// path asks — and memoized per CLI execution, so every ask after the first
// shares the same answer. Dependencies between values are ordinary function
// calls: resolving a stack resolves the backend, which resolves the project.
//
// Values that come with a flag (e.g. [StackFlag]) register their flag at
// command construction and resolve lazily through the returned handle.
//
// The memos live on the context.Context threaded through cobra, seeded by
// [WithBag]. The root command does this for the CLI; a test that executes a
// command directly must do it itself:
//
//	cmd.SetContext(adder.WithBag(context.Background()))
package adder

import (
	"context"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/spf13/cobra"
)

// Environment carries the seams commands resolve values through.
//
// adder is pure given an Environment: resolvers touch the outside world —
// the workspace, credentials, the network, diagnostics — only through these
// fields, never through package-level state, so substituting an Environment
// substitutes every effect of resolution. This is where testing fixtures are
// injected.
//
// WS must always be set; every other field defaults at resolution time (after
// flags are parsed, so flag-dependent defaults like colorization are correct).
// The defaults are the one sanctioned impurity: a field left unset falls back
// to the process-global implementation.
type Environment struct {
	Env      env.Env
	WS       pkgWorkspace.Context // No default; must be set.
	Color    colors.Colorization
	DiagSink diag.Sink
	LM       cmdBackend.LoginManager
}

// defaults returns a copy of e with unset fields filled in.
func (e Environment) defaults(cmd *cobra.Command) Environment {
	contract.Assertf(e.WS != nil, "adder: Environment.WS must be set")
	if e.Env == nil {
		e.Env = env.NewEnv(env.Global)
	}
	if e.Color == "" {
		e.Color = cmdutil.GetGlobalColorization()
	}
	if e.DiagSink == nil {
		e.DiagSink = diag.DefaultSink(cmd.OutOrStdout(), cmd.ErrOrStderr(), diag.FormatOptions{
			Color: e.Color,
		})
	}
	if e.LM == nil {
		e.LM = cmdBackend.DefaultLoginManager
	}
	return e
}

type bagKey struct{}

// bag holds the per-execution memos, one slot per resolvable value.
type bag struct {
	project        memo[projectInfo]
	loginBackend   memo[backend.Backend]
	currentBackend memo[backend.Backend]
	registry       memo[registry.Registry]
	stack          memo[backend.Stack]
}

// WithBag seeds ctx with a fresh set of memos. It must wrap the context a
// command executes under; the CLI's root command seeds it for every
// invocation.
func WithBag(ctx context.Context) context.Context {
	return context.WithValue(ctx, bagKey{}, &bag{})
}

func bagFrom(cmd *cobra.Command) *bag {
	b, ok := cmd.Context().Value(bagKey{}).(*bag)
	contract.Assertf(ok, "adder: command executed without memos; seed the context with adder.WithBag")
	return b
}

// memo caches the first resolution of a value; later callers share the first
// caller's result (including its error and any context it resolved under).
type memo[T any] struct {
	once sync.Once
	v    T
	err  error
}

func (m *memo[T]) get(f func() (T, error)) (T, error) {
	m.once.Do(func() { m.v, m.err = f() })
	return m.v, m.err
}
