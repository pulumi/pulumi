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

// Package tui is the Bubble Tea application for `pulumi cloud api`.
//
// The parent `api` package exports data types, helpers, and the HTTP
// client the TUI relies on; this package consumes them and surfaces a
// three-tab UI (Browse, Request, Response). The TUI is wired in at init
// time so the parent package never imports this one — see Run below and
// the blank import in cmd/pulumi/pulumi.go.
package tui

import (
	"context"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/api"
)

// init registers this package's Run function with the parent api package
// so `runInteractive` can dispatch through it without creating an import
// cycle.
func init() {
	api.InteractiveRunner = Run
}

// Run starts the Bubble Tea TUI. On exit it returns the optional stdout
// payload (non-nil when the user hit the send-to-stdout keybinding) plus
// any fatal error. Non-fatal errors (e.g. a single failed send) are
// surfaced inside the TUI and do not propagate.
func Run(ctx context.Context, idx *api.Index, cfg api.TUIConfig) ([]byte, error) {
	m := newRootModel(ctx, idx, cfg)
	p := tea.NewProgram(m,
		tea.WithContext(ctx),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stderr),
	)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	if fm, ok := final.(rootModel); ok {
		return fm.stdoutOnExit, fm.lastError
	}
	return nil, nil
}
