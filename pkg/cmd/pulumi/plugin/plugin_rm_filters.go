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
	"sort"
	"strings"
	"time"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type pluginDeleteSelection struct {
	Plugin  workspace.PluginInfo
	Reasons []string
}

// recordedPluginLastUsed returns the sidecar-recorded last-used time for a
// plugin, or (zero, false) if no sidecar exists. Destructive age filtering
// uses the sidecar directly instead of PluginInfo.LastUsedTime, which falls
// back to filesystem atime — "no sidecar" must mean "unknown", not "very old".
func recordedPluginLastUsed(p workspace.PluginInfo) (time.Time, bool) {
	if p.Path == "" {
		return time.Time{}, false
	}
	return workspace.RecordedLastUsedTime(p.Path)
}

func selectPluginsToDelete(
	plugins []workspace.PluginInfo,
	kind apitype.PluginKind,
	name string,
	version *semver.Range,
	olderThan *time.Duration,
	keepLatest int,
	now time.Time,
) []pluginDeleteSelection {
	base := matchingPlugins(plugins, kind, name, version)
	protected := latestPluginIDs(base, keepLatest)

	var out []pluginDeleteSelection
	for _, p := range base {
		reasons := []string{}
		if keepLatest > 0 {
			if _, ok := protected[pluginID(p)]; ok {
				continue
			}
			reasons = append(reasons,
				fmt.Sprintf("outside latest %d version%s", keepLatest, plural(keepLatest)))
		}
		if olderThan != nil {
			lastUsed, ok := recordedPluginLastUsed(p)
			if !ok || !lastUsed.Before(now.Add(-*olderThan)) {
				continue
			}
			reasons = append(reasons, "last used "+lastUsed.UTC().Format(time.RFC3339))
		}
		out = append(out, pluginDeleteSelection{Plugin: p, Reasons: reasons})
	}
	return out
}

func matchingPlugins(
	plugins []workspace.PluginInfo,
	kind apitype.PluginKind,
	name string,
	version *semver.Range,
) []workspace.PluginInfo {
	out := make([]workspace.PluginInfo, 0, len(plugins))
	for _, p := range plugins {
		if (kind == "" || p.Kind == kind) &&
			(name == "" || p.Name == name) &&
			(version == nil || (p.Version != nil && (*version)(*p.Version))) {
			out = append(out, p)
		}
	}
	return out
}

func latestPluginIDs(in []workspace.PluginInfo, n int) map[string]struct{} {
	protected := map[string]struct{}{}
	if n <= 0 {
		return protected
	}

	type key struct {
		kind apitype.PluginKind
		name string
	}
	groups := map[key][]workspace.PluginInfo{}
	for _, p := range in {
		k := key{kind: p.Kind, name: p.Name}
		groups[k] = append(groups[k], p)
	}
	for _, g := range groups {
		sort.Sort(sort.Reverse(workspace.SortedPluginInfo(g)))
		for i := 0; i < len(g) && i < n; i++ {
			protected[pluginID(g[i])] = struct{}{}
		}
	}
	return protected
}

func pluginID(p workspace.PluginInfo) string {
	if p.Path != "" {
		return p.Path
	}
	version := ""
	if p.Version != nil {
		version = p.Version.String()
	}
	return strings.Join([]string{string(p.Kind), p.Name, version}, "\x00")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
