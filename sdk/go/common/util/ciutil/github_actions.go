// Copyright 2016-2019, Pulumi Corporation.
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

package ciutil

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// githubActionsCI represents the GitHub Actions CI system.
type githubActionsCI struct {
	baseCI
}

// githubPR represents the `pull_request` payload posted by GitHub to trigger
// workflows for PRs. Note that this is only a partial representation as we
// don't need anything other than the PR number.
// See https://developer.github.com/webhooks/event-payloads/#pull_request.
type githubPR struct {
	Action string `json:"action"`
	Number int64  `json:"number"`
}

// githubActionsPullRequestEvent represents the webhook payload for a pull_request event.
// https://help.github.com/en/actions/reference/events-that-trigger-workflows#pull-request-event-pull_request
type githubActionsPullRequestEvent struct {
	PullRequest githubPR `json:"pull_request"`
}

// DetectVars detects the GitHub Actions env vars.
// See https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables.
func (t githubActionsCI) DetectVars() Vars {
	v := Vars{Name: GitHubActions}
	v.BuildID = os.Getenv("GITHUB_RUN_ID")
	v.BuildNumber = os.Getenv("GITHUB_RUN_NUMBER")
	v.BuildType = os.Getenv("GITHUB_EVENT_NAME")
	v.SHA = os.Getenv("GITHUB_SHA")
	v.BranchName = os.Getenv("GITHUB_REF")
	repoSlug := os.Getenv("GITHUB_REPOSITORY")
	if repoSlug != "" && v.BuildID != "" {
		v.BuildURL = fmt.Sprintf("https://github.com/%s/actions/runs/%s", repoSlug, v.BuildID)
	}

	// Try to use the pull_request webhook payload to extract the PR number.
	// For Pull Requests, GitHub stores the payload of the webhook that triggered the
	// workflow in a path. The path is identified by GITHUB_EVENT_PATH.
	if v.BuildType == "pull_request" {
		eventPath := os.Getenv("GITHUB_EVENT_PATH")
		var prEvent githubActionsPullRequestEvent
		if err := json.Unmarshal([]byte(eventPath), &prEvent); err == nil {
			v.PRNumber = strconv.FormatInt(prEvent.PullRequest.Number, 10)
		}
	}
	return v
}
