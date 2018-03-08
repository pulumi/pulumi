// Copyright 2018, Pulumi Corporation.  All rights reserved.

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

// UpdateKind is an enum for the type of update performed.
type UpdateKind string

const (
	// DeployUpdate is the prototypical Pulumi program update.
	DeployUpdate UpdateKind = "update"
	// PreviewUpdate is a preview of an update, without impacting resources.
	PreviewUpdate = "preview"
	// DestroyUpdate is an update which removes all resources.
	DestroyUpdate = "destroy"
)

// UpdateResult is an enum for the result of the update.
type UpdateResult string

const (
	// InProgressResult is for updates that have not yet completed.
	InProgressResult = "in-progress"
	// SucceededResult is for updates that completed successfully.
	SucceededResult UpdateResult = "succeeded"
	// FailedResult is for updates that have failed.
	FailedResult = "failed"
)

// Keys we use for values put into UpdateInfo.Environment.
const (
	// GitHead is the commit hash of HEAD.
	GitHead = "git.head"
	// GitDirty ("true", "false") indiciates if there are any unstaged or modified files in the local repo.
	GitDirty = "git.dirty"

	// GitHubLogin is the user/organization who owns the local repo, if the origin remote is hosted on GitHub.com.
	GitHubLogin = "github.login"
	// GitHubRepo is the name of the GitHub repo, if the local git repo's remote origin is hosted on GitHub.com.
	GitHubRepo = "github.repo"
)

// UpdateInfo describes a previous update.
type UpdateInfo struct {
	// Information known before an update is started.
	Kind      UpdateKind `json:"kind"`
	StartTime int64      `json:"startTime"`

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
	Deployment      *apitype.Deployment    `json:"deployment,omitempty"`
	ResourceChanges engine.ResourceChanges `json:"resourceChanges,omitempty"`
}
