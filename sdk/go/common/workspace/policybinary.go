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
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// AnalyzerBinaryPrefix is the filename prefix of a policy pack's pre-built analyzer
// binary. A binary policy pack ships a single executable "pulumi-analyzer-<name>"
// (".exe" on Windows) that the engine execs directly, like a provider plugin — no
// manifest and no language host involved.
const AnalyzerBinaryPrefix = "pulumi-analyzer-"

// AnalyzerBinaryName returns the canonical filename of a policy pack's analyzer binary
// on the given platform, e.g. "pulumi-analyzer-mypack" ("pulumi-analyzer-mypack.exe" on
// Windows).
func AnalyzerBinaryName(name, platform string) string {
	binName := AnalyzerBinaryPrefix + name
	if strings.HasPrefix(platform, "windows-") {
		binName += ".exe"
	}
	return binName
}

// ValidPolicyBinaryPlatforms are the "<os>-<arch>" platforms a policy pack may publish
// pre-built analyzer binaries for.
var ValidPolicyBinaryPlatforms = map[string]bool{
	"linux-amd64":   true,
	"linux-arm64":   true,
	"darwin-amd64":  true,
	"darwin-arm64":  true,
	"windows-amd64": true,
	"windows-arm64": true,
}

// CurrentPlatform returns the host platform in the "<os>-<arch>" form used to key
// policy pack analyzer binaries.
func CurrentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// FindAnalyzerBinary returns a pre-built analyzer binary in dir runnable on the current
// platform, if one is present. Binary policy packs ship a single "pulumi-analyzer-<name>"
// executable ("pulumi-analyzer-<name>.exe" on Windows) at the pack root, exec'd directly
// like a provider plugin — no manifest read required. A directory with no such binary (a
// source checkout, or a language pack) returns false so the pack runs through its
// language runtime instead.
func FindAnalyzerBinary(dir string) (string, bool) {
	matches, err := filepath.Glob(filepath.Join(dir, AnalyzerBinaryPrefix+"*"))
	if err != nil {
		return "", false
	}
	sort.Strings(matches)
	windows := strings.HasPrefix(CurrentPlatform(), "windows-")
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || info.IsDir() {
			continue
		}
		// Only the current platform's binary is runnable: require the ".exe" suffix on
		// Windows and reject it elsewhere.
		if windows != strings.EqualFold(filepath.Ext(m), ".exe") {
			continue
		}
		return m, true
	}
	return "", false
}
