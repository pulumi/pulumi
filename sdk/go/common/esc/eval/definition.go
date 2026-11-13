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

package eval

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/ast"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// topLevelKey returns the spelling of the last top-level key of the document that matches the given key. Top-level
// keys are case-insensitive and the last match wins, per the parser.
func topLevelKey(doc *yaml.Node, key string) (string, bool) {
	if doc != nil && doc.Kind == yaml.DocumentNode && len(doc.Content) != 0 {
		doc = doc.Content[0]
	}
	if doc == nil || doc.Kind != yaml.MappingNode {
		return "", false
	}
	for i := len(doc.Content) - 2; i >= 0; i -= 2 {
		if k := doc.Content[i].Value; strings.EqualFold(k, key) {
			return k, true
		}
	}
	return "", false
}

// DefinitionPath returns the path within an environment definition document at which the given logical path lives.
//
// A logical path is either a path into the environment's imports, which starts with "imports", or a path into its
// values. An environment may spell its imports as either `imports` or `environments`, and may declare each of the
// `pulumiConfig`, `environmentVariables`, and `files` sections either inside `values` or at the top level.
// DefinitionPath resolves the logical path to the spelling the document uses, so that edits land where the evaluator
// reads. Paths the document does not define yet resolve to the legacy spelling: `imports`, or a key inside `values`.
func DefinitionPath(doc *yaml.Node, path resource.PropertyPath) resource.PropertyPath {
	if len(path) == 0 {
		return resource.PropertyPath{"values"}
	}
	rest := path[1:]

	name, _ := path[0].(string)
	if name == "imports" {
		key := "imports"
		if _, ok := topLevelKey(doc, "imports"); !ok {
			if k, ok := topLevelKey(doc, "environments"); ok {
				key = k
			}
		}
		return append(resource.PropertyPath{key}, rest...)
	}

	legacy := append(resource.PropertyPath{"values"}, path...)
	keys := ast.TopLevelSectionKeys(name)
	if len(keys) == 0 {
		return legacy
	}

	// The legacy spelling wins when both are present.
	if valuesKey, ok := topLevelKey(doc, "values"); ok {
		legacy[0] = valuesKey
		if values := mappingValue(doc, valuesKey); values != nil && mappingValue(values, name) != nil {
			return legacy
		}
	}
	for _, key := range keys {
		if k, ok := topLevelKey(doc, key); ok {
			return append(resource.PropertyPath{k}, rest...)
		}
	}
	return legacy
}

// mappingValue returns the value of the last entry of the given mapping with exactly the given key.
func mappingValue(n *yaml.Node, key string) *yaml.Node {
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) != 0 {
		n = n.Content[0]
	}
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := len(n.Content) - 2; i >= 0; i -= 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
