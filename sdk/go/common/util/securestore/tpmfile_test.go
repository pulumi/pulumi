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

//go:build linux || windows

package securestore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTPMFileStoreLifecycle(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "pulumi-home")

	store := tpmFileStore{path: filepath.Join(home, tpmFileName)}
	outcome, err := store.available()
	require.NoError(t, err)
	require.Equal(t, Ready, outcome, "a writable home dir must be available")

	_, err = store.get()
	assert.ErrorIs(t, err, ErrKeyNotFound)

	require.NoError(t, store.set("first-value"))
	got, err := store.get()
	require.NoError(t, err)
	assert.Equal(t, "first-value", got)

	path := filepath.Join(home, "credentials-key.tpm")
	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "key file must be private")
		dirInfo, err := os.Stat(home)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm(), "created home dir must be private")
	}

	require.NoError(t, store.set("second-value"))
	got, err = store.get()
	require.NoError(t, err)
	assert.Equal(t, "second-value", got)

	require.NoError(t, store.delete())
	require.NoError(t, store.delete())
	_, err = store.get()
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestTPMFileStoreAvailableCreatesDir(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), "deep", "nested", ".pulumi")

	store := tpmFileStore{path: filepath.Join(home, tpmFileName)}
	outcome, err := store.available()
	require.NoError(t, err)
	require.Equal(t, Ready, outcome)
	info, err := os.Stat(home)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// The writability probe must not leave files behind.
	entries, err := os.ReadDir(home)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

//nolint:paralleltest // t.Setenv forbids parallel runs
func TestTPMFileStoreResolvesUnderPulumiHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PULUMI_HOME", home)
	store, ok := newTPMFileStore().(tpmFileStore)
	require.True(t, ok)
	require.NoError(t, store.err)
	assert.Equal(t, filepath.Join(home, tpmFileName), store.path)
}

func TestTPMFileStoreUnavailableWithoutHomeDir(t *testing.T) {
	t.Parallel()
	store := tpmFileStore{err: errors.New("home dir went missing")}
	outcome, err := store.available()
	assert.Equal(t, Absent, outcome)
	assert.ErrorContains(t, err, "home dir went missing")
}
