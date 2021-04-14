package display

import (
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// getProperty fetches the child property with the indicated key from the given property value. If the key does not
// exist, it returns an empty `PropertyValue`.
func getProperty(key interface{}, v resource.PropertyValue) resource.PropertyValue {
	switch {
	case v.IsArray():
		index, ok := key.(int)
		if !ok || index < 0 || index >= len(v.ArrayValue()) {
			return resource.PropertyValue{}
		}
		return v.ArrayValue()[index]
	case v.IsObject():
		k, ok := key.(string)
		if !ok {
			return resource.PropertyValue{}
		}
		return v.ObjectValue()[resource.PropertyKey(k)]
	case v.IsComputed() || v.IsOutput() || v.IsSecret():
		// We consider the contents of these values opaque and return them as-is, as we cannot know whether or not the
		// value will or does contain an element with the given key.
		return v
	default:
		return resource.PropertyValue{}
	}
}

// addDiff inserts a diff of the given kind at the given path into the parent ValueDiff.
//
// If the path consists of a single element, a diff of the indicated kind is inserted directly. Otherwise, if the
// property named by the first element of the path exists in both parents, we snip off the first element of the path
// and recurse into the property itself. If the property does not exist in one parent or the other, the diff kind is
// disregarded and the change is treated as either an Add or a Delete.
func addDiff(path resource.PropertyPath, kind plugin.DiffKind, parent *resource.ValueDiff,
	oldParent, newParent resource.PropertyValue) {

	contract.Require(len(path) > 0, "len(path) > 0")

	element := path[0]

	old, new := getProperty(element, oldParent), getProperty(element, newParent)

	switch element := element.(type) {
	case int:
		if parent.Array == nil {
			parent.Array = &resource.ArrayDiff{
				Adds:    make(map[int]resource.PropertyValue),
				Deletes: make(map[int]resource.PropertyValue),
				Sames:   make(map[int]resource.PropertyValue),
				Updates: make(map[int]resource.ValueDiff),
			}
		}

		// For leaf diffs, the provider tells us exactly what to record. For other diffs, we will derive the
		// difference from the old and new property values.
		if len(path) == 1 {
			switch kind {
			case plugin.DiffAdd, plugin.DiffAddReplace:
				parent.Array.Adds[element] = new
			case plugin.DiffDelete, plugin.DiffDeleteReplace:
				parent.Array.Deletes[element] = old
			case plugin.DiffUpdate, plugin.DiffUpdateReplace:
				valueDiff := resource.ValueDiff{Old: old, New: new}
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
			parent.Object = &resource.ObjectDiff{
				Adds:    make(resource.PropertyMap),
				Deletes: make(resource.PropertyMap),
				Sames:   make(resource.PropertyMap),
				Updates: make(map[resource.PropertyKey]resource.ValueDiff),
			}
		}

		e := resource.PropertyKey(element)
		if len(path) == 1 {
			switch kind {
			case plugin.DiffAdd, plugin.DiffAddReplace:
				parent.Object.Adds[e] = new
			case plugin.DiffDelete, plugin.DiffDeleteReplace:
				parent.Object.Deletes[e] = old
			case plugin.DiffUpdate, plugin.DiffUpdateReplace:
				valueDiff := resource.ValueDiff{Old: old, New: new}
				if d := old.Diff(new); d != nil {
					valueDiff = *d
				}
				parent.Object.Updates[e] = valueDiff
			default:
				contract.Failf("unexpected diff kind %v", kind)
			}
		} else {
			switch {
			case old.IsNull() && !new.IsNull():
				parent.Object.Adds[e] = new
			case !old.IsNull() && new.IsNull():
				parent.Object.Deletes[e] = old
			default:
				ed := parent.Object.Updates[e]
				addDiff(path[1:], kind, &ed, old, new)
				parent.Object.Updates[e] = ed
			}
		}
	default:
		contract.Failf("unexpected path element type: %T", element)
	}
}

// translateDetailedDiff converts the detailed diff stored in the step event into an ObjectDiff that is appropriate
// for display.
func translateDetailedDiff(step engine.StepEventMetadata) *resource.ObjectDiff {
	contract.Assert(step.DetailedDiff != nil)

	// The rich diff is presented as a list of simple JS property paths and corresponding diffs. We translate this to
	// an ObjectDiff by iterating the list and inserting ValueDiffs that reflect the changes in the detailed diff. Old
	// values are always taken from a step's Outputs; new values are always taken from its Inputs.

	var diff resource.ValueDiff
	for path, pdiff := range step.DetailedDiff {
		elements, err := resource.ParsePropertyPath(path)
		if err != nil {
			elements = []interface{}{path}
		}

		olds := resource.NewObjectProperty(step.Old.Outputs)
		if pdiff.InputDiff {
			olds = resource.NewObjectProperty(step.Old.Inputs)
		}
		addDiff(elements, pdiff.Kind, &diff, olds, resource.NewObjectProperty(step.New.Inputs))
	}

	return diff.Object
}
