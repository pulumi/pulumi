// Copyright 2025, Pulumi Corporation.
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

package diy

import (
	"context"
	"encoding/json"
	"fmt"

	"gocloud.dev/gcerrors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// stackTagsFile represents the structure of the stack tags file.
type stackTagsFile struct {
	Version int                             `json:"version"`
	Tags    map[apitype.StackTagName]string `json:"tags"`
}

const stackTagsVersion = 1

// stackTagsPath returns the path to the stack tags file for a given stack reference.
func (b *diyBackend) stackTagsPath(ref *diyBackendReference) string {
	// Use the same pattern as stack files but with .pulumi-tags extension
	// This avoids confusion with .json stack files during stack discovery
	return ref.StackBasePath() + ".pulumi-tags"
}

// loadStackTags loads tags for a stack from the storage backend.
// Returns an empty map if no tags file exists (backward compatibility).
func (b *diyBackend) loadStackTags(
	ctx context.Context, ref *diyBackendReference,
) (map[apitype.StackTagName]string, error) {
	contract.Requiref(ref != nil, "ref", "ref cannot be nil")

	tagsPath := b.stackTagsPath(ref)
	logging.V(9).Infof("Loading stack tags from %s", tagsPath)

	// Try to read the tags file
	tagsBytes, err := b.bucket.ReadAll(ctx, tagsPath)
	if err != nil {
		// If the file doesn't exist, return empty tags (backward compatibility)
		if gcerrors.Code(err) == gcerrors.NotFound {
			logging.V(9).Infof("No tags file found for stack %s, returning empty tags", ref.String())
			return make(map[apitype.StackTagName]string), nil
		}
		return nil, fmt.Errorf("failed to read stack tags file: %w", err)
	}

	// Parse the tags file
	var tagsFile stackTagsFile
	if err := json.Unmarshal(tagsBytes, &tagsFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stack tags: %w", err)
	}

	// Validate version
	if tagsFile.Version != stackTagsVersion {
		return nil, fmt.Errorf("unsupported stack tags version: %d (expected %d)", tagsFile.Version, stackTagsVersion)
	}

	if tagsFile.Tags == nil {
		tagsFile.Tags = make(map[apitype.StackTagName]string)
	}

	logging.V(9).Infof("Loaded %d tags for stack %s", len(tagsFile.Tags), ref.String())
	return tagsFile.Tags, nil
}

// saveStackTags saves tags for a stack to the storage backend.
func (b *diyBackend) saveStackTags(
	ctx context.Context, ref *diyBackendReference, tags map[apitype.StackTagName]string,
) error {
	contract.Requiref(ref != nil, "ref", "ref cannot be nil")
	contract.Requiref(tags != nil, "tags", "tags cannot be nil")

	tagsPath := b.stackTagsPath(ref)
	logging.V(9).Infof("Saving %d stack tags to %s", len(tags), tagsPath)

	// Create the tags file structure
	tagsFile := stackTagsFile{
		Version: stackTagsVersion,
		Tags:    tags,
	}

	// Marshal to JSON
	tagsBytes, err := json.MarshalIndent(tagsFile, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal stack tags: %w", err)
	}

	// Write to storage
	if err := b.bucket.WriteAll(ctx, tagsPath, tagsBytes, nil); err != nil {
		return fmt.Errorf("failed to write stack tags: %w", err)
	}

	logging.V(9).Infof("Successfully saved stack tags for stack %s", ref.String())
	return nil
}

// deleteStackTags removes the tags file for a stack.
// This is called when a stack is deleted to clean up associated metadata.
func (b *diyBackend) deleteStackTags(ctx context.Context, ref *diyBackendReference) error {
	contract.Requiref(ref != nil, "ref", "ref cannot be nil")

	tagsPath := b.stackTagsPath(ref)
	logging.V(9).Infof("Deleting stack tags file %s", tagsPath)

	err := b.bucket.Delete(ctx, tagsPath)
	if err != nil && gcerrors.Code(err) != gcerrors.NotFound {
		return fmt.Errorf("failed to delete stack tags file: %w", err)
	}

	logging.V(9).Infof("Successfully deleted stack tags for stack %s", ref.String())
	return nil
}

// updateStackTagsFromMetadata ensures that system tags (like pulumi:project)
// are set in the tags based on available metadata.
// This function only adds system tags if they don't already exist (preserves user overrides).
// This is a placeholder for future enhancements to extract metadata from checkpoints.
func (b *diyBackend) updateStackTagsFromMetadata(tags map[apitype.StackTagName]string, stackRef *diyBackendReference) {
	// For now, we can set basic metadata based on the stack reference
	// Only set if not already present (preserve user overrides)
	if stackRef.project != "" {
		if _, exists := tags["pulumi:project"]; !exists {
			tags["pulumi:project"] = string(stackRef.project)
		}
	}

	// Additional system tags could be added here in the future
	// by reading from the checkpoint or project files
}
