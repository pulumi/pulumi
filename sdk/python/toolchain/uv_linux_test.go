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

//go:build !windows

package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureVenv(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	uv, err := newUv(root, "")
	require.NoError(t, err)

	// Create a virtualenv and record the directory's inode.
	err = uv.EnsureVenv(context.Background(), root, false /* useLanguageVersionTools */, false, /* showOutput */
		nil /* infoWriter */, nil /* infoWriter */)
	require.NoError(t, err)
	info, err := os.Stat(filepath.Join(root, ".venv"))
	require.NoError(t, err)
	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)
	inode1 := stat.Ino

	// Run EnsureVenv again and ensure the directory's inode is the same.
	err = uv.EnsureVenv(context.Background(), root, false /* useLanguageVersionTools */, false, /* showOutput */
		nil /* infoWriter */, nil /* infoWriter */)
	require.NoError(t, err)
	info, err = os.Stat(filepath.Join(root, ".venv"))
	require.NoError(t, err)
	stat, ok = info.Sys().(*syscall.Stat_t)
	require.True(t, ok)
	inode2 := stat.Ino

	require.Equal(t, inode1, inode2)
}
