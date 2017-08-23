package engine

import (
	"fmt"

	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

func ListConfig(envName string) error {
	info, err := initEnvCmdName(tokens.QName(envName), "")
	if err != nil {
		return err
	}
	config := info.Target.Config

	if config != nil {
		fmt.Fprintf(E.Stdout, "%-32s %-32s\n", "KEY", "VALUE")
		for _, key := range info.Target.Config.StableKeys() {
			v := info.Target.Config[key]
			// TODO[pulumi/pulumi-fabric#113]: print complex values.
			fmt.Fprintf(E.Stdout, "%-32s %-32s\n", key, v)
		}
	}
	return nil
}
