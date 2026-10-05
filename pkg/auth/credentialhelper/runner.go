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

package credentialhelper

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
	// Stderr receives helper diagnostics. If nil, diagnostics go to os.Stderr.
	Stderr io.Writer
}

// Run invokes the helper with a timeout of one minute. A nil response means the helper declined.
func (r Runner) Run(ctx context.Context, request Request) (*Response, error) {
	if !filepath.IsAbs(r.Path) {
		return nil, errors.New("credential helper path must be absolute")
	}
	request.Version = 1
	input, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encoding credential helper request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Path, r.Args...)
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
			return nil, fmt.Errorf("running credential helper: %w", ctx.Err())
		}
		return nil, fmt.Errorf("running credential helper: %w", err)
	}
	response, err := decodeResponse(stdout.Bytes(), request)
	if err != nil {
		return nil, fmt.Errorf("invalid credential helper response: %w", err)
	}
	return response, nil
}
