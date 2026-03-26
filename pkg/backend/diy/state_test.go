// Copyright 2026, Pulumi Corporation.
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
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// spyBucket wraps a real Bucket and counts I/O operations.
type spyBucket struct {
	inner Bucket

	mu          sync.Mutex
	existsCalls int
	copyCalls   int
	deleteCalls int
	writeCalls  int
}

func (s *spyBucket) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.Lock()
	s.existsCalls++
	s.mu.Unlock()
	return s.inner.Exists(ctx, key)
}

func (s *spyBucket) Copy(ctx context.Context, dstKey, srcKey string, opts *blob.CopyOptions) error {
	s.mu.Lock()
	s.copyCalls++
	s.mu.Unlock()
	return s.inner.Copy(ctx, dstKey, srcKey, opts)
}

func (s *spyBucket) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	s.deleteCalls++
	s.mu.Unlock()
	return s.inner.Delete(ctx, key)
}

func (s *spyBucket) WriteAll(ctx context.Context, key string, p []byte, opts *blob.WriterOptions) error {
	s.mu.Lock()
	s.writeCalls++
	s.mu.Unlock()
	return s.inner.WriteAll(ctx, key, p, opts)
}

func (s *spyBucket) List(opts *blob.ListOptions) *blob.ListIterator {
	return s.inner.List(opts)
}

func (s *spyBucket) SignedURL(ctx context.Context, key string, opts *blob.SignedURLOptions) (string, error) {
	return s.inner.SignedURL(ctx, key, opts)
}

func (s *spyBucket) ReadAll(ctx context.Context, key string) ([]byte, error) {
	return s.inner.ReadAll(ctx, key)
}

// TestSaveCheckpointBucketOps verifies that saveCheckpoint makes the minimum
// number of bucket operations needed, avoiding unnecessary Exists/Copy/Delete
// calls that add latency on remote backends like S3.
func TestSaveCheckpointBucketOps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		// The compression mode the backend is configured with.
		compression encoding.Compression
		// The compression of the file that already exists on disk
		// (empty string = no file exists).
		existingCompression encoding.Compression
		// Whether to create an existing stack file before the test.
		createExisting bool

		// Expected number of Exists calls during saveCheckpoint.
		// Each Exists call is an S3 HEAD request.
		wantExistsCalls int
	}{
		{
			name:            "no compression, file exists, same format",
			compression:     encoding.CompressionNone,
			createExisting:  true,
			wantExistsCalls: 1, // just backup the current file
		},
		{
			name:            "gzip compression, file exists, same format",
			compression:     encoding.CompressionGzip,
			createExisting:  true,
			wantExistsCalls: 1, // just backup the current file
		},
		{
			name:            "zstd compression, file exists, same format",
			compression:     encoding.CompressionZstd,
			createExisting:  true,
			wantExistsCalls: 1, // just backup the current file
		},
		{
			name:            "no compression, no existing file (first save)",
			compression:     encoding.CompressionNone,
			createExisting:  false,
			wantExistsCalls: 1, // tries to backup, finds nothing
		},
		{
			name:                "switching from none to gzip",
			compression:         encoding.CompressionGzip,
			existingCompression: encoding.CompressionNone,
			createExisting:      true,
			wantExistsCalls:     2, // backup new gzip (not found) + cleanup old plain
		},
		{
			name:                "switching from gzip to none",
			compression:         encoding.CompressionNone,
			existingCompression: encoding.CompressionGzip,
			createExisting:      true,
			wantExistsCalls:     2, // backup new plain (not found) + cleanup old gzip
		},
		{
			name:                "switching from none to zstd",
			compression:         encoding.CompressionZstd,
			existingCompression: encoding.CompressionNone,
			createExisting:      true,
			wantExistsCalls:     2, // backup new zstd (not found) + cleanup old plain
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stateDir := t.TempDir()
			ctx := t.Context()

			// Create the backend with the existing compression first (to create the file),
			// then switch to the target compression.
			existingStore := env.MapStore{}
			switch tt.existingCompression {
			case encoding.CompressionGzip:
				existingStore[env.DIYBackendGzip.Var().Name()] = "true"
			case encoding.CompressionZstd:
				existingStore[env.DIYBackendZstd.Var().Name()] = "true"
			}

			project := &workspace.Project{Name: "testproj"}
			b, err := newDIYBackend(
				ctx, diagtest.LogSink(t),
				"file://"+filepath.ToSlash(stateDir),
				project,
				&diyBackendOptions{Env: env.NewEnv(existingStore)},
			)
			require.NoError(t, err)

			ref, err := b.ParseStackReference("test-stack")
			require.NoError(t, err)

			if tt.createExisting {
				_, err = b.CreateStack(ctx, ref, "", nil, nil)
				require.NoError(t, err)
			}

			// Now create the backend with the target compression and wrap the
			// bucket to count operations.
			targetStore := env.MapStore{}
			switch tt.compression {
			case encoding.CompressionGzip:
				targetStore[env.DIYBackendGzip.Var().Name()] = "true"
			case encoding.CompressionZstd:
				targetStore[env.DIYBackendZstd.Var().Name()] = "true"
			}

			b, err = newDIYBackend(
				ctx, diagtest.LogSink(t),
				"file://"+filepath.ToSlash(stateDir),
				project,
				&diyBackendOptions{Env: env.NewEnv(targetStore)},
			)
			require.NoError(t, err)

			spy := &spyBucket{inner: b.bucket}
			b.bucket = spy

			diyRef := ref.(*diyBackendReference)

			// Get or create a checkpoint to save.
			chk, _, _, _ := b.getCheckpoint(ctx, diyRef)
			if chk == nil {
				chk = &apitype.CheckpointV3{}
			}
			vchk := &apitype.VersionedCheckpoint{
				Version:    3,
				Checkpoint: mustMarshalCheckpoint(t, chk),
			}

			_, _, err = b.saveCheckpoint(ctx, diyRef, vchk)
			require.NoError(t, err)

			assert.Equal(t, tt.wantExistsCalls, spy.existsCalls,
				"expected %d Exists calls (S3 HEAD requests), got %d",
				tt.wantExistsCalls, spy.existsCalls)
		})
	}
}

// TestSaveCheckpointTransitionCleansUpOldFile verifies that when switching
// compression formats, the old format file is removed.
func TestSaveCheckpointTransitionCleansUpOldFile(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	ctx := t.Context()
	project := &workspace.Project{Name: "testproj"}

	stackPath := filepath.Join(stateDir, ".pulumi", "stacks", "testproj", "cleanup-test.json")
	stackPathGzip := stackPath + encoding.GZIPExt
	stackPathZstd := stackPath + encoding.ZSTDExt

	newBackend := func(store env.MapStore) *diyBackend {
		t.Helper()
		b, err := newDIYBackend(
			ctx, diagtest.LogSink(t),
			"file://"+filepath.ToSlash(stateDir),
			project,
			&diyBackendOptions{Env: env.NewEnv(store)},
		)
		require.NoError(t, err)
		return b
	}

	// Create a stack with no compression.
	b := newBackend(env.MapStore{})
	ref, err := b.ParseStackReference("cleanup-test")
	require.NoError(t, err)
	_, err = b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	assert.FileExists(t, stackPath)

	// Switch to gzip. Old plain file should be cleaned up.
	b = newBackend(env.MapStore{env.DIYBackendGzip.Var().Name(): "true"})
	s, err := b.GetStack(ctx, ref)
	require.NoError(t, err)
	require.NotNil(t, s)
	deployment, err := b.ExportDeployment(ctx, s)
	require.NoError(t, err)
	err = b.ImportDeployment(ctx, s, deployment)
	require.NoError(t, err)
	assert.NoFileExists(t, stackPath, "plain file should be removed after switching to gzip")
	assert.FileExists(t, stackPathGzip)
	assert.NoFileExists(t, stackPathZstd)

	// Switch to zstd. Old gzip file should be cleaned up.
	b = newBackend(env.MapStore{env.DIYBackendZstd.Var().Name(): "true"})
	s, err = b.GetStack(ctx, ref)
	require.NoError(t, err)
	require.NotNil(t, s)
	deployment, err = b.ExportDeployment(ctx, s)
	require.NoError(t, err)
	err = b.ImportDeployment(ctx, s, deployment)
	require.NoError(t, err)
	assert.NoFileExists(t, stackPath)
	assert.NoFileExists(t, stackPathGzip, "gzip file should be removed after switching to zstd")
	assert.FileExists(t, stackPathZstd)

	// Switch back to none. Old zstd file should be cleaned up.
	b = newBackend(env.MapStore{})
	s, err = b.GetStack(ctx, ref)
	require.NoError(t, err)
	require.NotNil(t, s)
	deployment, err = b.ExportDeployment(ctx, s)
	require.NoError(t, err)
	err = b.ImportDeployment(ctx, s, deployment)
	require.NoError(t, err)
	assert.FileExists(t, stackPath)
	assert.NoFileExists(t, stackPathGzip)
	assert.NoFileExists(t, stackPathZstd, "zstd file should be removed after switching to none")
}

func mustMarshalCheckpoint(t *testing.T, chk *apitype.CheckpointV3) []byte {
	t.Helper()
	b, err := encoding.JSON.Marshal(chk)
	require.NoError(t, err)
	return b
}
