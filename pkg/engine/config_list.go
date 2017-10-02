// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

func (eng *Engine) GetConfiguration(environment string) (map[tokens.ModuleMember]string, error) {
	info, err := eng.initEnvCmdName(tokens.QName(environment), "")
	if err != nil {
		return nil, err
	}

	return info.Target.Config, nil
}
