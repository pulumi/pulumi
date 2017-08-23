package engine

import "github.com/pulumi/pulumi-fabric/pkg/tokens"

func (eng *Engine) Destroy(envName string, pkgarg string, dryRun bool, debug bool, summary bool) error {
	info, err := eng.initEnvCmdName(tokens.QName(envName), pkgarg)
	if err != nil {
		return err
	}

	return eng.deployLatest(info, deployOptions{
		Debug:   debug,
		Destroy: true,
		DryRun:  dryRun,
		Summary: summary,
	})
}
