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
	URN URNInput
	// The previous name of the resource.  If not provided, the current name of the resource is used.
	Name StringInput
	// The previous type of the resource.  If not provided, the current type of the resource is used.
	Type StringInput
	// The previous parent of the resource. If not provided, the current parent of the resource is used by default.
	// This option is mutually exclusive to `ParentURN` and `NoParent`.
	// Use `Alias { NoParent: pulumi.Bool(true) }` to avoid defaulting to the current parent.
	Parent Resource
	// The previous parent of the resource in URN format, mutually exclusive to `Parent` and `ParentURN`.
	// To specify no original parent, use `Alias { NoParent: pulumi.Bool(true) }`.
	ParentURN URNInput
	// When true, indicates that the resource previously had no parent.
	// This option is mutually exclusive to `Parent` and `ParentURN`.
	NoParent BoolInput
	// The name of the previous stack of the resource.  If not provided, defaults to `context.GetStack()
	Stack StringInput
	// The previous project of the resource. If not provided, defaults to `context.GetProject()`.
	Project StringInput
}

// More than one bool is set to true.
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
	defaultProject, defaultStack string) (URNOutput, error) {

	if a.URN != nil {
		return a.URN.ToURNOutput(), nil
	}

	n := a.Name
	if n == nil {
		n = String(defaultName)
	}
	t := a.Type
	if t == nil {
		t = String(defaultType)
	}

	var parent StringInput = String("")
	if defaultParent != nil {
		parent = defaultParent.URN().ToStringOutput()
	}
	if multipleTrue(a.Parent != nil, a.ParentURN != nil, a.NoParent != nil) {
		return URNOutput{}, errors.New("alias can specify Parent, ParentURN or NoParent but not more then one")
	}
	if a.Parent != nil {
		parent = a.Parent.URN().ToStringOutput()
	}
	if a.ParentURN != nil {
		parent = a.ParentURN.ToURNOutput()
	}
	if a.NoParent != nil {
		parent = All(a.NoParent.ToBoolOutput(), parent).ApplyT(func(a []interface{}) string {
			if a[0].(bool) {
				return ""
			}
			return a[1].(string)
		}).(StringOutput)
	}

	project := a.Project
	if project == nil {
		project = String(defaultProject)
	}
	stack := a.Stack
	if stack == nil {
		stack = String(defaultStack)
	}

	return CreateURN(n, t, parent, project, stack), nil
}

// CreateURN computes a URN from the combination of a resource name, resource type, and optional parent,
func CreateURN(name, t, parent, project, stack StringInput) URNOutput {
	if parent == nil {
		parent = String("")
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
	// The explicit call to `ToStringOutput` is necessary because `URNOutput`
	// conforms to `StringInput` so `parent.(string)` can fail without the
	// explicit conversion.
	return All(parent.ToStringOutput(), stack, project, t, name).ApplyT(func(a []interface{}) URN {
		return createURN(a[0].(string), a[1].(string), a[2].(string), a[3].(string), a[4].(string))
	}).(URNOutput)
}

// inheritedChildAlias computes the alias that should be applied to a child based on an alias applied to it's parent.
// This may involve changing the name of the resource in cases where the resource has a named derived from the name of
// the parent, and the parent name changed.
func inheritedChildAlias(childName, parentName, childType, project, stack string, parentURN URNOutput) URNOutput {
	aliasName := StringInput(String(childName))
	if strings.HasPrefix(childName, parentName) {
		aliasName = parentURN.ApplyT(func(urn URN) string {
			parentPrefix := urn[strings.LastIndex(string(urn), "::")+2:]
			return string(parentPrefix) + childName[len(parentName):]
		}).(StringOutput)
	}
	return CreateURN(aliasName, String(childType), parentURN.ToStringOutput(), String(project), String(stack))
}
