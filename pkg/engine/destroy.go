// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func Destroy(update Update, events chan<- Event, opts UpdateOptions) error {
	contract.Require(update != nil, "update")

	defer func() { events <- cancelEvent() }()

	info, err := planContextFromUpdate(update)
	if err != nil {
		return err
	}
	defer info.Close()

	return deployLatest(info, deployOptions{
		UpdateOptions: opts,

		Create:  false,
		Destroy: true,

		Events: events,
		Diag: newEventSink(events, diag.FormatOptions{
			Color: opts.Color,
		}),
	})
}
