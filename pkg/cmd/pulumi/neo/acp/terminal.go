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

package acp

import (
	"context"
	"errors"
)

// ClientTerminal runs a command in the editor's terminal via the ACP terminal/*
// methods, so shell tool calls execute in the editor (with its own UI and
// environment) instead of as a local child process. Gate its use on the client
// having advertised the terminal capability during initialize.
//
// https://agentclientprotocol.com/protocol/terminals
type ClientTerminal struct {
	Caller    Caller
	SessionID string
}

// TerminalResult is the outcome of running a command in the editor's terminal.
// The editor merges stdout and stderr into a single Output stream. ExitCode is
// 0 when the process was terminated by a signal (Signal is then set). TimedOut
// is true when the run was killed because the caller's context expired.
type TerminalResult struct {
	Output    string
	ExitCode  int
	Signal    string
	Truncated bool
	TimedOut  bool
}

type createTerminalParams struct {
	SessionID       string   `json:"sessionId"`
	Command         string   `json:"command"`
	Args            []string `json:"args,omitempty"`
	Cwd             string   `json:"cwd,omitempty"`
	OutputByteLimit int      `json:"outputByteLimit,omitempty"`
}

type createTerminalResult struct {
	TerminalID string `json:"terminalId"`
}

// terminalRef identifies a created terminal for the output/wait/kill/release
// methods.
type terminalRef struct {
	SessionID  string `json:"sessionId"`
	TerminalID string `json:"terminalId"`
}

// exitStatus is the process termination status. ExitCode is null when the
// process was terminated by a signal.
type exitStatus struct {
	ExitCode *int   `json:"exitCode"`
	Signal   string `json:"signal,omitempty"`
}

type terminalOutputResult struct {
	Output     string      `json:"output"`
	Truncated  bool        `json:"truncated"`
	ExitStatus *exitStatus `json:"exitStatus,omitempty"`
}

// Run creates a terminal for command (with args, in cwd), waits for it to exit,
// collects its output, and releases the terminal. outputByteLimit caps the
// captured output (0 means the client's default). If ctx expires while waiting,
// the command is killed, whatever output it produced is returned with TimedOut
// set, and ctx's error is returned. The terminal is always released.
func (c *ClientTerminal) Run(
	ctx context.Context, command string, args []string, cwd string, outputByteLimit int,
) (TerminalResult, error) {
	var created createTerminalResult
	if err := c.Caller.Call(ctx, "terminal/create", createTerminalParams{
		SessionID:       c.SessionID,
		Command:         command,
		Args:            args,
		Cwd:             cwd,
		OutputByteLimit: outputByteLimit,
	}, &created); err != nil {
		return TerminalResult{}, err
	}

	ref := terminalRef{SessionID: c.SessionID, TerminalID: created.TerminalID}
	// Always release, even on timeout/cancel, so the editor frees the terminal.
	defer func() { _ = c.Caller.Call(context.WithoutCancel(ctx), "terminal/release", ref, nil) }()

	var exit exitStatus
	if err := c.Caller.Call(ctx, "terminal/wait_for_exit", ref, &exit); err != nil {
		// The wait failed. Usually ctx expired (the caller's timeout fired);
		// it can also be a transport error. Either way, kill the command and
		// return whatever output it produced so the agent sees partial results.
		// Only flag TimedOut when ctx actually hit its deadline — matching the
		// local shell tool — so a transport failure isn't mislabeled as a timeout.
		_ = c.Caller.Call(context.WithoutCancel(ctx), "terminal/kill", ref, nil)
		out, _ := c.fetchOutput(context.WithoutCancel(ctx), ref)
		out.TimedOut = errors.Is(err, context.DeadlineExceeded)
		return out, err
	}

	out, err := c.fetchOutput(ctx, ref)
	if err != nil {
		return TerminalResult{}, err
	}
	out.ExitCode = derefInt(exit.ExitCode)
	out.Signal = exit.Signal
	return out, nil
}

// fetchOutput reads the terminal's accumulated output and (if it has exited) its
// exit status.
func (c *ClientTerminal) fetchOutput(ctx context.Context, ref terminalRef) (TerminalResult, error) {
	var o terminalOutputResult
	if err := c.Caller.Call(ctx, "terminal/output", ref, &o); err != nil {
		return TerminalResult{}, err
	}
	res := TerminalResult{Output: o.Output, Truncated: o.Truncated}
	if o.ExitStatus != nil {
		res.ExitCode = derefInt(o.ExitStatus.ExitCode)
		res.Signal = o.ExitStatus.Signal
	}
	return res, nil
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
