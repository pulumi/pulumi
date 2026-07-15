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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// platformLinuxAmd64 is mandatory for binary-published policy packs: server-side
// policy evaluation runs on linux-amd64.
const platformLinuxAmd64 = "linux-amd64"

// discoverPolicyBinaries finds pre-built analyzer binaries in binaryDir by convention.
// Binaries are named "pulumi-analyzer-<name>-<os>-<arch>" (with a ".exe" suffix on
// Windows); the platform is read from the filename. It returns a platform -> absolute
// path map, empty when the directory has none.
func discoverPolicyBinaries(binaryDir string) (map[string]string, error) {
	matches, err := filepath.Glob(filepath.Join(binaryDir, workspace.AnalyzerBinaryPrefix+"*"))
	if err != nil {
		return nil, err
	}
	slices.Sort(matches)

	found := map[string]string{}
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || info.IsDir() {
			continue
		}
		base := filepath.Base(m)
		stem := strings.TrimSuffix(base, ".exe")
		hasExe := base != stem
		for platform := range workspace.ValidPolicyBinaryPlatforms {
			if !strings.HasSuffix(stem, "-"+platform) {
				continue
			}
			// Windows binaries carry the ".exe" suffix; others must not.
			if strings.HasPrefix(platform, "windows-") != hasExe {
				continue
			}
			if existing, ok := found[platform]; ok {
				return nil, fmt.Errorf("found multiple analyzer binaries for %s: %s and %s",
					platform, filepath.Base(existing), base)
			}
			found[platform] = m
			break
		}
	}
	return found, nil
}

// validateBinaryMatrix enforces the publish-time platform requirements for a policy
// pack publishing binaries: linux-amd64 is mandatory (server-side policy evaluation
// runs there), every discovered binary must exist, and the publishing host's platform
// must be present so conformance checks can boot it.
func validateBinaryMatrix(binaries map[string]string) error {
	if _, ok := binaries[platformLinuxAmd64]; !ok {
		return fmt.Errorf(
			"policy packs published with binaries must include a %s binary: "+
				"server-side policy evaluation runs on %s", platformLinuxAmd64, platformLinuxAmd64)
	}
	for _, platform := range slices.Sorted(maps.Keys(binaries)) {
		if _, err := os.Stat(binaries[platform]); err != nil {
			return fmt.Errorf("the binary for %s was not found at %s: %w", platform, binaries[platform], err)
		}
	}
	hostPlatform := workspace.CurrentPlatform()
	if _, ok := binaries[hostPlatform]; !ok {
		return fmt.Errorf(
			"cannot publish from %s: no %s binary was found, which is required to run "+
				"publish-time conformance checks", hostPlatform, hostPlatform)
	}
	return nil
}

// stageBinaryArtifact lays out one platform's artifact in a temp directory: just the
// binary under its canonical name, no manifest — consumers dispatch to it by convention,
// exactly like a provider plugin. The caller removes the directory.
func stageBinaryArtifact(binaryPath, analyzerName, platform string) (string, error) {
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

	dst := filepath.Join(stage, workspace.AnalyzerBinaryName(analyzerName, platform))
	if err := fsutil.CopyFile(dst, binaryPath, nil); err != nil {
		return "", err
	}
	// Guarantee the staged binary is executable regardless of the source file's mode:
	// the conformance boot execs it directly.
	if err := os.Chmod(dst, 0o755); err != nil {
		return "", err
	}
	keep = true
	return stage, nil
}

// buildBinaryArtifact builds the published artifact for one platform: a gzipped tarball
// of the canonically named binary, nested under the standard "package" directory.
func buildBinaryArtifact(binaryPath, analyzerName, platform string) ([]byte, error) {
	stage, err := stageBinaryArtifact(binaryPath, analyzerName, platform)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	return archive.TGZ(stage, packageDir, false /*useDefaultExcludes*/)
}

// buildBinaryArtifacts conformance-checks the host platform's binary and builds the
// artifact to publish for each discovered platform. It returns the analyzer metadata
// reported by the conformance run — the binary self-describes, like a provider's schema.
func buildBinaryArtifacts(
	ctx context.Context, op backend.PublishOperation, binaries map[string]string,
) (plugin.AnalyzerInfo, map[string][]byte, error) {
	if err := validateBinaryMatrix(binaries); err != nil {
		return plugin.AnalyzerInfo{}, nil, err
	}

	fmt.Println("Running conformance checks against the host platform binary")

	// Boot the exact shape consumers will install: the canonical binary in an otherwise
	// empty directory. Booting the author's pack dir instead would let build residue
	// (node_modules, package.json) mask a non-self-contained binary.
	hostPlatform := workspace.CurrentPlatform()
	stage, err := stageBinaryArtifact(binaries[hostPlatform], "conformance", hostPlatform)
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
	for platform, binPath := range binaries {
		tarball, err := buildBinaryArtifact(binPath, analyzerInfo.Name, platform)
		if err != nil {
			return plugin.AnalyzerInfo{}, nil, fmt.Errorf("building artifact for %s: %w", platform, err)
		}
		archives[platform] = tarball
	}

	return analyzerInfo, archives, nil
}

// sourceTarballBinaries lists which of the declared pack-relative paths are present
// under the source tarball's "package/" root. It never returns an error: a tarball it
// can't parse is reported as having no detected binaries, since this feeds a best-effort
// warning, not a gate.
func sourceTarballBinaries(sourceTarball []byte, declared map[string]bool) []string {
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
// dual-published alongside the platform binaries also contains one of the discovered
// analyzer binaries. buildSourceTarball reuses npm pack / gitignore-based archiving, so
// a pack author who doesn't exclude bin/ can end up shipping an un-conformance-checked
// binary inside the source fallback, bloating downloads for CLIs too old to use the
// binary artifacts. Binaries outside the pack directory can't be in the source archive
// and are ignored.
func warnIfSourceTarballContainsBinaries(sourceTarball []byte, packDir string, binaries map[string]string) {
	declared := map[string]bool{}
	for _, binPath := range binaries {
		rel, err := filepath.Rel(packDir, binPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		declared[path.Clean(filepath.ToSlash(rel))] = true
	}
	found := sourceTarballBinaries(sourceTarball, declared)
	if len(found) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr,
		"warning: the source archive for this policy pack contains built analyzer binaries (%s); "+
			"exclude them (e.g. via .npmignore or .gitignore) so they aren't shipped unconformance-checked "+
			"and bloating the source download\n",
		strings.Join(found, ", "))
}
