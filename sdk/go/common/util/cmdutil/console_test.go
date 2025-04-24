// Copyright 2016-2021, Pulumi Corporation.
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

package cmdutil

import (
	"regexp"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/stretchr/testify/assert"
)

func TestMeasureText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text     string
		expected int
	}{
		{
			text:     "",
			expected: 0,
		},
		{
			text:     "a",
			expected: 1,
		},
		{
			text:     "├",
			expected: 1,
		},
		{
			text:     "├─  ",
			expected: 4,
		},
		{
			text:     "\x1b[4m\x1b[38;5;12mType\x1b[0m",
			expected: 4,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.text, func(t *testing.T) {
			t.Parallel()

			count := MeasureText(c.text)
			assert.Equal(t, c.expected, count)
		})
	}
}

func TestTablePrinting(t *testing.T) {
	t.Parallel()

	rows := []TableRow{
		{Columns: []string{"A", "B", "C"}},
		{Columns: []string{"Some A", "B", "Some C"}},
	}

	table := &Table{
		Headers: []string{"ColumnA", "Long column B", "C"},
		Rows:    rows,
		Prefix:  "  ",
	}

	expected := "" +
		"  ColumnA  Long column B  C\n" +
		"  A        B              C\n" +
		"  Some A   B              Some C\n"

	assert.Equal(t, expected, table.ToStringWithGap("  "))
}

func TestColorTablePrinting(t *testing.T) {
	t.Parallel()

	greenText := func(msg string) string {
		return colors.Always.Colorize(colors.Green + msg + colors.Reset)
	}

	rows := []TableRow{
		{Columns: []string{greenText("+"), "pulumi:pulumi:Stack", "aws-cs-webserver-test", greenText("create")}},
		{Columns: []string{greenText("+"), "├─ aws:ec2/instance:Instance", "web-server-www", greenText("create")}},
		{Columns: []string{greenText("+"), "├─ aws:ec2/securityGroup:SecurityGroup", "web-secgrp", greenText("create")}},
		{Columns: []string{greenText("+"), "└─ pulumi:providers:aws", "default_4_25_0", greenText("create")}},
	}

	columnHeader := func(msg string) string {
		return colors.Always.Colorize(colors.Underline + colors.BrightBlue + msg + colors.Reset)
	}

	table := &Table{
		Headers: []string{"", columnHeader("Type"), columnHeader("Name"), columnHeader("Plan")},
		Rows:    rows,
		Prefix:  "  ",
	}

	expected := "" +
		"     Type                                    Name                   Plan\n" +
		"  +  pulumi:pulumi:Stack                     aws-cs-webserver-test  create\n" +
		"  +  ├─ aws:ec2/instance:Instance            web-server-www         create\n" +
		"  +  ├─ aws:ec2/securityGroup:SecurityGroup  web-secgrp             create\n" +
		"  +  └─ pulumi:providers:aws                 default_4_25_0         create\n"

	colorTable := table.ToStringWithGap("  ")
	// 7-bit C1 ANSI sequences
	ansiEscape := regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)
	cleanTable := ansiEscape.ReplaceAllString(colorTable, "")

	assert.Equal(t, expected, cleanTable)
}

func TestIsTruthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give string
		want bool
	}{
		{"1", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"0", false},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, IsTruthy(tt.give))
		})
	}
}

func TestReadConsoleFancy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc  string
		model readConsoleModel

		// Prompt expected on the command line, if any
		// after which we begin typing.
		expectPrompt string

		// Messages to send to the model in order.
		// Does not include Enter or Ctrl+C.
		giveMsgs []tea.Msg

		// Output visible before we hit Enter.
		wantEcho string

		// Output visible after we hit Enter.
		wantAccepted string

		// Value returned by the model.
		wantValue string
	}{
		{
			desc:         "plain",
			model:        newReadConsoleModel("Enter a value", false /* secret */),
			expectPrompt: "Enter a value: ",
			giveMsgs: []tea.Msg{
				tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune("hello"),
				},
			},
			wantEcho:     "Enter a value: hello",
			wantAccepted: "Enter a value: hello ",
			wantValue:    "hello",
		},
		{
			desc:         "secret",
			model:        newReadConsoleModel("Password", true /* secret */),
			expectPrompt: "Password: ",
			giveMsgs: []tea.Msg{
				tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune("hunter2"),
				},
			},
			wantEcho:     "Password: *******",
			wantAccepted: "Password: ",
			wantValue:    "hunter2",
		},
		{
			desc:  "no prompt",
			model: newReadConsoleModel("" /* prompt */, false /* secret */),
			giveMsgs: []tea.Msg{
				tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune("hello"),
				},
			},
			wantEcho:     "> hello",
			wantAccepted: "> hello ",
			wantValue:    "hello",
		},
		{
			desc:  "backspace",
			model: newReadConsoleModel("" /* prompt */, false /* secret */),
			giveMsgs: []tea.Msg{
				tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune("foobar"),
				},
				tea.KeyMsg{Type: tea.KeyBackspace},
				tea.KeyMsg{Type: tea.KeyBackspace},
				tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune("az"),
				},
			},
			wantEcho:     "> foobaz",
			wantAccepted: "> foobaz ",
			wantValue:    "foobaz",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			var m tea.Model = tt.model
			m.Init()

			if tt.expectPrompt != "" {
				assert.Contains(t, m.View(), tt.expectPrompt,
					"initial view should contain prompt")
			}

			for _, msg := range tt.giveMsgs {
				m, _ = m.Update(msg)
			}

			assert.Contains(t, m.View(), tt.wantEcho,
				"prompt before pressing enter did not match")

			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

			assert.Contains(t, m.View(), tt.wantAccepted,
				"prompt after pressing enter did not match")

			final, ok := m.(readConsoleModel)
			assert.True(t, ok, "expected readConsoleModel, got %T", m)

			assert.Equal(t, tt.wantValue, final.Value, "final value should match")
			assert.False(t, final.Canceled, "should not be canceled")
		})
	}
}

func TestReadConsoleFancy_cancel(t *testing.T) {
	t.Parallel()

	var m tea.Model = newReadConsoleModel("Name:", false /* secret */)
	m.Init()

	m, _ = m.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("hello"),
	})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	final, ok := m.(readConsoleModel)
	assert.True(t, ok, "expected readConsoleModel, got %T", m)
	assert.True(t, final.Canceled, "should be canceled")
}
