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
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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
	for _, platform := range slices.Sorted(maps.Keys(binaries)) {
		rel := binaries[platform]
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
// executable policy pack: a gzipped tarball containing the pack manifest,
// that platform's binary, and package.json when present, nested under the
// standard "package" directory.
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
	// The nodejs policy SDK reads the pack's package.json at boot for version
	// metadata, so ship it alongside the binary when the pack has one.
	if _, err := os.Stat(filepath.Join(packDir, "package.json")); err == nil {
		if err := copyFile(
			filepath.Join(packDir, "package.json"),
			filepath.Join(stage, "package.json"), 0o644); err != nil {
			return nil, err
		}
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

func (pack *cloudPolicyPack) publishExecutable(ctx context.Context, op backend.PublishOperation) error {
	packDir, err := filepath.Abs(op.PlugCtx.Pwd)
	if err != nil {
		return err
	}

	binaries, err := op.PolicyPack.ExecutableBinaries()
	if err != nil {
		return err
	}
	if err := validateExecutableMatrix(packDir, binaries); err != nil {
		return err
	}

	fmt.Println("Running conformance checks against the host platform binary")

	analyzer, err := op.PlugCtx.Host.PolicyAnalyzer(op.PlugCtx, tokens.QName(packDir), op.PlugCtx.Pwd, nil /*opts*/)
	if err != nil {
		return fmt.Errorf("conformance check failed: the %s binary did not boot: %w",
			workspace.CurrentPlatform(), err)
	}

	analyzerInfo, err := analyzer.GetAnalyzerInfo(ctx)
	if err != nil {
		return fmt.Errorf("conformance check failed: GetAnalyzerInfo: %w", err)
	}

	if _, err := analyzer.Analyze(ctx, plugin.AnalyzerResource{
		URN: resource.NewURN(
			tokens.QName("conformance"), tokens.PackageName("conformance"),
			tokens.Type(""), tokens.Type("pulumi:pulumi:Stack"), "conformance"),
		Type: tokens.Type("pulumi:pulumi:Stack"),
		Name: "conformance",
	}); err != nil {
		return fmt.Errorf("conformance check failed: synthetic Analyze call: %w", err)
	}

	pack.ref.name = tokens.QName(analyzerInfo.Name)
	pack.ref.versionTag = analyzerInfo.Version

	fmt.Println("Building per-platform artifacts")

	archives := make(map[string][]byte, len(binaries))
	for platform, rel := range binaries {
		tarball, err := buildExecutablePlatformTarball(packDir, rel)
		if err != nil {
			return fmt.Errorf("building artifact for %s: %w", platform, err)
		}
		archives[platform] = tarball
	}

	fmt.Println("Uploading policy pack to Pulumi service")

	publishedVersion, err := pack.cl.PublishPolicyPackPlatforms(
		ctx, pack.ref.orgName, workspace.PolicyRuntimeExecutable, analyzerInfo, archives, op.Metadata)
	if err != nil {
		return err
	}

	fmt.Printf("\nPermalink: %s/%s\n", pack.ref.CloudConsoleURL(), publishedVersion)
	return nil
}
