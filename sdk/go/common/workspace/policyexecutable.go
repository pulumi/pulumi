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
	"maps"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// PolicyRuntimeExecutable is the policy pack runtime whose packs are pre-built per-platform
// binaries serving the analyzer gRPC protocol.
const PolicyRuntimeExecutable = "executable"

// PlatformLinuxAmd64 is mandatory for published executable packs: server-side policy
// evaluation runs on linux-amd64.
const PlatformLinuxAmd64 = "linux-amd64"

var validExecutablePlatforms = map[string]bool{
	"linux-amd64":   true,
	"linux-arm64":   true,
	"darwin-amd64":  true,
	"darwin-arm64":  true,
	"windows-amd64": true,
	"windows-arm64": true,
}

// CurrentPlatform returns the host platform in the "<os>-<arch>" form used to key executable
// policy pack binaries and artifacts.
func CurrentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// ExecutableBinaries returns the validated platform-to-binary-path map from an executable
// policy pack's runtime options. Paths are relative to the pack directory, in the platform's
// native separator form.
func (proj *PolicyPackProject) ExecutableBinaries() (map[string]string, error) {
	return ParseExecutableBinaries(proj.Runtime.Options())
}

// SelectPlatformBinary returns the binary path declared for the host platform, or an error
// naming the platforms the pack does support.
func SelectPlatformBinary(binaries map[string]string) (string, error) {
	platform := CurrentPlatform()
	binary, ok := binaries[platform]
	if !ok {
		return "", fmt.Errorf(
			"this policy pack does not provide a binary for %s; it supports: %s. "+
				"The pack must be republished with a %s binary to run on this machine",
			platform, strings.Join(sortedKeys(binaries), ", "), platform)
	}
	return binary, nil
}

// ParseExecutableBinaries validates the "binaries" runtime option of an executable policy pack
// and returns the platform-to-binary-path map. Paths are relative to the pack directory, in the
// platform's native separator form.
func ParseExecutableBinaries(options map[string]any) (map[string]string, error) {
	raw, has := options["binaries"]
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
				platform, strings.Join(sortedKeys(validExecutablePlatforms), ", "))
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

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
