// Copyright 2019, Pulumi Corporation.
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

package engine

import (
	"cmp"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// getProperty fetches the child property with the indicated key from the given property value. The second result
// reports whether the key exists; if it does not, an empty property.Value is returned.
func getProperty(key any, v property.Value) (property.Value, bool) {
	switch {
	case v.IsArray():
		index, ok := key.(int)
		arr := v.AsArray()
		if !ok || index < 0 || index >= arr.Len() {
			return property.Value{}, false
		}
		return arr.Get(index), true
	case v.IsMap():
		k, ok := key.(string)
		if !ok {
			return property.Value{}, false
		}
		return v.AsMap().GetOk(k)
	case v.IsComputed() || v.Secret():
		// We consider the contents of these values opaque and return them as-is, as we cannot know whether or not the
		// value will or does contain an element with the given key.
		return v, true
	default:
		return property.Value{}, false
	}
}

// valueOrUnknown returns v if the property it came from exists, and an unknown value otherwise.
func valueOrUnknown(v property.Value, exists bool) property.Value {
	if exists {
		return v
	}
	return property.New(property.Computed)
}

// addDiff inserts a diff of the given kind at the given path into the parent ValueDiff.
//
// If the path consists of a single element, a diff of the indicated kind is inserted directly. Otherwise, if the
// property named by the first element of the path exists in both parents, we snip off the first element of the path
// and recurse into the property itself. If the property does not exist in one parent or the other, the diff kind is
// disregarded and the change is treated as either an Add or a Delete.
func addDiff(path resource.PropertyPath, kind plugin.DiffKind, parent *property.ValueDiff,
	oldParent, newParent property.Value,
) {
	contract.Requiref(len(path) > 0, "path", "must not be empty")

	element := path[0]

	old, hasOld := getProperty(element, oldParent)
	new, hasNew := getProperty(element, newParent)

	switch element := element.(type) {
	case int:
		if parent.Array == nil {
			parent.Array = &property.ArrayDiff{
				Adds:    make(map[int]property.Value),
				Deletes: make(map[int]property.Value),
				Sames:   make(map[int]property.Value),
				Updates: make(map[int]property.ValueDiff),
			}
		}

		// For leaf diffs, the provider tells us exactly what to record. For other diffs, we will derive the
		// difference from the old and new property values.
		if len(path) == 1 {
			switch kind {
			case plugin.DiffAdd, plugin.DiffAddReplace:
				parent.Array.Adds[element] = valueOrUnknown(new, hasNew)
			case plugin.DiffDelete, plugin.DiffDeleteReplace:
				parent.Array.Deletes[element] = valueOrUnknown(old, hasOld)
			case plugin.DiffUpdate, plugin.DiffUpdateReplace:
				valueDiff := property.ValueDiff{Old: old, New: new}
				if d := old.Diff(new); d != nil {
					valueDiff = *d
				}
				parent.Array.Updates[element] = valueDiff
			default:
				contract.Failf("unexpected diff kind %v", kind)
			}
		} else {
			switch {
			case old.IsNull() && !new.IsNull():
				parent.Array.Adds[element] = new
			case !old.IsNull() && new.IsNull():
				parent.Array.Deletes[element] = old
			default:
				ed := parent.Array.Updates[element]
				addDiff(path[1:], kind, &ed, old, new)
				parent.Array.Updates[element] = ed
			}
		}
	case string:
		if parent.Object == nil {
			parent.Object = &property.ObjectDiff{
				Adds:    make(map[string]property.Value),
				Deletes: make(map[string]property.Value),
				Sames:   make(map[string]property.Value),
				Updates: make(map[string]property.ValueDiff),
			}
		}

		if len(path) == 1 {
			switch kind {
			case plugin.DiffAdd, plugin.DiffAddReplace:
				parent.Object.Adds[element] = valueOrUnknown(new, hasNew)
			case plugin.DiffDelete, plugin.DiffDeleteReplace:
				parent.Object.Deletes[element] = valueOrUnknown(old, hasOld)
			case plugin.DiffUpdate, plugin.DiffUpdateReplace:
				valueDiff := property.ValueDiff{Old: old, New: new}
				if d := old.Diff(new); d != nil {
					valueDiff = *d
				}
				parent.Object.Updates[element] = valueDiff
			default:
				contract.Failf("unexpected diff kind %v", kind)
			}
		} else {
			switch {
			case old.IsNull() && !new.IsNull():
				parent.Object.Adds[element] = new
			case !old.IsNull() && new.IsNull():
				parent.Object.Deletes[element] = old
			default:
				ed := parent.Object.Updates[element]
				addDiff(path[1:], kind, &ed, old, new)
				parent.Object.Updates[element] = ed
			}
		}
	default:
		contract.Failf("unexpected path element type: %T", element)
	}
}

// TranslateDetailedDiff converts the detailed diff stored in the step event into an ObjectDiff that is appropriate
// for display.
//
// The second returned argument is the list of hidden diffs.
func TranslateDetailedDiff(step *StepEventMetadata, refresh bool) (*property.ObjectDiff, []resource.PropertyPath) {
	contract.Assertf(step.DetailedDiff != nil, "%v step has no detailed diff", step.Op)

	// The rich diff is presented as a list of simple JS property paths and corresponding diffs. We translate this to
	// an ObjectDiff by iterating the list and inserting ValueDiffs that reflect the changes in the detailed diff. Old
	// values are always taken from a step's Outputs; new values are always taken from its Inputs.

	var hiddenPaths []resource.PropertyPath
	var hiddenDiffs []resource.PropertyPath
	if step.New != nil {
		hiddenPaths = step.New.HideDiffs
	} else if step.Old != nil {
		hiddenPaths = step.Old.HideDiffs
	}

	var diff property.ValueDiff
diffs:
	for path, pdiff := range step.DetailedDiff {
		elements, err := resource.ParsePropertyPath(path)
		if err != nil {
			elements = []any{path}
		}

		for _, hiddenPath := range hiddenPaths {
			if hiddenPath.Contains(elements) {
				hiddenDiffs = append(hiddenDiffs, hiddenPath)
				continue diffs
			}
		}

		olds := property.New(step.Old.Outputs)
		if pdiff.InputDiff {
			olds = property.New(step.Old.Inputs)
		}

		news := property.New(step.New.Inputs)
		if refresh && !pdiff.InputDiff {
			news = property.New(step.New.Outputs)
		}

		addDiff(elements, pdiff.Kind, &diff, olds, news)
	}

	// Ensure that our paths are unique and sorted
	slices.SortFunc(hiddenDiffs, func(a, b resource.PropertyPath) int {
		return cmp.Compare(a.String(), b.String())
	})
	hiddenDiffs = slices.CompactFunc(hiddenDiffs, func(a, b resource.PropertyPath) bool {
		return a.String() == b.String()
	})

	return diff.Object, hiddenDiffs
}
