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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// validateBinaryMatrix enforces the publish-time platform requirements for a policy
// pack publishing binaries: linux-amd64 is mandatory (server-side policy evaluation
// runs there), every declared binary must exist, and the publishing host's platform
// must be present so conformance checks can boot it.
func validateBinaryMatrix(packDir string, binaries map[string]string) error {
	if _, ok := binaries[workspace.PlatformLinuxAmd64]; !ok {
		return fmt.Errorf(
			"policy packs published with binaries must include a %s binary: "+
				"server-side policy evaluation runs on %s",
			workspace.PlatformLinuxAmd64, workspace.PlatformLinuxAmd64)
	}
	for _, platform := range slices.Sorted(maps.Keys(binaries)) {
		rel := binaries[platform]
		if _, err := os.Stat(filepath.Join(packDir, rel)); err != nil {
			return fmt.Errorf("the binary for %s was not found at %s: %w", platform, rel, err)
		}
	}
	hostPlatform := workspace.CurrentPlatform()
	if _, ok := binaries[hostPlatform]; !ok {
		return fmt.Errorf(
			"cannot publish from %s: no %s binary was built, "+
				"which is required to run publish-time conformance checks",
			hostPlatform, hostPlatform)
	}
	return nil
}

func canonicalBinaryName(analyzerName, platform string) string {
	name := "pulumi-analyzer-" + analyzerName
	if strings.HasPrefix(platform, "windows-") {
		name += ".exe"
	}
	return name
}

// stageBinaryArtifact lays out one platform's artifact in a temp directory: the pack
// manifest plus the binary under its canonical name. The caller removes the directory.
func stageBinaryArtifact(packDir, binaryRelPath, analyzerName, platform string) (string, error) {
	stage, err := os.MkdirTemp("", "pulumi-policy-artifact-")
	if err != nil {
		return "", err
	}
	if err := copyFile(
		filepath.Join(packDir, "PulumiPolicy.yaml"),
		filepath.Join(stage, "PulumiPolicy.yaml"), 0o644); err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	if err := copyFile(
		filepath.Join(packDir, binaryRelPath),
		filepath.Join(stage, canonicalBinaryName(analyzerName, platform)), 0o755); err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	return stage, nil
}

// buildBinaryArtifact builds the published artifact for one platform: a gzipped
// tarball of the pack manifest and the canonically named binary, nested under the
// standard "package" directory.
func buildBinaryArtifact(packDir, binaryRelPath, analyzerName, platform string) ([]byte, error) {
	stage, err := stageBinaryArtifact(packDir, binaryRelPath, analyzerName, platform)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
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

// publishWithBinaries publishes a pack that built per-platform binaries: it boots the
// host platform's staged artifact for conformance, builds every platform's artifact,
// and dual-uploads the source archive plus the binaries.
func (pack *cloudPolicyPack) publishWithBinaries(
	ctx context.Context, op backend.PublishOperation, binaries map[string]string,
) error {
	packDir, err := filepath.Abs(op.PlugCtx.Pwd)
	if err != nil {
		return err
	}
	if err := validateBinaryMatrix(packDir, binaries); err != nil {
		return err
	}

	fmt.Println("Running conformance checks against the host platform binary")

	// Boot the exact shape consumers will install: manifest + canonical binary in an
	// otherwise empty directory. Booting the author's pack dir instead would let
	// build residue (node_modules, package.json) mask a non-self-contained binary.
	hostPlatform := workspace.CurrentPlatform()
	stage, err := stageBinaryArtifact(packDir, binaries[hostPlatform], "conformance", hostPlatform)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	analyzer, err := op.PlugCtx.Host.PolicyAnalyzer(op.PlugCtx, tokens.QName(stage), stage, nil /*opts*/)
	if err != nil {
		return fmt.Errorf("conformance check failed: the %s binary did not boot: %w", hostPlatform, err)
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
		tarball, err := buildBinaryArtifact(packDir, rel, analyzerInfo.Name, platform)
		if err != nil {
			return fmt.Errorf("building artifact for %s: %w", platform, err)
		}
		archives[platform] = tarball
	}

	fmt.Println("Compressing policy pack")

	sourceTarball, err := buildSourceTarball(ctx, op)
	if err != nil {
		return err
	}

	fmt.Println("Uploading policy pack to Pulumi service")

	publishedVersion, err := pack.cl.PublishPolicyPackWithPlatforms(
		ctx, pack.ref.orgName, op.PolicyPack.Runtime.Name(), analyzerInfo,
		sourceTarball, archives, op.Metadata)
	if err != nil {
		return err
	}

	fmt.Printf("\nPermalink: %s/%s\n", pack.ref.CloudConsoleURL(), publishedVersion)
	return nil
}
