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
	"os"
)

// travisCI represents the Travis CI system.
type travisCI struct {
	baseCI
}

// DetectVars detects the Travis env vars.
// See https://docs.travis-ci.com/user/environment-variables/.
func (t travisCI) DetectVars() Vars {
	v := Vars{Name: Travis}
	v.BuildID = os.Getenv("TRAVIS_JOB_ID")
	v.BuildNumber = os.Getenv("TRAVIS_JOB_NUMBER")
	v.BuildType = os.Getenv("TRAVIS_EVENT_TYPE")
	v.BuildURL = os.Getenv("TRAVIS_BUILD_WEB_URL")
	v.SHA = os.Getenv("TRAVIS_PULL_REQUEST_SHA")
	v.BranchName = os.Getenv("TRAVIS_BRANCH")
	v.CommitMessage = os.Getenv("TRAVIS_COMMIT_MESSAGE")
	// Travis sets the value of TRAVIS_PULL_REQUEST to false if the build
	// is not a PR build.
	// See: https://docs.travis-ci.com/user/environment-variables/#convenience-variables
	if prNumber := os.Getenv("TRAVIS_PULL_REQUEST"); prNumber != "false" {
		v.PRNumber = prNumber
	}

	return v
}
