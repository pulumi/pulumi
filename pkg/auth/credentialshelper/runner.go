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

package credentialshelper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Runner executes one configured helper with literal arguments.
type Runner struct {
	Path string
	Args []string
	// Env is the subprocess environment. A nil slice inherits the CLI environment.
	Env []string
	// Stderr receives helper diagnostics. If nil, diagnostics go to os.Stderr.
	Stderr io.Writer
}

// Run invokes the helper with a 30-second timeout. A nil response means the helper declined.
func (r Runner) Run(ctx context.Context, request Request, previous *Response) (*Response, error) {
	if !filepath.IsAbs(r.Path) {
		return nil, errors.New("credentials helper path must be absolute")
	}
	if err := request.validate(previous); err != nil {
		return nil, err
	}
	wireRequest := struct {
		Version int `json:"version"`
		Request
	}{Version: 1, Request: request}
	input, err := json.Marshal(wireRequest)
	if err != nil {
		return nil, fmt.Errorf("encoding credentials helper request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Path, r.Args...)
	cmd.Env = r.Env
	cmd.Stdin = bytes.NewReader(append(input, '\n'))
	cmd.Stderr = r.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	// Descendants may outlive the helper while keeping its output pipes open.
	cmd.WaitDelay = time.Second
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("running credentials helper: %w", ctx.Err())
		}
		return nil, fmt.Errorf("running credentials helper: %w", err)
	}
	response, err := decodeResponse(stdout.Bytes(), request, previous)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials helper response: %w", err)
	}
	return response, nil
}
