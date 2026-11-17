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

package ui

import (
	"io"
	"os"
	"runtime"
	"testing"
	"time"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Subtests replace process stdin/stdout.
func TestAskOneTerminalStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTYs are not supported on Windows")
	}
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")
	for _, err := range []error{nil, terminal.InterruptErr} {
		name := "answered"
		if err != nil {
			name = "interrupted"
		}
		t.Run(name, func(t *testing.T) {
			ptyController, programTTY, openErr := pty.Open()
			require.NoError(t, openErr)
			t.Cleanup(func() {
				require.NoError(t, ptyController.Close())
				require.NoError(t, programTTY.Close())
			})
			stdin, stdout := os.Stdin, os.Stdout
			os.Stdin, os.Stdout = programTTY, programTTY
			t.Cleanup(func() { os.Stdin, os.Stdout = stdin, stdout })

			prompt := &statusTestPrompt{prompt: func(config *survey.PromptConfig) (any, error) {
				assert.Equal(t, 12, config.PageSize)
				return "answer", err
			}}
			var answer string
			actualErr := AskOne(prompt, &answer, nil, survey.WithPageSize(12),
				survey.WithStdio(programTTY, programTTY, programTTY))
			if err == nil {
				require.NoError(t, actualErr)
				assert.Equal(t, "answer", answer)
			} else {
				assert.ErrorIs(t, actualErr, err)
				assert.Empty(t, answer)
			}

			want := "\x1b]7501;state=blocked:id=pulumi/prompt:app=pulumi\x1b\\" +
				"\x1b]7501;state=clear:id=pulumi/prompt:app=pulumi\x1b\\"
			got := make([]byte, len(want))
			read := make(chan error, 1)
			go func() {
				_, err := io.ReadFull(ptyController, got)
				read <- err
			}()
			select {
			case err := <-read:
				require.NoError(t, err)
				assert.Equal(t, want, string(got))
			case <-time.After(5 * time.Second):
				t.Fatal("timed out waiting for prompt status reports")
			}
		})
	}
}

type statusTestPrompt struct {
	prompt func(*survey.PromptConfig) (any, error)
}

func (p *statusTestPrompt) Prompt(config *survey.PromptConfig) (any, error) {
	return p.prompt(config)
}

func (*statusTestPrompt) Cleanup(*survey.PromptConfig, any) error { return nil }
func (*statusTestPrompt) Error(*survey.PromptConfig, error) error { return nil }
