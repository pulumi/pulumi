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
// Commands [Offer] hints while they run, and the root command calls [Flush] when it exits. Flush shows
// at most one hint per command. A hint is shown only if hints are enabled, the user hasn't turned that
// hint off, the user has completed at least one successful update (unless [Hint.BeforeFirstUpdate] is set),
// and the hint's own [Hint.Cooldown] has passed.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/ciutil"
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
	// BeforeFirstUpdate is true for hints that may be shown before the user has completed a successful update,
	// because they are meant for users who are just starting out.
	BeforeFirstUpdate bool
}

// state is the content of the hints state file.
type state struct {
	// SuccessfulUpdate is true once the user has completed a `pulumi up`. Most hints are not shown before that.
	SuccessfulUpdate bool `json:"successfulUpdate,omitempty"`
	// LastShown records when each hint was last shown, by hint ID.
	LastShown map[string]time.Time `json:"lastShown,omitempty"`
}

// manager holds the hints offered during this CLI process.
type manager struct {
	mu         sync.Mutex
	pending    []Hint
	suppressed bool

	now       func() time.Time
	statePath func() (string, error)
	settings  func() settings
	isCI      func() bool
}

func newManager() *manager {
	return &manager{
		now:       time.Now,
		statePath: func() (string, error) { return workspace.GetPulumiPath(stateFileName) },
		settings:  loadSettings,
		isCI:      ciutil.IsCI,
	}
}

var defaultManager = newManager()

// Offer queues a hint to be shown when the command exits, subject to the rules described in the package
// documentation. Offering a hint does not guarantee that it is shown.
func Offer(h Hint) { defaultManager.offer(h) }

// Suppress prevents any hint from being shown by this process, for example because the command writes
// machine-readable output.
func Suppress() { defaultManager.suppress() }

// RecordSuccessfulUpdate notes that the user has completed a successful update, which enables hints.
func RecordSuccessfulUpdate() { defaultManager.recordSuccessfulUpdate() }

// Flush shows at most one of the offered hints on w and clears the queue. It never fails the command:
// errors reading or writing the state file are logged, and no hint is shown.
func Flush(w io.Writer) { defaultManager.flush(w) }

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

func (m *manager) offer(h Hint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending = append(m.pending, h)
}

func (m *manager) suppress() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.suppressed = true
}

func (m *manager) recordSuccessfulUpdate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, path, err := m.load()
	if err != nil {
		slog.Info("could not read hints state", "err", err)
		return
	}
	if st.SuccessfulUpdate {
		return
	}
	st.SuccessfulUpdate = true
	if err := save(path, st); err != nil {
		slog.Info("could not write hints state", "err", err)
	}
}

func (m *manager) flush(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pending := m.pending
	m.pending = nil
	if len(pending) == 0 || m.suppressed || m.isCI() {
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
	for _, h := range pending {
		if settings.skip[h.ID] {
			continue
		}
		if !st.SuccessfulUpdate && !h.BeforeFirstUpdate {
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
	label := cmdutil.EmojiOr("💡 ", "") + "Hint:"
	// Line up the later lines of the message with its first line. The emoji is two columns wide.
	indent := cmdutil.EmojiOr("   ", "") + "      "
	message := strings.ReplaceAll(h.Message, "\n", "\n"+indent)
	msg := fmt.Sprintf("\n%s%s%s %s\n", colors.SpecInfo, label, colors.Reset, message)
	_, err := fmt.Fprint(w, cmdutil.GetGlobalColorization().Colorize(msg))
	contract.IgnoreError(err)
}

func (m *manager) load() (state, string, error) {
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

// save atomically replaces the state file, so that concurrent CLI processes never see a partial file.
func save(path string, st state) error {
	raw, err := json.MarshalIndent(st, "", "    ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "hints-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		contract.IgnoreClose(tmp)
		contract.IgnoreError(os.Remove(tmp.Name()))
		return err
	}
	if err := tmp.Close(); err != nil {
		contract.IgnoreError(os.Remove(tmp.Name()))
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil { //nolint:forbidigo // atomic replace of a state file
		contract.IgnoreError(os.Remove(tmp.Name()))
		return err
	}
	return nil
}
