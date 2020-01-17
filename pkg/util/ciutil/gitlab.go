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

// gitlabCI represents the GitLab CI system.
type gitlabCI struct {
	baseCI
}

// DetectVars detects the Travis env vars.
// See https://docs.gitlab.com/ee/ci/variables/.
func (gl gitlabCI) DetectVars() Vars {
	v := Vars{Name: gl.Name}
	v.BuildID = os.Getenv("CI_PIPELINE_ID")
	v.BuildNumber = os.Getenv("CI_PIPELINE_IID")
	v.BuildType = os.Getenv("CI_PIPELINE_SOURCE")
	v.BuildURL = os.Getenv("CI_JOB_URL")
	v.SHA = os.Getenv("CI_COMMIT_SHA")
	v.BranchName = os.Getenv("CI_COMMIT_REF_NAME")
	v.CommitMessage = os.Getenv("CI_COMMIT_MESSAGE")
	v.PRNumber = os.Getenv("CI_MERGE_REQUEST_IID")

	return v
}
