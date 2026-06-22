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

package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// InstallDependencies installs the project's dependencies in e.CWD via `pulumi install`, with the locally-built core
// SDK swapped in so tests exercise local SDK changes.
func (e *Environment) InstallDependencies() {
	e.Helper()

	runtime := detectRuntime(e.CWD)
	if runtime == "" {
		e.Fatalf("InstallDependencies: could not detect runtime in %s", e.CWD)
	}

	if runtime == "nodejs" {
		configureNodejsCoreSDK(e.T, e.CWD) // add the SDK to package.json, it will be installed via `pulumi install`
	}

	e.RunCommand("pulumi", "install")

	if runtime == "python" {
		// For Python we install the core SDK into the virtual environment, on top of whatever is already there. This
		// runs `uv pip ...` to install the core SDK, which is very fast and does not modify pyproject.toml or
		// requirements.txt.
		installPythonCoreSDK(e.T, e.CWD)
	}
}

func detectRuntime(dir string) string {
	for _, manifest := range []string{"Pulumi.yaml", "PulumiPlugin.yaml"} {
		if proj, ok := readProjectYAML(filepath.Join(dir, manifest)); ok && proj.Runtime.Name != "" {
			return proj.Runtime.Name
		}
	}
	if fileExists(filepath.Join(dir, "package.json")) {
		return "nodejs"
	}
	if fileExists(filepath.Join(dir, "requirements.txt")) ||
		fileExists(filepath.Join(dir, "pyproject.toml")) {
		return "python"
	}
	return ""
}

type projectYAML struct {
	Runtime struct {
		Name    string         `yaml:"name"`
		Options map[string]any `yaml:"options"`
	} `yaml:"runtime"`
}

func readProjectYAML(path string) (projectYAML, bool) {
	var proj projectYAML
	data, err := os.ReadFile(path)
	if err != nil {
		return proj, false
	}
	if err := yaml.Unmarshal(data, &proj); err != nil {
		return proj, false
	}
	return proj, true
}

// InstallDependencies installs dir's dependencies (npm for Node.js, uv for Python) with the locally-built core SDK
// swapped in so tests exercise local SDK changes. The runtime is detected from dir. Unlike the Environment method
// of the same name, this installs the package manager's dependencies directly (no `pulumi install`), for installing
// provider and component fixtures that aren't run as the Environment's main project.
func InstallDependencies(t *testing.T, dir string) {
	t.Helper()
	switch detectRuntime(dir) {
	case "nodejs":
		installNodejsDependencies(t, dir)
	case "python":
		installPythonDependencies(t, dir)
	default:
		t.Fatalf("InstallDependencies: could not detect runtime in %s", dir)
	}
}

func installNodejsDependencies(t *testing.T, dir string) {
	t.Helper()

	restoreAtCleanup(t, dir, "package.json")
	configureNodejsCoreSDK(t, dir)

	retry(t, dir, "npm install", func() ([]byte, error) {
		cmd := exec.Command("npm", "install")
		cmd.Dir = dir
		return cmd.CombinedOutput()
	})
}

// configureNodejsCoreSDK points dir's @pulumi/pulumi at the locally-built SDK so that the subsequent npm install
// (whether run directly or by `pulumi install`) exercises local SDK changes. It pins @pulumi/pulumi to the local
// SDK tarball and forces every transitive @pulumi/pulumi to resolve to that same tarball via `overrides`.
func configureNodejsCoreSDK(t *testing.T, dir string) {
	t.Helper()
	spec := "file:" + localCoreSDKTarball(t)

	pkgPath := filepath.Join(dir, "package.json")
	pkg := map[string]any{}
	if data, err := os.ReadFile(pkgPath); err == nil {
		require.NoError(t, json.Unmarshal(data, &pkg))
	}
	// Drop any @pulumi/pulumi peerDependency: we add it as a direct dependency below, and npm rejects an
	// `override` for a package also declared as a peer dependency with a different spec.
	if peers, ok := pkg["peerDependencies"].(map[string]any); ok {
		delete(peers, "@pulumi/pulumi")
		if len(peers) == 0 {
			delete(pkg, "peerDependencies")
		}
	}
	setStringMapEntry(pkg, "dependencies", "@pulumi/pulumi", spec)
	setStringMapEntry(pkg, "overrides", "@pulumi/pulumi", spec)
	data, err := json.MarshalIndent(pkg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pkgPath, append(data, '\n'), 0o600))
}

// coreSDKTarball packs the locally-built @pulumi/pulumi SDK into a tarball
var coreSDKTarball = sync.OnceValues(func() (string, error) {
	coreSDK, err := repoPath("sdk", "nodejs", "bin")
	if err != nil {
		return "", err
	}
	dest, err := os.MkdirTemp("", "pulumi-core-sdk-")
	if err != nil {
		return "", err
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("npm", "pack", coreSDK, "--pack-destination", dest)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("npm pack %s: %w\n%s", coreSDK, err, stderr.String())
	}
	// `npm pack` prints the tarball filename as the last line of stdout; diagnostics go to stderr.
	out := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	name := strings.TrimSpace(out[len(out)-1])
	if name == "" {
		return "", fmt.Errorf("npm pack %s produced no artifact name", coreSDK)
	}
	return filepath.Join(dest, name), nil
})

func localCoreSDKTarball(t *testing.T) string {
	t.Helper()
	path, err := coreSDKTarball()
	require.NoError(t, err)
	return path
}

func setStringMapEntry(pkg map[string]any, section, key, value string) {
	m, _ := pkg[section].(map[string]any)
	if m == nil {
		m = map[string]any{}
	}
	m[key] = value
	pkg[section] = m
}

func retry(t *testing.T, dir, what string, fn func() ([]byte, error)) {
	t.Helper()
	var out []byte
	var err error
	for i := range 3 {
		out, err = fn()
		if err == nil {
			return
		}
		t.Logf("%s in %s failed (attempt %d/3): %v\noutput: %s", what, dir, i+1, err, out)
	}
	t.Fatalf("%s in %s failed after 3 retries: %v\noutput: %s", what, dir, err, out)
}

// installPythonDependencies installs the python dependencies in dir and then layers the locally-built core SDK on top.
func installPythonDependencies(t *testing.T, dir string) {
	t.Helper()

	dir, err := filepath.Abs(dir)
	require.NoError(t, err)

	coreSDK, err := repoPath("sdk", "python")
	require.NoError(t, err)

	if fileExists(filepath.Join(dir, "pyproject.toml")) {
		restoreAtCleanup(t, dir, "pyproject.toml", "uv.lock")
		uvRun(t, dir, "add", coreSDK)
		return
	}

	// requirements.txt project or a bare plugin (PulumiPlugin.yaml + __main__.py)
	venvDir := pythonVirtualenv(dir)
	uvRun(t, dir, "venv", "--quiet", "--allow-existing", venvDir)
	if fileExists(filepath.Join(dir, "requirements.txt")) {
		uvRun(t, dir, "pip", "install", "--python", venvDir, "-r", filepath.Join(dir, "requirements.txt"))
	}
	installPythonCoreSDK(t, dir)
}

func installPythonCoreSDK(t *testing.T, dir string) {
	t.Helper()
	coreSDK, err := repoPath("sdk", "python")
	require.NoError(t, err)
	uvRun(t, dir, "pip", "install", "--python", pythonVirtualenv(dir), coreSDK)
}

// restoreAtCleanup snapshots the named files in dir and, when the test finishes, restores each to its original
// contents (or removes it if it didn't exist). Use it around installs that must mutate a tracked fixture's manifest
// while the test runs (so the runtime keeps the linked SDK) but should leave the fixture pristine afterwards.
func restoreAtCleanup(t *testing.T, dir string, names ...string) {
	t.Helper()
	type snapshot struct {
		path    string
		data    []byte
		existed bool
	}
	snaps := make([]snapshot, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		snaps = append(snaps, snapshot{path: path, data: data, existed: err == nil})
	}
	t.Cleanup(func() {
		// The directory may already be gone (e.g. a temp test environment that was deleted); there's then nothing
		// tracked to keep pristine, so skip.
		if _, err := os.Stat(dir); err != nil {
			return
		}
		for _, s := range snaps {
			if s.existed {
				require.NoError(t, os.WriteFile(s.path, s.data, 0o600))
			} else {
				require.NoError(t, os.RemoveAll(s.path))
			}
		}
	})
}

func uvRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("uv", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "`uv %s` in %s failed: %s", strings.Join(args, " "), dir, string(out))
}

func pythonVirtualenv(dir string) string {
	venvName := ".venv"
	for _, manifest := range []string{"Pulumi.yaml", "PulumiPlugin.yaml"} {
		if proj, ok := readProjectYAML(filepath.Join(dir, manifest)); ok {
			if v, _ := proj.Runtime.Options["virtualenv"].(string); v != "" {
				venvName = v
				break
			}
		}
	}
	if filepath.IsAbs(venvName) {
		return venvName
	}
	return filepath.Join(dir, venvName)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// RepoRoot returns the absolute path to the root of the pulumi repository, via `git rev-parse --show-toplevel`.
func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("finding repo root via git: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// repoPath joins parts onto the repository root
func repoPath(parts ...string) (string, error) {
	root, err := RepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{root}, parts...)...), nil
}
