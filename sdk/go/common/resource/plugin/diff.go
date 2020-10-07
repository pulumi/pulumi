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

package plugin

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
)

type stringSet map[string]struct{}

func newStringSet(strings []string) stringSet {
	ss := stringSet{}
	for _, s := range strings {
		ss[s] = struct{}{}
	}
	return ss
}

func (ss stringSet) has(s string) bool {
	_, ok := ss[s]
	return ok
}

type differ struct {
	replacePaths   stringSet
	ignorePaths    stringSet
	inputDiffPaths stringSet

	result map[string]PropertyDiff
}

func (d *differ) ignore(path string) bool {
	return d.ignorePaths.has(path)
}

func (d *differ) propertyDiff(path string, kind DiffKind) PropertyDiff {
	if d.replacePaths.has(path) {
		switch kind {
		case DiffAdd:
			kind = DiffAddReplace
		case DiffDelete:
			kind = DiffDeleteReplace
		case DiffUpdate:
			kind = DiffUpdateReplace
		}
	}
	return PropertyDiff{
		Kind:      kind,
		InputDiff: d.inputDiffPaths.has(path),
	}
}

func (d *differ) addValue(path string, value resource.PropertyValue) {
	d.result[path] = d.propertyDiff(path, DiffAdd)
}

func (d *differ) deleteValue(path string, value resource.PropertyValue) {
	d.result[path] = d.propertyDiff(path, DiffDelete)
}

func (d *differ) flattenValueDiff(path string, diff resource.ValueDiff) {
	if d.ignore(path) {
		return
	}

	switch {
	case diff.Array != nil:
		d.flattenArrayDiff(path, diff.Array)
	case diff.Object != nil:
		d.flattenObjectDiff(path, diff.Object)
	default:
		d.result[path] = d.propertyDiff(path, DiffUpdate)
	}
}

func (d *differ) flattenArrayDiff(path string, diff *resource.ArrayDiff) {
	if d.ignore(path) {
		return
	}

	elementPath := func(path string, index int) string {
		return fmt.Sprintf("%v[%v]", path, index)
	}

	for index, v := range diff.Adds {
		d.addValue(elementPath(path, index), v)
	}
	for index, v := range diff.Deletes {
		d.deleteValue(elementPath(path, index), v)
	}
	for index, diff := range diff.Updates {
		d.flattenValueDiff(elementPath(path, index), diff)
	}
}

func (d *differ) flattenObjectDiff(path string, diff *resource.ObjectDiff) {
	if d.ignore(path) {
		return
	}

	elementPath := func(path string, key resource.PropertyKey) string {
		if path != "" {
			path += "."
		}
		if strings.ContainsRune(string(key), '.') {
			return fmt.Sprintf("%v[\"%v\"]", path, key)
		}
		return fmt.Sprintf("%v%v", path, key)
	}

	for key, v := range diff.Adds {
		d.addValue(elementPath(path, key), v)
	}
	for key, v := range diff.Deletes {
		d.deleteValue(elementPath(path, key), v)
	}
	for key, diff := range diff.Updates {
		d.flattenValueDiff(elementPath(path, key), diff)
	}
}

type DetailedDiffOptions struct {
	ReplacePaths   []string
	IgnorePaths    []string
	InputDiffPaths []string
}

func DetailedDiff(base, changed resource.PropertyMap, options DetailedDiffOptions) map[string]PropertyDiff {
	if base == nil {
		base = resource.PropertyMap{}
	}
	if changed == nil {
		changed = resource.PropertyMap{}
	}

	diff := base.Diff(changed)
	if diff == nil {
		return nil
	}

	differ := &differ{
		ignorePaths:    newStringSet(options.IgnorePaths),
		replacePaths:   newStringSet(options.ReplacePaths),
		inputDiffPaths: newStringSet(options.InputDiffPaths),
		result:         map[string]PropertyDiff{},
	}
	differ.flattenObjectDiff("", diff)
	return differ.result
}
