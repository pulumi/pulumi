// Copyright 2016-2020, Pulumi Corporation.
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

package npm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha1" //nolint:gosec // this is what NPM wants
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	pulumi_testing "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chdir temporarily changes the current directory of the program.
// It restores it to the original directory when the test is done.
func chdir(t *testing.T, dir string) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir)) // Set directory
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(cwd)) // Restore directory
		restoredDir, err := os.Getwd()
		if assert.NoError(t, err) {
			assert.Equal(t, cwd, restoredDir)
		}
	})
}

//nolint:paralleltest // mutates environment variables, changes working directory
func TestNPMInstall(t *testing.T) {
	t.Run("development", func(t *testing.T) {
		testInstall(t, "npm", false /*production*/)
	})

	t.Run("production", func(t *testing.T) {
		testInstall(t, "npm", true /*production*/)
	})
}

//nolint:paralleltest // mutates environment variables, changes working directory
func TestYarnInstall(t *testing.T) {
	t.Setenv("PULUMI_PREFER_YARN", "true")

	t.Run("development", func(t *testing.T) {
		testInstall(t, "yarn", false /*production*/)
	})

	t.Run("production", func(t *testing.T) {
		testInstall(t, "yarn", true /*production*/)
	})
}

//nolint:paralleltest // mutates environment variables, changes working directory
func TestPNPMInstall(t *testing.T) {
	t.Setenv("PULUMI_PREFER_PNPM", "true")

	t.Run("development", func(t *testing.T) {
		testInstall(t, "pnpm", false /*production*/)
	})

	t.Run("production", func(t *testing.T) {
		testInstall(t, "pnpm", true /*production*/)
	})
}

func testInstall(t *testing.T, expectedBin string, production bool) {
	// To test this functionality without actually hitting NPM,
	// we'll spin up a local HTTP server that implements a subset
	// of the NPM registry API.
	//
	// We'll tell NPM to use this server with a ~/.npmrc file
	// containing the line:
	//
	//   registry = <srv.URL>
	//
	// Similarly, we'll tell Yarn to use this server with a
	// ~/.yarnrc file containing the line:
	//
	//  registry "<srv.URL>"
	home := t.TempDir()
	t.Setenv("HOME", home)
	registryURL := fakeNPMRegistry(t)
	writeFile(t, filepath.Join(home, ".npmrc"),
		"registry="+registryURL)
	writeFile(t, filepath.Join(home, ".yarnrc"),
		"registry "+strconv.Quote(registryURL))

	// Create a new empty test directory and change the current working directory to it.
	tempdir := t.TempDir()
	chdir(t, tempdir)

	// Create a package directory to install dependencies into.
	pkgdir := filepath.Join(tempdir, "package")
	assert.NoError(t, os.Mkdir(pkgdir, 0o700))

	// Write out a minimal package.json file that has at least one dependency.
	packageJSONFilename := filepath.Join(pkgdir, "package.json")
	packageJSON := []byte(`{
	    "name": "test-package",
	    "license": "MIT",
	    "dependencies": {
	        "@pulumi/pulumi": "latest"
	    }
	}`)
	assert.NoError(t, os.WriteFile(packageJSONFilename, packageJSON, 0o600))

	// Install dependencies, passing nil for stdout and stderr, which connects
	// them to the file descriptor for the null device (os.DevNull).
	pulumi_testing.YarnInstallMutex.Lock()
	defer pulumi_testing.YarnInstallMutex.Unlock()
	out := iotest.LogWriter(t)
	bin, err := Install(context.Background(), pkgdir, production, out, out)
	assert.NoError(t, err)
	assert.Equal(t, expectedBin, bin)
}

// fakeNPMRegistry starts up an HTTP server that implements a subset of the NPM registry API
// that is sufficient for the tests in this file.
// The server will shut down when the test is complete.
//
// The server responds with fake information about a single package:
// @pulumi/pulumi.
//
// See https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md for
// details on the protocol.
func fakeNPMRegistry(t testing.TB) string {
	t.Helper()

	// The server needs the tarball's SHA-1 hash so we'll build it in
	// advance.
	tarball, tarballSHA1 := tarballOf(t,
		// The bare minimum files needed by NPM.
		"package/package.json", `{
			"name": "@pulumi/pulumi",
			"license": "MIT"
		}`)

	var srv *httptest.Server
	// Separate assignment so we can access srv.URL in the handler.
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("[fakeNPMRegistry] %v %v", r.Method, r.URL.Path)

		if r.Method != http.MethodGet {
			// We only expect GET requests.
			http.NotFound(w, r)
			return
		}

		switch r.URL.Path {
		case "/@pulumi/pulumi":
			tarballURL := srv.URL + "/@pulumi/pulumi/-/pulumi-3.0.0.tgz"
			fmt.Fprintf(w, `{
				"name": "@pulumi/pulumi",
				"dist-tags": {"latest": "3.0.0"},
				"versions": {
					"3.0.0": {
						"name": "@pulumi/pulumi",
						"version": "3.0.0",
						"dist": {
							"tarball": %q,
							"shasum": %q
						}
					}
				}
			}`, tarballURL, tarballSHA1)

		case "/@pulumi/pulumi/-/pulumi-3.0.0.tgz":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.Itoa(len(tarball)))
			_, err := w.Write(tarball)
			if !assert.NoError(t, err) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// tarballOf constructs a .tar.gz archive containing the given files
// and returns the raw bytes and the SHA-1 hash of the archive.
//
// The files are specified as a list of pairs of paths and contents.
// The paths are relative to the root of the archive.
func tarballOf(t testing.TB, pairs ...string) (data []byte, sha string) {
	t.Helper()

	require.True(t, len(pairs)%2 == 0, "pairs must be a list of path/contents pairs")

	var buff bytes.Buffer // raw .tar.gz bytes
	hash := sha1.New()    //nolint:gosec // this is what NPM wants

	// Order of which writer wraps which is important here.
	// .tar.gz means we need .gz to be the innermost writer.
	gzipw := gzip.NewWriter(io.MultiWriter(&buff, hash))
	tarw := tar.NewWriter(gzipw)

	for i := 0; i < len(pairs); i += 2 {
		path, contents := pairs[i], pairs[i+1]
		require.NoError(t, tarw.WriteHeader(&tar.Header{
			Name: path,
			Mode: 0o600,
			Size: int64(len(contents)),
		}), "WriteHeader(%q)", path)
		_, err := tarw.Write([]byte(contents))
		require.NoError(t, err, "WriteContents(%q)", path)
	}

	// Closing the writers will flush them and write the final tarball bytes.
	require.NoError(t, tarw.Close())
	require.NoError(t, gzipw.Close())

	return buff.Bytes(), hex.EncodeToString(hash.Sum(nil))
}

// writeFile creates a file at the given path with the given contents.
func writeFile(t testing.TB, path, contents string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
}

// This test verifies that the package management resolution algorithm
// selects the right package manager based on environment variables.
//nolint:paralleltest // mutates environment variables
func TestPkgManagerResolutionEnvVar(t *testing.T) {
	type TestCase struct {
		preferYarn, preferPNPM bool
		expectedManager        string
		expectedErr            error
	}

	// These cases test all combinations of environment variables.
	cases := []TestCase{
		{
			preferYarn:      false,
			preferPNPM:      false,
			expectedManager: "npm",
		},
		{
			preferYarn:      false,
			preferPNPM:      true,
			expectedManager: "pnpm",
		},
		{
			preferYarn:      true,
			preferPNPM:      false,
			expectedManager: "yarn",
		},
		{
			preferYarn:  true,
			preferPNPM:  true,
			expectedErr: mutuallyExclusiveEnvVars,
		},
	}

	//  • Create a temp directory, which we know to be empty,
	//    that we can use as PWD.
	pwd, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.Remove(pwd)

	fakePath := fakePkgManagerPaths(t)
	defer os.RemoveAll(fakePath)

	for _, tc := range cases {
		tc := tc
		name := fmt.Sprintf("PREFER_YARN=%v,PREFER_PNPM=%v", tc.preferYarn, tc.preferPNPM)
		t.Run(name, func(tt *testing.T) {
			// Set the environment variables before running the resolver.
			setYarn := fmt.Sprintf("%t", tc.preferYarn)
			setPNPM := fmt.Sprintf("%t", tc.preferPNPM)
			t.Setenv("PULUMI_PREFER_YARN", setYarn)
			t.Setenv("PULUMI_PREFER_PNPM", setPNPM)
			manager, err := ResolvePackageManager(pwd)
			// If we expect an error, check that it occurs, the move
			// to the next test case.
			if tc.expectedErr != nil {
				assert.EqualError(tt, err, tc.expectedErr.Error())
				return
			}
			// Check that the manager we received matches the manager we expected.
			require.NoError(tt, err)
			assert.Equal(tt, tc.expectedManager, manager.Name())
		})
	}
}

// This test verifies that the package management resolution algorithm
// selects the right package manager based on lockfiles. We expect
// yarn > pnpm > default (npm)
//nolint:paralleltest // mutates environment variables
func TestPkgManagerResolutionLockfiles(t *testing.T) {
	type TestCase struct {
		hasYarnLock, hasPNPMLock bool
		expectedManager          string
	}

	// These cases test all combinations of lockfile we care about.
	cases := []TestCase{
		{
			hasYarnLock:     false,
			hasPNPMLock:     false,
			expectedManager: "npm",
		},
		{
			hasYarnLock:     false,
			hasPNPMLock:     true,
			expectedManager: "pnpm",
		},
		{
			hasYarnLock:     true,
			hasPNPMLock:     false,
			expectedManager: "yarn",
		},
		{
			hasYarnLock:     true,
			hasPNPMLock:     true,
			expectedManager: "yarn",
		},
	}

	fakePath := fakePkgManagerPaths(t)
	defer os.RemoveAll(fakePath)

	for _, tc := range cases {
		tc := tc
		name := fmt.Sprintf("has-yarn-lock=%v,has-pnpmlock-%v", tc.hasYarnLock, tc.hasPNPMLock)
		t.Run(name, func(tt *testing.T) {
			//  • Create a temp directory, which we know to be empty,
			//    that we can use as PWD and write lockfiles into.
			pwd, err := os.MkdirTemp("", "")
			require.NoError(tt, err)
			defer os.RemoveAll(pwd)
			// • Touch the lockfiles conditionally.
			touchFile(tt, tc.hasYarnLock, pwd, "yarn.lock")
			touchFile(tt, tc.hasPNPMLock, pwd, "pnpm-lock.yaml")
			// • Resolve the manager.
			manager, err := ResolvePackageManager(pwd)
			// Check that the manager we received matches the manager we expected.
			require.NoError(tt, err)
			assert.Equal(tt, tc.expectedManager, manager.Name())
		})
	}
}

func touchFile(t *testing.T, enabled bool, dir, name string) {
	if enabled {
		lockfile := filepath.Join(dir, name)
		_, err := os.Create(lockfile)
		require.NoError(t, err)
	}
}

// fakePkgManagerPaths creates a directory to store fake package manager
// executables for lookup. It returns the name of the directory, and
// updates the PATH to point to this directory.
func fakePkgManagerPaths(t *testing.T) string {
	//  • Create a temp directory that we can use as the fake path.
	//    We store our fake executables here.
	fakePath, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	// Create three fake executables in the temp dir, one for yarn, pnpm,
	// and npm.
	for _, pkgManager := range []string{"npm", "yarn", "pnpm"} {
		fullPath := filepath.Join(fakePath, pkgManager)
		// Create the file with permission to read, execute, and write.
		_, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0o755)
		require.NoError(t, err)
	}

	// • Override the PATH so we can mock the location of yarn, pnpm, and npm.
	t.Setenv("PATH", fakePath)
	return fakePath
}