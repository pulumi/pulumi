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

package oci

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
)

func TestParseRequiredPackages(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		m, err := ParseRequiredPackages([]byte(
			`{"plugins":[{"resource":true,"name":"random","version":"4.16.0"},` +
				`{"resource":true,"name":"aws","version":"6.0.0","server":"https://get.example.test"}]}`))
		require.NoError(t, err)
		require.Len(t, m.Plugins, 2)
		assert.Equal(t, plugin.PulumiPluginJSON{Resource: true, Name: "random", Version: "4.16.0"}, m.Plugins[0])
		assert.Equal(t, "https://get.example.test", m.Plugins[1].Server)
	})

	t.Run("empty and whitespace are empty manifests, not errors", func(t *testing.T) {
		t.Parallel()
		for _, in := range [][]byte{nil, {}, []byte("  \n\t ")} {
			m, err := ParseRequiredPackages(in)
			require.NoError(t, err)
			assert.Empty(t, m.Plugins)
		}
	})

	t.Run("malformed is an error", func(t *testing.T) {
		t.Parallel()
		_, err := ParseRequiredPackages([]byte(`{"plugins": [`))
		require.Error(t, err)
	})
}

func TestRequiredPackagesSummary(t *testing.T) {
	t.Parallel()
	m := RequiredPackagesManifest{Plugins: []plugin.PulumiPluginJSON{
		{Resource: true, Name: "random", Version: "4.16.0"},
		{Resource: true, Name: "unversioned"},
	}}
	assert.Equal(t, "random@4.16.0, unversioned", m.Summary())
}

// tarOf builds the single-regular-file tar archive `docker cp <container>:<file> -`
// streams to stdout, so ReadImageFile can be unit-tested without a daemon.
func tarOf(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
	}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

func TestReadImageFileCreatesCopiesReaps(t *testing.T) {
	t.Parallel()
	content := []byte(`{"plugins":[]}` + "\n")
	fake := &fakeRunner{respond: func(args []string) (string, string, error) {
		switch args[0] {
		case "create":
			return "container-xyz", "", nil
		case "cp":
			return string(tarOf(t, "required-packages.json", content)), "", nil
		case "rm":
			return "", "", nil
		}
		return "", "", errors.New("unexpected call")
	}}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	data, err := pm.ReadImageFile(t.Context(), "prog:latest", "/pulumi/required-packages.json")
	require.NoError(t, err)
	assert.Equal(t, content, data, "the file's exact bytes are recovered from the tar stream")

	require.Len(t, fake.calls, 3, "create, cp, rm")
	assert.Equal(t, []string{"create", "--label", "com.pulumi.pod=p1", "prog:latest"}, fake.calls[0])
	assert.Equal(t, []string{"cp", "container-xyz:/pulumi/required-packages.json", "-"}, fake.calls[1])
	assert.Equal(t, []string{"rm", "-f", "container-xyz"}, fake.calls[2], "the throwaway container is reaped")
}

func TestReadImageFileMissingFileIsAbsence(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{respond: func(args []string) (string, string, error) {
		switch args[0] {
		case "create":
			return "container-xyz", "", nil
		case "cp":
			return "", "Error response from daemon: Could not find the file " +
				"/pulumi/required-packages.json in container container-xyz", errors.New("exit status 1")
		}
		return "", "", nil // rm
	}}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	data, err := pm.ReadImageFile(t.Context(), "prog:latest", "/pulumi/required-packages.json")
	require.NoError(t, err)
	assert.Nil(t, data)
}

func TestReadImageFileRealErrorPropagates(t *testing.T) {
	t.Parallel()
	// A daemon-level failure at create time is not "file absent" and must surface.
	fake := &fakeRunner{respond: func([]string) (string, string, error) {
		return "", "Cannot connect to the Docker daemon at unix:///var/run/docker.sock",
			errors.New("exit status 1")
	}}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	_, err := pm.ReadImageFile(t.Context(), "prog:latest", "/pulumi/required-packages.json")
	require.Error(t, err)
}

func TestReadRequiredPackagesFromImage(t *testing.T) {
	t.Parallel()
	manifest := []byte(`{"plugins":[{"resource":true,"name":"random","version":"4.16.0"}]}`)
	fake := &fakeRunner{respond: func(args []string) (string, string, error) {
		switch args[0] {
		case "create":
			return "cid", "", nil
		case "cp":
			return string(tarOf(t, "required-packages.json", manifest)), "", nil
		}
		return "", "", nil // rm
	}}
	pm := NewDockerPodManager("p1", withRunner(fake.run))

	m, err := ReadRequiredPackagesFromImage(t.Context(), pm, "prog:latest")
	require.NoError(t, err)
	require.Len(t, m.Plugins, 1)
	assert.Equal(t, "random@4.16.0", m.Summary())
}

// writePluginMetadata writes a package's Pulumi plugin metadata into a fake
// dependency tree, in the carrier the given language uses: Node embeds it in
// package.json under "pulumi"; Go/Python use a standalone pulumi-plugin.json.
func writePluginMetadata(t *testing.T, root, relDir, carrier, contents string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(relDir))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, carrier), []byte(contents), 0o600))
}

// TestNodeRequiredPackagesGenerator exercises the Node generator head-on: because the
// manifest is best-effort (a wrong one fails silently, masked by lazy discovery), the
// generator is the piece with real correctness risk, so it is tested against a fixture
// that HAS real provider deps — not a bare program that registers nothing.
func TestNodeRequiredPackagesGenerator(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join("smoketest", "templates", "oci-nodejs", "oci-required-packages.cjs")
	requireScript(t, script)

	// Node carries the metadata in package.json's "pulumi" field. Cover every branch
	// of name derivation:
	//   @pulumi/random: explicit pulumi.name              -> "random"
	//   @scope/thirdparty: explicit pulumi.name + server  -> "thirdparty" (+ server)
	//   @pulumi/scoped-noname: NO pulumi.name, @pulumi     -> "scoped-noname"@2.0.0
	//   @vendor/nameless: NO pulumi.name, third-party      -> OMITTED (can't be named)
	//   @pulumi/pulumi: pulumi.resource=false             -> excluded (not a provider)
	//   left-pad: no pulumi metadata                      -> excluded
	root := t.TempDir()
	nm := filepath.Join(root, "node_modules")
	writePluginMetadata(t, nm, "@pulumi/random", "package.json",
		`{"name":"@pulumi/random","version":"4.16.0","pulumi":{"resource":true,"name":"random","version":"4.16.0"}}`)
	writePluginMetadata(t, nm, "@scope/thirdparty", "package.json",
		`{"name":"@scope/thirdparty","version":"1.0.0","pulumi":`+
			`{"resource":true,"name":"thirdparty","version":"1.0.0","server":"https://get.example.test"}}`)
	writePluginMetadata(t, nm, "@pulumi/scoped-noname", "package.json",
		`{"name":"@pulumi/scoped-noname","version":"2.0.0","pulumi":{"resource":true}}`)
	writePluginMetadata(t, nm, "@vendor/nameless", "package.json",
		`{"name":"@vendor/nameless","version":"9.9.9","pulumi":{"resource":true}}`)
	writePluginMetadata(t, nm, "@pulumi/pulumi", "package.json",
		`{"name":"@pulumi/pulumi","version":"3.100.0","pulumi":{"resource":false}}`)
	writePluginMetadata(t, nm, "left-pad", "package.json", `{"name":"left-pad","version":"1.3.0"}`)

	m := runGenerator(t, node, script, nm)
	require.Len(t, m.Plugins, 3, "the three nameable resource plugins, sorted; @vendor/nameless omitted")
	assert.Equal(t, plugin.PulumiPluginJSON{Resource: true, Name: "random", Version: "4.16.0"}, m.Plugins[0])
	assert.Equal(t, plugin.PulumiPluginJSON{Resource: true, Name: "scoped-noname", Version: "2.0.0"}, m.Plugins[1],
		"an @pulumi-scoped provider with no pulumi.name derives its unscoped name")
	assert.Equal(t, plugin.PulumiPluginJSON{
		Resource: true, Name: "thirdparty", Version: "1.0.0", Server: "https://get.example.test",
	}, m.Plugins[2])
}

// TestGoRequiredPackagesGenerator and TestPythonRequiredPackagesGenerator exercise the
// Go/Python generators, which read the SAME PulumiPluginJSON content but from a
// standalone pulumi-plugin.json FILE (the real carrier difference vs Node). Both walk a
// dependency-tree root, so the fixture mirrors that.
func TestGoRequiredPackagesGenerator(t *testing.T) {
	t.Parallel()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not available")
	}
	script := filepath.Join("smoketest", "templates", "oci-go", "ocigen", "main.go")
	requireScript(t, script)

	// The generator scans a module dir at pulumi-language-go's candidate paths
	// (module root, go/, go/*, */). The fixture treats `root` as one module dir:
	//   pulumi-plugin.json        -> "random" (candidate: module root)
	//   go/tls/pulumi-plugin.json -> "tls"    (candidate: go/*)
	//   other/pulumi-plugin.json  -> resource:false, excluded (candidate: */)
	//   internal/testdata/mock/pulumi-plugin.json -> IGNORED: too deep to be a
	//     candidate. This is the over-report guard — the pulumi SDK ships exactly
	//     such testdata fixtures, and a naive recursive walk would report them.
	root := t.TempDir()
	writePluginMetadata(t, root, ".", "pulumi-plugin.json",
		`{"resource":true,"name":"random","version":"4.16.0"}`)
	writePluginMetadata(t, root, "go/tls", "pulumi-plugin.json",
		`{"resource":true,"name":"tls","version":"5.0.0"}`)
	writePluginMetadata(t, root, "other", "pulumi-plugin.json", `{"resource":false}`)
	writePluginMetadata(t, root, "internal/testdata/mock", "pulumi-plugin.json",
		`{"resource":true,"name":"mock_package","version":"1.2.3"}`)

	m := runGenerator(t, goBin, "run", script, root)
	require.Len(t, m.Plugins, 2, "random + tls; the deep testdata mock_package is not a candidate path")
	assert.Equal(t, plugin.PulumiPluginJSON{Resource: true, Name: "random", Version: "4.16.0"}, m.Plugins[0])
	assert.Equal(t, plugin.PulumiPluginJSON{Resource: true, Name: "tls", Version: "5.0.0"}, m.Plugins[1])
}

func TestPythonRequiredPackagesGenerator(t *testing.T) {
	t.Parallel()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	script := filepath.Join("smoketest", "templates", "oci-python", "oci-required-packages.py")
	requireScript(t, script)

	root := t.TempDir()
	writePluginMetadata(t, root, "pulumi_random", "pulumi-plugin.json",
		`{"resource":true,"name":"random","version":"4.16.0"}`)
	writePluginMetadata(t, root, "pulumi", "pulumi-plugin.json", `{"resource":false}`)
	writePluginMetadata(t, root, "left_pad", "__init__.py", `# not a pulumi package`)

	m := runGenerator(t, py, script, root)
	require.Len(t, m.Plugins, 1)
	assert.Equal(t, plugin.PulumiPluginJSON{Resource: true, Name: "random", Version: "4.16.0"}, m.Plugins[0])
}

func requireScript(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("generator script not found: %v", err)
	}
}

// runGenerator runs a generator (interpreter + leading args, then the fake dependency
// root and an output path), reads the emitted manifest, and parses it.
func runGenerator(t *testing.T, bin string, argsBeforeRoot ...string) RequiredPackagesManifest {
	t.Helper()
	out := filepath.Join(t.TempDir(), "manifest.json")
	args := append(append([]string{}, argsBeforeRoot...), out)
	cmd := exec.Command(bin, args...)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run())

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	m, err := ParseRequiredPackages(data)
	require.NoError(t, err)
	return m
}
