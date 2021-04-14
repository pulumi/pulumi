// Copyright 2016-2018, Pulumi Corporation.
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
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"

	"github.com/stretchr/testify/assert"
)

func TestIngoreSimple(t *testing.T) {
	doArchiveTest(t,
		fileContents{name: ".gitignore", contents: []byte("node_modules/pulumi/"), shouldRetain: true},
		fileContents{name: "included.txt", shouldRetain: true},
		fileContents{name: "node_modules/included.txt", shouldRetain: true},
		fileContents{name: "node_modules/pulumi/excluded.txt", shouldRetain: false},
		fileContents{name: "node_modules/pulumi/excluded/excluded.txt", shouldRetain: false})
}

func TestIgnoreNegate(t *testing.T) {
	doArchiveTest(t,
		fileContents{name: ".gitignore", contents: []byte("/*\n!/foo\n/foo/*\n!/foo/bar"), shouldRetain: false},
		fileContents{name: "excluded.txt", shouldRetain: false},
		fileContents{name: "foo/excluded.txt", shouldRetain: false},
		fileContents{name: "foo/baz/exlcuded.txt", shouldRetain: false},
		fileContents{name: "foo/bar/included.txt", shouldRetain: true})
}

func TestNested(t *testing.T) {
	doArchiveTest(t,
		fileContents{name: ".gitignore", contents: []byte("node_modules/pulumi/"), shouldRetain: true},
		fileContents{name: "node_modules/.gitignore", contents: []byte("@pulumi/"), shouldRetain: true},
		fileContents{name: "included.txt", shouldRetain: true},
		fileContents{name: "node_modules/included.txt", shouldRetain: true},
		fileContents{name: "node_modules/pulumi/excluded.txt", shouldRetain: false},
		fileContents{name: "node_modules/@pulumi/pulumi-cloud/excluded.txt", shouldRetain: false})
}

func TestTypicalPythonPolicyPackDir(t *testing.T) {
	doArchiveTest(t,
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
	doArchiveTest(t,
		fileContents{name: ".git/HEAD", shouldRetain: false},
		fileContents{name: ".git/objects/00/02ae827766d77ee9e2082fee9adeaae90aff65", shouldRetain: false},
		fileContents{name: "__main__.py", shouldRetain: true},
		fileContents{name: "PulumiPolicy.yaml", shouldRetain: true},
		fileContents{name: "requirements.txt", shouldRetain: true})
}

func doArchiveTest(t *testing.T, files ...fileContents) {
	doTest := func(prefixPathInsideTar string) {
		tarball, err := archiveContents(prefixPathInsideTar, files...)
		assert.NoError(t, err)

		tarReader := bytes.NewReader(tarball)
		gzr, err := gzip.NewReader(tarReader)
		assert.NoError(t, err)
		r := tar.NewReader(gzr)

		checkFiles(t, prefixPathInsideTar, files, r)
	}
	for _, prefix := range []string{"", "package"} {
		doTest(prefix)
	}
}

func archiveContents(prefixPathInsideTar string, files ...fileContents) ([]byte, error) {
	dir, err := ioutil.TempDir("", "archive-test")
	if err != nil {
		return nil, err
	}

	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
	}()

	for _, file := range files {
		name := file.name
		if os.PathSeparator != '/' {
			name = strings.ReplaceAll(name, "/", string(os.PathSeparator))
		}

		err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755)
		if err != nil {
			return nil, err
		}

		err = ioutil.WriteFile(filepath.Join(dir, name), file.contents, 0600)
		if err != nil {
			return nil, err
		}
	}

	return TGZ(dir, prefixPathInsideTar, true /*useDefaultExcludes*/)
}

func checkFiles(t *testing.T, prefixPathInsideTar string, expected []fileContents, r *tar.Reader) {
	var expectedFiles []string
	var actualFiles []string

	for _, f := range expected {
		if f.shouldRetain {
			name := f.name
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
		assert.NoError(t, err)

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
