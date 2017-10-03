// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func (eng *Engine) GetConfiguration(environment tokens.QName) (map[tokens.ModuleMember]string, error) {
	contract.Require(environment != tokens.QName(""), "environment")

	info, err := eng.initEnvCmdName(environment, "")
	if err != nil {
		return nil, err
	}

	return info.Target.Config, nil
}
