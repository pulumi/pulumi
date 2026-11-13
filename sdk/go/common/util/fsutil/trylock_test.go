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

//go:build !js

package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTryLockFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "lock")
	owner, err := os.Create(path)
	require.NoError(t, err)
	defer owner.Close()
	other, err := os.Open(path)
	require.NoError(t, err)
	defer other.Close()

	require.True(t, TryLockFile(owner))
	require.False(t, TryLockFile(other))

	require.NoError(t, UnlockFile(owner))
	require.True(t, TryLockFile(other))
	require.False(t, TryLockFile(owner))
	require.NoError(t, UnlockFile(other))
}
