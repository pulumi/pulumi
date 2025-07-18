// Copyright 2019-2024, Pulumi Corporation.
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
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	declared "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

// This file contains copies of old backend tests
// that were upgraded to run with project support.
// This duplicates those tests to run with legacy, non-project state,
// validating that the legacy behavior is preserved.

func TestListStacksWithMultiplePassphrases_legacy(t *testing.T) {
	// Login to a temp dir diy backend
	tmpDir := markLegacyStore(t, t.TempDir())
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Create stack "a" and import a checkpoint with a secret
	aStackRef, err := b.ParseStackReference("a")
	require.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, aStack)
	defer func() {
		t.Setenv("PULUMI_CONFIG_PASSPHRASE", "abc123")
		_, err := b.RemoveStack(ctx, aStack, true)
		require.NoError(t, err)
	}()
	deployment, err := makeUntypedDeployment("a", "abc123",
		"v1:4iF78gb0nF0=:v1:Co6IbTWYs/UdrjgY:FSrAWOFZnj9ealCUDdJL7LrUKXX9BA==")
	require.NoError(t, err)
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "abc123")
	err = b.ImportDeployment(ctx, aStack, deployment)
	require.NoError(t, err)

	// Create stack "b" and import a checkpoint with a secret
	bStackRef, err := b.ParseStackReference("b")
	require.NoError(t, err)
	bStack, err := b.CreateStack(ctx, bStackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, bStack)
	defer func() {
		t.Setenv("PULUMI_CONFIG_PASSPHRASE", "123abc")
		_, err := b.RemoveStack(ctx, bStack, true)
		require.NoError(t, err)
	}()
	deployment, err = makeUntypedDeployment("b", "123abc",
		"v1:C7H2a7/Ietk=:v1:yfAd1zOi6iY9DRIB:dumdsr+H89VpHIQWdB01XEFqYaYjAg==")
	require.NoError(t, err)
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "123abc")
	err = b.ImportDeployment(ctx, bStack, deployment)
	require.NoError(t, err)

	// Remove the config passphrase so that we can no longer deserialize the checkpoints
	err = os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
	require.NoError(t, err)

	// Ensure that we can list the stacks we created even without a passphrase
	stacks, outContToken, err := b.ListStacks(ctx, backend.ListStacksFilter{}, nil /* inContToken */)
	require.NoError(t, err)
	assert.Nil(t, outContToken)
	assert.Len(t, stacks, 2)
	for _, stack := range stacks {
		require.NotNil(t, stack.ResourceCount())
		assert.Equal(t, 1, *stack.ResourceCount())
	}
}

func TestDrillError_legacy(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := markLegacyStore(t, t.TempDir())
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Get a non-existent stack and expect a nil error because it won't be found.
	stackRef, err := b.ParseStackReference("dev")
	if err != nil {
		t.Fatalf("unexpected error %v when parsing stack reference", err)
	}
	_, err = b.GetStack(ctx, stackRef)
	require.NoError(t, err)
}

func TestCancel_legacy(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := markLegacyStore(t, t.TempDir())
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Check that trying to cancel a stack that isn't created yet doesn't error
	aStackRef, err := b.ParseStackReference("a")
	require.NoError(t, err)
	err = b.CancelCurrentUpdate(ctx, aStackRef)
	require.NoError(t, err)

	// Check that trying to cancel a stack that isn't locked doesn't error
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, aStack)
	err = b.CancelCurrentUpdate(ctx, aStackRef)
	require.NoError(t, err)

	// Locking and lock checks are only part of the internal interface
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)

	// Lock the stack and check CancelCurrentUpdate deletes the lock file
	err = lb.Lock(ctx, aStackRef)
	require.NoError(t, err)
	// check the lock file exists
	lockExists, err := lb.bucket.Exists(ctx, lb.lockPath(aStackRef))
	require.NoError(t, err)
	assert.True(t, lockExists)
	// Call CancelCurrentUpdate
	err = lb.CancelCurrentUpdate(ctx, aStackRef)
	require.NoError(t, err)
	// Now check the lock file no longer exists
	lockExists, err = lb.bucket.Exists(ctx, lb.lockPath(aStackRef))
	require.NoError(t, err)
	assert.False(t, lockExists)

	// Make another diy backend which will have a different lockId
	ob, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)
	otherBackend, ok := ob.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)

	// Lock the stack with this new backend, then check that checkForLocks on the first backend now errors
	err = otherBackend.Lock(ctx, aStackRef)
	require.NoError(t, err)
	err = lb.checkForLock(ctx, aStackRef)
	assert.Error(t, err)
	// Now call CancelCurrentUpdate and check that checkForLocks no longer errors
	err = lb.CancelCurrentUpdate(ctx, aStackRef)
	require.NoError(t, err)
	err = lb.checkForLock(ctx, aStackRef)
	require.NoError(t, err)
}

func TestRemoveMakesBackups_legacy(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := markLegacyStore(t, t.TempDir())
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Grab the bucket interface to test with
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)

	// Check that creating a new stack doesn't make a backup file
	aStackRef, err := lb.parseStackReference("a")
	require.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, aStack)

	// Check the stack file now exists, but the backup file doesn't
	stackFileExists, err := lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef))
	require.NoError(t, err)
	assert.True(t, stackFileExists)
	backupFileExists, err := lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef)+".bak")
	require.NoError(t, err)
	assert.False(t, backupFileExists)

	// Now remove the stack
	removed, err := b.RemoveStack(ctx, aStack, false)
	require.NoError(t, err)
	assert.False(t, removed)

	// Check the stack file is now gone, but the backup file exists
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef))
	require.NoError(t, err)
	assert.False(t, stackFileExists)
	backupFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef)+".bak")
	require.NoError(t, err)
	assert.True(t, backupFileExists)
}

func TestRenameWorks_legacy(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := markLegacyStore(t, t.TempDir())
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Grab the bucket interface to test with
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)

	// Create a new stack
	aStackRef, err := lb.parseStackReference("a")
	require.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, aStack)

	// Check the stack file now exists
	stackFileExists, err := lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef))
	require.NoError(t, err)
	assert.True(t, stackFileExists)

	// Fake up some history
	err = lb.addToHistory(ctx, aStackRef, backend.UpdateInfo{Kind: apitype.DestroyUpdate})
	require.NoError(t, err)
	// And pollute the history folder
	err = lb.bucket.WriteAll(ctx, path.Join(aStackRef.HistoryDir(), "randomfile.txt"), []byte{0, 13}, nil)
	require.NoError(t, err)

	// Rename the stack
	bStackRefI, err := b.RenameStack(ctx, aStack, "b")
	require.NoError(t, err)
	assert.Equal(t, "b", bStackRefI.String())
	bStackRef := bStackRefI.(*diyBackendReference)

	// Check the new stack file now exists and the old one is gone
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, bStackRef))
	require.NoError(t, err)
	assert.True(t, stackFileExists)
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef))
	require.NoError(t, err)
	assert.False(t, stackFileExists)

	// Rename again
	bStack, err := b.GetStack(ctx, bStackRef)
	require.NoError(t, err)
	cStackRefI, err := b.RenameStack(ctx, bStack, "c")
	require.NoError(t, err)
	assert.Equal(t, "c", cStackRefI.String())
	cStackRef := cStackRefI.(*diyBackendReference)

	// Check the new stack file now exists and the old one is gone
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, cStackRef))
	require.NoError(t, err)
	assert.True(t, stackFileExists)
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, bStackRef))
	require.NoError(t, err)
	assert.False(t, stackFileExists)

	// Check we can still get the history
	history, err := b.GetHistory(ctx, cStackRef, 10, 0)
	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, apitype.DestroyUpdate, history[0].Kind)
}

// Regression test for https://github.com/pulumi/pulumi/issues/10439
func TestHtmlEscaping_legacy(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()
	resources := []*resource.State{
		{
			URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", "name"),
			Type: "a:b:c",
			Inputs: resource.PropertyMap{
				resource.PropertyKey("html"): resource.NewStringProperty("<html@tags>"),
			},
		},
	}

	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, resources, nil, deploy.SnapshotMetadata{})
	ctx := context.Background()

	sdep, err := stack.SerializeDeployment(ctx, snap, false /* showSecrets */)
	require.NoError(t, err)

	data, err := encoding.JSON.Marshal(sdep)
	require.NoError(t, err)

	// Ensure data has the string contents "<html@tags>"", not "\u003chtml\u0026tags\u003e"
	// ImportDeployment below should not modify the data
	assert.Contains(t, string(data), "<html@tags>")

	udep := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(data),
	}

	// Login to a temp dir diy backend
	tmpDir := markLegacyStore(t, t.TempDir())
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Create stack "a" and import a checkpoint with a secret
	aStackRef, err := b.ParseStackReference("a")
	require.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, aStack)
	err = b.ImportDeployment(ctx, aStack, udep)
	require.NoError(t, err)

	// Ensure the file has the string contents "<html@tags>"", not "\u003chtml\u0026tags\u003e"

	// Grab the bucket interface to read the file with
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)

	chkpath := lb.stackPath(ctx, aStackRef.(*diyBackendReference))
	bytes, err := lb.bucket.ReadAll(context.Background(), chkpath)
	require.NoError(t, err)
	state := string(bytes)
	assert.Contains(t, state, "<html@tags>")
}

func TestDIYBackendRejectsStackInitOptions_legacy(t *testing.T) {
	t.Parallel()

	// Here, we provide options that illegally specify a team on a
	// backend that does not support teams. We expect this to create
	// an error later when we call CreateStack.
	illegalOptions := &backend.CreateStackOptions{Teams: []string{"red-team"}}

	// • Create a mock diy backend
	tmpDir := markLegacyStore(t, t.TempDir())
	dirURI := "file://" + filepath.ToSlash(tmpDir)
	diy, err := New(context.Background(), diagtest.LogSink(t), dirURI, nil)
	require.NoError(t, err)
	ctx := context.Background()

	// • Simulate `pulumi stack init`, passing non-nil init options
	fakeStackRef, err := diy.ParseStackReference("foobar")
	require.NoError(t, err)
	_, err = diy.CreateStack(ctx, fakeStackRef, "", nil, illegalOptions)
	assert.ErrorIs(t, err, backend.ErrTeamsNotSupported)
}

// markLegacyStore marks the given directory as a legacy store.
// This is done by dropping a single file into the bookkeeping directory.
// ensurePulumiMeta will treat this as a legacy store if the directory exists.
//
// Returns the directory that was marked.
func markLegacyStore(t *testing.T, dir string) string {
	metaPath := filepath.Join(dir, pulumiMetaPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(metaPath), 0o755))
	require.NoError(t, os.WriteFile(metaPath, []byte(`version: 0`), 0o600))
	return dir
}

func TestParallelStackFetch_legacy(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend with legacy format
	tmpDir := markLegacyStore(t, t.TempDir())
	ctx := context.Background()

	// Create a custom environment with DIYBackendParallel set
	s := make(declared.MapStore)
	s[env.DIYBackendParallel.Var().Name()] = "5" // Set parallel to 5

	b, err := newDIYBackend(
		ctx,
		diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir),
		nil, // No project for legacy backend
		&diyBackendOptions{Env: declared.NewEnv(s)},
	)
	require.NoError(t, err)

	// Create multiple stacks to test parallel fetching
	numStacks := 10
	stackRefs := make([]backend.StackReference, numStacks)
	for i := 0; i < numStacks; i++ {
		stackName := fmt.Sprintf("stack%d", i)
		stackRef, err := b.ParseStackReference(stackName)
		require.NoError(t, err)
		stackRefs[i] = stackRef

		// Create the stack
		_, err = b.CreateStack(ctx, stackRef, "", nil, nil)
		require.NoError(t, err)
	}

	// List stacks to trigger parallel fetching
	filter := backend.ListStacksFilter{} // No filter
	stacks, token, err := b.ListStacks(ctx, filter, nil)
	require.NoError(t, err)
	assert.Nil(t, token)
	assert.Len(t, stacks, numStacks)

	// Verify all stacks were fetched
	stackNames := make(map[string]bool)
	for _, stack := range stacks {
		stackNames[stack.Name().String()] = true
	}

	for i := 0; i < numStacks; i++ {
		stackName := fmt.Sprintf("stack%d", i)
		assert.True(t, stackNames[stackName], "Stack %s should be in the results", stackName)
	}
}
