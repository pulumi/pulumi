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

import "charm.land/bubbles/v2/key"

// keyMap is the global keybinding set for the TUI. All global actions
// use ctrl modifiers (plus `esc` for back) so they never collide with
// typing in the list filter, huh form inputs, or the body textarea.
type keyMap struct {
	Quit key.Binding
	Back key.Binding

	// Browse-tab actions.
	Select key.Binding

	// Request-tab actions.
	Send         key.Binding
	SendToStdout key.Binding
	TogglePaging key.Binding
	EditBody     key.Binding

	// Response-tab actions.
	Filter  key.Binding
	Headers key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "configure"),
		),
		Send: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "send"),
		),
		SendToStdout: key.NewBinding(
			key.WithKeys("ctrl+y"),
			key.WithHelp("ctrl+y", "send & exit to stdout"),
		),
		TogglePaging: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "toggle paginate"),
		),
		EditBody: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("ctrl+e", "edit body in $EDITOR"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "jq filter"),
		),
		Headers: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "toggle headers"),
		),
	}
}
