// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func (eng *Engine) InitEnv(name tokens.QName) error {
	contract.Require(name != tokens.QName(""), "name")

	return eng.createEnv(name)
}
