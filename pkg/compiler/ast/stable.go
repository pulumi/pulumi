// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ast

import (
	"sort"

	"github.com/pulumi/lumi/pkg/tokens"
)

func StableClassMembers(cm ClassMembers) []tokens.ClassMemberName {
	sorted := make(classMemberNames, 0, len(cm))
	for m := range cm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModules(mm Modules) []tokens.ModuleName {
	sorted := make(moduleNames, 0, len(mm))
	for m := range mm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModuleExports(me ModuleExports) []tokens.ModuleMemberName {
	sorted := make(moduleMemberNames, 0, len(me))
	for e := range me {
		sorted = append(sorted, e)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModuleMembers(mm ModuleMembers) []tokens.ModuleMemberName {
	sorted := make(moduleMemberNames, 0, len(mm))
	for m := range mm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

type classMemberNames []tokens.ClassMemberName

func (s classMemberNames) Len() int {
	return len(s)
}

func (s classMemberNames) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s classMemberNames) Less(i, j int) bool {
	return s[i] < s[j]
}

type moduleNames []tokens.ModuleName

func (s moduleNames) Len() int {
	return len(s)
}

func (s moduleNames) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s moduleNames) Less(i, j int) bool {
	return s[i] < s[j]
}

type moduleMemberNames []tokens.ModuleMemberName

func (s moduleMemberNames) Len() int {
	return len(s)
}

func (s moduleMemberNames) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s moduleMemberNames) Less(i, j int) bool {
	return s[i] < s[j]
}
