package yamlutil

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

type HasRawValue interface {
	RawValue() []byte
}

// Edit does a deep comparison on original and new and returns a YAML document that modifies only
// the nodes changed between original and new.
func Edit(original []byte, new interface{}) ([]byte, error) {
	var err error
	var oldDoc yaml.Node
	err = yaml.Unmarshal(original, &oldDoc)
	if err != nil {
		return nil, err
	}

	newBytes, err := yaml.Marshal(new)
	if err != nil {
		return nil, err
	}
	var newValue yaml.Node
	err = yaml.Unmarshal(newBytes, &newValue)
	if err != nil {
		return nil, err
	}

	newValue, err = editNodes(&oldDoc, &newValue)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&b)
	yamlEncoder.SetIndent(2)
	err = yamlEncoder.Encode(&newValue)
	if err != nil {
		return nil, fmt.Errorf("error editing value %#v: %w", newValue, err)
	}

	return b.Bytes(), nil
}

func editNodes(original, new *yaml.Node) (yaml.Node, error) {
	if original.Kind != new.Kind {
		return *new, nil
	}

	ret := *original
	ret.Tag = new.Tag
	ret.Value = new.Value

	switch original.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		var minLen int
		var content []*yaml.Node
		if len(new.Content) < len(original.Content) {
			minLen = len(new.Content)
		} else {
			minLen = len(original.Content)
		}

		for i := 0; i < minLen; i++ {
			item, err := editNodes(original.Content[i], new.Content[i])
			if err != nil {
				return ret, err
			}
			content = append(content, &item)
		}
		// Any excess nodes in the new value are copied verbatim
		content = append(content, new.Content[minLen:]...)

		ret.Content = content
		return ret, nil
	case yaml.MappingNode:
		origKeys := make(map[string]int)
		newKeys := make(map[string]int)
		var newKeyList []string

		var content []*yaml.Node
		for i := 0; i < len(original.Content); i += 2 {
			origKeys[original.Content[i].Value] = i
		}
		for i := 0; i < len(new.Content); i += 2 {
			value := new.Content[i].Value
			newKeys[value] = i
			newKeyList = append(newKeyList, value)
		}
		for _, k := range newKeyList {
			newIdx := newKeys[k]
			origIdx, has := origKeys[k]
			var err error
			var key yaml.Node
			var value yaml.Node
			if has {
				key, err = editNodes(original.Content[origIdx], new.Content[newIdx])
				if err != nil {
					return ret, err
				}
				value, err = editNodes(original.Content[origIdx+1], new.Content[newIdx+1])
				if err != nil {
					return ret, err
				}
			} else {
				key = *new.Content[newIdx]
				value = *new.Content[newIdx+1]
			}
			content = append(content, &key)
			content = append(content, &value)
		}

		ret.Content = content
		return ret, nil
	default: // alias and scalar nodes

		ret.Content = new.Content
		return ret, nil
	}
}
