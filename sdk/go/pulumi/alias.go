// Copyright 2016-2020, Pulumi Corporation.
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
	"strings"

	"github.com/pkg/errors"
)

// Alias is a partial description of prior named used for a resource. It can be processed in the
// context of a resource creation to determine what the full aliased URN would be.
type Alias struct {
	// The URN that uniquely identifies a resource
	URN URNInput
	// The previous name of the resource.  If not provided, the current name of the resource is used.
	Name StringInput
	// The previous type of the resource.  If not provided, the current type of the resource is used.
	Type StringInput
	// The previous parent of the resource.  If not provided, the current parent of the resource is used.
	Parent Resource
	// The previous parent of the resource in URN format, mutually exclusive to 'Parent'
	ParentURN URNInput
	// The name of the previous stack of the resource.  If not provided, defaults to `context.GetStack()
	Stack StringInput
	// The previous project of the resource. If not provided, defaults to `context.GetProject()`.
	Project StringInput
}

func (a *Alias) isCollapsed() bool {
	return a.URN != nil
}

func (a *Alias) collapseToURN(defaultName string,
	defaultType string,
	defaultParent Resource,
	defaultProject string,
	defaultStack string) error {
	if a.URN != nil {
		return nil
	}

	n := a.Name
	if n == nil {
		n = String(defaultName)
	}
	t := a.Type
	if t == nil {
		t = String(defaultType)
	}
	var parent StringInput

	if a.Parent != nil && a.ParentURN != nil {
		return errors.New("alias can specify either Parent or ParentURN but not both")
	}
	if a.Parent != nil {
		parent = a.Parent.URN()
	}
	if a.ParentURN != nil {
		parent = a.ParentURN.ToURNOutput()
	}

	project := a.Project
	if project == nil {
		project = String(defaultProject)
	}
	stack := a.Stack
	if stack == nil {
		stack = String(defaultStack)
	}

	a.URN = createURN(n, t, parent, project, stack)

	return nil
}

func createURN(name StringInput, t StringInput, parent StringInput, project StringInput, stack StringInput) URNOutput {
	var parentPrefix StringInput
	if parent != nil {
		parentPrefix = parent.ToStringOutput().ApplyString(func(p string) string {
			return ""
		})
	} else {
		parentPrefix = All(stack, project).ApplyString(func(a []interface{}) string {
			return "urn:pulumi:" + a[0].(string) + "::" + a[1].(string) + "::"
		})

	}

	return All(parentPrefix, t, name).ApplyString(func(a []interface{}) string {
		return a[0].(string) + a[1].(string) + "::" + a[2].(string)
	}).ApplyURN(func(u string) URN {
		return URN(u)
	})
}

func inheritedChildAlias(childName string,
	parentName string,
	childType string,
	project string,
	stack string,
	parent Alias) (URNOutput, error) {
	if !parent.isCollapsed() {
		return URNInput(URN("")).ToURNOutput(), errors.New("Alias must be collapsed to create inherited alias")
	}

	aliasName := StringInput(String(childName))
	if strings.HasPrefix(childName, parentName) {
		aliasName = parent.URN.ToURNOutput().ApplyString(func(u string) string {
			parentName := u[strings.LastIndex(u, "::")+2:]
			return parentName + childName[len(parentName):]
		})
	}
	toStrIn := func(s string) StringInput { return StringInput(String(s)) }

	return createURN(aliasName, toStrIn(childType), parent.URN.ToURNOutput(), toStrIn(project), toStrIn(stack)), nil
}
