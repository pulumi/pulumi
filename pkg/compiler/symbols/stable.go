// Copyright 2016 Pulumi, Inc. All rights reserved.

package symbols

import (
	"sort"

	"github.com/pulumi/coconut/pkg/tokens"
)

func StableClassMemberMap(cm ClassMemberMap) []tokens.ClassMemberName {
	sorted := make(classMemberNames, 0, len(cm))
	for m := range cm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModuleMap(mm ModuleMap) []tokens.ModuleName {
	sorted := make(moduleNames, 0, len(mm))
	for m := range mm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModuleExportMap(me ModuleExportMap) []tokens.ModuleMemberName {
	sorted := make(moduleMemberNames, 0, len(me))
	for e := range me {
		sorted = append(sorted, e)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModuleMemberMap(mm ModuleMemberMap) []tokens.ModuleMemberName {
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
