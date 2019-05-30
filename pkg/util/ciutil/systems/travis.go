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

package systems

import (
	"os"
)

// TravisCISystem represents the Travis CI system.
type TravisCISystem struct {
	BaseCISystem
}

// DetectVars detects the Travis env vars.
// See https://docs.travis-ci.com/user/environment-variables/.
func (t TravisCISystem) DetectVars() Vars {
	v := Vars{Name: Travis}
	v.BuildID = os.Getenv("TRAVIS_JOB_ID")
	v.BuildType = os.Getenv("TRAVIS_EVENT_TYPE")
	v.BuildURL = os.Getenv("TRAVIS_BUILD_WEB_URL")
	v.SHA = os.Getenv("TRAVIS_PULL_REQUEST_SHA")
	v.BranchName = os.Getenv("TRAVIS_BRANCH")
	v.CommitMessage = os.Getenv("TRAVIS_COMMIT_MESSAGE")
	v.PRNumber = os.Getenv("TRAVIS_PULL_REQUEST")

	return v
}
