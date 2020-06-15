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
	"io/ioutil"
	"os"
	"strconv"
)

// githubActionsCI represents the GitHub Actions CI system.
type githubActionsCI struct {
	baseCI
}

type githubPRHead struct {
	SHA string `json:"sha"`
	Ref string `json:"ref"`
}

// githubPR represents the `pull_request` payload posted by GitHub to trigger
// workflows for PRs. Note that this is only a partial representation as we
// don't need anything other than the PR number.
// See https://developer.github.com/webhooks/event-payloads/#pull_request.
type githubPR struct {
	Head githubPRHead `json:"head"`
}

// githubActionsPullRequestEvent represents the webhook payload for a pull_request event.
// https://help.github.com/en/actions/reference/events-that-trigger-workflows#pull-request-event-pull_request
type githubActionsPullRequestEvent struct {
	Action      string   `json:"action"`
	Number      int64    `json:"number"`
	PullRequest githubPR `json:"pull_request"`
}

// DetectVars detects the GitHub Actions env vars.
// See https://help.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables.
func (t githubActionsCI) DetectVars() Vars {
	v := Vars{Name: GitHubActions}
	v.BuildID = os.Getenv("GITHUB_RUN_ID")
	v.BuildNumber = os.Getenv("GITHUB_RUN_NUMBER")
	v.BuildType = os.Getenv("GITHUB_EVENT_NAME")
	v.BranchName = os.Getenv("GITHUB_REF")
	repoSlug := os.Getenv("GITHUB_REPOSITORY")
	if repoSlug != "" && v.BuildID != "" {
		v.BuildURL = fmt.Sprintf("https://github.com/%s/actions/runs/%s", repoSlug, v.BuildID)
	}

	v.SHA = os.Getenv("GITHUB_SHA")
	if v.BuildType == "pull_request" {
		event := t.GetPREvent()
		if event != nil {
			prNumber := strconv.FormatInt(event.Number, 10)
			v.PRNumber = prNumber
			v.SHA = event.PullRequest.Head.SHA
			v.BranchName = event.PullRequest.Head.Ref
		}
	}
	return v
}

// TryGetEvent returns the GitHub webhook payload found in the GitHub Actions environment.
// GitHub stores the JSON payload of the webhook that triggered the workflow in a path.
// The path is set as the value of the env var GITHUB_EVENT_PATH. Returns nil if an error
// is encountered or the GITHUB_EVENT_PATH is not set.
func (t githubActionsCI) GetPREvent() *githubActionsPullRequestEvent {
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return nil
	}

	b, err := ioutil.ReadFile(eventPath)
	if err != nil {
		return nil
	}

	var prEvent githubActionsPullRequestEvent
	if err := json.Unmarshal(b, &prEvent); err != nil {
		return nil
	}

	return &prEvent
}
