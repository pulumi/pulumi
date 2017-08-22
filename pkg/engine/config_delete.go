package engine

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

func DeleteConfig(envName string, key string) error {
	info, err := initEnvCmdName(tokens.QName(envName), "")
	if err != nil {
		return err
	}
	config := info.Target.Config

	if config != nil {
		delete(config, tokens.Token(key))

		if !saveEnv(info.Target, info.Snapshot, "", true) {
			return errors.Errorf("could not save configuration value")
		}
	}

	return nil
}
