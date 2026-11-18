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
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/credentialcheck"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
)

// offerAndRunRemediation asks the user whether to run the command the provider suggested to fix
// problem, runs it if they agree, and reports whether it succeeded. It reports false if the user
// interrupted pulumi meanwhile.
func (pf credentialsPreflight) offerAndRunRemediation(ctx context.Context, problem *credentialcheck.Problem) bool {
	r := problem.Remediation()
	if r == nil || pf.confirm == nil {
		return false
	}
	command := shellquote.Join(r.Command...)
	prompt := fmt.Sprintf("Run `%s` now?", command)
	if r.Description != "" {
		prompt = fmt.Sprintf("%s: run `%s` now?", r.Description, command)
	}
	if !pf.confirm(prompt) {
		fmt.Fprintf(pf.stdout, "Skipped running `%s`.\n\n", command)
		return false
	}
	err := pf.run(ctx, *r)
	if ctx.Err() != nil {
		return false
	}
	if err != nil {
		fmt.Fprintf(pf.stdout, "`%s` did not complete; run it yourself, then `pulumi up`.\n\n", command)
		return false
	}
	return true
}

// runRemediation runs r attached to the terminal, since sign-in commands prompt and wait for the
// user. The preflight only runs in interactive mode (see [credentialsCheckEnabled]), which
// requires stdin and stdout to be terminals.
func runRemediation(ctx context.Context, r plugin.CommandRemediation) error {
	cmd := exec.CommandContext(ctx, r.Command[0], r.Command[1:]...)
	cmd.Env = remediationEnv(ctx, os.Environ(), r.Env)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout //nolint:forbidigo
	cmd.Stderr = os.Stderr //nolint:forbidigo
	return cmd.Run()
}

// remediationEnv returns base with the variables a provider asked to set added to it. Variables that
// change which programs or libraries the command loads are dropped, since the user is shown the
// command but not its environment.
func remediationEnv(ctx context.Context, base []string, env map[string]string) []string {
	result := slices.Clone(base)
	for k, v := range env {
		// Windows environment variable names are case-insensitive.
		name := strings.ToUpper(k)
		if name == "PATH" || strings.HasPrefix(name, "LD_") || strings.HasPrefix(name, "DYLD_") {
			slog.DebugContext(ctx, "ignoring remediation environment variable", "name", k)
			continue
		}
		result = append(result, k+"="+v)
	}
	return result
}
