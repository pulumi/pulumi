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
	"io"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// validateExecutableMatrix enforces the publish-time platform requirements for
// an executable policy pack: linux-amd64 is mandatory (server-side evaluation
// runs there), every declared binary must exist, and the publishing host's
// platform must be declared so conformance checks can boot it.
func validateExecutableMatrix(packDir string, binaries map[string]string) error {
	if _, ok := binaries[workspace.PlatformLinuxAmd64]; !ok {
		return fmt.Errorf(
			"executable policy packs must declare a %s binary: server-side policy evaluation runs on %s",
			workspace.PlatformLinuxAmd64, workspace.PlatformLinuxAmd64)
	}
	for platform, rel := range binaries {
		if _, err := os.Stat(filepath.Join(packDir, rel)); err != nil {
			return fmt.Errorf("the binary declared for %s was not found at %s: %w", platform, rel, err)
		}
	}
	hostPlatform := workspace.CurrentPlatform()
	if _, ok := binaries[hostPlatform]; !ok {
		return fmt.Errorf(
			"cannot publish from %s: the pack does not declare a %s binary, "+
				"which is required to run publish-time conformance checks",
			hostPlatform, hostPlatform)
	}
	return nil
}

// buildExecutablePlatformTarball builds the artifact for one platform of an
// executable policy pack: a gzipped tarball containing only the pack manifest
// and that platform's binary, nested under the standard "package" directory.
func buildExecutablePlatformTarball(packDir, binaryRelPath string) ([]byte, error) {
	stage, err := os.MkdirTemp("", "pulumi-policy-artifact-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)

	if err := copyFile(
		filepath.Join(packDir, "PulumiPolicy.yaml"),
		filepath.Join(stage, "PulumiPolicy.yaml"), 0o644); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(stage, binaryRelPath)), 0o755); err != nil {
		return nil, err
	}
	if err := copyFile(
		filepath.Join(packDir, binaryRelPath),
		filepath.Join(stage, binaryRelPath), 0o755); err != nil {
		return nil, err
	}

	return archive.TGZ(stage, packageDir, false /*useDefaultExcludes*/)
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
