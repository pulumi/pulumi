// Copyright 2016-2023, Pulumi Corporation.
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

//go:build integration
// +build integration

package diy

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/fileblob"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestStackTagsWithFileBackend tests stack tags with the file:// backend
func TestStackTagsWithFileBackend(t *testing.T) {
	t.Parallel()

	// Create temporary directory for backend storage
	tmpDir, err := ioutil.TempDir("", "pulumi-test-backend-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	// Create file-based backend
	bucket, err := fileblob.OpenBucket(tmpDir, nil)
	require.NoError(t, err)
	defer bucket.Close()

	// Create backend with project store
	backend := &diyBackend{
		bucket: bucket,
		store: newProjectReferenceStore(bucket, func() *workspace.Project {
			return &workspace.Project{Name: "integration-test"}
		}),
	}

	// Test SupportsTags
	assert.True(t, backend.SupportsTags())

	// Create a stack reference
	ref := &diyBackendReference{
		project: tokens.Name("integration-test"),
		name:    tokens.MustParseStackName("test-stack"),
		store:   backend.store,
	}

	// Create stack
	stack := newStack(ref, backend)

	// Test initial tags (should be empty)
	initialTags := stack.Tags()
	assert.Empty(t, initialTags)

	// Test setting tags
	testTags := map[apitype.StackTagName]string{
		"environment": "test",
		"owner":       "integration-tests",
		"cost-center": "engineering",
		"version":     "1.0.0",
	}

	err = backend.UpdateStackTags(ctx, stack, testTags)
	require.NoError(t, err)

	// Verify tags were set
	updatedTags := stack.Tags()
	assert.Equal(t, "test", updatedTags["environment"])
	assert.Equal(t, "integration-tests", updatedTags["owner"])
	assert.Equal(t, "engineering", updatedTags["cost-center"])
	assert.Equal(t, "1.0.0", updatedTags["version"])
	assert.Equal(t, "integration-test", updatedTags["pulumi:project"]) // System tag

	// Verify tags persist after recreating the stack
	newStack := newStack(ref, backend)
	persistedTags := newStack.Tags()
	assert.Equal(t, updatedTags, persistedTags)

	// Test updating tags (replace all)
	replaceTags := map[apitype.StackTagName]string{
		"environment": "production",
		"owner":       "ops-team",
		"release":     "v2.0",
	}

	err = backend.UpdateStackTags(ctx, stack, replaceTags)
	require.NoError(t, err)

	finalTags := stack.Tags()
	assert.Equal(t, "production", finalTags["environment"])
	assert.Equal(t, "ops-team", finalTags["owner"])
	assert.Equal(t, "v2.0", finalTags["release"])
	assert.Equal(t, "integration-test", finalTags["pulumi:project"])
	assert.NotContains(t, finalTags, "cost-center") // Should be removed
	assert.NotContains(t, finalTags, "version")     // Should be removed

	// Test that tags file was created in the correct location
	expectedTagsPath := filepath.Join(tmpDir, ".pulumi", "stacks", "integration-test", "test-stack.pulumi-tags")
	_, err = os.Stat(expectedTagsPath)
	assert.NoError(t, err, "Tags file should exist at %s", expectedTagsPath)

	// Verify file content
	content, err := ioutil.ReadFile(expectedTagsPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "environment")
	assert.Contains(t, string(content), "production")
	assert.Contains(t, string(content), "\"version\": 1")
}

// TestStackTagsFilteringIntegration tests stack filtering by tags
func TestStackTagsFilteringIntegration(t *testing.T) {
	t.Parallel()

	// Create temporary directory for backend storage
	tmpDir, err := ioutil.TempDir("", "pulumi-test-filtering-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	// Create file-based backend
	bucket, err := fileblob.OpenBucket(tmpDir, nil)
	require.NoError(t, err)
	defer bucket.Close()

	backend := &diyBackend{
		bucket: bucket,
		store: newProjectReferenceStore(bucket, func() *workspace.Project {
			return &workspace.Project{Name: "filter-test"}
		}),
	}

	// Create multiple stacks with different tags
	stacks := []struct {
		name string
		tags map[apitype.StackTagName]string
	}{
		{
			name: "dev-api",
			tags: map[apitype.StackTagName]string{
				"environment": "development",
				"component":   "api",
				"team":        "backend",
			},
		},
		{
			name: "dev-ui",
			tags: map[apitype.StackTagName]string{
				"environment": "development",
				"component":   "ui",
				"team":        "frontend",
			},
		},
		{
			name: "prod-api",
			tags: map[apitype.StackTagName]string{
				"environment": "production",
				"component":   "api",
				"team":        "backend",
			},
		},
		{
			name: "staging-db",
			tags: map[apitype.StackTagName]string{
				"environment": "staging",
				"component":   "database",
				"team":        "data",
			},
		},
		{
			name: "no-tags",
			tags: map[apitype.StackTagName]string{},
		},
	}

	// Create stacks and set their tags
	for _, s := range stacks {
		ref := &diyBackendReference{
			project: tokens.Name("filter-test"),
			name:    tokens.MustParseStackName(s.name),
			store:   backend.store,
		}
		stack := newStack(ref, backend)

		err = backend.UpdateStackTags(ctx, stack, s.tags)
		require.NoError(t, err)
	}

	// Test filtering by environment=development  
	// We'll test the filtering logic directly by simulating the tag check
	allRefs := []*diyBackendReference{
		{project: "filter-test", name: tokens.MustParseStackName("dev-api"), store: backend.store},
		{project: "filter-test", name: tokens.MustParseStackName("dev-ui"), store: backend.store},
		{project: "filter-test", name: tokens.MustParseStackName("prod-api"), store: backend.store},
		{project: "filter-test", name: tokens.MustParseStackName("staging-db"), store: backend.store},
		{project: "filter-test", name: tokens.MustParseStackName("no-tags"), store: backend.store},
	}

	// Filter by environment=development
	var devStacks []*diyBackendReference
	for _, ref := range allRefs {
		tags, err := backend.loadStackTags(ctx, ref)
		require.NoError(t, err)

		if env, exists := tags["environment"]; exists && env == "development" {
			devStacks = append(devStacks, ref)
		}
	}

	// Should match dev-api and dev-ui
	assert.Len(t, devStacks, 2)
	devNames := make([]string, len(devStacks))
	for i, ref := range devStacks {
		devNames[i] = ref.name.String()
	}
	assert.Contains(t, devNames, "dev-api")
	assert.Contains(t, devNames, "dev-ui")

	// Filter by team=backend
	var backendStacks []*diyBackendReference
	for _, ref := range allRefs {
		tags, err := backend.loadStackTags(ctx, ref)
		require.NoError(t, err)

		if team, exists := tags["team"]; exists && team == "backend" {
			backendStacks = append(backendStacks, ref)
		}
	}

	// Should match dev-api and prod-api
	assert.Len(t, backendStacks, 2)
	backendNames := make([]string, len(backendStacks))
	for i, ref := range backendStacks {
		backendNames[i] = ref.name.String()
	}
	assert.Contains(t, backendNames, "dev-api")
	assert.Contains(t, backendNames, "prod-api")
}

// TestStackTagsWithLegacyBackend tests stack tags with legacy (non-project) backend
func TestStackTagsWithLegacyBackend(t *testing.T) {
	t.Parallel()

	// Create temporary directory for backend storage
	tmpDir, err := ioutil.TempDir("", "pulumi-test-legacy-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	// Create file-based backend
	bucket, err := fileblob.OpenBucket(tmpDir, nil)
	require.NoError(t, err)
	defer bucket.Close()

	// Create backend with legacy store (no project)
	backend := &diyBackend{
		bucket: bucket,
		store:  newLegacyReferenceStore(bucket),
	}

	// Create a legacy stack reference (no project)
	ref := &diyBackendReference{
		project: "", // Empty for legacy
		name:    tokens.MustParseStackName("legacy-stack"),
		store:   backend.store,
	}

	// Create stack
	stack := newStack(ref, backend)

	// Test setting tags
	testTags := map[apitype.StackTagName]string{
		"environment": "legacy-env",
		"owner":       "legacy-owner",
	}

	err = backend.UpdateStackTags(ctx, stack, testTags)
	require.NoError(t, err)

	// Verify tags were set
	updatedTags := stack.Tags()
	assert.Equal(t, "legacy-env", updatedTags["environment"])
	assert.Equal(t, "legacy-owner", updatedTags["owner"])
	// Should not have pulumi:project tag since project is empty
	assert.NotContains(t, updatedTags, "pulumi:project")

	// Test that tags file was created in the correct location (legacy path)
	expectedTagsPath := filepath.Join(tmpDir, ".pulumi", "stacks", "legacy-stack.pulumi-tags")
	_, err = os.Stat(expectedTagsPath)
	assert.NoError(t, err, "Tags file should exist at %s", expectedTagsPath)
}

// TestStackDeletionCleansUpTags tests that deleting a stack also removes its tags
func TestStackDeletionCleansUpTags(t *testing.T) {
	t.Parallel()

	// Create temporary directory for backend storage
	tmpDir, err := ioutil.TempDir("", "pulumi-test-deletion-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	// Create file-based backend
	bucket, err := fileblob.OpenBucket(tmpDir, nil)
	require.NoError(t, err)
	defer bucket.Close()

	backend := &diyBackend{
		bucket: bucket,
		store: newProjectReferenceStore(bucket, func() *workspace.Project {
			return &workspace.Project{Name: "deletion-test"}
		}),
	}

	// Create a stack reference
	ref := &diyBackendReference{
		project: tokens.Name("deletion-test"),
		name:    tokens.MustParseStackName("temp-stack"),
		store:   backend.store,
	}

	// Create stack and set tags
	stack := newStack(ref, backend)
	testTags := map[apitype.StackTagName]string{
		"temporary": "true",
		"owner":     "test",
	}

	err = backend.UpdateStackTags(ctx, stack, testTags)
	require.NoError(t, err)

	// Verify tags file exists
	expectedTagsPath := filepath.Join(tmpDir, ".pulumi", "stacks", "deletion-test", "temp-stack.pulumi-tags")
	_, err = os.Stat(expectedTagsPath)
	require.NoError(t, err, "Tags file should exist before deletion")

	// Delete the stack (simulate removal)
	err = backend.removeStack(ctx, ref)
	require.NoError(t, err)

	// Verify tags file was deleted
	_, err = os.Stat(expectedTagsPath)
	assert.True(t, os.IsNotExist(err), "Tags file should be deleted after stack removal")
}