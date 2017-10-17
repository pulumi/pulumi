// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package encoding

import (
	"encoding/json"
	"path/filepath"
	"reflect"

	"github.com/ghodss/yaml"
	goyaml "gopkg.in/yaml.v2"
)

var JSONExt = ".json"
var YAMLExt = ".yaml"

// Exts contains a list of all the valid marshalable extension types.
var Exts = []string{
	JSONExt,
	YAMLExt,
	// Although ".yml" is not a sanctioned YAML extension, it is used quite broadly; so we will support it.
	".yml",
}

// Detect auto-detects a marshaler for the given path.
func Detect(path string) (Marshaler, string) {
	ext := filepath.Ext(path)
	if ext == "" {
		ext = DefaultExt() // default to the first (preferred) marshaler.
	}
	return Marshalers[ext], ext
}

// Marshalers is a map of extension to a Marshaler object for that extension.
var Marshalers map[string]Marshaler

// Default returns the default marshaler object.
func Default() Marshaler {
	return Marshalers[DefaultExt()]
}

// DefaultExt returns the default extension to use.
func DefaultExt() string {
	return Exts[0]
}

// Marshaler is a type that knows how to marshal and unmarshal data in one format.
type Marshaler interface {
	IsJSONLike() bool
	IsYAMLLike() bool
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

var JSON Marshaler = &jsonMarshaler{}

type jsonMarshaler struct {
}

func (m *jsonMarshaler) IsJSONLike() bool {
	return true
}

func (m *jsonMarshaler) IsYAMLLike() bool {
	return false
}

func (m *jsonMarshaler) Marshal(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "    ")
}

func (m *jsonMarshaler) Unmarshal(data []byte, v interface{}) error {
	// IDEA: use a "strict" marshaler, so that we can warn on unrecognized keys (avoiding silly mistakes).  We should
	//     set aside an officially sanctioned area in the metadata for extensibility by 3rd parties.
	return json.Unmarshal(data, v)
}

var YAML Marshaler = &yamlMarshaler{}

type yamlMarshaler struct {
}

func (m *yamlMarshaler) IsJSONLike() bool {
	return false
}

func (m *yamlMarshaler) IsYAMLLike() bool {
	return true
}

func (m *yamlMarshaler) Marshal(v interface{}) ([]byte, error) {
	if hasYamlTags(reflect.TypeOf(v)) {
		return goyaml.Marshal(v)
	}

	return yaml.Marshal(v)
}

func (m *yamlMarshaler) Unmarshal(data []byte, v interface{}) error {
	// IDEA: use a "strict" marshaler, so that we can warn on unrecognized keys (avoiding silly mistakes).  We should
	//     set aside an officially sanctioned area in the metadata for extensibility by 3rd parties.
	if hasYamlTags(reflect.TypeOf(v)) {
		return goyaml.Unmarshal(data, v)
	}

	return yaml.Unmarshal(data, v)
}

// hasYamlTags checks to see if all fields of a struct have yaml tags (and hence it would be safe) to use go-yaml directly.
func hasYamlTags(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		return hasYamlTags(t.Elem())
	}

	if t.Kind() != reflect.Struct {
		return false
	}

	allOk := true

	for i := 0; i < t.NumField(); i++ {
		_, has := t.Field(i).Tag.Lookup("yaml")
		allOk = allOk && has
	}

	return allOk
}
