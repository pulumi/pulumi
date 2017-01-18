// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"sort"

	"github.com/marapongo/mu/pkg/tokens"
)

func StableDependencyRefs(drs DependencyRefs) []tokens.Ref {
	sorted := make(refs, 0, len(drs))
	for dr := range drs {
		sorted = append(sorted, dr)
	}
	sort.Sort(sorted)
	return sorted
}

func StableKeys(ps PropertyBag) []string {
	sorted := make([]string, 0, len(ps))
	for p := range ps {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	return sorted
}

func StableProperties(ps Properties) []string {
	sorted := make([]string, 0, len(ps))
	for p := range ps {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	return sorted
}

func StablePropertyBag(ps PropertyBag) []string {
	sorted := make([]string, 0, len(ps))
	for p := range ps {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	return sorted
}

func StableSchemas(ss SchemaMap) []tokens.Name {
	sorted := make(Names, 0, len(ss))
	for s := range ss {
		sorted = append(sorted, s)
	}
	sort.Sort(sorted)
	return sorted
}

func StableServices(ss ServiceMap) []tokens.Name {
	sorted := make(Names, 0, len(ss))
	for s := range ss {
		sorted = append(sorted, s)
	}
	sort.Sort(sorted)
	return sorted
}

func StableStringStringMap(ssm map[string]string) []string {
	sorted := make([]string, 0, len(ssm))
	for s := range ssm {
		sorted = append(sorted, s)
	}
	sort.Strings(sorted)
	return sorted
}

func StableUntypedServices(ss UntypedServiceMap) []tokens.Name {
	sorted := make(Names, 0, len(ss))
	for s := range ss {
		sorted = append(sorted, s)
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
