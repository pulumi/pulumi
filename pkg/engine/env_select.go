package engine

import "github.com/pulumi/pulumi-fabric/pkg/tokens"

func SelectEnv(envName string) error {
	setCurrentEnv(tokens.QName(envName), true)
	return nil
}
