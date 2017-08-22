package engine

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

func GetConfig(envName string, key string) error {
	info, err := initEnvCmdName(tokens.QName(envName), "")
	if err != nil {
		return err
	}
	config := info.Target.Config

	if config != nil {
		if v, has := config[tokens.Token(key)]; has {
			fmt.Printf("%v\n", v)
			return nil
		}
	}

	return errors.Errorf("configuration key '%v' not found for environment '%v'", key, info.Target.Name)
}
