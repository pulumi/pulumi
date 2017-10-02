// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import "github.com/pulumi/pulumi/pkg/tokens"

func (eng *Engine) SelectEnv(envName tokens.QName) error {
	eng.setCurrentEnv(envName, true)
	return nil
}
