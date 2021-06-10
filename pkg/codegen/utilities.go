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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

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

func (ss StringSet) Any() bool {
	return len(ss) > 0
}

func (ss StringSet) Delete(s string) {
	delete(ss, s)
}

func (ss StringSet) Has(s string) bool {
	_, ok := ss[s]
	return ok
}

func (ss StringSet) SortedValues() []string {
	values := make([]string, 0, len(ss))
	for v := range ss {
		values = append(values, v)
	}
	sort.Strings(values)
	return values
}

// Contains returns true if all elements of the subset are also present in the current set. It also returns true
// if subset is empty.
func (ss StringSet) Contains(subset StringSet) bool {
	for v := range subset {
		if !ss.Has(v) {
			return false
		}
	}
	return true
}

// Subtract returns a new string set with all elements of the current set that are not present in the other set.
func (ss StringSet) Subtract(other StringSet) StringSet {
	result := NewStringSet()
	for v := range ss {
		if !other.Has(v) {
			result.Add(v)
		}
	}
	return result
}

type Set map[interface{}]struct{}

func (s Set) Add(v interface{}) {
	s[v] = struct{}{}
}

func (s Set) Has(v interface{}) bool {
	_, ok := s[v]
	return ok
}

// SortedKeys returns a sorted list of keys for the given map. The map's key type must be of kind string.
func SortedKeys(m interface{}) []string {
	mv := reflect.ValueOf(m)

	contract.Require(mv.Type().Kind() == reflect.Map, "m")
	contract.Require(mv.Type().Key().Kind() == reflect.String, "m")

	keys := make([]string, mv.Len())
	for i, k := range mv.MapKeys() {
		keys[i] = k.String()
	}
	sort.Strings(keys)

	return keys
}

// CleanDir removes all existing files from a directory except those in the exclusions list.
// Note: The exclusions currently don't function recursively, so you cannot exclude a single file
// in a subdirectory, only entire subdirectories. This function will need improvements to be able to
// target that use-case.
func CleanDir(dirPath string, exclusions StringSet) error {
	subPaths, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return err
	}

	if len(subPaths) > 0 {
		for _, path := range subPaths {
			if !exclusions.Has(path.Name()) {
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
