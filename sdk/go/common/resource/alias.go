// Copyright 2022-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"fmt"
	"strings"
)

type Alias struct {
	URN      URN
	Name     string
	Type     string
	Project  string
	Stack    string
	Parent   URN
	NoParent bool
}

func (a *Alias) GetURN() URN {
	if a.URN != "" {
		return a.URN
	}
	return CreateURN(a.Name, a.Type, a.Parent, a.Project, a.Stack)
}

// CreateURN computes a URN from the combination of a resource name, resource type, and optional parent,
func CreateURN(name string, t string, parent URN, project string, stack string) URN {
	createURN := func(parent URN, stack string, project string, t string, name string) URN {
		parentString := string(parent)
		var parentPrefix string
		if parent == "" {
			parentPrefix = "urn:pulumi:" + stack + "::" + project + "::"
		} else {
			ix := strings.LastIndex(parentString, "::")
			if ix == -1 {
				panic(fmt.Sprintf("Expected 'parent' string '%s' to contain '::'", parent))
			}
			parentPrefix = parentString[0:ix] + "$"
		}
		return URN(parentPrefix + t + "::" + name)
	}
	return createURN(parent, stack, project, t, name)
}
