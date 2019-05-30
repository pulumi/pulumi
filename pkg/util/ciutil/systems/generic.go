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

// GenericCICI represents a generic CI/CD system
// that belongs to the Azure DevOps product suite.
type GenericCICI struct {
	BaseCI
}

// DetectVars detects the env vars from Azure Piplines.
func (g GenericCICI) DetectVars() Vars {
	v := Vars{Name: g.Name}
	v.BuildID = os.Getenv("PULUMI_CI_BUILD_ID")
	v.BuildType = os.Getenv("PULUMI_CI_BUILD_TYPE")
	v.BuildURL = os.Getenv("PULUMI_CI_BUILD_URL")
	v.SHA = os.Getenv("PULUMI_CI_PULL_REQUEST_SHA")

	return v
}
