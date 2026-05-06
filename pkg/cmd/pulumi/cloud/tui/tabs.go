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

package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// mode is the active tab.
type mode int

const (
	modeBrowse mode = iota
	modeRequest
	modeResponse
)

// tabHit maps a horizontal cell range on the tab row back to the mode it
// renders. startCol is inclusive, endCol is exclusive (half-open), matching
// how MouseMsg X coordinates are compared elsewhere.
type tabHit struct {
	mode     mode
	startCol int
	endCol   int
}

// renderTabs renders the tab bar and returns the rendered string alongside
// the per-tab cell ranges so the root model can route mouse clicks back to
// the tab under the cursor. Key hints shown at right are rendered inline by
// the parent model's footer, not here.
func renderTabs(t Theme, active mode) (string, []tabHit) {
	labels := []string{"Browse", "Request", "Response"}
	parts := make([]string, len(labels))
	hits := make([]tabHit, len(labels))

	col := 0
	for i, label := range labels {
		if mode(i) == active {
			parts[i] = t.TabActive.Render(label)
		} else {
			parts[i] = t.TabInactive.Render(label)
		}
		w := lipgloss.Width(parts[i])
		hits[i] = tabHit{mode: mode(i), startCol: col, endCol: col + w}
		col += w
	}

	return t.TabBar.Render(strings.Join(parts, "")), hits
}
