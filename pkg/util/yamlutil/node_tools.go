package yamlutil

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type yamlError struct {
	*yaml.Node
	err string
}

func (e yamlError) Error() string {
	return fmt.Sprintf("[%v:%v] %v", e.Node.Line, e.Node.Column, e.err)
}

func YamlErrorf(node *yaml.Node, format string, a ...interface{}) error {
	return yamlError{Node: node, err: fmt.Sprintf(format, a...)}
}

// Get attempts to obtain the value at an index in a sequence or in a mapping. Returns the node and true if successful;
// nil and false if out of bounds or not found; and nil, false, and an error if any error occurs.
func Get(l *yaml.Node, key interface{}) (*yaml.Node, bool, error) {
	switch l.Kind {
	case yaml.DocumentNode:
		// Automatically recurse as documents contain a single element
		return Get(l.Content[0], key)
	case yaml.SequenceNode:
		if idx, ok := key.(int); ok {
			if len(l.Content) > idx {
				return l.Content[idx], true, nil
			}
			return nil, false, nil
		}
		return nil, false, YamlErrorf(l, "unsupported index type %T, found sequence node and expected int index", key)
	case yaml.MappingNode:
		for i, v := range l.Content {
			if i%2 == 1 {
				continue
			}

			switch k := key.(type) {
			case string:
				if v.Tag == "str" && v.Value == k {
					return l.Content[i+1], true, nil
				}
			default:
				return nil, false, YamlErrorf(l, "unsupported key type %T, found mapping node and expected string key", key)
			}
		}
		// failure to get is not an error
		return nil, false, nil
	case yaml.ScalarNode:
		return nil, false, YamlErrorf(l, "failed to get key %v from scalar: %v", key, l.Value)
	case yaml.AliasNode:
		return nil, false, YamlErrorf(l, "aliases not supported in project files: %v", l.Value)
	default:
		return nil, false, YamlErrorf(l, "failed to parse yaml node: %v", l.Value)
	}
}

func Set(l *yaml.Node, value string) error {
	var newNode yaml.Node
	err := yaml.Unmarshal([]byte(value), &newNode)
	if err != nil {
		return err
	}

	*l = *newNode.Content[0]

	return nil
}

func Insert(l *yaml.Node, key interface{}, value string) error {
	var newNode yaml.Node
	err := yaml.Unmarshal([]byte(value), &newNode)
	if err != nil {
		return err
	}
	newNode = *newNode.Content[0]

	switch l.Kind {
	case yaml.DocumentNode:
		// Automatically recurse as documents contain a single element
		return Insert(l.Content[0], key, value)
	case yaml.SequenceNode:
		if idx, ok := key.(int); ok {
			if len(l.Content) > idx {
				l.Content[idx] = &newNode
			} else if len(l.Content) == idx {
				l.Content = append(l.Content, &newNode)
			}
			return YamlErrorf(l, "index %v out of bounds of node: %v", idx, l.Value)
		}
		return YamlErrorf(l, "unsupported index type %T, found sequence node and expected int index", key)
	case yaml.MappingNode:
		for i, v := range l.Content {
			if i%2 == 1 {
				continue
			}

			switch k := key.(type) {
			case string:
				if v.Tag == "!!str" && v.Value == k {
					l.Content[i+1] = &newNode
					return nil
				}
			default:
				return YamlErrorf(l, "unsupported key type %T, found mapping node and expected string key", key)
			}
		}

		var keyNode yaml.Node
		switch k := key.(type) {
		case string:
			err := Set(&keyNode, k)
			if err != nil {
				return err
			}
			l.Content = append(l.Content, &keyNode, &newNode)
			return nil
		default:
			return YamlErrorf(l, "unsupported key type %T, found mapping node and expected string key", key)
		}
	case yaml.ScalarNode:
		return YamlErrorf(l, "failed to get key %v from scalar: %v", key, l.Content)
	case yaml.AliasNode:
		return YamlErrorf(l, "aliases not supported in project files: %v", l.Content)
	default:
		return YamlErrorf(l, "failed to parse yaml node: %v", l.Content)
	}
}

func Delete(l *yaml.Node, key interface{}) error {
	switch l.Kind {
	case yaml.DocumentNode:
		// Automatically recurse as documents contain a single element
		return Delete(l.Content[0], key)
	case yaml.SequenceNode:
		if idx, ok := key.(int); ok {
			var content []*yaml.Node
			content = append(content, l.Content[:idx]...)
			content = append(content, l.Content[idx+1:]...)
			l.Content = content
			return nil
		}
		return YamlErrorf(l, "unsupported index type %T, found sequence node and expected int index", key)

	case yaml.MappingNode:
		for idx, v := range l.Content {
			if idx%2 == 1 {
				continue
			}

			switch k := key.(type) {
			case string:
				if v.Tag == "!!str" && v.Value == k {
					var content []*yaml.Node
					content = append(content, l.Content[:idx]...)
					content = append(content, l.Content[idx+2:]...)
					l.Content = content
					return nil
				}
			default:
				return YamlErrorf(l, "unsupported key type %T, found mapping node and expected string key", key)
			}
		}
		return nil
	case yaml.ScalarNode:
		return YamlErrorf(l, "failed to get key %v from scalar: %v", key, l.Content)
	case yaml.AliasNode:
		return YamlErrorf(l, "aliases not supported in project files: %v", l.Content)
	default:
		return YamlErrorf(l, "failed to parse yaml node: %v", l.Content)
	}
}
