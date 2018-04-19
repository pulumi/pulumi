package local

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type EngineOperationContext struct {
	sigint chan os.Signal
	events chan engine.Event
}

func NewEngineOperationContext() (*EngineOperationContext, *engine.Context) {
	cancellationContext, cancel := context.WithCancel(context.Background())
	terminationContext, terminate := context.WithCancel(context.Background())
	events := make(chan engine.Event)

	c := &EngineOperationContext{
		sigint: make(chan os.Signal),
		events: events,
	}

	go func() {
		for range c.sigint {
			// If we haven't yet received a SIGINT, call the cancellation func. Otherwise call the termination
			// func.
			if cancellationContext.Err() == nil {
				const message = "^C received; cancelling. If you would like to terminate immediately, press\n" +
					"again. Note that terminating immediately may lead to orphaned resources and other inconsistent\n" +
					"states."
				_, err := fmt.Fprintf(os.Stderr, message)
				contract.IgnoreError(err)
				cancel()
			} else {
				_, err := fmt.Fprintf(os.Stderr, "^C received; terminating.")
				contract.IgnoreError(err)
				terminate()
			}
		}
	}()
	signal.Notify(c.sigint, os.Interrupt)

	return c, engine.NewContext(cancellationContext, terminationContext, events)
}

func (c *EngineOperationContext) Events() <-chan engine.Event {
	return c.events
}

func (c *EngineOperationContext) Close() {
	signal.Stop(c.sigint)
	close(c.sigint)
	close(c.events)
}
