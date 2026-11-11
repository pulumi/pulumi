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

package telemetry

import (
	"net/url"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/ciutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

func Command(cmd *cobra.Command) KV {
	path := CommandName(cmd)
	if !slices.Contains(schema["command"].Values, path) {
		path = "other"
	}
	return enumFact("command", path)
}

func CommandName(cmd *cobra.Command) string {
	return strings.ReplaceAll(cmd.CommandPath(), " ", ":")
}

func Flags(cmd *cobra.Command) KV {
	var names []string
	cmd.Flags().Visit(func(f *pflag.Flag) { names = append(names, f.Name) })
	return setFact("flags", names...)
}

func EnvVars(store env.Store) KV {
	var names []string
	for _, v := range env.Variables() {
		name := v.Name()
		if _, ok := store.Raw(name); ok && strings.HasPrefix(name, env.Prefix) {
			names = append(names, name)
		}
	}
	return setFact("env-vars", names...)
}

func CI(ci bool) KV { return boolFact("ci", ci) }

var ciSystems = map[ciutil.SystemName]string{
	ciutil.AppVeyor:                    "appveyor",
	ciutil.AWSCodeBuild:                "aws-codebuild",
	ciutil.AtlassianBamboo:             "atlassian-bamboo",
	ciutil.AtlassianBitbucketPipelines: "atlassian-bitbucket-pipelines",
	ciutil.AzurePipelines:              "azure-pipelines",
	ciutil.Buildkite:                   "buildkite",
	ciutil.CircleCI:                    "circleci",
	ciutil.Codefresh:                   "codefresh",
	ciutil.Codeship:                    "codeship",
	ciutil.Drone:                       "drone",
	ciutil.GenericCI:                   "generic-ci",
	ciutil.GitHubActions:               "github-actions",
	ciutil.GitLab:                      "gitlab",
	ciutil.GoCD:                        "gocd",
	ciutil.Hudson:                      "hudson",
	ciutil.Jenkins:                     "jenkins",
	ciutil.MagnumCI:                    "magnum-ci",
	ciutil.Semaphore:                   "semaphore",
	ciutil.Spacelift:                   "spacelift",
	ciutil.TaskCluster:                 "taskcluster",
	ciutil.TeamCity:                    "teamcity",
	ciutil.Travis:                      "travis",
}

func CISystem(system ciutil.SystemName) KV {
	if system == "" {
		return enumFact("ci-system", "none")
	}
	if name, ok := ciSystems[system]; ok {
		return enumFact("ci-system", name)
	}
	return enumFact("ci-system", "other")
}

func Interactive(interactive bool) KV { return boolFact("interactive", interactive) }

func AutomationAPI(automation bool) KV { return boolFact("automation-api", automation) }

var agents = []string{
	"antigravity", "augment-cli", "claude", "codex", "cowork", "cursor", "cursor-cli", "gemini",
	"github-copilot", "goose", "opencode", "pi", "replit",
}

func UsingAI(getenv func(string) string) KV {
	agent, _, _ := strings.Cut(agentdetect.Detect(getenv), "_")
	if agent == "claude-code" {
		agent = "claude"
	}
	switch {
	case agent == "":
		return enumFact("using-ai", "no")
	case slices.Contains(agents, agent):
		return enumFact("using-ai", agent)
	default:
		return enumFact("using-ai", "other")
	}
}

func OS() KV {
	if slices.Contains(osValues, runtime.GOOS) {
		return enumFact("os", runtime.GOOS)
	}
	return enumFact("os", "other")
}

func Arch() KV {
	if slices.Contains(archValues, runtime.GOARCH) {
		return enumFact("arch", runtime.GOARCH)
	}
	return enumFact("arch", "other")
}

func backendFact(backendURL string) KV {
	u, err := url.Parse(backendURL)
	if err != nil {
		return enumFact("backend", "other")
	}
	switch {
	case u.Scheme == "http" || u.Scheme == "https":
		return enumFact("backend", "pulumi")
	case slices.Contains(diySchemes, u.Scheme):
		return enumFact("backend", u.Scheme)
	default:
		return enumFact("backend", "other")
	}
}

func ExitCode(code int) KV { return numberFact("exit-code", float64(code)) }

func Duration(d time.Duration) KV {
	return numberFact("duration-ms", float64(d.Milliseconds()))
}
