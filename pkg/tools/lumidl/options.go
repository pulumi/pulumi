// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lumidl

import (
	"reflect"
	"strings"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
)

// PropertyOptionsTag is the field tag the IDL compiler uses to find property options.
const PropertyOptionsTag = "lumi"

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
				case "out":
					opts.Out = true
				default:
					cmdutil.Sink().Errorf(diag.Message("unrecognized tag `lumi:\"%v\"`", key))
				}
			}
		}
	}
	return opts
}
