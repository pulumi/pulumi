// Copyright 2026, Pulumi Corporation.
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

package operations

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/engine"
)

type engineEventsKey struct{}

// ContextWithEngineEvents returns a context that makes preview and up send their engine events to events.
// The caller must receive from events while the command runs.
func ContextWithEngineEvents(ctx context.Context, events chan<- engine.Event) context.Context {
	return context.WithValue(ctx, engineEventsKey{}, events)
}

// withContextEngineEvents returns a channel that sends to both events and the context's channel.
// Call the returned func once the operation has returned.
func withContextEngineEvents(ctx context.Context, events chan<- engine.Event) (chan<- engine.Event, func()) {
	observer, _ := ctx.Value(engineEventsKey{}).(chan<- engine.Event)
	switch {
	case observer == nil:
		return events, func() {}
	case events == nil:
		return observer, func() {}
	}
	both := make(chan engine.Event)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range both {
			observer <- e
			events <- e
		}
	}()
	return both, func() {
		close(both)
		<-done
	}
}
