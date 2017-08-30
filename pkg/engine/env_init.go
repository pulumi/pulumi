// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import "github.com/pulumi/pulumi-fabric/pkg/tokens"

func (eng *Engine) InitEnv(name string) error {
	eng.createEnv(tokens.QName(name))
	return nil
}
