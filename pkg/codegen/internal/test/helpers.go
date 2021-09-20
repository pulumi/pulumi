// Copyright 2016-2021, Pulumi Corporation.
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

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// GenPkgSignature corresponds to the shape of the codegen GeneratePackage functions.
type GenPkgSignature func(string, *schema.Package, map[string][]byte) (map[string][]byte, error)

// GeneratePackageFilesFromSchema loads a schema and generates files using the provided GeneratePackage function.
func GeneratePackageFilesFromSchema(schemaPath string, genPackageFunc GenPkgSignature) (map[string][]byte, error) {
	// Read in, decode, and import the schema.
	schemaBytes, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(schemaPath)

	var pkgSpec schema.PackageSpec
	if ext == ".yaml" || ext == ".yml" {
		err = yaml.Unmarshal(schemaBytes, &pkgSpec)
	} else {
		err = json.Unmarshal(schemaBytes, &pkgSpec)
	}
	if err != nil {
		return nil, err
	}

	pkg, err := schema.ImportSpec(pkgSpec, nil)
	if err != nil {
		return nil, err
	}

	return genPackageFunc("test", pkg, nil)
}

// LoadFiles loads the provided list of files from a directory.
func LoadFiles(dir, lang string, files []string) (map[string][]byte, error) {
	result := map[string][]byte{}
	for _, file := range files {
		fileBytes, err := ioutil.ReadFile(filepath.Join(dir, lang, file))
		if err != nil {
			return nil, err
		}

		result[file] = fileBytes
	}

	return result, nil
}

// Recursively loads files from a directory into the `fs` map. Ignores
// entries that match `ignore(path)==true`, also skips descending into
// directories that are ignored. This is useful for example to avoid
// `node_modules`.
func loadDirectory(fs map[string][]byte, root, path string, ignore func(path string) bool) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, e := range entries {
		entryPath := filepath.Join(path, e.Name())
		relativeEntryPath := entryPath[len(root)+1:]
		baseName := filepath.Base(relativeEntryPath)
		if ignore != nil && (ignore(relativeEntryPath) || ignore(baseName)) {
			// pass
		} else if e.IsDir() {
			if err = loadDirectory(fs, root, entryPath, ignore); err != nil {
				return err
			}
		} else {
			contents, err := os.ReadFile(entryPath)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(relativeEntryPath)
			fs[name] = contents
		}
	}

	return nil
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err == nil {
		return true, nil
	}

	return false, err
}

// Reads `.sdkcodegenignore` file if present to use as loadDirectory ignore func.
func loadIgnoreMap(dir string) (func(path string) bool, error) {

	load1 := func(dir string, ignoredPathSet map[string]bool) error {
		p := filepath.Join(dir, ".sdkcodegenignore")

		gotIgnore, err := PathExists(p)
		if err != nil {
			return err
		}

		if gotIgnore {
			contents, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, s := range strings.Split(string(contents), "\n") {
				s = strings.Trim(s, " \r\n\t")

				if s != "" {
					ignoredPathSet[s] = true
				}
			}
		}
		return nil
	}

	loadAll := func(dir string, ignoredPathSet map[string]bool) error {
		for {
			atTopOfRepo, err := PathExists(filepath.Join(dir, ".git"))
			if err != nil {
				return err
			}

			err = load1(dir, ignoredPathSet)
			if err != nil {
				return err
			}

			if atTopOfRepo || dir == "." {
				return nil
			}

			dir = filepath.Dir(dir)
		}
	}

	ignoredPathSet := make(map[string]bool)
	err := loadAll(dir, ignoredPathSet)
	if err != nil {
		return nil, err
	}

	return func(path string) bool {
		path = strings.ReplaceAll(path, "\\", "/")
		_, ignoredPath := ignoredPathSet[path]
		return ignoredPath
	}, nil
}

// LoadBaseline loads the contents of the given baseline directory.
func LoadBaseline(dir, lang string) (map[string][]byte, error) {
	dir = filepath.Join(dir, lang)

	fs := map[string][]byte{}

	ignore, err := loadIgnoreMap(dir)
	if err != nil {
		return nil, err
	}

	if err := loadDirectory(fs, dir, dir, ignore); err != nil {
		return nil, err
	}
	return fs, nil
}

// ValidateFileEquality compares maps of files for equality.
func ValidateFileEquality(t *testing.T, actual, expected map[string][]byte) bool {
	ok := true
	for name, file := range expected {
		_, inActual := actual[name]
		if inActual {
			if !assert.Equal(t, string(file), string(actual[name]), name) {
				t.Logf("%s did not agree", name)
				ok = false
			}
		} else {
			t.Logf("File %s was expected but is missing from the actual fileset", name)
			ok = false
		}
	}
	for name := range actual {
		if _, inExpected := expected[name]; !inExpected {
			t.Logf("File %s from the actual fileset was not expected", name)
			ok = false
		}
	}
	return ok
}

// If PULUMI_ACCEPT is set, writes out actual output to the expected
// file set, so we can continue enjoying golden tests without manually
// modifying the expected output.
func RewriteFilesWhenPulumiAccept(t *testing.T, dir, lang string, actual map[string][]byte) bool {
	if os.Getenv("PULUMI_ACCEPT") == "" {
		return false
	}

	baseline := filepath.Join(dir, lang)

	// Remove the baseline directory's current contents.
	_, err := os.ReadDir(baseline)
	switch {
	case err == nil:
		err = os.RemoveAll(baseline)
		require.NoError(t, err)
	case os.IsNotExist(err):
		// OK
	default:
		require.NoError(t, err)
	}

	for file, bytes := range actual {
		relPath := filepath.FromSlash(file)
		path := filepath.Join(dir, lang, relPath)

		err := writeFileEnsuringDir(path, bytes)
		require.NoError(t, err)
	}

	return true
}

// Useful for populating code-generated destination
// `codeDir=$dir/$lang` with extra manually written files such as the
// unit test files. These files are copied from `$dir/$lang-extras`
// folder if present.
func CopyExtraFiles(t *testing.T, dir, lang string) {
	codeDir := filepath.Join(dir, lang)
	extrasDir := filepath.Join(dir, fmt.Sprintf("%s-extras", lang))
	gotExtras, err := PathExists(extrasDir)

	if !gotExtras {
		return
	}

	if err != nil {
		t.Error(err)
		return
	}

	filepath.Walk(extrasDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(extrasDir, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(codeDir, relPath)

		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		writeFileEnsuringDir(destPath, bytes)
		t.Logf("Copied %s to %s", path, destPath)
		return nil
	})
}

func writeFileEnsuringDir(path string, bytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && !os.IsExist(err) {
		return err
	}

	return ioutil.WriteFile(path, bytes, 0600)
}

// CheckAllFilesGenerated ensures that the set of expected and actual files generated
// are exactly equivalent.
func CheckAllFilesGenerated(t *testing.T, actual, expected map[string][]byte) {
	seen := map[string]bool{}
	for x := range expected {
		seen[x] = true
	}
	for a := range actual {
		assert.Contains(t, seen, a, "Unexpected file generated: %s", a)
		if seen[a] {
			delete(seen, a)
		}
	}

	for s := range seen {
		assert.Fail(t, "No content generated for expected file %s", s)
	}
}

// Validates a transformer on a single file.
func ValidateFileTransformer(
	t *testing.T,
	inputFile string,
	expectedOutputFile string,
	transformer func(reader io.Reader, writer io.Writer) error) {

	reader, err := os.Open(inputFile)
	if err != nil {
		t.Error(err)
		return
	}

	var buf bytes.Buffer

	err = transformer(reader, &buf)
	if err != nil {
		t.Error(err)
		return
	}

	actualBytes := buf.Bytes()

	if os.Getenv("PULUMI_ACCEPT") != "" {
		err := ioutil.WriteFile(expectedOutputFile, actualBytes, 0600)
		if err != nil {
			t.Error(err)
			return
		}
	}

	actual := map[string][]byte{expectedOutputFile: actualBytes}

	expectedBytes, err := ioutil.ReadFile(expectedOutputFile)
	if err != nil {
		t.Error(err)
		return
	}

	expected := map[string][]byte{expectedOutputFile: expectedBytes}

	ValidateFileEquality(t, actual, expected)
}
