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

// circleCICI represents the "Circle CI" CI system.
type circleCICI struct {
	baseCI
}

// DetectVars detects the Circle CI env vars.
// See: https://circleci.com/docs/2.0/env-vars/
func (c circleCICI) DetectVars() Vars {
	v := Vars{Name: c.Name}
	v.BuildID = os.Getenv("CIRCLE_BUILD_NUM")
	v.BuildURL = os.Getenv("CIRCLE_BUILD_URL")
	v.SHA = os.Getenv("CIRCLE_SHA1")
	v.BranchName = os.Getenv("CIRCLE_BRANCH")

	return v
}
