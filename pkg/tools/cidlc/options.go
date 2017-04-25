// Copyright 2017 Pulumi, Inc. All rights reserved.

package cidlc

import (
	"reflect"
	"strings"

	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/util/cmdutil"
)

// PropertyOptionsTag is the field tag the IDL compiler uses to find property options.
const PropertyOptionsTag = "coco"

// PropertyOptions represents a parsed field tag, controlling how properties are treated.
type PropertyOptions struct {
	Name     string // the property name to emit into the package.
	Optional bool   // true if this is an optional property.
	Replaces bool   // true if changing this property triggers a replacement of this resource.
	Out      bool   // true if the property is part of the resource's output, rather than input, properties.
}

// ParsePropertyOptions parses a tag into a structured set of options.
func ParsePropertyOptions(tag string) PropertyOptions {
	opts := PropertyOptions{}
	if coco, has := reflect.StructTag(tag).Lookup(PropertyOptionsTag); has {
		// The first element is the name; all others are optional flags.  All are delimited by commas.
		if keys := strings.Split(coco, ","); len(keys) > 0 {
			opts.Name = keys[0]
			for _, key := range keys[1:] {
				switch key {
				case "optional":
					opts.Optional = true
				case "replaces":
					opts.Replaces = true
				case "out":
					opts.Out = true
				default:
					cmdutil.Sink().Errorf(diag.Message("unrecognized tag `coco:\"%v\"`", key))
				}
			}
		}
	}
	return opts
}
