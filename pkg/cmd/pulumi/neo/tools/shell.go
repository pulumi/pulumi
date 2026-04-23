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
// with arguments `{command: string, cwd?: string}`. If cwd is supplied it must resolve to a
// subdirectory of Cwd; otherwise the request is rejected.
type Shell struct {
	Cwd            string
	DefaultTimeout time.Duration
}

// NewShell creates a Shell handler with sensible defaults. The working directory is
// resolved to its canonical path (following symlinks) so that the containment check
// in resolveDir cannot be bypassed via symlinks.
func NewShell(cwd string) (*Shell, error) {
	abs, err := canonicalRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolving shell cwd: %w", err)
	}
	return &Shell{Cwd: abs, DefaultTimeout: 2 * time.Minute}, nil
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
	dir, err := s.resolveDir(p.Cwd)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, p.Command, dir)
}

// resolveDir validates that dir is under s.Cwd. An empty dir defaults to s.Cwd.
// Symlinks are resolved to prevent symlink-based directory traversal.
func (s *Shell) resolveDir(dir string) (string, error) {
	if dir == "" {
		return s.Cwd, nil
	}
	return resolveUnderRoot(s.Cwd, dir, false)
}

// maxOutputBytes is the maximum number of bytes captured from stdout or stderr.
// Output beyond this limit is silently discarded and "truncated" is set in the result.
const maxOutputBytes = 1 << 20 // 1 MiB

// cappedBuffer is a bytes.Buffer that stops accepting writes after a limit.
type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	remaining := c.limit - c.buf.Len()
	if remaining <= 0 {
		c.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		c.truncated = true
		p = p[:remaining]
	}
	return c.buf.Write(p)
}

func (s *Shell) run(ctx context.Context, command string, dir string) (any, error) {
	runCtx, cancel := context.WithTimeout(ctx, s.DefaultTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(runCtx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(runCtx, "sh", "-c", command)
	}
	cmd.Dir = dir

	stdout := &cappedBuffer{limit: maxOutputBytes}
	stderr := &cappedBuffer{limit: maxOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()

	result := map[string]any{
		"stdout":    stdout.buf.String(),
		"stderr":    stderr.buf.String(),
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
	if stdout.truncated || stderr.truncated {
		result["truncated"] = true
	}
	return result, nil
}
