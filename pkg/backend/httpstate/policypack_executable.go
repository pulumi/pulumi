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

package httpstate

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// validateExecutableMatrix enforces the publish-time platform requirements for an executable
// policy pack: linux-amd64 is mandatory because server-side policy evaluation runs there, and
// every declared binary must exist on disk.
//
// There is no check that the publishing host's own platform is declared. Publish boots the pack
// to read its metadata, so a missing host binary already fails earlier, in the analyzer.
func validateExecutableMatrix(packDir string, binaries map[string]string) error {
	if _, ok := binaries[workspace.PlatformLinuxAmd64]; !ok {
		return fmt.Errorf(
			"executable policy packs must declare a %s binary: server-side policy evaluation runs on %s",
			workspace.PlatformLinuxAmd64, workspace.PlatformLinuxAmd64)
	}
	for _, platform := range slices.Sorted(maps.Keys(binaries)) {
		rel := binaries[platform]
		if _, err := os.Stat(filepath.Join(packDir, rel)); err != nil {
			return fmt.Errorf("the binary declared for %s was not found at %s: %w", platform, rel, err)
		}
	}
	return nil
}

// buildExecutableArtifacts builds one tarball per declared platform of an executable policy pack.
func buildExecutableArtifacts(
	proj *workspace.PolicyPackProject, packDir string,
) (map[string][]byte, error) {
	binaries, err := proj.ExecutableBinaries()
	if err != nil {
		return nil, err
	}
	if err := validateExecutableMatrix(packDir, binaries); err != nil {
		return nil, err
	}

	artifacts := make(map[string][]byte, len(binaries))
	for platform, rel := range binaries {
		tarball, err := buildExecutablePlatformTarball(packDir, rel)
		if err != nil {
			return nil, fmt.Errorf("building artifact for %s: %w", platform, err)
		}
		artifacts[platform] = tarball
	}
	return artifacts, nil
}

// buildExecutablePlatformTarball builds the artifact for one platform of an executable policy
// pack: a gzipped tarball containing only the pack manifest and that platform's binary, nested
// under the standard "package" directory.
func buildExecutablePlatformTarball(packDir, binaryRelPath string) ([]byte, error) {
	return archive.TGZFiles([]archive.File{
		{
			Path:   "PulumiPolicy.yaml",
			Source: filepath.Join(packDir, "PulumiPolicy.yaml"),
			Mode:   0o644,
		},
		{
			Path:   binaryRelPath,
			Source: filepath.Join(packDir, binaryRelPath),
			Mode:   0o755,
		},
	}, packageDir)
}
