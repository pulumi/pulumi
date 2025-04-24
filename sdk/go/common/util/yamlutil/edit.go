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

package yamlutil

import (
	"errors"

	"gopkg.in/yaml.v3"
)

type HasRawValue interface {
	RawValue() []byte
}

// Edit does a deep comparison on original and new and returns a YAML document that modifies only
// the nodes changed between original and new.
func Edit(original []byte, newValue interface{}) ([]byte, error) {
	var err error
	var oldDoc yaml.Node
	err = yaml.Unmarshal(original, &oldDoc)
	if err != nil {
		return nil, err
	}

	newBytes, err := yaml.Marshal(newValue)
	if err != nil {
		return nil, err
	}
	var newNode yaml.Node
	err = yaml.Unmarshal(newBytes, &newNode)
	if err != nil {
		return nil, err
	}

	newNode, err = editNodes(&oldDoc, &newNode)
	if err != nil {
		return nil, err
	}

	return YamlEncode(&newNode)
}

func editNodes(original, newNode *yaml.Node) (yaml.Node, error) {
	if original.Kind != newNode.Kind {
		return *newNode, nil
	}

	ret := *original
	ret.Tag = newNode.Tag
	ret.Value = newNode.Value

	switch original.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		var minLen int
		var content []*yaml.Node
		minLen = min(len(newNode.Content), len(original.Content))

		for i := 0; i < minLen; i++ {
			item, err := editNodes(original.Content[i], newNode.Content[i])
			if err != nil {
				return ret, err
			}
			content = append(content, &item)
		}
		// Any excess nodes in the new value are copied verbatim
		content = append(content, newNode.Content[minLen:]...)

		ret.Content = content
		return ret, nil
	case yaml.MappingNode:
		content, err := handleMappingNode(original, newNode)
		if err != nil {
			return ret, err
		}

		ret.Content = content
		return ret, nil
	case yaml.ScalarNode, yaml.AliasNode:
		ret.Content = newNode.Content
		return ret, nil
	}
	return yaml.Node{}, errors.New("unknown node type")
}

func handleMappingNode(original, newNode *yaml.Node) ([]*yaml.Node, error) {
	origKeys := make(map[string]int)
	newKeys := make(map[string]int)
	var origKeyList, newKeyList []string

	content := make([]*yaml.Node, 0, len(newNode.Content))
	// In each of the Content slices, the keys are at even indices and the values
	// are at odd indices, so we iterate by 2 each time.
	for i := 0; i < len(original.Content); i += 2 {
		origKeys[original.Content[i].Value] = i
		origKeyList = append(origKeyList, original.Content[i].Value)
	}
	for i := 0; i < len(newNode.Content); i += 2 {
		newKey := newNode.Content[i].Value
		_, inOriginal := origKeys[newNode.Content[i].Value]
		newKeys[newKey] = i

		// Keep a list of all the keys that are not in the original to add them to
		// the end, (see pulumi/pulumi#14860).
		if !inOriginal {
			newKeyList = append(newKeyList, newKey)
		}
	}

	// Process in original key order, so that we preserve the order of keys in
	// the file. Add and modify all the keys in the original first, then new
	// add keys.
	for _, k := range origKeyList {
		origIdx := origKeys[k]
		newIdx, stillPresent := newKeys[k]
		if !stillPresent {
			// Key was in the original but not in the new value, so we skip it.
			continue
		}

		key, err := editNodes(original.Content[origIdx], newNode.Content[newIdx])
		if err != nil {
			return nil, err
		}
		value, err := editNodes(original.Content[origIdx+1], newNode.Content[newIdx+1])
		if err != nil {
			return nil, err
		}

		content = append(content, &key)
		content = append(content, &value)
	}

	// Apply keys that were not in the original at the end.
	for _, k := range newKeyList {
		newIdx := newKeys[k]
		key := *newNode.Content[newIdx]
		value := *newNode.Content[newIdx+1]

		content = append(content, &key)
		content = append(content, &value)
	}

	return content, nil
}
