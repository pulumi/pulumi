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

package tools

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// Shell is the local handler for the Neo `shell` tool. The cloud agent exposes a single
// method named `shell_execute` (see pulumi-service:cmd/agents/src/agents_py/mcp/shell_mcp.py)
// with arguments `{command: string, cwd?: string}`.
type Shell struct {
	Cwd            string
	DefaultTimeout time.Duration
}

// NewShell creates a Shell handler with sensible defaults.
func NewShell(cwd string) *Shell {
	return &Shell{Cwd: cwd, DefaultTimeout: 2 * time.Minute}
}

// Invoke dispatches a single shell method call.
func (s *Shell) Invoke(ctx context.Context, method string, args json.RawMessage) (any, error) {
	if method != "shell_execute" {
		return nil, fmt.Errorf("unknown shell method %q", method)
	}
	var p struct {
		Command string `json:"command"`
		Cwd     string `json:"cwd,omitempty"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("decoding shell_execute args: %w", err)
	}
	if p.Command == "" {
		return nil, errors.New("shell_execute requires a non-empty command")
	}
	return s.run(ctx, p.Command, p.Cwd)
}

func (s *Shell) run(ctx context.Context, command, cwd string) (any, error) {
	runCtx, cancel := context.WithTimeout(ctx, s.DefaultTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(runCtx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(runCtx, "sh", "-c", command)
	}
	cmd.Dir = cmp.Or(cwd, s.Cwd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := map[string]any{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": 0,
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result["exit_code"] = exitErr.ExitCode()
		} else {
			result["error"] = err.Error()
		}
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		result["timed_out"] = true
	}
	return result, nil
}
