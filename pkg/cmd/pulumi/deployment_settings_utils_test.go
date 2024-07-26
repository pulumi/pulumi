// Copyright 2016-2024, Pulumi Corporation.
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
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoLookup(t *testing.T) {
	t.Parallel()
	t.Run("should handle directories that are not a git repo", func(t *testing.T) {
		t.Parallel()
		wd := "/"

		rl, err := newRepoLookup(wd)
		assert.NoError(t, err)
		assert.IsType(t, &noRepoLookupImpl{}, rl)

		dir, err := rl.GetRootDirectory(wd)
		assert.NoError(t, err)
		assert.Equal(t, ".", dir)

		branch := rl.GetBranchName()
		assert.Equal(t, "", branch)

		remote, err := rl.RemoteURL()
		assert.NoError(t, err)
		assert.Equal(t, "", remote)

		root := rl.GetRepoRoot()
		assert.Equal(t, "", root)
	})

	t.Run("should handle directories that are a git repo", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		repo := auto.GitRepo{
			URL:         "https://github.com/pulumi/test-repo.git",
			ProjectPath: "goproj",
			Shallow:     true,
			Branch:      "master",
		}
		ws, err := auto.NewLocalWorkspace(ctx, auto.Repo(repo))
		require.NoError(t, err)

		rl, err := newRepoLookup(ws.WorkDir())
		assert.NoError(t, err)
		assert.IsType(t, &repoLookupImpl{}, rl)

		dir, err := rl.GetRootDirectory(filepath.Join(ws.WorkDir(), "something"))
		assert.NoError(t, err)
		// should assure the directory is using linux path separator as deployments are
		// currently run only on linux images.
		assert.Equal(t, "goproj/something", dir)

		branch := rl.GetBranchName()
		assert.Equal(t, "refs/heads/master", branch)

		remote, err := rl.RemoteURL()
		assert.NoError(t, err)
		assert.Equal(t, "https://github.com/pulumi/test-repo.git", remote)

		assert.Equal(t, filepath.Dir(ws.WorkDir()), rl.GetRepoRoot())
	})
}

type relativeDirectoryValidationCase struct {
	Valid bool
	Path  string
}

func TestValidateRelativeDirectory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	repo := auto.GitRepo{
		URL:         "https://github.com/pulumi/test-repo.git",
		ProjectPath: "goproj",
		Shallow:     true,
		Branch:      "master",
	}
	ws, err := auto.NewLocalWorkspace(ctx, auto.Repo(repo))
	require.NoError(t, err)

	// relative directory values are always linux type paths
	pathsToTest := []relativeDirectoryValidationCase{
		{true, "./goproj"},
		{false, "./goproj/child"},
		{false, "./goproj/Pulumi.yaml"},
	}

	for _, c := range pathsToTest {
		err = ValidateRelativeDirectory(filepath.Dir(ws.WorkDir()))(c.Path)
		if c.Valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err, "invalid relative path %s", c.Path)
		}
	}
}

func TestValidateGitURL(t *testing.T) {
	t.Parallel()

	err := ValidateGitURL("https://github.com/pulumi/test-repo.git")
	require.NoError(t, err)

	err = ValidateGitURL("https://something.com")
	require.Error(t, err, "invalid Git URL")
}

func TestValidateShortInput(t *testing.T) {
	t.Parallel()

	err := ValidateShortInput("")
	require.NoError(t, err)

	err = ValidateShortInput("a")
	require.NoError(t, err)

	err = ValidateShortInput(strings.Repeat("a", 256))
	require.NoError(t, err)

	err = ValidateShortInput(strings.Repeat("a", 257))
	require.Error(t, err, "must be 256 characters or less")
}

func TestValidateShortInputNonEmpty(t *testing.T) {
	t.Parallel()

	err := ValidateShortInputNonEmpty("")
	require.Error(t, err, "should not be empty")

	err = ValidateShortInputNonEmpty("a")
	require.NoError(t, err)

	err = ValidateShortInputNonEmpty(strings.Repeat("a", 256))
	require.NoError(t, err)

	err = ValidateShortInputNonEmpty(strings.Repeat("a", 257))
	require.Error(t, err, "must be 256 characters or less")
}
