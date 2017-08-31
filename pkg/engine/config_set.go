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

	if err = eng.Environment.SaveEnvironment(info.Target, info.Snapshot); err != nil {
		return errors.Wrap(err, "could not save configuration value")
	}

	return nil
}

// ReplaceConfig sets the config for an environment to match `newConfig` and then saves
// the environment. Note that config values that were present in the old environment but are
// not present in `newConfig` will be removed from the environment
func (eng *Engine) ReplaceConfig(envName string, newConfig map[string]string) error {
	info, err := eng.initEnvCmdName(tokens.QName(envName), "")
	if err != nil {
		return err
	}
	config := make(resource.ConfigMap)

	for k, v := range newConfig {
		config[tokens.Token(k)] = v
	}

	info.Target.Config = config

	if err = eng.Environment.SaveEnvironment(info.Target, info.Snapshot); err != nil {
		return errors.Wrap(err, "could not save configuration value")
	}

	return nil
}
