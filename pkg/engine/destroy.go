// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func Destroy(update Update, events chan<- Event, opts UpdateOptions) (ResourceChanges, error) {
	contract.Require(update != nil, "update")

	defer func() { events <- cancelEvent() }()

	info, err := planContextFromUpdate(update)
	if err != nil {
		return nil, err
	}
	defer info.Close()

	emitter := makeEventEmitter(events, update)

	return deployLatest(info, deployOptions{
		UpdateOptions: opts,
		Destroy:       true,
		Events:        emitter,
		Diag:          newEventSink(emitter),
	})
}
