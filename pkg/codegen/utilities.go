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

package codegen

import (
	"os"
	"path/filepath"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// StringSet is a set of strings.
//
// Deprecated: use [mapset.Set] from github.com/deckarep/golang-set/v2 instead.
type StringSet = mapset.Set[string]

// NewStringSet creates a new StringSet from the given values.
//
// Deprecated: use [mapset.NewSet] from github.com/deckarep/golang-set/v2 instead.
func NewStringSet(values ...string) StringSet {
	return mapset.NewSet(values...)
}

// Set is a set of arbitrary comparable values.
//
// Deprecated: use [mapset.Set] from github.com/deckarep/golang-set/v2 instead.
type Set = mapset.Set[any]

// CleanDir removes all existing files from a directory except those in the exclusions list.
// Note: The exclusions currently don't function recursively, so you cannot exclude a single file
// in a subdirectory, only entire subdirectories. This function will need improvements to be able to
// target that use-case.
func CleanDir(dirPath string, exclusions mapset.Set[string]) error {
	subPaths, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	if len(subPaths) > 0 {
		for _, path := range subPaths {
			if !exclusions.Contains(path.Name()) {
				err = os.RemoveAll(filepath.Join(dirPath, path.Name()))
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

var commonEnumNameReplacements = map[string]string{
	"*": "Asterisk",
	"0": "Zero",
	"1": "One",
	"2": "Two",
	"3": "Three",
	"4": "Four",
	"5": "Five",
	"6": "Six",
	"7": "Seven",
	"8": "Eight",
	"9": "Nine",
}

func ExpandShortEnumName(name string) string {
	if replacement, ok := commonEnumNameReplacements[name]; ok {
		return replacement
	}
	return name
}

// A simple in memory file system.
type Fs map[string][]byte

// Add a new file to the Fs.
//
// Panic if the file is a duplicate.
func (fs Fs) Add(path string, contents []byte) {
	_, has := fs[path]
	contract.Assertf(!has, "duplicate file: %s", path)
	fs[path] = contents
}

// Check if two packages are the same.
func PkgEquals(p1, p2 schema.PackageReference) bool {
	if p1 == p2 {
		return true
	} else if p1 == nil || p2 == nil {
		return false
	}

	if p1.Name() != p2.Name() {
		return false
	}

	v1, v2 := p1.Version(), p2.Version()
	if v1 == v2 {
		return true
	} else if v1 == nil || v2 == nil {
		return false
	}
	return v1.Equals(*v2)
}

// A .gitattributes file allows us to mark filepath patterns as containing generated content. This enables git tools to
// treat these files differently, for example GitHub will show these files as collapsed by default.
func GenGitAttributesFile() []byte {
	return []byte("* linguist-generated\n")
}
