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

package schema

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func readSchemaFile(file string) (pkgSpec PackageSpec) {
	// Read in, decode, and import the schema.
	schemaBytes, err := ioutil.ReadFile(filepath.Join("..", "internal", "test", "testdata", file))
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(schemaBytes, &pkgSpec); err != nil {
		panic(err)
	}

	return pkgSpec
}

func TestImportSpec(t *testing.T) {
	// Read in, decode, and import the schema.
	pkgSpec := readSchemaFile("kubernetes.json")

	pkg, err := ImportSpec(pkgSpec, nil)
	if err != nil {
		t.Errorf("ImportSpec() error = %v", err)
	}

	for _, r := range pkg.Resources {
		assert.NotNil(t, r.Package, "expected resource %s to have an associated Package", r.Token)
	}
}

var enumTests = []struct {
	filename    string
	shouldError bool
}{
	{"bad-enum-1.json", true},
	{"bad-enum-2.json", true},
	{"bad-enum-3.json", true},
	{"bad-enum-4.json", true},
	{"good-enum-1.json", false},
	{"good-enum-2.json", false},
	{"good-enum-3.json", false},
	{"good-enum-4.json", false},
}

func TestEnums(t *testing.T) {
	for _, tt := range enumTests {
		t.Run(tt.filename, func(t *testing.T) {
			pkgSpec := readSchemaFile(filepath.Join("schema", tt.filename))

			_, err := ImportSpec(pkgSpec, nil)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				if err != nil {
					t.Error(err)
				}
			}
		})
	}
}
