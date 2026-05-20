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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDetectMetadata(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex"), 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(home, ".codex", "config.toml"),
		[]byte("model = \"gpt-codex-test\"\n"),
		0o600))

	getEnv := func(name string) string {
		switch name {
		case "CODEX_THREAD_ID":
			return "thread"
		case "HOME":
			return home
		default:
			return ""
		}
	}

	assert.Equal(t, Metadata{Name: "codex", Model: "gpt-codex-test"}, DetectMetadata(getEnv))
}

func TestDetectModel(t *testing.T) {
	t.Parallel()

	t.Run("explicit pulumi env", func(t *testing.T) {
		t.Parallel()
		model := DetectModel("codex", func(name string) string {
			if name == "PULUMI_AGENT_MODEL" {
				return "gpt-test"
			}
			return ""
		})
		assert.Equal(t, "gpt-test", model)
	})

	t.Run("claude env", func(t *testing.T) {
		t.Parallel()
		model := DetectModel("claude", func(name string) string {
			if name == "ANTHROPIC_MODEL" {
				return "claude-test"
			}
			return ""
		})
		assert.Equal(t, "claude-test", model)
	})

	t.Run("codex config", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex"), 0o700))
		require.NoError(t, os.WriteFile(
			filepath.Join(home, ".codex", "config.toml"),
			[]byte("model = \"gpt-codex-test\"\n"),
			0o600))
		model := DetectModel("codex", func(name string) string {
			if name == "HOME" {
				return home
			}
			return ""
		})
		assert.Equal(t, "gpt-codex-test", model)
	})

	t.Run("claude config", func(t *testing.T) {
		t.Parallel()
		configDir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(configDir, "settings.json"),
			[]byte(`{"model":"claude-config-test"}`),
			0o600))
		model := DetectModel("claude", func(name string) string {
			if name == "CLAUDE_CONFIG_DIR" {
				return configDir
			}
			return ""
		})
		assert.Equal(t, "claude-config-test", model)
	})

	t.Run("unknown", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, DetectModel("cursor", func(string) string { return "" }))
	})
}
