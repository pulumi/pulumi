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
	"slices"
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

	// ErrSecretObject is returned by ValueNode for a secure object value, which an environment cannot store.
	ErrSecretObject = errors.New("secret object values are not supported for stack-managed environments; " +
		"set each leaf with --path --secret")
)

// isReservedName matches ESC, which rejects an fn:: key in any object, and a Fn:: key in any case when it is the
// object's only key.
func isReservedName(k string, onlyKey bool) bool {
	if onlyKey {
		k = strings.ToLower(k)
	}
	return strings.HasPrefix(k, functionPrefix)
}

// escape quotes s as an ESC literal, so a $ is not read as an interpolation.
func escape(s string) string {
	return strings.ReplaceAll(s, "$", "$$")
}

func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null"
}

// isConfigLeaf reports whether n is a single config value: a scalar or an fn::secret.
func isConfigLeaf(n *yaml.Node) bool {
	isSecret := n.Kind == yaml.MappingNode && len(n.Content) == 2 && n.Content[0].Value == "fn::secret"
	return n.Kind == yaml.ScalarNode || isSecret
}

// isFunctionCall reports whether n is an ESC function call such as fn::open.
func isFunctionCall(n *yaml.Node) bool {
	return n.Kind == yaml.MappingNode && len(n.Content) == 2 && isReservedName(n.Content[0].Value, true)
}

// resetNode replaces n with an empty node of the given kind, keeping its head and line comments.
func resetNode(n *yaml.Node, kind yaml.Kind, tag string) {
	*n = yaml.Node{Kind: kind, Tag: tag, HeadComment: n.HeadComment, LineComment: n.LineComment}
}

// rootMapping returns the top-level mapping of an environment definition.
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

// childMapping returns the mapping at parent[key]; a missing or null child is created if create is set, else nil.
func childMapping(parent *yaml.Node, key string, create bool) (*yaml.Node, error) {
	child, ok := encoding.YAMLSyntax{Node: parent}.Get(resource.PropertyPath{key})
	if !ok {
		if !create {
			return nil, nil
		}
		return encoding.YAMLSyntax{Node: parent}.Set(
			nil, resource.PropertyPath{key}, yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"})
	}
	if isNull(child) {
		if !create {
			return nil, nil
		}
		resetNode(child, yaml.MappingNode, "!!map")
		return child, nil
	}
	if child.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a mapping", key)
	}
	if create && isFunctionCall(child) {
		return nil, fmt.Errorf("%s: cannot set a key inside %s", key, child.Content[0].Value)
	}
	return child, nil
}

// pulumiConfigNode returns the definition's values.pulumiConfig mapping, creating it when create is set.
func pulumiConfigNode(def *yaml.Node, create bool) (*yaml.Node, error) {
	root, err := rootMapping(def)
	if err != nil {
		return nil, err
	}
	values, err := childMapping(root, "values", create)
	if err != nil || values == nil {
		return nil, err
	}
	return childMapping(values, "pulumiConfig", create)
}

// matchingNames returns the names that are entries in pc.
func matchingNames(pc *yaml.Node, names []string) []string {
	var found []string
	for _, name := range names {
		if _, ok := (encoding.YAMLSyntax{Node: pc}).Get(resource.PropertyPath{escape(name)}); ok {
			found = append(found, name)
		}
	}
	return found
}

// checkNames rejects an empty names list and an fn:: key, in any case, in the names or the path.
func checkNames(names []string, path resource.PropertyPath) error {
	if len(names) == 0 {
		return errors.New("at least one config key name is required")
	}
	if slices.ContainsFunc(names, func(name string) bool { return isReservedName(name, true) }) {
		return errReservedName
	}
	for _, seg := range path {
		if s, ok := seg.(string); ok && isReservedName(s, true) {
			return errReservedName
		}
	}
	return nil
}

// escapePath escapes each string key in path.
func escapePath(path resource.PropertyPath) resource.PropertyPath {
	escaped := make(resource.PropertyPath, len(path))
	for i, seg := range path {
		if s, ok := seg.(string); ok {
			seg = escape(s)
		}
		escaped[i] = seg
	}
	return escaped
}

// Find returns the first of names that is an entry in the definition's values.pulumiConfig.
func Find(def *yaml.Node, names []string) (name string, ok bool) {
	pc, err := pulumiConfigNode(def, false)
	if err != nil || pc == nil {
		return "", false
	}
	found := matchingNames(pc, names)
	if len(found) == 0 {
		return "", false
	}
	return found[0], true
}

// Get returns the node at path inside the pulumiConfig entry name. name and path are literal; Get escapes them.
func Get(def *yaml.Node, name string, path resource.PropertyPath) (*yaml.Node, bool) {
	pc, err := pulumiConfigNode(def, false)
	if err != nil || pc == nil {
		return nil, false
	}
	full := append(resource.PropertyPath{escape(name)}, escapePath(path)...)
	return encoding.YAMLSyntax{Node: pc}.Get(full)
}

// Set writes value at path inside the pulumiConfig entry named by whichever of names exists, or by names[0] if
// none does. An empty path replaces the whole entry. names and path are literal; Set escapes them.
func Set(def *yaml.Node, names []string, path resource.PropertyPath, value yaml.Node) error {
	if err := checkNames(names, path); err != nil {
		return err
	}
	pc, err := pulumiConfigNode(def, true)
	if err != nil {
		return err
	}

	entry := names[0]
	if found := matchingNames(pc, names); len(found) > 1 {
		return fmt.Errorf("pulumiConfig has both %s", strings.Join(found, " and "))
	} else if len(found) == 1 {
		entry = found[0]
	}

	full := append(resource.PropertyPath{escape(entry)}, escapePath(path)...)
	syntax := encoding.YAMLSyntax{Node: pc}
	if len(path) > 0 {
		parent, err := prepareContainers(syntax, full)
		if err != nil {
			return err
		}
		if leavesLoneSecureKey(parent, full[len(full)-1], value) {
			return errSecureReprReserved
		}
	}
	leaf, err := syntax.Set(nil, full, value)
	if err != nil {
		return err
	}
	// YAMLSyntax.Set does not copy Style.
	leaf.Style = value.Style
	return nil
}

// The stack reads pulumiConfig through config.NewPlaintext, which panics on a {secure: <string>} mapping.
func leavesLoneSecureKey(parent *yaml.Node, key any, value yaml.Node) bool {
	isOnlyKey := len(parent.Content) == 2
	return key == "secure" && isOnlyKey && value.Kind == yaml.ScalarNode && value.ShortTag() == "!!str"
}

// YAMLSyntax.Set gives a zero-kind node its container type from the next path segment.
const placeholderKind yaml.Kind = 0

// prepareContainers matches config set --path where YAMLSyntax.Set differs. It returns the container that holds the
// final segment, which it adds as a placeholder if missing.
func prepareContainers(syntax encoding.YAMLSyntax, full resource.PropertyPath) (*yaml.Node, error) {
	var parent *yaml.Node
	cur := syntax.Node
	for i, segment := range full {
		parent = cur
		child, ok := encoding.YAMLSyntax{Node: cur}.Get(resource.PropertyPath{segment})
		if !ok {
			if index, isIndex := segment.(int); isIndex && cur.Kind == placeholderKind && index > 0 {
				cur.Kind, cur.Tag = yaml.SequenceNode, "!!seq"
				for range index {
					cur.Content = append(cur.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"})
				}
			}
			var err error
			child, err = encoding.YAMLSyntax{Node: cur}.Set(nil, resource.PropertyPath{segment}, yaml.Node{})
			if err != nil {
				return nil, err
			}
		}
		isContainerStep := i < len(full)-1
		isEntry := i == 0
		if isContainerStep && isConfigLeaf(child) {
			if !isEntry && !isNull(child) {
				return nil, fmt.Errorf("%v: expected an array or an object", full[:i+2])
			}
			resetNode(child, placeholderKind, "")
		} else if isContainerStep && isFunctionCall(child) {
			// ESC rejects a function key with siblings.
			return nil, fmt.Errorf("%v: cannot set a path inside %s", full[:i+1], child.Content[0].Value)
		}
		cur = child
	}
	return parent, nil
}

// Remove deletes the path inside every pulumiConfig entry named by names, or the whole entries when path is empty.
func Remove(def *yaml.Node, names []string, path resource.PropertyPath) error {
	if err := checkNames(names, path); err != nil {
		return err
	}
	pc, err := pulumiConfigNode(def, false)
	if err != nil || pc == nil {
		return err
	}
	syntax := encoding.YAMLSyntax{Node: pc}
	var targets []resource.PropertyPath
	for _, name := range matchingNames(pc, names) {
		full := append(resource.PropertyPath{escape(name)}, escapePath(path)...)
		if _, ok := syntax.Get(full); !ok {
			continue
		}
		if key, value, ok := loneEntryAfterRemove(syntax, full); ok && len(path) > 0 {
			if key.Value == "secure" && value.Kind == yaml.ScalarNode && value.ShortTag() == "!!str" {
				return errSecureReprReserved
			}
			if isReservedName(key.Value, true) {
				return errReservedName
			}
		}
		targets = append(targets, full)
	}
	for _, full := range targets {
		if err := syntax.Delete(nil, full); err != nil {
			return err
		}
	}
	return nil
}

// loneEntryAfterRemove returns the key and value that would be left alone in their mapping if full were removed.
func loneEntryAfterRemove(syntax encoding.YAMLSyntax, full resource.PropertyPath) (key, value *yaml.Node, ok bool) {
	removedKey, isKey := full[len(full)-1].(string)
	if !isKey {
		return nil, nil, false
	}
	parent, ok := syntax.Get(full[:len(full)-1])
	if !ok || parent.Kind != yaml.MappingNode || len(parent.Content) != 4 {
		return nil, nil, false
	}
	kept := parent.Content[0:2]
	if kept[0].Value == removedKey {
		kept = parent.Content[2:4]
	}
	return kept[0], kept[1], true
}

// ValueNode renders v as a YAML node using the typing rules of "pulumi config set". nested reports whether v is set
// below the top level of a --path key; only then are untyped strings such as "42" or "true" coerced.
func ValueNode(v config.Value, nested bool) (yaml.Node, error) {
	if v.Secure() {
		if v.Object() {
			return yaml.Node{}, ErrSecretObject
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

// literalValue escapes the strings and keys in v and rejects fn:: keys.
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
			if isReservedName(k, len(v) == 1) {
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

// secretNode renders ciphertext as fn::secret.
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
