package provider

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
)

func normalizeValue(v reflect.Value) reflect.Value {
	// Dereferences pointers and interface values to their concrete value.
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:

		// Normalize numbers to float64.
		return v.Convert(reflect.TypeOf(float64(0)))
	default:
		return v
	}
}

type differ struct {
	ignoreChanges map[string]struct{}
	result        map[string]plugin.PropertyDiff
}

func (d *differ) setDiff(path string, kind plugin.DiffKind, isImmutable bool) {
	if isImmutable {
		switch kind {
		case plugin.DiffAdd:
			kind = plugin.DiffAddReplace
		case plugin.DiffUpdate:
			kind = plugin.DiffUpdateReplace
		case plugin.DiffDelete:
			kind = plugin.DiffDeleteReplace
		}
	}

	d.result[path] = plugin.PropertyDiff{Kind: kind, InputDiff: true}
}

func (d *differ) addValue(path string, v reflect.Value, isImmutable bool) error {
	v = normalizeValue(v)

	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := d.addValue(listElementPath(path, i), v.Index(i), isImmutable); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("%v: map keys must be strings", path)
		}

		for iter := v.MapRange(); iter.Next(); {
			if err := d.addValue(objectPropertyPath(path, iter.Key().String()), iter.Value(), isImmutable); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			desc, err := getFieldDesc(t.Field(i))
			if err != nil {
				return fmt.Errorf("internal error: %v", err)
			}
			if desc == nil {
				continue
			}

			if err := d.addValue(objectPropertyPath(path, desc.name), v.Field(i), isImmutable || desc.immutable); err != nil {
				return err
			}
		}
		return nil
	default:
		d.setDiff(path, plugin.DiffAdd, isImmutable)
		return nil
	}
}

func (d *differ) removeValue(path string, v reflect.Value, isImmutable bool) error {
	v = normalizeValue(v)

	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := d.removeValue(listElementPath(path, i), v.Index(i), isImmutable); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("%v: map keys must be strings", path)
		}

		for iter := v.MapRange(); iter.Next(); {
			if err := d.removeValue(objectPropertyPath(path, iter.Key().String()), iter.Value(), isImmutable); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			desc, err := getFieldDesc(t.Field(i))
			if err != nil {
				return fmt.Errorf("internal error: %v", err)
			}
			if desc == nil {
				continue
			}

			if err := d.removeValue(objectPropertyPath(path, desc.name), v.Field(i), isImmutable || desc.immutable); err != nil {
				return err
			}
		}
		return nil
	default:
		d.setDiff(path, plugin.DiffDelete, isImmutable)
		return nil
	}
}

func (d *differ) replaceValue(path string, current, new reflect.Value, isImmutable bool) error {
	if err := d.removeValue(path, current, isImmutable); err != nil {
		return err
	}
	return d.addValue(path, new, isImmutable)
}

func (d *differ) diffLists(path string, current, new reflect.Value, isImmutable bool) error {
	i := 0
	for ; i < current.Len() && i < new.Len(); i++ {
		if err := d.diffValues(listElementPath(path, i), current.Index(i), new.Index(i), isImmutable); err != nil {
			return err
		}
	}
	for ; i < current.Len(); i++ {
		if err := d.removeValue(listElementPath(path, i), current.Index(i), isImmutable); err != nil {
			return err
		}
	}
	for ; i < new.Len(); i++ {
		if err := d.addValue(listElementPath(path, i), new.Index(i), isImmutable); err != nil {
			return err
		}
	}
	return nil
}

func (d *differ) diffObjects(path string, current, new reflect.Value, isImmutable bool) error {
	currentObject, err := makeObject(current)
	if err != nil {
		return err
	}
	newObject, err := makeObject(new)
	if err != nil {
		return nil
	}

	for iter := currentObject.iterate(); iter.next(); {
		current, desc := iter.value()

		propertyPath := objectPropertyPath(path, desc.name)
		if new, newDesc, ok := newObject.index(desc.name); ok {
			if err := d.diffValues(propertyPath, current, new, isImmutable || desc.immutable || newDesc.immutable); err != nil {
				return err
			}
		} else {
			if err := d.removeValue(propertyPath, current, isImmutable || desc.immutable); err != nil {
				return err
			}
		}
	}

	for iter := newObject.iterate(); iter.next(); {
		new, desc := iter.value()
		if _, _, ok := currentObject.index(desc.name); !ok {
			if err := d.addValue(objectPropertyPath(path, desc.name), new, isImmutable); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *differ) diffValues(path string, current, new reflect.Value, isImmutable bool) error {
	if _, ok := d.ignoreChanges[path]; ok {
		return nil
	}

	current, new = normalizeValue(current), normalizeValue(new)

	switch current.Kind() {
	case reflect.Bool:
		if new.Kind() != reflect.Bool {
			return d.replaceValue(path, current, new, isImmutable)
		}
		if current.Bool() != new.Bool() {
			d.setDiff(path, plugin.DiffUpdate, isImmutable)
		}
		return nil
	case reflect.Float64:
		if new.Kind() != reflect.Float64 {
			return d.replaceValue(path, current, new, isImmutable)
		}
		if current.Float() != new.Float() {
			d.setDiff(path, plugin.DiffUpdate, isImmutable)
		}
		return nil
	case reflect.String:
		if new.Kind() != reflect.String {
			return d.replaceValue(path, current, new, isImmutable)
		}
		if current.String() != new.String() {
			d.setDiff(path, plugin.DiffUpdate, isImmutable)
		}
		return nil
	case reflect.Array, reflect.Slice:
		if new.Kind() != reflect.Array && new.Kind() != reflect.Slice {
			return d.replaceValue(path, current, new, isImmutable)
		}
		return d.diffLists(path, current, new, isImmutable)
	case reflect.Map, reflect.Struct:
		if new.Kind() != reflect.Map && new.Kind() != reflect.Struct {
			return d.replaceValue(path, current, new, isImmutable)
		}
		return d.diffObjects(path, current, new, isImmutable)
	default:
		return fmt.Errorf("internal error: %v has unsupported type %v", path, current.Type())
	}
}

func Diff(currentArgs, newArgs interface{}, ignoreChanges []string) (plugin.DiffResult, error) {
	currentV, newV := reflect.ValueOf(currentArgs), reflect.ValueOf(newArgs)
	if currentV.Type() != newV.Type() {
		return plugin.DiffResult{}, fmt.Errorf("internal error: current and new args must have the same type (%T vs. %T)", currentArgs, newArgs)
	}

	ignoredPaths := map[string]struct{}{}
	for _, p := range ignoreChanges {
		path, err := resource.ParsePropertyPath(p)
		if err != nil {
			continue
		}
		ignoredPaths[path.String()] = struct{}{}
	}

	d := differ{
		ignoreChanges: ignoredPaths,
		result:        map[string]plugin.PropertyDiff{},
	}
	if err := d.diffValues("", currentV, newV, false); err != nil {
		return plugin.DiffResult{}, err
	}
	return plugin.DiffResult{DetailedDiff: d.result}, nil
}
