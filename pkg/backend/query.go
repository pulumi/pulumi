// Copyright 2019-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
)

type MakeQuery func(context.Context, QueryOperation) (engine.QueryInfo, error)

// RunQuery executes a query program against the resource outputs of a diy hosted stack.
func RunQuery(ctx context.Context, b Backend, op QueryOperation,
	callerEventsOpt chan<- engine.Event, newQuery MakeQuery,
) error {
	q, err := newQuery(ctx, op)
	if err != nil {
		return err
	}

	// Render query output to CLI.
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)
	go display.ShowQueryEvents("running query", displayEvents, displayDone, op.Opts.Display)

	// The engineEvents channel receives all events from the engine, which we then forward onto other
	// channels for actual processing. (displayEvents and callerEventsOpt.)
	engineEvents := make(chan engine.Event)
	eventsDone := make(chan bool)
	go func() {
		for e := range engineEvents {
			displayEvents <- e
			if callerEventsOpt != nil {
				callerEventsOpt <- e
			}
		}

		close(eventsDone)
	}()

	// Depending on the action, kick off the relevant engine activity.  Note that we don't immediately check and
	// return error conditions, because we will do so below after waiting for the display channels to close.
	cancellationScope := op.Scopes.NewScope(engineEvents, true /*dryRun*/)
	engineCtx := &engine.Context{
		Cancel:        cancellationScope.Context(),
		Events:        engineEvents,
		BackendClient: NewBackendClient(b, op.SecretsProvider),
	}
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		engineCtx.ParentSpan = parentSpan.Context()
	}

	res := engine.Query(engineCtx, q, op.Opts.Engine)

	// Wait for dependent channels to finish processing engineEvents before closing.
	<-displayDone
	cancellationScope.Close() // Don't take any cancellations anymore, we're shutting down.
	close(engineEvents)

	// Make sure that the goroutine writing to displayEvents and callerEventsOpt
	// has exited before proceeding
	<-eventsDone
	close(displayEvents)

	return res
}
