package local

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cancel"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type EngineOperationContext struct {
	sigint chan os.Signal
	events chan engine.Event
}

func NewEngineOperationContext() (*EngineOperationContext, *engine.Context) {
	cancelContext, cancelSource := cancel.NewContext(nil)
	events := make(chan engine.Event)

	c := &EngineOperationContext{
		sigint: make(chan os.Signal),
		events: events,
	}

	go func() {
		for range c.sigint {
			// If we haven't yet received a SIGINT, call the cancellation func. Otherwise call the termination
			// func.
			if cancelContext.CancelErr() == nil {
				const message = "^C received; cancelling. If you would like to terminate immediately, press\n" +
					"again. Note that terminating immediately may lead to orphaned resources and other inconsistent\n" +
					"states."
				_, err := fmt.Fprintf(os.Stderr, message)
				contract.IgnoreError(err)
				cancelSource.Cancel()
			} else {
				_, err := fmt.Fprintf(os.Stderr, "^C received; terminating.")
				contract.IgnoreError(err)
				cancelSource.Terminate()
			}
		}
	}()
	signal.Notify(c.sigint, os.Interrupt)

	return c, &engine.Context{Cancel: cancelContext, Events: events}
}

func (c *EngineOperationContext) Events() <-chan engine.Event {
	return c.events
}

func (c *EngineOperationContext) Close() {
	signal.Stop(c.sigint)
	close(c.sigint)
	close(c.events)
}
