// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"sort"

	"github.com/marapongo/mu/pkg/pack/symbols"
)

func StableClassMembers(cm ClassMembers) Tokens {
	sorted := make(Tokens, 0, len(cm))
	for m := range cm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModules(mm Modules) ModuleTokens {
	sorted := make(ModuleTokens, 0, len(mm))
	for m := range mm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

func StableModuleMembers(mm ModuleMembers) Tokens {
	sorted := make(Tokens, 0, len(mm))
	for m := range mm {
		sorted = append(sorted, m)
	}
	sort.Sort(sorted)
	return sorted
}

type Tokens []symbols.Token

func (s Tokens) Len() int {
	return len(s)
}

func (s Tokens) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Tokens) Less(i, j int) bool {
	return s[i] < s[j]
}

type ModuleTokens []symbols.ModuleToken

func (s ModuleTokens) Len() int {
	return len(s)
}

func (s ModuleTokens) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ModuleTokens) Less(i, j int) bool {
	return s[i] < s[j]
}
