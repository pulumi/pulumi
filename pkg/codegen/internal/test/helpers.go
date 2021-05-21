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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
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

	var pkgSpec schema.PackageSpec
	err = json.Unmarshal(schemaBytes, &pkgSpec)
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

// ValidateFileEquality compares maps of files for equality.
func ValidateFileEquality(t *testing.T, actual, expected map[string][]byte) {
	for name, file := range expected {
		assert.Contains(t, actual, name)
		assert.Equal(t, string(file), string(actual[name]), name)
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

// If PULUMI_ACCEPT is set, writes out actual output to th expected
// file set, so we can continue enjoying golden tests without manually
// modifying the expected output.
func RewriteFilesWhenPulumiAccept(t *testing.T, dir, lang string, actual map[string][]byte, expected []string) {
	if os.Getenv("PULUMI_ACCEPT") != "" {
		for _, file := range expected {
			bytes := actual[file]
			err := ioutil.WriteFile(filepath.Join(dir, lang, file), bytes, 0600)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
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
