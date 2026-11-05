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

package newcmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/kballard/go-shellquote"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/credentialcheck"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
)

// remediationOffer offers to run the command a provider suggested to fix a credentials problem.
type remediationOffer struct {
	stdout  io.Writer
	confirm func(prompt string) bool
	run     func(ctx context.Context, r plugin.CommandRemediation) error
}

// offer asks the user whether to run the provider's first suggested command, runs it if they
// agree, and reports whether it succeeded.
func (o remediationOffer) offer(ctx context.Context, problem *credentialcheck.Problem) bool {
	remediations := problem.Remediations()
	if len(remediations) == 0 {
		return false
	}
	r := remediations[0]
	command := shellquote.Join(r.Command...)
	prompt := fmt.Sprintf("Run `%s` now?", command)
	if r.Description != "" {
		prompt = fmt.Sprintf("%s now? This runs `%s`.", r.Description, command)
	}
	if !o.confirm(prompt) {
		fmt.Fprintln(o.stdout)
		return false
	}
	if err := o.run(ctx, r); err != nil {
		if ctx.Err() == nil {
			fmt.Fprintf(o.stdout, "`%s` did not complete; run it yourself, then `pulumi up`.\n\n", command)
		}
		return false
	}
	return true
}

// runRemediation runs a suggested command attached to the user's terminal, so that it can prompt.
// The cobra writers may be buffered or piped, so the command gets the process's own streams.
func runRemediation(ctx context.Context, r plugin.CommandRemediation) error {
	cmd := exec.CommandContext(ctx, r.Command[0], r.Command[1:]...)
	cmd.Env = os.Environ()
	for k, v := range r.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout //nolint:forbidigo
	cmd.Stderr = os.Stderr //nolint:forbidigo
	return cmd.Run()
}
