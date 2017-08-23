package engine

import "github.com/pulumi/pulumi-fabric/pkg/tokens"

func (eng *Engine) SelectEnv(envName string) error {
	eng.setCurrentEnv(tokens.QName(envName), true)
	return nil
}
