// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type DestroyOptions struct {
	Package  string
	DryRun   bool
	Parallel int
	Summary  bool
	Color    colors.Colorization
}

func (eng *Engine) Destroy(stack tokens.QName, events chan<- Event, opts DestroyOptions) error {
	contract.Require(stack != tokens.QName(""), "stack")

	defer func() { events <- cancelEvent() }()

	info, err := eng.planContextFromStack(stack, opts.Package)
	if err != nil {
		return err
	}
	defer info.Close()

	return eng.deployLatest(info, deployOptions{
		Destroy:  true,
		DryRun:   opts.DryRun,
		Parallel: opts.Parallel,
		Summary:  opts.Summary,
		Color:    opts.Color,
		Events:   events,
		Diag: newEventSink(events, diag.FormatOptions{
			Color: opts.Color,
		}),
	})
}
