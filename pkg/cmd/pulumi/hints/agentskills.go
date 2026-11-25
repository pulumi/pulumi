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
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// OfferAgentSkills offers the agent skills hint for a project created in dir, unless Pulumi's agent skills appear
// to be installed.
func (m *Manager) OfferAgentSkills(dir string) {
	// Looking for the skills reads many files, so do it only if the hint would otherwise be shown.
	m.OfferIf(AgentSkills, func() bool {
		// Without a home directory there's nowhere to look, so assume the skills aren't installed.
		home, err := os.UserHomeDir()
		return err != nil || !agentSkillsInstalled(os.Getenv, home, dir)
	})
}

// pulumiSkills are skills from each of pulumi/agent-skills' end-user groups. Finding any of them is enough.
var pulumiSkills = []string{
	"pulumi-best-practices", "pulumi-overview", "pulumi-esc", "pulumi-terraform-to-pulumi", "pulumi-neo-handoff",
}

// agentSkillsInstalled reports whether Pulumi's agent skills look installed for an agent working in dir. It only
// looks at local files, and an install it doesn't recognize counts as not installed.
func agentSkillsInstalled(getEnv func(string) string, home, dir string) bool {
	claudeHome := cmp.Or(strings.TrimSpace(getEnv("CLAUDE_CONFIG_DIR")), filepath.Join(home, ".claude"))
	configHome := cmp.Or(strings.TrimSpace(getEnv("XDG_CONFIG_HOME")), filepath.Join(home, ".config"))

	// Where `npx skills add` installs skills globally for the agents Pulumi detects. See
	// https://github.com/vercel-labs/skills/blob/main/src/agents.ts.
	skillDirs := []string{
		filepath.Join(claudeHome, "skills"),
		filepath.Join(configHome, "agents", "skills"),
		filepath.Join(configHome, "opencode", "skills"),
		filepath.Join(configHome, "goose", "skills"),
		filepath.Join(home, ".gemini", "config", "skills"),
	}
	for _, agent := range []string{".agents", ".cursor", ".codex", ".gemini", ".copilot", ".augment"} {
		skillDirs = append(skillDirs, filepath.Join(home, agent, "skills"))
	}
	// Claude Code plugins from the pulumi/agent-skills marketplace are listed in these files.
	claudeFiles := []string{
		filepath.Join(claudeHome, "plugins", "installed_plugins.json"),
		filepath.Join(claudeHome, "settings.json"),
	}

	// Skills installed for a project live in the project, which for `pulumi new` may be any directory above dir.
	for d := dir; ; d = filepath.Dir(d) {
		for _, agent := range []string{".agents", ".claude", ".augment", ".goose"} {
			skillDirs = append(skillDirs, filepath.Join(d, agent, "skills"))
		}
		claudeFiles = append(claudeFiles,
			filepath.Join(d, ".claude", "settings.json"), filepath.Join(d, ".claude", "settings.local.json"))
		if filepath.Dir(d) == d {
			break
		}
	}

	// The Gemini CLI installs the skills as an extension rather than into a skills directory.
	markers := make([]string, 0, 1+len(skillDirs)*len(pulumiSkills))
	markers = append(markers, filepath.Join(home, ".gemini", "extensions", "pulumi"))
	for _, sd := range skillDirs {
		for _, skill := range pulumiSkills {
			markers = append(markers, filepath.Join(sd, skill, "SKILL.md"))
		}
	}
	return slices.ContainsFunc(markers, exists) || slices.ContainsFunc(claudeFiles, listsPulumiPlugin)
}

// listsPulumiPlugin reports whether the Claude Code file at path, either installed_plugins.json or a settings file,
// lists a plugin from the pulumi/agent-skills marketplace as installed or enabled.
func listsPulumiPlugin(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var f struct {
		Plugins        map[string]json.RawMessage `json:"plugins"`
		EnabledPlugins map[string]bool            `json:"enabledPlugins"`
	}
	if json.Unmarshal(raw, &f) != nil {
		return false
	}
	isPulumi := func(id string) bool { return strings.HasSuffix(id, "@pulumi-agent-skills") }
	for id := range f.Plugins {
		if isPulumi(id) {
			return true
		}
	}
	for id, enabled := range f.EnabledPlugins {
		if enabled && isPulumi(id) {
			return true
		}
	}
	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
