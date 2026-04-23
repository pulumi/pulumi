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

// Package automation is the production boilerplate that hosts the generated
// CLI wrapper methods. The generator copies this file into
// output/automation/api.go and appends one method per executable CLI
// command/menu underneath.
package automation

import (
	"context"
	"io"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/tools/automation/boilerplate/base"
)

// API wraps a PulumiCommand and exposes one Go method per pulumi CLI
// subcommand. All methods are appended to this file by the generator.
type API struct {
	cmd base.PulumiCommand
}

// New constructs an API backed by the given PulumiCommand.
func New(cmd base.PulumiCommand) *API {
	return &API{cmd: cmd}
}

// run is the single choke point through which every generated method reaches
// the CLI. Callers hand in their accumulated base options and the resolved
// argument vector; run adapts those into the shape PulumiCommand expects.
//
// The generator appends method declarations to a copy of this file in
// output/automation/, where `run` becomes used. The linter sees this file
// in isolation and cannot tell.
//
//nolint:unused
func (a *API) run(
	ctx context.Context,
	opts base.BaseOptions,
	args []string,
) (base.CommandResult, error) {
	workdir := opts.Cwd
	if workdir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return base.CommandResult{}, err
		}
		workdir = cwd
	}

	var additionalOutput []io.Writer
	if opts.Stdout != nil {
		additionalOutput = append(additionalOutput, opts.Stdout)
	}

	var additionalErrorOutput []io.Writer
	if opts.Stderr != nil {
		additionalErrorOutput = append(additionalErrorOutput, opts.Stderr)
	}

	env := make([]string, 0, len(opts.AdditionalEnv))
	for k, v := range opts.AdditionalEnv {
		env = append(env, k+"="+v)
	}

	stdout, stderr, code, err := a.cmd.Run(
		ctx,
		workdir,
		opts.Stdin,
		additionalOutput,
		additionalErrorOutput,
		env,
		args...,
	)
	return base.CommandResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: code,
	}, err
}
