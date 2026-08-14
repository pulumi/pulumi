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

package ints

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// fakeInstall lays a directory out the way get.pulumi.com does, with the CLI under test and stand-ins for the
// language plugins that ship beside it. withPlugins controls whether those stand-ins are created, which is what
// tells `pulumi self-update` the directory is a real install rather than a shim or a development build.
func fakeInstall(t *testing.T, withPlugins bool) string {
	t.Helper()

	pulumiName := "pulumi"
	if runtime.GOOS == "windows" {
		pulumiName = "pulumi.exe"
	}

	pulumiBin, err := exec.LookPath(pulumiName)
	require.NoError(t, err, "the CLI under test must be on $PATH")

	binDir := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o700))

	source, err := os.ReadFile(pulumiBin)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(binDir, pulumiName), source, 0o700)) //nolint:gosec // an executable

	if withPlugins {
		for _, runtimeName := range []string{"nodejs", "python", "go", "dotnet", "yaml", "java"} {
			name := "pulumi-language-" + runtimeName
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			require.NoError(t, os.WriteFile(filepath.Join(binDir, name), []byte("stub"), 0o700)) //nolint:gosec
		}
	}

	return filepath.Join(binDir, pulumiName)
}

// TestSelfUpdateRefusesAnInstallItDoesNotOwn checks that a CLI which is not part of a get.pulumi.com bundle is left
// alone, so that a copy managed by a package manager is never replaced underneath it.
func TestSelfUpdateRefusesAnInstallItDoesNotOwn(t *testing.T) {
	t.Parallel()

	cli := fakeInstall(t, false)

	//nolint:gosec // the path is built by the test
	out, err := exec.Command(cli, "self-update", "--dry-run").CombinedOutput()

	require.Error(t, err, "expected a non-zero exit, got: %s", out)
	assert.Contains(t, string(out), "does not look like a Pulumi CLI install")
}

// TestSelfUpdateDryRunResolvesARealRelease exercises the command against the live release endpoints. It reports the
// version and archive it would install without downloading or replacing anything.
func TestSelfUpdateDryRunResolvesARealRelease(t *testing.T) {
	t.Parallel()

	cli := fakeInstall(t, true)

	//nolint:gosec // the path is built by the test
	stdout, err := exec.Command(cli, "self-update", "--dry-run").CombinedOutput()
	require.NoError(t, err, "self-update --dry-run failed: %s", stdout)

	// The CLI under test carries a development version, so there is always a newer release to report.
	assert.Contains(t, string(stdout), "Would update Pulumi")

	artifact := artifactFromDryRun(t, string(stdout))
	assertPublishedArtifact(t, artifact)
}

// artifactFromDryRun pulls the archive name out of the dry run report.
func artifactFromDryRun(t *testing.T, out string) string {
	t.Helper()

	matches := regexp.MustCompile(`using (pulumi-v\S+?)\.\s*$`).FindStringSubmatch(strings.TrimSpace(out))
	require.Len(t, matches, 2, "could not find the archive name in: %s", out)
	return matches[1]
}

// assertPublishedArtifact confirms the archive the CLI intends to fetch is one the release actually publishes. The
// unit tests serve their own archives, so this is what keeps the naming convention honest against the real releases.
func assertPublishedArtifact(t *testing.T, artifact string) {
	t.Helper()

	version := regexp.MustCompile(`^pulumi-v(\d+\.\d+\.\d+\S*?)-`).FindStringSubmatch(artifact)
	require.Len(t, version, 2, "could not read a version out of %q", artifact)

	url := fmt.Sprintf(
		"https://github.com/pulumi/pulumi/releases/download/v%s/pulumi-%s-checksums.txt", version[1], version[1])
	resp, err := http.Get(url) //nolint:gosec,noctx // a fixed release URL
	require.NoError(t, err)
	defer contract.IgnoreClose(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET %s", url)

	checksums, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(checksums), artifact,
		"the release for v%s does not publish %s", version[1], artifact)
}
