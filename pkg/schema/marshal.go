// Copyright 2016 Marapongo, Inc. All rights reserved.

package schema

import (
	"encoding/json"

	"github.com/ghodss/yaml"
)

// Marshaler is a type that knows how to marshal and unmarshal data in one format.
type Marshaler interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

type jsonMarshaler struct {
}

func (m *jsonMarshaler) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (m *jsonMarshaler) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

type yamlMarshaler struct {
}

func (m *yamlMarshaler) Marshal(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

func (m *yamlMarshaler) Unmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}
