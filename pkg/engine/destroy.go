// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type DestroyOptions struct {
	DryRun   bool
	Parallel int
	Summary  bool
	Color    colors.Colorization
}

func Destroy(update Update, events chan<- Event, opts DestroyOptions) error {
	contract.Require(update != nil, "update")

	defer func() { events <- cancelEvent() }()

	info, err := planContextFromUpdate(update)
	if err != nil {
		return err
	}
	defer info.Close()

	return deployLatest(info, deployOptions{
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
