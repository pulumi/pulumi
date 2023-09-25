// Copyright 2016-2020, Pulumi Corporation.
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

package auto

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
)

const unknownErrorCode = -2

func runPulumiCommandSync(
	ctx context.Context,
	workdir string,
	stdin io.Reader,
	additionalOutput []io.Writer,
	additionalErrorOutput []io.Writer,
	additionalEnv []string,
	args ...string,
) (string, string, int, error) {
	// all commands should be run in non-interactive mode.
	// this causes commands to fail rather than prompting for input (and thus hanging indefinitely)
	args = withNonInteractiveArg(args)
	cmd := exec.CommandContext(ctx, "pulumi", args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), additionalEnv...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	additionalOutput = append(additionalOutput, &stdout)
	additionalErrorOutput = append(additionalErrorOutput, &stderr)
	cmd.Stdout = io.MultiWriter(additionalOutput...)
	cmd.Stderr = io.MultiWriter(additionalErrorOutput...)
	cmd.Stdin = stdin

	code := unknownErrorCode
	err := cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok {
		code = exitError.ExitCode()
	} else if err == nil {
		// If there was no error then the exit code was 0
		code = 0
	}
	return stdout.String(), stderr.String(), code, err
}

func withNonInteractiveArg(args []string) []string {
	out := slice.Prealloc[string](len(args))
	seen := false
	for _, a := range args {
		out = append(out, a)
		if a == "--non-interactive" {
			seen = true
		}
	}
	if !seen {
		out = append(out, "--non-interactive")
	}
	return out
}
