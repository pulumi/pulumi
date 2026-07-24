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

package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

func TestIgnoreSimple(t *testing.T) {
	t.Parallel()

	doArchiveTest(t, ".",
		fileContents{name: ".gitignore", contents: []byte("node_modules/pulumi/"), shouldRetain: true},
		fileContents{name: "included.txt", shouldRetain: true},
		fileContents{name: "node_modules/included.txt", shouldRetain: true},
		fileContents{name: "node_modules/pulumi/excluded.txt", shouldRetain: false},
		fileContents{name: "node_modules/pulumi/excluded/excluded.txt", shouldRetain: false})
}

func TestIgnoreNegate(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipped on Windows: TODO[pulumi/pulumi#8648] handle Windows paths in test logic")
	}

	doArchiveTest(t, ".",
		fileContents{name: ".gitignore", contents: []byte("/*\n!/foo\n/foo/*\n!/foo/bar"), shouldRetain: false},
		fileContents{name: "excluded.txt", shouldRetain: false},
		fileContents{name: "foo/excluded.txt", shouldRetain: false},
		fileContents{name: "foo/baz/exlcuded.txt", shouldRetain: false},
		fileContents{name: "foo/bar/included.txt", shouldRetain: true})
}

func TestNested(t *testing.T) {
	t.Parallel()

	doArchiveTest(t, ".",
		fileContents{name: ".gitignore", contents: []byte("node_modules/pulumi/"), shouldRetain: true},
		fileContents{name: "node_modules/.gitignore", contents: []byte("@pulumi/"), shouldRetain: true},
		fileContents{name: "included.txt", shouldRetain: true},
		fileContents{name: "node_modules/included.txt", shouldRetain: true},
		fileContents{name: "node_modules/pulumi/excluded.txt", shouldRetain: false},
		fileContents{name: "node_modules/@pulumi/pulumi-cloud/excluded.txt", shouldRetain: false})
}

func TestTypicalPythonPolicyPackDir(t *testing.T) {
	t.Parallel()

	doArchiveTest(t, ".",
		fileContents{name: "__main__.py", shouldRetain: true},
		fileContents{name: ".gitignore", contents: []byte("*.pyc\nvenv/\n"), shouldRetain: true},
		fileContents{name: "PulumiPolicy.yaml", shouldRetain: true},
		fileContents{name: "requirements.txt", shouldRetain: true},
		fileContents{name: "venv/bin/activate", shouldRetain: false},
		fileContents{name: "venv/bin/pip", shouldRetain: false},
		fileContents{name: "venv/bin/python", shouldRetain: false},
		fileContents{name: "__pycache__/__main__.cpython-37.pyc", shouldRetain: false})
}

func TestIgnoreContentOfDotGit(t *testing.T) {
	t.Parallel()

	doArchiveTest(t, ".",
		fileContents{name: ".git/HEAD", shouldRetain: false},
		fileContents{name: ".git/objects/00/02ae827766d77ee9e2082fee9adeaae90aff65", shouldRetain: false},
		fileContents{name: "__main__.py", shouldRetain: true},
		fileContents{name: "PulumiPolicy.yaml", shouldRetain: true},
		fileContents{name: "requirements.txt", shouldRetain: true})
}

func TestNestedPath(t *testing.T) {
	t.Parallel()

	doArchiveTest(t, "pkg/",
		fileContents{name: "excluded.txt", shouldRetain: false},
		fileContents{name: "pkg/.gitignore", contents: []byte("node_modules/pulumi/"), shouldRetain: true},
		fileContents{name: "pkg/node_modules/included.txt", shouldRetain: true},
		fileContents{name: "pkg/node_modules/pulumi/excluded.txt", shouldRetain: false},
		fileContents{name: "pkg/node_modules/pulumi/excluded/excluded.txt", shouldRetain: false})
}

func TestIgnoreNestedGitignore(t *testing.T) {
	t.Parallel()

	doArchiveTest(t, "pkg/",
		fileContents{name: ".gitignore", contents: []byte("*.ts"), shouldRetain: false},
		fileContents{name: "excluded.txt", shouldRetain: false},
		fileContents{name: "pkg/.gitignore", contents: []byte("node_modules/pulumi/"), shouldRetain: true},
		fileContents{name: "pkg/node_modules/excluded.ts", shouldRetain: false},
		fileContents{name: "pkg/node_modules/included.txt", shouldRetain: true},
		fileContents{name: "pkg/node_modules/pulumi/excluded.txt", shouldRetain: false},
		fileContents{name: "pkg/node_modules/pulumi/excluded/excluded.txt", shouldRetain: false})
}

// TestIgnorePrecomposesUnicode verifies that a .gitignore pattern authored in
// composed (NFC) form matches a directory whose name is stored decomposed (NFD)
// on disk — mirroring git's core.precomposeunicode. This is macOS-only behavior:
// precomposeUnicode normalizes readdir output to NFC there and is a no-op
// elsewhere, so the test only runs on darwin.
func TestIgnorePrecomposesUnicode(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "darwin" {
		t.Skip("precomposeUnicode only normalizes on macOS")
	}

	// The directory is created on disk in NFD (decomposed) form while the
	// .gitignore pattern uses NFC (precomposed). APFS preserves the exact bytes
	// we write, so readdir hands the name back in NFD; the match then succeeds
	// only because precomposeUnicode brings it back to NFC before matching.
	nfc := norm.NFC.String("café")
	nfd := norm.NFD.String("café")
	require.NotEqual(t, nfc, nfd, "expected NFC and NFD forms to differ")

	doArchiveTest(t, ".",
		fileContents{name: ".gitignore", contents: []byte(nfc + "/"), shouldRetain: true},
		fileContents{name: "included.txt", shouldRetain: true},
		fileContents{name: nfd + "/excluded.txt", shouldRetain: false})
}

func doArchiveTest(t *testing.T, path string, files ...fileContents) {
	doTest := func(prefixPathInsideTar, path string) {
		tarball, err := archiveContents(t, prefixPathInsideTar, path, files...)
		require.NoError(t, err)

		tarReader := bytes.NewReader(tarball)
		gzr, err := gzip.NewReader(tarReader)
		require.NoError(t, err)
		r := tar.NewReader(gzr)

		checkFiles(t, prefixPathInsideTar, path, files, r)
	}
	for _, prefix := range []string{"", "package"} {
		doTest(prefix, path)
	}
}

func archiveContents(t *testing.T, prefixPathInsideTar, path string, files ...fileContents) ([]byte, error) {
	dir := t.TempDir()

	for _, file := range files {
		name := file.name
		if os.PathSeparator != '/' {
			name = strings.ReplaceAll(name, "/", string(os.PathSeparator))
		}

		err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		if err != nil {
			return nil, err
		}

		err = os.WriteFile(filepath.Join(dir, name), file.contents, 0o600)
		if err != nil {
			return nil, err
		}
	}

	return TGZ(filepath.Join(dir, path), prefixPathInsideTar, true /*useDefaultExcludes*/)
}

func checkFiles(t *testing.T, prefixPathInsideTar, path string, expected []fileContents, r *tar.Reader) {
	var expectedFiles []string
	var actualFiles []string

	for _, f := range expected {
		if f.shouldRetain {
			name := f.name
			if path != "." {
				name = strings.Replace(name, path, "", 1)
			}
			if prefixPathInsideTar != "" {
				// Joining with '/' rather than platform-specific `filepath.Join` because we expect
				// the name in the tar to be using '/'.
				name = fmt.Sprintf("%s/%s", prefixPathInsideTar, name)
			}
			expectedFiles = append(expectedFiles, name)
		}
	}

	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		// Ignore anything other than regular files (e.g. directories) since we only care
		// that the files themselves are correct.
		if header.Typeflag != tar.TypeReg {
			continue
		}

		actualFiles = append(actualFiles, header.Name)
	}

	sort.Strings(expectedFiles)
	sort.Strings(actualFiles)

	assert.Equal(t, expectedFiles, actualFiles)
}

type fileContents struct {
	name         string
	contents     []byte
	shouldRetain bool
}

// buildTGZ produces an in-memory .tar.gz containing the given regular-file entries.
func buildTGZ(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range entries {
		typ := tar.TypeReg
		if len(content) == 0 {
			typ = tar.TypeDir
		}

		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o600,
			Size:     int64(len(content)),
			Typeflag: byte(typ),
		}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestExtractTGZ(t *testing.T) {
	t.Parallel()

	tarball := buildTGZ(t, map[string][]byte{
		"file.txt":        []byte("hello"),
		"sub":             {},
		"sub/nested.txt":  []byte("world"),
		"sub/dir/leaf.go": []byte("package main"),
	})

	dest := t.TempDir()
	require.NoError(t, ExtractTGZ(bytes.NewReader(tarball), dest))

	for name, want := range map[string]string{
		"file.txt":        "hello",
		"sub/nested.txt":  "world",
		"sub/dir/leaf.go": "package main",
	} {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
	}
}

func TestExtractTGZRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		entry string
	}{
		{"parent", "../escape.txt"},
		{"nested-parent", "a/../../escape.txt"},
		{"deep-parent", "../../../../etc/passwd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tarball := buildTGZ(t, map[string][]byte{tc.entry: []byte("malicious")})

			// Extract into a nested directory so the parent of dest is itself a temp dir
			// — that way if the guard fails, the escape lands somewhere observable but
			// still inside the test's tempdir tree.
			parent := t.TempDir()
			dest := filepath.Join(parent, "dest")
			require.NoError(t, os.Mkdir(dest, 0o700))

			err := ExtractTGZ(bytes.NewReader(tarball), dest)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "escapes destination directory")

			// Make sure nothing was written outside dest.
			escaped, err := filepath.Glob(filepath.Join(parent, "escape.txt"))
			require.NoError(t, err)
			assert.Empty(t, escaped, "file escaped destination directory")
		})
	}
}

//nolint:paralleltest // t.Chdir() requires sequential test execution
func TestExtractTGZRelativePathWithEscape(t *testing.T) {
	// Test that path traversal is caught even when passing a relative destination path.
	// This ensures the absolute path conversion in ExtractTGZ is working.
	tarball := buildTGZ(t, map[string][]byte{
		"../../../../escape.txt": []byte("malicious"),
	})

	// Create a temp directory and use a relative path.
	parent := t.TempDir()
	dest := filepath.Join(parent, "dest")
	require.NoError(t, os.Mkdir(dest, 0o700))

	t.Chdir(parent)

	// Extract using relative path "dest".
	err := ExtractTGZ(bytes.NewReader(tarball), "dest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes destination directory")

	// Verify no file was written outside dest.
	escaped, err := filepath.Glob("escape.txt")
	require.NoError(t, err)
	assert.Empty(t, escaped, "file escaped destination directory using relative path")
}

func TestExtractTGZSymlink(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipped on Windows: symlink creation requires elevated privileges")
	}

	buffer := &bytes.Buffer{}
	gw := gzip.NewWriter(buffer)
	tw := tar.NewWriter(gw)

	contents := []byte("hello")
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "target.txt",
		Typeflag: tar.TypeReg,
		Mode:     0o600,
		Size:     int64(len(contents)),
	}))
	_, err := tw.Write(contents)
	require.NoError(t, err)

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "link.txt",
		Typeflag: tar.TypeSymlink,
		Linkname: "target.txt",
		Mode:     0o777,
	}))

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	dir := t.TempDir()
	require.NoError(t, ExtractTGZ(buffer, dir))

	linkPath := filepath.Join(dir, "link.txt")
	target, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, "target.txt", target)

	got, err := os.ReadFile(linkPath)
	require.NoError(t, err)
	assert.Equal(t, contents, got)
}

func TestExtractTGZSymlinkEscape(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipped on Windows: symlink creation requires elevated privileges")
	}

	tgzWithSymlink := func(t *testing.T, name, linkname string) io.Reader {
		buffer := &bytes.Buffer{}
		gw := gzip.NewWriter(buffer)
		tw := tar.NewWriter(gw)
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeSymlink,
			Linkname: linkname,
			Mode:     0o777,
		}))
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())
		return buffer
	}

	cases := []struct {
		name     string
		linkname string
		linkpath string
	}{
		{name: "relative parent escape", linkname: "../escape.txt", linkpath: "link.txt"},
		{name: "nested relative escape", linkname: "../../escape.txt", linkpath: "sub/link.txt"},
		{name: "absolute escape", linkname: "/etc/passwd", linkpath: "link.txt"},
		{name: "relative within escape", linkname: "sub/../../../escape.txt", linkpath: "link.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			err := ExtractTGZ(tgzWithSymlink(t, tc.linkpath, tc.linkname), dir)
			require.ErrorContains(t, err, "points outside the extraction directory")

			// Nothing should have been created.
			_, statErr := os.Lstat(filepath.Join(dir, tc.linkpath))
			assert.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}

	t.Run("relative within is allowed", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		err := ExtractTGZ(tgzWithSymlink(t, "link.txt", "sub/../target.txt"), dir)
		require.NoError(t, err)
		target, err := os.Readlink(filepath.Join(dir, "link.txt"))
		require.NoError(t, err)
		assert.Equal(t, "sub/../target.txt", target)
	})
}

func TestTGZFiles(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	manifest := filepath.Join(src, "PulumiPolicy.yaml")
	require.NoError(t, os.WriteFile(manifest, []byte("runtime: executable\n"), 0o600))
	binary := filepath.Join(src, "policy")
	require.NoError(t, os.WriteFile(binary, []byte("binary"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(src, "ignored.txt"), []byte("x"), 0o600))

	tgz, err := TGZFiles([]File{
		{Path: "PulumiPolicy.yaml", Source: manifest, Mode: 0o644},
		{Path: filepath.Join("bin", "policy"), Source: binary, Mode: 0o755},
	}, "package")
	require.NoError(t, err)

	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	require.NoError(t, err)
	reader := tar.NewReader(gz)

	modes := map[string]int64{}
	contents := map[string]string{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		modes[header.Name] = header.Mode
		body, err := io.ReadAll(reader)
		require.NoError(t, err)
		contents[header.Name] = string(body)
	}

	// The archive holds exactly the named files, with the modes the caller asked for rather than
	// the 0o600 the sources happen to carry on disk.
	assert.Equal(t, map[string]int64{
		"package/PulumiPolicy.yaml": 0o644,
		"package/bin/policy":        0o755,
	}, modes)
	assert.Equal(t, "runtime: executable\n", contents["package/PulumiPolicy.yaml"])
	assert.Equal(t, "binary", contents["package/bin/policy"])
}

func TestTGZFilesRejectsNonRegularFile(t *testing.T) {
	t.Parallel()

	_, err := TGZFiles([]File{{Path: "dir", Source: t.TempDir(), Mode: 0o644}}, "")
	assert.ErrorContains(t, err, "is not a regular file")
}
