// Copyright 2016 Marapongo, Inc. All rights reserved.

package pack

import (
	"sort"

	"github.com/marapongo/mu/pkg/tokens"
)

func StableDependencies(deps Dependencies) []tokens.PackageName {
	sorted := make(packageNames, 0, len(deps))
	for dep := range deps {
		sorted = append(sorted, dep)
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
