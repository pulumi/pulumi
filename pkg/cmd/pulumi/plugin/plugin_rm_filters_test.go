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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func mkPlugin(t *testing.T, name, version string, lastUsed *time.Time) workspace.PluginInfo {
	t.Helper()

	v := semver.MustParse(version)
	dir := filepath.Join(t.TempDir(), fmt.Sprintf("resource-%s-v%s", name, version))
	require.NoError(t, os.MkdirAll(dir, 0o755))

	if lastUsed != nil {
		sidecar := workspace.LastUsedSidecarPath(dir)
		body := lastUsed.UTC().Format(time.RFC3339) + "\n"
		require.NoError(t, os.WriteFile(sidecar, []byte(body), 0o600))
		require.NoError(t, os.Chtimes(sidecar, *lastUsed, *lastUsed))
	}

	return workspace.PluginInfo{Name: name, Kind: apitype.ResourcePlugin, Version: &v, Path: dir}
}

func mkPluginKind(
	t *testing.T,
	kind apitype.PluginKind,
	name, version string,
	lastUsed *time.Time,
) workspace.PluginInfo {
	t.Helper()

	p := mkPlugin(t, name, version, lastUsed)
	p.Kind = kind
	return p
}

func selectedVersions(selections []pluginDeleteSelection) []string {
	versions := make([]string, 0, len(selections))
	for _, s := range selections {
		versions = append(versions, s.Plugin.Name+"@"+s.Plugin.Version.String())
	}
	return versions
}

func TestSelectPluginsToDeleteOlderThan_DropsRecent(t *testing.T) {
	t.Parallel()

	now := time.Now()
	recent := now.Add(-10 * 24 * time.Hour)
	old := now.Add(-90 * 24 * time.Hour)
	threshold := 30 * 24 * time.Hour

	in := []workspace.PluginInfo{
		mkPlugin(t, "aws", "7.30.0", &recent),
		mkPlugin(t, "aws", "7.29.0", &old),
	}
	got := selectPluginsToDelete(in, "", "", nil, &threshold, 0, now)
	require.Len(t, got, 1)
	assert.Equal(t, "7.29.0", got[0].Plugin.Version.String())
	assert.Contains(t, got[0].Reasons[0], "last used")
}

func TestSelectPluginsToDeleteOlderThan_SkipsMissingSidecar(t *testing.T) {
	t.Parallel()

	threshold := 30 * 24 * time.Hour
	in := []workspace.PluginInfo{mkPlugin(t, "aws", "7.30.0", nil)}
	got := selectPluginsToDelete(in, "", "", nil, &threshold, 0, time.Now())
	assert.Empty(t, got, "plugins without a sidecar must be skipped")
}

func TestSelectPluginsToDeleteKeepLatest_ProtectsNewestAcrossWholeGroup(t *testing.T) {
	t.Parallel()

	now := time.Now()
	recent := now.Add(-10 * 24 * time.Hour)
	old := now.Add(-90 * 24 * time.Hour)
	threshold := 30 * 24 * time.Hour

	in := []workspace.PluginInfo{
		mkPlugin(t, "aws", "7.30.0", &recent),
		mkPlugin(t, "aws", "7.29.0", &old),
		mkPlugin(t, "aws", "7.28.0", &old),
		mkPlugin(t, "gcp", "8.0.0", &old),
	}
	got := selectPluginsToDelete(in, "", "", nil, &threshold, 2, now)

	assert.ElementsMatch(t, []string{"aws@7.28.0"}, selectedVersions(got),
		"keep-latest should protect aws@7.30.0 and aws@7.29.0 before age filtering deletes older candidates")
}

func TestSelectPluginsToDeleteKeepLatestOnly(t *testing.T) {
	t.Parallel()

	now := time.Now()
	in := []workspace.PluginInfo{
		mkPlugin(t, "aws", "7.30.0", &now),
		mkPlugin(t, "aws", "7.29.0", &now),
		mkPlugin(t, "aws", "7.28.0", &now),
	}

	got := selectPluginsToDelete(in, "", "", nil, nil, 1, now)
	assert.ElementsMatch(t, []string{"aws@7.29.0", "aws@7.28.0"}, selectedVersions(got))
}

func TestSelectPluginsToDeleteKeepLatest_GroupsByKindName(t *testing.T) {
	t.Parallel()

	now := time.Now()

	in := []workspace.PluginInfo{
		mkPlugin(t, "aws", "7.30.0", &now),
		mkPlugin(t, "aws", "7.29.0", &now),
		mkPlugin(t, "aws", "7.28.0", &now),

		mkPlugin(t, "gcp", "8.2.0", &now),
		mkPlugin(t, "gcp", "8.1.0", &now),
		mkPlugin(t, "gcp", "8.0.0", &now),

		mkPluginKind(t, apitype.LanguagePlugin, "nodejs", "3.2.0", &now),
		mkPluginKind(t, apitype.LanguagePlugin, "nodejs", "3.1.0", &now),
		mkPluginKind(t, apitype.LanguagePlugin, "nodejs", "3.0.0", &now),
	}

	got := selectPluginsToDelete(in, "", "", nil, nil, 2, now)
	assert.ElementsMatch(t,
		[]string{"aws@7.28.0", "gcp@8.0.0", "nodejs@3.0.0"},
		selectedVersions(got),
		"keep-latest must protect 2 newest per (kind, name) — including across different plugin kinds")
}
