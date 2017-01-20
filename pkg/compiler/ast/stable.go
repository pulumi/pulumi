// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"sort"

	"github.com/marapongo/mu/pkg/tokens"
)

func StableClassMembers(cm ClassMembers) Names {
	sorted := make(Names, 0, len(cm))
	for m := range cm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModules(mm Modules) Names {
	sorted := make(Names, 0, len(mm))
	for m := range mm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModuleMembers(mm ModuleMembers) Names {
	sorted := make(Names, 0, len(mm))
	for m := range mm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

type Names []tokens.Name

func (s Names) Len() int {
	return len(s)
}

func (s Names) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Names) Less(i, j int) bool {
	return s[i] < s[j]
}
