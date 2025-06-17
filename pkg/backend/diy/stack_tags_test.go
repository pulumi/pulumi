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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/memblob"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// setupTestBackend creates a test backend with in-memory storage
func setupTestBackend(_ *testing.T) (*diyBackend, context.Context) {
	bucket := memblob.OpenBucket(nil)
	b := &diyBackend{
		bucket: bucket,
		store: newProjectReferenceStore(bucket, func() *workspace.Project {
			return &workspace.Project{Name: "test-project"}
		}),
	}
	ctx := context.Background()
	return b, ctx
}

// createTestStackRef creates a test stack reference
func createTestStackRef(project, stack string) *diyBackendReference {
	return &diyBackendReference{
		project: tokens.Name(project),
		name:    tokens.MustParseStackName(stack),
		store:   nil, // Will be set when needed
	}
}

func TestStackTagsBasicOperations(t *testing.T) {
	t.Parallel()

	b, ctx := setupTestBackend(t)
	ref := createTestStackRef("myproject", "mystack")
	ref.store = b.store

	// Test loading tags from non-existent file - should return empty map
	tags, err := b.loadStackTags(ctx, ref)
	require.NoError(t, err)
	assert.Empty(t, tags)

	// Test saving tags
	testTags := map[apitype.StackTagName]string{
		"env":            "dev",
		"owner":          "team-foo",
		"pulumi:project": "myproject",
		"custom:tag":     "value123",
	}

	err = b.saveStackTags(ctx, ref, testTags)
	require.NoError(t, err)

	// Test loading saved tags
	loadedTags, err := b.loadStackTags(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, testTags, loadedTags)

	// Test updating tags (partial update)
	updatedTags := map[apitype.StackTagName]string{
		"env":            "prod",
		"owner":          "team-bar",
		"pulumi:project": "myproject",
		"new:tag":        "newvalue",
	}

	err = b.saveStackTags(ctx, ref, updatedTags)
	require.NoError(t, err)

	loadedTags, err = b.loadStackTags(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, updatedTags, loadedTags)

	// Verify old tags are gone and new ones are present
	assert.Equal(t, "prod", loadedTags["env"])
	assert.Equal(t, "newvalue", loadedTags["new:tag"])
	assert.NotContains(t, loadedTags, "custom:tag")
}

func TestStackTagsDeletion(t *testing.T) {
	t.Parallel()

	b, ctx := setupTestBackend(t)
	ref := createTestStackRef("myproject", "mystack")
	ref.store = b.store

	// Save some tags first
	testTags := map[apitype.StackTagName]string{
		"env":   "dev",
		"owner": "team-foo",
	}

	err := b.saveStackTags(ctx, ref, testTags)
	require.NoError(t, err)

	// Verify tags exist
	loadedTags, err := b.loadStackTags(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, testTags, loadedTags)

	// Delete tags
	err = b.deleteStackTags(ctx, ref)
	require.NoError(t, err)

	// Verify tags are gone - should return empty map for non-existent file
	loadedTags, err = b.loadStackTags(ctx, ref)
	require.NoError(t, err)
	assert.Empty(t, loadedTags)

	// Test deleting non-existent tags (should not error)
	err = b.deleteStackTags(ctx, ref)
	require.NoError(t, err)
}

func TestStackTagsPath(t *testing.T) {
	t.Parallel()

	b, _ := setupTestBackend(t)

	// Test project-scoped reference
	ref := createTestStackRef("myproject", "mystack")
	ref.store = b.store
	expectedPath := ".pulumi/stacks/myproject/mystack.pulumi-tags"
	assert.Equal(t, expectedPath, b.stackTagsPath(ref))

	// Test legacy reference (no project)
	legacyRef := &diyBackendReference{
		project: "",
		name:    tokens.MustParseStackName("mystack"),
		store:   newLegacyReferenceStore(b.bucket),
	}
	expectedLegacyPath := ".pulumi/stacks/mystack.pulumi-tags"
	assert.Equal(t, expectedLegacyPath, b.stackTagsPath(legacyRef))
}

func TestStackTagsEmptyAndNilValues(t *testing.T) {
	t.Parallel()

	b, ctx := setupTestBackend(t)
	ref := createTestStackRef("myproject", "mystack")
	ref.store = b.store

	// Test saving empty tags map
	emptyTags := make(map[apitype.StackTagName]string)
	err := b.saveStackTags(ctx, ref, emptyTags)
	require.NoError(t, err)

	loadedTags, err := b.loadStackTags(ctx, ref)
	require.NoError(t, err)
	assert.Empty(t, loadedTags)

	// Test saving tags with empty values
	tagsWithEmptyValues := map[apitype.StackTagName]string{
		"nonempty": "value",
		"empty":    "",
	}
	err = b.saveStackTags(ctx, ref, tagsWithEmptyValues)
	require.NoError(t, err)

	loadedTags, err = b.loadStackTags(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, tagsWithEmptyValues, loadedTags)
	assert.Equal(t, "value", loadedTags["nonempty"])
	assert.Equal(t, "", loadedTags["empty"])
}

func TestUpdateStackTagsFromMetadata(t *testing.T) {
	t.Parallel()

	b, _ := setupTestBackend(t)
	ref := createTestStackRef("myproject", "mystack")

	// Test system tags are added
	tags := make(map[apitype.StackTagName]string)
	b.updateStackTagsFromMetadata(tags, ref)

	assert.Equal(t, "myproject", tags["pulumi:project"])

	// Test system tags don't overwrite existing user tags
	tags["pulumi:project"] = "user-override"
	b.updateStackTagsFromMetadata(tags, ref)
	assert.Equal(t, "user-override", tags["pulumi:project"]) // Should remain unchanged

	// Test with empty project name
	emptyRef := createTestStackRef("", "mystack")
	emptyTags := make(map[apitype.StackTagName]string)
	b.updateStackTagsFromMetadata(emptyTags, emptyRef)
	assert.NotContains(t, emptyTags, "pulumi:project")
}

func TestBackendUpdateStackTags(t *testing.T) {
	t.Parallel()

	b, ctx := setupTestBackend(t)
	ref := createTestStackRef("myproject", "mystack")
	ref.store = b.store

	// Create a stack
	stack := newStack(ref, b)

	// Test SupportsTags returns true
	assert.True(t, b.SupportsTags())

	// Test UpdateStackTags
	testTags := map[apitype.StackTagName]string{
		"env":   "dev",
		"owner": "team-foo",
	}

	err := b.UpdateStackTags(ctx, stack, testTags)
	require.NoError(t, err)

	// Verify tags were saved and system tags were added
	savedTags, err := b.loadStackTags(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "dev", savedTags["env"])
	assert.Equal(t, "team-foo", savedTags["owner"])
	assert.Equal(t, "myproject", savedTags["pulumi:project"]) // System tag should be added

	// Test that stack caching works
	stackTags := stack.Tags()
	assert.Equal(t, savedTags, stackTags)

	// Test updating tags again (should invalidate cache)
	newTags := map[apitype.StackTagName]string{
		"env": "prod",
	}

	err = b.UpdateStackTags(ctx, stack, newTags)
	require.NoError(t, err)

	// Verify cache was updated
	stackTags = stack.Tags()
	assert.Equal(t, "prod", stackTags["env"])
	assert.Equal(t, "myproject", stackTags["pulumi:project"])
	assert.NotContains(t, stackTags, "owner") // Should be gone
}

func TestStackTagsFiltering(t *testing.T) {
	t.Parallel()

	b, ctx := setupTestBackend(t)

	// Create multiple stacks with different tags
	stacks := []struct {
		name string
		tags map[apitype.StackTagName]string
	}{
		{
			name: "stack1",
			tags: map[apitype.StackTagName]string{
				"env":   "dev",
				"team":  "backend",
				"owner": "alice",
			},
		},
		{
			name: "stack2",
			tags: map[apitype.StackTagName]string{
				"env":   "prod",
				"team":  "frontend",
				"owner": "bob",
			},
		},
		{
			name: "stack3",
			tags: map[apitype.StackTagName]string{
				"env":     "dev",
				"team":    "backend",
				"feature": "experimental",
			},
		},
		{
			name: "stack4",
			tags: map[apitype.StackTagName]string{}, // No tags
		},
	}

	// Save all stacks and their tags
	for _, s := range stacks {
		ref := createTestStackRef("testproject", s.name)
		ref.store = b.store
		err := b.saveStackTags(ctx, ref, s.tags)
		require.NoError(t, err)

		// Mock the store to return these stacks
		// This is a simplified test - in real usage, ListReferences would return these
	}

	// Create mock references for testing
	refs := make([]*diyBackendReference, len(stacks))
	for i, s := range stacks {
		refs[i] = createTestStackRef("testproject", s.name)
		refs[i].store = b.store
	}

	// Test filtering by tag name only
	tagName := "env"
	var matchedRefs []*diyBackendReference
	for _, ref := range refs {
		tags, err := b.loadStackTags(ctx, ref)
		require.NoError(t, err)
		if _, exists := tags[tagName]; exists {
			matchedRefs = append(matchedRefs, ref)
		}
	}
	assert.Len(t, matchedRefs, 3) // stack1, stack2, stack3 have "env" tag

	// Test filtering by tag name and value
	tagValue := "dev"
	matchedRefs = nil
	for _, ref := range refs {
		tags, err := b.loadStackTags(ctx, ref)
		require.NoError(t, err)
		if value, exists := tags[tagName]; exists && value == tagValue {
			matchedRefs = append(matchedRefs, ref)
		}
	}
	assert.Len(t, matchedRefs, 2) // stack1, stack3 have env=dev

	// Test filtering by tag value only (search all tags)
	tagValue = "backend"
	matchedRefs = nil
	for _, ref := range refs {
		tags, err := b.loadStackTags(ctx, ref)
		require.NoError(t, err)
		found := false
		for _, value := range tags {
			if value == tagValue {
				found = true
				break
			}
		}
		if found {
			matchedRefs = append(matchedRefs, ref)
		}
	}
	assert.Len(t, matchedRefs, 2) // stack1, stack3 have team=backend
}

func TestStackTagsJSONFormat(t *testing.T) {
	t.Parallel()

	b, ctx := setupTestBackend(t)
	ref := createTestStackRef("myproject", "mystack")
	ref.store = b.store

	// Save tags
	testTags := map[apitype.StackTagName]string{
		"env":   "dev",
		"owner": "team-foo",
	}

	err := b.saveStackTags(ctx, ref, testTags)
	require.NoError(t, err)

	// Read the raw JSON from storage and verify format
	tagsPath := b.stackTagsPath(ref)
	rawData, err := b.bucket.ReadAll(ctx, tagsPath)
	require.NoError(t, err)

	// Verify it's valid JSON with expected structure
	var tagsFile stackTagsFile
	err = json.Unmarshal(rawData, &tagsFile)
	require.NoError(t, err)

	assert.Equal(t, stackTagsVersion, tagsFile.Version)
	assert.Equal(t, testTags, tagsFile.Tags)

	// Verify the JSON is properly formatted (indented)
	assert.Contains(t, string(rawData), "    ") // Should contain indentation
	assert.Contains(t, string(rawData), "\"version\"")
	assert.Contains(t, string(rawData), "\"tags\"")
}

func TestInvalidStackReference(t *testing.T) {
	t.Parallel()

	b, ctx := setupTestBackend(t)

	// Mock the Ref() method with invalid reference type
	mockStackWithRef := &mockStackForTesting{
		ref: &struct{ backend.StackReference }{}, // Invalid type
	}

	tags := map[apitype.StackTagName]string{"test": "value"}
	err := b.UpdateStackTags(ctx, mockStackWithRef, tags)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid stack reference type")
}

// mockStackForTesting is a minimal stack implementation for testing
type mockStackForTesting struct {
	ref backend.StackReference
}

func (m *mockStackForTesting) Ref() backend.StackReference { return m.ref }
func (m *mockStackForTesting) ConfigLocation() backend.StackConfigLocation {
	return backend.StackConfigLocation{}
}

func (m *mockStackForTesting) LoadRemoteConfig(
	context.Context, *workspace.Project,
) (*workspace.ProjectStack, error) {
	return nil, nil
}

func (m *mockStackForTesting) SaveRemoteConfig(context.Context, *workspace.ProjectStack) error {
	return nil
}

func (m *mockStackForTesting) Snapshot(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
	return nil, nil
}
func (m *mockStackForTesting) Backend() backend.Backend              { return nil }
func (m *mockStackForTesting) Tags() map[apitype.StackTagName]string { return nil }
func (m *mockStackForTesting) DefaultSecretManager(*workspace.ProjectStack) (secrets.Manager, error) {
	return nil, nil
}

// TestStackTagsDoNotInterfereWithStackDiscovery tests that stack tags files
// do not interfere with normal stack discovery and listing operations.
func TestStackTagsDoNotInterfereWithStackDiscovery(t *testing.T) {
	t.Parallel()

	bucket := memblob.OpenBucket(nil)
	backend := &diyBackend{
		bucket: bucket,
		store: newProjectReferenceStore(bucket, func() *workspace.Project {
			return &workspace.Project{Name: "test-project"}
		}),
	}
	ctx := context.Background()

	// Create a stack reference
	ref := &diyBackendReference{
		project: tokens.Name("test-project"),
		name:    tokens.MustParseStackName("dev"),
		store:   backend.store,
	}

	// Create a minimal stack checkpoint file manually
	// (normally this would be done by stack operations)
	stackPath := ref.StackBasePath() + ".json"
	checkpointContent := `{
		"version": 3,
		"checkpoint": {
			"stack": "dev",
			"config": {},
			"latest": null
		}
	}`
	err := bucket.WriteAll(ctx, stackPath, []byte(checkpointContent), nil)
	require.NoError(t, err)

	// Create stack and add tags
	stack := newStack(ref, backend)
	testTags := map[apitype.StackTagName]string{
		"environment": "development",
		"owner":       "test-team",
	}

	err = backend.UpdateStackTags(ctx, stack, testTags)
	require.NoError(t, err)

	// Verify that ListReferences only returns the actual stack, not the tags file
	references, err := backend.store.ListReferences(ctx)
	require.NoError(t, err)
	require.Len(t, references, 1, "Should find exactly one stack reference")

	foundRef := references[0]
	assert.Equal(t, "test-project", string(foundRef.project))
	assert.Equal(t, "dev", foundRef.name.String())

	// Verify that getStacks also works correctly
	stacks, err := backend.getStacks(ctx)
	require.NoError(t, err)
	require.Len(t, stacks, 1, "Should find exactly one stack")

	foundStack := stacks[0]
	assert.Equal(t, "test-project", string(foundStack.project))
	assert.Equal(t, "dev", foundStack.name.String())

	// Verify that stackPath returns the correct path for the actual stack file
	actualStackPath := backend.stackPath(ctx, ref)
	assert.Contains(t, actualStackPath, "dev.json", "Stack path should point to the .json checkpoint file")
	assert.NotContains(t, actualStackPath, ".pulumi-tags", "Stack path should not point to tags file")

	// Verify that tags still work correctly
	stackTags := stack.Tags()
	assert.Equal(t, "development", stackTags["environment"])
	assert.Equal(t, "test-team", stackTags["owner"])
	assert.Equal(t, "test-project", stackTags["pulumi:project"])

	// Verify that the tags file exists but doesn't interfere
	tagsPath := backend.stackTagsPath(ref)
	assert.Contains(t, tagsPath, ".pulumi-tags", "Tags path should use .pulumi-tags extension")

	// Check that tags file exists
	exists, err := bucket.Exists(ctx, tagsPath)
	require.NoError(t, err)
	assert.True(t, exists, "Tags file should exist")

	// Check that the checkpoint file also exists
	exists, err = bucket.Exists(ctx, actualStackPath)
	require.NoError(t, err)
	assert.True(t, exists, "Checkpoint file should exist")
}

// TestLegacyStackTagsDoNotInterfereWithStackDiscovery tests the same with legacy store
func TestLegacyStackTagsDoNotInterfereWithStackDiscovery(t *testing.T) {
	t.Parallel()

	bucket := memblob.OpenBucket(nil)
	backend := &diyBackend{
		bucket: bucket,
		store:  newLegacyReferenceStore(bucket),
	}
	ctx := context.Background()

	// Create a legacy stack reference (no project)
	ref := &diyBackendReference{
		project: "",
		name:    tokens.MustParseStackName("legacy-stack"),
		store:   backend.store,
	}

	// Create a minimal stack checkpoint file manually
	stackPath := ref.StackBasePath() + ".json"
	checkpointContent := `{
		"version": 3,
		"checkpoint": {
			"stack": "legacy-stack",
			"config": {},
			"latest": null
		}
	}`
	err := bucket.WriteAll(ctx, stackPath, []byte(checkpointContent), nil)
	require.NoError(t, err)

	// Create stack and add tags
	stack := newStack(ref, backend)
	testTags := map[apitype.StackTagName]string{
		"environment": "legacy",
		"owner":       "legacy-team",
	}

	err = backend.UpdateStackTags(ctx, stack, testTags)
	require.NoError(t, err)

	// Verify that ListReferences only returns the actual stack, not the tags file
	references, err := backend.store.ListReferences(ctx)
	require.NoError(t, err)
	require.Len(t, references, 1, "Should find exactly one stack reference")

	foundRef := references[0]
	assert.Equal(t, "", string(foundRef.project)) // Legacy has no project
	assert.Equal(t, "legacy-stack", foundRef.name.String())

	// Verify that tags still work correctly
	stackTags := stack.Tags()
	assert.Equal(t, "legacy", stackTags["environment"])
	assert.Equal(t, "legacy-team", stackTags["owner"])
	// No pulumi:project tag for legacy stacks with empty project
	assert.NotContains(t, stackTags, "pulumi:project")
}
