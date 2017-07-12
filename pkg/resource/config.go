// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"sort"

	"github.com/pulumi/lumi/pkg/tokens"
)

// ConfigMap contains a mapping from variable token to the value to poke into that variable.
type ConfigMap map[tokens.Token]interface{}

func (config ConfigMap) StableKeys() []tokens.Token {
	sorted := make([]tokens.Token, 0, len(config))
	for key := range config {
		sorted = append(sorted, key)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted
}
