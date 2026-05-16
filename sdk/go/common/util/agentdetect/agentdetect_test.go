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

package agentdetect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "explicit AI_AGENT wins",
			env: map[string]string{
				"AI_AGENT":        "my-agent",
				"CODEX_THREAD_ID": "thread",
			},
			want: "my-agent",
		},
		{
			name: "normalize copilot cli alias",
			env: map[string]string{
				"AI_AGENT": "github-copilot-cli",
			},
			want: "github-copilot",
		},
		{
			name: "codex",
			env: map[string]string{
				"CODEX_THREAD_ID": "thread",
			},
			want: "codex",
		},
		{
			name: "cowork beats claude",
			env: map[string]string{
				"CLAUDE_CODE_IS_COWORK": "1",
				"CLAUDE_CODE":           "1",
			},
			want: "cowork",
		},
		{
			name: "copilot vars",
			env: map[string]string{
				"COPILOT_MODEL": "gpt-5",
			},
			want: "github-copilot",
		},
		{
			name: "none",
			env:  map[string]string{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getEnv := func(key string) string {
				return tt.env[key]
			}
			assert.Equal(t, tt.want, Detect(getEnv))
		})
	}
}
