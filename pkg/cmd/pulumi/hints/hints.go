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

// Package hints shows short, contextual product hints at the end of a CLI command.
//
// Commands [Manager.Offer] hints while they run, and the root command calls [Manager.Flush] when it exits. Flush shows
// at most one hint per command: the first offered hint that the user hasn't turned off and whose fields, such as
// [Hint.Cooldown], allow it to be shown now.
//
// Users turn all hints off with PULUMI_DISABLE_HINTS or `hints.disabled` in the Pulumi config file, and
// turn individual hints off by [Hint.ID] with PULUMI_SKIP_HINTS or `hints.skip`.
package hints

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// stateFileName is the name of the file under the Pulumi home directory that records when hints were shown.
const stateFileName = "hints.json"

// Hint is a product hint that the CLI can show.
type Hint struct {
	// ID identifies the hint in the state file. It must be stable across releases.
	ID string
	// Cooldown is the minimum time between two showings of this hint.
	Cooldown time.Duration
	// Message is the text of the hint. It may contain color directives, and newlines to start a new line, for example
	// before a "Learn more" link.
	Message string
}

// state is the content of the hints state file.
type state struct {
	// LastShown records when each hint was last shown, by hint ID.
	LastShown map[string]time.Time `json:"lastShown,omitempty"`
}

// Manager holds the hints offered while a CLI command runs. The root command creates one and passes it to the
// commands that offer hints. Its methods do nothing on a nil Manager, so commands created without one, as in tests,
// offer no hints.
type Manager struct {
	mu         sync.Mutex
	pending    []Hint
	suppressed bool

	now       func() time.Time
	statePath func() (string, error)
	settings  func() settings
}

// New returns a Manager that reads the user's hint settings and records shown hints under the Pulumi home directory.
func New() *Manager {
	return &Manager{
		now:       time.Now,
		statePath: func() (string, error) { return workspace.GetPulumiPath(stateFileName) },
		settings:  loadSettings,
	}
}

// settings are the user's choices about which hints to show.
type settings struct {
	// disabled is true if the user has turned all hints off.
	disabled bool
	// skip holds the IDs of the hints the user has turned off.
	skip map[string]bool
}

// loadSettings reads the user's hint settings from PULUMI_DISABLE_HINTS, PULUMI_SKIP_HINTS and the
// `hints` section of the Pulumi config file.
func loadSettings() settings {
	s := settings{disabled: env.DisableHints.Value(), skip: map[string]bool{}}
	for id := range strings.SplitSeq(env.SkipHints.Value(), ",") {
		if id = strings.TrimSpace(id); id != "" {
			s.skip[id] = true
		}
	}
	cfg, err := workspace.GetPulumiConfig()
	if err != nil {
		slog.Info("could not read Pulumi config for hints", "err", err)
		return s
	}
	if cfg.Hints != nil {
		s.disabled = s.disabled || cfg.Hints.Disabled
		for _, id := range cfg.Hints.Skip {
			s.skip[id] = true
		}
	}
	return s
}

// Offer queues a hint to be shown when the command exits, subject to the rules described in the package
// documentation.
func (m *Manager) Offer(h Hint) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending = append(m.pending, h)
}

// Suppress prevents any hint from being shown for the current command, for example because it writes
// machine-readable output or runs in CI.
func (m *Manager) Suppress() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.suppressed = true
}

// Flush shows at most one of the offered hints on w. It never fails the command: errors reading or writing the state
// file are logged, and no hint is shown.
func (m *Manager) Flush(w io.Writer) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pending) == 0 || m.suppressed {
		return
	}
	settings := m.settings()
	if settings.disabled {
		return
	}

	st, path, err := m.load()
	if err != nil {
		slog.Info("could not read hints state", "err", err)
		return
	}
	now := m.now()
	for _, h := range m.pending {
		if settings.skip[h.ID] {
			continue
		}
		if last, ok := st.LastShown[h.ID]; ok && now.Sub(last) < h.Cooldown {
			continue
		}

		// Record the hint before showing it, so that a hint is never shown again early because the state
		// could not be saved.
		if st.LastShown == nil {
			st.LastShown = map[string]time.Time{}
		}
		st.LastShown[h.ID] = now
		if err := save(path, st); err != nil {
			slog.Info("could not write hints state", "err", err)
			return
		}
		show(w, h)
		return
	}
}

func show(w io.Writer, h Hint) {
	const label = "Hint:"
	// Line up the later lines of the message with its first line, after the label, its space and the emoji, which
	// is two columns wide.
	indent := strings.Repeat(" ", len(label)+1) + cmdutil.EmojiOr("   ", "")
	message := strings.ReplaceAll(h.Message, "\n", "\n"+indent)
	msg := fmt.Sprintf("\n%s%s%s%s %s\n", colors.SpecInfo, cmdutil.EmojiOr("💡 ", ""), label, colors.Reset, message)
	_, err := fmt.Fprint(w, cmdutil.GetGlobalColorization().Colorize(msg))
	contract.IgnoreError(err)
}

func (m *Manager) load() (state, string, error) {
	path, err := m.statePath()
	if err != nil {
		return state{}, "", err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return state{}, path, nil
	} else if err != nil {
		return state{}, "", err
	}
	var st state
	if err := json.Unmarshal(raw, &st); err != nil {
		// A corrupt state file is replaced on the next save rather than blocking hints forever.
		slog.Info("ignoring invalid hints state", "path", path, "err", err)
		return state{}, path, nil
	}
	return st, path, nil
}

// save writes the state file. Concurrent CLI processes may read a partly written file, which load treats as
// empty, so at worst a hint is shown again before its cooldown ends.
func save(path string, st state) error {
	raw, err := json.MarshalIndent(st, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
