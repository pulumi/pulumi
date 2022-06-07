// Copyright 2016-2021, Pulumi Corporation.
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
	"os"
)

// buildkiteCI represents a Buildkite CI/CD system.
type buildkiteCI struct {
	baseCI
}

// DetectVars detects the env vars for a Buildkite Build.
func (bci buildkiteCI) DetectVars() Vars {
	v := Vars{Name: Buildkite}
	// https://buildkite.com/docs/pipelines/environment-variables#bk-env-vars-buildkite-branch
	v.BranchName = os.Getenv("BUILDKITE_BRANCH")
	// https://buildkite.com/docs/pipelines/environment-variables#bk-env-vars-buildkite-build-id
	v.BuildID = os.Getenv("BUILDKITE_BUILD_ID")
	// https://buildkite.com/docs/pipelines/environment-variables#bk-env-vars-buildkite-build-number
	v.BuildNumber = os.Getenv("BUILDKITE_BUILD_NUMBER")
	// https://buildkite.com/docs/pipelines/environment-variables#bk-env-vars-buildkite-build-url
	v.BuildURL = os.Getenv("BUILDKITE_BUILD_URL")
	// https://buildkite.com/docs/pipelines/environment-variables#bk-env-vars-buildkite-message
	// This is usually the commit message but can be other messages.
	v.CommitMessage = os.Getenv("BUILDKITE_MESSAGE")
	// https://buildkite.com/docs/pipelines/environment-variables#bk-env-vars-buildkite-pull-request
	// If Buildkite's PR env var it is a pull request of the supplied number, else the build type is
	// whatever Buildkite says it is. Pull requests are webhooks just like a standard push so this allows
	// us to differentiate the two.
	prNumber := os.Getenv("BUILDKITE_PULL_REQUEST")
	if prNumber != "false" {
		v.PRNumber = prNumber
		v.BuildType = "PullRequest"
	} else {
		// https://buildkite.com/docs/pipelines/environment-variables#bk-env-vars-buildkite-source
		v.BuildType = os.Getenv("BUILDKITE_SOURCE")
	}
	// https://buildkite.com/docs/pipelines/environment-variables#bk-env-vars-buildkite-commit
	v.SHA = os.Getenv("BUILDKITE_COMMIT")

	return v
}
