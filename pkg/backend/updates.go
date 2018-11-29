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

package backend

import (
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
)

// UpdateMetadata describes optional metadata about an update.
type UpdateMetadata struct {
	// Message is an optional message associated with the update.
	Message string `json:"message"`
	// Environment contains optional data from the deploying environment. e.g. the current
	// source code control commit information.
	Environment map[string]string `json:"environment"`
}

// UpdateResult is an enum for the result of the update.
type UpdateResult string

const (
	// InProgressResult is for updates that have not yet completed.
	InProgressResult UpdateResult = "in-progress"
	// SucceededResult is for updates that completed successfully.
	SucceededResult UpdateResult = "succeeded"
	// FailedResult is for updates that have failed.
	FailedResult UpdateResult = "failed"
)

// Keys we use for values put into UpdateInfo.Environment.
const (
	// GitHead is the commit hash of HEAD.
	GitHead = "git.head"
	// GitHeadName is the name of the HEAD ref. e.g. "refs/heads/master" or "refs/tags/v1.0.0".
	GitHeadName = "git.headName"
	// GitDirty ("true", "false") indiciates if there are any unstaged or modified files in the local repo.
	GitDirty = "git.dirty"

	// GitCommitter is the name of the person who committed the commit at HEAD.
	GitCommitter = "git.committer"
	// GitCommitterEmail is the Email address associated with the committer.
	GitCommitterEmail = "git.committer.email"
	// GitAuthor is the name of the person who authored the commit at HEAD.
	GitAuthor = "git.author"
	// GitAuthorEmail is the email address associated with the commit's author.
	GitAuthorEmail = "git.author.email"

	// GitHubLogin is the user/organization who owns the local repo, if the origin remote is hosted on GitHub.com.
	// TODO [pulumi/pulumi-service#2306] Once the UI is updated, we would no longer need the GitHub specific keys.
	GitHubLogin = "github.login"
	// GitHubRepo is the name of the GitHub repo, if the local git repo's remote origin is hosted on GitHub.com.
	GitHubRepo = "github.repo"

	// VCSRepoOwner is the user who owns the local repo, if the origin remote is a cloud host.
	VCSRepoOwner = "vcs.owner"
	// VCSRepoName is the name of the repo, if the local git repo's remote origin is a cloud host.
	VCSRepoName = "vcs.repo"
	//VCSRepoKind is the cloud host where the repo is hosted.
	VCSRepoKind = "vcs.kind"

	// CISystem is the name of the CI system running the pulumi operation.
	CISystem = "ci.system"
	// CIBuildID is an opaque ID of the build in the CI system.
	CIBuildID = "ci.build.id"
	// CIBuildType is the type of build of the CI system, e.g. "push", "pull_request", "test_only".
	CIBuildType = "ci.build.type"
	// CIBuildURL is a URL to get more information about the particular CI build.
	CIBuildURL = "ci.build.url"

	// CIPRHeadSHA is the SHA of the HEAD commit of a pull request running on CI. This is needed since the CI
	// server will run at a different, merge commit. (headSHA merged into the target branch.)
	CIPRHeadSHA = "ci.pr.headSHA"
)

// UpdateInfo describes a previous update.
type UpdateInfo struct {
	// Information known before an update is started.
	Kind      apitype.UpdateKind `json:"kind"`
	StartTime int64              `json:"startTime"`

	// Message is an optional message associated with the update.
	Message string `json:"message"`

	// Environment contains optional data from the deploying environment. e.g. the current
	// source code control commit information.
	Environment map[string]string `json:"environment"`

	// Config used for the update.
	Config config.Map `json:"config"`

	// Information obtained from an update completing.
	Result          UpdateResult           `json:"result"`
	EndTime         int64                  `json:"endTime"`
	ResourceChanges engine.ResourceChanges `json:"resourceChanges,omitempty"`
}
