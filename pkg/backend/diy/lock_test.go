// Copyright 2016, Pulumi Corporation.
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
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockURLForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		lockPath string
		expected string
	}{
		{
			name:     "Local file URL",
			baseURL:  "file:///Users/user",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "file:///Users/user/.pulumi/locks/organization/proj/stack/" +
				"18262c43-124d-4f19-b90f-24db3c0a22a3.json",
		},
		{
			name:     "Local file URL with query param",
			baseURL:  "file:///Users/user?no_tmp_dir=true",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "file:///Users/user/.pulumi/locks/organization/proj/stack/" +
				"18262c43-124d-4f19-b90f-24db3c0a22a3.json?no_tmp_dir=true",
		},
		{
			name:     "S3 URL",
			baseURL:  "s3://mybucket/testfile",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "s3://mybucket/testfile/.pulumi/locks/organization/proj/stack/" +
				"18262c43-124d-4f19-b90f-24db3c0a22a3.json",
		},
		{
			name:     "S3 URL with query param",
			baseURL:  "s3://mybucket/testfile?region=eu-central-1",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "s3://mybucket/testfile/.pulumi/locks/organization/proj/stack/" +
				"18262c43-124d-4f19-b90f-24db3c0a22a3.json?region=eu-central-1",
		},
		{
			name:     "Local path",
			baseURL:  "/Users/user",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "/Users/user/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
		},
		{
			name:     "Invalid URL format",
			baseURL:  ":bad:url",
			lockPath: "lock/file",
			expected: ":bad:url/lock/file",
		},
		{
			name:     "URL with password",
			baseURL:  "https://user:password@example.com",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "https://****:****@example.com/.pulumi/locks/organization/proj/stack/" +
				"18262c43-124d-4f19-b90f-24db3c0a22a3.json",
		},
		{
			name:     "URL without password",
			baseURL:  "https://example.com",
			lockPath: "/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
			expected: "https://example.com/.pulumi/locks/organization/proj/stack/18262c43-124d-4f19-b90f-24db3c0a22a3.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			backend := &diyBackend{url: tt.baseURL}
			result := backend.lockURLForError(tt.lockPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCheckForLock_StealsDeadLock proves crash recovery: a lock left by a process on THIS
// host that is no longer running is provably stale, so checkForLock removes it and proceeds
// instead of demanding a manual `pulumi cancel`. A lock whose owner is alive still blocks.
func TestCheckForLock_StealsDeadLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmpDir := t.TempDir()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)
	ref, err := b.ParseStackReference("organization/proj/locked")
	require.NoError(t, err)

	hostname, err := os.Hostname()
	require.NoError(t, err)
	writeLock := func(name string, pid int) string {
		content, merr := json.Marshal(lockContent{
			Pid: pid, Username: "tester", Hostname: hostname, Timestamp: time.Now(),
		})
		require.NoError(t, merr)
		path := filepath.ToSlash(filepath.Join(stackLockDir(ref.FullyQualifiedName()), name))
		require.NoError(t, b.(*diyBackend).bucket.WriteAll(ctx, path, content, nil))
		return path
	}

	// A dead pid: spawn and reap a real process so the pid existed but is gone.
	cmd := exec.Command("true")
	require.NoError(t, cmd.Start())
	deadPid := cmd.Process.Pid
	require.NoError(t, cmd.Wait())

	writeLock("dead.json", deadPid)
	err = b.(*diyBackend).checkForLock(ctx, ref)
	require.NoError(t, err, "a same-host lock with a dead owner must be stolen, not block")

	// The stale lock file itself must be gone, so nothing re-trips on it.
	files, err := listBucket(ctx, b.(*diyBackend).bucket, stackLockDir(ref.FullyQualifiedName()))
	require.NoError(t, err)
	assert.Empty(t, files)

	// A live owner (this test process) still blocks.
	writeLock("live.json", os.Getpid())
	err = b.(*diyBackend).checkForLock(ctx, ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "currently locked")
}
