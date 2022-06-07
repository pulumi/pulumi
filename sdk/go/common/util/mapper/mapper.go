// Copyright 2016-2018, Pulumi Corporation.
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

package mapper

import (
	"reflect"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Mapper can map from weakly typed JSON-like property bags to strongly typed structs, and vice versa.
type Mapper interface {
	// Decode decodes a JSON-like object into the target pointer to a structure.
	Decode(obj map[string]interface{}, target interface{}) MappingError
	// DecodeField decodes a single JSON-like value (with a given type and name) into a target pointer to a structure.
	DecodeValue(obj map[string]interface{}, ty reflect.Type, key string, target interface{}, optional bool) FieldError
	// Encode encodes an object into a JSON-like in-memory object.
	Encode(source interface{}) (map[string]interface{}, MappingError)
	// EncodeValue encodes a value into its JSON-like in-memory value format.
	EncodeValue(v interface{}) (interface{}, MappingError)
}

// New allocates a new mapper object with the given options.
func New(opts *Opts) Mapper {
	var initOpts Opts
	if opts != nil {
		initOpts = *opts
	}
	if initOpts.CustomDecoders == nil {
		initOpts.CustomDecoders = make(Decoders)
	}
	return &mapper{opts: initOpts}
}

// Opts controls the way mapping occurs; for default behavior, simply pass an empty struct.
type Opts struct {
	Tags               []string // the tag names to recognize (`json` and `pulumi` if unspecified).
	OptionalTags       []string // the tags to interpret to mean "optional" (`optional` if unspecified).
	SkipTags           []string // the tags to interpret to mean "skip" (`skip` if unspecified).
	CustomDecoders     Decoders // custom decoders.
	IgnoreMissing      bool     // ignore missing required fields.
	IgnoreUnrecognized bool     // ignore unrecognized fields.
}

type mapper struct {
	opts Opts
}

// Map decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.
func Map(obj map[string]interface{}, target interface{}) MappingError {
	return New(nil).Decode(obj, target)
}

// MapI decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.  This variant
// ignores any missing required fields in the payload in addition to any unrecognized fields.
func MapI(obj map[string]interface{}, target interface{}) MappingError {
	return New(&Opts{
		IgnoreMissing:      true,
		IgnoreUnrecognized: true,
	}).Decode(obj, target)
}

// MapIM decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.  This variant
// ignores any missing required fields in the payload.
func MapIM(obj map[string]interface{}, target interface{}) MappingError {
	return New(&Opts{IgnoreMissing: true}).Decode(obj, target)
}

// MapIU decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.  This variant
// ignores any unrecognized fields in the payload.
func MapIU(obj map[string]interface{}, target interface{}) MappingError {
	return New(&Opts{IgnoreUnrecognized: true}).Decode(obj, target)
}

// Unmap translates an already mapped target object into a raw, unmapped form.
func Unmap(obj interface{}) (map[string]interface{}, error) {
	return New(nil).Encode(obj)
}

// structFields digs into a type to fetch all fields, including its embedded structs, for a given type.
func structFields(t reflect.Type) []reflect.StructField {
	contract.Assertf(t.Kind() == reflect.Struct,
		"StructFields only valid on struct types; %v is not one (kind %v)", t, t.Kind())

	var fldinfos []reflect.StructField
	fldtypes := []reflect.Type{t}
	for len(fldtypes) > 0 {
		fldtype := fldtypes[0]
		fldtypes = fldtypes[1:]
		for i := 0; i < fldtype.NumField(); i++ {
			if fldinfo := fldtype.Field(i); fldinfo.Anonymous {
				// If an embedded struct, push it onto the queue to visit.
				if fldinfo.Type.Kind() == reflect.Struct {
					fldtypes = append(fldtypes, fldinfo.Type)
				}
			} else {
				// Otherwise, we will go ahead and consider this field in our decoding.
				fldinfos = append(fldinfos, fldinfo)
			}
		}
	}
	return fldinfos
}

// defaultTags fetches the mapper's tag names from the options, or supplies defaults if not present.
func (md *mapper) defaultTags() (tags []string, optionalTags []string, skipTags []string) {
	if md.opts.Tags == nil {
		tags = []string{"json", "pulumi"}
	} else {
		tags = md.opts.Tags
	}
	if md.opts.OptionalTags == nil {
		optionalTags = []string{"omitempty", "optional"}
	} else {
		optionalTags = md.opts.OptionalTags
	}
	if md.opts.SkipTags == nil {
		skipTags = []string{"skip"}
	} else {
		skipTags = md.opts.SkipTags
	}
	return tags, optionalTags, skipTags
}

// structFieldTags includes a field's information plus any parsed tags.
type structFieldTags struct {
	Info     reflect.StructField // the struct field info.
	Optional bool                // true if this can be missing.
	Skip     bool                // true to skip a field.
	Key      string              // the JSON key name.
}

// structFieldsTags digs into a type to fetch all fields, including embedded structs, plus any associated tags.
func (md *mapper) structFieldsTags(t reflect.Type) []structFieldTags {
	// Fetch the tag names to use.
	tags, optionalTags, skipTags := md.defaultTags()

	// Now walk the field infos and parse the tags to create the requisite structures.
	var fldtags []structFieldTags
	for _, fldinfo := range structFields(t) {
		for _, tagname := range tags {
			if tag := fldinfo.Tag.Get(tagname); tag != "" {
				var key string    // the JSON key name.
				var optional bool // true if this can be missing.
				var skip bool     // true if we should skip auto-marshaling.

				// Decode the tag.
				tagparts := strings.Split(tag, ",")
				contract.Assertf(len(tagparts) > 0,
					"Expected >0 tagparts on field %v.%v; got %v", t.Name(), fldinfo.Name, len(tagparts))
				key = tagparts[0]
				if key == "-" {
					skip = true // a name of "-" means skip
				}
				for _, part := range tagparts[1:] {
					var match bool
					for _, optionalTag := range optionalTags {
						if part == optionalTag {
							optional = true
							match = true
							break
						}
					}
					if !match {
						for _, skipTag := range skipTags {
							if part == skipTag {
								skip = true
								match = true
								break
							}
						}
					}
					contract.Assertf(match, "Unrecognized tagpart on field %v.%v: %v", t.Name(), fldinfo.Name, part)
				}

				fldtags = append(fldtags, structFieldTags{
					Key:      key,
					Optional: optional,
					Skip:     skip,
					Info:     fldinfo,
				})
			}
		}
	}
	return fldtags
}
