package main

import (
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type Renderer struct {
	events   chan events.Event
	canceled bool
	done     chan bool
}

func (r *Renderer) Render(event events.Event) {
	r.events <- event
	if event.Type == events.CancelEvent {
		r.canceled = true
		close(r.events)
	}
}

func (r *Renderer) Close() error {
	if !r.canceled {
		r.events <- events.NewEvent(events.CancelEvent, nil)
		close(r.events)
		<-r.done
	}
	return nil
}

func NewRenderer(op string, action apitype.UpdateKind, stack tokens.QName, proj tokens.PackageName, isPreview bool, opts display.Options) *Renderer {
	eventsC := make(chan events.Event)
	done := make(chan bool)

	go display.ShowProgressEvents(op, action, stack, proj, eventsC, done, opts, isPreview)

	return &Renderer{
		events: eventsC,
		done:   done,
	}
}
