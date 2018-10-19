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
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/gitutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Stack is a stack associated with a particular backend implementation.
type Stack interface {
	Ref() StackReference                                    // this stack's identity.
	Config() config.Map                                     // the current config map.
	Snapshot(ctx context.Context) (*deploy.Snapshot, error) // the latest deployment snapshot.
	Backend() Backend                                       // the backend this stack belongs to.

	// Preview changes to this stack.
	Preview(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, error)
	// Update this stack.
	Update(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, error)
	// Refresh this stack's state from the cloud provider.
	Refresh(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, error)
	// Destroy this stack's resources.
	Destroy(ctx context.Context, op UpdateOperation) (engine.ResourceChanges, error)

	// remove this stack.
	Remove(ctx context.Context, force bool) (bool, error)
	// list log entries for this stack.
	GetLogs(ctx context.Context, query operations.LogQuery) ([]operations.LogEntry, error)
	// export this stack's deployment.
	ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error)
	// import the given deployment into this stack.
	ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error
}

// RemoveStack returns the stack, or returns an error if it cannot.
func RemoveStack(ctx context.Context, s Stack, force bool) (bool, error) {
	return s.Backend().RemoveStack(ctx, s.Ref(), force)
}

// PreviewStack previews changes to this stack.
func PreviewStack(ctx context.Context, s Stack, op UpdateOperation) (engine.ResourceChanges, error) {
	return s.Backend().Preview(ctx, s.Ref(), op)
}

// UpdateStack updates the target stack with the current workspace's contents (config and code).
func UpdateStack(ctx context.Context, s Stack, op UpdateOperation) (engine.ResourceChanges, error) {
	return s.Backend().Update(ctx, s.Ref(), op)
}

// RefreshStack refresh's the stack's state from the cloud provider.
func RefreshStack(ctx context.Context, s Stack, op UpdateOperation) (engine.ResourceChanges, error) {
	return s.Backend().Refresh(ctx, s.Ref(), op)
}

// DestroyStack destroys all of this stack's resources.
func DestroyStack(ctx context.Context, s Stack, op UpdateOperation) (engine.ResourceChanges, error) {
	return s.Backend().Destroy(ctx, s.Ref(), op)
}

// GetStackCrypter fetches the encrypter/decrypter for a stack.
func GetStackCrypter(s Stack) (config.Crypter, error) {
	return s.Backend().GetStackCrypter(s.Ref())
}

// GetLatestConfiguration returns the configuration for the most recent deployment of the stack.
func GetLatestConfiguration(ctx context.Context, s Stack) (config.Map, error) {
	return s.Backend().GetLatestConfiguration(ctx, s.Ref())
}

// GetStackLogs fetches a list of log entries for the current stack in the current backend.
func GetStackLogs(ctx context.Context, s Stack, query operations.LogQuery) ([]operations.LogEntry, error) {
	return s.Backend().GetLogs(ctx, s.Ref(), query)
}

// ExportStackDeployment exports the given stack's deployment as an opaque JSON message.
func ExportStackDeployment(ctx context.Context, s Stack) (*apitype.UntypedDeployment, error) {
	return s.Backend().ExportDeployment(ctx, s.Ref())
}

// ImportStackDeployment imports the given deployment into the indicated stack.
func ImportStackDeployment(ctx context.Context, s Stack, deployment *apitype.UntypedDeployment) error {
	return s.Backend().ImportDeployment(ctx, s.Ref(), deployment)
}

// GetStackTags returns the set of tags for the "current" stack, based on the environment
// and Pulumi.yaml file.
func GetStackTags() (map[apitype.StackTagName]string, error) {
	tags := make(map[apitype.StackTagName]string)

	// Tags based on Pulumi.yaml.
	projPath, err := workspace.DetectProjectPath()
	if err != nil {
		return nil, err
	}
	if projPath != "" {
		proj, err := workspace.LoadProject(projPath)
		if err != nil {
			return nil, errors.Wrapf(err, "error loading project %q", projPath)
		}
		tags[apitype.ProjectNameTag] = proj.Name.String()
		tags[apitype.ProjectRuntimeTag] = proj.RuntimeInfo.Name()
		if proj.Description != nil {
			tags[apitype.ProjectDescriptionTag] = *proj.Description
		}

		// Add the git metadata to the tags
		if err := addGitMetadataToStackTags(tags, projPath); err != nil { // nolint
			// Intentionally ignore errors adding git metadata to stack tags.
		}
	}

	return tags, nil
}

// addGitMetadataToStackTags fetches the git repository from the directory, and attempts to detect
// and add any relevant git metadata as stack tags
func addGitMetadataToStackTags(tags map[apitype.StackTagName]string, projPath string) error {
	repo, err := gitutil.GetGitRepository(filepath.Dir(projPath))
	if repo == nil {
		return fmt.Errorf("no git repository found from %v", projPath)
	}
	if err != nil {
		return err
	}

	remoteURL, err := gitutil.GetGitRemoteURL(repo, "origin")

	if err != nil {
		return err
	}

	// check if the remote URL is a GitHub or a GitLab URL
	if gitutil.IsGitOriginURLGitHub(remoteURL) {
		if owner, repo, err := gitutil.GetGitHubProjectForOriginByURL(remoteURL); err != nil {
			tags[apitype.GitHubOwnerNameTag] = owner
			tags[apitype.GitHubRepositoryNameTag] = repo
		}
	} else if gitutil.IsGitOriginURLGitLab(remoteURL) {
		if owner, repo, err := gitutil.GetGitLabProjectForOriginByURL(remoteURL); err != nil {
			tags[apitype.GitLabOwnerNameTag] = owner
			tags[apitype.GitLabRepositoryNameTag] = repo
		}
	}

	return nil
}

// validateStackName checks if s is a valid stack name, otherwise returns a descritive error.
// This should match the stack naming rules enforced by the Pulumi Service.
func validateStackName(s string) error {
	stackNameRE := regexp.MustCompile("^[a-zA-Z0-9-_.]{1,100}$")
	if stackNameRE.MatchString(s) {
		return nil
	}
	return errors.New("a stack name may only contain alphanumeric, hyphens, underscores, or periods")
}

// ValidateStackProperties validates the stack name and its tags to confirm they adhear to various
// naming and length restrictions.
func ValidateStackProperties(stack string, tags map[apitype.StackTagName]string) error {
	const maxStackName = 100 // Derived from the regex in validateStackName.
	if len(stack) > maxStackName {
		return errors.Errorf("stack name too long (max length %d characters)", maxStackName)
	}
	if err := validateStackName(stack); err != nil {
		return errors.Wrapf(err, "invalid stack name")
	}

	// Ensure tag values won't be rejected by the Pulumi Service. We do not validate that their
	// values make sense, e.g. ProjectRuntimeTag is a supported runtime.
	const maxTagName = 40
	const maxTagValue = 256
	for t, v := range tags {
		if len(t) == 0 {
			return errors.Errorf("invalid stack tag %q", t)
		}
		if len(t) > maxTagName {
			return errors.Errorf("stack tag %q is too long (max length %d characters)", t, maxTagName)
		}
		if len(v) > maxTagValue {
			return errors.Errorf("stack tag %q value is too long (max length %d characters)", t, maxTagValue)
		}
	}

	return nil
}
