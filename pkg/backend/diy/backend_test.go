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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
	"gocloud.dev/blob/fileblob"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // mutates global configuration
func TestEnabledFullyQualifiedStackNames(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	stackName := "organization/project-12345/stack-67890"
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	previous := cmdutil.FullyQualifyStackNames
	expected := stackName

	// Act
	cmdutil.FullyQualifyStackNames = true
	defer func() { cmdutil.FullyQualifyStackNames = previous }()

	actual := s.Ref().String()

	// Assert
	assert.Equal(t, expected, actual)
}

//nolint:paralleltest // mutates global configuration
func TestDisabledFullyQualifiedStackNames(t *testing.T) {
	// Arrange
	// Create a new project
	projectDir := t.TempDir()
	pyaml := filepath.Join(projectDir, "Pulumi.yaml")
	err := os.WriteFile(pyaml, []byte("name: project-12345\nruntime: test"), 0o600)
	require.NoError(t, err)
	proj, err := workspace.LoadProject(pyaml)
	require.NoError(t, err)

	chdir(t, projectDir)

	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), proj)
	require.NoError(t, err)

	stackName := "organization/project-12345/stack-67890"
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	previous := cmdutil.FullyQualifyStackNames
	expected := "stack-67890"

	// Act
	cmdutil.FullyQualifyStackNames = false
	defer func() { cmdutil.FullyQualifyStackNames = previous }()

	actual := s.Ref().String()

	// Assert
	assert.Equal(t, expected, actual)
}

func TestMassageBlobPath(t *testing.T) {
	t.Parallel()

	testMassagePath := func(t *testing.T, s string, want string) {
		t.Helper()

		massaged, err := massageBlobPath(s)
		require.NoError(t, err)
		assert.Equal(t, want, massaged,
			"massageBlobPath(%s) didn't return expected result.\nWant: %q\nGot:  %q", s, want, massaged)
	}

	// URLs not prefixed with "file://" are kept as-is. Also why we add FilePathPrefix as a prefix for other tests.
	t.Run("NonFilePrefixed", func(t *testing.T) {
		t.Parallel()

		testMassagePath(t, "asdf-123", "asdf-123")
	})

	noTmpDirSuffix := "?no_tmp_dir=true"

	// The home directory is converted into the user's actual home directory.
	// Which requires even more tweaks to work on Windows.
	t.Run("PrefixedWithTilde", func(t *testing.T) {
		t.Parallel()

		usr, err := user.Current()
		if err != nil {
			t.Fatalf("Unable to get current user: %v", err)
		}

		homeDir := usr.HomeDir

		// When running on Windows, the "home directory" takes on a different meaning.
		if runtime.GOOS == "windows" {
			t.Logf("Running on %v", runtime.GOOS)

			t.Run("NormalizeDirSeparator", func(t *testing.T) {
				t.Parallel()

				testMassagePath(t, FilePathPrefix+`C:\Users\steve\`, FilePathPrefix+"/C:/Users/steve"+noTmpDirSuffix)
			})

			newHomeDir := "/" + filepath.ToSlash(homeDir)
			t.Logf("Changed homeDir to expect from %q to %q", homeDir, newHomeDir)
			homeDir = newHomeDir
		}

		testMassagePath(t, FilePathPrefix+"~", FilePathPrefix+homeDir+noTmpDirSuffix)
		testMassagePath(t, FilePathPrefix+"~/alpha/beta", FilePathPrefix+homeDir+"/alpha/beta"+noTmpDirSuffix)
	})

	t.Run("MakeAbsolute", func(t *testing.T) {
		t.Parallel()

		// Run the expected result through filepath.Abs, since on Windows we expect "C:\1\2".
		expected := "/1/2"
		abs, err := filepath.Abs(expected)
		require.NoError(t, err)

		expected = filepath.ToSlash(abs)
		if expected[0] != '/' {
			expected = "/" + expected // A leading slash is added on Windows.
		}

		testMassagePath(t, FilePathPrefix+"/1/2/3/../4/..", FilePathPrefix+expected+noTmpDirSuffix)
	})

	t.Run("AlreadySuffixedWithNoTmpDir", func(t *testing.T) {
		t.Parallel()

		testMassagePath(t, FilePathPrefix+"/1?no_tmp_dir=yes", FilePathPrefix+"/1?no_tmp_dir=yes")
	})

	t.Run("AlreadySuffixedWithOtherQuery", func(t *testing.T) {
		t.Parallel()

		testMassagePath(t, FilePathPrefix+"/1?foo=bar", FilePathPrefix+"/1?foo=bar&no_tmp_dir=true")
	})

	t.Run("NoTmpDirFalseStripped", func(t *testing.T) {
		t.Parallel()

		testMassagePath(t, FilePathPrefix+"/1?no_tmp_dir=false", FilePathPrefix+"/1")
		testMassagePath(t, FilePathPrefix+"/1?foo=bar&no_tmp_dir=false", FilePathPrefix+"/1?foo=bar")
	})
}

func TestGetLogsForTargetWithNoSnapshot(t *testing.T) {
	t.Parallel()

	target := &deploy.Target{
		Name:      tokens.MustParseStackName("test"),
		Config:    config.Map{},
		Decrypter: config.NopDecrypter,
		Snapshot:  nil,
	}
	query := operations.LogQuery{}
	res, err := GetLogsForTarget(target, query)
	require.NoError(t, err)
	assert.Nil(t, res)
}

func makeUntypedDeployment(name string, phrase, state string) (*apitype.UntypedDeployment, error) {
	return makeUntypedDeploymentTimestamp(name, phrase, state, nil, nil)
}

func makeUntypedDeploymentTimestamp(
	name string,
	phrase, state string,
	created, modified *time.Time,
) (*apitype.UntypedDeployment, error) {
	sm, err := passphrase.GetPassphraseSecretsManager(phrase, state)
	if err != nil {
		return nil, err
	}

	resources := []*resource.State{
		{
			URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", name),
			Type: "a:b:c",
			Inputs: resource.PropertyMap{
				resource.PropertyKey("secret"): resource.MakeSecret(resource.NewStringProperty("s3cr3t")),
			},
			Created:  created,
			Modified: modified,
		},
	}

	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, resources, nil, deploy.SnapshotMetadata{})
	ctx := context.Background()

	udep, err := stack.SerializeUntypedDeployment(ctx, snap, nil /*opts*/)
	if err != nil {
		return nil, err
	}

	return udep, nil
}

func TestListStacksWithMultiplePassphrases(t *testing.T) {
	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Create stack "a" and import a checkpoint with a secret
	aStackRef, err := b.ParseStackReference("organization/project/a")
	require.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, aStack)
	defer func() {
		t.Setenv("PULUMI_CONFIG_PASSPHRASE", "abc123")
		_, err := b.RemoveStack(ctx, aStack, true /*force*/, false /*removeBackups*/)
		require.NoError(t, err)
	}()
	deployment, err := makeUntypedDeployment("a", "abc123",
		"v1:4iF78gb0nF0=:v1:Co6IbTWYs/UdrjgY:FSrAWOFZnj9ealCUDdJL7LrUKXX9BA==")
	require.NoError(t, err)
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "abc123")
	err = b.ImportDeployment(ctx, aStack, deployment)
	require.NoError(t, err)

	// Create stack "b" and import a checkpoint with a secret
	bStackRef, err := b.ParseStackReference("organization/project/b")
	require.NoError(t, err)
	bStack, err := b.CreateStack(ctx, bStackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, bStack)
	defer func() {
		t.Setenv("PULUMI_CONFIG_PASSPHRASE", "123abc")
		_, err := b.RemoveStack(ctx, bStack, true /*force*/, false /*removeBackups*/)
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
	require.Len(t, stacks, 2)
	for _, stack := range stacks {
		require.NotNil(t, stack.ResourceCount())
		assert.Equal(t, 1, *stack.ResourceCount())
	}
}

func TestDrillError(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Get a non-existent stack and expect a nil error because it won't be found.
	stackRef, err := b.ParseStackReference("organization/project/dev")
	if err != nil {
		t.Fatalf("unexpected error %v when parsing stack reference", err)
	}
	_, err = b.GetStack(ctx, stackRef)
	require.NoError(t, err)
}

func TestCancel(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Check that trying to cancel a stack that isn't created yet doesn't error
	aStackRef, err := b.ParseStackReference("organization/project/a")
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

func TestRemoveMakesBackups(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Grab the bucket interface to test with
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)

	// Check that creating a new stack doesn't make a backup file
	aStackRef, err := lb.parseStackReference("organization/project/a")
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
	removed, err := b.RemoveStack(ctx, aStack, false /*force*/, false /*removeBackups*/)
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

func TestRemoveBackups(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	requireDirNotEmpty := func(lb *diyBackend, dir string) {
		iter := lb.bucket.List(&blob.ListOptions{
			Delimiter: "/",
			Prefix:    dir + "/",
		})
		next, err := iter.Next(ctx)
		require.NotNil(t, next, "Expected directory %q to not be empty", dir)
		require.NoError(t, err, "Expected directory %q to not be empty", dir)
	}

	requireDirEmpty := func(lb *diyBackend, dir string) {
		iter := lb.bucket.List(&blob.ListOptions{
			Delimiter: "/",
			Prefix:    dir + "/",
		})
		next, err := iter.Next(ctx)
		require.Nil(t, next, "Expected directory %q to be empty", dir)
		require.ErrorIs(t, err, io.EOF, "Expected directory %q to be empty", dir)
	}

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Grab the bucket interface to test with
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)

	// Create a new stack
	aStackRef, err := lb.parseStackReference("organization/project/a")
	require.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, aStack)

	// Fake up some history
	err = lb.addToHistory(ctx, aStackRef, backend.UpdateInfo{Kind: apitype.DestroyUpdate})
	require.NoError(t, err)
	requireDirNotEmpty(lb, aStackRef.HistoryDir())

	// Export then import the deployment to create a backup file
	ud, err := lb.ExportDeployment(ctx, aStack)
	require.NoError(t, err)
	require.NotNil(t, ud)
	err = lb.ImportDeployment(ctx, aStack, ud)
	require.NoError(t, err)

	// Check the stack file and backup file now exist
	stackFileExists, err := lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef))
	require.NoError(t, err)
	assert.True(t, stackFileExists)
	backupFileExists, err := lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef)+".bak")
	require.NoError(t, err)
	assert.True(t, backupFileExists)

	// Backup the stack
	err = lb.backupStack(ctx, aStackRef)
	require.NoError(t, err)
	requireDirNotEmpty(lb, aStackRef.BackupDir())

	// Now remove the stack, removing backups
	removed, err := b.RemoveStack(ctx, aStack, false /*force*/, true /*removeBackups*/)
	require.NoError(t, err)
	assert.False(t, removed)

	// Check the stack file and backup files are both gone
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef))
	require.NoError(t, err)
	assert.False(t, stackFileExists)
	backupFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef)+".bak")
	require.NoError(t, err)
	assert.False(t, backupFileExists)

	// Check that the history and backup folders are empty
	requireDirEmpty(lb, aStackRef.BackupDir())
	requireDirEmpty(lb, aStackRef.HistoryDir())
}

func TestRenameWorks(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Grab the bucket interface to test with
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)

	// Create a new stack
	aStackRef, err := lb.parseStackReference("organization/project/a")
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
	bStackRefI, err := b.RenameStack(ctx, aStack, "organization/project/b")
	require.NoError(t, err)
	assert.Equal(t, "organization/project/b", bStackRefI.String())
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
	cStackRefI, err := b.RenameStack(ctx, bStack, "organization/project/c")
	require.NoError(t, err)
	assert.Equal(t, "organization/project/c", cStackRefI.String())
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
	require.Len(t, history, 1)
	assert.Equal(t, apitype.DestroyUpdate, history[0].Kind)
}

func TestRenamePreservesIntegrity(t *testing.T) {
	t.Parallel()

	// Arrange.
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	stackRef, err := b.ParseStackReference("organization/project/a")
	require.NoError(t, err)
	stk, err := b.CreateStack(ctx, stackRef, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, stk)

	rBase := &resource.State{
		URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", "base"),
		Type: "a:b:c",
		Inputs: resource.PropertyMap{
			resource.PropertyKey("p"): resource.NewStringProperty("v"),
		},
	}

	rDependency := &resource.State{
		URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", "dependency"),
		Type: "a:b:c",
		Inputs: resource.PropertyMap{
			resource.PropertyKey("p"): resource.NewStringProperty("v"),
		},
		Dependencies: []resource.URN{rBase.URN},
	}

	rPropertyDependency := &resource.State{
		URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", "property-dependency"),
		Type: "a:b:c",
		Inputs: resource.PropertyMap{
			resource.PropertyKey("p"): resource.NewStringProperty("v"),
		},
		PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			resource.PropertyKey("p"): {rBase.URN},
		},
	}

	rDeletedWith := &resource.State{
		URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", "deleted-with"),
		Type: "a:b:c",
		Inputs: resource.PropertyMap{
			resource.PropertyKey("p"): resource.NewStringProperty("v"),
		},
		DeletedWith: rBase.URN,
	}

	rParent := &resource.State{
		URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", "parent"),
		Type: "a:b:c",
		Inputs: resource.PropertyMap{
			resource.PropertyKey("p"): resource.NewStringProperty("v"),
		},
		Parent: rBase.URN,
	}

	resources := []*resource.State{
		rBase,
		rDependency,
		rPropertyDependency,
		rDeletedWith,
		rParent,
	}

	snap := deploy.NewSnapshot(deploy.Manifest{}, nil, resources, nil, deploy.SnapshotMetadata{})
	ctx = context.Background()

	udep, err := stack.SerializeUntypedDeployment(ctx, snap, nil /*opts*/)
	require.NoError(t, err)

	err = b.ImportDeployment(ctx, stk, udep)
	require.NoError(t, err)

	err = snap.VerifyIntegrity()
	require.NoError(t, err)

	// Act.
	renamedStackRef, err := b.RenameStack(ctx, stk, "organization/project/a-renamed")
	require.NoError(t, err)

	// Assert.
	renamedStk, err := b.GetStack(ctx, renamedStackRef)
	require.NoError(t, err)
	require.NotNil(t, renamedStk)

	renamedSnap, err := renamedStk.Snapshot(ctx, nil)
	require.NoError(t, err)

	err = renamedSnap.VerifyIntegrity()
	require.NoError(t, err)
}

func TestRenameProjectWorks(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Grab the bucket interface to test with
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)

	// Create a new stack
	aStackRef, err := lb.parseStackReference("organization/project/a")
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

	// Rename the project and stack
	bStackRefI, err := b.RenameStack(ctx, aStack, "organization/newProject/b")
	require.NoError(t, err)
	assert.Equal(t, "organization/newProject/b", bStackRefI.String())
	bStackRef := bStackRefI.(*diyBackendReference)

	// Check the new stack file now exists and the old one is gone
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, bStackRef))
	require.NoError(t, err)
	assert.True(t, stackFileExists)
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef))
	require.NoError(t, err)
	assert.False(t, stackFileExists)

	// Check we can still get the history
	history, err := b.GetHistory(ctx, bStackRef, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, apitype.DestroyUpdate, history[0].Kind)
}

func TestLoginToNonExistingFolderFails(t *testing.T) {
	t.Parallel()

	fakeDir := "file://" + filepath.ToSlash(os.TempDir()) + "/non-existing"
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), fakeDir, nil)
	assert.Error(t, err)
	assert.Nil(t, b)
}

// TestParseEmptyStackFails demonstrates that ParseStackReference returns
// an error when the stack name is the empty string.TestParseEmptyStackFails
func TestParseEmptyStackFails(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	_, err = b.ParseStackReference("")
	assert.Error(t, err)
}

// Regression test for https://github.com/pulumi/pulumi/issues/10439
func TestHtmlEscaping(t *testing.T) {
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

	udep, err := stack.SerializeUntypedDeployment(ctx, snap, &stack.SerializeOptions{
		Pretty: true,
	})
	require.NoError(t, err)

	// Ensure data has the string contents "<html@tags>"", not "\u003chtml\u0026tags\u003e"
	// ImportDeployment below should not modify the data
	assert.Contains(t, string(udep.Deployment), "<html@tags>")

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Create stack "a" and import a checkpoint with a secret
	aStackRef, err := b.ParseStackReference("organization/project/a")
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

func TestDIYBackendRejectsStackInitOptions(t *testing.T) {
	t.Parallel()
	// Here, we provide options that illegally specify a team on a
	// backend that does not support teams. We expect this to create
	// an error later when we call CreateStack.
	illegalOptions := &backend.CreateStackOptions{Teams: []string{"red-team"}}

	// • Create a mock diy backend
	tmpDir := t.TempDir()
	dirURI := "file://" + filepath.ToSlash(tmpDir)
	diy, err := New(context.Background(), diagtest.LogSink(t), dirURI, nil)
	require.NoError(t, err)
	ctx := context.Background()

	// • Simulate `pulumi stack init`, passing non-nil init options
	fakeStackRef, err := diy.ParseStackReference("organization/b/foobar")
	require.NoError(t, err)
	_, err = diy.CreateStack(ctx, fakeStackRef, "", nil, illegalOptions)
	assert.ErrorIs(t, err, backend.ErrTeamsNotSupported)
}

func TestLegacyFolderStructure(t *testing.T) {
	t.Parallel()

	// Make a dummy stack file in the legacy location
	tmpDir := t.TempDir()
	err := os.MkdirAll(path.Join(tmpDir, ".pulumi", "stacks"), os.ModePerm)
	require.NoError(t, err)
	err = os.WriteFile(path.Join(tmpDir, ".pulumi", "stacks", "a.json"), []byte("{}"), 0o600)
	require.NoError(t, err)

	// Login to a temp dir diy backend
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)
	// Check the backend says it's NOT in project mode
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)
	assert.IsType(t, &legacyReferenceStore{}, lb.store)

	// Check that list stack shows that stack
	stacks, token, err := b.ListStacks(ctx, backend.ListStacksFilter{}, nil /* inContToken */)
	require.NoError(t, err)
	assert.Nil(t, token)
	require.Len(t, stacks, 1)
	assert.Equal(t, "a", stacks[0].Name().String())

	// Create a new non-project stack
	bRef, err := b.ParseStackReference("b")
	require.NoError(t, err)
	assert.Equal(t, "b", bRef.String())
	bStack, err := b.CreateStack(ctx, bRef, "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "b", bStack.Ref().String())
	assert.FileExists(t, path.Join(tmpDir, ".pulumi", "stacks", "b.json"))
}

func TestListStacksFilter(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	ctx := context.Background()
	tmpDir := t.TempDir()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Create two different project stack
	aRef, err := b.ParseStackReference("organization/proj1/a")
	require.NoError(t, err)
	_, err = b.CreateStack(ctx, aRef, "", nil, nil)
	require.NoError(t, err)

	bRef, err := b.ParseStackReference("organization/proj2/b")
	require.NoError(t, err)
	_, err = b.CreateStack(ctx, bRef, "", nil, nil)
	require.NoError(t, err)

	// Check that list stack with a filter only shows one stack
	filter := "proj1"
	stacks, token, err := b.ListStacks(ctx, backend.ListStacksFilter{
		Project: &filter,
	}, nil /* inContToken */)
	require.NoError(t, err)
	assert.Nil(t, token)
	require.Len(t, stacks, 1)
	assert.Equal(t, "organization/proj1/a", stacks[0].Name().String())
}

func TestOptIntoLegacyFolderStructure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ctx := context.Background()
	s := make(env.MapStore)
	s[env.DIYBackendLegacyLayout.Var().Name()] = "true"
	b, err := newDIYBackend(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil,
		&diyBackendOptions{Env: env.NewEnv(s)},
	)
	require.NoError(t, err)

	// Verify that a new stack is created in the legacy location.
	foo, err := b.ParseStackReference("foo")
	require.NoError(t, err)

	_, err = b.CreateStack(ctx, foo, "", nil, nil)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(tmpDir, ".pulumi", "stacks", "foo.json"))
}

// Verifies that the StackReference.String method
// takes the current project name into account,
// even if the current project name changes
// after the stack reference is created.
func TestStackReferenceString_currentProjectChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(dir), nil)
	require.NoError(t, err)

	foo, err := b.ParseStackReference("organization/proj1/foo")
	require.NoError(t, err)

	bar, err := b.ParseStackReference("organization/proj2/bar")
	require.NoError(t, err)

	assert.Equal(t, "organization/proj1/foo", foo.String())
	assert.Equal(t, "organization/proj2/bar", bar.String())

	// Change the current project name
	b.SetCurrentProject(&workspace.Project{Name: "proj1"})

	assert.Equal(t, "foo", foo.String())
	assert.Equal(t, "organization/proj2/bar", bar.String())
}

// Verifies that there's no data race in calling StackReference.String
// and diyBackend.SetCurrentProject concurrently.
func TestStackReferenceString_currentProjectChange_race(t *testing.T) {
	t.Parallel()

	const N = 1000

	dir := t.TempDir()
	ctx := context.Background()

	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(dir), nil)
	require.NoError(t, err)

	projects := make([]*workspace.Project, N)
	refs := make([]backend.StackReference, N)
	for i := 0; i < N; i++ {
		name := fmt.Sprintf("proj%d", i)
		projects[i] = &workspace.Project{Name: tokens.PackageName(name)}
		refs[i], err = b.ParseStackReference(fmt.Sprintf("organization/%v/foo", name))
		require.NoError(t, err)
	}

	// To exercise this data race, we'll have two goroutines.
	// One goroutine will call StackReference.String repeatedly
	// on all the stack references,
	// and the other goroutine will call diyBackend.SetCurrentProject
	// with all the projects.

	var wg sync.WaitGroup
	ready := make(chan struct{}) // both goroutines wait on this

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ready
		for i := 0; i < N; i++ {
			_ = refs[i].String()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ready
		for i := 0; i < N; i++ {
			b.SetCurrentProject(projects[i])
		}
	}()

	close(ready) // start racing
	wg.Wait()
}

func TestProjectFolderStructure(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend

	// Make a dummy file in the legacy location which isn't a stack file, we should still automatically turn
	// this into project mode.
	tmpDir := t.TempDir()
	err := os.MkdirAll(path.Join(tmpDir, ".pulumi", "plugins"), os.ModePerm)
	require.NoError(t, err)
	err = os.MkdirAll(path.Join(tmpDir, ".pulumi", "stacks"), os.ModePerm)
	require.NoError(t, err)
	err = os.WriteFile(path.Join(tmpDir, ".pulumi", "stacks", "a.txt"), []byte("{}"), 0o600)
	require.NoError(t, err)

	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	// Check the backend says it's in project mode
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)
	assert.IsType(t, &projectReferenceStore{}, lb.store)

	// Make a dummy stack file in the new project location
	err = os.MkdirAll(path.Join(tmpDir, ".pulumi", "stacks", "testproj"), os.ModePerm)
	require.NoError(t, err)
	err = os.WriteFile(path.Join(tmpDir, ".pulumi", "stacks", "testproj", "a.json"), []byte("{}"), 0o600)
	require.NoError(t, err)

	// Check that testproj is reported as existing
	exists, err := b.DoesProjectExist(ctx, "", "testproj")
	require.NoError(t, err)
	assert.True(t, exists)

	// Check that list stack shows that stack
	stacks, token, err := b.ListStacks(ctx, backend.ListStacksFilter{}, nil /* inContToken */)
	require.NoError(t, err)
	assert.Nil(t, token)
	require.Len(t, stacks, 1)
	assert.Equal(t, "organization/testproj/a", stacks[0].Name().String())

	// Create a new project stack
	bRef, err := b.ParseStackReference("organization/testproj/b")
	require.NoError(t, err)
	assert.Equal(t, "organization/testproj/b", bRef.String())
	bStack, err := b.CreateStack(ctx, bRef, "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "organization/testproj/b", bStack.Ref().String())
	assert.FileExists(t, path.Join(tmpDir, ".pulumi", "stacks", "testproj", "b.json"))
}

func chdir(t *testing.T, dir string) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir)) // Set directory
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(cwd)) // Restore directory
		restoredDir, err := os.Getwd()
		require.NoError(t, err)
		require.Equal(t, cwd, restoredDir)
	})
}

//nolint:paralleltest // mutates cwd
func TestProjectNameMustMatch(t *testing.T) {
	// Create a new project
	projectDir := t.TempDir()
	pyaml := filepath.Join(projectDir, "Pulumi.yaml")
	err := os.WriteFile(pyaml, []byte("name: my-project\nruntime: test"), 0o600)
	require.NoError(t, err)
	proj, err := workspace.LoadProject(pyaml)
	require.NoError(t, err)

	chdir(t, projectDir)

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), proj)
	require.NoError(t, err)

	// Create a new implicit-project stack
	aRef, err := b.ParseStackReference("a")
	require.NoError(t, err)
	assert.Equal(t, "a", aRef.String())
	aStack, err := b.CreateStack(ctx, aRef, "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "a", aStack.Ref().String())
	assert.FileExists(t, path.Join(tmpDir, ".pulumi", "stacks", "my-project", "a.json"))

	// Create a new project stack with the wrong project name
	bRef, err := b.ParseStackReference("organization/not-my-project/b")
	require.NoError(t, err)
	assert.Equal(t, "organization/not-my-project/b", bRef.String())
	bStack, err := b.CreateStack(ctx, bRef, "", nil, nil)
	assert.Error(t, err)
	assert.Nil(t, bStack)

	// Create a new project stack with the right project name
	cRef, err := b.ParseStackReference("organization/my-project/c")
	require.NoError(t, err)
	assert.Equal(t, "c", cRef.String())
	cStack, err := b.CreateStack(ctx, cRef, "", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "c", cStack.Ref().String())
	assert.FileExists(t, path.Join(tmpDir, ".pulumi", "stacks", "my-project", "c.json"))
}

func TestNew_legacyFileWarning(t *testing.T) {
	t.Parallel()

	// Verifies the names of files printed in warnings
	// when legacy files are found while running in project mode.

	tests := []struct {
		desc    string
		files   map[string]string
		env     env.MapStore
		wantOut string
	}{
		{
			desc: "no legacy stacks",
			files: map[string]string{
				// Should ignore non-stack files.
				".pulumi/foo/extraneous_file": "",
			},
		},
		{
			desc: "legacy stacks",
			files: map[string]string{
				".pulumi/stacks/a.json":     "{}",
				".pulumi/stacks/b.json":     "{}",
				".pulumi/stacks/c.json.bak": "{}", // should ignore backup files
			},
			wantOut: "warning: Found legacy stack files in state store:\n" +
				"  - a\n" +
				"  - b\n" +
				"Please run 'pulumi state upgrade' to migrate them to the new format.\n" +
				"Set PULUMI_DIY_BACKEND_NO_LEGACY_WARNING=1 to disable this warning.\n",
		},
		{
			desc: "warning opt-out",
			files: map[string]string{
				".pulumi/stacks/a.json": "{}",
				".pulumi/stacks/b.json": "{}",
			},
			env: map[string]string{
				"PULUMI_DIY_BACKEND_NO_LEGACY_WARNING": "true",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			stateDir := t.TempDir()
			bucket, err := fileblob.OpenBucket(stateDir, nil)
			require.NoError(t, err)

			ctx := context.Background()
			require.NoError(t,
				bucket.WriteAll(ctx, ".pulumi/meta.yaml", []byte("version: 1"), nil),
				"write meta.yaml")

			for path, contents := range tt.files {
				require.NoError(t, bucket.WriteAll(ctx, path, []byte(contents), nil),
					"write %q", path)
			}

			var buff bytes.Buffer
			sink := diag.DefaultSink(io.Discard, &buff, diag.FormatOptions{Color: colors.Never})

			_, err = newDIYBackend(ctx, sink, "file://"+filepath.ToSlash(stateDir), nil,
				&diyBackendOptions{Env: env.NewEnv(tt.env)})
			require.NoError(t, err)

			assert.Equal(t, tt.wantOut, buff.String())
		})
	}
}

func TestLegacyUpgrade(t *testing.T) {
	t.Parallel()

	// Make a dummy stack file in the legacy location
	tmpDir := t.TempDir()
	err := os.MkdirAll(path.Join(tmpDir, ".pulumi", "stacks"), os.ModePerm)
	require.NoError(t, err)
	err = os.WriteFile(path.Join(tmpDir, ".pulumi", "stacks", "a.json"), []byte(`{
		"latest": {
			"resources": [
				{
					"type": "package:module:resource",
					"urn": "urn:pulumi:stack::project::package:module:resource::name"
				}
			]
		}
	}`), 0o600)
	require.NoError(t, err)

	var output bytes.Buffer
	sink := diag.DefaultSink(&output, &output, diag.FormatOptions{Color: colors.Never})

	// Login to a temp dir diy backend
	ctx := context.Background()
	b, err := New(ctx, sink, "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)
	// Check the backend says it's NOT in project mode
	lb, ok := b.(*diyBackend)
	assert.True(t, ok)
	require.NotNil(t, lb)
	assert.IsType(t, &legacyReferenceStore{}, lb.store)

	err = lb.Upgrade(ctx, nil /* opts */)
	require.NoError(t, err)
	assert.IsType(t, &projectReferenceStore{}, lb.store)

	assert.Contains(t, output.String(), "Upgraded 1 stack(s) to project mode")

	// Check that a has been moved
	aStackRef, err := lb.parseStackReference("organization/project/a")
	require.NoError(t, err)
	stackFileExists, err := lb.bucket.Exists(ctx, lb.stackPath(ctx, aStackRef))
	require.NoError(t, err)
	assert.True(t, stackFileExists)

	// Write b.json and upgrade again
	err = os.WriteFile(path.Join(tmpDir, ".pulumi", "stacks", "b.json"), []byte(`{
		"latest": {
			"resources": [
				{
					"type": "package:module:resource",
					"urn": "urn:pulumi:stack::other-project::package:module:resource::name"
				}
			]
		}
	}`), 0o600)
	require.NoError(t, err)

	err = lb.Upgrade(ctx, nil /* opts */)
	require.NoError(t, err)

	// Check that b has been moved
	bStackRef, err := lb.parseStackReference("organization/other-project/b")
	require.NoError(t, err)
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(ctx, bStackRef))
	require.NoError(t, err)
	assert.True(t, stackFileExists)
}

func TestLegacyUpgrade_partial(t *testing.T) {
	t.Parallel()

	// Verifies that we can upgrade a subset of stacks.

	stateDir := t.TempDir()
	bucket, err := fileblob.OpenBucket(stateDir, nil)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t,
		bucket.WriteAll(ctx, ".pulumi/stacks/foo.json", []byte(`{
		"latest": {
			"resources": [
				{
					"type": "package:module:resource",
					"urn": "urn:pulumi:stack::project::package:module:resource::name"
				}
			]
		}
	}`), nil))
	require.NoError(t,
		// no resources, can't guess project name
		bucket.WriteAll(ctx, ".pulumi/stacks/bar.json",
			[]byte(`{"latest": {"resources": []}}`), nil))

	var buff bytes.Buffer
	sink := diag.DefaultSink(io.Discard, &buff, diag.FormatOptions{Color: colors.Never})
	b, err := New(ctx, sink, "file://"+filepath.ToSlash(stateDir), nil)
	require.NoError(t, err)

	require.NoError(t, b.Upgrade(ctx, nil /* opts */))
	assert.Contains(t, buff.String(), `Skipping stack "bar": no project name found`)

	exists, err := bucket.Exists(ctx, ".pulumi/stacks/project/foo.json")
	require.NoError(t, err)
	assert.True(t, exists, "foo was not migrated")

	ref, err := b.ParseStackReference("organization/project/foo")
	require.NoError(t, err)
	assert.Equal(t, tokens.QName("organization/project/foo"), ref.FullyQualifiedName())
}

// When a stack project could not be determined,
// we should fill it in with ProjectsForDetachedStacks.
func TestLegacyUpgrade_ProjectsForDetachedStacks(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	bucket, err := fileblob.OpenBucket(stateDir, nil)
	require.NoError(t, err)

	// Write a few empty stacks.
	// These stacks have no resources, so we can't guess the project name.
	ctx := context.Background()
	for _, stack := range []string{"foo", "bar", "baz"} {
		statePath := path.Join(".pulumi", "stacks", stack+".json")
		require.NoError(t,
			bucket.WriteAll(ctx, statePath,
				[]byte(`{"latest": {"resources": []}}`), nil),
			"write stack %s", stack)
	}

	var stderr bytes.Buffer
	sink := diag.DefaultSink(io.Discard, &stderr, diag.FormatOptions{Color: colors.Never})
	b, err := New(ctx, sink, "file://"+filepath.ToSlash(stateDir), nil)
	require.NoError(t, err)

	// For the first two stacks, we'll return project names to upgrade them.
	// For the third stack, we will not set a project name, and it should be skipped.
	err = b.Upgrade(ctx, &UpgradeOptions{
		ProjectsForDetachedStacks: func(stacks []tokens.StackName) (projects []tokens.Name, err error) {
			assert.ElementsMatch(t, []tokens.StackName{
				tokens.MustParseStackName("foo"),
				tokens.MustParseStackName("bar"),
				tokens.MustParseStackName("baz"),
			}, stacks)

			projects = make([]tokens.Name, len(stacks))
			for idx, stack := range stacks {
				switch stack.String() {
				case "foo":
					projects[idx] = "proj1"
				case "bar":
					projects[idx] = "proj2"
				case "baz":
					// Leave baz detached.
				}
			}
			return projects, nil
		},
	})
	require.NoError(t, err)

	for _, stack := range []string{"foo", "bar"} {
		assert.NotContains(t, stderr.String(), fmt.Sprintf("Skipping stack %q", stack))
	}
	assert.Contains(t, stderr.String(), fmt.Sprintf("Skipping stack %q", "baz"))

	wantFiles := []string{
		".pulumi/stacks/proj1/foo.json",
		".pulumi/stacks/proj2/bar.json",
		".pulumi/stacks/baz.json",
	}
	for _, file := range wantFiles {
		exists, err := bucket.Exists(ctx, file)
		require.NoError(t, err, "exists(%q)", file)
		assert.True(t, exists, "file %q must exist", file)
	}
}

// When a stack project could not be determined
// and ProjectsForDetachedStacks returns an error,
// the upgrade should fail.
func TestLegacyUpgrade_ProjectsForDetachedStacks_error(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	bucket, err := fileblob.OpenBucket(stateDir, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// We have one stack with a guessable project name, and one without.
	// If ProjectsForDetachedStacks returns an error, the upgrade should
	// fail for both because the user likely cancelled the upgrade.
	require.NoError(t,
		bucket.WriteAll(ctx, ".pulumi/stacks/foo.json", []byte(`{
		"latest": {
			"resources": [
				{
					"type": "package:module:resource",
					"urn": "urn:pulumi:stack::project::package:module:resource::name"
				}
			]
		}
	}`), nil))
	require.NoError(t,
		bucket.WriteAll(ctx, ".pulumi/stacks/bar.json",
			[]byte(`{"latest": {"resources": []}}`), nil))

	sink := diag.DefaultSink(io.Discard, iotest.LogWriter(t), diag.FormatOptions{Color: colors.Never})
	b, err := New(ctx, sink, "file://"+filepath.ToSlash(stateDir), nil)
	require.NoError(t, err)

	giveErr := errors.New("canceled operation")
	err = b.Upgrade(ctx, &UpgradeOptions{
		ProjectsForDetachedStacks: func(stacks []tokens.StackName) (projects []tokens.Name, err error) {
			assert.Equal(t, []tokens.StackName{
				tokens.MustParseStackName("bar"),
			}, stacks)
			return nil, giveErr
		},
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, giveErr)

	wantFiles := []string{
		".pulumi/stacks/foo.json",
		".pulumi/stacks/bar.json",
	}
	for _, file := range wantFiles {
		exists, err := bucket.Exists(ctx, file)
		require.NoError(t, err, "exists(%q)", file)
		assert.True(t, exists, "file %q must exist", file)
	}
}

// If an upgrade failed because we couldn't write the meta.yaml,
// the stacks should be left in legacy mode.
func TestLegacyUpgrade_writeMetaError(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	bucket, err := fileblob.OpenBucket(stateDir, nil)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t,
		bucket.WriteAll(ctx, ".pulumi/stacks/foo.json", []byte(`{
		"latest": {
			"resources": [
				{
					"type": "package:module:resource",
					"urn": "urn:pulumi:stack::project::package:module:resource::name"
				}
			]
		}
	}`), nil))

	// To prevent a write to meta.yaml, we'll create a directory with that name.
	// The system will reject creating a file with the same name.
	require.NoError(t, os.MkdirAll(filepath.Join(stateDir, ".pulumi", "meta.yaml"), 0o755))

	var buff bytes.Buffer
	sink := diag.DefaultSink(io.Discard, &buff, diag.FormatOptions{Color: colors.Never})
	b, err := New(ctx, sink, "file://"+filepath.ToSlash(stateDir), nil)
	require.NoError(t, err)

	require.Error(t, b.Upgrade(ctx, nil /* opts */))

	stderr := buff.String()
	assert.Contains(t, stderr, "error: Could not write new state metadata file")
	assert.Contains(t, stderr, "Please verify that the storage is writable")

	assert.FileExists(t, filepath.Join(stateDir, ".pulumi", "stacks", "foo.json"),
		"foo.json should not have been upgraded")
}

func TestNew_unsupportedStoreVersion(t *testing.T) {
	t.Parallel()

	// Verifies that we fail to initialize a backend if the store version is
	// newer than the CLI version.

	stateDir := t.TempDir()
	bucket, err := fileblob.OpenBucket(stateDir, nil)
	require.NoError(t, err)

	// Set up a meta.yaml "from the future".
	ctx := context.Background()
	require.NoError(t,
		bucket.WriteAll(ctx, ".pulumi/meta.yaml", []byte("version: 999999999"), nil))

	_, err = New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(stateDir), nil)
	assert.ErrorContains(t, err, "state store unsupported")
	assert.ErrorContains(t, err, "'meta.yaml' version (999999999) is not supported")
}

// TestSerializeTimestampRFC3339 captures our expectations that Created and Modified will be serialized to
// RFC3339.
func TestSerializeTimestampRFC3339(t *testing.T) {
	t.Parallel()

	created := time.Now().UTC()
	modified := created.Add(time.Hour)

	deployment, err := makeUntypedDeploymentTimestamp("b", "123abc",
		"v1:C7H2a7/Ietk=:v1:yfAd1zOi6iY9DRIB:dumdsr+H89VpHIQWdB01XEFqYaYjAg==", &created, &modified)
	require.NoError(t, err)

	createdStr := created.Format(time.RFC3339Nano)
	modifiedStr := modified.Format(time.RFC3339Nano)
	assert.Contains(t, string(deployment.Deployment), createdStr)
	assert.Contains(t, string(deployment.Deployment), modifiedStr)
}

func TestUpgrade_manyFailures(t *testing.T) {
	t.Parallel()

	const (
		numStacks    = 100
		badStackBody = `{"latest": {"resources": []}}`
	)

	tmpDir := t.TempDir()

	bucket, err := fileblob.OpenBucket(tmpDir, nil)
	require.NoError(t, err)
	ctx := context.Background()
	for i := 0; i < numStacks; i++ {
		stackPath := path.Join(".pulumi", "stacks", fmt.Sprintf("stack-%d.json", i))
		require.NoError(t, bucket.WriteAll(ctx, stackPath, []byte(badStackBody), nil))
	}

	var output bytes.Buffer
	sink := diag.DefaultSink(io.Discard, &output, diag.FormatOptions{Color: colors.Never})

	// Login to a temp dir diy backend
	b, err := New(ctx, sink, "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	require.NoError(t, b.Upgrade(ctx, nil /* opts */))
	out := output.String()
	for i := 0; i < numStacks; i++ {
		assert.Contains(t, out, fmt.Sprintf(`Skipping stack "stack-%d"`, i))
	}
}

func TestCreateStack_gzip(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	ctx := context.Background()

	s := make(env.MapStore)
	s[env.DIYBackendGzip.Var().Name()] = "true"

	b, err := newDIYBackend(
		ctx,
		diagtest.LogSink(t), "file://"+filepath.ToSlash(stateDir),
		&workspace.Project{Name: "testproj"},
		&diyBackendOptions{Env: env.NewEnv(s)},
	)
	require.NoError(t, err)

	fooRef, err := b.ParseStackReference("foo")
	require.NoError(t, err)

	_, err = b.CreateStack(ctx, fooRef, "", nil, nil)
	require.NoError(t, err)

	// With PULUMI_DIY_BACKEND_GZIP enabled,
	// we'll store state into gzipped files.
	assert.FileExists(t, filepath.Join(stateDir, ".pulumi", "stacks", "testproj", "foo.json.gz"))
}

func TestCreateStack_retainCheckpoints(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	ctx := context.Background()

	s := make(env.MapStore)
	s[env.DIYBackendRetainCheckpoints.Var().Name()] = "true"

	b, err := newDIYBackend(
		ctx,
		diagtest.LogSink(t), "file://"+filepath.ToSlash(stateDir),
		&workspace.Project{Name: "testproj"},
		&diyBackendOptions{Env: env.NewEnv(s)},
	)
	require.NoError(t, err)

	fooRef, err := b.ParseStackReference("foo")
	require.NoError(t, err)

	_, err = b.CreateStack(ctx, fooRef, "", nil, nil)
	require.NoError(t, err)

	// With PULUMI_RETAIN_CHECKPOINTS enabled,
	// we'll make copies of files under $orig.$timestamp.
	// Since we can't predict the timestamp,
	// we'll just check that there's at least one file
	// with a timestamp extension.
	got, err := filepath.Glob(
		filepath.Join(stateDir, ".pulumi", "stacks", "testproj", "foo.json.*"))
	require.NoError(t, err)

	checkpointExtRe := regexp.MustCompile(`^\.[0-9]+$`)
	var found bool
	for _, f := range got {
		if checkpointExtRe.MatchString(filepath.Ext(f)) {
			found = true
			break
		}
	}
	assert.True(t, found,
		"file with a timestamp extension not found in %v", got)
}

// Tests that a DIY backend's CreateStack implementation will persist supplied initial states.
func TestCreateStack_WritesInitialState(t *testing.T) {
	t.Parallel()

	// Arrange.
	//
	// Matching expected and actual state byte-for-byte is tricky due to e.g. whitespace changes that may occur during
	// JSON serialization. Consequently we implement a best-effort approach where we look for a (hopefully) sufficiently
	// unique token in the serialized state.

	magic := "6826601b489b8b121f77668d401fe7cfc7d1488148e57ed6987b7303ab066919"

	cases := []struct {
		name     string
		state    *apitype.UntypedDeployment
		contains string
	}{
		{
			name: "invalid",
			state: &apitype.UntypedDeployment{
				Version:    3,
				Deployment: json.RawMessage(`{"manifest":1337331}`),
			},
			contains: `1337331`,
		},
		{
			name: "invalid snapshot (magic number)",
			state: &apitype.UntypedDeployment{
				Version:    3,
				Deployment: []byte(`{"manifest":{"magic":"incorrect", "version": "3.134.1-dev.1337"}}`),
			},
			contains: `"3.134.1-dev.1337"`,
		},
		{
			name: "invalid snapshot (bad dependencies)",
			state: &apitype.UntypedDeployment{
				Version: 3,
				Deployment: []byte(`{
					"resources": [
						{
							"urn": "urn:pulumi:stack::proj::type::name1",
							"type": "type",
							"parent": "urn:pulumi:stack::proj::type::name2"
						},
						{
							"urn": "urn:pulumi:stack::proj::type::name2",
							"type": "type"
						}
					]
				}`),
			},
			contains: "urn:pulumi:stack::proj::type::name2",
		},
		{
			name: "valid",
			state: &apitype.UntypedDeployment{
				Version: 3,
				Deployment: []byte(`{
					"manifest":{
						"time": "2024-09-24T17:40:37.722248188+01:00",
						"magic": "` + magic + `",
						"version": "3.134.1-dev.0"
					}
				}`),
			},
			contains: magic,
		},
	}

	project := "testproj"
	stack := "teststack"

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			stateDir := t.TempDir()
			stateFile := path.Join(stateDir, ".pulumi", "stacks", project, stack+".json")

			ctx := context.Background()

			b, err := newDIYBackend(
				ctx,
				diagtest.LogSink(t), "file://"+filepath.ToSlash(stateDir),
				&workspace.Project{Name: tokens.PackageName(project)},
				nil,
			)
			require.NoError(t, err)

			stackRef, err := b.ParseStackReference(stack)
			require.NoError(t, err)

			// Act.
			_, err = b.CreateStack(ctx, stackRef, "", c.state, nil)

			// Assert.
			require.NoError(t, err)
			assert.FileExists(t, stateFile)
			stateBytes, err := os.ReadFile(stateFile)
			require.NoError(t, err)
			assert.Contains(t, string(stateBytes), c.contains)
		})
	}
}

//nolint:paralleltest // mutates global state
func TestDisableIntegrityChecking(t *testing.T) {
	stateDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(stateDir), &workspace.Project{Name: "testproj"})
	require.NoError(t, err)

	ref, err := b.ParseStackReference("stack")
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	// make up a bad stack
	deployment := apitype.UntypedDeployment{
		Version: 3,
		Deployment: json.RawMessage(`{
			"resources": [
				{
					"urn": "urn:pulumi:stack::proj::type::name1",
					"type": "type",
					"parent": "urn:pulumi:stack::proj::type::name2"
				},
				{
					"urn": "urn:pulumi:stack::proj::type::name2",
					"type": "type"
				}
			]
		}`),
	}

	// Import deployment doesn't verify the deployment
	err = b.ImportDeployment(ctx, s, &deployment)
	require.NoError(t, err)

	backend.DisableIntegrityChecking = false
	snap, err := s.Snapshot(ctx, b64.Base64SecretsProvider)
	require.ErrorContains(t, err,
		"child resource urn:pulumi:stack::proj::type::name1's parent urn:pulumi:stack::proj::type::name2 comes after it")
	assert.Nil(t, snap)

	backend.DisableIntegrityChecking = true
	snap, err = s.Snapshot(ctx, b64.Base64SecretsProvider)
	require.NoError(t, err)
	require.NotNil(t, snap)
}

func TestParallelStackFetch(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create a custom environment with DIYBackendParallel set
	s := make(env.MapStore)
	s[env.DIYBackendParallel.Var().Name()] = "5" // Set parallel to 5

	b, err := newDIYBackend(
		ctx,
		diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir),
		&workspace.Project{Name: "testproj"},
		&diyBackendOptions{Env: env.NewEnv(s)},
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
	require.Len(t, stacks, numStacks)

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

func TestParallelStackFetchDefaultValue(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create a backend without setting DIYBackendParallel
	b, err := newDIYBackend(
		ctx,
		diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir),
		&workspace.Project{Name: "testproj"},
		&diyBackendOptions{Env: env.NewEnv(make(env.MapStore))},
	)
	require.NoError(t, err)

	// Create multiple stacks to test parallel fetching with default value
	numStacks := 5
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

	// List stacks to trigger parallel fetching with default value
	filter := backend.ListStacksFilter{} // No filter
	stacks, token, err := b.ListStacks(ctx, filter, nil)
	require.NoError(t, err)
	assert.Nil(t, token)
	require.Len(t, stacks, numStacks)

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

func TestListStackNames(t *testing.T) {
	t.Parallel()

	// Login to a temp dir diy backend
	tmpDir := t.TempDir()
	ctx := context.Background()

	b, err := newDIYBackend(
		ctx,
		diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir),
		&workspace.Project{Name: "testproj"},
		&diyBackendOptions{Env: env.NewEnv(make(env.MapStore))},
	)
	require.NoError(t, err)

	// Create test stacks
	numStacks := 3
	expectedStackNames := make([]string, numStacks)
	for i := 0; i < numStacks; i++ {
		stackName := fmt.Sprintf("stack%d", i)
		expectedStackNames[i] = stackName
		stackRef, err := b.ParseStackReference(stackName)
		require.NoError(t, err)

		// Create the stack
		_, err = b.CreateStack(ctx, stackRef, "", nil, nil)
		require.NoError(t, err)
	}

	// Test ListStackNames (should only return references, no metadata fetching)
	filter := backend.ListStackNamesFilter{} // No filter
	stackRefs, token, err := b.ListStackNames(ctx, filter, nil)
	require.NoError(t, err)
	assert.Nil(t, token)
	require.Len(t, stackRefs, numStacks)

	// Verify all expected stack names are present
	actualNames := make(map[string]bool)
	for _, stackRef := range stackRefs {
		actualNames[stackRef.Name().String()] = true
	}

	for _, expectedName := range expectedStackNames {
		assert.True(t, actualNames[expectedName], "Stack %s should be in the results", expectedName)
	}

	// Verify that ListStackNames doesn't return StackSummary objects (just references)
	// This ensures we're not fetching metadata unnecessarily
	assert.IsType(t, []backend.StackReference{}, stackRefs)
}
