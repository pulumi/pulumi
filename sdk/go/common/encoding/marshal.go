// Copyright 2016-2022, Pulumi Corporation.
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

package encoding

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

var (
	JSONExt = ".json"
	YAMLExt = ".yaml"
	GZIPExt = ".gz"
)

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
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

// JSON is a Marshaler that marshals and unmarshals JSON with indented printing.
var JSON Marshaler = &jsonMarshaler{}

type jsonMarshaler struct {
}

func (m *jsonMarshaler) Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *jsonMarshaler) Unmarshal(data []byte, v interface{}) error {
	// IDEA: use a "strict" marshaler, so that we can warn on unrecognized keys (avoiding silly mistakes).  We should
	//     set aside an officially sanctioned area in the metadata for extensibility by 3rd parties.
	return json.Unmarshal(data, v)
}

var YAML Marshaler = &yamlMarshaler{}

type yamlMarshaler struct {
}

func (m *yamlMarshaler) Marshal(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

func (m *yamlMarshaler) Unmarshal(data []byte, v interface{}) error {
	// IDEA: use a "strict" marshaler, so that we can warn on unrecognized keys (avoiding silly mistakes).  We should
	//     set aside an officially sanctioned area in the metadata for extensibility by 3rd parties.

	err := yaml.Unmarshal(data, v)
	if err != nil {
		// Return type errors directly
		if _, ok := err.(*yaml.TypeError); ok {
			return err
		}
		// Other errors will be parse errors due to invalid syntax
		return fmt.Errorf("invalid YAML file: %w", err)
	}
	return nil
}

type gzipMarshaller struct {
	inner Marshaler
}

func (m *gzipMarshaller) Marshal(v interface{}) ([]byte, error) {
	b, err := m.inner.Marshal(v)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	defer writer.Close()
	_, err = writer.Write(b)
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil

}

func (m *gzipMarshaller) Unmarshal(data []byte, v interface{}) error {
	buf := bytes.NewBuffer(data)
	reader, err := gzip.NewReader(buf)
	if err != nil {
		return err
	}
	defer reader.Close()
	inflated, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	if err := reader.Close(); err != nil {
		return err
	}
	return m.inner.Unmarshal(inflated, v)
}

// IsCompressed returns if data is zip compressed.
func IsCompressed(buf []byte) bool {
	// Taken from compress/gzip/gunzip.go
	return len(buf) >= 3 && buf[0] == 31 && buf[1] == 139 && buf[2] == 8
}

func Gzip(m Marshaler) Marshaler {
	_, alreadyGZIP := m.(*gzipMarshaller)
	if alreadyGZIP {
		return m
	}
	return &gzipMarshaller{m}
}
