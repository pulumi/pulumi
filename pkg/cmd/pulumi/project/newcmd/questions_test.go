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

package newcmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
)

// Ctrl-C is simulated the way the real prompts report it; see isCancelled.

// cancellingPrompt answers text prompts with their defaults and cancels the first prompt whose
// valueType contains cancelAt after skipping skip of them, along with any prompt asked after that.
// When asked is non-nil, it records every prompt.
func cancellingPrompt(cancelAt string, skip int, asked *[]string) promptForValueFunc {
	cancelled := false
	return func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		if asked != nil {
			*asked = append(*asked, valueType)
		}
		if !cancelled && cancelAt != "" && strings.Contains(valueType, cancelAt) {
			if skip == 0 {
				cancelled = true
			}
			skip--
		}
		if cancelled {
			// Cancel again so a command that keeps asking cannot loop.
			return "", io.EOF
		}
		return defaultValue, nil
	}
}

// assertCancelled checks, in separate subtests, that a cancelled `pulumi new` left nothing behind and
// said so.
func assertCancelled(t *testing.T, err error, dir string, created []string) {
	t.Helper()

	require.Error(t, err, "a cancelled `pulumi new` must fail")

	t.Run("nothing left behind", func(t *testing.T) {
		entries, readErr := os.ReadDir(dir)
		require.NoError(t, readErr)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.Empty(t, names, "cancelling must not leave project files behind")
		assert.Empty(t, created, "cancelling must not leave a stack behind")
		require.NoError(t, ErrorIfNotEmptyDirectory(dir), "re-running `pulumi new` here must not be blocked")
	})

	t.Run("says no project was created", func(t *testing.T) {
		assert.ErrorIs(t, err, errNoProjectCreated)
	})
}

//nolint:paralleltest // changes directory for process, mocks login manager
func TestNewCancelLeavesNothingBehind(t *testing.T) {
	cases := []struct {
		name string
		// guided runs `pulumi new` with no template argument; otherwise the template is named.
		guided   bool
		template string
		// cancelAt is a substring of the text prompt to cancel at.
		cancelAt string
		// selects scripts the guided select prompts, including a terminal.InterruptErr to cancel one.
		selects        []any
		runtimeOptions runtimeOptionsFunc
	}{
		{
			name:           "named template/project name",
			template:       localTemplateYAML,
			cancelAt:       "Project name",
			runtimeOptions: runtimeOptionsNone,
		},
		{
			name:           "named template/project description",
			template:       localTemplateYAML,
			cancelAt:       "Project description",
			runtimeOptions: runtimeOptionsNone,
		},
		{
			name:           "guided/confirmation",
			guided:         true,
			template:       guidedConfigTemplateYAML,
			selects:        []any{"AWS", "Python", terminal.InterruptErr},
			runtimeOptions: runtimeOptionsNone,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempProjectDir(t)
			t.Chdir(dir)

			var templateArg string
			if tc.guided {
				guidedRepoTemplate(t, "aws-python", tc.template)
			} else {
				templateArg = writeLocalTemplate(t, t.TempDir(), "template", tc.template)
			}

			b, created := guidedNewTestBackend(t)
			mockCurrentBackend(t, b)

			selectOne, _ := scriptedSelect(t, tc.selects...)
			var out, errOut bytes.Buffer
			err := runNew(t.Context(), newArgs{
				interactive:          true,
				templateNameOrURL:    templateArg,
				prompt:               cancellingPrompt(tc.cancelAt, 0, nil),
				promptRuntimeOptions: tc.runtimeOptions,
				languageTemplate:     languageTemplateMock,
				selectOne:            selectOne,
				secretsProvider:      "default",
				stdout:               &out,
				stderr:               &errOut,
			})

			assertCancelled(t, err, dir, *created)
		})
	}
}
