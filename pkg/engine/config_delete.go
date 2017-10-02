// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/tokens"
)

func (eng *Engine) DeleteConfig(envName tokens.QName, key tokens.ModuleMember) error {
	info, err := eng.initEnvCmdName(envName, "")
	if err != nil {
		return err
	}
	config := info.Target.Config

	if config != nil {
		delete(config, key)

		if err = eng.Environment.SaveEnvironment(info.Target, info.Snapshot); err != nil {
			return errors.Wrap(err, "could not save configuration value")
		}
	}

	return nil
}
