package engine

import "github.com/pulumi/pulumi-fabric/pkg/tokens"

func InitEnv(name string) error {
	createEnv(tokens.QName(name))
	return nil
}
