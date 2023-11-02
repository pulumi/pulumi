// Copyright 2023, Pulumi Corporation.
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

package syntax

// Walk recursively walks the tree rooted at the given node. The visitor is called after the Node's descendants have
// been walked (i.e. visitation is post-order).
func Walk(n Node, visitor func(n Node) (Node, Diagnostics, error)) (Node, Diagnostics, error) {
	var diags Diagnostics
	switch n := n.(type) {
	case *ArrayNode:
		for i, e := range n.elements {
			m, d, err := Walk(e, visitor)
			diags.Extend(d...)
			if err != nil {
				return nil, diags, err
			}
			n.elements[i] = m
		}
	case *ObjectNode:
		for i, e := range n.entries {
			v, d, err := Walk(e.Value, visitor)
			diags.Extend(d...)
			if err != nil {
				return nil, diags, err
			}
			n.entries[i] = ObjectProperty(e.Key, v)
		}
	}

	n, d, err := visitor(n)
	diags.Extend(d...)
	return n, diags, err
}
