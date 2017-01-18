// Copyright 2016 Marapongo, Inc. All rights reserved.

package workspace

import (
	"sort"

	"github.com/marapongo/mu/pkg/tokens"
)

func StableClusters(cs Clusters) []string {
	sorted := make([]string, 0, len(cs))
	for c := range cs {
		sorted = append(sorted, c)
	}
	sort.Strings(sorted)
	return sorted
}

func StableDependencies(ds Dependencies) []tokens.Ref {
	sorted := make(refs, 0, len(ds))
	for d := range ds {
		sorted = append(sorted, d)
	}
	sort.Sort(sorted)
	return sorted
}

type refs []tokens.Ref

func (s refs) Len() int {
	return len(s)
}

func (s refs) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s refs) Less(i, j int) bool {
	return s[i] < s[j]
}
