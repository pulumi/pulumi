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
	"strings"
)

// azurePipelinesCI represents the Azure Pipelines CI/CD system
// that belongs to the Azure DevOps product suite.
type azurePipelinesCI struct {
	baseCI
}

// DetectVars detects the env vars from Azure Piplines.
// See:
// https://docs.microsoft.com/en-us/azure/devops/pipelines/build/variables?view=azure-devops&tabs=yaml#build-variables
func (az azurePipelinesCI) DetectVars() Vars {
	v := Vars{Name: AzurePipelines}
	v.BuildID = os.Getenv("BUILD_BUILDID")
	v.BuildType = os.Getenv("BUILD_REASON")
	v.SHA = os.Getenv("BUILD_SOURCEVERSION")
	v.CommitMessage = os.Getenv("BUILD_SOURCEVERSIONMESSAGE")

	orgURI := os.Getenv("SYSTEM_TEAMFOUNDATIONCOLLECTIONURI")
	orgURI = strings.TrimSuffix(orgURI, "/")
	projectName := os.Getenv("SYSTEM_TEAMPROJECT")
	v.BuildURL = fmt.Sprintf("%v/%v/_build/results?buildId=%v", orgURI, projectName, v.BuildID)

	// Azure Pipelines can be connected to external repos.
	// If the repo provider is GitHub, then we need to use
	// `SYSTEM_PULLREQUEST_PULLREQUESTNUMBER` instead of
	// `SYSTEM_PULLREQUEST_PULLREQUESTID`. For other Git repos,
	// `SYSTEM_PULLREQUEST_PULLREQUESTID` may be the only variable
	// that is set if the build is running for a PR build.
	//
	// Note that the PR ID/number only applies to Git repos.
	vcsProvider := os.Getenv("BUILD_REPOSITORY_PROVIDER")
	switch vcsProvider {
	case "GitHub":
		// GitHub is a git repo hosted on GitHub.
		v.PRNumber = os.Getenv("SYSTEM_PULLREQUEST_PULLREQUESTNUMBER")
	default:
		v.PRNumber = os.Getenv("SYSTEM_PULLREQUEST_PULLREQUESTID")
	}

	// Build.SourceBranchName is the last part of the head.
	// If the build is running because of a PR, we should use the
	// PR source branch name, instead of Build.SourceBranchName.
	// That's because Build.SourceBranchName will always be `merge` --
	// the last part of `refs/pull/1/merge`.
	if v.PRNumber != "" {
		v.BranchName = os.Getenv("SYSTEM_PULLREQUEST_SOURCEBRANCH")
	} else {
		v.BranchName = os.Getenv("BUILD_SOURCEBRANCHNAME")
	}

	return v
}
