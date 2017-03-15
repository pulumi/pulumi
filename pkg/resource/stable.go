// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

import (
	"sort"
)

func StablePropertyKeys(props PropertyMap) []PropertyKey {
	sorted := make(propertyKeys, 0, len(props))
	for prop := range props {
		sorted = append(sorted, prop)
	}
	sort.Sort(sorted)
	return sorted
}

type propertyKeys []PropertyKey

func (s propertyKeys) Len() int {
	return len(s)
}

func (s propertyKeys) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s propertyKeys) Less(i, j int) bool {
	return s[i] < s[j]
}
