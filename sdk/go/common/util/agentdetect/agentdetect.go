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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Metadata describes the detected AI coding agent environment.
type Metadata struct {
	Name  string
	Model string
}

type detector struct {
	name string
	envs []string
}

// These are sourced from https://github.com/unjs/std-env/blob/main/src/agents.ts and
// https://github.com/vercel/vercel/blob/main/packages/detect-agent/src/index.ts, as a reference for common
// environment variables set by AI agents and tools.
//
// Order matters: specific forms should be identified before broad IDE/tool markers.
var agents = []detector{
	{name: "cursor", envs: []string{"CURSOR_TRACE_ID"}},
	{name: "cursor-cli", envs: []string{"CURSOR_AGENT"}},
	{name: "gemini", envs: []string{"GEMINI_CLI"}},
	{name: "codex", envs: []string{"CODEX_SANDBOX", "CODEX_CI", "CODEX_THREAD_ID"}},
	{name: "antigravity", envs: []string{"ANTIGRAVITY_AGENT"}},
	{name: "augment-cli", envs: []string{"AUGMENT_AGENT"}},
	{name: "opencode", envs: []string{"OPENCODE", "OPENCODE_CALLER", "OPENCODE_CLIENT"}},
	{name: "cowork", envs: []string{"CLAUDE_CODE_IS_COWORK"}},
	{name: "claude", envs: []string{"CLAUDECODE", "CLAUDE_CODE"}},
	{name: "replit", envs: []string{"REPL_ID"}},
	{name: "github-copilot", envs: []string{"COPILOT_MODEL", "COPILOT_ALLOW_ALL", "COPILOT_GITHUB_TOKEN"}},
	{name: "goose", envs: []string{"GOOSE_PROVIDER"}},
}

func DetectionEnvVars() []string {
	vars := []string{"AI_AGENT"}
	for _, d := range agents {
		vars = append(vars, d.envs...)
	}
	return vars
}

// Detect returns a normalized name for the AI coding agent driving
// the CLI (e.g. "claude", "cursor", "codex"), or "" if none is detected.
// Detection is based on environment variables.
func Detect(getEnv func(string) string) string {
	normalized := func(agent string) string {
		agent = strings.TrimSpace(strings.ToLower(agent))
		switch agent {
		case "github-copilot-cli":
			return "github-copilot"
		default:
			return agent
		}
	}
	if agent := normalized(getEnv("AI_AGENT")); agent != "" {
		return agent
	}

	for _, d := range agents {
		for _, envVar := range d.envs {
			if getEnv(envVar) != "" {
				return d.name
			}
		}
	}

	return ""
}

// OtherAgent is what KnownName returns for an agent it doesn't recognize.
const OtherAgent = "other"

// knownNameAliases maps names that agents report for themselves through
// AI_AGENT to the name of the matching detector above.
var knownNameAliases = map[string]string{
	"claude-code":        "claude",
	"github-copilot-cli": "github-copilot",
}

// KnownName reduces an agent name, as returned by Detect, to one of the names
// in the detector list above, or OtherAgent if it isn't one of them. It
// returns "" when no agent was detected.
//
// Detect passes an AI_AGENT value through as is, and agents put more than
// their name in it: Claude Code sets "claude-code_<version>_agent". KnownName
// keeps only the part before the first underscore, so the result never
// carries a version or other free-form text.
func KnownName(detected string) string {
	name := strings.TrimSpace(strings.ToLower(detected))
	if name == "" {
		return ""
	}
	name, _, _ = strings.Cut(name, "_")
	if alias, ok := knownNameAliases[name]; ok {
		return alias
	}
	for _, d := range agents {
		if d.name == name {
			return name
		}
	}
	return OtherAgent
}

// DetectMetadata returns the normalized agent name and best-effort model
// information for the AI coding agent driving the CLI.
func DetectMetadata(getEnv func(string) string) Metadata {
	name := Detect(getEnv)
	return Metadata{
		Name:  name,
		Model: DetectModel(name, getEnv),
	}
}

// DetectModel returns best-effort model information for the detected agent.
func DetectModel(agentName string, getEnv func(string) string) string {
	for _, envVar := range []string{"PULUMI_AGENT_MODEL", "AI_AGENT_MODEL"} {
		if model := strings.TrimSpace(getEnv(envVar)); model != "" {
			return model
		}
	}
	switch strings.ToLower(strings.TrimSpace(agentName)) {
	case "claude", "cowork":
		if model := strings.TrimSpace(getEnv("ANTHROPIC_MODEL")); model != "" {
			return model
		}
		return detectClaudeModel(getEnv)
	case "codex":
		return detectCodexModel(getEnv)
	case "gemini":
		return detectGeminiModel(getEnv)
	default:
		return ""
	}
}

// detectCodexModel reads Codex's local config and returns the configured model,
// if one is available.
func detectCodexModel(getEnv func(string) string) string {
	type codexConfig struct {
		Model string `toml:"model"`
	}
	var cfg codexConfig
	if readTOMLFile(filepath.Join(homeDir(getEnv), ".codex", "config.toml"), &cfg) {
		return strings.TrimSpace(cfg.Model)
	}
	return ""
}

// detectClaudeModel reads Claude's local settings and returns the configured
// model, if one is available.
func detectClaudeModel(getEnv func(string) string) string {
	type claudeSettings struct {
		Model string `json:"model"`
	}
	configDir := strings.TrimSpace(getEnv("CLAUDE_CONFIG_DIR"))
	if configDir == "" {
		configDir = filepath.Join(homeDir(getEnv), ".claude")
	}
	for _, path := range []string{
		filepath.Join(configDir, "settings.json"),
		filepath.Join(configDir, "settings.local.json"),
	} {
		var settings claudeSettings
		if readJSONFile(path, &settings) && strings.TrimSpace(settings.Model) != "" {
			return strings.TrimSpace(settings.Model)
		}
	}
	return ""
}

// detectGeminiModel reads Gemini's local settings and returns the configured
// model, if one is available.
func detectGeminiModel(getEnv func(string) string) string {
	type geminiSettings struct {
		Model string `json:"model"`
	}
	var settings geminiSettings
	if readJSONFile(filepath.Join(homeDir(getEnv), ".gemini", "settings.json"), &settings) {
		return strings.TrimSpace(settings.Model)
	}
	return ""
}

// homeDir returns the best available home directory from the supplied
// environment reader, falling back to os.UserHomeDir.
func homeDir(getEnv func(string) string) string {
	if home := strings.TrimSpace(getEnv("HOME")); home != "" {
		return home
	}
	if home := strings.TrimSpace(getEnv("USERPROFILE")); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// readTOMLFile decodes path into v and reports whether decoding succeeded.
func readTOMLFile(path string, v any) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	if _, err := toml.DecodeFile(path, v); err != nil {
		return false
	}
	return true
}

// readJSONFile decodes path into v and reports whether decoding succeeded.
func readJSONFile(path string, v any) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
}
