// Copyright 2016-2021, Pulumi Corporation.
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

package pulumi

import (
	"errors"
	"fmt"
	"strings"
)

// Alias is a partial description of prior named used for a resource. It can be processed in the
// context of a resource creation to determine what the full aliased URN would be.
type Alias struct {
	// Optional URN that uniquely identifies a resource. If specified, it takes preference and
	// other members of the struct are ignored.
	URN *Output[URN]
	// The previous name of the resource.  If not provided, the current name of the resource is used.
	Name *Output[string]
	// The previous type of the resource.  If not provided, the current type of the resource is used.
	Type *Output[string]
	// The previous parent of the resource. If not provided, the current parent of the resource is used by default.
	// This option is mutually exclusive to `ParentURN` and `NoParent`.
	// Use `Alias { NoParent: pulumi.Bool(true) }` to avoid defaulting to the current parent.
	Parent *Resource
	// The previous parent of the resource in URN format, mutually exclusive to `Parent` and `ParentURN`.
	// To specify no original parent, use `Alias { NoParent: pulumi.Bool(true) }`.
	ParentURN *Output[URN]
	// When true, indicates that the resource previously had no parent.
	// This option is mutually exclusive to `Parent` and `ParentURN`.
	NoParent *Output[bool]
	// The name of the previous stack of the resource.  If not provided, defaults to `context.GetStack()
	Stack *Output[string]
	// The previous project of the resource. If not provided, defaults to `context.GetProject()`.
	Project *Output[string]
}

// More then one bool is set to true.
func multipleTrue(booleans ...bool) bool {
	var found bool
	for _, b := range booleans {
		if b && found {
			return true
		} else if b {
			found = true
		}
	}
	return false
}

func (a Alias) collapseToURN(defaultName, defaultType string, defaultParent Resource,
	defaultProject, defaultStack string) (*Output[URN], error) {

	if a.URN != nil {
		return a.URN, nil
	}

	var n Output[string]
	if a.Name != nil {
		n = *a.Name
	} else {
		n = In(defaultName)
	}
	var t Output[string]
	if a.Type != nil {
		t = *a.Type
	} else {
		t = In(defaultType)
	}

	var parent Output[string] = In("")
	if defaultParent != nil {
		parent = ToString(defaultParent.URN())
	}
	if multipleTrue(a.Parent != nil, a.ParentURN != nil, a.NoParent != nil) {
		return nil, errors.New("alias can specify Parent, ParentURN or NoParent but not more then one")
	}
	if a.Parent != nil {
		parent = ToString((*a.Parent).URN())
	}
	if a.ParentURN != nil {
		parent = ToString(*a.ParentURN)
	}
	if a.NoParent != nil {
		parent = Cast[string](
			All(a.NoParent, parent).ApplyT(func(arr interface{}) interface{} {
				a := arr.([]interface{})
				if a[0].(bool) {
					return ""
				}
				return a[1].(string)
			}),
		)
	}

	var project Output[string]
	if a.Project != nil {
		project = *a.Project
	} else {
		project = In(defaultProject)
	}
	var stack Output[string]
	if a.Stack != nil {
		stack = *a.Stack
	} else {
		stack = In(defaultStack)
	}

	urn := CreateURN(n, t, &parent, project, stack)
	return &urn, nil
}

// CreateURN computes a URN from the combination of a resource name, resource type, and optional parent,
func CreateURN(name, t Output[string], parent *Output[string], project, stack Output[string]) Output[URN] {
	var par Output[string]
	if parent != nil {
		par = *parent
	} else {
		par = In("")
	}
	createURN := func(parent, stack, project, t, name string) URN {
		var parentPrefix string
		if parent == "" {
			parentPrefix = "urn:pulumi:" + stack + "::" + project + "::"
		} else {
			ix := strings.LastIndex(parent, "::")
			if ix == -1 {
				panic(fmt.Sprintf("Expected 'parent' string '%s' to contain '::'", parent))
			}
			parentPrefix = parent[0:ix] + "$"
		}
		return URN(parentPrefix + t + "::" + name)
	}
	// The explicit call to `ToStringOutput` is necessary because `Output[URN]`
	// conforms to `Output[string]` so `parent.(string)` can fail without the
	// explicit conversion.
	return Cast[URN](
		All(par, stack, project, t, name).ApplyT(func(arr interface{}) interface{} {
			a := arr.([]interface{})
			return createURN(a[0].(string), a[1].(string), a[2].(string), a[3].(string), a[4].(string))
		}),
	)
}

// inheritedChildAlias computes the alias that should be applied to a child based on an alias applied to it's parent.
// This may involve changing the name of the resource in cases where the resource has a named derived from the name of
// the parent, and the parent name changed.
func inheritedChildAlias(childName, parentName, childType, project, stack string, parentURN Output[URN]) Output[URN] {
	aliasName := In(childName)
	if strings.HasPrefix(childName, parentName) {
		aliasName = Apply(parentURN, func(urn URN) string {
			parentPrefix := urn[strings.LastIndex(string(urn), "::")+2:]
			return string(parentPrefix) + childName[len(parentName):]
		})
	}
	parent := ToString(parentURN)
	return CreateURN(aliasName, In(childType), &parent, In(project), In(stack))
}
