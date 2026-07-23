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

package plugin

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/djherbis/times"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/pluginstorage"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// fakePluginFS is an in-memory workspace.PluginFS. It reports lastUsed as the plugin's access time
// and records removals instead of touching the real filesystem, so plugin removal can be exercised
// without any on-disk state.
type fakePluginFS struct {
	lastUsed time.Time
	removed  bool
}

func (f *fakePluginFS) Stat(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }
func (f *fakePluginFS) Remove(string) error              { return nil }

func (f *fakePluginFS) RemoveAll(string) error {
	f.removed = true
	return nil
}

func (f *fakePluginFS) GetTimes(os.FileInfo) times.Timespec {
	return fakeTimespec{t: f.lastUsed}
}

// fakeTimespec is a times.Timespec that reports the same time for every clock.
type fakeTimespec struct{ t time.Time }

func (f fakeTimespec) ModTime() time.Time    { return f.t }
func (f fakeTimespec) AccessTime() time.Time { return f.t }
func (f fakeTimespec) ChangeTime() time.Time { return f.t }
func (f fakeTimespec) BirthTime() time.Time  { return f.t }
func (f fakeTimespec) HasChangeTime() bool   { return true }
func (f fakeTimespec) HasBirthTime() bool    { return true }

// fakeFileInfo is a stub os.FileInfo; the value is only ever handed back to fakePluginFS.GetTimes,
// which ignores it.
type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

// makeTestPlugin builds a PluginInfo backed by an in-memory fakePluginFS. The returned fake records
// whether the plugin was removed, which is how the tests assert on filtering behavior.
func makeTestPlugin(
	name string, kind apitype.PluginKind, version string, lastUsed time.Time,
) (workspace.PluginInfo, *fakePluginFS) {
	var v *semver.Version
	if version != "" {
		parsed := semver.MustParse(version)
		v = &parsed
	}

	fs := &fakePluginFS{lastUsed: lastUsed}
	info := workspace.PluginInfo{
		Name:    name,
		Path:    string(kind) + "-" + name + "-" + version,
		Kind:    kind,
		Version: v,
		FS:      fs,
	}
	return info, fs
}

// runRemove executes `pulumi plugin remove` against a mock plugin context that returns the given
// plugins. --yes is always passed so that the confirmation prompt (which reads os.Stdin and would
// block in an interactive terminal) is skipped; matching plugins are therefore "removed", which the
// caller asserts against via each plugin's fakePluginFS.
func runRemove(t *testing.T, plugins []workspace.PluginInfo, args ...string) (string, error) {
	t.Helper()

	ctx := pluginstorage.MockContext{
		GetPluginsF: func(_ context.Context) ([]workspace.PluginInfo, error) {
			return plugins, nil
		},
	}

	cmd := newPluginRemoveCmd(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--yes"}, args...))

	err := cmd.Execute()
	return out.String(), err
}

// The threshold used by the tests. Plugins whose last-used time is strictly before this are matched.
var (
	testThreshold = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	before        = testThreshold.AddDate(-1, 0, 0) // last used before the threshold: matches.
	after         = testThreshold.AddDate(1, 0, 0)  // last used after the threshold: does not match.
)

func TestPluginRemoveLastUsedBefore_NoMatches(t *testing.T) {
	t.Parallel()

	aws, awsFS := makeTestPlugin("aws", apitype.ResourcePlugin, "1.0.0", after)
	gcp, gcpFS := makeTestPlugin("gcp", apitype.ResourcePlugin, "2.0.0", after)

	_, err := runRemove(t, []workspace.PluginInfo{aws, gcp},
		"--all", "--last-used-before", testThreshold.Format(time.RFC3339))
	require.NoError(t, err)

	// Nothing was used before the threshold, so nothing should have been removed.
	require.False(t, awsFS.removed)
	require.False(t, gcpFS.removed)
}

func TestPluginRemoveLastUsedBefore_SomeMatch(t *testing.T) {
	t.Parallel()

	old, oldFS := makeTestPlugin("aws", apitype.ResourcePlugin, "1.0.0", before)
	recent, recentFS := makeTestPlugin("gcp", apitype.ResourcePlugin, "2.0.0", after)

	out, err := runRemove(t, []workspace.PluginInfo{old, recent},
		"--all", "--last-used-before", testThreshold.Format(time.RFC3339))
	require.NoError(t, err)

	// Only the plugin last used before the threshold should have been removed.
	require.True(t, oldFS.removed)
	require.False(t, recentFS.removed)
	require.Contains(t, out, "removed: resource aws-1.0.0")
	require.NotContains(t, out, "gcp")
}

func TestPluginRemoveLastUsedBefore_AllMatch(t *testing.T) {
	t.Parallel()

	aws, awsFS := makeTestPlugin("aws", apitype.ResourcePlugin, "1.0.0", before)
	gcp, gcpFS := makeTestPlugin("gcp", apitype.ResourcePlugin, "2.0.0", before)

	_, err := runRemove(t, []workspace.PluginInfo{aws, gcp},
		"--all", "--last-used-before", testThreshold.Format(time.RFC3339))
	require.NoError(t, err)

	// Both plugins were used before the threshold, so both should have been removed.
	require.True(t, awsFS.removed)
	require.True(t, gcpFS.removed)
}

func TestPluginRemoveLastUsedBefore_InvalidDateTime(t *testing.T) {
	t.Parallel()

	aws, awsFS := makeTestPlugin("aws", apitype.ResourcePlugin, "1.0.0", before)

	_, err := runRemove(t, []workspace.PluginInfo{aws},
		"--all", "--last-used-before", "not-a-real-date")
	require.Error(t, err)
	require.ErrorContains(t, err, "last-used-before")

	// A parse failure must not remove anything.
	require.False(t, awsFS.removed)
}

// TestPluginRemoveLastUsedBefore_WithFilters checks that --last-used-before composes with the
// kind, name, and version filters: a plugin is only removed when it matches all of them.
func TestPluginRemoveLastUsedBefore_WithFilters(t *testing.T) {
	t.Parallel()

	t.Run("kind", func(t *testing.T) {
		t.Parallel()

		// Both used before the threshold, but only the resource plugin should be removed.
		resource, resourceFS := makeTestPlugin("aws", apitype.ResourcePlugin, "1.0.0", before)
		language, languageFS := makeTestPlugin("nodejs", apitype.LanguagePlugin, "1.0.0", before)

		_, err := runRemove(t, []workspace.PluginInfo{resource, language},
			"resource", "--last-used-before", testThreshold.Format(time.RFC3339))
		require.NoError(t, err)

		require.True(t, resourceFS.removed)
		require.False(t, languageFS.removed)
	})

	t.Run("name", func(t *testing.T) {
		t.Parallel()

		aws, awsFS := makeTestPlugin("aws", apitype.ResourcePlugin, "1.0.0", before)
		gcp, gcpFS := makeTestPlugin("gcp", apitype.ResourcePlugin, "1.0.0", before)

		_, err := runRemove(t, []workspace.PluginInfo{aws, gcp},
			"resource", "aws", "--last-used-before", testThreshold.Format(time.RFC3339))
		require.NoError(t, err)

		require.True(t, awsFS.removed)
		require.False(t, gcpFS.removed)
	})

	t.Run("version", func(t *testing.T) {
		t.Parallel()

		v1, v1FS := makeTestPlugin("aws", apitype.ResourcePlugin, "1.0.0", before)
		v2, v2FS := makeTestPlugin("aws", apitype.ResourcePlugin, "2.0.0", before)

		_, err := runRemove(t, []workspace.PluginInfo{v1, v2},
			"resource", "aws", "1.0.0", "--last-used-before", testThreshold.Format(time.RFC3339))
		require.NoError(t, err)

		require.True(t, v1FS.removed)
		require.False(t, v2FS.removed)
	})

	t.Run("filter matches but last-used does not", func(t *testing.T) {
		t.Parallel()

		// Matches the kind/name/version filter, but was used after the threshold, so it stays.
		aws, awsFS := makeTestPlugin("aws", apitype.ResourcePlugin, "1.0.0", after)

		_, err := runRemove(t, []workspace.PluginInfo{aws},
			"resource", "aws", "1.0.0", "--last-used-before", testThreshold.Format(time.RFC3339))
		require.NoError(t, err)

		require.False(t, awsFS.removed)
	})
}
