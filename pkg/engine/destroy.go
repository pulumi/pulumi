// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/diag"
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

func (eng *Engine) Destroy(environment tokens.QName, events chan Event, opts DestroyOptions) error {
	contract.Require(environment != tokens.QName(""), "environment")

	info, err := eng.planContextFromEnvironment(environment, opts.Package)
	if err != nil {
		return err
	}

	diag := newEventSink(events, diag.FormatOptions{
		Colors: true,
		Debug:  opts.Debug,
	})

	defer close(events)

	return eng.deployLatest(info, deployOptions{
		Debug:    opts.Debug,
		Destroy:  true,
		DryRun:   opts.DryRun,
		Parallel: opts.Parallel,
		Summary:  opts.Summary,
		Events:   events,
		Diag:     diag,
	})
}
