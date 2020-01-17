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

// codefreshCI represents the Codefresh CI system.
type codefreshCI struct {
	baseCI
}

// DetectVars detects the env vars for a Codefresh CI system.
// See: https://codefresh.io/docs/docs/codefresh-yaml/variables/
func (c codefreshCI) DetectVars() Vars {
	v := Vars{Name: c.Name}
	v.BuildID = os.Getenv("CF_BUILD_ID")
	v.BuildURL = os.Getenv("CF_BUILD_URL")
	v.SHA = os.Getenv("CF_REVISION")
	v.BranchName = os.Getenv("CF_BRANCH")
	v.CommitMessage = os.Getenv("CF_COMMIT_MESSAGE")
	v.PRNumber = os.Getenv("CF_PULL_REQUEST_NUMBER")

	if v.PRNumber == "" {
		v.BuildType = "PullRequest"
	} else {
		v.BuildType = "Push"
	}

	return v
}
