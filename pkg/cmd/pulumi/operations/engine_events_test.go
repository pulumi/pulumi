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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/engine"
)

func TestWithContextEngineEvents(t *testing.T) {
	t.Parallel()

	event := engine.NewEvent(engine.StdoutEventPayload{Message: "hello"})

	t.Run("no observer", func(t *testing.T) {
		t.Parallel()
		events := make(chan engine.Event)
		got, wait := withContextEngineEvents(t.Context(), events)
		wait()
		assert.Equal(t, (chan<- engine.Event)(events), got)
	})

	t.Run("observer only", func(t *testing.T) {
		t.Parallel()
		observer := make(chan engine.Event, 1)
		got, wait := withContextEngineEvents(ContextWithEngineEvents(t.Context(), observer), nil)
		got <- event
		wait()
		assert.Equal(t, event, <-observer)
	})

	t.Run("observer and events", func(t *testing.T) {
		t.Parallel()
		observer := make(chan engine.Event, 1)
		events := make(chan engine.Event, 1)
		got, wait := withContextEngineEvents(ContextWithEngineEvents(t.Context(), observer), events)
		got <- event
		wait()
		assert.Equal(t, event, <-observer)
		assert.Equal(t, event, <-events)
	})
}
