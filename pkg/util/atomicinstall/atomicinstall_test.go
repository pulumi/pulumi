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

package atomicinstall_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/util/atomicinstall"
)

// The cross-process tests run this test binary again as a helper process. helperEnv selects what the helper does.
const (
	helperEnv     = "ATOMICINSTALL_TEST_HELPER"
	helperRootEnv = "ATOMICINSTALL_TEST_ROOT"
	helperLogEnv  = "ATOMICINSTALL_TEST_LOG"

	helperKey = "entry"
)

func TestMain(m *testing.M) {
	switch os.Getenv(helperEnv) {
	case "":
		os.Exit(m.Run())
	case "install":
		os.Exit(helperInstall())
	case "crash":
		os.Exit(helperCrash())
	default:
		fmt.Fprintf(os.Stderr, "unknown helper %q\n", os.Getenv(helperEnv))
		os.Exit(2)
	}
}

// helperInstall installs helperKey. Each call to populate leaves one file in the log directory.
func helperInstall() int {
	store := atomicinstall.NewStore(os.Getenv(helperRootEnv))
	_, err := store.Install(context.Background(), helperKey, func(_ context.Context, dir string) error {
		f, err := os.CreateTemp(os.Getenv(helperLogEnv), "populate")
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		// Hold the lock long enough that the other helpers wait on it.
		time.Sleep(200 * time.Millisecond)
		return os.WriteFile(filepath.Join(dir, "content"), []byte("complete"), 0o600)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// helperCrash dies in the middle of an install, without Close.
func helperCrash() int {
	store := atomicinstall.NewStore(os.Getenv(helperRootEnv))
	p, err := store.Mutate(context.Background(), helperKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dir, err := p.Reset()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(dir, "half-written"), nil, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func helperCommand(t *testing.T, helper, root, log string) *exec.Cmd {
	exe, err := os.Executable()
	require.NoError(t, err)
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), helperEnv+"="+helper, helperRootEnv+"="+root, helperLogEnv+"="+log)
	return cmd
}

func fileNames(t *testing.T, dir string) []string {
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name())
	}
	return names
}

func writeContent(content string) atomicinstall.PopulateFunc {
	return func(_ context.Context, dir string) error {
		return os.WriteFile(filepath.Join(dir, "content"), []byte(content), 0o600)
	}
}

func TestConcurrentProcessesPopulateOnce(t *testing.T) {
	t.Parallel()
	root, log := t.TempDir(), t.TempDir()

	var wg sync.WaitGroup
	outputs := make([]string, 4)
	for i := range outputs {
		cmd := helperCommand(t, "install", root, log)
		wg.Go(func() {
			out, err := cmd.CombinedOutput()
			if err != nil {
				outputs[i] = fmt.Sprintf("%v: %s", err, out)
			}
		})
	}
	wg.Wait()

	assert.Equal(t, []string{"", "", "", ""}, outputs)
	require.Len(t, fileNames(t, log), 1, "exactly one process populates the entry")

	dir, ok, err := atomicinstall.NewStore(root).Lookup(helperKey)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, []string{"content"}, fileNames(t, dir))
}

func TestCrashedInstallIsDiscarded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := atomicinstall.NewStore(root)

	out, err := helperCommand(t, "crash", root, "").CombinedOutput()
	require.NoError(t, err, string(out))
	require.Equal(t, []string{"entry", "entry.lock", "entry.partial"}, fileNames(t, root))

	_, ok, err := store.Lookup(helperKey)
	require.NoError(t, err)
	assert.False(t, ok)
	keys, err := store.List()
	require.NoError(t, err)
	assert.Empty(t, keys)

	dir, err := store.Install(t.Context(), helperKey, writeContent("complete"))
	require.NoError(t, err)
	assert.Equal(t, []string{"content"}, fileNames(t, dir))
	assert.Equal(t, []string{"entry", "entry.lock"}, fileNames(t, root))
}

func TestConcurrentInstallPopulatesOnce(t *testing.T) {
	t.Parallel()
	store := atomicinstall.NewStore(t.TempDir())

	var populated atomic.Int32
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = store.Install(t.Context(), "k", func(ctx context.Context, dir string) error {
				populated.Add(1)
				return writeContent("complete")(ctx, dir)
			})
		})
	}
	wg.Wait()

	assert.Equal(t, make([]error, 8), errs)
	assert.Equal(t, int32(1), populated.Load())
}

func TestFailedPopulateLeavesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := atomicinstall.NewStore(root)

	failure := errors.New("dependency install failed")
	_, err := store.Install(t.Context(), "k", func(ctx context.Context, dir string) error {
		require.NoError(t, writeContent("half")(ctx, dir))
		return failure
	})
	assert.Equal(t, failure, err)

	_, ok, err := store.Lookup("k")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, []string{"k.lock"}, fileNames(t, root))

	dir, err := store.Install(t.Context(), "k", writeContent("complete"))
	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(dir, "content"))
	require.NoError(t, err)
	assert.Equal(t, "complete", string(content))
}

func TestReinstallReplacesContent(t *testing.T) {
	t.Parallel()
	store := atomicinstall.NewStore(t.TempDir())

	_, err := store.Install(t.Context(), "k", func(_ context.Context, dir string) error {
		return os.WriteFile(filepath.Join(dir, "old"), nil, 0o600)
	})
	require.NoError(t, err)

	dir, err := store.Reinstall(t.Context(), "k", writeContent("new"))
	require.NoError(t, err)
	assert.Equal(t, []string{"content"}, fileNames(t, dir))
}

func TestRemoveKeepsLockFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := atomicinstall.NewStore(root)

	_, err := store.Install(t.Context(), "k", writeContent("complete"))
	require.NoError(t, err)

	require.NoError(t, store.Remove(t.Context(), "k"))
	_, ok, err := store.Lookup("k")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, []string{"k.lock"}, fileNames(t, root))

	require.NoError(t, store.Remove(t.Context(), "k"))
}

func TestListSkipsEntriesThatAreNotInstalled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := atomicinstall.NewStore(root)

	for _, key := range []string{"b", "a"} {
		_, err := store.Install(t.Context(), key, writeContent("complete"))
		require.NoError(t, err)
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "stray-file"), nil, 0o600))

	p, err := store.Mutate(t.Context(), "in-progress")
	require.NoError(t, err)
	_, err = p.Reset()
	require.NoError(t, err)

	keys, err := store.List()
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, keys)

	require.NoError(t, p.Close())
}

func TestMutateWaitsForLock(t *testing.T) {
	t.Parallel()
	store := atomicinstall.NewStore(t.TempDir())

	holder, err := store.Mutate(t.Context(), "k")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err = store.Mutate(ctx, "k")
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// The lock is independent for each key.
	other, err := store.Mutate(t.Context(), "other")
	require.NoError(t, err)
	require.NoError(t, other.Close())

	// The canceled Mutate must not keep the lock after the holder releases it.
	require.NoError(t, holder.Close())
	ctx, cancel = context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	next, err := store.Mutate(ctx, "k")
	require.NoError(t, err)
	require.NoError(t, next.Close())
}

func TestPendingRejectsUseOutOfOrder(t *testing.T) {
	t.Parallel()
	store := atomicinstall.NewStore(t.TempDir())

	p, err := store.Mutate(t.Context(), "k")
	require.NoError(t, err)
	_, ok := p.Installed()
	assert.False(t, ok)
	assert.EqualError(t, p.Commit(), "atomicinstall: Commit without Reset")

	dir, err := p.Reset()
	require.NoError(t, err)
	_, ok = p.Installed()
	assert.False(t, ok)

	require.NoError(t, p.Commit())
	installed, ok := p.Installed()
	assert.True(t, ok)
	assert.Equal(t, dir, installed)

	require.NoError(t, p.Close())
	_, err = p.Reset()
	assert.EqualError(t, err, "atomicinstall: Reset of a closed Pending")
	assert.EqualError(t, p.Commit(), "atomicinstall: Commit of a closed Pending")

	// Close after Commit must not discard the entry.
	_, ok, err = store.Lookup("k")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestInvalidKeys(t *testing.T) {
	t.Parallel()
	store := atomicinstall.NewStore(t.TempDir())

	onePathElement := func(key string) string {
		return fmt.Sprintf("atomicinstall: invalid key %q: a key is one path element", key)
	}
	reservedSuffix := func(key string) string {
		return fmt.Sprintf(`atomicinstall: invalid key %q: a key cannot end in ".lock" or ".partial"`, key)
	}
	for key, expected := range map[string]string{
		"":          onePathElement(""),
		".":         onePathElement("."),
		"..":        onePathElement(".."),
		"a/b":       onePathElement("a/b"),
		`a\b`:       onePathElement(`a\b`),
		"/abs":      onePathElement("/abs"),
		"k.lock":    reservedSuffix("k.lock"),
		"k.partial": reservedSuffix("k.partial"),
	} {
		_, _, err := store.Lookup(key)
		assert.EqualError(t, err, expected)
		_, err = store.Mutate(t.Context(), key)
		assert.EqualError(t, err, expected)
	}
}
