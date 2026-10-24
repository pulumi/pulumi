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
	"fmt"
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

// ValueNode renders v as a YAML node using the typing rules of "pulumi config set": top-level strings stay strings
// and --path leaves are coerced.
func ValueNode(key config.Key, v config.Value, path bool) (yaml.Node, error) {
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

	// As element 0 of a new array the leaf keeps Map.Set's --path coercion but skips its "secure"-key check, which
	// needs the entry's siblings; Set makes that check against the real definition.
	scratchKey := key
	isNested := false
	if path {
		segments, err := resource.ParsePropertyPathStrict(key.Name())
		if err != nil {
			return yaml.Node{}, fmt.Errorf("invalid config key path: %w", err)
		}
		if len(segments) > 1 {
			isNested = true
			scratchKey = config.MustMakeKey(key.Namespace(), resource.PropertyPath{segments[0], 0}.String())
		}
	}
	scratch := config.Map{}
	if err := scratch.Set(scratchKey, v, path); err != nil {
		return yaml.Node{}, err
	}
	var stored config.Value
	for _, value := range scratch {
		stored = value
	}
	obj, err := stored.ToObject()
	if err != nil {
		return yaml.Node{}, err
	}
	if isNested {
		obj = obj.([]any)[0]
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
		// The stack reads pulumiConfig through config.NewPlaintext, which panics on a {secure: <string>} mapping.
		if _, ok := v["secure"].(string); ok && len(v) == 1 {
			return nil, errSecureReprReserved
		}
		out := make(map[string]any, len(v))
		for k, elem := range v {
			if strings.HasPrefix(k, functionPrefix) {
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
