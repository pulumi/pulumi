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

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Stack is used to manage stacks of resources against a pluggable backend.
type Stack interface {
	// Ref returns this stack's identity.
	Ref() StackReference
	// HasRemoteConfig indicates if the backend has configuration stored independent of the local file stack config.
	HasRemoteConfig() bool
	// LoadRemote the stack's configuration remotely from the backend.
	LoadRemote(ctx context.Context, project *workspace.Project) (*workspace.ProjectStack, error)
	// SaveRemote the stack's configuration remotely to the backend.
	SaveRemote(ctx context.Context, projectStack *workspace.ProjectStack) error
	// Snapshot returns the latest deployment snapshot.
	Snapshot(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error)
	// Backend returns the backend this stack belongs to.
	Backend() Backend
	// Tags return the stack's existing tags.
	Tags() map[apitype.StackTagName]string
	// Preview changes to this stack if an Update was run.
	Preview(
		ctx context.Context, op UpdateOperation, events chan<- engine.Event,
	) (*deploy.Plan, display.ResourceChanges, error)
	// Update this stack.
	Update(ctx context.Context, op UpdateOperation) (display.ResourceChanges, error)
	// Import resources into this stack.
	Import(ctx context.Context, op UpdateOperation, imports []deploy.Import) (display.ResourceChanges, error)
	// Refresh this stack's state from the cloud provider.
	Refresh(ctx context.Context, op UpdateOperation) (display.ResourceChanges, error)
	Destroy(ctx context.Context, op UpdateOperation) (display.ResourceChanges, error)
	// Watch this stack.
	Watch(ctx context.Context, op UpdateOperation, paths []string) error

	// Remove this stack.
	Remove(ctx context.Context, force bool) (bool, error)
	// Rename this stack.
	Rename(ctx context.Context, newName tokens.QName) (StackReference, error)
	// GetLogs lists log entries for this stack.
	GetLogs(ctx context.Context, secretsProvider secrets.Provider,
		cfg StackConfiguration, query operations.LogQuery) ([]operations.LogEntry, error)
	// ExportDeployment exports this stack's deployment.
	ExportDeployment(ctx context.Context) (*apitype.UntypedDeployment, error)
	// ImportDeployment imports the given deployment into this stack.
	ImportDeployment(ctx context.Context, deployment *apitype.UntypedDeployment) error

	// DefaultSecretManager returns the default secrets manager to use for this stack. This may be more specific than
	// Backend.DefaultSecretManager.
	DefaultSecretManager(info *workspace.ProjectStack) (secrets.Manager, error)
}

// RemoveStack returns the stack, or returns an error if it cannot.
func RemoveStack(ctx context.Context, s Stack, force bool) (bool, error) {
	return s.Backend().RemoveStack(ctx, s, force)
}

// RenameStack renames the stack, or returns an error if it cannot.
func RenameStack(ctx context.Context, s Stack, newName tokens.QName) (StackReference, error) {
	return s.Backend().RenameStack(ctx, s, newName)
}

// PreviewStack previews changes to this stack.
func PreviewStack(
	ctx context.Context,
	s Stack,
	op UpdateOperation,
	events chan<- engine.Event,
) (*deploy.Plan, display.ResourceChanges, error) {
	return s.Backend().Preview(ctx, s, op, events)
}

// UpdateStack updates the target stack with the current workspace's contents (config and code).
func UpdateStack(ctx context.Context, s Stack, op UpdateOperation) (display.ResourceChanges, error) {
	return s.Backend().Update(ctx, s, op)
}

// ImportStack updates the target stack with the current workspace's contents (config and code).
func ImportStack(ctx context.Context, s Stack, op UpdateOperation,
	imports []deploy.Import,
) (display.ResourceChanges, error) {
	return s.Backend().Import(ctx, s, op, imports)
}

// RefreshStack refresh's the stack's state from the cloud provider.
func RefreshStack(ctx context.Context, s Stack, op UpdateOperation) (display.ResourceChanges, error) {
	return s.Backend().Refresh(ctx, s, op)
}

// DestroyStack destroys all of this stack's resources.
func DestroyStack(ctx context.Context, s Stack, op UpdateOperation) (display.ResourceChanges, error) {
	return s.Backend().Destroy(ctx, s, op)
}

// WatchStack watches the projects working directory for changes and automatically updates the
// active stack.
func WatchStack(ctx context.Context, s Stack, op UpdateOperation, paths []string) error {
	return s.Backend().Watch(ctx, s, op, paths)
}

// GetLatestConfiguration returns the configuration for the most recent deployment of the stack.
func GetLatestConfiguration(ctx context.Context, s Stack) (config.Map, error) {
	return s.Backend().GetLatestConfiguration(ctx, s)
}

// GetStackLogs fetches a list of log entries for the current stack in the current backend.
func GetStackLogs(ctx context.Context, secretsProvider secrets.Provider, s Stack, cfg StackConfiguration,
	query operations.LogQuery,
) ([]operations.LogEntry, error) {
	return s.Backend().GetLogs(ctx, secretsProvider, s, cfg, query)
}

// ExportStackDeployment exports the given stack's deployment as an opaque JSON message.
func ExportStackDeployment(
	ctx context.Context,
	s Stack,
) (*apitype.UntypedDeployment, error) {
	return s.Backend().ExportDeployment(ctx, s)
}

// ImportStackDeployment imports the given deployment into the indicated stack.
func ImportStackDeployment(ctx context.Context, s Stack, deployment *apitype.UntypedDeployment) error {
	return s.Backend().ImportDeployment(ctx, s, deployment)
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func UpdateStackTags(ctx context.Context, s Stack, tags map[apitype.StackTagName]string) error {
	return s.Backend().UpdateStackTags(ctx, s, tags)
}

// GetMergedStackTags returns the stack's existing tags merged with fresh tags from the environment
// and Pulumi.yaml file.
func GetMergedStackTags(ctx context.Context, s Stack,
	root string, project *workspace.Project, cfg config.Map,
) (map[apitype.StackTagName]string, error) {
	// Get the stack's existing tags.
	tags := s.Tags()
	if tags == nil {
		tags = make(map[apitype.StackTagName]string)
	}

	// Get latest environment tags for the current stack.
	envTags, err := GetEnvironmentTagsForCurrentStack(root, project, cfg)
	if err != nil {
		return nil, err
	}

	// Add each new environment tag to the existing tags, overwriting existing tags with the
	// latest values.
	for k, v := range envTags {
		tags[k] = v
	}

	return tags, nil
}

// GetEnvironmentTagsForCurrentStack returns the set of tags for the "current" stack, based on the environment
// and Pulumi.yaml file.
func GetEnvironmentTagsForCurrentStack(root string,
	project *workspace.Project, cfg config.Map,
) (map[apitype.StackTagName]string, error) {
	tags := make(map[apitype.StackTagName]string)

	// Tags based on Pulumi.yaml.
	if project != nil {
		tags[apitype.ProjectNameTag] = project.Name.String()
		tags[apitype.ProjectRuntimeTag] = project.Runtime.Name()
		if project.Description != nil {
			tags[apitype.ProjectDescriptionTag] = *project.Description
		}
	}

	// Grab any `pulumi:tag` config values and use those to update the stack's tags.
	configTags, has, err := cfg.Get(config.MustParseKey(apitype.PulumiTagsConfigKey), false)
	contract.AssertNoErrorf(err, "Config.Get(\"%s\") failed unexpectedly", apitype.PulumiTagsConfigKey)
	if has {
		configTagInterface, err := configTags.ToObject()
		if err != nil {
			return nil, fmt.Errorf("%s must be an object of strings", apitype.PulumiTagsConfigKey)
		}
		configTagObject, ok := configTagInterface.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s must be an object of strings", apitype.PulumiTagsConfigKey)
		}

		for name, value := range configTagObject {
			stringValue, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%s] must be a string", apitype.PulumiTagsConfigKey, name)
			}

			tags[name] = stringValue
		}
	}

	// Add the git metadata to the tags, ignoring any errors that come from it.
	if root != "" {
		ignoredErr := addGitMetadataToStackTags(tags, root)
		contract.IgnoreError(ignoredErr)
	}

	return tags, nil
}

// addGitMetadataToStackTags fetches the git repository from the directory, and attempts to detect
// and add any relevant git metadata as stack tags.
func addGitMetadataToStackTags(tags map[apitype.StackTagName]string, projPath string) error {
	repo, err := gitutil.GetGitRepository(projPath)
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
	if remoteURL == "" {
		return nil
	}

	if vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL); err == nil {
		tags[apitype.VCSOwnerNameTag] = vcsInfo.Owner
		tags[apitype.VCSRepositoryNameTag] = vcsInfo.Repo
		tags[apitype.VCSRepositoryKindTag] = vcsInfo.Kind
	} else {
		return fmt.Errorf("detecting VCS info for stack tags for remote %v: %w", remoteURL, err)
	}

	return nil
}
