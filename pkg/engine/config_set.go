package engine

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

func (eng *Engine) SetConfig(envName string, key string, value string) error {
	info, err := eng.initEnvCmdName(tokens.QName(envName), "")
	if err != nil {
		return err
	}
	config := info.Target.Config

	if config == nil {
		config = make(resource.ConfigMap)
		info.Target.Config = config
	}

	config[tokens.Token(key)] = value

	if !eng.saveEnv(info.Target, info.Snapshot, "", true) {
		return errors.Errorf("could not save configuration value")
	}

	return nil
}
