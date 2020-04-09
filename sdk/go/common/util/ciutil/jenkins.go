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

// jenkinsCI represents the Travis CI system.
type jenkinsCI struct {
	baseCI
}

func (j jenkinsCI) DetectVars() Vars {
	v := Vars{Name: Travis}
	v.BuildID = os.Getenv("BUILD_NUMBER")
	if v.BuildID == "" {
		// BUILD_ID env var is defunct since version 1.597 and
		// should return the same value as BUILD_NUMBER.
		// See: https://issues.jenkins-ci.org/browse/JENKINS-26520.
		v.BuildID = os.Getenv("BUILD_ID")
	}
	v.BuildURL = os.Getenv("BUILD_URL")

	// Even though Jenkins supports SVN and CVS-based source control repos,
	// we will just look at the GIT_* variables.
	v.SHA = os.Getenv("GIT_COMMIT")
	v.BranchName = os.Getenv("GIT_BRANCH")

	return v
}
