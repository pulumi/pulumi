// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import "github.com/pulumi/pulumi/pkg/tokens"

func (eng *Engine) InitEnv(name tokens.QName) error {
	eng.createEnv(name)
	return nil
}
