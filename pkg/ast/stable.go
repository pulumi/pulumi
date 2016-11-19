// Copyright 2016 Marapongo, Inc. All rights reserved.

package ast

import (
	"sort"
)

func StableDependencies(d Dependencies) []Name {
	deps := make(Names, 0, len(d))
	for dep := range d {
		deps = append(deps, dep)
	}
	sort.Sort(deps)
	return deps
}

func StableParameters(p Parameters) []string {
	params := make([]string, 0, len(p))
	for param := range p {
		params = append(params, param)
	}
	sort.Strings(params)
	return params
}

func StableServices(s ServiceMap) []Name {
	svcs := make(Names, 0, len(s))
	for svc := range s {
		svcs = append(svcs, svc)
	}
	sort.Sort(svcs)
	return svcs
}

func StableUntypedServices(s UntypedServiceMap) []Name {
	svcs := make(Names, 0, len(s))
	for svc := range s {
		svcs = append(svcs, svc)
	}
	sort.Sort(svcs)
	return svcs
}

func StableTargets(t Targets) []string {
	targets := make([]string, 0, len(t))
	for target := range t {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

type Names []Name

func (s Names) Len() int {
	return len(s)
}

func (s Names) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Names) Less(i, j int) bool {
	return s[i] < s[j]
}
