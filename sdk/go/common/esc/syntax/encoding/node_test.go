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

package encoding

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/esc/syntax"
)

type NodeProperty struct {
	Key   Node `json:"key"`
	Value Node `json:"value"`
}

type Node struct {
	Literal any            `json:"literal,omitempty"`
	Array   []Node         `json:"array,omitempty"`
	Object  []NodeProperty `json:"object,omitempty"`
	Range   *hcl.Range     `json:"range,omitempty"`
}

func NewNode(n syntax.Node) Node {
	switch n := n.(type) {
	case nil:
		return Node{}
	case *syntax.NullNode:
		return Node{Range: n.Syntax().Range()}
	case *syntax.BooleanNode:
		return Node{Literal: n.Value(), Range: n.Syntax().Range()}
	case *syntax.NumberNode:
		return Node{Literal: n.Value(), Range: n.Syntax().Range()}
	case *syntax.StringNode:
		return Node{Literal: n.Value(), Range: n.Syntax().Range()}
	case *syntax.ArrayNode:
		arr := make([]Node, n.Len())
		for i := range arr {
			arr[i] = NewNode(n.Index(i))
		}
		return Node{Array: arr, Range: n.Syntax().Range()}
	case *syntax.ObjectNode:
		obj := make([]NodeProperty, n.Len())
		for i := 0; i < n.Len(); i++ {
			prop := n.Index(i)
			obj[i] = NodeProperty{Key: NewNode(prop.Key), Value: NewNode(prop.Value)}
		}
		return Node{Object: obj, Range: n.Syntax().Range()}
	default:
		panic(fmt.Errorf("unexpected node of type %T", n))
	}
}
