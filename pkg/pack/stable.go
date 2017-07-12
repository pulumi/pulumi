// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package pack

import (
	"sort"

	"github.com/pulumi/lumi/pkg/tokens"
)

func StableDependencies(deps Dependencies) []tokens.PackageName {
	sorted := make(packageNames, 0, len(deps))
	for dep := range deps {
		sorted = append(sorted, dep)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModuleAliases(aliases ModuleAliases) []tokens.ModuleName {
	sorted := make(moduleNames, 0, len(aliases))
	for alias := range aliases {
		sorted = append(sorted, alias)
	}
	sort.Sort(sorted)
	return sorted
}

type packageNames []tokens.PackageName

func (s packageNames) Len() int {
	return len(s)
}

func (s packageNames) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s packageNames) Less(i, j int) bool {
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
