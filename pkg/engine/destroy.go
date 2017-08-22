package engine

import "github.com/pulumi/pulumi-fabric/pkg/tokens"

func Destroy(envName string, pkgarg string, dryRun bool, debug bool, summary bool) error {
	info, err := initEnvCmdName(tokens.QName(envName), pkgarg)
	if err != nil {
		return err
	}

	return deployLatest(info, deployOptions{
		Debug:   debug,
		Destroy: true,
		DryRun:  dryRun,
		Summary: summary,
	})
}
