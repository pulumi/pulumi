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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// platformLinuxAmd64 is mandatory for binary-published policy packs: server-side
// policy evaluation runs on linux-amd64.
const platformLinuxAmd64 = "linux-amd64"

// validateBinaryMatrix enforces the publish-time platform requirements for a policy
// pack declaring binaries: linux-amd64 is mandatory (server-side policy evaluation
// runs there), every declared binary must exist, and the publishing host's platform
// must be present so conformance checks can boot it.
func validateBinaryMatrix(packDir string, binaries map[string]string) error {
	if _, ok := binaries[platformLinuxAmd64]; !ok {
		return fmt.Errorf(
			"policy packs published with binaries must include a %s binary: "+
				"server-side policy evaluation runs on %s; "+
				"remove the 'binary' option from PulumiPolicy.yaml to publish source only",
			platformLinuxAmd64, platformLinuxAmd64)
	}
	for _, platform := range slices.Sorted(maps.Keys(binaries)) {
		rel := binaries[platform]
		if _, err := os.Stat(filepath.Join(packDir, filepath.FromSlash(rel))); err != nil {
			return fmt.Errorf("the binary for %s was not found at %s: %w", platform, rel, err)
		}
	}
	hostPlatform := workspace.CurrentPlatform()
	if _, ok := binaries[hostPlatform]; !ok {
		return fmt.Errorf(
			"cannot publish from %s: no %s binary is declared in PulumiPolicy.yaml, "+
				"which is required to run publish-time conformance checks; "+
				"remove the 'binary' option to publish source only",
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

// stageBinaryArtifact lays out one platform's artifact in a temp directory: the binary
// under its canonical name, plus a generated manifest that declares it for the
// platform (so consumers dispatch to the binary without any language toolchain). The
// caller removes the directory.
func stageBinaryArtifact(
	packDir, binaryRelPath, analyzerName, platform string, runtime workspace.ProjectRuntimeInfo,
) (string, error) {
	stage, err := os.MkdirTemp("", "pulumi-policy-artifact-")
	if err != nil {
		return "", err
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(stage)
		}
	}()

	binName := canonicalBinaryName(analyzerName, platform)
	proj := &workspace.PolicyPackProject{
		Runtime: runtime,
		Binary:  map[string]string{platform: binName},
	}
	if err := proj.Save(filepath.Join(stage, "PulumiPolicy.yaml")); err != nil {
		return "", err
	}
	if err := copyFile(
		filepath.Join(packDir, filepath.FromSlash(binaryRelPath)),
		filepath.Join(stage, binName), 0o755); err != nil {
		return "", err
	}
	keep = true
	return stage, nil
}

// buildBinaryArtifact builds the published artifact for one platform: a gzipped
// tarball of the generated manifest and the canonically named binary, nested under the
// standard "package" directory.
func buildBinaryArtifact(
	packDir, binaryRelPath, analyzerName, platform string, runtime workspace.ProjectRuntimeInfo,
) ([]byte, error) {
	stage, err := stageBinaryArtifact(packDir, binaryRelPath, analyzerName, platform, runtime)
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

// buildBinaryArtifacts conformance-checks the host platform's declared binary and
// builds the artifact to publish for each platform the pack's manifest declares. It
// returns the analyzer metadata reported by the conformance run.
func buildBinaryArtifacts(
	ctx context.Context, op backend.PublishOperation,
) (plugin.AnalyzerInfo, map[string][]byte, error) {
	binaries := op.PolicyPack.Binary
	runtime := op.PolicyPack.Runtime

	packDir, err := filepath.Abs(op.PlugCtx.Pwd)
	if err != nil {
		return plugin.AnalyzerInfo{}, nil, err
	}
	if err := validateBinaryMatrix(packDir, binaries); err != nil {
		return plugin.AnalyzerInfo{}, nil, err
	}

	fmt.Println("Running conformance checks against the host platform binary")

	// Boot the exact shape consumers will install: manifest + canonical binary in an
	// otherwise empty directory. Booting the author's pack dir instead would let
	// build residue (node_modules, package.json) mask a non-self-contained binary.
	hostPlatform := workspace.CurrentPlatform()
	stage, err := stageBinaryArtifact(packDir, binaries[hostPlatform], "conformance", hostPlatform, runtime)
	if err != nil {
		return plugin.AnalyzerInfo{}, nil, err
	}
	defer os.RemoveAll(stage)

	analyzer, err := op.PlugCtx.Host.PolicyAnalyzer(op.PlugCtx, tokens.QName(stage), stage, nil /*opts*/)
	if err != nil {
		return plugin.AnalyzerInfo{}, nil,
			fmt.Errorf("conformance check failed: the %s binary did not boot: %w", hostPlatform, err)
	}
	// Declared after the stage removal defer so it runs first (defers are LIFO): the
	// analyzer process must exit before its staged directory is deleted, or removal
	// fails on Windows and the process otherwise leaks.
	defer contract.IgnoreClose(analyzer)

	analyzerInfo, err := analyzer.GetAnalyzerInfo(ctx)
	if err != nil {
		return plugin.AnalyzerInfo{}, nil, fmt.Errorf("conformance check failed: GetAnalyzerInfo: %w", err)
	}

	if _, err := analyzer.Analyze(ctx, plugin.AnalyzerResource{
		URN: resource.NewURN(
			tokens.QName("conformance"), tokens.PackageName("conformance"),
			tokens.Type(""), tokens.Type("pulumi:pulumi:Stack"), "conformance"),
		Type: tokens.Type("pulumi:pulumi:Stack"),
		Name: "conformance",
	}); err != nil {
		return plugin.AnalyzerInfo{}, nil, fmt.Errorf("conformance check failed: synthetic Analyze call: %w", err)
	}

	fmt.Println("Building per-platform artifacts")

	archives := make(map[string][]byte, len(binaries))
	for platform, rel := range binaries {
		tarball, err := buildBinaryArtifact(packDir, rel, analyzerInfo.Name, platform, runtime)
		if err != nil {
			return plugin.AnalyzerInfo{}, nil, fmt.Errorf("building artifact for %s: %w", platform, err)
		}
		archives[platform] = tarball
	}

	return analyzerInfo, archives, nil
}

// sourceTarballBinaries lists the declared binary paths that are present under the
// source tarball's "package/" root. It never returns an error: a tarball it can't
// parse is reported as having no detected binaries, since this feeds a best-effort
// warning, not a gate.
func sourceTarballBinaries(sourceTarball []byte, binaries map[string]string) []string {
	declared := make(map[string]bool, len(binaries))
	for _, rel := range binaries {
		declared[path.Clean(filepath.ToSlash(rel))] = true
	}

	gz, err := gzip.NewReader(bytes.NewReader(sourceTarball))
	if err != nil {
		return nil
	}
	defer gz.Close()

	var found []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Clean(strings.TrimPrefix(filepath.ToSlash(hdr.Name), packageDir+"/"))
		if declared[name] {
			found = append(found, name)
		}
	}
	slices.Sort(found)
	return found
}

// warnIfSourceTarballContainsBinaries prints a loud warning if the source archive
// dual-published alongside the platform binaries also contains one of the declared
// analyzer binaries. buildSourceTarball reuses npm pack / gitignore-based archiving,
// so a pack author who doesn't exclude bin/ can end up shipping an
// un-conformance-checked binary inside the source fallback, bloating downloads for
// CLIs too old to use the binary artifacts.
func warnIfSourceTarballContainsBinaries(sourceTarball []byte, binaries map[string]string) {
	found := sourceTarballBinaries(sourceTarball, binaries)
	if len(found) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr,
		"warning: the source archive for this policy pack contains built analyzer binaries (%s); "+
			"exclude them (e.g. via .npmignore or .gitignore) so they aren't shipped unconformance-checked "+
			"and bloating the source download\n",
		strings.Join(found, ", "))
}
