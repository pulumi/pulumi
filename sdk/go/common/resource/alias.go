package resource

import (
	"fmt"
	"strings"
)

type Alias struct {
	URN     URN
	Name    string
	Type    string
	Project string
	Stack   string
	Parent  URN
}

func (a *Alias) GetURN() URN {
	if a.URN != "" {
		return a.URN
	}
	return CreateURN(a.Name, a.Type, a.Parent, a.Project, a.Stack)
}

func (a *Alias) NoParent() bool {
	return a.Parent == ""
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
