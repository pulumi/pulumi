// Copyright 2026, Pulumi Corporation.
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

package display

import (
	"cmp"
	"context"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// ObjectDiffJSON is the JSON projection of a resource.ObjectDiff: the same
// structure the `--diff` view renders as text, emitted when `--diff` and
// `--output json` are combined.
type ObjectDiffJSON struct {
	// Adds holds the properties present only in the new value.
	Adds map[string]any `json:"adds,omitempty"`
	// Deletes holds the properties present only in the old value.
	Deletes map[string]any `json:"deletes,omitempty"`
	// Sames holds the properties that did not change.
	Sames map[string]any `json:"sames,omitempty"`
	// Updates holds the properties that changed, keyed by property name.
	Updates map[string]ValueDiffJSON `json:"updates,omitempty"`
	// Hidden lists property paths whose diffs were withheld from this object.
	Hidden []string `json:"hidden,omitempty"`
}

// ValueDiffJSON is the JSON projection of a resource.ValueDiff. Exactly one of
// Array, Object, or the Old/New pair is populated, matching the shape of the
// value that changed.
type ValueDiffJSON struct {
	// Old is the previous value, for a change between two non-composite values.
	Old *any `json:"old,omitempty"`
	// New is the replacement value, for a change between two non-composite values.
	New *any `json:"new,omitempty"`
	// Array is the element-wise diff, when both values are arrays.
	Array *ArrayDiffJSON `json:"array,omitempty"`
	// Object is the property-wise diff, when both values are objects.
	Object *ObjectDiffJSON `json:"object,omitempty"`
}

// ArrayDiffJSON is the JSON projection of a resource.ArrayDiff. Its maps are
// keyed by array index.
type ArrayDiffJSON struct {
	// Adds holds the elements present only in the new array.
	Adds map[int]any `json:"adds,omitempty"`
	// Deletes holds the elements present only in the old array.
	Deletes map[int]any `json:"deletes,omitempty"`
	// Sames holds the elements that did not change.
	Sames map[int]any `json:"sames,omitempty"`
	// Updates holds the elements that changed.
	Updates map[int]ValueDiffJSON `json:"updates,omitempty"`
}

// diffJSONEncoder converts property values to their JSON representation, using
// the same blinding encrypter the streaming `--json` event display uses so that
// secrets do not leak into the output.
type diffJSONEncoder struct {
	ctx         context.Context
	enc         config.Encrypter
	showSecrets bool
}

func newDiffJSONEncoder(showSecrets bool) diffJSONEncoder {
	return diffJSONEncoder{
		ctx:         context.TODO(),
		enc:         config.BlindingCrypter,
		showSecrets: showSecrets,
	}
}

func (e diffJSONEncoder) value(v resource.PropertyValue) *any {
	out, err := stack.SerializePropertyValue(e.ctx, v, e.enc, e.showSecrets)
	contract.IgnoreError(err)
	return &out
}

func (e diffJSONEncoder) values(m resource.PropertyMap) map[string]any {
	out, err := stack.SerializeProperties(e.ctx, m, e.enc, e.showSecrets)
	contract.IgnoreError(err)
	return out
}

func (e diffJSONEncoder) elements(m map[int]resource.PropertyValue) map[int]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[int]any, len(m))
	for i, v := range m {
		out[i] = *e.value(v)
	}
	return out
}

func (e diffJSONEncoder) objectDiff(diff *resource.ObjectDiff) *ObjectDiffJSON {
	if diff == nil {
		return nil
	}
	out := &ObjectDiffJSON{
		Adds:    e.values(diff.Adds),
		Deletes: e.values(diff.Deletes),
		Sames:   e.values(diff.Sames),
	}
	if len(diff.Updates) > 0 {
		out.Updates = make(map[string]ValueDiffJSON, len(diff.Updates))
		for k, v := range diff.Updates {
			out.Updates[string(k)] = e.valueDiff(v)
		}
	}
	return out
}

func (e diffJSONEncoder) valueDiff(diff resource.ValueDiff) ValueDiffJSON {
	switch {
	case diff.Array != nil:
		return ValueDiffJSON{Array: e.arrayDiff(diff.Array)}
	case diff.Object != nil:
		return ValueDiffJSON{Object: e.objectDiff(diff.Object)}
	default:
		return ValueDiffJSON{Old: e.value(diff.Old), New: e.value(diff.New)}
	}
}

func (e diffJSONEncoder) arrayDiff(diff *resource.ArrayDiff) *ArrayDiffJSON {
	out := &ArrayDiffJSON{
		Adds:    e.elements(diff.Adds),
		Deletes: e.elements(diff.Deletes),
		Sames:   e.elements(diff.Sames),
	}
	if len(diff.Updates) > 0 {
		out.Updates = make(map[int]ValueDiffJSON, len(diff.Updates))
		for i, v := range diff.Updates {
			out.Updates[i] = e.valueDiff(v)
		}
	}
	return out
}

// stepObjectDiff computes the property diff a step displays, mirroring the
// selection renderDiff makes: the provider's detailed diff when it supplied
// one, and a structural diff of the step's old and new state otherwise. It also
// returns the top-level keys the diff should be restricted to, if any, and the
// property paths whose diffs are hidden.
func stepObjectDiff(
	step engine.StepEventMetadata,
) (*resource.ObjectDiff, []resource.PropertyKey, []resource.PropertyPath) {
	// An OpSame may carry a metadata change (e.g. protect) but never a property diff.
	if step.Op == deploy.OpSame {
		return nil, nil, nil
	}

	if step.DetailedDiff != nil && step.Old != nil && step.New != nil {
		diff, hidden := engine.TranslateDetailedDiff(&step, false /* refresh */)
		return diff, nil, hidden
	}

	var hidePaths []resource.PropertyPath
	if step.New != nil {
		hidePaths = step.New.HideDiffs
	} else if step.Old != nil {
		hidePaths = step.Old.HideDiffs
	}

	var hidden []resource.PropertyPath
	opts := []resource.DiffOption{
		resource.IgnoreKeyFunc(resource.IsInternalPropertyKey),
		resource.IgnorePathFunc(func(path resource.PropertyPath) bool {
			for _, v := range hidePaths {
				if v.Contains(path) {
					hidden = append(hidden, v)
					return true
				}
			}
			return false
		}),
	}

	old, new := step.Old, step.New
	switch {
	case old == nil && new == nil:
		return nil, nil, nil
	case old == nil:
		news := new.Inputs
		if len(new.Outputs) > 0 {
			news = new.Outputs
		}
		return resource.PropertyMap{}.DiffWithOptions(news, opts...), nil, hidden
	case new == nil:
		return old.Inputs.DiffWithOptions(nil, opts...), nil, hidden
	case len(new.Outputs) > 0 && step.Op != deploy.OpImport && step.Op != deploy.OpImportReplacement:
		return old.Outputs.DiffWithOptions(new.Outputs, opts...), nil, hidden
	default:
		return old.Inputs.DiffWithOptions(new.Inputs, opts...), step.Diffs, hidden
	}
}

// filterObjectDiff restricts a diff to the given top-level keys, matching the
// filtering printObjectDiff applies when a step reports its changed keys.
func filterObjectDiff(diff *resource.ObjectDiff, include []resource.PropertyKey) *resource.ObjectDiff {
	if diff == nil || include == nil {
		return diff
	}

	keep := make(map[resource.PropertyKey]bool, len(include))
	for _, k := range include {
		keep[k] = true
	}

	filterMap := func(m resource.PropertyMap) resource.PropertyMap {
		out := make(resource.PropertyMap, len(m))
		for k, v := range m {
			if keep[k] {
				out[k] = v
			}
		}
		return out
	}

	out := &resource.ObjectDiff{
		Adds:    filterMap(diff.Adds),
		Deletes: filterMap(diff.Deletes),
		Sames:   filterMap(diff.Sames),
		Updates: make(map[resource.PropertyKey]resource.ValueDiff, len(diff.Updates)),
	}
	for k, v := range diff.Updates {
		if keep[k] {
			out.Updates[k] = v
		}
	}
	return out
}

// stepDiffJSON renders a step's property diff as JSON, or nil when the step has
// no diff to show.
func stepDiffJSON(step engine.StepEventMetadata, showSecrets bool) *ObjectDiffJSON {
	diff, include, hidden := stepObjectDiff(step)
	diff = filterObjectDiff(diff, include)
	if diff == nil && len(hidden) == 0 {
		return nil
	}

	out := newDiffJSONEncoder(showSecrets).objectDiff(diff)
	if out == nil {
		out = &ObjectDiffJSON{}
	}
	for _, p := range sortedUniquePaths(hidden) {
		out.Hidden = append(out.Hidden, p.String())
	}
	return out
}

// sortedUniquePaths sorts property paths and removes duplicates, so that hidden
// paths are reported deterministically.
func sortedUniquePaths(paths []resource.PropertyPath) []resource.PropertyPath {
	slices.SortFunc(paths, func(a, b resource.PropertyPath) int {
		return cmp.Compare(a.String(), b.String())
	})
	return slices.CompactFunc(paths, func(a, b resource.PropertyPath) bool {
		return a.String() == b.String()
	})
}
