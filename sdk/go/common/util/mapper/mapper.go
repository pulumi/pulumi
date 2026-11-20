// Copyright 2016, Pulumi Corporation.
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
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Mapper can map from weakly typed JSON-like property bags to strongly typed structs, and vice versa.
type Mapper interface {
	// Decode decodes a JSON-like object into the target pointer to a structure.
	Decode(obj map[string]any, target any) MappingError

	// DecodeValue stores the value of obj[key] in the value pointed to by target.
	// DecodeValue panics if target is not a non-nil pointer to a settable value.
	//
	// Decoding is primarily driven by the type of target and any associated
	// metadata (e.g. struct fields). Source types are coerced or converted where
	// possible.
	//
	// Given a target type:
	//
	// If the type is a pointer, DecodeValue decodes into the value pointed to,
	// allocating a new value if the pointer is nil. Non-nil source pointers are
	// dereferenced before decoding.
	//
	// If the value being decoded is assignable to the target type, it is assigned
	// as-is.
	//
	// If the value being decoded is convertible to the target type, it is converted
	// with reflect.Value.Convert, then assigned. For example, a float64 value can
	// be decoded into an int, and a value with the same underlying type as the
	// target can be decoded directly. Note that numeric conversions may truncate or
	// overflow without error.
	//
	// Otherwise, decoding uses the following type-dependent rules:
	//
	// To decode a map[string]any into a struct, DecodeValue uses the same tag
	// rules described in EncodeValue. Key matching is exact and case-sensitive.
	// Fields without recognized tags are ignored. Decoding replaces the destination
	// struct rather than merging with its existing fields.
	//
	// If Opts.CustomDecoders contains a decoder for the destination type, that
	// decoder determines how a map[string]any is decoded instead. Custom decoders
	// do not apply when decoding maps into maps.
	//
	// Struct fields are required unless tagged optional or omitempty, or
	// Opts.IgnoreMissing is true. Missing optional fields remain at their zero
	// values in the decoded struct. Unrecognized object keys cause errors unless
	// Opts.IgnoreUnrecognized is true. Keys belonging to skipped fields are
	// recognized, but their values are not decoded. Tagged fields of embedded
	// structs are treated as fields of the outer struct.
	//
	// To decode into an interface, the source value must implement that interface.
	// In particular, decoding into any preserves the source's concrete type after
	// dereferencing source pointers.
	//
	// To decode a slice of type []T into a slice of type []U, each element is
	// recursively decoded into the destination element type. Decoding produces a
	// separate slice rather than reusing the destination's elements. A zero-length
	// source slice, whether nil or non-nil, decodes as a nil slice. For example, an
	// empty []any decodes into a nil []string.
	//
	// To decode a map of type map[K1]V1 into a map of type map[K2]V2, each key
	// and value is recursively decoded into the destination key and value
	// types. Decoding produces a separate map rather than retaining existing
	// destination entries. The result is always non-nil, even if the source is
	// nil or empty. For example, a nil map[string]any decodes into a non-nil
	// empty map[string]string.
	//
	// To decode a string into a target of a different type T, *T must implement
	// encoding.TextUnmarshaler. DecodeValue then calls UnmarshalText with the source
	// string.
	//
	// A missing key or a nil interface value is treated as a missing field, not
	// as an instruction to clear the destination. If optional or Opts.IgnoreMissing
	// is true, target is left unchanged; otherwise a MissingError is returned.
	// Typed nil slices and maps are present values and follow the collection rules
	// above; they are not treated as missing fields. A typed nil pointer in
	// obj[key] panics.
	//
	// Incompatible types produce a WrongTypeError. Slice element and map key or
	// value errors identify the failing index or key. Errors from custom decoders,
	// nested struct decoding, and UnmarshalText are wrapped in a FieldError.
	// Destination pointers may have been allocated before an error is returned.
	DecodeValue(obj map[string]any, ty reflect.Type, key string, target any, optional bool) FieldError

	// Encode encodes an object into a JSON-like in-memory object.
	Encode(source any) (map[string]any, MappingError)

	// EncodeValue returns the JSON-like in-memory encoding of v.
	//
	// EncodeValue traverses v recursively. It uses the following type-dependent
	// encodings, including for named types with the corresponding underlying kind.
	//
	// Boolean values encode as bool.
	//
	// Integer and floating-point values encode as float64. This includes all
	// signed and unsigned integer types and uintptr. Integer conversion may lose
	// precision. NaN and infinities are preserved.
	//
	// String values encode as string, without escaping or UTF-8 validation.
	//
	// Slice values encode as []any, with each element encoded recursively. Byte
	// slices follow this same rule. A nil slice encodes as nil, and a non-nil
	// empty slice encodes as an empty slice. Nil elements are retained as nil.
	//
	// Struct values encode as map[string]any. Only fields with recognized tags
	// become members of the object; untagged fields are ignored.
	//
	// Each struct field's encoding is directed by a nonempty tag under the json
	// or pulumi key by default. The tag gives the object key, optionally followed
	// by a comma-separated list of options. An empty key means the empty string,
	// not the Go field name. Both tags are processed when both are present.
	// Opts.Tags overrides the recognized tag names.
	//
	// By default, a key of "-" or the tag option skip excludes a field. The tag
	// options omitempty and optional allow a field's encoded value to be nil,
	// in which case the key is omitted. Other zero values, including false, 0,
	// "", and non-nil empty slices and maps, are not omitted. A nil encoded field
	// without an optional tag produces a MissingError unless Opts.IgnoreMissing
	// is true, in which case it is omitted. Opts.OptionalTags and Opts.SkipTags
	// override the option names. Unknown tag options panic.
	//
	// Examples of struct field tags and their meanings, using the default options:
	//
	//	// Field appears under key "myName" and is required when decoding.
	//	Field int `json:"myName"`
	//
	//	// The pulumi tag also specifies the key.
	//	Field int `pulumi:"myName"`
	//
	//	// Field is omitted if nil and may be missing when decoding.
	//	Field *int `pulumi:"myName,optional"`
	//
	//	// omitempty has the same meaning as optional: a pointer to zero
	//	// is still encoded, and only a nil pointer is omitted.
	//	Field *int `json:"myName,omitempty"`
	//
	//	// Field uses the empty string as its key, not "Field".
	//	// A zero int is still encoded.
	//	Field int `json:",omitempty"`
	//
	//	// Field is neither encoded nor decoded. The key "myName" is
	//	// recognized during decoding, so it is not an unrecognized field.
	//	Field int `pulumi:"myName,skip"`
	//
	//	// Field is neither encoded nor decoded; its recognized key is "-".
	//	Field int `json:"-"`
	//
	//	// Field has no recognized tag and is neither encoded nor decoded.
	//	Field int
	//
	// Fields of embedded structs are encoded as fields of the outer struct.
	// Other anonymous fields, including embedded pointers to structs, are ignored.
	// Field lookup follows Go's field selection rules. If multiple fields encode
	// to the same key, later values overwrite earlier ones.
	//
	// Map values encode as map[string]any, with each value encoded recursively.
	// The map key type must have string kind; the keys are used directly. A nil
	// map encodes as nil, and a non-nil empty map encodes as an empty map. Nil map
	// values are retained as nil.
	//
	// Pointer values encode as the value pointed to. A nil pointer encodes as nil.
	//
	// Interface values encode as the value contained in the interface. A nil
	// interface value encodes as nil. EncodeValue(nil) also returns nil.
	//
	// Arrays, complex numbers, channels, functions, unsafe pointers, and maps
	// whose keys do not have string kind cause a panic.
	//
	// EncodeValue does not detect cycles. Cyclic values must not be passed to it.
	//
	// Field encoding errors are collected in a MappingError. Struct encoding may
	// return a partially populated map with an error; slice and map encoding
	// return nil with an error if any element or value fails to encode.
	EncodeValue(v any) (any, MappingError)
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
func Map(obj map[string]any, target any) MappingError {
	return New(nil).Decode(obj, target)
}

// MapI decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.  This variant
// ignores any missing required fields in the payload in addition to any unrecognized fields.
func MapI(obj map[string]any, target any) MappingError {
	return New(&Opts{
		IgnoreMissing:      true,
		IgnoreUnrecognized: true,
	}).Decode(obj, target)
}

// MapIM decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.  This variant
// ignores any missing required fields in the payload.
func MapIM(obj map[string]any, target any) MappingError {
	return New(&Opts{IgnoreMissing: true}).Decode(obj, target)
}

// MapIU decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.  This variant
// ignores any unrecognized fields in the payload.
func MapIU(obj map[string]any, target any) MappingError {
	return New(&Opts{IgnoreUnrecognized: true}).Decode(obj, target)
}

// Unmap translates an already mapped target object into a raw, unmapped form.
func Unmap(obj any) (map[string]any, error) {
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
		for fldinfo := range fldtype.Fields() {
			if fldinfo.Anonymous {
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
					if slices.Contains(optionalTags, part) {
						optional = true
						match = true
					}
					if !match {
						if slices.Contains(skipTags, part) {
							skip = true
							match = true
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
