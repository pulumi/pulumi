// Copyright 2016 Marapongo, Inc. All rights reserved.

// Because of the complex structure of the MuPack and MuIL metadata formats, we cannot rely on the standard JSON
// marshaling and unmarshaling routines.  Instead, we will need to do it mostly "by hand".  This package does that.
package encoding

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/pack"
	//"github.com/marapongo/mu/pkg/pack/ast"
	"github.com/marapongo/mu/pkg/pack/symbols"
	"github.com/marapongo/mu/pkg/util"
)

type object map[string]interface{}
type array []interface{}

// Decode unmarshals the entire contents of the given byte array into a Package object.
func Decode(m encoding.Marshaler, b []byte) (*pack.Package, error) {
	// First convert the whole contents of the metadata into a map.  Although it would be more efficient to walk the
	// token stream, token by token, this allows us to reuse existing YAML packages in addition to JSON ones.
	var tree object
	if err := m.Unmarshal(b, &tree); err != nil {
		return nil, err
	}
	return decodePackage(tree)
}

func noExcess(tree object, ty string, fields ...string) error {
	m := make(map[string]bool)
	for _, f := range fields {
		m[f] = true
	}
	for k := range tree {
		if !m[k] {
			return fmt.Errorf("Unrecognized %v field `%v`", ty, k)
		}
	}
	return nil
}

func newMissing(ty string, field string) error {
	return fmt.Errorf("Missing required %v field `%v`", ty, field)
}

func newWrongType(ty string, field string, expect string, actual string) error {
	msg := fmt.Sprintf("%v `%v` must be a `%v`", ty, field, expect)
	if actual != "" {
		msg += fmt.Sprintf(", got `%v`", actual)
	}
	return errors.New(msg)
}

// decodeField decodes primitive fields.  For fields of complex types, we use custom deserialization.
func decodeField(tree object, ty string, field string, target interface{}, req bool) error {
	vdst := reflect.ValueOf(target)
	util.AssertM(vdst.Kind() == reflect.Ptr && !vdst.IsNil() && vdst.Elem().CanSet(),
		"Target must be a non-nil, settable pointer")
	if v, has := tree[field]; has {
		// The field exists; okay, try to map it to the right type.
		vsrc := reflect.ValueOf(v)
		vsrcType := vsrc.Type()
		vdstType := vdst.Type().Elem()

		// So long as the target element is a pointer, we have a pointer to pointer; keep digging through until we
		// bottom out on the non-pointer type that matches the source.  This assumes the source isn't itself a pointer!
		util.Assert(vsrcType.Kind() != reflect.Ptr)
		for vdstType.Kind() == reflect.Ptr {
			vdst = vdst.Elem()
			vdstType = vdstType.Elem()
			if !vdst.Elem().CanSet() {
				// If the pointer is nil, initialize it so we can set it below.
				util.Assert(vdst.IsNil())
				vdst.Set(reflect.New(vdstType))
			}
		}

		// If the source and destination types don't match, after depointerizing the type above, bail right away.
		if vsrcType != vdstType {
			return newWrongType(ty, field, vdstType.Name(), vsrcType.Name())
		}

		// Otherwise, go ahead and copy the value from source to the target.
		vdst.Elem().Set(vsrc)
	} else if req {
		// The field doesn't exist and yet it is required; issue an error.
		return newMissing(ty, field)
	}
	return nil
}

func decodeMetadata(tree object) (*pack.Metadata, error) {
	var meta pack.Metadata
	if err := decodeField(tree, "Metadata", "name", &meta.Name, true); err != nil {
		return nil, err
	}
	if err := decodeField(tree, "Metadata", "description", &meta.Description, false); err != nil {
		return nil, err
	}
	if err := decodeField(tree, "Metadata", "author", &meta.Author, false); err != nil {
		return nil, err
	}
	if err := decodeField(tree, "Metadata", "website", &meta.Website, false); err != nil {
		return nil, err
	}
	if err := decodeField(tree, "Metadata", "license", &meta.License, false); err != nil {
		return nil, err
	}
	return &meta, nil
}

func decodePackage(tree object) (*pack.Package, error) {
	// Ensure only recognized fields are present.
	if err := noExcess(tree,
		"Package", "name", "description", "author", "website", "license", "dependencies", "modules"); err != nil {
		return nil, err
	}

	// Deserialize the informational metadata portion.
	meta, err := decodeMetadata(tree)
	if err != nil {
		return nil, err
	}

	// Now deserialize the dependencies and modules section.
	pack := pack.Package{Metadata: *meta}

	if deps, has := tree["dependencies"]; has {
		if adeps, ok := deps.([]interface{}); ok {
			var tokens []symbols.ModuleToken
			for i, dep := range adeps {
				if sdep, ok := dep.(string); ok {
					tokens = append(tokens, symbols.ModuleToken(sdep))
				} else {
					return nil, newWrongType(
						"Package", fmt.Sprintf("dependencies[%v]", i),
						"string", reflect.ValueOf(dep).Type().Name())
				}
			}
			pack.Dependencies = &tokens
		} else {
			return nil, newWrongType(
				"Package", "dependencies",
				"[]ModuleToken", reflect.ValueOf(deps).Type().Name())
		}
	}

	// TODO: dependencies.
	// TODO: modules.

	return &pack, nil
}
