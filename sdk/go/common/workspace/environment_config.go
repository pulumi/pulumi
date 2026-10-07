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

package workspace

import (
	"encoding/json"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ErrEnvironmentNotDefinition is returned by the pulumiConfig editors when the environment block is a
// list of environments to import rather than an inline definition.
var ErrEnvironmentNotDefinition = errors.New("the environment block is not an inline environment definition")

// SetPulumiConfig sets `values.pulumiConfig.<key>` in an inline environment definition, creating the
// `values` and `pulumiConfig` mappings as needed. A non-empty path addresses a property nested inside
// the key's value, one segment per element: a string selects a mapping entry and an int a sequence
// element, creating containers on the way; appending to a sequence is allowed, skipping ahead is not.
// Trivia around untouched nodes is preserved.
func (e *Environment) SetPulumiConfig(key string, path []any, value *yaml.Node) error {
	return e.editDefinition(func(root *yaml.Node) error {
		values, err := ensureMappingEntry(root, "values")
		if err != nil {
			return err
		}
		pulumiConfig, err := ensureMappingEntry(values, "pulumiConfig")
		if err != nil {
			return err
		}
		return setNodePath(pulumiConfig, append([]any{key}, path...), value)
	})
}

// RemovePulumiConfig removes `values.pulumiConfig.<key>` (or the property at path inside it) from an
// inline environment definition. It reports whether something was removed.
func (e *Environment) RemovePulumiConfig(key string, path []any) (bool, error) {
	removed := false
	err := e.editDefinition(func(root *yaml.Node) error {
		values := mappingEntry(root, "values")
		if values == nil {
			return nil
		}
		pulumiConfig := mappingEntry(values, "pulumiConfig")
		if pulumiConfig == nil {
			return nil
		}
		var err error
		removed, err = removeNodePath(pulumiConfig, append([]any{key}, path...))
		return err
	})
	return removed, err
}

// editDefinition runs edit against the definition's root mapping. A YAML definition is edited in
// place; a JSON one is decoded into a YAML node for the edit and re-encoded afterwards.
func (e *Environment) editDefinition(edit func(root *yaml.Node) error) error {
	if !e.IsDefinition() {
		return ErrEnvironmentNotDefinition
	}
	if e.node != nil {
		if e.node.Kind != yaml.MappingNode {
			return errors.New("the environment definition is not a mapping")
		}
		return edit(e.node)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(e.message, &doc); err != nil {
		return fmt.Errorf("parsing environment definition: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return errors.New("the environment definition is not a mapping")
	}
	if err := edit(doc.Content[0]); err != nil {
		return err
	}
	var decoded any
	if err := doc.Content[0].Decode(&decoded); err != nil {
		return fmt.Errorf("re-encoding environment definition: %w", err)
	}
	message, err := json.Marshal(decoded)
	if err != nil {
		return fmt.Errorf("re-encoding environment definition: %w", err)
	}
	e.message = message
	return nil
}

// mappingEntry returns the value node stored under key in mapping m, or nil.
func mappingEntry(m *yaml.Node, key string) *yaml.Node {
	if m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := m.Content[i]; k.Kind == yaml.ScalarNode && k.Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// ensureMappingEntry returns the mapping stored under key in m, creating it (or turning a null
// placeholder into one) when necessary.
func ensureMappingEntry(m *yaml.Node, key string) (*yaml.Node, error) {
	if m.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected %q to be inside a mapping", key)
	}
	entry := mappingEntry(m, key)
	if entry == nil {
		entry = nullNode()
		m.Content = append(m.Content, scalarNode(key), entry)
	}
	if err := makeContainer(entry, yaml.MappingNode); err != nil {
		return nil, fmt.Errorf("%q: %w", key, err)
	}
	return entry, nil
}

// setNodePath stores value at path below parent, creating intermediate containers.
func setNodePath(parent *yaml.Node, path []any, value *yaml.Node) error {
	if len(path) == 0 {
		replaceNode(parent, value)
		return nil
	}
	switch seg := path[0].(type) {
	case string:
		if err := makeContainer(parent, yaml.MappingNode); err != nil {
			return fmt.Errorf("%q: %w", seg, err)
		}
		child := mappingEntry(parent, seg)
		if child == nil {
			child = nullNode()
			parent.Content = append(parent.Content, scalarNode(seg), child)
		}
		return setNodePath(child, path[1:], value)
	case int:
		if err := makeContainer(parent, yaml.SequenceNode); err != nil {
			return fmt.Errorf("[%d]: %w", seg, err)
		}
		switch {
		case seg < 0 || seg > len(parent.Content):
			return fmt.Errorf("index %d is out of range for a list of %d items", seg, len(parent.Content))
		case seg == len(parent.Content):
			parent.Content = append(parent.Content, nullNode())
		}
		return setNodePath(parent.Content[seg], path[1:], value)
	default:
		return fmt.Errorf("unsupported path segment %v", path[0])
	}
}

// removeNodePath removes the node at path below parent and reports whether it existed.
func removeNodePath(parent *yaml.Node, path []any) (bool, error) {
	switch seg := path[0].(type) {
	case string:
		if parent.Kind != yaml.MappingNode {
			return false, nil
		}
		for i := 0; i+1 < len(parent.Content); i += 2 {
			if k := parent.Content[i]; k.Kind != yaml.ScalarNode || k.Value != seg {
				continue
			}
			if len(path) == 1 {
				parent.Content = append(parent.Content[:i], parent.Content[i+2:]...)
				return true, nil
			}
			return removeNodePath(parent.Content[i+1], path[1:])
		}
		return false, nil
	case int:
		if parent.Kind != yaml.SequenceNode || seg < 0 || seg >= len(parent.Content) {
			return false, nil
		}
		if len(path) == 1 {
			parent.Content = append(parent.Content[:seg], parent.Content[seg+1:]...)
			return true, nil
		}
		return removeNodePath(parent.Content[seg], path[1:])
	default:
		return false, fmt.Errorf("unsupported path segment %v", path[0])
	}
}

// makeContainer ensures n is a container of the given kind, converting a null placeholder (and an
// empty flow-style container) into a block-style one.
func makeContainer(n *yaml.Node, kind yaml.Kind) error {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
		n.Kind = kind
		n.Tag = ""
		n.Value = ""
		n.Style = 0
		return nil
	}
	if n.Kind != kind {
		want := "a mapping"
		if kind == yaml.SequenceNode {
			want = "a list"
		}
		return fmt.Errorf("expected %s", want)
	}
	if len(n.Content) == 0 {
		// `{}` / `[]` would otherwise keep their flow style and swallow the new entries onto one line.
		n.Style = 0
	}
	return nil
}

// replaceNode overwrites dst with src while keeping dst's comments.
func replaceNode(dst *yaml.Node, src *yaml.Node) {
	head, line, foot := dst.HeadComment, dst.LineComment, dst.FootComment
	*dst = *src
	dst.HeadComment, dst.LineComment, dst.FootComment = head, line, foot
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func nullNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
}
