// Copyright 2026, Pulumi Corporation.
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

// Package pulumiconfig edits the values.pulumiConfig section of an ESC environment definition on behalf of
// "pulumi config" commands, preserving comments on untouched nodes.
package pulumiconfig

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

const functionPrefix = "fn::"

var (
	errReservedName       = errors.New("keys starting with fn:: are reserved in environments")
	errSecureReprReserved = errors.New(`maps with the single key "secure" are reserved`)
)

// isReservedName matches ESC, which rejects a single-key object whose key starts with fn:: in any case.
func isReservedName(k string) bool {
	return strings.HasPrefix(strings.ToLower(k), functionPrefix)
}

func escape(s string) string {
	return strings.ReplaceAll(s, "$", "$$")
}

func rootMapping(def *yaml.Node) (*yaml.Node, error) {
	if def == nil {
		return nil, errors.New("environment definition is empty")
	}
	if def.Kind == yaml.DocumentNode {
		if len(def.Content) == 0 {
			return nil, errors.New("environment definition is empty")
		}
		def = def.Content[0]
	}
	if def.Kind != yaml.MappingNode {
		return nil, errors.New("environment definition must be a mapping")
	}
	return def, nil
}

// ValueNode renders v as a YAML node using the typing rules of "pulumi config set". nested reports whether v is set
// below the top level of a --path key; only then are untyped strings such as "42" or "true" coerced.
func ValueNode(v config.Value, nested bool) (yaml.Node, error) {
	if v.Secure() {
		if v.Object() {
			return yaml.Node{}, errors.New("secret object values are not supported for stack-managed environments; " +
				"set each leaf with --path --secret")
		}
		ciphertext, err := v.Value(config.NopDecrypter)
		if err != nil {
			return yaml.Node{}, err
		}
		return secretNode(ciphertext), nil
	}
	// The stack reads pulumiConfig through config.NewPlaintext, which panics on a {secure: <string>} mapping, and
	// config reads that shape anywhere in an object value as a secret, so ToObject would hide it.
	secrets, err := v.SecureValues(config.NopDecrypter)
	if err != nil {
		return yaml.Node{}, err
	}
	if len(secrets) != 0 {
		return yaml.Node{}, errSecureReprReserved
	}

	toObject := v.ToObject
	if nested {
		toObject = v.ToCoercedObject
	}
	obj, err := toObject()
	if err != nil {
		return yaml.Node{}, err
	}
	literal, err := literalValue(obj)
	if err != nil {
		return yaml.Node{}, err
	}
	var node yaml.Node
	if err := node.Encode(literal); err != nil {
		return yaml.Node{}, err
	}
	return node, nil
}

func literalValue(v any) (any, error) {
	switch v := v.(type) {
	case string:
		return escape(v), nil
	case []any:
		out := make([]any, len(v))
		for i, elem := range v {
			lit, err := literalValue(elem)
			if err != nil {
				return nil, err
			}
			out[i] = lit
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, elem := range v {
			if isReservedName(k) {
				return nil, errReservedName
			}
			lit, err := literalValue(elem)
			if err != nil {
				return nil, err
			}
			out[escape(k)] = lit
		}
		return out, nil
	default:
		return v, nil
	}
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func secretNode(ciphertext string) yaml.Node {
	return yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		scalar("fn::secret"),
		{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar("ciphertext"), scalar(ciphertext)}},
	}}
}

// ImportsDefinition returns a definition holding only def's imports, or false if def has none.
func ImportsDefinition(def *yaml.Node) ([]byte, bool) {
	root, err := rootMapping(def)
	if err != nil {
		return nil, false
	}
	imports, ok := encoding.YAMLSyntax{Node: root}.Get(resource.PropertyPath{"imports"})
	if !ok || imports.Kind != yaml.SequenceNode || len(imports.Content) == 0 {
		return nil, false
	}
	only := yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar("imports"), imports}}
	out, err := yaml.Marshal(&only)
	if err != nil {
		return nil, false
	}
	return out, true
}
