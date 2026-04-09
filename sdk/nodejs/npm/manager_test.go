// Copyright 2016, Pulumi Corporation.
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
	"crypto/sha1" //nolint:gosec // this is what NPM wants
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // changes working directory
func TestNPMInstall(t *testing.T) {
	t.Run("development", func(t *testing.T) {
		testInstall(t, "npm", false /*production*/)
	})

	t.Run("production", func(t *testing.T) {
		testInstall(t, "npm", true /*production*/)
	})
}

//nolint:paralleltest // changes working directory
func TestYarnInstall(t *testing.T) {
	t.Run("development", func(t *testing.T) {
		testInstall(t, "yarn", false /*production*/)
	})

	t.Run("production", func(t *testing.T) {
		testInstall(t, "yarn", true /*production*/)
	})
}

//nolint:paralleltest // changes working directory
func TestPnpmInstall(t *testing.T) {
	t.Run("development", func(t *testing.T) {
		testInstall(t, "pnpm", false /*production*/)
	})

	t.Run("production", func(t *testing.T) {
		testInstall(t, "pnpm", true /*production*/)
	})
}

//nolint:paralleltest // changes working directory
func TestBunInstall(t *testing.T) {
	t.Run("development", func(t *testing.T) {
		testInstall(t, "bun", false /*production*/)
	})

	/*
		Commenting this out because Bun has a bug where --production
		enforces "frozen lockfile" when it probably shouldn't: https://github.com/oven-sh/bun/issues/10949
	*/
	// t.Run("production", func(t *testing.T) {
	// 	testInstall(t, "bun", true /*production*/)
	// })
}

func TestResolvePackageManager(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		pm        PackageManagerType
		lockFiles []string
		// manifests names manifest files to write into the project directory. Each entry is one of "package.json" or
		// "package.yaml".
		manifests []string
		// expected is the package manager name we expect to resolve to. Leave empty when expecting an error.
		expected string
		// expectedErr, if non-empty, asserts that ResolvePackageManager returns an error containing this substring.
		expectedErr string
	}{
		{name: "defaults to npm", pm: AutoPackageManager, expected: "npm"},
		{name: "picks npm", pm: NpmPackageManager, expected: "npm"},
		{name: "picks yarn", pm: YarnPackageManager, expected: "yarn"},
		{name: "picks pnpm", pm: PnpmPackageManager, expected: "pnpm"},
		{name: "picks bun", pm: BunPackageManager, expected: "bun"},
		{name: "picks npm based on lockfile", pm: AutoPackageManager, lockFiles: []string{"npm"}, expected: "npm"},
		{name: "picks yarn based on lockfile", pm: AutoPackageManager, lockFiles: []string{"yarn"}, expected: "yarn"},
		{name: "picks pnpm based on lockfile", pm: AutoPackageManager, lockFiles: []string{"pnpm"}, expected: "pnpm"},
		{name: "picks bun based on lockfile", pm: AutoPackageManager, lockFiles: []string{"bun"}, expected: "bun"},
		{
			name: "yarn > pnpm > npm", pm: AutoPackageManager,
			lockFiles: []string{"yarn", "pnpm", "npm"}, expected: "yarn",
		},
		{
			name: "pnpm > bun > npm", pm: AutoPackageManager,
			lockFiles: []string{"pnpm", "bun", "npm"}, expected: "pnpm",
		},
		{name: "bun > npm", pm: AutoPackageManager, lockFiles: []string{"bun", "npm"}, expected: "bun"},
		// pnpm allows package.yaml in place of package.json
		{
			name: "auto picks pnpm when only package.yaml exists",
			pm:   AutoPackageManager, manifests: []string{"package.yaml"}, expected: "pnpm",
		},
		{
			name: "auto picks pnpm with package.yaml even if yarn.lock is present",
			pm:   AutoPackageManager, manifests: []string{"package.yaml"},
			lockFiles: []string{"yarn"}, expected: "pnpm",
		},
		{
			name: "auto respects yarn.lock when both manifests exist",
			pm:   AutoPackageManager, manifests: []string{"package.json", "package.yaml"},
			lockFiles: []string{"yarn"}, expected: "yarn",
		},
		{
			name: "explicit pnpm with package.yaml works",
			pm:   PnpmPackageManager, manifests: []string{"package.yaml"}, expected: "pnpm",
		},
		{
			name: "explicit npm with only package.yaml is rejected",
			pm:   NpmPackageManager, manifests: []string{"package.yaml"},
			expectedErr: "package.yaml",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for _, lockFile := range tt.lockFiles {
				writeLockFile(t, dir, lockFile)
			}
			for _, manifest := range tt.manifests {
				writeManifest(t, dir, manifest)
			}
			pm, err := ResolvePackageManager(tt.pm, dir)
			if tt.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, pm.Name())
		})
	}
}

func TestResolvePackageManager_SearchesUp(t *testing.T) {
	t.Parallel()

	t.Run("finds package.yaml in parent dir", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "package.yaml"), "name: test\nversion: 1.0.0\n")
		sub := filepath.Join(root, "project")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		pm, err := ResolvePackageManager(AutoPackageManager, sub)
		require.NoError(t, err)
		require.Equal(t, "pnpm", pm.Name())
	})

	t.Run("nearest package.json wins over package.yaml further up", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "package.yaml"), "name: root\nversion: 1.0.0\n")
		sub := filepath.Join(root, "subdir")
		require.NoError(t, os.MkdirAll(sub, 0o755))
		writeFile(t, filepath.Join(sub, "package.json"), `{"name":"subdir","version":"1.0.0"}`)
		project := filepath.Join(sub, "project")
		require.NoError(t, os.MkdirAll(project, 0o755))

		pm, err := ResolvePackageManager(AutoPackageManager, project)
		require.NoError(t, err)
		require.Equal(t, "npm", pm.Name())
	})

	t.Run("nearest package.yaml wins over package.json further up", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "package.json"), `{"name":"root","version":"1.0.0"}`)
		sub := filepath.Join(root, "subdir")
		require.NoError(t, os.MkdirAll(sub, 0o755))
		writeFile(t, filepath.Join(sub, "package.yaml"), "name: subdir\nversion: 1.0.0\n")
		project := filepath.Join(sub, "project")
		require.NoError(t, os.MkdirAll(project, 0o755))

		pm, err := ResolvePackageManager(AutoPackageManager, project)
		require.NoError(t, err)
		require.Equal(t, "pnpm", pm.Name())
	})

	t.Run("explicit npm rejected when nearest manifest is package.yaml in parent", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "package.yaml"), "name: test\nversion: 1.0.0\n")
		project := filepath.Join(root, "project")
		require.NoError(t, os.MkdirAll(project, 0o755))

		_, err := ResolvePackageManager(NpmPackageManager, project)
		require.Error(t, err)
		require.Contains(t, err.Error(), "package.yaml")
	})
}

func TestFilterDirectDependencies_PackageYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.yaml"), []byte(
		"name: test\n"+
			"version: 1.0.0\n"+
			"dependencies:\n"+
			"  '@pulumi/pulumi': ^3.0.0\n"+
			"devDependencies:\n"+
			"  typescript: ^5.0.0\n",
	), 0o600))

	deps := []plugin.DependencyInfo{
		{Name: "@pulumi/pulumi", Version: "3.224.0"},
		{Name: "typescript", Version: "5.9.3"},
		{Name: "@grpc/grpc-js", Version: "1.14.3"}, // transitive — should be filtered out
	}

	got, err := filterDirectDependencies(dir, deps)
	require.NoError(t, err)
	require.ElementsMatch(t, []plugin.DependencyInfo{
		{Name: "@pulumi/pulumi", Version: "3.224.0"},
		{Name: "typescript", Version: "5.9.3"},
	}, got)
}

func TestPack(t *testing.T) {
	t.Parallel()

	packageJSON := []byte(`{
	    "name": "test-package",
		"version": "1.0"
	}`)

	for _, pm := range []string{"npm", "yarn", "pnpm", "bun"} {
		t.Run(pm, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeLockFile(t, dir, pm)
			packageJSONFilename := filepath.Join(dir, "package.json")
			require.NoError(t, os.WriteFile(packageJSONFilename, packageJSON, 0o600))
			stderr := new(bytes.Buffer)

			artifact, err := Pack(t.Context(), AutoPackageManager, dir, stderr)

			require.NoError(t, err)
			// check that the artifact contains a package.json
			b, err := gzip.NewReader(bytes.NewReader((artifact)))
			require.NoError(t, err)
			tr := tar.NewReader(b)
			for {
				h, err := tr.Next()
				if err == io.EOF {
					require.Fail(t, "package.json not found")
					break
				}
				require.NoError(t, err)
				if h.Name == "package/package.json" {
					break
				}
			}
		})
	}
}

func TestPackInvalidPackageJSON(t *testing.T) {
	t.Parallel()

	// Missing a version field
	packageJSON := []byte(`{
	    "name": "test-package"
	}`)

	for _, tt := range []struct{ packageManager, expectedErrorMessage string }{
		{"npm", "Invalid package, must have name and version"},
		{"yarn", "Package doesn't have a version"},
		{"pnpm", "Package version is not defined in the package.json"},
		{"bun", "error: package.json must have `name` and `version` fields"},
	} {
		t.Run(tt.packageManager, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeLockFile(t, dir, tt.packageManager)
			packageJSONFilename := filepath.Join(dir, "package.json")
			require.NoError(t, os.WriteFile(packageJSONFilename, packageJSON, 0o600))
			stderr := new(bytes.Buffer)

			_, err := Pack(t.Context(), AutoPackageManager, dir, stderr)
			exitErr := new(exec.ExitError)
			require.ErrorAs(t, err, &exitErr)
			assert.NotZero(t, exitErr.ExitCode())
			require.Contains(t, stderr.String(), tt.expectedErrorMessage)
		})
	}
}

//nolint:paralleltest
func TestBunPackNonExistentPackageJSON(t *testing.T) {
	dir := t.TempDir()
	stderr := new(bytes.Buffer)
	errorMessage := "error: No package.json was found for directory"

	_, err := Pack(t.Context(), "bun", dir, stderr)
	exitErr := new(exec.ExitError)
	require.ErrorAs(t, err, &exitErr)
	assert.NotZero(t, exitErr.ExitCode())
	require.Contains(t, stderr.String(), errorMessage)
}

//nolint:paralleltest // chdir
func TestManagerVersion(t *testing.T) {
	for _, pmType := range []PackageManagerType{
		NpmPackageManager,
		YarnPackageManager,
		PnpmPackageManager,
		BunPackageManager,
	} {
		t.Run(string(pmType), func(t *testing.T) {
			//nolint:paralleltest // chdir
			dir := t.TempDir()
			t.Chdir(dir)

			pm, err := ResolvePackageManager(pmType, dir)
			require.NoError(t, err)

			version, err := pm.Version()
			require.NoError(t, err)
			require.NotEqual(t, version, semver.Version{})
		})
	}
}

// writeLockFile writes a mock lockfile for the selected package manager
func writeLockFile(t *testing.T, dir string, packageManager string) {
	t.Helper()
	switch packageManager {
	case "npm":
		writeFile(t, filepath.Join(dir, "package-lock.json"), "{\"lockfileVersion\": 2}")
	case "yarn":
		writeFile(t, filepath.Join(dir, "yarn.lock"), "# yarn lockfile v1")
	case "pnpm":
		writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: '6.0'")
	case "bun":
		writeFile(t, filepath.Join(dir, "bun.lock"), "{\"lockfileVersion\": 1, \"workspaces\": "+
			"{\"\": {\"name\": \"test-package\",},}}")
	}
}

func writeManifest(t *testing.T, dir string, filename string) {
	t.Helper()
	switch filename {
	case "package.json":
		writeFile(t, filepath.Join(dir, filename), `{"name":"test","version":"1.0.0"}`)
	case "package.yaml":
		writeFile(t, filepath.Join(dir, filename), "name: test\nversion: 1.0.0\n")
	default:
		t.Fatalf("unknown manifest filename: %s", filename)
	}
}

func testInstall(t *testing.T, packageManager string, production bool) {
	// To test this functionality without actually hitting NPM,
	// we'll spin up a local HTTP server that implements a subset
	// of the NPM registry API.
	//
	// We'll tell NPM to use this server with a ~/.npmrc file
	// containing the line:
	//
	//   registry = <srv.URL>
	//
	// Pnpm reads the same .npmrc file.
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
	t.Chdir(tempdir)

	// Create a package directory to install dependencies into.
	pkgdir := filepath.Join(tempdir, "package")
	require.NoError(t, os.Mkdir(pkgdir, 0o700))

	// Write out a minimal package.json file that has at least one dependency.
	packageJSONFilename := filepath.Join(pkgdir, "package.json")
	packageJSON := []byte(`{
	    "name": "test-package",
	    "license": "MIT",
	    "dependencies": {
	        "@pulumi/pulumi": "latest"
	    }
	}`)
	require.NoError(t, os.WriteFile(packageJSONFilename, packageJSON, 0o600))

	writeLockFile(t, pkgdir, packageManager)

	// Install dependencies, passing nil for stdout and stderr, which connects
	// them to the file descriptor for the null device (os.DevNull).
	ptesting.YarnInstallMutex.Lock()
	defer ptesting.YarnInstallMutex.Unlock()
	out := iotest.LogWriter(t)
	bin, err := Install(t.Context(), AutoPackageManager, pkgdir, production, out, out)
	require.NoError(t, err)
	assert.Equal(t, packageManager, bin)
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
			require.NoError(t, err)

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
