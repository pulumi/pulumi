// Copyright 2016-2018, Pulumi Corporation.
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

//go:build !js
// +build !js

package cmdutil

import (
	"io"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func readConsoleFancy(stdout io.Writer, stdin io.Reader, prompt string, secret bool) (string, error) {
	final, err := tea.NewProgram(
		newReadConsoleModel(prompt, secret),
		tea.WithInput(stdin),
		tea.WithOutput(stdout),
	).Run()
	if err != nil {
		return "", err
	}

	model, ok := final.(readConsoleModel)
	contract.Assertf(ok, "expected readConsoleModel, got %T", final)
	if model.Canceled {
		return "", io.EOF
	}

	return model.Value, nil
}

// readConsoleModel drives a bubbletea widget that reads from the console.
type readConsoleModel struct {
	input  textinput.Model
	secret bool

	// Canceled is set to true when the model finishes
	// if the user canceled the operation by pressing Ctrl-C or Esc.
	Canceled bool

	// Value is the user's response to the prompt.
	Value string
}

var _ tea.Model = readConsoleModel{}

func newReadConsoleModel(prompt string, secret bool) readConsoleModel {
	input := textinput.New()
	input.Cursor.Style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")) // 205 = hot pink cursor
	if secret {
		input.EchoMode = textinput.EchoPassword
	}
	if prompt != "" {
		input.Prompt = prompt + ": "
	}
	input.Focus() // required to receive input

	return readConsoleModel{
		input:  input,
		secret: secret,
	}
}

// Init initializes the model.
// We don't have any initialization to do, so we just return nil.
func (readConsoleModel) Init() tea.Cmd { return nil }

// Update handles a single tick of the bubbletea loop.
func (m readConsoleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If the user pressed enter, Ctrl-C, or Esc,
		// it's time to stop the bubbletea loop.
		//
		// Only Enter is considered a success.
		//nolint:exhaustive // We only want special handling for these keys.
		switch msg.Type {
		case tea.KeyEnter, tea.KeyCtrlC, tea.KeyEsc:
			m.Value = m.input.Value()
			m.Canceled = msg.Type != tea.KeyEnter

			m.input.Blur() // hide the cursor
			if m.secret {
				// If we're in secret mode, don't include
				// the '*' characters in the final output
				// so as not to leak the length of the input.
				m.input.EchoMode = textinput.EchoNone
			}

			var cmds []tea.Cmd
			if !m.Canceled {
				// If the user accepts the input,
				// we'll primnt the prompt to the terminal
				// before exiting this loop.
				cmds = append(cmds, tea.Println(m.input.View()))
			}
			cmds = append(cmds, tea.Quit)

			return m, tea.Sequence(cmds...)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the prompt.
func (m readConsoleModel) View() string {
	return m.input.View()
}
