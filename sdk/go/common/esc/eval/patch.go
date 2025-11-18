// Copyright 2025, Pulumi Corporation.
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

package eval

import (
	"encoding/json"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"gopkg.in/yaml.v3"
)

// Patch represents a value that should be written back to the environment at the given path.
type Patch struct {
	DocPath     string
	Replacement esc.Value
}

// ApplyValuePatches applies a set of patches values to an environment definition.
// If patch values contain secret values, they will be wrapped with fn::secret.
func ApplyValuePatches(source []byte, patches []*Patch) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(source, &doc); err != nil {
		return nil, err
	}

	for _, patch := range patches {
		path, err := resource.ParsePropertyPath(patch.DocPath)
		if err != nil {
			return nil, err
		}

		// convert the esc.Value into a yaml node that can be set on the environment
		replacement, err := valueToSecretJSON(patch.Replacement)
		if err != nil {
			return nil, err
		}
		bytes, err := yaml.Marshal(replacement)
		if err != nil {
			return nil, err
		}
		var yamlValue yaml.Node
		if err := yaml.Unmarshal(bytes, &yamlValue); err != nil {
			return nil, err
		}
		yamlValue = *yamlValue.Content[0]

		_, err = encoding.YAMLSyntax{Node: &doc}.Set(nil, path, yamlValue)
		if err != nil {
			return nil, err
		}
	}

	return yaml.Marshal(doc.Content[0])
}

// valueToSecretJSON converts a Value into a plain-old-JSON value, but secret values are wrapped with fn::secret
func valueToSecretJSON(v esc.Value) (any, error) {
	// If this value is secret at the top level, we need to handle it specially
	// to avoid nested fn::secret calls and to properly encode non-primitive secrets
	if v.Secret {
		return wrapSecret(v.ToJSON(false))
	}

	// For non-secret values, recurse normally
	var err error
	switch pv := v.Value.(type) {
	case []esc.Value:
		a := make([]any, len(pv))
		for i, v := range pv {
			a[i], err = valueToSecretJSON(v)
			if err != nil {
				return nil, err
			}
		}
		return a, nil
	case map[string]esc.Value:
		m := make(map[string]any, len(pv))
		for k, v := range pv {
			m[k], err = valueToSecretJSON(v)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	default:
		return pv, nil
	}
}

// wrapSecret wraps a value in a fn::secret function call
func wrapSecret(v any) (any, error) {
	if _, ok := v.(string); ok {
		return map[string]any{
			"fn::secret": v,
		}, nil
	}

	// fn::secret requires a string literal, so encode any non-strings as a JSON
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"fn::fromJSON": map[string]any{
			"fn::secret": string(encoded),
		},
	}, nil
}
