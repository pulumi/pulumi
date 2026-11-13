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

package cli

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/eval"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// valueParent locates the given value path within an environment definition. It returns the node the value lives
// under, and the value's path relative to that node.
//
// Most values live under the definition's `values` key. The `pulumiConfig`, `environmentVariables`, and `files`
// sections may instead be declared at the top level of the definition, in which case the parent is the document
// itself. See [eval.DefinitionPath].
//
// If the definition has no `values` key and the value belongs there, valueParent creates the key when create is set
// and otherwise returns a nil parent.
func valueParent(
	docNode *yaml.Node,
	path resource.PropertyPath,
	create bool,
) (*yaml.Node, resource.PropertyPath, error) {
	defPath := eval.DefinitionPath(docNode, path)
	if len(path) != 0 && len(defPath) == len(path) {
		// The path starts at a top-level section.
		return docNode, defPath, nil
	}

	valuesNode, ok := encoding.YAMLSyntax{Node: docNode}.Get(defPath[:1])
	if !ok {
		if !create {
			return nil, nil, nil
		}
		var err error
		valuesNode, err = encoding.YAMLSyntax{Node: docNode}.Set(nil, defPath[:1], yaml.Node{
			Kind: yaml.MappingNode,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("internal error: %w", err)
		}
	}
	return valuesNode, path, nil
}
