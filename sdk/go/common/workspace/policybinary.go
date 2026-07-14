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
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// PlatformLinuxAmd64 is mandatory for binary-published policy packs: server-side
// policy evaluation runs on linux-amd64.
const PlatformLinuxAmd64 = "linux-amd64"

const policyBinaryPrefix = "pulumi-analyzer-"

var validPolicyBinaryPlatforms = map[string]bool{
	"linux-amd64":   true,
	"linux-arm64":   true,
	"darwin-amd64":  true,
	"darwin-arm64":  true,
	"windows-amd64": true,
	"windows-arm64": true,
}

// CurrentPlatform returns the host platform in the "<os>-<arch>" form used to key
// policy pack binary artifacts.
func CurrentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// DiscoverPolicyBinaries scans a policy pack's bin/ directory for binaries built to
// the pulumi-analyzer-<name>-<os>-<arch>[.exe] convention and returns platform to
// pack-relative path. It returns an empty map when the pack has no binaries.
func DiscoverPolicyBinaries(packDir string) (map[string]string, error) {
	entries, err := os.ReadDir(filepath.Join(packDir, "bin"))
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	binaries := map[string]string{}
	names := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), policyBinaryPrefix) {
			continue
		}
		stem := strings.TrimSuffix(strings.TrimPrefix(e.Name(), policyBinaryPrefix), ".exe")
		parts := strings.Split(stem, "-")
		if len(parts) < 3 {
			continue
		}
		platform := parts[len(parts)-2] + "-" + parts[len(parts)-1]
		if !validPolicyBinaryPlatforms[platform] {
			continue
		}
		names[strings.Join(parts[:len(parts)-2], "-")] = true
		binaries[platform] = filepath.Join("bin", e.Name())
	}

	if len(names) > 1 {
		return nil, fmt.Errorf(
			"found binaries for more than one policy pack name in bin/: %s",
			strings.Join(slices.Sorted(maps.Keys(names)), ", "))
	}
	return binaries, nil
}

// ParsePolicyBinaryOverrides parses --binary flag values of the form
// "<os>-<arch>=<path>" into a platform-to-path map. Paths must be relative to the
// policy pack directory.
func ParsePolicyBinaryOverrides(flags []string) (map[string]string, error) {
	binaries := make(map[string]string, len(flags))
	for _, f := range flags {
		platform, path, ok := strings.Cut(f, "=")
		if !ok || path == "" {
			return nil, fmt.Errorf("invalid --binary value %q: expected <os>-<arch>=<path>", f)
		}
		if !validPolicyBinaryPlatforms[platform] {
			return nil, fmt.Errorf("unknown platform %q; valid platforms are: %s",
				platform, strings.Join(slices.Sorted(maps.Keys(validPolicyBinaryPlatforms)), ", "))
		}
		if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
			return nil, fmt.Errorf("binary path for %q must be relative to the policy pack directory", platform)
		}
		binaries[platform] = filepath.Clean(filepath.FromSlash(path))
	}
	return binaries, nil
}

// PolicyPackBinary reports whether the policy pack at dir is a binary pack, and if
// so returns the path of the binary to exec. Installed packs carry the binary at the
// pack root as pulumi-analyzer-<name>; locally built packs carry it at the build
// convention path bin/pulumi-analyzer-<name>-<os>-<arch>.
func PolicyPackBinary(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	var rootMatches []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), policyBinaryPrefix) {
			continue
		}
		rootMatches = append(rootMatches, filepath.Join(dir, e.Name()))
	}
	if len(rootMatches) == 1 {
		return rootMatches[0], true
	}
	if len(rootMatches) > 1 {
		return "", false
	}

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	matches, err := filepath.Glob(
		filepath.Join(dir, "bin", policyBinaryPrefix+"*-"+CurrentPlatform()+suffix))
	if err != nil || len(matches) != 1 {
		return "", false
	}
	return matches[0], true
}
