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

package codegen

import (
	"os"
	"path/filepath"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Deprecated: Use "github.com/deckarep/golang-set/v2".Set[T] instead.
type StringSet map[string]struct{}

func NewStringSet(values ...string) StringSet {
	s := StringSet{}
	for _, v := range values {
		s.Add(v)
	}
	return s
}

func (ss StringSet) Add(s string) {
	ss[s] = struct{}{}
}

func (ss StringSet) Has(s string) bool {
	_, ok := ss[s]
	return ok
}

// Except returns a new set with all elements of `a` minus the elements `bs`.
// It's a functional equivalent of `a.Remove(b)`.
func Except[T comparable](a mapset.Set[T], b T, bs ...T) mapset.Set[T] {
	aCopy := a.Clone()
	aCopy.Remove(b)
	for _, b := range bs {
		aCopy.Remove(b)
	}
	return aCopy
}

// SortedValues returns a sorted list of values for the given set.
func SortedValues(m mapset.Set[string]) []string {
	keys := slice.Prealloc[string](m.Cardinality())
	m.Each(func(v string) bool {
		keys = append(keys, v)
		return false
	})
	sort.Strings(keys)
	return keys
}

// SortedKeys returns a sorted list of keys for the given map.
func SortedKeys[T any](m map[string]T) []string {
	keys := slice.Prealloc[string](len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

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
