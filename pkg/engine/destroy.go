// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type DestroyOptions struct {
	Package  string
	DryRun   bool
	Debug    bool
	Parallel int
	Summary  bool
}

func (eng *Engine) Destroy(environment tokens.QName, opts DestroyOptions) error {
	contract.Require(environment != tokens.QName(""), "environment")

	info, err := eng.initEnvCmdName(environment, opts.Package)
	if err != nil {
		return err
	}

	return eng.deployLatest(info, deployOptions{
		Debug:    opts.Debug,
		Destroy:  true,
		DryRun:   opts.DryRun,
		Parallel: opts.Parallel,
		Summary:  opts.Summary,
	})
}
