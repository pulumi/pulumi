// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package lumidl

import (
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
)

// PropertyOptionsTag is the field tag the IDL compiler uses to find property options.
const PropertyOptionsTag = "pulumi"

// PropertyOptions represents a parsed field tag, controlling how properties are treated.
type PropertyOptions struct {
	Name     string // the property name to emit into the package.
	Optional bool   // true if this is an optional property.
	Replaces bool   // true if changing this property triggers a replacement of this resource.
	In       bool   // true if this is part of the resource's input, but not its output, properties.
	Out      bool   // true if the property is part of the resource's output, rather than input, properties.
}

// ParsePropertyOptions parses a tag into a structured set of options.
func ParsePropertyOptions(tag string) PropertyOptions {
	opts := PropertyOptions{}
	if lumi, has := reflect.StructTag(tag).Lookup(PropertyOptionsTag); has {
		// The first element is the name; all others are optional flags.  All are delimited by commas.
		if keys := strings.Split(lumi, ","); len(keys) > 0 {
			opts.Name = keys[0]
			for _, key := range keys[1:] {
				switch key {
				case "optional":
					opts.Optional = true
				case "replaces":
					opts.Replaces = true
				case "in":
					opts.In = true
				case "out":
					opts.Out = true
				default:
					cmdutil.Diag().Errorf(diag.Message("unrecognized tag `pulumi:\"%v\"`"), key)
				}
			}
		}
	}
	return opts
}
