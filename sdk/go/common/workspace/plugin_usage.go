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

package workspace

import (
	"os"
	"time"
)

const lastUsedSidecarSuffix = ".lastused"

// LastUsedSidecarPath returns the path of the last-used sidecar that pairs
// with the given plugin directory.
func LastUsedSidecarPath(pluginDir string) string {
	return pluginDir + lastUsedSidecarSuffix
}

// recordPluginUsage writes an RFC3339 timestamp sidecar next to a plugin
// directory. Best-effort: the file's mtime, not its contents, is what
// RecordedLastUsedTime reads. Callers should ignore the returned error in
// normal operation.
//
// Concurrent writes from multiple pulumi processes are safe: a torn body is
// harmless because readers consult mtime, not contents, and os.WriteFile sets
// mtime atomically on the truncate-then-write path used here.
func recordPluginUsage(pluginDir string) error {
	path := LastUsedSidecarPath(pluginDir)
	body := time.Now().UTC().Format(time.RFC3339) + "\n"
	return os.WriteFile(path, []byte(body), 0o600)
}

// RecordedLastUsedTime returns the recorded last-used time for the plugin at
// pluginDir, or (zero, false) if no sidecar exists or cannot be stat'd.
//
// This is distinct from PluginInfo.LastUsedTime, which falls back to
// filesystem access time when no sidecar exists. Callers that need a reliable
// age signal must use sidecar-only semantics and treat "no sidecar" as
// "unknown age", not "very old".
func RecordedLastUsedTime(pluginDir string) (time.Time, bool) {
	info, err := os.Stat(LastUsedSidecarPath(pluginDir))
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}
