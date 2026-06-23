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
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// Shell is the local handler for the Neo `shell` tool. The cloud agent exposes a single
// method named `shell_execute` (see pulumi-service:cmd/agents/src/agents_py/mcp/shell_mcp.py)
// with arguments `{command: string, cwd?: string, timeout?: number}`. timeout is in
// seconds; 0 or omitted falls back to DefaultTimeout. If cwd is supplied it must resolve
// under one of the allowed roots (Cwd or any extras passed to NewShell, e.g. /tmp);
// otherwise the request is rejected.
type Shell struct {
	// Cwd is the default working directory used when the request omits cwd.
	Cwd            string
	DefaultTimeout time.Duration
	// allowedRoots is Cwd followed by any extra roots passed to NewShell.
	allowedRoots []string
	// OnExec, when non-nil, runs the command in place of local execution. dir is
	// the already-resolved, root-validated working directory and timeout the
	// effective timeout. The Neo ACP adapter sets this to run the command in the
	// editor's terminal (terminal/*). It returns the same ShellResult this tool
	// produces locally. A nil OnExec runs the command on this machine.
	OnExec func(ctx context.Context, command, dir string, timeout time.Duration) (ShellResult, error)
}

// NewShell creates a Shell handler with sensible defaults. The working directory is
// resolved to its canonical path (following symlinks) so that the containment check
// in resolveDir cannot be bypassed via symlinks. extraRoots are additional directories
// the agent may run commands under (e.g. /tmp).
func NewShell(cwd string, extraRoots ...string) (*Shell, error) {
	abs, err := canonicalRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolving shell cwd: %w", err)
	}
	allowed := []string{abs}
	for _, extra := range extraRoots {
		canonical, err := canonicalRoot(extra)
		if err != nil {
			return nil, fmt.Errorf("resolving shell extra root %q: %w", extra, err)
		}
		extraInfo, err := os.Stat(canonical)
		if err != nil {
			return nil, fmt.Errorf("shell extra root %q: %w", canonical, err)
		}
		if !extraInfo.IsDir() {
			return nil, fmt.Errorf("shell extra root %q is not a directory", canonical)
		}
		allowed = append(allowed, canonical)
	}
	return &Shell{Cwd: abs, DefaultTimeout: 2 * time.Minute, allowedRoots: allowed}, nil
}

// Invoke dispatches a single shell method call.
func (s *Shell) Invoke(ctx context.Context, method string, args json.RawMessage) (any, error) {
	if method != "shell_execute" {
		return nil, fmt.Errorf("unknown shell method %q", method)
	}
	var p struct {
		Command string  `json:"command"`
		Cwd     string  `json:"cwd,omitempty"`
		Timeout float64 `json:"timeout,omitempty"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, fmt.Errorf("decoding shell_execute args: %w", err)
	}
	if p.Command == "" {
		return nil, errors.New("shell_execute requires a non-empty command")
	}
	if p.Timeout < 0 {
		return nil, fmt.Errorf("shell_execute timeout must be non-negative, got %g", p.Timeout)
	}
	timeout := s.DefaultTimeout
	if p.Timeout > 0 {
		timeout = time.Duration(p.Timeout * float64(time.Second))
	}
	dir, err := s.resolveDir(p.Cwd)
	if err != nil {
		return nil, err
	}
	if s.OnExec != nil {
		return s.OnExec(ctx, p.Command, dir, timeout)
	}
	return s.run(ctx, p.Command, dir, timeout)
}

// resolveDir validates that dir is under one of the allowed roots. An empty dir
// defaults to s.Cwd. Symlinks are resolved to prevent symlink-based directory traversal.
func (s *Shell) resolveDir(dir string) (string, error) {
	if dir == "" {
		return s.Cwd, nil
	}
	return resolveUnderRoots(s.allowedRoots, dir, false)
}

// childEnvWithAgent returns parent with any existing AI_AGENT entry stripped
// and AI_AGENT=neo appended.
func childEnvWithAgent(parent []string) []string {
	out := make([]string, 0, len(parent)+1)
	for _, kv := range parent {
		if !strings.HasPrefix(kv, "AI_AGENT=") {
			out = append(out, kv)
		}
	}
	return append(out, "AI_AGENT=neo")
}

// ShellInvocation wraps a shell command string in the platform shell used to run
// it: `sh -c` on Unix, `cmd /C` on Windows. Both the local runner here and the
// Neo ACP editor-terminal runner call this so the two paths invoke commands
// identically.
func ShellInvocation(command string) (program string, args []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/C", command}
	}
	return "sh", []string{"-c", command}
}

// ShellResult is the structured outcome of running a shell command and the
// canonical shape the `shell` tool returns to the agent. Both the local runner
// and the ACP editor-terminal runner return it directly, so the wire shape can't
// drift between them. Its JSON tags define that wire shape: stdout, stderr, and
// exit_code are always present; error and the boolean flags appear only when set.
type ShellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	// Err is a transport/exec failure that isn't an exit code (e.g. the command
	// could not be started). Omitted when empty.
	Err       string `json:"error,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	TimedOut  bool   `json:"timed_out,omitempty"`
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

func (s *Shell) run(ctx context.Context, command string, dir string, timeout time.Duration) (ShellResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	program, args := ShellInvocation(command)
	cmd := exec.CommandContext(runCtx, program, args...)
	cmd.Dir = dir
	cmd.Env = childEnvWithAgent(os.Environ())

	// Run the command in its own process group so we can SIGKILL the whole tree
	// (and not just sh) when the deadline fires; without this, long-running
	// children like `kubectl logs -f` outlive sh and keep the inherited stdout
	// pipe open, hanging cmd.Wait() indefinitely.
	cmdutil.RegisterProcessGroup(cmd)
	cmd.Cancel = func() error {
		return errors.Join(cmdutil.KillChildren(cmd.Process.Pid), cmd.Process.Kill())
	}
	// Even after the tree is killed, a grandchild that inherited stdout/stderr
	// may keep the pipes open. WaitDelay forces cmd.Wait to return after this
	// grace period instead of blocking forever on the inherited pipes.
	cmd.WaitDelay = 5 * time.Second

	stdout := &cappedBuffer{limit: maxOutputBytes}
	stderr := &cappedBuffer{limit: maxOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()

	res := ShellResult{
		Stdout:    stdout.buf.String(),
		Stderr:    stderr.buf.String(),
		Truncated: stdout.truncated || stderr.truncated,
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
		} else {
			res.Err = err.Error()
		}
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
		return res, fmt.Errorf("shell command timed out after %s", timeout)
	}
	return res, nil
}
