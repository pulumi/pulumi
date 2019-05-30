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
	v.BranchName = os.Getenv("BUILD_SOURCEBRANCHNAME")
	v.CommitMessage = os.Getenv("BUILD_SOURCEVERSIONMESSAGE")
	// Azure Pipelines can be connected to external repos.
	// So we check if the provider is GitHub, then we use
	// `SYSTEM_PULLREQUEST_PULLREQUESTNUMBER` instead of `SYSTEM_PULLREQUEST_PULLREQUESTID`.
	// The PR ID/number only applies to Git repos.
	vcsProvider := os.Getenv("BUILD_REPOSITORY_PROVIDER")
	switch vcsProvider {
	case "TfsGit":
		orgURI := os.Getenv("SYSTEM_PULLREQUEST_TEAMFOUNDATIONCOLLECTIONURI")
		projectName := os.Getenv("SYSTEM_TEAMPROJECT")
		v.BuildURL = fmt.Sprintf("%v/%v/_build/results?buildId=%v", orgURI, projectName, v.BuildID)
		// TfsGit is a git repo hosted on Azure DevOps.
		v.PRNumber = os.Getenv("SYSTEM_PULLREQUEST_PULLREQUESTID")
	case "GitHub":
		// GitHub is a git repo hosted on GitHub.
		v.PRNumber = os.Getenv("SYSTEM_PULLREQUEST_PULLREQUESTNUMBER")
	}

	return v
}
