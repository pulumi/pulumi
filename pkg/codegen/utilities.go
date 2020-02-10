package codegen

import "sort"

type StringSet map[string]struct{}

func NewStringSet(values ...string) StringSet {
	s := StringSet{}
	for _, v := range values {
		s.Add(v)
	}
	return s
}

func (ss StringSet) Add(s string) {
	ss[s] = struct{}{}
}

func (ss StringSet) Has(s string) bool {
	_, ok := ss[s]
	return ok
}

func (ss StringSet) SortedValues() []string {
	values := make([]string, 0, len(ss))
	for v := range ss {
		values = append(values, v)
	}
	sort.Strings(values)
	return values
}

type Set map[interface{}]struct{}

func (s Set) Add(v interface{}) {
	s[v] = struct{}{}
}

func (s Set) Has(v interface{}) bool {
	_, ok := s[v]
	return ok
}
