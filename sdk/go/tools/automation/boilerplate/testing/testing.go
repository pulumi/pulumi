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

// Package automation is the testing boilerplate. Its run method does not
// spawn a process; instead it renders the command line that would have been
// executed, which generator tests can assert against directly. Mirrors the
// `testing` boilerplates of the NodeJS and Python generators.
package automation

import (
	"context"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/tools/automation/boilerplate/base"
)

// API is the same public shape as the production boilerplate. The generator
// appends the generated methods after this declaration.
type API struct{}

// New returns an API that renders command lines instead of executing them.
// The cmd argument is accepted (and ignored) for API parity with the
// standard boilerplate.
func New(_ base.PulumiCommand) *API {
	return &API{}
}

// run returns the rendered `pulumi ...` command line in CommandResult.Stdout.
// The BaseOptions and ctx parameters are accepted and ignored.
//
// The generator appends method declarations to a copy of this file in
// output/automation/, where `run` becomes used. The linter sees this file
// in isolation and cannot tell.
//
//nolint:unused
func (a *API) run(
	_ context.Context,
	_ base.BaseOptions,
	args []string,
) (base.CommandResult, error) {
	return base.CommandResult{
		Stdout: "pulumi " + strings.Join(args, " "),
	}, nil
}
