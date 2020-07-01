package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

// GetMergedStackTags returns the stack's existing tags merged with fresh tags from the environment
// and Pulumi.yaml file.
func GetMergedStackTags(ctx context.Context, s *Stack) (map[apitype.StackTagName]string, error) {
	// Get the stack's existing tags.
	tags, err := s.b.GetStackTags(ctx, s)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = make(map[apitype.StackTagName]string)
	}

	// Get latest environment tags for the current stack.
	envTags, err := GetEnvironmentTagsForCurrentStack()
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
func GetEnvironmentTagsForCurrentStack() (map[apitype.StackTagName]string, error) {
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
		tags[apitype.ProjectRuntimeTag] = proj.Runtime.Name()
		if proj.Description != nil {
			tags[apitype.ProjectDescriptionTag] = *proj.Description
		}

		// Add the git metadata to the tags, ignoring any errors that come from it.
		ignoredErr := addGitMetadataToStackTags(tags, projPath)
		contract.IgnoreError(ignoredErr)
	}

	return tags, nil
}

// addGitMetadataToStackTags fetches the git repository from the directory, and attempts to detect
// and add any relevant git metadata as stack tags.
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
	if remoteURL == "" {
		return nil
	}

	if vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL); err == nil {
		tags[apitype.VCSOwnerNameTag] = vcsInfo.Owner
		tags[apitype.VCSRepositoryNameTag] = vcsInfo.Repo
		tags[apitype.VCSRepositoryKindTag] = vcsInfo.Kind
	} else {
		return errors.Wrapf(err, "detecting VCS info for stack tags for remote %v", remoteURL)
	}
	// Set the old stack tags keys as GitHub so that the UI will continue to work,
	// regardless of whether the remote URL is a GitHub URL or not.
	// TODO remove these when the UI no longer needs them.
	if tags[apitype.VCSOwnerNameTag] != "" {
		tags[apitype.GitHubOwnerNameTag] = tags[apitype.VCSOwnerNameTag]
		tags[apitype.GitHubRepositoryNameTag] = tags[apitype.VCSRepositoryNameTag]
	}

	return nil
}
