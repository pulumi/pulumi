package engine

import "github.com/pulumi/pulumi-fabric/pkg/tokens"

type DestroyOptions struct {
	Environment string
	Package     string
	DryRun      bool
	Debug       bool
	Summary     bool
}

func (eng *Engine) Destroy(opts DestroyOptions) error {
	info, err := eng.initEnvCmdName(tokens.QName(opts.Environment), opts.Package)
	if err != nil {
		return err
	}

	return eng.deployLatest(info, deployOptions{
		Debug:   opts.Debug,
		Destroy: true,
		DryRun:  opts.DryRun,
		Summary: opts.Summary,
	})
}
