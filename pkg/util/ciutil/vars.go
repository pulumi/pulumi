// Copyright 2016-2018, Pulumi Corporation.
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

// Vars contains a set of metadata variables about a CI system.
type Vars struct {
	// Name is a required friendly name of the CI system.
	Name System
	// BuildID is an optional unique identifier for the current build/job.
	BuildID string
	// BuildType is an optional friendly type name of the build/job type.
	BuildType string
	// BuildURL is an optional URL for this build/job's webpage.
	BuildURL string
	// SHA is the SHA hash of the code repo at which this build/job is running.
	SHA string
	// BranchName is the name of the feature branch currently being built.
	BranchName string
	// CommitMessage is the full message of the Git commit being built.
	CommitMessage string
	// PRNumber is the pull-request ID/number in the source control system.
	PRNumber string
}

// DetectVars detects and returns the CI variables for the current environment.
func DetectVars() Vars {
	if os.Getenv("PULUMI_DISABLE_CI_DETECTION") != "" {
		return Vars{Name: ""}
	}

	systemName := DetectSystem()
	// If CI variables have been set specifically for Pulumi in the environment,
	// use that in preference to attempting to automatically detect the CI system.
	// This allows Pulumi to work with any CI system with appropriate configuration,
	// rather than requiring explicit support for each one.
	if os.Getenv("PULUMI_CI_SYSTEM") != "" {
		v := Vars{}
		// Override the name with the actual name in the env var.
		v.Name = System(os.Getenv("PULUMI_CI_SYSTEM"))
		v.BuildID = os.Getenv("PULUMI_CI_BUILD_ID")
		v.BuildType = os.Getenv("PULUMI_CI_BUILD_TYPE")
		v.BuildURL = os.Getenv("PULUMI_CI_BUILD_URL")
		v.SHA = os.Getenv("PULUMI_CI_PULL_REQUEST_SHA")

		// Don't proceed with automatic CI detection since we are using the PULUMI_* values.
		return v
	}

	return detectCISystemVars(systemName)
}

// detectCISystemVars detects the various aspects of a CI environment
// using pre-defined variables provided by each CI system.
// It's okay if we can't detect the additional metadata,
// we'll just have reduced functionality for our various CI integrations.
func detectCISystemVars(systemName System) Vars {
	v := Vars{Name: systemName}

	switch v.Name {
	case GitLab:
		// See https://docs.gitlab.com/ee/ci/variables/.
		v.BuildID = os.Getenv("CI_JOB_ID")
		v.BuildURL = os.Getenv("CI_JOB_URL")
		v.BuildType = os.Getenv("CI_PIPELINE_SOURCE")
		v.SHA = os.Getenv("CI_COMMIT_SHA")
		v.BranchName = os.Getenv("CI_COMMIT_REF_NAME")
		v.CommitMessage = os.Getenv("CI_COMMIT_MESSAGE")
		v.PRNumber = os.Getenv("CI_MERGE_REQUEST_ID")
	case Travis:
		// See https://docs.travis-ci.com/user/environment-variables/.
		v.BuildID = os.Getenv("TRAVIS_JOB_ID")
		v.BuildType = os.Getenv("TRAVIS_EVENT_TYPE")
		v.BuildURL = os.Getenv("TRAVIS_BUILD_WEB_URL")
		v.SHA = os.Getenv("TRAVIS_PULL_REQUEST_SHA")
		v.BranchName = os.Getenv("TRAVIS_BRANCH")
		v.CommitMessage = os.Getenv("TRAVIS_COMMIT_MESSAGE")
		v.PRNumber = os.Getenv("TRAVIS_PULL_REQUEST")
	case CircleCI:
		// See: https://circleci.com/docs/2.0/env-vars/
		v.BuildID = os.Getenv("CIRCLE_BUILD_NUM")
		v.BuildURL = os.Getenv("CIRCLE_BUILD_URL")
		v.SHA = os.Getenv("CIRCLE_SHA1")
		v.BranchName = os.Getenv("CIRCLE_BRANCH")
	case AzurePipelines:
		// See:
		// https://docs.microsoft.com/en-us/azure/devops/pipelines/build/variables?view=azure-devops&tabs=yaml#build-variables
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
			// TfsGit is a git repo hosted on Azure DevOps.
			v.PRNumber = os.Getenv("SYSTEM_PULLREQUEST_PULLREQUESTID")
		case "GitHub":
			// GitHub is a git repo hosted on GitHub.
			v.PRNumber = os.Getenv("SYSTEM_PULLREQUEST_PULLREQUESTNUMBER")
		}
	}
	return v
}
