// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"sort"

	"github.com/marapongo/mu/pkg/symbols"
)

func StableClusters(cs Clusters) []string {
	sorted := make([]string, 0, len(cs))
	for c := range cs {
		sorted = append(sorted, c)
	}
	sort.Strings(sorted)
	return sorted
}

func StableDependencies(ds Dependencies) []symbols.Ref {
	sorted := make(Refs, 0, len(ds))
	for d := range ds {
		sorted = append(sorted, d)
	}
	sort.Sort(sorted)
	return sorted
}

func StableDependencyRefs(refs DependencyRefs) []symbols.Ref {
	sorted := make(Refs, 0, len(refs))
	for ref := range refs {
		sorted = append(sorted, ref)
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

func StableSchemas(ss SchemaMap) []symbols.Name {
	sorted := make(Names, 0, len(ss))
	for s := range ss {
		sorted = append(sorted, s)
	}
	sort.Sort(sorted)
	return sorted
}

func StableServices(ss ServiceMap) []symbols.Name {
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

func StableUntypedServices(ss UntypedServiceMap) []symbols.Name {
	sorted := make(Names, 0, len(ss))
	for s := range ss {
		sorted = append(sorted, s)
	}
	sort.Sort(sorted)
	return sorted
}

type Names []symbols.Name

func (s Names) Len() int {
	return len(s)
}

func (s Names) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Names) Less(i, j int) bool {
	return s[i] < s[j]
}

type Refs []symbols.Ref

func (s Refs) Len() int {
	return len(s)
}

func (s Refs) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Refs) Less(i, j int) bool {
	return s[i] < s[j]
}
