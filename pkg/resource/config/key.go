// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package config

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

type Key = tokens.ModuleMember

func ParseKey(key string) (Key, error) {
	return tokens.ParseModuleMember(key)
}
