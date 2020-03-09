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
	"fmt"
	"os"
)

// bitbucketPipelinesCI represents the Bitbucket CI system.
type bitbucketPipelinesCI struct {
	baseCI
}

// DetectVars detects the Bitbucket env vars.
// See https://confluence.atlassian.com/bitbucket/environment-variables-794502608.html.
func (bb bitbucketPipelinesCI) DetectVars() Vars {
	v := Vars{Name: bb.Name}

	buildID := os.Getenv("BITBUCKET_BUILD_NUMBER")
	v.BuildID = buildID

	repoURL := os.Getenv("BITBUCKET_GIT_HTTP_ORIGIN")
	if repoURL != "" {
		buildURL := fmt.Sprintf("%v/addon/pipelines/home#!/results/%v", repoURL, buildID)
		v.BuildURL = buildURL
	}
	v.SHA = os.Getenv("BITBUCKET_COMMIT")
	v.BranchName = os.Getenv("BITBUCKET_BRANCH")
	v.PRNumber = os.Getenv("BITBUCKET_PR_ID")

	return v
}
