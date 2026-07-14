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
	"path"
	"runtime"
	"slices"
	"strings"
)

var validPolicyBinaryPlatforms = map[string]bool{
	"linux-amd64":   true,
	"linux-arm64":   true,
	"darwin-amd64":  true,
	"darwin-arm64":  true,
	"windows-amd64": true,
	"windows-arm64": true,
}

// CurrentPlatform returns the host platform in the "<os>-<arch>" form used to key
// policy pack binaries in a PulumiPolicy.yaml `binary` mapping.
func CurrentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// validatePolicyBinaries validates a PulumiPolicy.yaml `binary` mapping: platforms
// must be known, and paths must be slash-separated and relative to the pack directory.
func validatePolicyBinaries(binaries map[string]string) error {
	for _, platform := range slices.Sorted(maps.Keys(binaries)) {
		if !validPolicyBinaryPlatforms[platform] {
			return fmt.Errorf("unknown platform %q in 'binary'; valid platforms are: %s",
				platform, strings.Join(slices.Sorted(maps.Keys(validPolicyBinaryPlatforms)), ", "))
		}
		p := binaries[platform]
		if p == "" {
			return fmt.Errorf("the 'binary' entry for %s is missing a path", platform)
		}
		// Paths are slash-separated; also reject Windows absolute forms (`C:...`, `\...`)
		// so a manifest authored on Windows fails the same way everywhere.
		if path.IsAbs(p) || strings.HasPrefix(p, `\`) || (len(p) >= 2 && p[1] == ':') {
			return fmt.Errorf("the 'binary' path for %s must be relative to the policy pack directory", platform)
		}
	}
	return nil
}
