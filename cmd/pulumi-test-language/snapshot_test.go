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

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyDirectory(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello, world"), 0o600)
	require.NoError(t, err)

	dst := t.TempDir()

	err = copyDirectory(os.DirFS(src), ".", dst, nil, nil)
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello, world", string(b))
}

func TestCopyDirectoryWithEdit(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	err := os.WriteFile(filepath.Join(src, "file1.txt"), []byte("hello, world"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(src, "file2.txt"), []byte("goodbye, world"), 0o600)
	require.NoError(t, err)

	dst := t.TempDir()

	edits := []compiledReplacement{
		{regexp.MustCompile("file2.txt"), regexp.MustCompile("goodbye"), "hello"},
	}

	err = copyDirectory(os.DirFS(src), ".", dst, edits, nil)
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(dst, "file1.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello, world", string(b))

	b, err = os.ReadFile(filepath.Join(dst, "file2.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello, world", string(b))
}

func TestCopyDirectoryWithMultilineEdit(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	err := os.WriteFile(filepath.Join(src, "file1.txt"), []byte("hello, world"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(src, "file2.txt"), []byte("goodbye, world\nI said goodbye\n"), 0o600)
	require.NoError(t, err)

	dst := t.TempDir()

	edits := []compiledReplacement{
		{regexp.MustCompile("file2.txt"), regexp.MustCompile("goodbye"), "hello"},
	}

	err = copyDirectory(os.DirFS(src), ".", dst, edits, nil)
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(dst, "file1.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello, world", string(b))

	b, err = os.ReadFile(filepath.Join(dst, "file2.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello, world\nI said hello\n", string(b))
}
