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
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const PolicyRuntimeExecutable = "executable"

const PlatformLinuxAmd64 = "linux-amd64"

var validExecutablePlatforms = map[string]bool{
	"linux-amd64":   true,
	"linux-arm64":   true,
	"darwin-amd64":  true,
	"darwin-arm64":  true,
	"windows-amd64": true,
	"windows-arm64": true,
}

func CurrentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func (proj *PolicyPackProject) ExecutableBinaries() (map[string]string, error) {
	raw, has := proj.Runtime.Options()["binaries"]
	if !has {
		return nil, errors.New(
			"executable policy packs require a 'binaries' map of platform to binary path in the runtime options")
	}
	m, ok := raw.(map[string]any)
	if !ok || len(m) == 0 {
		return nil, errors.New(
			"the 'binaries' runtime option must be a non-empty map of platform to binary path")
	}

	binaries := make(map[string]string, len(m))
	for platform, v := range m {
		if !validExecutablePlatforms[platform] {
			return nil, fmt.Errorf("unknown platform %q in 'binaries'; valid platforms are: %s",
				platform, strings.Join(sortedPlatforms(validExecutablePlatforms), ", "))
		}
		path, ok := v.(string)
		if !ok || path == "" {
			return nil, fmt.Errorf("binary path for platform %q must be a non-empty string", platform)
		}
		if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
			return nil, fmt.Errorf("binary path for platform %q must be relative to the policy pack directory", platform)
		}
		clean := filepath.Clean(filepath.FromSlash(path))
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("binary path for platform %q must not escape the policy pack directory", platform)
		}
		binaries[platform] = clean
	}
	return binaries, nil
}

func sortedPlatforms(set map[string]bool) []string {
	platforms := make([]string, 0, len(set))
	for p := range set {
		platforms = append(platforms, p)
	}
	slices.Sort(platforms)
	return platforms
}
