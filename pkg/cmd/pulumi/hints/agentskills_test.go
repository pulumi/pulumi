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

package hints

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentSkillsInstalled(t *testing.T) {
	t.Parallel()

	skill := func(dir string) map[string]string {
		return map[string]string{filepath.Join(dir, "pulumi-best-practices", "SKILL.md"): "---\nname: x\n---\n"}
	}
	tests := []struct {
		name string
		// files maps paths, relative to a root holding "home" and "work/project", to their content.
		files map[string]string
		env   map[string]string
		want  bool
	}{
		{name: "nothing installed", want: false},
		{name: "global skills", files: skill("home/.agents/skills"), want: true},
		{name: "claude skills", files: skill("home/.claude/skills"), want: true},
		{
			name:  "claude skills in CLAUDE_CONFIG_DIR",
			files: skill("claude-config/skills"),
			env:   map[string]string{"CLAUDE_CONFIG_DIR": "claude-config"},
			want:  true,
		},
		{name: "XDG config skills", files: skill("home/.config/opencode/skills"), want: true},
		{
			name:  "skills in XDG_CONFIG_HOME",
			files: skill("xdg/opencode/skills"),
			env:   map[string]string{"XDG_CONFIG_HOME": "xdg"},
			want:  true,
		},
		{name: "project skills", files: skill("work/project/.agents/skills"), want: true},
		{name: "skills in a parent directory", files: skill("work/.claude/skills"), want: true},
		{
			name:  "other skills only",
			files: map[string]string{"home/.agents/skills/something-else/SKILL.md": ""},
			want:  false,
		},
		{
			name:  "skill directory without SKILL.md",
			files: map[string]string{"home/.agents/skills/pulumi-best-practices/README.md": ""},
			want:  false,
		},
		{
			name: "claude plugin installed",
			files: map[string]string{
				"home/.claude/plugins/installed_plugins.json": `{"version": 2, "plugins": {"pulumi@pulumi-agent-skills": []}}`,
			},
			want: true,
		},
		{
			name: "other claude plugin installed",
			files: map[string]string{
				"home/.claude/plugins/installed_plugins.json": `{"version": 2, "plugins": {"slack@claude-plugins-official": []}}`,
			},
			want: false,
		},
		{
			name: "claude plugin enabled for the project",
			files: map[string]string{
				"work/.claude/settings.json": `{"enabledPlugins": {"pulumi@pulumi-agent-skills": true}}`,
			},
			want: true,
		},
		{
			name: "claude plugin enabled in local settings",
			files: map[string]string{
				"work/project/.claude/settings.local.json": `{"enabledPlugins": {"pulumi@pulumi-agent-skills": true}}`,
			},
			want: true,
		},
		{
			name: "claude plugin disabled",
			files: map[string]string{
				"home/.claude/settings.json": `{"enabledPlugins": {"pulumi@pulumi-agent-skills": false}}`,
			},
			want: false,
		},
		{
			name:  "corrupt claude settings",
			files: map[string]string{"home/.claude/settings.json": `{not json`},
			want:  false,
		},
		{
			name:  "gemini extension",
			files: map[string]string{"home/.gemini/extensions/pulumi/gemini-extension.json": "{}"},
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			home, project := filepath.Join(root, "home"), filepath.Join(root, "work", "project")
			require.NoError(t, os.MkdirAll(home, 0o700))
			require.NoError(t, os.MkdirAll(project, 0o700))
			for path, content := range tt.files {
				path = filepath.Join(root, path)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
				require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
			}
			getEnv := func(key string) string {
				if v, ok := tt.env[key]; ok {
					return filepath.Join(root, v)
				}
				return ""
			}
			assert.Equal(t, tt.want, agentSkillsInstalled(getEnv, home, project))
		})
	}
}
