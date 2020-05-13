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
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/stretchr/testify/assert"
)

func TestIngoreSimple(t *testing.T) {
	doArchiveTest(t,
		fileContents{name: ".pulumiignore", contents: []byte("node_modules/pulumi/"), shouldRetain: true},
		fileContents{name: "included.txt", shouldRetain: true},
		fileContents{name: "node_modules/included.txt", shouldRetain: true},
		fileContents{name: "node_modules/pulumi/excluded.txt", shouldRetain: false},
		fileContents{name: "node_modules/pulumi/excluded/excluded.txt", shouldRetain: false})
}

func TestIgnoreNegate(t *testing.T) {
	doArchiveTest(t,
		fileContents{name: ".pulumiignore", contents: []byte("/*\n!/foo\n/foo/*\n!/foo/bar"), shouldRetain: false},
		fileContents{name: "excluded.txt", shouldRetain: false},
		fileContents{name: "foo/excluded.txt", shouldRetain: false},
		fileContents{name: "foo/baz/exlcuded.txt", shouldRetain: false},
		fileContents{name: "foo/bar/included.txt", shouldRetain: true})
}

func TestNested(t *testing.T) {
	doArchiveTest(t,
		fileContents{name: ".pulumiignore", contents: []byte("node_modules/pulumi/"), shouldRetain: true},
		fileContents{name: "node_modules/.pulumiignore", contents: []byte("@pulumi/"), shouldRetain: true},
		fileContents{name: "included.txt", shouldRetain: true},
		fileContents{name: "node_modules/included.txt", shouldRetain: true},
		fileContents{name: "node_modules/pulumi/excluded.txt", shouldRetain: false},
		fileContents{name: "node_modules/@pulumi/pulumi-cloud/excluded.txt", shouldRetain: false})
}

func doArchiveTest(t *testing.T, files ...fileContents) {
	archive, err := archiveContents(files...)
	assert.NoError(t, err)

	fmt.Println(archive.Len())

	r, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	assert.NoError(t, err)

	checkFiles(t, files, r.File)
}

func archiveContents(files ...fileContents) (*bytes.Buffer, error) {
	dir, err := ioutil.TempDir("", "archive-test")
	if err != nil {
		return nil, err
	}

	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
	}()

	for _, file := range files {
		err := os.MkdirAll(path.Dir(path.Join(dir, file.name)), 0755)
		if err != nil {
			return nil, err
		}

		err = ioutil.WriteFile(path.Join(dir, file.name), file.contents, 0644)
		if err != nil {
			return nil, err
		}
	}

	return Process(dir, false)
}

func checkFiles(t *testing.T, expected []fileContents, actual []*zip.File) {
	var expectedFiles []string
	var actualFiles []string

	for _, f := range expected {
		if f.shouldRetain {
			expectedFiles = append(expectedFiles, f.name)
		}
	}

	for _, f := range actual {

		// Ignore any directories (we only care that the files themselves are correct)
		if strings.HasSuffix(f.Name, "/") {
			continue
		}

		actualFiles = append(actualFiles, f.Name)
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
