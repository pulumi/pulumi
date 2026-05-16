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

import "strings"

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

	type detector struct {
		name string
		envs []string
	}
	// These are sourced from https://github.com/unjs/std-env/blob/main/src/agents.ts and
	// https://github.com/vercel/vercel/blob/main/packages/detect-agent/src/index.ts, as a reference for common
	// environment variables set by AI agents and tools.
	//
	// Order matters: specific forms should be identified before broad IDE/tool markers.
	agents := []detector{
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

	for _, d := range agents {
		for _, envVar := range d.envs {
			if getEnv(envVar) != "" {
				return d.name
			}
		}
	}

	return ""
}
