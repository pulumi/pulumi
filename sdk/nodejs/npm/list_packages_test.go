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

package npm

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPackages(t *testing.T) {
	t.Parallel()

	t.Run("npm", func(t *testing.T) {
		t.Parallel()
		dir := "testdata/list-packages/npm"
		npm, err := newNPM()
		require.NoError(t, err)

		t.Run("transitive", func(t *testing.T) {
			t.Parallel()
			got, err := npm.ListPackages(t.Context(), dir, true)
			require.NoError(t, err)
			assert.Greater(t, len(got), 2)
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@pulumi/pulumi", Version: "3.224.0"})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "typescript", Version: "5.9.3"})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@grpc/grpc-js", Version: "1.14.3"})
		})

		t.Run("direct", func(t *testing.T) {
			t.Parallel()
			got, err := npm.ListPackages(t.Context(), dir, false)
			require.NoError(t, err)
			assert.ElementsMatch(t, []plugin.DependencyInfo{
				{Name: "@pulumi/pulumi", Version: "3.224.0"},
				{Name: "typescript", Version: "5.9.3"},
			}, got)
		})
	})

	t.Run("yarn", func(t *testing.T) {
		t.Parallel()
		dir := "testdata/list-packages/yarn"
		yarn, err := newYarnClassic()
		require.NoError(t, err)

		t.Run("transitive", func(t *testing.T) {
			t.Parallel()
			got, err := yarn.ListPackages(t.Context(), dir, true)
			require.NoError(t, err)
			assert.Greater(t, len(got), 2)
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@pulumi/pulumi", Version: "3.224.0"})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "typescript", Version: "5.9.3"})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@grpc/grpc-js", Version: "1.14.3"})
		})

		t.Run("direct", func(t *testing.T) {
			t.Parallel()
			got, err := yarn.ListPackages(t.Context(), dir, false)
			require.NoError(t, err)
			assert.ElementsMatch(t, []plugin.DependencyInfo{
				{Name: "@pulumi/pulumi", Version: "3.224.0"},
				{Name: "typescript", Version: "5.9.3"},
			}, got)
		})
	})

	t.Run("pnpm", func(t *testing.T) {
		t.Parallel()
		dir := "testdata/list-packages/pnpm"
		pnpm, err := newPnpm()
		require.NoError(t, err)

		// pnpm encodes peer dependencies in the version string, e.g. "3.224.0_typescript@5.9.3".
		const pulumiVersion = "3.224.0_typescript@5.9.3"

		t.Run("transitive", func(t *testing.T) {
			t.Parallel()
			got, err := pnpm.ListPackages(t.Context(), dir, true)
			require.NoError(t, err)
			assert.Greater(t, len(got), 2)
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@pulumi/pulumi", Version: pulumiVersion})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "typescript", Version: "5.9.3"})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@grpc/grpc-js", Version: "1.14.3"})
		})

		t.Run("direct", func(t *testing.T) {
			t.Parallel()
			got, err := pnpm.ListPackages(t.Context(), dir, false)
			require.NoError(t, err)
			assert.ElementsMatch(t, []plugin.DependencyInfo{
				{Name: "@pulumi/pulumi", Version: pulumiVersion},
				{Name: "typescript", Version: "5.9.3"},
			}, got)
		})
	})

	t.Run("pnpm-v9", func(t *testing.T) {
		t.Parallel()
		dir := "testdata/list-packages/pnpm-v9"
		pnpm, err := newPnpm()
		require.NoError(t, err)

		t.Run("transitive", func(t *testing.T) {
			t.Parallel()
			got, err := pnpm.ListPackages(t.Context(), dir, true)
			require.NoError(t, err)
			assert.Greater(t, len(got), 2)
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@pulumi/pulumi", Version: "3.224.0"})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "typescript", Version: "5.9.3"})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@grpc/grpc-js", Version: "1.14.3"})
		})

		t.Run("direct", func(t *testing.T) {
			t.Parallel()
			got, err := pnpm.ListPackages(t.Context(), dir, false)
			require.NoError(t, err)
			assert.ElementsMatch(t, []plugin.DependencyInfo{
				{Name: "@pulumi/pulumi", Version: "3.224.0"},
				{Name: "typescript", Version: "5.9.3"},
			}, got)
		})
	})

	t.Run("bun", func(t *testing.T) {
		t.Parallel()
		dir := "testdata/list-packages/bun"
		bun, err := newBun()
		require.NoError(t, err)

		t.Run("transitive", func(t *testing.T) {
			t.Parallel()
			got, err := bun.ListPackages(t.Context(), dir, true)
			require.NoError(t, err)
			assert.Greater(t, len(got), 2)
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@pulumi/pulumi", Version: "3.224.0"})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "typescript", Version: "5.9.3"})
			assert.Contains(t, got, plugin.DependencyInfo{Name: "@grpc/grpc-js", Version: "1.14.3"})
		})

		t.Run("direct", func(t *testing.T) {
			t.Parallel()
			got, err := bun.ListPackages(t.Context(), dir, false)
			require.NoError(t, err)
			assert.ElementsMatch(t, []plugin.DependencyInfo{
				{Name: "@pulumi/pulumi", Version: "3.224.0"},
				{Name: "typescript", Version: "5.9.3"},
			}, got)
		})
	})
}

func TestListPackagesBunLockb(t *testing.T) {
	t.Parallel()

	bun, err := newBun()
	require.NoError(t, err)

	dir := t.TempDir()
	writeFile(t, dir+"/bun.lockb", "")

	_, err = bun.ListPackages(t.Context(), dir, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bun.lockb")
	assert.Contains(t, err.Error(), "upgrade to bun >= 1.2")
}
